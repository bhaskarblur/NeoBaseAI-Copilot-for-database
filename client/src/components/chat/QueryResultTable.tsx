import { useState } from 'react';
import NestedJsonCell from './NestedJsonCell';
import { isDateString, formatDateString } from '../../utils/queryUtils';

interface QueryResultTableProps {
    data: any[];
    nonTechMode?: boolean;
    searchQuery?: string;
    /** Map of column name → true means display as human-readable date */
    dateColumns: Record<string, boolean>;
    setDateColumns: React.Dispatch<React.SetStateAction<Record<string, boolean>>>;
    expandedCells: Record<string, boolean>;
    setExpandedCells: React.Dispatch<React.SetStateAction<Record<string, boolean>>>;
    /** Shared expansion-state store for nested JSON cells */
    expandedNodesRef: React.MutableRefObject<Record<string, boolean>>;
}

// ─── DateFormatToggle ─────────────────────────────────────────────────────────

interface DateFormatToggleProps {
    column: string;
    dateColumns: Record<string, boolean>;
    setDateColumns: React.Dispatch<React.SetStateAction<Record<string, boolean>>>;
}

function DateFormatToggle({ column, dateColumns, setDateColumns }: DateFormatToggleProps) {
    return (
        <button
            onClick={e => {
                e.stopPropagation();
                e.preventDefault();
                setDateColumns(prev => ({ ...prev, [column]: !prev[column] }));
            }}
            className="inline-flex items-center text-xs px-1.5 py-0.5 ml-2 bg-gray-700 hover:bg-gray-600 rounded-sm text-gray-300"
            title="Toggle date format"
        >
            <svg xmlns="http://www.w3.org/2000/svg" width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="mr-1">
                <rect x="3" y="4" width="18" height="18" rx="2" ry="2" />
                <line x1="16" y1="2" x2="16" y2="6" />
                <line x1="8" y1="2" x2="8" y2="6" />
                <line x1="3" y1="10" x2="21" y2="10" />
            </svg>
            {dateColumns[column] ? 'ISO' : 'Human'}
        </button>
    );
}

// ─── QueryResultTable ─────────────────────────────────────────────────────────

export default function QueryResultTable({
    data,
    nonTechMode,
    dateColumns,
    setDateColumns,
    expandedCells,
    setExpandedCells,
    expandedNodesRef,
}: QueryResultTableProps) {
    const [, forceUpdate] = useState(0); // trigger re-render on cell expansion

    if (!data || data.length === 0) {
        return (
            <div className="text-gray-500">
                {nonTechMode ? 'No Data found' : 'No data to display'}
            </div>
        );
    }

    // Collect columns preserving first-row order, then adding any extras
    const columnSet = new Set<string>();
    const columnOrder: string[] = [];
    Object.keys(data[0]).forEach(col => { columnSet.add(col); columnOrder.push(col); });
    for (let i = 1; i < data.length; i++) {
        Object.keys(data[i]).forEach(col => {
            if (!columnSet.has(col)) { columnSet.add(col); columnOrder.push(col); }
        });
    }

    const dateColumnList = columnOrder.filter(column => {
        for (let i = 0; i < Math.min(data.length, 5); i++) {
            if (isDateString(data[i][column])) return true;
        }
        return false;
    });

    const toggleCellExpansion = (cellId: string) => {
        setExpandedCells(prev => ({ ...prev, [cellId]: !prev[cellId] }));
        forceUpdate(n => n + 1);
    };

    const renderCellValue = (value: any, column: string, rowId: string) => {
        const cellId = `${rowId}-${column}`;
        const isCellExpanded = expandedCells[cellId];

        if (value === null) {
            return nonTechMode
                ? <span className="text-gray-400">-</span>
                : <span className="text-yellow-400">null</span>;
        }
        if (value === undefined) {
            return nonTechMode
                ? <span className="text-gray-400">-</span>
                : <span className="text-yellow-400">undefined</span>;
        }
        if (typeof value === 'object' && value !== null) {
            if (Object.keys(value).length === 0) return <span className="text-gray-400">-</span>;
            return <NestedJsonCell data={value} expandedNodesRef={expandedNodesRef} />;
        }
        if (typeof value === 'number') return <span className="text-cyan-400">{value}</span>;
        if (typeof value === 'string') {
            if (isDateString(value)) {
                return (
                    <span className="text-yellow-300">
                        {formatDateString(value, dateColumns[column] !== false)}
                    </span>
                );
            }
            if (value.length > 140) {
                const maxLength = Math.min(250, value.length);
                const truncatedText = isCellExpanded ? value : value.substring(0, maxLength);
                return (
                    <span onClick={() => toggleCellExpansion(cellId)} className="cursor-pointer">
                        <span className="text-green-400">"{truncatedText}</span>
                        {!isCellExpanded ? (
                            <span className="text-green-400">..<br /><span className="text-cyan-400">Show More</span></span>
                        ) : (
                            <span className="text-cyan-400"><br />Show Less</span>
                        )}
                    </span>
                );
            }
            return <span className="text-green-400">"{value}"</span>;
        }
        if (typeof value === 'boolean') return <span className="text-purple-400">{String(value)}</span>;
        return <span>{String(value)}</span>;
    };

    return (
        <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
                <thead>
                    <tr>
                        {columnOrder.map(column => (
                            <th key={column} className="py-2 px-4 bg-gray-800 border-b border-gray-700 text-gray-300 font-mono">
                                <div className="flex items-center">
                                    <span>{column}</span>
                                    {dateColumnList.includes(column) && (
                                        <DateFormatToggle
                                            column={column}
                                            dateColumns={dateColumns}
                                            setDateColumns={setDateColumns}
                                        />
                                    )}
                                </div>
                            </th>
                        ))}
                    </tr>
                </thead>
                <tbody>
                    {data.map((row, i) => (
                        <tr key={i} className="border-b border-gray-700">
                            {columnOrder.map(column => {
                                const isComplexObject =
                                    typeof row[column] === 'object' &&
                                    row[column] !== null &&
                                    Object.keys(row[column]).length > 0;
                                const isTooLong = row[column] != null && typeof row[column] === 'string' && row[column].length > 140;
                                const isDateColumn = dateColumnList.includes(column);
                                return (
                                    <td
                                        key={column}
                                        className={`py-2 px-4 ${isComplexObject ? 'min-w-[300px]' : ''} ${isDateColumn ? 'min-w-[200px] whitespace-nowrap' : ''} ${isTooLong ? 'min-w-[480px]' : ''}`}
                                    >
                                        {renderCellValue(row[column], column, row.id ?? String(i))}
                                    </td>
                                );
                            })}
                        </tr>
                    ))}
                </tbody>
            </table>
        </div>
    );
}
