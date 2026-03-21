// Dashboard types matching backend DTOs

// === Core Dashboard Types ===

export interface Dashboard {
  id: string;
  chat_id: string;
  name: string;
  description?: string;
  template_type?: string;
  is_default: boolean;
  refresh_interval: number; // seconds: 0 = manual, 15, 30, 60, 300, 600, 3600
  time_range: string;       // "1h", "6h", "24h", "7d", "30d"
  layout: WidgetLayout[];
  widgets: Widget[];
  created_at: string;
  updated_at: string;
}

export interface DashboardListItem {
  id: string;
  name: string;
  description?: string;
  template_type?: string;
  is_default: boolean;
  widget_count: number;
  created_at: string;
  updated_at: string;
}

export interface WidgetLayout {
  widget_id: string;
  x: number;
  y: number;
  w: number;
  h: number;
  min_w?: number;
  min_h?: number;
}

// === Widget Types ===

export type WidgetType = 'stat' | 'line' | 'bar' | 'area' | 'pie' | 'table' | 'combo' | 'gauge' | 'bar_gauge' | 'heatmap' | 'histogram';

export interface Widget {
  id: string;
  dashboard_id: string;
  title: string;
  description?: string;
  widget_type: WidgetType;
  query: string;
  query_type?: string;
  tables?: string;
  chart_config_json?: string;
  stat_config?: StatWidgetConfig;
  table_config?: TableWidgetConfig;
  gauge_config?: GaugeWidgetConfig;
  bar_gauge_config?: BarGaugeWidgetConfig;
  heatmap_config?: HeatmapWidgetConfig;
  histogram_config?: HistogramWidgetConfig;
  last_refreshed_at?: string;
  generated_prompt?: string;
  created_at: string;
  updated_at: string;

  // Runtime state (not persisted, managed on client)
  data?: Record<string, unknown>[];
  is_loading?: boolean;
  error?: string;
  
  // Cursor-based pagination state (client-side only)
  current_page?: number;
  next_cursor?: string | null;
  has_more?: boolean;
  cursor_field?: string;
  // Cursor stack for backward navigation: each entry is the cursor used to load that page
  // cursor_stack[0] = null (initial page), cursor_stack[1] = cursor for page 2, etc.
  cursor_stack?: (string | null)[];
  // Per-page data cache for instant bi-directional navigation (keyed by page number, 1-based)
  page_data_cache?: Record<number, { data: Record<string, unknown>[]; next_cursor: string | null; has_more: boolean }>;
}

export interface StatWidgetConfig {
  value_query: string;
  comparison_query?: string;
  format?: 'number' | 'currency' | 'percentage' | 'duration';
  prefix?: string;
  suffix?: string;
  decimal_places?: number;
  trend_direction?: 'up_is_good' | 'down_is_good';
  // Runtime computed values
  current_value?: number;
  comparison_value?: number;
  change_percentage?: number;
}

export interface TableWidgetConfig {
  columns: TableWidgetColumn[];
  sort_by?: string;
  sort_direction?: 'asc' | 'desc';
  page_size?: number;
  cursor_field?: string;  // Field used for cursor-based pagination (e.g., "id", "created_at")
}

export interface TableWidgetColumn {
  key: string;
  label: string;
  format?: 'text' | 'number' | 'date' | 'currency';
  width?: string;
}

export interface GaugeWidgetConfig {
  min: number;              // Minimum value (default: 0)
  max: number;              // Maximum value (default: 100)
  thresholds?: Threshold[]; // Color thresholds
  show_threshold?: boolean; // Show threshold markers
  decimal_places?: number;  // Value precision
  unit?: string;            // '%', 'ms', 'req/s', etc.
}

export interface BarGaugeWidgetConfig {
  min: number;              // Minimum value
  max: number;              // Maximum value
  thresholds?: Threshold[]; // Color thresholds
  orientation: 'horizontal' | 'vertical';
  display_mode: 'basic' | 'lcd' | 'gradient';
  show_unfilled?: boolean;  // Show unfilled portion
  decimal_places?: number;
  unit?: string;
}

export interface HeatmapWidgetConfig {
  x_axis_column: string;    // Time or category column
  y_axis_column: string;    // Category column
  value_column: string;     // Metric/value column
  color_scheme: 'green-red' | 'blue-yellow' | 'grayscale';
  show_values?: boolean;    // Display values in cells
  show_legend?: boolean;    // Show color scale legend
  bucket_size?: string;     // '1h', '1d' for time-based
}

export interface HistogramWidgetConfig {
  value_column: string;     // Column to create histogram from
  bucket_count: number;     // Number of bins/buckets
  bucket_size?: number;     // Fixed bucket size (alternative to count)
  show_mean?: boolean;      // Show mean line
  show_median?: boolean;    // Show median line
  decimal_places?: number;
}

export interface Threshold {
  value: number;  // Threshold value
  color: string;  // Color when value exceeds threshold (hex or name)
}

// === Blueprint Types (Recommendation Flow) ===

export interface DashboardBlueprint {
  index: number;
  name: string;
  description: string;
  template_type: string;
  proposed_widgets: BlueprintWidgetPreview[];
}

export interface BlueprintWidgetPreview {
  title: string;
  widget_type: WidgetType;
}

// === Request Types ===

export interface CreateDashboardRequest {
  prompt: string;
}

export interface UpdateDashboardRequest {
  name?: string;
  description?: string;
  refresh_interval?: number;
  time_range?: string;
  layout?: WidgetLayout[];
  is_default?: boolean;
}

export interface RegenerateDashboardRequest {
  reason: 'try_another_variant' | 'schema_changed';
  custom_instructions?: string;
}

export interface AddWidgetRequest {
  prompt: string;
}

export interface EditWidgetRequest {
  prompt: string;
}

export interface CreateFromBlueprintsRequest {
  blueprint_indices: number[];
}

// === SSE Event Data Types ===

export interface DashboardBlueprintEvent {
  blueprints: DashboardBlueprint[];
}

export interface DashboardGenerationProgressEvent {
  dashboard_id?: string;
  status: 'generating' | 'testing_queries' | 'finalizing';
  message: string;
  progress: number; // 0-100
}

export interface DashboardGenerationCompleteEvent {
  dashboard_id: string;
  dashboard: Dashboard;
}

export interface DashboardWidgetDataEvent {
  widget_id: string;
  // null when the event is an error (backend sends nil slice → JSON null)
  data: Record<string, unknown>[] | null;
  row_count: number;
  execution_time_ms: number;
  error?: string;
  // next_cursor has omitempty on the backend (pointer type), so absent when not paginating
  next_cursor?: string | null;
  // has_more has no omitempty — always present; false on error / non-paginated events
  has_more: boolean;
}

// === Dashboard View State ===

export type DashboardViewMode = 'chat' | 'dashboard';

export const REFRESH_INTERVAL_OPTIONS = [
  { label: 'Off', value: 0 },
  { label: '15 sec', value: 15 },
  { label: '30 sec', value: 30 },
  { label: '1 min', value: 60 },
  { label: '5 min', value: 300 },
  { label: '10 min', value: 600 },
  { label: '1 hour', value: 3600 },
] as const;

export const TIME_RANGE_OPTIONS = [
  { label: '1 hour', value: '1h' },
  { label: '6 hours', value: '6h' },
  { label: '24 hours', value: '24h' },
  { label: '7 days', value: '7d' },
  { label: '30 days', value: '30d' },
] as const;

// SSE event type constants (matching backend)
export const DASHBOARD_SSE_EVENTS = {
  BLUEPRINTS: 'dashboard-blueprints',
  GENERATION_PROGRESS: 'dashboard-generation-progress',
  GENERATION_COMPLETE: 'dashboard-generation-complete',
  WIDGET_DATA: 'dashboard-widget-data',
  WIDGET_ERROR: 'dashboard-widget-error',
} as const;

// === Import/Export ===

export interface ValidateImportRequest {
  json: string;
}

export interface ValidateImportResponse {
  valid: boolean;
  errors?: string[];
  warnings?: string[];
  requiredConnections?: ConnectionRequired[];
}

export interface ConnectionRequired {
  name: string;
  type: string;
  usedBy: string[]; // Widget titles
  suggestions?: string[]; // Connection IDs or names
}

export interface ImportDashboardRequest {
  json: string;
  mappings: Record<string, string>; // source name -> target connection ID
  options: ImportOptions;
}

export interface ImportOptions {
  skipInvalidWidgets: boolean;
  autoCreateConnections: boolean;
}

export interface ImportDashboardResponse {
  dashboardId: string;
  summary: ImportSummary;
}

export interface ImportSummary {
  widgetsImported: number;
  widgetsSkipped: number;
  warnings?: string[];
  connectionsUsed: Record<string, string>; // source name -> target name
}
