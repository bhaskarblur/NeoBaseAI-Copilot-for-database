import { useMemo } from 'react';
import { Plus } from 'lucide-react';
import { Dashboard, Widget } from '../../types/dashboard';
import DashboardWidgetCard from './DashboardWidgetCard';

interface DashboardWidgetGridProps {
  dashboard: Dashboard;
  onDeleteWidget: (widgetId: string) => void;
  onEditWidget: (widgetId: string) => void;
  onRefreshWidget: (widgetId: string) => void;
  onCancelWidgetRefresh?: (widgetId: string) => void;
  individuallyRefreshingWidgets?: Set<string>;
  onAddWidget: () => void;
}

/**
 * Smart row-based layout:
 * - Stat (KPI) widgets share a row equally (2 → 50/50, 3 → 33/33/33, 4+ → flex-wrap)
 * - Charts get their own row each, or pair up (2 charts → 1 row 50/50)
 * - Tables always get their own full-width row
 */
export default function DashboardWidgetGrid({
  dashboard,
  onDeleteWidget,
  onEditWidget,
  onRefreshWidget,
  onCancelWidgetRefresh,
  individuallyRefreshingWidgets,
  onAddWidget,
}: Readonly<DashboardWidgetGridProps>) {
  const renderWidgetCard = (widget: Widget) => {
    const layout = dashboard.layout.find((l) => l.widget_id === widget.id);
    const isIndividuallyRefreshing = individuallyRefreshingWidgets?.has(widget.id);
    return (
      <DashboardWidgetCard
        key={widget.id}
        widget={widget}
        layout={layout}
        onDelete={() => onDeleteWidget(widget.id)}
        onEdit={() => onEditWidget(widget.id)}
        onRefresh={() => onRefreshWidget(widget.id)}
        onCancelRefresh={isIndividuallyRefreshing && onCancelWidgetRefresh ? () => onCancelWidgetRefresh(widget.id) : undefined}
      />
    );
  };

  // Build smart rows from widget list
  const rows = useMemo(() => {
    const statWidgets: Widget[] = [];
    const pieWidgets: Widget[] = [];
    const otherChartWidgets: Widget[] = []; // line, bar, area
    const tableWidgets: Widget[] = [];

    for (const w of dashboard.widgets) {
      if (w.widget_type === 'stat') statWidgets.push(w);
      else if (w.widget_type === 'table') tableWidgets.push(w);
      else if (w.widget_type === 'pie') pieWidgets.push(w);
      else otherChartWidgets.push(w); // line, bar, area
    }

    const result: { widgets: Widget[]; type: 'stat-row' | 'chart-row' | 'table-row' }[] = [];

    // All stat widgets in one row shared equally
    if (statWidgets.length > 0) {
      result.push({ widgets: statWidgets, type: 'stat-row' });
    }

    // Smart chart layout:
    // - Pie charts NEVER get a full row alone (they center-align and waste space)
    // - Line/bar/area can take full width (they utilize it well)
    // Strategy: Pair up pies first, then handle other charts
    
    const allCharts = [...pieWidgets, ...otherChartWidgets];
    const used = new Set<string>();

    // First pass: Pair all pie charts with companions
    for (const pie of pieWidgets) {
      if (used.has(pie.id)) continue;
      
      // Find a companion for this pie (prefer another chart)
      const companion = allCharts.find(w => 
        w.id !== pie.id && 
        !used.has(w.id)
      );
      
      if (companion) {
        result.push({ widgets: [pie, companion], type: 'chart-row' });
        used.add(pie.id);
        used.add(companion.id);
      } else {
        // No companion available - pie must go alone (rare edge case)
        result.push({ widgets: [pie], type: 'chart-row' });
        used.add(pie.id);
      }
    }

    // Second pass: Handle remaining non-pie charts
    // These can take full width or pair up
    const remainingCharts = otherChartWidgets.filter(w => !used.has(w.id));
    for (let i = 0; i < remainingCharts.length; i += 2) {
      if (i + 1 < remainingCharts.length) {
        // Pair two charts
        result.push({ widgets: [remainingCharts[i], remainingCharts[i + 1]], type: 'chart-row' });
      } else {
        // Single chart gets full row (OK for line/bar/area)
        result.push({ widgets: [remainingCharts[i]], type: 'chart-row' });
      }
    }

    // Each table gets its own row
    for (const t of tableWidgets) {
      result.push({ widgets: [t], type: 'table-row' });
    }

    return result;
  }, [dashboard.widgets]);

  return (
    <div className="px-4 md:px-8 lg:px-12 mx-auto space-y-5">
      {rows.map((row, rowIdx) => {
        if (row.type === 'stat-row') {
          // Stat widgets share the row equally
          return (
            <div key={`row-${rowIdx}`} className="grid gap-5" style={{ gridTemplateColumns: `repeat(${Math.min(row.widgets.length, 4)}, 1fr)` }}>
              {row.widgets.map((w) => (
                <div key={w.id}>{renderWidgetCard(w)}</div>
              ))}
            </div>
          );
        }

        if (row.type === 'chart-row') {
          // 1 chart = full width, 2 charts = 50/50
          return (
            <div key={`row-${rowIdx}`} className={`grid gap-5 ${row.widgets.length === 2 ? 'grid-cols-1 lg:grid-cols-2' : 'grid-cols-1'}`}>
              {row.widgets.map((w) => (
                <div key={w.id}>{renderWidgetCard(w)}</div>
              ))}
            </div>
          );
        }

        // Table — full width
        return (
          <div key={`row-${rowIdx}`}>
            {row.widgets.map((w) => (
              <div key={w.id}>{renderWidgetCard(w)}</div>
            ))}
          </div>
        );
      })}

      {/* Add Widget Card */}
      {dashboard.widgets.length < 3 && (
        <div className="max-w-md mx-auto">
          <button
            onClick={onAddWidget}
            className="
              w-full flex flex-col items-center justify-center
              min-h-[140px] p-6
              bg-white/50 border-2 border-dashed border-gray-300 rounded-xl
              hover:border-black hover:bg-white/80
              transition-all duration-200 group
            "
          >
            <div className="
              w-10 h-10 rounded-full
              bg-gray-100 group-hover:bg-[#FFD700]
              border-2 border-gray-300 group-hover:border-black
              flex items-center justify-center
              transition-all duration-200 mb-2
            ">
              <Plus className="w-5 h-5 text-gray-400 group-hover:text-black transition-colors" />
            </div>
            <span className="text-sm font-bold text-gray-400 group-hover:text-black transition-colors">
              Add Widget
            </span>
          </button>
        </div>
      )}
    </div>
  );
}
