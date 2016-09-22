package client

import (
	"errors"
	"fmt"

	"io/ioutil"
	"strconv"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	bigquery "github.com/dailyburn/google-api-go-client-bigquery/bigquery/v2"
)

const authURL = "https://accounts.google.com/o/oauth2/auth"
const tokenURL = "https://accounts.google.com/o/oauth2/token"

const defaultPageSize = 5000
const defaultRequestTimeout = 60000
const maxRequestRetry = 5

// Client a big query client instance
type Client struct {
	pemPath           string
	token             *oauth2.Token
	service           *bigquery.Service
	allowLargeResults bool
	tempTableName     string
	flattenResults    bool
	PrintDebug        bool
	RequestTimeout    int64 // how long (in milliseconds) to try to create requests for large data (not a query timeout); defaults to 60000
}

// Data is a containing type used for Async data response handling including Headers, Rows and an Error that will be populated in the event of an Error querying
type Data struct {
	Headers []string
	Rows    [][]interface{}
	Err     error
}

// New instantiates a new client with the given params and return a reference to it
func New(pemPath string, options ...func(*Client) error) *Client {
	c := Client{
		pemPath:        pemPath,
		RequestTimeout: defaultRequestTimeout,
	}

	c.PrintDebug = false

	for _, option := range options {
		err := option(&c)
		if err != nil {
			return nil
		}
	}

	return &c
}

// AllowLargeResults is a configuration function that can be used to enable the AllowLargeResults setting
// of a bigquery request, as well as a temp table name to use to build the result data
//
// An example use is:
//
// client.New(pemPath, serviceAccountEmailAddress, serviceUserAccountClientID, clientSecret, client.AllowLargeResults(true, "tempTableName"))
//
func AllowLargeResults(shouldAllow bool, tempTableName string, flattenResults bool) func(*Client) error {
	return func(c *Client) error {
		return c.setAllowLargeResults(shouldAllow, tempTableName, flattenResults)
	}
}

// setAllowLargeResults - private function to set the AllowLargeResults and tempTableName values
func (c *Client) setAllowLargeResults(shouldAllow bool, tempTableName string, flattenResults bool) error {
	c.allowLargeResults = shouldAllow
	c.tempTableName = tempTableName
	c.flattenResults = flattenResults
	return nil
}

// connect - opens a new connection to bigquery, reusing the token if possible or regenerating a new auth token if required
func (c *Client) connect() (*bigquery.Service, error) {
	if c.token != nil {
		if !c.token.Valid() && c.service != nil {
			return c.service, nil
		}
	}

	// generate auth token and create service object
	//authScope := bigquery.BigqueryScope
	pemKeyBytes, err := ioutil.ReadFile(c.pemPath)
	if err != nil {
		panic(err)
	}

	t, err := google.JWTConfigFromJSON(
		pemKeyBytes,
		"https://www.googleapis.com/auth/bigquery")
	//t := jwt.NewToken(c.accountEmailAddress, bigquery.BigqueryScope, pemKeyBytes)
	client := t.Client(oauth2.NoContext)

	service, err := bigquery.New(client)
	if err != nil {
		return nil, err
	}

	c.service = service
	return service, nil
}

// InsertRow inserts a new row into the desired project, dataset and table or returns an error
func (c *Client) InsertRow(projectID, datasetID, tableID string, rowData map[string]interface{}) error {
	service, err := c.connect()
	if err != nil {
		return err
	}

	insertRequest := buildBigQueryInsertRequest([]map[string]interface{}{rowData})

	result, err := service.Tabledata.InsertAll(projectID, datasetID, tableID, insertRequest).Do()
	if err != nil {
		c.printDebug("Error inserting row: ", err)
		return err
	}

	if len(result.InsertErrors) > 0 {
		return errors.New("Error inserting row")
	}

	return nil
}

func (c *Client) InsertRows(projectID, datasetID, tableID string, rows []map[string]interface{}) error {
	service, err := c.connect()
	if err != nil {
		return err
	}

	insertRequest := buildBigQueryInsertRequest(rows)
	result, err := service.Tabledata.InsertAll(projectID, datasetID, tableID, insertRequest).Do()
	if err != nil {
		c.printDebug("Error inserting rows: ", err)
		return err
	}

	if len(result.InsertErrors) > 0 {
		return errors.New("Error inserting rows")
	}

	return nil
}

func buildBigQueryInsertRequest(rows []map[string]interface{}) *bigquery.TableDataInsertAllRequest {
	requestRows := []*bigquery.TableDataInsertAllRequestRows{}
	for _, row := range rows {
		requestRows = append(requestRows, &bigquery.TableDataInsertAllRequestRows{Json: rowToBigQueryJSON(row)})
	}
	return &bigquery.TableDataInsertAllRequest{Rows: requestRows}
}

func rowToBigQueryJSON(row map[string]interface{}) map[string]bigquery.JsonValue {
	// convert to the custom type bigquery lib wants
	jsonData := make(map[string]bigquery.JsonValue)
	for k, v := range row {
		jsonData[k] = bigquery.JsonValue(v)
	}
	return jsonData
}

// AsyncQuery loads the data by paging through the query results and sends back payloads over the dataChan - dataChan sends a payload containing Data objects made up of the headers, rows and an error attribute
func (c *Client) AsyncQuery(pageSize int, dataset, project, queryStr string, dataChan chan Data) {
	c.pagedQuery(pageSize, dataset, project, queryStr, dataChan)
}

// Query loads the data for the query paging if necessary and return the data rows, headers and error
func (c *Client) Query(dataset, project, queryStr string) ([][]interface{}, []string, error) {
	return c.pagedQuery(defaultPageSize, dataset, project, queryStr, nil)
}

// stdPagedQuery executes a query using default job parameters and paging over the results, returning them over the data chan provided
func (c *Client) stdPagedQuery(service *bigquery.Service, pageSize int, dataset, project, queryStr string, dataChan chan Data) ([][]interface{}, []string, error) {
	c.printDebug("std paged query")
	datasetRef := &bigquery.DatasetReference{
		DatasetId: dataset,
		ProjectId: project,
	}

	query := &bigquery.QueryRequest{
		DefaultDataset: datasetRef,
		MaxResults:     int64(pageSize),
		Kind:           "json",
		Query:          queryStr,
	}

	qr, err := service.Jobs.Query(project, query).Do()

	// extract the initial rows that have already been returned with the Query
	headers, rows := c.headersAndRows(qr.Schema, qr.Rows)

	if err != nil {
		c.printDebug("Error loading query: ", err)
		if dataChan != nil {
			dataChan <- Data{Err: err}
		}

		return nil, nil, err
	}

	return c.processPagedQuery(qr.JobReference, qr.PageToken, dataChan, headers, rows)
}

// largeDataPagedQuery builds a job and inserts it into the job queue allowing the flexibility to set the custom AllowLargeResults flag for the job
func (c *Client) largeDataPagedQuery(service *bigquery.Service, pageSize int, dataset, project, queryStr string, dataChan chan Data) ([][]interface{}, []string, error) {
	c.printDebug("largeDataPagedQuery starting")
	ts := time.Now()
	// start query
	tableRef := bigquery.TableReference{DatasetId: dataset, ProjectId: project, TableId: c.tempTableName}
	jobConfigQuery := bigquery.JobConfigurationQuery{}

	datasetRef := &bigquery.DatasetReference{
		DatasetId: dataset,
		ProjectId: project,
	}

	jobConfigQuery.AllowLargeResults = true
	jobConfigQuery.Query = queryStr
	jobConfigQuery.DestinationTable = &tableRef
	jobConfigQuery.DefaultDataset = datasetRef
	if !c.flattenResults {
		c.printDebug("setting FlattenResults to false")
		// need a pointer to bool
		f := false
		jobConfigQuery.FlattenResults = &f
	}
	jobConfigQuery.WriteDisposition = "WRITE_TRUNCATE"
	jobConfigQuery.CreateDisposition = "CREATE_IF_NEEDED"

	jobConfig := bigquery.JobConfiguration{}

	jobConfig.Query = &jobConfigQuery

	job := bigquery.Job{}
	job.Configuration = &jobConfig

	jobInsert := service.Jobs.Insert(project, &job)
	runningJob, jerr := jobInsert.Do()

	if jerr != nil {
		c.printDebug("Error inserting job!", jerr)
		if dataChan != nil {
			dataChan <- Data{Err: jerr}
		}
		return nil, nil, jerr
	}

	var qr *bigquery.GetQueryResultsResponse
	var rows [][]interface{}
	var headers []string
	var err error

	// Periodically, job references are not created, but errors are also not thrown.
	// In this scenario, retry up to 5 times to get a job reference before giving up.
	for i := 1; ; i++ {
		r := service.Jobs.GetQueryResults(project, runningJob.JobReference.JobId)
		r.TimeoutMs(c.RequestTimeout)
		qr, err = r.Do()

		headers, rows = c.headersAndRows(qr.Schema, qr.Rows)

		if i >= maxRequestRetry || qr.JobReference != nil || err != nil {
			if i > 1 {
				c.printDebug("Took %v tries to get a job reference", i)
			}
			break
		}
	}

	if err == nil && qr.JobReference == nil {
		err = fmt.Errorf("missing job reference")
	}

	if err != nil {
		c.printDebug("Error loading query: ", err)
		if dataChan != nil {
			dataChan <- Data{Err: err}
		}
		return nil, nil, err
	}

	rows, headers, err = c.processPagedQuery(qr.JobReference, qr.PageToken, dataChan, headers, rows)
	c.printDebug("largeDataPagedQuery completed in ", time.Now().Sub(ts).Seconds(), "s")

	return rows, headers, err
}

// pagedQuery executes the query using bq's paging mechanism to load all results and sends them back via dataChan if available, otherwise it returns the full result set, headers and error as return values
func (c *Client) pagedQuery(pageSize int, dataset, project, queryStr string, dataChan chan Data) ([][]interface{}, []string, error) {
	// connect to service
	service, err := c.connect()
	if err != nil {
		if dataChan != nil {
			dataChan <- Data{Err: err}
		}
		return nil, nil, err
	}

	if c.allowLargeResults && len(c.tempTableName) > 0 {
		return c.largeDataPagedQuery(service, pageSize, dataset, project, queryStr, dataChan)
	}

	return c.stdPagedQuery(service, pageSize, dataset, project, queryStr, dataChan)
}

func (c *Client) processPagedQuery(jobRef *bigquery.JobReference, pageToken string, dataChan chan Data, headers []string, rows [][]interface{}) ([][]interface{}, []string, error) {
	if len(rows) > 0 {
		dataChan <- Data{Headers: headers, Rows: rows}
	}

	resultChan := make(chan [][]interface{})
	headersChan := make(chan []string)

	go c.pageOverJob(len(rows), jobRef, pageToken, resultChan, headersChan)

L:
	for {
		select {
		case h, ok := <-headersChan:
			if ok {
				c.printDebug("got headers")
				headers = h
			}
		case newRows, ok := <-resultChan:
			if !ok {
				break L
			}
			if dataChan != nil {
				c.printDebug("got rows", len(newRows))
				dataChan <- Data{Headers: headers, Rows: newRows}
			} else {
				rows = append(rows, newRows...)
			}
		}
	}

	if dataChan != nil {
		close(dataChan)
	}

	return rows, headers, nil
}

// pageOverJob loads results for the given job reference and if the total results has not been hit continues to load recursively
func (c *Client) pageOverJob(rowCount int, jobRef *bigquery.JobReference, pageToken string, resultChan chan [][]interface{}, headersChan chan []string) error {
	service, err := c.connect()
	if err != nil {
		return err
	}

	qrc := service.Jobs.GetQueryResults(jobRef.ProjectId, jobRef.JobId)
	if len(pageToken) > 0 {
		qrc.PageToken(pageToken)
	}

	qr, err := qrc.Do()
	if err != nil {
		c.printDebug("Error loading additional data: ", err)
		close(resultChan)
		return err
	}

	if qr.JobComplete {
		c.printDebug("qr.JobComplete")
		headers, rows := c.headersAndRows(qr.Schema, qr.Rows)
		if headersChan != nil {
			headersChan <- headers
			close(headersChan)
		}

		// send back the rows we got
		c.printDebug("sending rows")
		resultChan <- rows
		rowCount = rowCount + len(rows)
		c.printDebug("Total rows: ", rowCount)
	}

	if qr.TotalRows > uint64(rowCount) || !qr.JobComplete {
		c.printDebug("!qr.JobComplete")
		if qr.JobReference == nil {
			c.pageOverJob(rowCount, jobRef, pageToken, resultChan, headersChan)
		} else {
			c.pageOverJob(rowCount, qr.JobReference, qr.PageToken, resultChan, nil)
		}
	} else {
		close(resultChan)
		return nil
	}

	return nil
}

// SyncQuery executes an arbitrary query string and returns the result synchronously (unless the response takes longer than the provided timeout)
func (c *Client) SyncQuery(dataset, project, queryStr string, maxResults int64) ([][]interface{}, error) {
	service, err := c.connect()
	if err != nil {
		return nil, err
	}

	datasetRef := &bigquery.DatasetReference{
		DatasetId: dataset,
		ProjectId: project,
	}

	query := &bigquery.QueryRequest{
		DefaultDataset: datasetRef,
		MaxResults:     maxResults,
		Kind:           "json",
		Query:          queryStr,
	}

	results, err := service.Jobs.Query(project, query).Do()
	if err != nil {
		c.printDebug("Query Error: ", err)
		return nil, err
	}

	// credit to https://github.com/getlantern/statshub for the row building approach
	numRows := int(results.TotalRows)
	if numRows > int(maxResults) {
		numRows = int(maxResults)
	}

	_, rows := c.headersAndRows(results.Schema, results.Rows)
	return rows, nil
}

func (c *Client) headersAndRows(bqSchema *bigquery.TableSchema, bqRows []*bigquery.TableRow) ([]string, [][]interface{}) {
	if bqSchema == nil || bqRows == nil {
		return nil, nil
	}

	c.printDebug("headersAndRows starting")
	ts := time.Now()
	headers := make([]string, len(bqSchema.Fields))
	// c.printDebug("len(bqRows) is", len(bqRows))
	rows := make([][]interface{}, len(bqRows))
	// Create headers
	for i, f := range bqSchema.Fields {
		headers[i] = f.Name
	}
	// Create rows
	for i, tableRow := range bqRows {
		row := make([]interface{}, len(bqSchema.Fields))
		for j, tableCell := range tableRow.F {
			schemaField := bqSchema.Fields[j]
			if schemaField.Type == "RECORD" {
				row[j] = c.nestedFieldsData(schemaField.Fields, tableCell.V)
			} else {
				row[j] = tableCell.V
			}
		}
		rows[i] = row
		// c.printDebug(fmt.Sprintf("built rows[%d] %+v", i, row))
	}
	c.printDebug("headersAndRows completed in ", time.Now().Sub(ts).Seconds(), "s")
	return headers, rows
}

func (c *Client) nestedFieldsData(nestedFields []*bigquery.TableFieldSchema, tableCellVal interface{}) interface{} {
	// the contents of the nested data is pretty crazy... takes a lot of debugging
	switch tcv := tableCellVal.(type) {
	// non-repeated RECORD
	case map[string]interface{}:
		data := make(map[string]interface{})
		vals, ok := tcv["f"]
		if !ok {
			c.printDebug("No f key found in nested values!")
		}

		for i, f := range nestedFields {
			v := vals.([]interface{})[i]
			vv := v.(map[string]interface{})["v"]
			if f.Type == "RECORD" {
				data[f.Name] = c.nestedFieldsData(f.Fields, vv)
			} else {
				data[f.Name] = vv
			}
		}
		return data
	// REPEATED RECORD
	case []interface{}:
		data := make([]map[string]interface{}, len(tcv))
		for j, mapv := range tcv {
			d := make(map[string]interface{})
			mapvv, ok := mapv.(map[string]interface{})["v"]
			if !ok {
				c.printDebug("No v key found in nested+repeated values!")
			}
			vals, ok := mapvv.(map[string]interface{})["f"]
			if !ok {
				c.printDebug("No f key found in nested+repeated values!")
			}

			for i, f := range nestedFields {
				v := vals.([]interface{})[i]
				vv := v.(map[string]interface{})["v"]
				if f.Type == "RECORD" {
					d[f.Name] = c.nestedFieldsData(f.Fields, vv)
				} else {
					d[f.Name] = vv
				}
			}
			data[j] = d
		}
		return data
	default:
		c.printDebug(fmt.Sprintf("Unexpected type in nestedFieldsdata: %T", tcv))
		return nil
	}
}

// Count loads the row count for the provided dataset.tablename
func (c *Client) Count(dataset, project, datasetTable string) int64 {
	qstr := fmt.Sprintf("select count(*) from [%s]", datasetTable)
	res, err := c.SyncQuery(dataset, project, qstr, 1)
	if err == nil {
		if len(res) > 0 {
			val, _ := strconv.ParseInt(res[0][0].(string), 10, 64)
			return val
		}
	}
	return 0
}

func (c *Client) printDebug(v ...interface{}) {
	if c.PrintDebug {
		fmt.Println(v)
	}
}
