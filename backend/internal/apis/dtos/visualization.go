package dtos

// GenerateVisualizationRequest is the request body for visualization endpoint
// POST /api/chats/:id/messages/:messageId/visualizations
// Frontend only sends query_id - backend fetches everything else from database
type GenerateVisualizationRequest struct {
	QueryID string `json:"query_id" binding:"required"` // ID of the query to generate visualization for
}

// VisualizationResponse is the response from AI for chart generation
type VisualizationResponse struct {
	VisualizationID    string              `json:"visualization_id,omitempty"` // ID of the saved visualization in database
	CanVisualize       bool                `json:"can_visualize"`
	Reason             string              `json:"reason,omitempty"`
	ChartConfiguration *ChartConfiguration `json:"chart_configuration,omitempty"`
	ChartData          interface{}         `json:"chart_data,omitempty"`     // Actual data rows for the chart
	TotalRecords       interface{}         `json:"total_records,omitempty"`  // Total record count
	ReturnedCount      interface{}         `json:"returned_count,omitempty"` // Rows returned in this response
	HasMore            interface{}         `json:"has_more,omitempty"`       // Whether more data is available
	UpdatedAt          string              `json:"updated_at,omitempty"`     // When the visualization was generated
	Error              string              `json:"error,omitempty"`
}

// ChartConfiguration contains all the info needed to render a chart
type ChartConfiguration struct {
	ChartType   string `json:"chart_type"` // "line" | "bar" | "area" | "scatter" | "pie" | "combo"
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`

	DataFetch      ChartDataFetch    `json:"data_fetch"`
	ChartRender    ChartRenderConfig `json:"chart_render"`
	RenderingHints RenderingHints    `json:"rendering_hints"`
}

// ChartDataFetch defines how to fetch and transform data for the chart
type ChartDataFetch struct {
	QueryStrategy         string `json:"query_strategy"`           // "original_query" | "aggregated_query" | "sampled_query"
	OptimizedQuery        string `json:"optimized_query"`          // The optimized SQL query
	Limit                 int    `json:"limit,omitempty"`          // Max rows to fetch
	SampleEveryN          int    `json:"sample_every_n,omitempty"` // If sampling, take every Nth row
	ProjectedRows         int    `json:"projected_rows"`           // Estimated output rows
	Transformation        string `json:"transformation,omitempty"` // "none" | "aggregate" | "interpolate" | "top_n"
	TransformationDetails string `json:"transformation_details,omitempty"`
}

// ChartRenderConfig defines how to render the chart
type ChartRenderConfig struct {
	Type     string         `json:"type"` // "line" | "bar" | "area" | "scatter" | "pie" | "combo"
	XAxis    AxisConfig     `json:"x_axis"`
	YAxis    *AxisConfig    `json:"y_axis,omitempty"` // Optional for pie charts
	Series   []SeriesConfig `json:"series,omitempty"`
	Pie      *PieConfig     `json:"pie,omitempty"` // For pie charts
	Colors   []string       `json:"colors"`
	Features ChartFeatures  `json:"features"`
}

// AxisConfig defines X and Y axis properties
type AxisConfig struct {
	DataKey string `json:"data_key"`         // Column name from query result
	Label   string `json:"label"`            // User-friendly label
	Type    string `json:"type"`             // "date" | "category" | "number"
	Format  string `json:"format,omitempty"` // "MMM DD" | "HH:mm" etc
}

// SeriesConfig defines data series for multi-line/multi-bar charts
type SeriesConfig struct {
	DataKey string `json:"data_key"`         // Column name from query result
	Name    string `json:"name"`             // Display name
	Type    string `json:"type,omitempty"`   // "monotone" | "natural" | "stepAfter"
	Stroke  string `json:"stroke,omitempty"` // Hex color
	Fill    string `json:"fill,omitempty"`   // Hex color
	Area    bool   `json:"area,omitempty"`   // For area charts
}

// PieConfig defines pie/donut chart properties
type PieConfig struct {
	DataKey     string `json:"data_key"`               // Value column
	NameKey     string `json:"name_key"`               // Label column
	InnerRadius int    `json:"inner_radius,omitempty"` // For donut chart
}

// ChartFeatures toggles chart interactive features
type ChartFeatures struct {
	Tooltip     bool `json:"tooltip"`
	Legend      bool `json:"legend"`
	Grid        bool `json:"grid"`
	Responsive  bool `json:"responsive"`
	ZoomEnabled bool `json:"zoom_enabled"`
}

// RenderingHints provides UI rendering recommendations
type RenderingHints struct {
	ChartHeight           int    `json:"chart_height"`
	ChartWidth            string `json:"chart_width,omitempty"`
	ColorScheme           string `json:"color_scheme"` // "neobase_primary" | "neobase_rainbow"
	ShouldAggregateBeyond int    `json:"should_aggregate_beyond"`
	ProjectedRowCount     int    `json:"projected_row_count"`
	DataDensity           string `json:"data_density"` // "sparse" | "moderate" | "dense"
}

// ExecuteChartQueryRequest is used to execute the optimized chart query
type ExecuteChartQueryRequest struct {
	ChatID         string `json:"chat_id" binding:"required"`
	ConnectionID   string `json:"connection_id" binding:"required"`
	OptimizedQuery string `json:"optimized_query" binding:"required"`
	Limit          int    `json:"limit,omitempty"`
}

// ExecuteChartQueryResponse returns the actual chart data
type ExecuteChartQueryResponse struct {
	Data            []map[string]interface{} `json:"data"`
	RowCount        int                      `json:"row_count"`
	ExecutionTimeMs float64                  `json:"execution_time_ms"`
	IsSampled       bool                     `json:"is_sampled,omitempty"`
	SamplingMethod  string                   `json:"sampling_method,omitempty"`
	Error           string                   `json:"error,omitempty"`
}
