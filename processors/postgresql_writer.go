package processors

import (
	"database/sql"

	"github.com/dailyburn/ratchet/data"
	"github.com/dailyburn/ratchet/logger"
	"github.com/dailyburn/ratchet/util"
)

// PostgreSQLWriter handles INSERTing data.JSON into a
// specified SQL table. If an error occurs while building
// or executing the INSERT, the error will be sent to the killChan.
//
// Note that the data.JSON must be a valid JSON object or a slice
// of valid objects, where the keys are column names and the
// the values are the SQL values to be inserted into those columns.
//
// For use-cases where a PostgreSQLWriter instance needs to write to
// multiple tables you can pass in SQLWriterData.
//
// Note that if `OnDupKeyUpdate` is true (the default), you *must*
// provide a value for `OnDupKeyIndex` (which is the PostgreSQL
// conflict target).
type PostgreSQLWriter struct {
	writeDB          *sql.DB
	TableName        string
	OnDupKeyUpdate   bool
	OnDupKeyIndex    string // The conflict target: see https://www.postgresql.org/docs/9.5/static/sql-insert.html
	OnDupKeyFields   []string
	ConcurrencyLevel int // See ConcurrentDataProcessor
	BatchSize        int
}

// NewPostgreSQLWriter returns a new PostgreSQLWriter
func NewPostgreSQLWriter(db *sql.DB, tableName string) *PostgreSQLWriter {
	return &PostgreSQLWriter{writeDB: db, TableName: tableName, OnDupKeyUpdate: true}
}

// ProcessData defers to util.PostgreSQLInsertData
func (s *PostgreSQLWriter) ProcessData(d data.JSON, outputChan chan data.JSON, killChan chan error) {
	// handle panics a bit more gracefully
	defer func() {
		if err := recover(); err != nil {
			util.KillPipelineIfErr(err.(error), killChan)
		}
	}()

	// First check for SQLWriterData
	var wd SQLWriterData
	err := data.ParseJSONSilent(d, &wd)
	logger.Info("PostgreSQLWriter: Writing data...")
	if err == nil && wd.TableName != "" && wd.InsertData != nil {
		logger.Debug("PostgreSQLWriter: SQLWriterData scenario")
		dd, err := data.NewJSON(wd.InsertData)
		util.KillPipelineIfErr(err, killChan)
		err = util.PostgreSQLInsertData(s.writeDB, dd, wd.TableName, s.OnDupKeyUpdate, s.OnDupKeyIndex, s.OnDupKeyFields, s.BatchSize)
		util.KillPipelineIfErr(err, killChan)
	} else {
		logger.Debug("PostgreSQLWriter: normal data scenario")
		err = util.PostgreSQLInsertData(s.writeDB, d, s.TableName, s.OnDupKeyUpdate, s.OnDupKeyIndex, s.OnDupKeyFields, s.BatchSize)
		util.KillPipelineIfErr(err, killChan)
	}
	logger.Info("PostgreSQLWriter: Write complete")
}

// Finish - see interface for documentation.
func (s *PostgreSQLWriter) Finish(outputChan chan data.JSON, killChan chan error) {
}

func (s *PostgreSQLWriter) String() string {
	return "PostgreSQLWriter"
}

// Concurrency defers to ConcurrentDataProcessor
func (s *PostgreSQLWriter) Concurrency() int {
	return s.ConcurrencyLevel
}
