package dtos

// ImportMetadata contains metadata about imported data
type ImportMetadata struct {
	TableName   string                 `json:"table_name"`
	RowCount    int                    `json:"row_count"`
	ColumnCount int                    `json:"column_count"`
	Quality     float64                `json:"quality_score"`
	Issues      []string               `json:"issues,omitempty"`
	Suggestions []string               `json:"suggestions,omitempty"`
	Columns     []ImportColumnMetadata `json:"columns"`
}

// ImportColumnMetadata contains metadata about an imported column
type ImportColumnMetadata struct {
	Name         string `json:"name"`
	OriginalName string `json:"original_name"`
	DataType     string `json:"data_type"`
	NullCount    int    `json:"null_count"`
	UniqueCount  int    `json:"unique_count"`
	IsEmpty      bool   `json:"is_empty"`
	IsPrimaryKey bool   `json:"is_primary_key"`
}