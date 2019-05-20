package util

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/dailyburn/ratchet/data"
	"github.com/dailyburn/ratchet/logger"
)

// MySQLInsertData abstracts building and executing a SQL INSERT
// statement for the given Data object.
//
// Note that the Data must be a valid JSON object
// (or an array of valid objects all with the same keys),
// where the keys are column names and the
// the values are SQL values to be inserted into those columns.
func MySQLInsertData(db *sql.DB, d data.JSON, tableName string, onDupKeyUpdate bool, onDupKeyFields []string, batchSize int) error {
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
			err = mysqlInsertObjects(db, objects[i:maxIndex], tableName, onDupKeyUpdate, onDupKeyFields)
			if err != nil {
				return err
			}
		}
		return nil
	}

	return mysqlInsertObjects(db, objects, tableName, onDupKeyUpdate, onDupKeyFields)
}

func mysqlInsertObjects(db *sql.DB, objects []map[string]interface{}, tableName string, onDupKeyUpdate bool, onDupKeyFields []string) error {
	logger.Info("MySQLInsertData: building INSERT for len(objects) =", len(objects))
	insertSQL, vals := buildMySQLInsertSQL(objects, tableName, onDupKeyUpdate, onDupKeyFields)

	logger.Debug("MySQLInsertData:", insertSQL)
	logger.Debug("MySQLInsertData: values", vals)

	stmt, err := db.Prepare(insertSQL)
	if err != nil {
		logger.Debug("MySQLInsertData: error preparing SQL")
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

	logger.Info(fmt.Sprintf("MySQLInsertData: rows affected = %d, last insert ID = %d", rowCnt, lastID))
	return nil
}

func buildMySQLInsertSQL(objects []map[string]interface{}, tableName string, onDupKeyUpdate bool, onDupKeyFields []string) (insertSQL string, vals []interface{}) {
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
