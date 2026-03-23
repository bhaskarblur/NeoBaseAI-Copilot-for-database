import {
    AlertTriangle,
    Info,
    MoreHorizontal,
    Pencil,
    RefreshCcw,
    Trash2,
    X,
} from 'lucide-react';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import ConfirmationModal from '../modals/ConfirmationModal';
import { Widget, WidgetLayout, WidgetType } from '../../types/dashboard';
import AreaChartWidget from './widgets/AreaChartWidget';
import BarChartWidget from './widgets/BarChartWidget';
import BarGaugeWidget from './widgets/BarGaugeWidget';
import GaugeWidget from './widgets/GaugeWidget';
import HeatmapWidget from './widgets/HeatmapWidget';
import HistogramWidget from './widgets/HistogramWidget';
import LineChartWidget from './widgets/LineChartWidget';
import PieChartWidget from './widgets/PieChartWidget';
import StatWidget from './widgets/StatWidget';
import TableWidget from './widgets/TableWidget';
import {
    GRAFANA_LOADING_STYLE,
    WIDGET_TYPE_ICONS,
    formatRelativeTime,
} from './widgets/widgetConstants';

interface DashboardWidgetCardProps {
    widget: Widget;
    layout?: WidgetLayout;
    onDelete: () => void;
    onEdit?: () => void;
    onRefresh?: () => void;
    onCancelRefresh?: () => void;
    onNextPage?: () => void;
    onPreviousPage?: () => void;
    onDataUpdate?: (data: Record<string, unknown>[]) => void;
    onError?: (error: string) => void;
}

export default function DashboardWidgetCard({
    widget,
    onDelete,
    onEdit,
    onRefresh,
    onCancelRefresh,
    onNextPage,
    onPreviousPage,
}: DashboardWidgetCardProps) {
    const [showMenu, setShowMenu] = useState(false);
    const [menuPosition, setMenuPosition] = useState<{ top: number; left: number } | null>(null);
    const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
    const [, setTick] = useState(0);
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
        setMenuPosition({ top: rect.bottom + 8, left: rect.right - 208 });
        setShowMenu(true);
    }, [showMenu]);

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
                        <span className="text-base text-red-500 line-clamp-2">
                            {widget.error.charAt(0).toUpperCase() + widget.error.slice(1)}
                        </span>
                    </div>
                </div>
            );
        }

        // No data at all — never loaded
        if (!displayData || displayData.length === 0) {
            if (widget.is_loading) {
                return (
                    <div className="flex items-center justify-center h-full min-h-[100px]">
                        <span className="text-base text-gray-400">Loading data...</span>
                    </div>
                );
            }
            return (
                <div className="flex items-center justify-center h-full min-h-[100px]">
                    <span className="text-center text-base text-gray-500">No data found</span>
                </div>
            );
        }

        // Has data — delegate to the appropriate widget component
        const widgetType: WidgetType = widget.widget_type;
        switch (widgetType) {
            case 'stat':
                return <StatWidget widget={widget} data={displayData} />;
            case 'line':
                return <LineChartWidget chartData={chartData} numericKeys={numericKeys} categoryKey={categoryKey} />;
            case 'bar':
                return <BarChartWidget chartData={chartData} numericKeys={numericKeys} categoryKey={categoryKey} />;
            case 'area':
                return <AreaChartWidget chartData={chartData} numericKeys={numericKeys} categoryKey={categoryKey} />;
            case 'pie':
                return <PieChartWidget data={displayData} />;
            case 'table':
                return <TableWidget widget={widget} data={displayData} onNextPage={onNextPage} onPreviousPage={onPreviousPage} />;
            case 'gauge':
                return <GaugeWidget widget={widget} data={displayData} />;
            case 'bar_gauge':
                return <BarGaugeWidget widget={widget} data={displayData} />;
            case 'heatmap':
                return <HeatmapWidget widget={widget} data={displayData} />;
            case 'histogram':
                return <HistogramWidget widget={widget} data={displayData} />;
            default:
                return <BarChartWidget chartData={chartData} numericKeys={numericKeys} categoryKey={categoryKey} />;
        }
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
                                    className="p-0.5 text-gray-500 hover:text-gray-700 transition-colors"
                                >
                                    <Info className="w-4 h-4 mt-0.5 -ml-0.5" />
                                </button>
                                {showInfo && (
                                    <div
                                        className="fixed z-[200] w-56 bg-black text-white text-sm rounded-lg px-3 py-2 shadow-lg leading-relaxed pointer-events-none"
                                        style={{ top: 'auto', left: 'auto' }}
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
                        {onRefresh && !widget.is_loading && (
                            <button
                                onClick={onRefresh}
                                className="p-2 rounded-lg hover:bg-gray-200 transition-colors"
                                title="Refresh widget data"
                            >
                                <RefreshCcw className="w-4 h-4 text-gray-500" />
                            </button>
                        )}
                        {widget.is_loading && onCancelRefresh && (
                            <button
                                onClick={onCancelRefresh}
                                className="p-2 rounded-lg hover:bg-red-100 transition-colors"
                                title="Cancel loading"
                            >
                                <X className="w-4 h-4 text-red-500" />
                            </button>
                        )}
                        {widget.is_loading && !onCancelRefresh && onRefresh && (
                            <button
                                disabled
                                className="p-2 rounded-lg opacity-50 cursor-not-allowed hover:bg-gray-200 transition-colors"
                                title="Loading..."
                            >
                                <RefreshCcw className="w-4 h-4 text-gray-500 animate-spin" />
                            </button>
                        )}
                        <button
                            onClick={handleToggleMenu}
                            className="widget-menu-btn p-1.5 rounded-lg hover:bg-gray-200 transition-colors"
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
                        <div className="flex items-center gap-1.5 px-3 py-2 bg-red-50 border-t border-red-200 text-sm text-red-500">
                            <AlertTriangle className="w-4 h-4 flex-shrink-0" />
                            <span className="truncate">{widget.error}</span>
                        </div>
                    )}
                </div>

                {/* Footer: Last refreshed */}
                {widget.last_refreshed_at && (
                    <div className="px-4 py-3 border-t border-gray-200 flex items-center gap-1.5">
                        <span className="text-xs text-gray-400">
                            Last refreshed:{' '}
                            <span className="text-gray-600">{formatRelativeTime(widget.last_refreshed_at)}</span>
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

            {/* Widget menu dropdown — fixed positioned */}
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
                                    onClick={() => { setShowMenu(false); setMenuPosition(null); onEdit(); }}
                                    className="flex items-center w-full text-left px-4 py-2 text-sm font-semibold text-black hover:bg-gray-200 transition-colors"
                                >
                                    <Pencil className="w-4 h-4 mr-2 text-black" />
                                    Edit Widget
                                </button>
                                <div className="h-px bg-gray-200 mx-2" />
                            </>
                        )}
                        <button
                            onClick={() => { setShowMenu(false); setMenuPosition(null); setShowDeleteConfirm(true); }}
                            className="flex items-center w-full text-left px-4 py-2 text-sm font-semibold text-red-500 hover:bg-neo-error hover:text-white transition-colors"
                        >
                            <Trash2 className="w-4 h-4 mr-2" />
                            Remove Widget
                        </button>
                    </div>
                </div>
            )}

            {/* Delete Confirmation Modal */}
            {showDeleteConfirm && (
                <ConfirmationModal
                    icon={<Trash2 className="w-6 h-6 text-neo-error" />}
                    title="Remove Widget"
                    message={`Are you sure you want to remove "${widget.title}"? This action cannot be undone.`}
                    buttonText="Remove"
                    onConfirm={async () => { setShowDeleteConfirm(false); onDelete(); }}
                    onCancel={() => setShowDeleteConfirm(false)}
                    zIndex="z-[120]"
                />
            )}
        </>
    );
}
