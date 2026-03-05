import {
  AlertTriangle,
  ArrowDown,
  ArrowUp,
  BarChart3,
  Info,
  MoreHorizontal,
  Pencil,
  PieChart,
  RefreshCcw,
  Table2,
  Trash2,
  TrendingUp,
} from 'lucide-react';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import ConfirmationModal from '../modals/ConfirmationModal';
import {
  AreaChart,
  Area,
  BarChart,
  Bar,
  LineChart,
  Line,
  PieChart as RechartsPieChart,
  Pie,
  Cell,
  ResponsiveContainer,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  Brush,
} from 'recharts';
import { Widget, WidgetLayout, WidgetType } from '../../types/dashboard';

interface DashboardWidgetCardProps {
  widget: Widget;
  layout?: WidgetLayout;
  onDelete: () => void;
  onEdit?: () => void;
  onRefresh?: () => void;
  onDataUpdate?: (data: Record<string, unknown>[]) => void;
  onError?: (error: string) => void;
}

// Balanced yellow / green / amber palette
const CHART_COLORS = ['#047857', '#f59e0b', '#10b981', '#d97706', '#14b8a6', '#FBBF24', '#059669', '#b45309'];

const WIDGET_TYPE_ICONS: Record<WidgetType, React.ReactNode> = {
  stat: <TrendingUp className="w-5 h-5" />,
  line: <TrendingUp className="w-5 h-5" />,
  bar: <BarChart3 className="w-5 h-5" />,
  area: <BarChart3 className="w-5 h-5" />,
  pie: <PieChart className="w-5 h-5" />,
  table: <Table2 className="w-5 h-5" />,
  combo: <BarChart3 className="w-5 h-5" />,
};

/* Grafana-style loading bar keyframes via inline style tag */
const GRAFANA_LOADING_STYLE = `
@keyframes grafana-loading-bar {
  0% { transform: translateX(-100%); }
  50% { transform: translateX(0%); }
  100% { transform: translateX(200%); }
}
`;

// Common tooltip style for all charts — white text on dark background
const TOOLTIP_STYLE = {
  contentStyle: {
    backgroundColor: '#1a1a1a',
    border: '2px solid #333',
    borderRadius: 8,
    padding: '10px 14px',
    boxShadow: '0 4px 12px rgba(0,0,0,0.3)',
  },
  labelStyle: { color: '#fff', fontWeight: 800 as const, fontSize: 14, marginBottom: 4 },
  itemStyle: { color: '#e5e7eb', fontSize: 14, padding: '2px 0' },
  cursor: { fill: 'rgba(255, 215, 0, 0.08)' },
};

// Capitalize first letter of each word for legend labels
const legendFormatter = (value: string) =>
  value.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());

// Try to parse any date-like string into short human-readable format for axes
const tryFormatDate = (value: unknown): string => {
  if (typeof value !== 'string') return String(value ?? '');
  if (/^\d{4}-\d{2}/.test(value) || /^\d{4}\/\d{2}/.test(value)) {
    try {
      const d = new Date(value);
      if (!isNaN(d.getTime())) {
        return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: '2-digit' });
      }
    } catch { /* fall through */ }
  }
  return value;
};

// Tooltip label formatter — capitalize
const tooltipLabelFormatter = (label: string | number) => {
  const s = String(label);
  return s.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
};

// Date detection & formatting helpers
const isDateString = (value: unknown): boolean =>
  typeof value === 'string' && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/.test(value);

const formatDateValue = (dateStr: string, friendly: boolean): string => {
  if (!friendly) return dateStr;
  try {
    const d = new Date(dateStr);
    if (isNaN(d.getTime())) return dateStr;
    return d.toLocaleString('en-US', {
      month: 'short', day: 'numeric', year: 'numeric',
      hour: 'numeric', minute: '2-digit', hour12: true,
    });
  } catch { return dateStr; }
};

export default function DashboardWidgetCard({
  widget,
  onDelete,
  onEdit,
  onRefresh,
}: DashboardWidgetCardProps) {
  const [showMenu, setShowMenu] = useState(false);
  const [menuPosition, setMenuPosition] = useState<{ top: number; left: number } | null>(null);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [, setTick] = useState(0);
  // Table date format: column key → true = human-readable, false = ISO
  const [dateFormats, setDateFormats] = useState<Record<string, boolean>>({});
  // Info tooltip
  const [showInfo, setShowInfo] = useState(false);

  // Ref to store previous data so we can show it while loading (Grafana-style)
  const prevDataRef = useRef<Record<string, unknown>[] | undefined>(undefined);
  if (widget.data && widget.data.length > 0) {
    prevDataRef.current = widget.data;
  }

  // Close menu on outside click
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (showMenu) {
        const target = e.target as HTMLElement;
        if (!target.closest('.widget-menu-dropdown') && !target.closest('.widget-menu-btn')) {
          setShowMenu(false);
          setMenuPosition(null);
        }
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [showMenu]);

  // Live timer — re-render every 30s to update relative time
  useEffect(() => {
    if (!widget.last_refreshed_at) return;
    const interval = setInterval(() => setTick((t) => t + 1), 30_000);
    return () => clearInterval(interval);
  }, [widget.last_refreshed_at]);

  const handleToggleMenu = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (showMenu) {
      setShowMenu(false);
      setMenuPosition(null);
      return;
    }
    const rect = e.currentTarget.getBoundingClientRect();
    setMenuPosition({
      top: rect.bottom + 8,
      left: rect.right - 208,
    });
    setShowMenu(true);
  }, [showMenu]);

  // Detect date columns for table widget
  useEffect(() => {
    if (widget.widget_type !== 'table' || !widget.data || widget.data.length === 0) return;
    const newDateCols: Record<string, boolean> = {};
    const firstRow = widget.data[0];
    for (const key of Object.keys(firstRow)) {
      if (isDateString(firstRow[key])) newDateCols[key] = true;
    }
    if (Object.keys(newDateCols).length > 0) {
      setDateFormats((prev) => {
        const merged = { ...prev };
        for (const k of Object.keys(newDateCols)) {
          if (!(k in merged)) merged[k] = true;
        }
        return merged;
      });
    }
  }, [widget.data, widget.widget_type]);

  // Use current data or previous data for Grafana-style "show stale data while loading"
  const displayData = widget.data && widget.data.length > 0 ? widget.data : prevDataRef.current;

  // Memoize chart keys & category detection from display data
  const { numericKeys, categoryKey, chartData } = useMemo(() => {
    const d = displayData || [];
    if (d.length === 0) return { numericKeys: [] as string[], categoryKey: undefined as string | undefined, chartData: d };
    const first = d[0];
    const nk = Object.keys(first).filter((k) => typeof first[k] === 'number');
    const ck = Object.keys(first).find((k) => typeof first[k] !== 'number');
    return { numericKeys: nk, categoryKey: ck, chartData: d };
  }, [displayData]);

  const renderWidgetContent = () => {
    // Error state — but still show previous data underneath if available
    if (widget.error && !displayData) {
      return (
        <div className="flex items-center justify-center h-full min-h-[120px] p-4">
          <div className="flex flex-col items-center gap-2 text-center">
            <AlertTriangle className="w-5 h-5 text-red-400" />
            <span className="text-sm text-red-500 line-clamp-2">{widget.error.charAt(0).toUpperCase() + widget.error.slice(1)}</span>
          </div>
        </div>
      );
    }

    // No data at all — never loaded
    if (!displayData || displayData.length === 0) {
      if (widget.is_loading) {
        return (
          <div className="flex items-center justify-center h-full min-h-[120px]">
            <span className="text-base text-gray-400">Loading data...</span>
          </div>
        );
      }
      return (
        <div className="flex items-center justify-center h-full min-h-[120px]">
          <span className="text-center text-base text-gray-500">No data loaded.<br />Try refreshing.</span>
        </div>
      );
    }

    // Has data (current or previous) — render chart/table normally
    switch (widget.widget_type) {
      case 'stat':
        return renderStatWidget();
      case 'line':
        return renderLineChart();
      case 'bar':
        return renderBarChart();
      case 'area':
        return renderAreaChart();
      case 'pie':
        return renderPieChart();
      case 'table':
        return renderTableWidget();
      default:
        return renderBarChart();
    }
  };

  const renderStatWidget = () => {
    const config = widget.stat_config;
    const data = displayData;
    if (!config || !data || data.length === 0) return null;

    const firstRow = data[0];
    const keys = Object.keys(firstRow);
    const mainValue = firstRow[keys[0]] as number;

    const formattedValue = formatStatValue(mainValue, config.format, config.prefix, config.suffix, config.decimal_places);

    const changePercent = config.change_percentage;
    const isPositive = changePercent !== undefined && changePercent >= 0;
    const trendIsGood = config.trend_direction === 'up_is_good' ? isPositive : !isPositive;

    return (
      <div className="flex flex-col justify-center p-1">
        <div className="text-4xl font-black text-black">{formattedValue}</div>
        {changePercent !== undefined && (
          <div className={`flex items-center gap-1 mt-2 text-base font-bold ${trendIsGood ? 'text-green-600' : 'text-red-600'}`}>
            {isPositive ? <ArrowUp className="w-4 h-4" /> : <ArrowDown className="w-4 h-4" />}
            <span>{Math.abs(changePercent).toFixed(1)}%</span>
          </div>
        )}
      </div>
    );
  };

  // Chart axis date tick formatter
  const axisTickFormatter = (value: string | number) => tryFormatDate(value);

  const renderLineChart = () => {
    const showBrush = chartData.length > 15;
    return (
      <div className="w-full outline-none focus:outline-none" style={{ height: showBrush ? 340 : 280 }} tabIndex={-1}>
        <ResponsiveContainer width="100%" height="100%">
          <LineChart data={chartData} margin={{ top: 5, right: 10, left: -15, bottom: 5 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#e5e7eb" />
            {categoryKey && <XAxis dataKey={categoryKey} tick={{ fontSize: 13, fill: '#6b7280' }} tickLine={false} axisLine={{ stroke: '#d1d5db' }} tickFormatter={axisTickFormatter} />}
            <YAxis tick={{ fontSize: 13, fill: '#6b7280' }} tickLine={false} axisLine={{ stroke: '#d1d5db' }} />
            <Tooltip {...TOOLTIP_STYLE} labelFormatter={tooltipLabelFormatter} formatter={(value: number | string, name: string) => [value, legendFormatter(name)]} />
            <Legend wrapperStyle={{ fontSize: 13, paddingTop: 6 }} formatter={legendFormatter} />
            {numericKeys.map((key, i) => (
              <Line key={key} type="monotone" dataKey={key} name={key} stroke={CHART_COLORS[i % CHART_COLORS.length]} strokeWidth={2.5} dot={false} activeDot={{ r: 5, strokeWidth: 2, stroke: '#000' }} />
            ))}
            {showBrush && <Brush dataKey={categoryKey} height={28} stroke="#047857" fill="#f9fafb" travellerWidth={10} tickFormatter={() => ''} startIndex={Math.max(0, chartData.length - 20)} />}
          </LineChart>
        </ResponsiveContainer>
      </div>
    );
  };

  const renderBarChart = () => {
    const showBrush = chartData.length > 12;
    return (
      <div className="w-full outline-none focus:outline-none" style={{ height: showBrush ? 340 : 280 }} tabIndex={-1}>
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={chartData} margin={{ top: 5, right: 10, left: 0, bottom: 5 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#e5e7eb" />
            {categoryKey && <XAxis dataKey={categoryKey} tick={{ fontSize: 13, fill: '#6b7280' }} tickLine={false} axisLine={{ stroke: '#d1d5db' }} tickFormatter={axisTickFormatter} />}
            <YAxis tick={{ fontSize: 13, fill: '#6b7280' }} tickLine={false} axisLine={{ stroke: '#d1d5db' }} />
            <Tooltip {...TOOLTIP_STYLE} labelFormatter={tooltipLabelFormatter} formatter={(value: number | string, name: string) => [value, legendFormatter(name)]} />
            <Legend wrapperStyle={{ fontSize: 14, paddingTop: 6 }} formatter={legendFormatter} />
            {numericKeys.map((key, i) => (
              <Bar key={key} dataKey={key} name={key} fill={CHART_COLORS[i % CHART_COLORS.length]} radius={[4, 4, 0, 0]} />
            ))}
            {showBrush && <Brush dataKey={categoryKey} height={28} stroke="#047857" fill="#f9fafb" travellerWidth={10} tickFormatter={() => ''} startIndex={Math.max(0, chartData.length - 15)} />}
          </BarChart>
        </ResponsiveContainer>
      </div>
    );
  };

  const renderAreaChart = () => {
    const showBrush = chartData.length > 15;
    return (
      <div className="w-full outline-none focus:outline-none" style={{ height: showBrush ? 340 : 280 }} tabIndex={-1}>
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={chartData} margin={{ top: 5, right: 10, left: 0, bottom: 5 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#e5e7eb" />
            {categoryKey && <XAxis dataKey={categoryKey} tick={{ fontSize: 13, fill: '#6b7280' }} tickLine={false} axisLine={{ stroke: '#d1d5db' }} tickFormatter={axisTickFormatter} />}
            <YAxis tick={{ fontSize: 13, fill: '#6b7280' }} tickLine={false} axisLine={{ stroke: '#d1d5db' }} />
            <Tooltip {...TOOLTIP_STYLE} labelFormatter={tooltipLabelFormatter} formatter={(value: number | string, name: string) => [value, legendFormatter(name)]} />
            <Legend wrapperStyle={{ fontSize: 14, paddingTop: 6 }} formatter={legendFormatter} />
            {numericKeys.map((key, i) => (
              <Area key={key} type="monotone" dataKey={key} name={key} stroke={CHART_COLORS[i % CHART_COLORS.length]} fill={CHART_COLORS[i % CHART_COLORS.length]} fillOpacity={0.15} strokeWidth={2.5} activeDot={{ r: 5, strokeWidth: 2, stroke: '#000' }} />
            ))}
            {showBrush && <Brush dataKey={categoryKey} height={28} stroke="#047857" fill="#f9fafb" travellerWidth={10} tickFormatter={() => ''} startIndex={Math.max(0, chartData.length - 20)} />}
          </AreaChart>
        </ResponsiveContainer>
      </div>
    );
  };

  const renderPieChart = () => {
    const data = displayData || [];
    const nameKey = data.length > 0 ? Object.keys(data[0]).find((k) => typeof data[0][k] !== 'number') : 'name';
    const valueKey = data.length > 0 ? Object.keys(data[0]).find((k) => typeof data[0][k] === 'number') : 'value';

    return (
      <div className={`${data.length >= 6 ? 'h-[280px]' : data.length >= 3 ? 'h-[240px]' : 'h-[200px]'} w-full outline-none focus:outline-none`} tabIndex={-1}>
        <ResponsiveContainer width="100%" height="100%">
          <RechartsPieChart>
            <Pie
              data={data}
              dataKey={valueKey || 'value'}
              nameKey={nameKey || 'name'}
              cx="50%"
              cy="48%"
              outerRadius={75}
              innerRadius={38}
              strokeWidth={2}
              stroke="#000"
            >
              {data.map((_entry, index) => (
                <Cell key={`cell-${index}`} fill={CHART_COLORS[index % CHART_COLORS.length]} />
              ))}
            </Pie>
            <Tooltip
              contentStyle={TOOLTIP_STYLE.contentStyle}
              labelStyle={TOOLTIP_STYLE.labelStyle}
              itemStyle={TOOLTIP_STYLE.itemStyle}
              formatter={(value: number | string, name: string) => [value, legendFormatter(name)]}
            />
            <Legend wrapperStyle={{ fontSize: 14, paddingTop: 8 }} formatter={legendFormatter} />
          </RechartsPieChart>
        </ResponsiveContainer>
      </div>
    );
  };

  const renderTableWidget = () => {
    const data = displayData || [];
    if (data.length === 0) return null;

    const columns = widget.table_config?.columns;
    const columnKeys = columns
      ? columns.map((c) => c.key)
      : Object.keys(data[0]).slice(0, 10);

    const columnLabels = columns
      ? columns.reduce((acc, c) => ({ ...acc, [c.key]: c.label }), {} as Record<string, string>)
      : columnKeys.reduce((acc, k) => ({ ...acc, [k]: k.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase()) }), {} as Record<string, string>);

    const pageSize = widget.table_config?.page_size || 25;
    const displayRows = data.slice(0, pageSize);

    // Detect date columns
    const dateColumnSet = new Set<string>();
    for (const key of columnKeys) {
      if (data.some((row) => isDateString(row[key]))) dateColumnSet.add(key);
    }

    const formatCell = (value: unknown, column: string): string => {
      if (value === null || value === undefined) return '—';
      if (dateColumnSet.has(column) && typeof value === 'string') {
        return formatDateValue(value, dateFormats[column] ?? true);
      }
      if (typeof value === 'object') return JSON.stringify(value);
      return String(value);
    };

    // Dynamic min-width per column based on column count
    const colWidth = columnKeys.length <= 3 ? 'min-w-[180px]' : columnKeys.length <= 5 ? 'min-w-[140px]' : 'min-w-[120px]';

    return (
      <div className="overflow-auto max-h-[420px]">
        <table className="w-full text-base leading-relaxed border-collapse min-w-max">
          <thead className="sticky top-0 z-10 bg-white">
            <tr className="border-b-2 border-black/10">
              {columnKeys.map((key) => (
                <th key={key} className={`text-left py-2.5 px-3 font-bold text-black whitespace-nowrap ${colWidth}`}>
                  <div className="flex items-center gap-1">
                    <span>{columnLabels[key] || key}</span>
                    {dateColumnSet.has(key) && (
                      <button
                        onClick={(ev) => { ev.stopPropagation(); setDateFormats((prev) => ({ ...prev, [key]: !prev[key] })); }}
                        className="inline-flex text-[10px] px-1.5 py-0.5 ml-1 bg-gray-200 hover:bg-gray-300 rounded text-gray-600 font-medium transition-colors focus:outline-none"
                        title="Toggle date format"
                      >
                        {dateFormats[key] !== false ? 'ISO' : 'Human'}
                      </button>
                    )}
                  </div>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {displayRows.map((row, i) => (
              <tr key={i} className="border-b border-gray-100 hover:bg-[#FFDB58]/20 transition-colors">
                {columnKeys.map((key) => (
                  <td key={key} className={`py-2 px-3 text-gray-700 whitespace-nowrap max-w-[480px] truncate ${colWidth}`} title={String(row[key] ?? '')}>
                    {formatCell(row[key], key)}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
        {data.length > pageSize && (
          <div className="sticky bottom-0 text-center py-2 text-xs text-gray-400 bg-white border-t border-gray-100">
            Showing {pageSize} of {data.length} rows
          </div>
        )}
      </div>
    );
  };

  return (
    <>
      <style>{GRAFANA_LOADING_STYLE}</style>
      <div
        className={`
          bg-white border-2 border-black rounded-xl
          shadow-[3px_3px_0px_0px_rgba(0,0,0,1)]
          overflow-hidden relative
          transition-shadow duration-200
          hover:shadow-[4px_4px_0px_0px_rgba(0,0,0,1)]
          outline-none
          ${widget.is_loading ? 'opacity-90' : ''}
        `}
        tabIndex={-1}
      >
        {/* Widget Header */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200">
          <div className="flex items-center gap-2 min-w-0">
            <span className="text-gray-500 flex-shrink-0">
              {WIDGET_TYPE_ICONS[widget.widget_type]}
            </span>
            <h3 className="text-base font-bold text-black truncate">{widget.title}</h3>
            {widget.description && (
              <div className="relative flex-shrink-0">
                <button
                  onMouseEnter={() => setShowInfo(true)}
                  onMouseLeave={() => setShowInfo(false)}
                  className="p-0.5 text-gray-400 hover:text-gray-600 transition-colors"
                >
                  <Info className="w-4 h-4 mt-0.5 -ml-0.5" />
                </button>
                {showInfo && (
                  <div className="fixed z-[200] w-56 bg-black text-white text-sm rounded-lg px-3 py-2 shadow-lg leading-relaxed pointer-events-none"
                    style={{
                      // Position relative to viewport, above the button
                      top: 'auto',
                      left: 'auto',
                    }}
                    ref={(el) => {
                      if (el) {
                        const parent = el.parentElement?.querySelector('button');
                        if (parent) {
                          const rect = parent.getBoundingClientRect();
                          el.style.top = `${rect.top - el.offsetHeight - 8}px`;
                          el.style.left = `${rect.left + rect.width / 2 - el.offsetWidth / 2}px`;
                        }
                      }
                    }}
                  >
                    {widget.description}
                    <div className="absolute top-full left-1/2 -translate-x-1/2 border-[5px] border-transparent border-t-black" />
                  </div>
                )}
              </div>
            )}
          </div>

          <div className="flex items-center gap-1 flex-shrink-0">
            {onRefresh && (
              <button
                onClick={onRefresh}
                disabled={widget.is_loading}
                className="p-1.5 rounded-lg hover:bg-gray-100 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                title="Refresh widget data"
              >
                <RefreshCcw className={`w-5 h-5 text-gray-500 ${widget.is_loading ? 'animate-spin' : ''}`} />
              </button>
            )}
            <button
              onClick={handleToggleMenu}
              className="widget-menu-btn p-1.5 rounded-lg hover:bg-gray-100 transition-colors"
            >
              <MoreHorizontal className="w-5 h-5 text-gray-500" />
            </button>
          </div>
        </div>

        {/* Widget Content */}
        <div className={`px-4 py-3 ${widget.widget_type === 'table' ? 'px-0 py-0' : ''}`}>
          {renderWidgetContent()}
          {/* Error overlay on top of stale data */}
          {widget.error && displayData && displayData.length > 0 && (
            <div className="flex items-center gap-1.5 px-3 py-1.5 bg-red-50 border-t border-red-200 text-xs text-red-500">
              <AlertTriangle className="w-3.5 h-3.5 flex-shrink-0" />
              <span className="truncate">{widget.error}</span>
            </div>
          )}
        </div>

        {/* Footer: Last refreshed */}
        {widget.last_refreshed_at && (
          <div className="px-4 py-2.5 border-t border-gray-200 flex items-center gap-1.5">
            <span className="text-xs text-gray-400">
              Last refreshed: <span className="text-gray-600">{formatRelativeTime(widget.last_refreshed_at)}</span>
            </span>
          </div>
        )}

        {/* Loading bar at BOTTOM of card */}
        {widget.is_loading && (
          <div className="absolute bottom-0 left-0 right-0 h-[4px] overflow-hidden z-10">
            <div
              className="h-full w-2/3 bg-gradient-to-r from-transparent via-emerald-500 to-emerald-200"
              style={{ animation: 'grafana-loading-bar 2s ease-in-out infinite' }}
            />
          </div>
        )}
      </div>

      {/* Widget menu dropdown — fixed positioned, matching ChatHeader style */}
      {showMenu && menuPosition && (
        <div
          className="widget-menu-dropdown fixed w-52 bg-white border-4 border-black rounded-lg shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] z-[100]"
          style={{ top: `${menuPosition.top}px`, left: `${menuPosition.left}px` }}
          onClick={(e) => e.stopPropagation()}
        >
          <div className="py-1">
            {onEdit && (
              <>
                <button
                  onClick={() => {
                    setShowMenu(false);
                    setMenuPosition(null);
                    onEdit();
                  }}
                  className="flex items-center w-full text-left px-4 py-2 text-sm font-semibold text-black hover:bg-gray-200 transition-colors"
                >
                  <Pencil className="w-4 h-4 mr-2 text-black" />
                  Edit Widget
                </button>
                <div className="h-px bg-gray-200 mx-2" />
              </>
            )}
            <button
              onClick={() => {
                setShowMenu(false);
                setMenuPosition(null);
                setShowDeleteConfirm(true);
              }}
              className="flex items-center w-full text-left px-4 py-2 text-sm font-semibold text-red-500 hover:bg-neo-error hover:text-white transition-colors"
            >
              <Trash2 className="w-4 h-4 mr-2" />
              Remove Widget
            </button>
          </div>
        </div>
      )}

      {/* Delete Confirmation Modal — uses ConfirmationModal */}
      {showDeleteConfirm && (
        <ConfirmationModal
          icon={<Trash2 className="w-6 h-6 text-neo-error" />}
          title="Remove Widget"
          message={`Are you sure you want to remove "${widget.title}"? This action cannot be undone.`}
          buttonText="Remove"
          onConfirm={async () => {
            setShowDeleteConfirm(false);
            onDelete();
          }}
          onCancel={() => setShowDeleteConfirm(false)}
          zIndex="z-[120]"
        />
      )}
    </>
  );
}

// Helper: format stat values
function formatStatValue(
  value: number,
  format?: string,
  prefix?: string,
  suffix?: string,
  decimalPlaces?: number
): string {
  const dp = decimalPlaces ?? 0;
  let formatted: string;

  switch (format) {
    case 'currency':
      formatted = value.toLocaleString(undefined, { minimumFractionDigits: dp, maximumFractionDigits: dp });
      return `${prefix || '$'}${formatted}${suffix || ''}`;
    case 'percentage':
      formatted = value.toFixed(dp);
      return `${prefix || ''}${formatted}%${suffix || ''}`;
    case 'duration':
      if (value >= 3600) return `${(value / 3600).toFixed(1)}h`;
      if (value >= 60) return `${(value / 60).toFixed(1)}m`;
      return `${value.toFixed(0)}s`;
    default:
      // Smart number formatting
      if (value >= 1_000_000) formatted = `${(value / 1_000_000).toFixed(1)}M`;
      else if (value >= 1_000) formatted = `${(value / 1_000).toFixed(1)}K`;
      else formatted = value.toLocaleString(undefined, { minimumFractionDigits: dp, maximumFractionDigits: dp });
      return `${prefix || ''}${formatted}${suffix || ''}`;
  }
}

// Helper: relative time
function formatRelativeTime(dateString: string): string {
  const now = Date.now();
  const then = new Date(dateString).getTime();
  const diff = Math.floor((now - then) / 1000);

  if (diff < 10) return 'Just now';
  if (diff < 60) return `${diff}s ago`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return `${Math.floor(diff / 86400)}d ago`;
}
