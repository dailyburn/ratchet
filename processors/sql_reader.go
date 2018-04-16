package processors

import (
	"database/sql"
	"errors"

	"github.com/dailyburn/ratchet/data"
	"github.com/dailyburn/ratchet/logger"
	"github.com/dailyburn/ratchet/util"
)

// SQLReader runs the given SQL and passes the resulting data
// to the next stage of processing.
//
// It can operate in 2 modes:
// 1) Static - runs the given SQL query and ignores any received data.
// 2) Dynamic - generates a SQL query for each data payload it receives.
//
// The dynamic SQL generation is implemented by passing in a "sqlGenerator"
// function to NewDynamicSQLReader. This allows you to write whatever code is
// needed to generate SQL based upon data flowing through the pipeline.
type SQLReader struct {
	readDB            *sql.DB
	query             string
	sqlGenerator      func(data.JSON) (string, error)
	BatchSize         int
	StructDestination interface{}
	ConcurrencyLevel  int // See ConcurrentDataProcessor
}

type dataErr struct {
	Error string
}

// NewSQLReader returns a new SQLReader operating in static mode.
func NewSQLReader(dbConn *sql.DB, sql string) *SQLReader {
	return &SQLReader{readDB: dbConn, query: sql, BatchSize: 1000}
}

// NewDynamicSQLReader returns a new SQLReader operating in dynamic mode.
func NewDynamicSQLReader(dbConn *sql.DB, sqlGenerator func(data.JSON) (string, error)) *SQLReader {
	return &SQLReader{readDB: dbConn, sqlGenerator: sqlGenerator, BatchSize: 1000}
}

// ProcessData - see interface for documentation.
func (s *SQLReader) ProcessData(d data.JSON, outputChan chan data.JSON, killChan chan error) {
	s.ForEachQueryData(d, killChan, func(d data.JSON) {
		outputChan <- d
	})
}

// ForEachQueryData handles generating the SQL (in case of dynamic mode),
// running the query and retrieving the data in data.JSON format, and then
// passing the results back within the function call to forEach.
func (s *SQLReader) ForEachQueryData(d data.JSON, killChan chan error, forEach func(d data.JSON)) {
	sql := ""
	var err error
	if s.query == "" && s.sqlGenerator != nil {
		sql, err = s.sqlGenerator(d)
		util.KillPipelineIfErr(err, killChan)
	} else if s.query != "" {
		sql = s.query
	} else {
		killChan <- errors.New("SQLReader: must have either static query or sqlGenerator func")
	}

	logger.Debug("SQLReader: Running - ", sql)
	// See sql.go
	dataChan, err := util.GetDataFromSQLQuery(s.readDB, sql, s.BatchSize, s.StructDestination)
	util.KillPipelineIfErr(err, killChan)

	for d := range dataChan {
		// First check if an error was returned back from the SQL processing
		// helper, then if not call forEach with the received data.
		var derr dataErr
		if err := data.ParseJSONSilent(d, &derr); err == nil {
			util.KillPipelineIfErr(errors.New(derr.Error), killChan)
		} else {
			forEach(d)
		}
	}
}

// Finish - see interface for documentation.
func (s *SQLReader) Finish(outputChan chan data.JSON, killChan chan error) {
}

func (s *SQLReader) String() string {
	return "SQLReader"
}

// Concurrency defers to ConcurrentDataProcessor
func (s *SQLReader) Concurrency() int {
	return s.ConcurrencyLevel
}
