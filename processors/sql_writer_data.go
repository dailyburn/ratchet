package processors

// SQLWriterData is a custom data structure you can send into a MySQLWriter
// stage or a PostreSQLWriter stage if you need to specify TableName on a
// per-data payload basis. No extra configuration is needed to use
// SQLWriterData, each data payload received is first checked for this structure
// before processing.
type SQLWriterData struct {
	TableName  string      `json:"table_name"`
	InsertData interface{} `json:"insert_data"`
}
