package util

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/dailyburn/ratchet/data"
	"github.com/dailyburn/ratchet/logger"
	"github.com/kisielk/sqlstruct"
)

// GetDataFromSQLQuery is a util function that, given a properly intialized sql.DB
// and a valid SQL query, will handle executing the query and getting back data.JSON
// objects. This function is asynch, and data.JSON should be received on the return
// data channel. If there was a problem setting up the query, then an error will also be
// returned immediately. It is also possible for errors to occur during execution as data
// is retrieved from the query. If this happens, the object returned will be a JSON
// object in the form of {"Error": "description"}.
func GetDataFromSQLQuery(db *sql.DB, query string, batchSize int, structDest interface{}) (chan data.JSON, error) {
	stmt, err := db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return nil, err
	}

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	dataChan := make(chan data.JSON)

	if structDest != nil {
		go scanRowsUsingStruct(rows, columns, structDest, batchSize, dataChan)
	} else {
		go scanDataGeneric(rows, columns, batchSize, dataChan)
	}

	return dataChan, nil
}

func scanRowsUsingStruct(rows *sql.Rows, columns []string, structDest interface{}, batchSize int, dataChan chan data.JSON) {
	defer rows.Close()

	tableData := []map[string]interface{}{}

	for rows.Next() {
		err := sqlstruct.Scan(structDest, rows)
		if err != nil {
			sendErr(err, dataChan)
		}

		d, err := data.NewJSON(structDest)
		if err != nil {
			sendErr(err, dataChan)
		}

		entry := make(map[string]interface{})
		err = data.ParseJSON(d, &entry)
		if err != nil {
			sendErr(err, dataChan)
		}

		tableData = append(tableData, entry)

		if batchSize > 0 && len(tableData) >= batchSize {
			sendTableData(tableData, dataChan)
			tableData = []map[string]interface{}{}
		}
	}
	if rows.Err() != nil {
		sendErr(rows.Err(), dataChan)
	}

	// Flush remaining tableData
	if len(tableData) > 0 {
		sendTableData(tableData, dataChan)
	}

	close(dataChan) // signal completion to caller
}

func scanDataGeneric(rows *sql.Rows, columns []string, batchSize int, dataChan chan data.JSON) {
	defer rows.Close()

	tableData := []map[string]interface{}{}
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := 0; i < len(columns); i++ {
		valuePtrs[i] = &values[i]
	}

	for rows.Next() {
		err := rows.Scan(valuePtrs...)
		if err != nil {
			sendErr(err, dataChan)
		}

		entry := make(map[string]interface{})
		for i, col := range columns {
			var v interface{}
			val := values[i]
			// logger.Debug("Value Type for", col, " -> ", reflect.TypeOf(val))
			switch vv := val.(type) {
			case []byte:
				v = string(vv)
			default:
				v = vv
			}
			entry[col] = v
		}
		tableData = append(tableData, entry)

		if batchSize > 0 && len(tableData) >= batchSize {
			sendTableData(tableData, dataChan)
			tableData = []map[string]interface{}{}
		}
	}
	if rows.Err() != nil {
		sendErr(rows.Err(), dataChan)
	}

	// Flush remaining tableData
	if len(tableData) > 0 {
		sendTableData(tableData, dataChan)
	}

	close(dataChan) // signal completion to caller
}

// http://play.golang.org/p/2wHfO6YS3_
func determineBytesValue(b []byte) (interface{}, error) {
	var v interface{}
	err := data.ParseJSONSilent(b, &v)
	if err != nil {
		// need to quote strings for JSON to parse correctly
		if !strings.Contains(string(b), `"`) {
			b = []byte(fmt.Sprintf(`"%v"`, string(b)))
			return determineBytesValue(b)
		}
	}
	switch vv := v.(type) {
	case []byte:
		return string(vv), err
	default:
		return v, err
	}
}

func sendTableData(tableData []map[string]interface{}, dataChan chan data.JSON) {
	d, err := data.NewJSON(tableData)
	if err != nil {
		sendErr(err, dataChan)
	} else {
		dataChan <- d
	}
}

func sendErr(err error, dataChan chan data.JSON) {
	dataChan <- []byte(`{"Error":"` + err.Error() + `"}`)
}

// ExecuteSQLQuery allows you to execute arbitrary SQL statements
func ExecuteSQLQuery(db *sql.DB, query string) error {
	stmt, err := db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Query(query)
	if err != nil {
		return err
	}

	return nil
}

// SQLInsertData abstracts building and executing a SQL INSERT
// statement for the given Data object.
//
// Note that the Data must be a valid JSON object
// (or an array of valid objects all with the same keys),
// where the keys are column names and the
// the values are SQL values to be inserted into those columns.
func SQLInsertData(db *sql.DB, d data.JSON, tableName string, onDupKeyUpdate bool, onDupKeyFields []string, batchSize int) error {
	objects, err := data.ObjectsFromJSON(d)
	if err != nil {
		return err
	}

	if batchSize > 0 {
		for i := 0; i < len(objects); i += batchSize {
			maxIndex := i + batchSize
			if maxIndex > len(objects) {
				maxIndex = len(objects)
			}
			err = insertObjects(db, objects[i:maxIndex], tableName, onDupKeyUpdate, onDupKeyFields)
			if err != nil {
				return err
			}
		}
		return nil
	}

	return insertObjects(db, objects, tableName, onDupKeyUpdate, onDupKeyFields)
}

func insertObjects(db *sql.DB, objects []map[string]interface{}, tableName string, onDupKeyUpdate bool, onDupKeyFields []string) error {
	logger.Info("SQLInsertData: building INSERT for len(objects) =", len(objects))
	insertSQL, vals := buildInsertSQL(objects, tableName, onDupKeyUpdate, onDupKeyFields)

	logger.Debug("SQLInsertData:", insertSQL)
	logger.Debug("SQLInsertData: values", vals)

	stmt, err := db.Prepare(insertSQL)
	if err != nil {
		logger.Debug("SQLInsertData: error preparing SQL")
		return err
	}
	defer stmt.Close()

	res, err := stmt.Exec(vals...)
	if err != nil {
		return err
	}
	lastID, err := res.LastInsertId()
	if err != nil {
		return err
	}
	rowCnt, err := res.RowsAffected()
	if err != nil {
		return err
	}

	logger.Info(fmt.Sprintf("SQLInsertData: rows affected = %d, last insert ID = %d", rowCnt, lastID))
	return nil
}

func buildInsertSQL(objects []map[string]interface{}, tableName string, onDupKeyUpdate bool, onDupKeyFields []string) (insertSQL string, vals []interface{}) {
	cols := sortedColumns(objects)

	// Format: INSERT INTO tablename(col1,col2) VALUES(?,?),(?,?)
	insertSQL = fmt.Sprintf("INSERT INTO %v(%v) VALUES", tableName, strings.Join(cols, ","))

	// builds the (?,?) part
	qs := "("
	for i := 0; i < len(cols); i++ {
		if i > 0 {
			qs += ","
		}
		qs += "?"
	}
	qs += ")"
	// append as many (?,?) parts as there are objects to insert
	for i := 0; i < len(objects); i++ {
		if i > 0 {
			insertSQL += ","
		}
		insertSQL += qs
	}

	if onDupKeyUpdate {
		// format: ON DUPLICATE KEY UPDATE a=VALUES(a), b=VALUES(b), c=VALUES(c)
		insertSQL += " ON DUPLICATE KEY UPDATE "

		// If this wasn't explicitly set, we want to update all columns
		if len(onDupKeyFields) == 0 {
			onDupKeyFields = cols
		}

		for i, c := range onDupKeyFields {
			if i > 0 {
				insertSQL += ","
			}
			insertSQL += "`" + c + "`=VALUES(`" + c + "`)"
		}
	}

	vals = []interface{}{}
	for _, obj := range objects {
		for _, col := range cols {
			if val, ok := obj[col]; ok {
				vals = append(vals, val)
			} else {
				vals = append(vals, nil)
			}
		}
	}

	return
}

func sortedColumns(objects []map[string]interface{}) []string {
	// Since we don't know if all objects have the same keys, we need to
	// iterate over all the objects to gather all possible keys/columns
	// to use in the INSERT statement.
	colsMap := make(map[string]struct{})
	for _, o := range objects {
		for col := range o {
			colsMap[col] = struct{}{}
		}
	}

	cols := []string{}
	for col := range colsMap {
		cols = append(cols, col)
	}
	sort.Strings(cols)
	return cols
}
