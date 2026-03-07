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

export type WidgetType = 'stat' | 'line' | 'bar' | 'area' | 'pie' | 'table' | 'combo';

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
  last_refreshed_at?: string;
  generated_prompt?: string;
  created_at: string;
  updated_at: string;

  // Runtime state (not persisted, managed on client)
  data?: Record<string, unknown>[];
  is_loading?: boolean;
  error?: string;
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
}

export interface TableWidgetColumn {
  key: string;
  label: string;
  format?: 'text' | 'number' | 'date' | 'currency';
  width?: string;
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
  data: Record<string, unknown>[];
  row_count: number;
  execution_time_ms: number;
  error?: string;
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
