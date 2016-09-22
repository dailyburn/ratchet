package processors

import (
	"database/sql"
	"errors"

	"github.com/dailyburn/ratchet/data"
	"github.com/dailyburn/ratchet/logger"
	"github.com/dailyburn/ratchet/util"
)

// SQLExecutor runs the given SQL and swallows any returned data.
//
// It can operate in 2 modes:
// 1) Static - runs the given SQL query and ignores any received data.
// 2) Dynamic - generates a SQL query for each data payload it receives.
//
// The dynamic SQL generation is implemented by passing in a "sqlGenerator"
// function to NewDynamicSQLExecutor. This allows you to write whatever
// code is needed to generate SQL based upon data flowing through the pipeline.
type SQLExecutor struct {
	readDB       *sql.DB
	query        string
	sqlGenerator func(data.JSON) (string, error)
}

// NewSQLExecutor returns a new SQLExecutor
func NewSQLExecutor(dbConn *sql.DB, sql string) *SQLExecutor {
	return &SQLExecutor{readDB: dbConn, query: sql}
}

// NewDynamicSQLExecutor returns a new SQLExecutor operating in dynamic mode.
func NewDynamicSQLExecutor(dbConn *sql.DB, sqlGenerator func(data.JSON) (string, error)) *SQLExecutor {
	return &SQLExecutor{readDB: dbConn, sqlGenerator: sqlGenerator}
}

// ProcessData runs the SQL statements, deferring to util.ExecuteSQLQuery
func (s *SQLExecutor) ProcessData(d data.JSON, outputChan chan data.JSON, killChan chan error) {
	// handle panics a bit more gracefully
	defer func() {
		if err := recover(); err != nil {
			util.KillPipelineIfErr(err.(error), killChan)
		}
	}()

	sql := ""
	var err error
	if s.query == "" && s.sqlGenerator != nil {
		sql, err = s.sqlGenerator(d)
		util.KillPipelineIfErr(err, killChan)
	} else if s.query != "" {
		sql = s.query
	} else {
		killChan <- errors.New("SQLExecutor: must have either static query or sqlGenerator func")
	}

	logger.Debug("SQLExecutor: Running - ", sql)
	// See sql.go
	err = util.ExecuteSQLQuery(s.readDB, sql)
	util.KillPipelineIfErr(err, killChan)
	logger.Info("SQLExecutor: Query complete")
}

// Finish - see interface for documentation.
func (s *SQLExecutor) Finish(outputChan chan data.JSON, killChan chan error) {
}

func (s *SQLExecutor) String() string {
	return "SQLExecutor"
}
