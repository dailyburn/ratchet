package util

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/dailyburn/ratchet/data"
	"github.com/dailyburn/ratchet/logger"
)

// PostgreSQLInsertData abstracts building and executing a SQL INSERT
// statement for the given Data object.
//
// Note that the Data must be a valid JSON object
// (or an array of valid objects all with the same keys),
// where the keys are column names and the
// the values are SQL values to be inserted into those columns.
//
// If onDupKeyUpdate is true, you must set an onDupKeyIndex. This translates
// to the conflict_target as specified in https://www.postgresql.org/docs/9.5/static/sql-insert.html
func PostgreSQLInsertData(db *sql.DB, d data.JSON, tableName string, onDupKeyUpdate bool, onDupKeyIndex string, onDupKeyFields []string, batchSize int) error {
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
			err = postgresInsertObjects(db, objects[i:maxIndex], tableName, onDupKeyUpdate, onDupKeyIndex, onDupKeyFields)
			if err != nil {
				return err
			}
		}
		return nil
	}

	return postgresInsertObjects(db, objects, tableName, onDupKeyUpdate, onDupKeyIndex, onDupKeyFields)
}

func postgresInsertObjects(db *sql.DB, objects []map[string]interface{}, tableName string, onDupKeyUpdate bool, onDupKeyIndex string, onDupKeyFields []string) error {
	logger.Info("PostgreSQLInsertData: building INSERT for len(objects) =", len(objects))
	insertSQL, vals := buildPostgreSQLInsertSQL(objects, tableName, onDupKeyUpdate, onDupKeyIndex, onDupKeyFields)

	logger.Debug("PostgreSQLInsertData:", insertSQL)
	logger.Debug("PostgreSQLInsertData: values", vals)

	stmt, err := db.Prepare(insertSQL)
	if err != nil {
		logger.Debug("PostgreSQLInsertData: error preparing SQL")
		return err
	}
	defer stmt.Close()

	res, err := stmt.Exec(vals...)
	if err != nil {
		return err
	}
	rowCnt, err := res.RowsAffected()
	if err != nil {
		return err
	}

	logger.Info(fmt.Sprintf("PostgreSQLInsertData: rows affected = %d", rowCnt))
	return nil
}

func buildPostgreSQLInsertSQL(objects []map[string]interface{}, tableName string, onDupKeyUpdate bool, onDupKeyIndex string, onDupKeyFields []string) (insertSQL string, vals []interface{}) {
	cols := sortedColumns(objects)

	// Format: INSERT INTO tablename(col1,col2) VALUES(?,?),(?,?)
	insertSQL = fmt.Sprintf("INSERT INTO %v(%v) VALUES", tableName, strings.Join(cols, ","))

	for i := 0; i < len(objects); i++ {
		row := "("

		if i > 0 {
			insertSQL += ", "
		}

		for j := 0; j < len(cols); j++ {
			if j > 0 {
				row += ","
			}

			placeholder := fmt.Sprintf("$%v", i*len(cols)+j+1)
			row += placeholder
		}

		row += ")"
		insertSQL += row
	}

	if onDupKeyUpdate {
		// format: ON CONFLICT (index) DO UPDATE SET a=EXCLUDED.a, b=EXCLUDED.b
		insertSQL += fmt.Sprintf(" ON CONFLICT (%v) DO UPDATE SET ", onDupKeyIndex)

		// If this wasn't explicitly set, we want to update all columns
		if len(onDupKeyFields) == 0 {
			onDupKeyFields = cols
		}

		for i, c := range onDupKeyFields {
			if i > 0 {
				insertSQL += ","
			}
			insertSQL += fmt.Sprintf("%v=EXCLUDED.%v", c, c)
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
