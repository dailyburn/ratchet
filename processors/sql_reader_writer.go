package processors

import (
	"database/sql"

	"github.com/dailyburn/ratchet/data"
)

// SQLReaderWriter performs both the job of a SQLReader and SQLWriter.
// This means it will run a SQL query, write the resulting data into a
// SQL database, and (if the write was successful) send the queried data
// to the next stage of processing.
//
// SQLReaderWriter is composed of both a SQLReader and SQLWriter, so it
// supports all of the same properties and usage options (such as static
// versus dynamic SQL querying).
type SQLReaderWriter struct {
	SQLReader
	SQLWriter
	ConcurrencyLevel int // See ConcurrentDataProcessor
}

// NewSQLReaderWriter returns a new SQLReaderWriter ready for static querying.
func NewSQLReaderWriter(readConn *sql.DB, writeConn *sql.DB, readQuery, writeTable string) *SQLReaderWriter {
	s := SQLReaderWriter{}
	s.SQLReader = *NewSQLReader(readConn, readQuery)
	s.SQLWriter = *NewSQLWriter(writeConn, writeTable)
	return &s
}

// NewDynamicSQLReaderWriter returns a new SQLReaderWriter ready for dynamic querying.
func NewDynamicSQLReaderWriter(readConn *sql.DB, writeConn *sql.DB, sqlGenerator func(data.JSON) (string, error), writeTable string) *SQLReaderWriter {
	s := NewSQLReaderWriter(readConn, writeConn, "", writeTable)
	s.sqlGenerator = sqlGenerator
	return s
}

// ProcessData uses SQLReader methods for processing data - this works via composition
func (s *SQLReaderWriter) ProcessData(d data.JSON, outputChan chan data.JSON, killChan chan error) {
	s.ForEachQueryData(d, killChan, func(d data.JSON) {
		s.SQLWriter.ProcessData(d, outputChan, killChan)
		outputChan <- d
	})
}

// Finish - see interface for documentation.
func (s *SQLReaderWriter) Finish(outputChan chan data.JSON, killChan chan error) {
}

func (s *SQLReaderWriter) String() string {
	return "SQLReaderWriter"
}

// Concurrency defers to ConcurrentDataProcessor
func (s *SQLReaderWriter) Concurrency() int {
	return s.ConcurrencyLevel
}
