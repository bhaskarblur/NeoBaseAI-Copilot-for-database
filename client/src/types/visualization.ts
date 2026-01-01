// Visualization types matching backend DTOs

export interface AxisConfig {
  data_key: string;
  label: string;
  type: 'date' | 'category' | 'number';
  format?: string;
}

export interface SeriesConfig {
  data_key: string;
  name: string;
  type?: 'monotone' | 'natural' | 'stepAfter';
  stroke?: string;
  fill?: string;
  area?: boolean;
}

export interface PieConfig {
  data_key: string;
  name_key: string;
  inner_radius?: number;
}

export interface HeatmapConfig {
  x_key: string;
  y_key: string;
  value_key: string;
  colors?: string[];
}

export interface FunnelConfig {
  stage_key: string;
  value_key: string;
  colors?: string[];
}

export interface BubbleConfig {
  x_key: string;
  y_key: string;
  size_key: string;
  category_key?: string;
  colors?: string[];
}

export interface WaterfallConfig {
  category_key: string;
  value_key: string;
  colors?: {
    increase?: string;
    decrease?: string;
    total?: string;
  };
}

export interface ChartDataFetch {
  query_strategy: 'original_query' | 'aggregated_query' | 'sampled_query';
  optimized_query: string;
  limit?: number;
  sample_every_n?: number;
  projected_rows: number;
  transformation?: 'none' | 'aggregate' | 'interpolate' | 'top_n';
  transformation_details?: string;
}

export interface ChartFeatures {
  tooltip: boolean;
  legend: boolean;
  grid: boolean;
  responsive: boolean;
  zoom_enabled: boolean;
}

export interface ChartRenderConfig {
  type: 'line' | 'bar' | 'pie' | 'area' | 'scatter' | 'combo' | 'heatmap' | 'funnel' | 'bubble' | 'waterfall';
  x_axis: AxisConfig;
  y_axis?: AxisConfig;
  series?: SeriesConfig[];
  pie?: PieConfig;
  heatmap?: HeatmapConfig;
  funnel?: FunnelConfig;
  bubble?: BubbleConfig;
  waterfall?: WaterfallConfig;
  colors: string[];
  features: ChartFeatures;
}

export interface RenderingHints {
  chart_height: number;
  chart_width?: string;
  color_scheme: string;
  should_aggregate_beyond: number;
  projected_row_count: number;
  data_density: 'sparse' | 'moderate' | 'dense';
}

export interface ChartConfiguration {
  chart_type: 'line' | 'bar' | 'pie' | 'area' | 'scatter' | 'combo' | 'heatmap' | 'funnel' | 'bubble' | 'waterfall';
  title: string;
  description?: string;
  data_fetch: ChartDataFetch;
  chart_render: ChartRenderConfig;
  rendering_hints: RenderingHints;
}

export interface VisualizationResponse {
  can_visualize: boolean;
  reason?: string;
  chart_configuration?: ChartConfiguration;
  chart_data?: Record<string, any>[];
  error?: string;
}

export interface GenerateVisualizationRequest {
  user_query: string;
  executed_query: string;
  query_result_sample: Record<string, any>[];
  total_row_count: number;
  connection_type: string;
  database_schema?: any;
  column_count?: number;
  column_types?: Record<string, string>;
  column_names?: string[];
  preferred_chart_type?: string;
  preferred_granularity?: string;
}

export interface ChartDataResponse {
  data: Record<string, any>[];
  row_count: number;
  execution_time_ms: number;
  is_sampled?: boolean;
  sampling_method?: string;
  error?: string;
}
