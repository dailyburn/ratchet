package processors

import (
	"database/sql"

	"github.com/dailyburn/ratchet/data"
	"github.com/dailyburn/ratchet/logger"
	"github.com/dailyburn/ratchet/util"
)

// SQLWriter handles INSERTing data.JSON into a
// specified SQL table. If an error occurs while building
// or executing the INSERT, the error will be sent to the killChan.
//
// Note that the data.JSON must be a valid JSON object or a slice
// of valid objects, where the keys are column names and the
// the values are the SQL values to be inserted into those columns.
//
// For use-cases where a SQLWriter instance needs to write to
// multiple tables you can pass in SQLWriterData.
type SQLWriter struct {
	writeDB          *sql.DB
	TableName        string
	OnDupKeyUpdate   bool
	OnDupKeyFields   []string
	ConcurrencyLevel int // See ConcurrentDataProcessor
	BatchSize        int
}

// SQLWriterData is a custom data structure you can send into a SQLWriter
// stage if you need to specify TableName on a per-data payload basis. No
// extra configuration is needed to use SQLWriterData, each data payload
// received is first checked for this structure before processing.
type SQLWriterData struct {
	TableName  string      `json:"table_name"`
	InsertData interface{} `json:"insert_data"`
}

// NewSQLWriter returns a new SQLWriter
func NewSQLWriter(db *sql.DB, tableName string) *SQLWriter {
	return &SQLWriter{writeDB: db, TableName: tableName, OnDupKeyUpdate: true}
}

// ProcessData defers to util.SQLInsertData
func (s *SQLWriter) ProcessData(d data.JSON, outputChan chan data.JSON, killChan chan error) {
	// handle panics a bit more gracefully
	defer func() {
		if err := recover(); err != nil {
			util.KillPipelineIfErr(err.(error), killChan)
		}
	}()

	// First check for SQLWriterData
	var wd SQLWriterData
	err := data.ParseJSONSilent(d, &wd)
	logger.Info("SQLWriter: Writing data...")
	if err == nil && wd.TableName != "" && wd.InsertData != nil {
		logger.Debug("SQLWriter: SQLWriterData scenario")
		dd, err := data.NewJSON(wd.InsertData)
		util.KillPipelineIfErr(err, killChan)
		err = util.SQLInsertData(s.writeDB, dd, wd.TableName, s.OnDupKeyUpdate, s.OnDupKeyFields, s.BatchSize)
		util.KillPipelineIfErr(err, killChan)
	} else {
		logger.Debug("SQLWriter: normal data scenario")
		err = util.SQLInsertData(s.writeDB, d, s.TableName, s.OnDupKeyUpdate, s.OnDupKeyFields, s.BatchSize)
		util.KillPipelineIfErr(err, killChan)
	}
	logger.Info("SQLWriter: Write complete")
}

// Finish - see interface for documentation.
func (s *SQLWriter) Finish(outputChan chan data.JSON, killChan chan error) {
}

func (s *SQLWriter) String() string {
	return "SQLWriter"
}

// Concurrency defers to ConcurrentDataProcessor
func (s *SQLWriter) Concurrency() int {
	return s.ConcurrencyLevel
}
