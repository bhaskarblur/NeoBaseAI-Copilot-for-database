import { ArrowDown, ArrowUp, ArrowUpDown, ChevronLeft, ChevronRight } from 'lucide-react';
import { useEffect, useState } from 'react';
import { Widget } from '../../../types/dashboard';
import { isDateString, formatDateValue } from './widgetConstants';

interface TableWidgetProps {
    widget: Widget;
    data: Record<string, unknown>[];
    onNextPage?: () => void;
    onPreviousPage?: () => void;
}

export default function TableWidget({ widget, data, onNextPage, onPreviousPage }: TableWidgetProps) {
    // Table date format: column key → true = human-readable, false = ISO
    const [dateFormats, setDateFormats] = useState<Record<string, boolean>>({});
    // Table cell expand state for Read more / Less
    const [expandedCells, setExpandedCells] = useState<Record<string, boolean>>({});
    // Table sorting: column key and direction
    const [sortConfig, setSortConfig] = useState<{ key: string; direction: 'asc' | 'desc' } | null>(null);
    // Sort tooltip hover state
    const [hoveredSortColumn, setHoveredSortColumn] = useState<string | null>(null);
    // In-memory page for non-cursor table widgets
    const [localPage, setLocalPage] = useState(1);

    // Detect date columns for table widget
    useEffect(() => {
        if (data.length === 0) return;
        const newDateCols: Record<string, boolean> = {};
        const firstRow = data[0];
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
    }, [data]);

    // Reset local page when data is refreshed
    useEffect(() => {
        setLocalPage(1);
    }, [data]);

    if (data.length === 0) return null;

    // Helper: Get nested value using dot-notation path (e.g., "careerIntent.intents")
    const getNestedValue = (obj: any, path: string): any => {
        if (path in obj) return obj[path];
        const parts = path.split('.');
        let value = obj;
        for (const part of parts) {
            if (value == null) return undefined;
            value = value[part];
        }
        return value;
    };

    const columns = widget.table_config?.columns;
    const columnKeys = columns ? columns.map((c) => c.key) : Object.keys(data[0]).slice(0, 10);
    const columnLabels = columns
        ? columns.reduce((acc, c) => ({ ...acc, [c.key]: c.label }), {} as Record<string, string>)
        : columnKeys.reduce(
              (acc, k) => ({ ...acc, [k]: k.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase()) }),
              {} as Record<string, string>,
          );

    const pageSize = widget.table_config?.page_size || 25;

    // Apply sorting
    let sortedData = [...data];
    if (sortConfig) {
        sortedData.sort((a, b) => {
            const aVal = getNestedValue(a, sortConfig.key);
            const bVal = getNestedValue(b, sortConfig.key);
            if (aVal == null && bVal == null) return 0;
            if (aVal == null) return sortConfig.direction === 'asc' ? 1 : -1;
            if (bVal == null) return sortConfig.direction === 'asc' ? -1 : 1;
            let cmp = 0;
            if (typeof aVal === 'number' && typeof bVal === 'number') {
                cmp = aVal - bVal;
            } else if (typeof aVal === 'string' && typeof bVal === 'string') {
                cmp = aVal.localeCompare(bVal);
            } else {
                cmp = String(aVal).localeCompare(String(bVal));
            }
            return sortConfig.direction === 'asc' ? cmp : -cmp;
        });
    }

    const totalPages = Math.ceil(sortedData.length / pageSize);
    const isCursorWidget = !!widget.table_config?.cursor_field;
    const effectivePage = isCursorWidget ? 1 : Math.min(localPage, Math.max(1, totalPages));
    const displayRows = sortedData.slice((effectivePage - 1) * pageSize, effectivePage * pageSize);

    const handleSort = (columnKey: string) => {
        setLocalPage(1);
        setSortConfig((current) => {
            if (current?.key === columnKey) {
                return current.direction === 'asc' ? { key: columnKey, direction: 'desc' } : null;
            }
            return { key: columnKey, direction: 'asc' };
        });
    };

    const getSortIcon = (columnKey: string) => {
        if (sortConfig?.key === columnKey) {
            return (
                <div className="flex items-center gap-1">
                    {sortConfig.direction === 'asc' ? (
                        <ArrowUp className="w-4 h-4 text-black" />
                    ) : (
                        <ArrowDown className="w-4 h-4 text-black" />
                    )}
                    <span className="text-[10px] font-semibold text-black uppercase">
                        {sortConfig.direction === 'asc' ? 'Asc' : 'Desc'}
                    </span>
                </div>
            );
        }
        return <ArrowUpDown className="w-4 h-4 text-gray-400" />;
    };

    const getSortTooltip = (columnKey: string) => {
        if (sortConfig?.key === columnKey) {
            return sortConfig.direction === 'asc' ? 'Sort descending' : 'Clear sorting';
        }
        return 'Sort ascending';
    };

    // Detect date columns
    const dateColumnSet = new Set<string>();
    for (const key of columnKeys) {
        if (data.some((row) => isDateString(getNestedValue(row, key)))) dateColumnSet.add(key);
    }

    const colWidth = columnKeys.length <= 3
        ? 'min-w-[180px]'
        : columnKeys.length <= 5
            ? 'min-w-[140px]'
            : 'min-w-[120px]';

    const renderCellContent = (value: unknown, column: string, rowIndex: number) => {
        const cellId = `${rowIndex}-${column}`;
        const isExpanded = expandedCells[cellId];

        if (value === null || value === undefined) {
            return <span className="text-gray-400">—</span>;
        }

        if (dateColumnSet.has(column) && typeof value === 'string') {
            return <span>{formatDateValue(value, dateFormats[column] ?? true)}</span>;
        }

        if (Array.isArray(value)) {
            if (value.length === 0) {
                return <span className="text-gray-400 text-sm">Empty list</span>;
            }
            return (
                <div className="flex flex-col gap-1">
                    <button
                        onClick={(e) => { e.stopPropagation(); setExpandedCells((prev) => ({ ...prev, [cellId]: !prev[cellId] })); }}
                        className="text-left text-sm text-green-600 hover:text-green-800 font-medium flex items-center gap-1"
                    >
                        {isExpanded ? '▼' : '▶'} List ({value.length} {value.length === 1 ? 'item' : 'items'})
                    </button>
                    {isExpanded && (
                        <div className="pl-4 space-y-1 text-sm">
                            {value.map((item, idx) => (
                                <div key={idx} className="py-1 border-l-2 border-gray-200 pl-2">
                                    {typeof item === 'object' && item !== null ? (
                                        <div className="text-base text-gray-700 bg-gray-100 p-3 rounded space-y-0.5">
                                            {Object.entries(item).map(([key, val]) => (
                                                <div key={key} className="flex gap-2">
                                                    <span className="font-semibold text-gray-600">{key.charAt(0).toUpperCase() + key.slice(1)}:</span>
                                                    <span className="text-gray-800">{typeof val === 'object' && val !== null ? JSON.stringify(val) : String(val)}</span>
                                                </div>
                                            ))}
                                        </div>
                                    ) : (
                                        <span>{String(item)}</span>
                                    )}
                                </div>
                            ))}
                        </div>
                    )}
                </div>
            );
        }

        if (typeof value === 'object') {
            const keys = Object.keys(value as object);
            if (keys.length === 0) {
                return <span className="text-gray-400 text-sm">Empty object</span>;
            }
            return (
                <div className="flex flex-col gap-1">
                    <button
                        onClick={(e) => { e.stopPropagation(); setExpandedCells((prev) => ({ ...prev, [cellId]: !prev[cellId] })); }}
                        className="text-left text-sm text-green-600 hover:text-green-800 font-medium flex items-center gap-1"
                    >
                        {isExpanded ? '▼' : '▶'} Object ({keys.length} {keys.length === 1 ? 'property' : 'properties'})
                    </button>
                    {isExpanded && (
                        <div className="text-sm text-gray-700 bg-gray-100 p-2 rounded space-y-0.5">
                            {Object.entries(value as object).map(([key, val]) => (
                                <div key={key} className="flex gap-2">
                                    <span className="font-semibold text-gray-600">{key.charAt(0).toUpperCase() + key.slice(1)}:</span>
                                    <span className="text-gray-800">{typeof val === 'object' && val !== null ? JSON.stringify(val) : String(val)}</span>
                                </div>
                            ))}
                        </div>
                    )}
                </div>
            );
        }

        if (typeof value === 'string' && value.length > 56) {
            return (
                <div className={isExpanded ? 'whitespace-normal break-words' : ''}>
                    <span>{isExpanded ? value : value.slice(0, 56) + '...'}</span>
                    <button
                        onClick={(e) => { e.stopPropagation(); setExpandedCells((prev) => ({ ...prev, [cellId]: !prev[cellId] })); }}
                        className="ml-1 text-sm text-green-600 hover:text-green-800 font-medium"
                    >
                        {isExpanded ? 'Less' : 'Read more'}
                    </button>
                </div>
            );
        }

        return <span>{String(value)}</span>;
    };

    const showPagination = isCursorWidget
        ? (widget.current_page && widget.current_page > 1) || widget.has_more || (widget.cursor_stack?.length ?? 0) > 0
        : totalPages > 1;

    return (
        <div className="overflow-auto max-h-[420px]">
            <table className="w-full text-base leading-relaxed border-collapse min-w-max">
                <thead className="sticky top-0 z-10 bg-white">
                    <tr className="border-b-2 border-black/10">
                        {columnKeys.map((key) => (
                            <th key={key} className={`text-left py-2.5 px-3 font-bold text-black whitespace-nowrap ${colWidth}`}>
                                <div className="flex items-center gap-2">
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
                                    <div className="relative">
                                        <button
                                            onClick={(ev) => { ev.stopPropagation(); handleSort(key); }}
                                            onMouseEnter={() => setHoveredSortColumn(key)}
                                            onMouseLeave={() => setHoveredSortColumn(null)}
                                            className="p-0.5 hover:bg-gray-100 rounded transition-colors focus:outline-none"
                                        >
                                            {getSortIcon(key)}
                                        </button>
                                        {hoveredSortColumn === key && (
                                            <div className="absolute z-[100] top-full mt-1 left-1/2 -translate-x-1/2 whitespace-nowrap bg-black text-white text-xs px-2 py-1 rounded shadow-lg pointer-events-none">
                                                {getSortTooltip(key)}
                                                <div className="absolute bottom-full left-1/2 -translate-x-1/2 border-[4px] border-transparent border-b-black" />
                                            </div>
                                        )}
                                    </div>
                                </div>
                            </th>
                        ))}
                    </tr>
                </thead>
                <tbody>
                    {displayRows.map((row, i) => (
                        <tr key={i} className="border-b border-gray-100 hover:bg-[#FFDB58]/20 transition-colors">
                            {columnKeys.map((key) => {
                                const value = getNestedValue(row, key);
                                const cellId = `${i}-${key}`;
                                const isExpanded = expandedCells[cellId];
                                const showTitle = !isExpanded && value !== null && value !== undefined &&
                                    typeof value !== 'object' && typeof value === 'string' && value.length > 56;
                                return (
                                    <td
                                        key={key}
                                        className={`py-2.5 px-3 pr-6 text-gray-700 max-w-[540px] ${isExpanded ? 'whitespace-normal' : 'whitespace-nowrap truncate'} ${colWidth}`}
                                        title={showTitle ? String(value) : undefined}
                                    >
                                        {renderCellContent(value, key, i)}
                                    </td>
                                );
                            })}
                        </tr>
                    ))}
                </tbody>
            </table>

            {/* Pagination UI */}
            {showPagination && (
                <div className="sticky bottom-0 bg-white border-t border-gray-100 px-2 py-2">
                    <div className="flex items-center justify-between text-sm">
                        <div className="text-gray-500">
                            {isCursorWidget ? (
                                widget.current_page ? (
                                    <span>Page <span className="text-gray-700">{widget.current_page}</span>, <span>Showing <span className="text-gray-700">{Math.min(pageSize, data.length)} rows</span></span></span>
                                ) : (
                                    <span>Showing <span className="text-gray-700">{Math.min(pageSize, data.length)} rows</span></span>
                                )
                            ) : (
                                <span>Page <span className="text-gray-700">{effectivePage}</span> of <span className="text-gray-700">{totalPages}</span></span>
                            )}
                        </div>
                        <div className="flex items-center gap-2">
                            <button
                                onClick={() => isCursorWidget ? onPreviousPage?.() : setLocalPage(p => Math.max(1, p - 1))}
                                disabled={isCursorWidget ? (widget.cursor_stack?.length ?? 0) <= 1 || widget.is_loading : effectivePage <= 1 || widget.is_loading}
                                className="flex items-center gap-1 px-3 py-1.5 rounded-lg text-sm font-medium transition-colors disabled:opacity-40 disabled:cursor-not-allowed enabled:hover:bg-gray-200 enabled:text-black text-gray-600"
                            >
                                <ChevronLeft className="w-4 h-4" />
                                Previous
                            </button>
                            <button
                                onClick={() => isCursorWidget ? onNextPage?.() : setLocalPage(p => Math.min(totalPages, p + 1))}
                                disabled={isCursorWidget ? !widget.has_more || widget.is_loading : effectivePage >= totalPages || widget.is_loading}
                                className="flex items-center gap-1 px-3 py-1.5 rounded-lg text-sm font-medium transition-colors disabled:opacity-40 disabled:cursor-not-allowed enabled:hover:bg-gray-200 enabled:text-black text-gray-600"
                            >
                                Next
                                <ChevronRight className="w-4 h-4" />
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
