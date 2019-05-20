package processors

import (
	"database/sql"

	"github.com/dailyburn/ratchet/data"
	"github.com/dailyburn/ratchet/logger"
	"github.com/dailyburn/ratchet/util"
)

// MySQLWriter handles INSERTing data.JSON into a
// specified SQL table. If an error occurs while building
// or executing the INSERT, the error will be sent to the killChan.
//
// Note that the data.JSON must be a valid JSON object or a slice
// of valid objects, where the keys are column names and the
// the values are the SQL values to be inserted into those columns.
//
// For use-cases where a MySQLWriter instance needs to write to
// multiple tables you can pass in SQLWriterData.
type MySQLWriter struct {
	writeDB          *sql.DB
	TableName        string
	OnDupKeyUpdate   bool
	OnDupKeyFields   []string
	ConcurrencyLevel int // See ConcurrentDataProcessor
	BatchSize        int
}

// NewMySQLWriter returns a new MySQLWriter
func NewMySQLWriter(db *sql.DB, tableName string) *MySQLWriter {
	return &MySQLWriter{writeDB: db, TableName: tableName, OnDupKeyUpdate: true}
}

// ProcessData defers to util.MySQLInsertData
func (s *MySQLWriter) ProcessData(d data.JSON, outputChan chan data.JSON, killChan chan error) {
	// handle panics a bit more gracefully
	defer func() {
		if err := recover(); err != nil {
			util.KillPipelineIfErr(err.(error), killChan)
		}
	}()

	// First check for SQLWriterData
	var wd SQLWriterData
	err := data.ParseJSONSilent(d, &wd)
	logger.Info("MySQLWriter: Writing data...")
	if err == nil && wd.TableName != "" && wd.InsertData != nil {
		logger.Debug("MySQLWriter: SQLWriterData scenario")
		dd, err := data.NewJSON(wd.InsertData)
		util.KillPipelineIfErr(err, killChan)
		err = util.MySQLInsertData(s.writeDB, dd, wd.TableName, s.OnDupKeyUpdate, s.OnDupKeyFields, s.BatchSize)
		util.KillPipelineIfErr(err, killChan)
	} else {
		logger.Debug("MySQLWriter: normal data scenario")
		err = util.MySQLInsertData(s.writeDB, d, s.TableName, s.OnDupKeyUpdate, s.OnDupKeyFields, s.BatchSize)
		util.KillPipelineIfErr(err, killChan)
	}
	logger.Info("MySQLWriter: Write complete")
}

// Finish - see interface for documentation.
func (s *MySQLWriter) Finish(outputChan chan data.JSON, killChan chan error) {
}

func (s *MySQLWriter) String() string {
	return "MySQLWriter"
}

// Concurrency defers to ConcurrentDataProcessor
func (s *MySQLWriter) Concurrency() int {
	return s.ConcurrencyLevel
}
