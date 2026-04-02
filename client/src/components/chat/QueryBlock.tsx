import { useState } from 'react';
import {
    AlertCircle, BarChart3, Braces, Clock, Copy, History,
    Loader, Pencil, Play, RefreshCcw, Table, XCircle,
} from 'lucide-react';
import toast from 'react-hot-toast';
import analyticsService from '../../services/analyticsService';
import { Message, QueryResult } from '../../types/query';
import { QueryResultState, QueryState } from '../../types/messageTile';
import { formatActionAt } from '../../utils/message';
import { removeDuplicateQueries, parseResults } from '../../utils/queryUtils';
import { highlightSearchText } from '../../utils/highlightSearch';
import QueryResultTable from './QueryResultTable';
import ColoredJsonView from './ColoredJsonView';
import QueryPagination from './QueryPagination';
import VisualizationPanel from './VisualizationPanel';
import ConfirmationModal from '../modals/ConfirmationModal';
import RollbackConfirmationModal from '../modals/RollbackConfirmationModal';

const toastStyle = {
    style: {
        background: '#000', color: '#fff', border: '4px solid #000', borderRadius: '12px',
        boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)', padding: '12px 24px',
        fontSize: '14px', fontWeight: '500',
    },
    position: 'bottom-center' as const,
    duration: 2000,
};

export interface QueryBlockProps {
    chatId: string;
    message: Message;
    query: QueryResult;
    index: number;
    isMessageStreaming: boolean;
    streamingQueryIndex: number;
    currentDescription: string;
    currentQuery: string;
    isDescriptionStreaming: boolean;
    isQueryStreaming: boolean;
    queryState: QueryState;
    queryResult: QueryResultState | undefined;
    isResultMinimized: boolean;
    isEditingQuery: boolean;
    editedQueryText: string;
    dateColumns: Record<string, boolean>;
    setDateColumns: React.Dispatch<React.SetStateAction<Record<string, boolean>>>;
    expandedCells: Record<string, boolean>;
    setExpandedCells: React.Dispatch<React.SetStateAction<Record<string, boolean>>>;
    expandedNodesRef: React.MutableRefObject<Record<string, boolean>>;
    pageDataCacheRef: React.MutableRefObject<Record<string, Record<number, any>>>;
    openDownloadMenu: string | null;
    setOpenDownloadMenu: React.Dispatch<React.SetStateAction<string | null>>;
    searchQuery?: string;
    searchResultRefs?: React.MutableRefObject<{ [key: string]: HTMLElement | null }>;
    userId?: string;
    userName?: string;
    // Handlers
    onSetIsEditingQuery: (queryId: string, value: boolean) => void;
    onSetEditedQueryText: (queryId: string, value: string) => void;
    onEditQuery: (messageId: string, queryId: string, query: string) => void;
    onExecuteQuery: (queryId: string) => Promise<void>;
    onRollback: (queryId: string) => Promise<void>;
    onPageChange: (queryId: string, page: number) => Promise<void>;
    onExportData: (queryId: string, format: 'csv' | 'json') => void;
    onExportVisualization: (queryId: string) => Promise<void>;
    onToggleResultMinimize: (queryId: string) => void;
    onQueryUpdate: (callback: () => void) => void;
    setQueryStates: React.Dispatch<React.SetStateAction<Record<string, QueryState>>>;
    abortControllerRef: React.MutableRefObject<Record<string, AbortController>>;
    queryTimeouts: React.MutableRefObject<Record<string, NodeJS.Timeout>>;
    onVisualizationSaved?: (queryId: string, vizData: any) => void;
}

export default function QueryBlock({
    chatId, message, query, index,
    isMessageStreaming, streamingQueryIndex,
    currentDescription, currentQuery, isDescriptionStreaming, isQueryStreaming,
    queryState, queryResult, isResultMinimized,
    isEditingQuery, editedQueryText,
    dateColumns, setDateColumns,
    expandedCells, setExpandedCells, expandedNodesRef,
    pageDataCacheRef, openDownloadMenu, setOpenDownloadMenu,
    searchQuery, searchResultRefs,
    userId, userName,
    onSetIsEditingQuery, onSetEditedQueryText, onEditQuery,
    onExecuteQuery, onRollback, onPageChange,
    onExportData, onExportVisualization,
    onToggleResultMinimize, onQueryUpdate, setQueryStates,
    abortControllerRef, queryTimeouts,
    onVisualizationSaved,
}: QueryBlockProps) {
    const [viewMode, setViewMode] = useState<'table' | 'json' | 'visualization'>('table');
    const [showCriticalConfirm, setShowCriticalConfirm] = useState(false);
    const [queryToExecute, setQueryToExecute] = useState<string | null>(null);
    const [showRollbackConfirm, setShowRollbackConfirm] = useState(false);

    if (isMessageStreaming && streamingQueryIndex !== -1 && index !== streamingQueryIndex) return null;

    const queryId = query.id;
    const isCurrentlyStreaming = !isMessageStreaming && streamingQueryIndex === index;
    const shouldShowExampleResult = !query.is_executed && !query.is_rolled_back;
    const resultToShow = shouldShowExampleResult ? query.example_result : query.execution_result;

    const shouldShowRollback =
        query.can_rollback &&
        ((query.rollback_query != null && query.rollback_query !== '') ||
            (query.rollback_dependent_query != null && query.rollback_dependent_query !== '')) &&
        query.is_executed && !query.is_rolled_back && !query.error;

    const handleExecute = () => {
        if (userId && userName) analyticsService.trackQueryExecuteClick(chatId, queryId, userId, userName);
        if (query.is_critical) { setQueryToExecute(queryId); setShowCriticalConfirm(true); return; }
        onExecuteQuery(queryId);
    };

    const handleCancelExecution = () => {
        if (abortControllerRef.current[queryId]) {
            abortControllerRef.current[queryId].abort();
            delete abortControllerRef.current[queryId];
        }
        if (queryTimeouts.current[queryId]) {
            clearTimeout(queryTimeouts.current[queryId]);
            delete queryTimeouts.current[queryId];
        }
        onQueryUpdate(() => setQueryStates(prev => ({ ...prev, [queryId]: { isExecuting: false, isExample: !query.is_executed } })));
        setTimeout(() => window.scrollTo(window.scrollX, window.scrollY), 0);
        toast.error('Query cancelled', toastStyle);
    };

    // ── Result area helpers ───────────────────────────────────────────────────
    const renderActiveResult = () => {
        if (!queryResult) return null;
        const parsedData = parseResults(queryResult.data);
        const useCursorPagination = query.pagination?.cursor_field != null;
        const totalRecords = queryResult.totalRecords || parsedData.length;
        const showPagination = useCursorPagination
            ? (queryResult.currentPage > 1 || queryResult.hasMore)
            : totalRecords > queryResult.pageSize;

        if (viewMode === 'json') {
            return (
                <pre className="overflow-x-auto whitespace-pre-wrap">
                    <ColoredJsonView data={parsedData} nonTechMode={message.non_tech_mode} />
                </pre>
            );
        }

        if (viewMode === 'visualization') {
            return (
                <VisualizationPanel
                    chatId={chatId}
                    messageId={message.id}
                    query={query}
                    userId={userId}
                    userName={userName}
                    initialVisualization={query.visualization}
                    onVisualizationSaved={onVisualizationSaved}
                />
            );
        }

        // Table view
        return (
            <>
                {totalRecords > 50 && (
                    <div className="text-gray-300 mb-4">
                        The result contains total <b className="text-yellow-500">{totalRecords}</b> records.
                    </div>
                )}
                {queryResult.loading ? (
                    <div className="flex justify-center p-4">
                        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-white" />
                    </div>
                ) : (
                    <>
                        {queryResult.error && (
                            <div className="text-red-500 py-2 mb-2">Error in fetching results: {queryResult.error}</div>
                        )}
                        {parsedData.length > 0 ? (
                            <QueryResultTable
                                data={parsedData}
                                nonTechMode={message.non_tech_mode}
                                searchQuery={searchQuery}
                                dateColumns={dateColumns}
                                setDateColumns={setDateColumns}
                                expandedCells={expandedCells}
                                setExpandedCells={setExpandedCells}
                                expandedNodesRef={expandedNodesRef}
                            />
                        ) : (
                            <div className="text-gray-500">{message.non_tech_mode ? 'No Data found' : 'No data to display'}</div>
                        )}
                        {showPagination && (
                            <QueryPagination
                                currentPage={queryResult.currentPage}
                                totalRecords={queryResult.totalRecords}
                                pageSize={queryResult.pageSize}
                                hasMore={queryResult.hasMore}
                                useCursorPagination={useCursorPagination}
                                isLoading={queryResult.loading}
                                onPageChange={page => onPageChange(queryId, page)}
                            />
                        )}
                    </>
                )}
            </>
        );
    };

    const descriptionText = isCurrentlyStreaming && isDescriptionStreaming ? currentDescription : query.description;
    const queryText = removeDuplicateQueries(
        isCurrentlyStreaming && isQueryStreaming ? currentQuery : (query.query || ''),
    );

    return (
        <div>
            {/* Explanation */}
            <p
                className="mb-4 mt-4 font-base text-base"
                ref={el => { if (searchResultRefs && el) searchResultRefs.current[`explanation-${message.id}-${index}`] = el; }}
            >
                <span className="text-black font-semibold">Explanation:</span>{' '}
                {searchQuery ? highlightSearchText(descriptionText, searchQuery) : descriptionText}
            </p>

            {/* Query block */}
            <div
                className="mt-4 bg-black text-white rounded-lg font-mono text-sm overflow-hidden w-full"
                style={{ minWidth: '100%' }}
                ref={el => { if (searchResultRefs && el) searchResultRefs.current[`query-${message.id}-${index}`] = el; }}
            >
                {/* Header toolbar */}
                <div className="flex flex-wrap items-center justify-between gap-2 mb-4 px-4 pt-4">
                    <div className="flex justify-between items-center gap-2">
                        <span>{message.non_tech_mode ? `Action ${index + 1}:` : `Query ${index + 1}:`}</span>
                        {query.is_edited && (
                            <span className="text-xs bg-gray-500/20 text-gray-300 px-2 py-0.5 rounded">Edited</span>
                        )}
                        {query.is_rolled_back ? (
                            <span className="text-xs bg-yellow-500/20 text-yellow-300 px-2 py-0.5 rounded">
                                Rolled Back on {query.action_at ? formatActionAt(query.action_at) : ''}
                            </span>
                        ) : query.is_executed && query.action_at ? (
                            <span className="w-[60%] md:w-auto text-xs bg-green-500/20 text-green-300 px-2 py-0.5 rounded">
                                Executed on {formatActionAt(query.action_at)}
                            </span>
                        ) : (
                            <span className="text-xs bg-blue-500/20 text-blue-300 px-2 py-0.5 rounded">Execute Manually</span>
                        )}
                    </div>

                    <div className="flex items-center">
                        {!queryState.isExecuting && !query.is_executed && (
                            <>
                                <button
                                    onClick={() => {
                                        if (userId && userName) analyticsService.trackQueryEditClick(chatId, queryId, userId, userName);
                                        onSetIsEditingQuery(queryId, true);
                                    }}
                                    className="p-2 hover:bg-gray-800 rounded transition-colors text-yellow-400 hover:text-yellow-300 hover-tooltip-messagetile"
                                    data-tooltip="Edit query" title="Edit query"
                                >
                                    <Pencil className="w-4 h-4" />
                                </button>
                                <div className="w-px h-4 bg-gray-700 mx-2" />
                            </>
                        )}
                        {queryState.isExecuting ? (
                            <button onClick={handleCancelExecution} className="p-2 hover:bg-gray-800 rounded transition-colors text-red-500 hover:text-red-400 hover-tooltip-messagetile" data-tooltip="Cancel query" title="Cancel query">
                                <XCircle className="w-4 h-4" />
                            </button>
                        ) : (
                            <button
                                onClick={handleExecute}
                                className="p-2 text-red-500 hover:text-red-400 hover:bg-gray-800 rounded transition-colors hover-tooltip-messagetile"
                                data-tooltip={query.is_executed ? 'Rerun query' : 'Execute query'}
                                title={query.is_executed ? 'Rerun query' : 'Execute query'}
                            >
                                {query.is_executed ? <RefreshCcw className="w-4 h-4" /> : <Play className="w-4 h-4" />}
                            </button>
                        )}
                        <div className="w-px h-4 bg-gray-700 mx-2" />
                        <button
                            onClick={() => { navigator.clipboard.writeText(query.query); toast('Copied to clipboard!', { ...toastStyle, icon: '📋' }); if (userId && userName) analyticsService.trackQueryCopyClick(chatId, queryId, userId, userName); }}
                            className="p-2 hover:bg-gray-800 rounded text-white hover:text-gray-200 hover-tooltip-messagetile"
                            data-tooltip="Copy query" title="Copy query"
                        >
                            <Copy className="w-4 h-4" />
                        </button>
                    </div>
                </div>

                {/* Query text / edit form */}
                {isEditingQuery ? (
                    <div className="px-4 pb-4 border-t border-gray-700 pt-4">
                        <textarea
                            value={editedQueryText}
                            onChange={e => onSetEditedQueryText(queryId, e.target.value)}
                            className="w-full bg-gray-900 text-white p-3 rounded-none border-4 border-gray-600 font-mono text-sm min-h-[120px] focus:outline-none focus:border-neo-gray shadow-[4px_4px_0px_0px_rgba(75,85,99,1)]"
                        />
                        <div className="flex justify-end gap-3 mt-4">
                            <button onClick={() => { onSetIsEditingQuery(queryId, false); onSetEditedQueryText(queryId, removeDuplicateQueries(query.query || '')); }} className="font-semibold px-4 py-2 bg-gray-800 text-white border-2 border-gray-600 hover:bg-gray-700 transition-colors shadow-[2px_2px_0px_0px_rgba(75,85,99,1)] active:translate-y-[1px]">Cancel</button>
                            <button onClick={() => { onSetIsEditingQuery(queryId, false); onEditQuery(message.id, queryId, editedQueryText); }} className="font-semibold px-4 py-2 bg-yellow-400 text-black border-2 border-black hover:bg-yellow-300 transition-colors shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] active:translate-y-[1px]">Save Changes</button>
                        </div>
                    </div>
                ) : !message.non_tech_mode ? (
                    <pre className={`text-sm overflow-x-auto p-4 border-t border-gray-700 ${isCurrentlyStreaming && isQueryStreaming ? 'animate-pulse duration-300' : ''}`}>
                        <code className="whitespace-pre-wrap break-words">
                            {searchQuery ? highlightSearchText(queryText, searchQuery) : queryText}
                        </code>
                    </pre>
                ) : <div />}

                {/* Result area */}
                {(query.execution_result || query.example_result || query.error || queryState.isExecuting) && (
                    <div
                        className="border-t border-gray-700 mt-2 w-full"
                        ref={el => { if (searchResultRefs && el) searchResultRefs.current[`result-${message.id}-${index}`] = el; }}
                    >
                        {queryState.isExecuting ? (
                            <div className="flex items-center justify-center p-8">
                                <Loader className="w-8 h-8 animate-spin text-gray-400" />
                                <span className="ml-3 text-gray-400">Executing query...</span>
                            </div>
                        ) : (
                            <div className="mt-3 px-4 pt-4 w-full">
                                {/* Result header */}
                                <div className="flex flex-wrap items-center justify-between gap-2 mb-4">
                                    <div className="flex items-center gap-2 text-gray-400">
                                        {query.error ? (
                                            <span className="text-neo-error font-medium flex items-center gap-2">
                                                <AlertCircle className="w-4 h-4" />Error
                                            </span>
                                        ) : (
                                            <div className="flex items-center gap-2">
                                                <button onClick={() => onToggleResultMinimize(queryId)} className="p-1 hover:bg-gray-800 rounded text-white hover:text-gray-200" title={isResultMinimized ? 'Expand' : 'Collapse'}>
                                                    {isResultMinimized
                                                        ? <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="m6 9 6 6 6-6" /></svg>
                                                        : <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="m18 15-6-6-6 6" /></svg>}
                                                </button>
                                                <span>{shouldShowExampleResult ? 'Example Result:' : query.is_rolled_back ? 'Rolled Back Result:' : 'Result:'}</span>
                                            </div>
                                        )}
                                        {query.example_execution_time && !query.execution_time && !query.is_executed && !query.error && (
                                            <span className="text-xs bg-gray-800 px-2 py-1 rounded flex items-center gap-1"><Clock className="w-3 h-3" />{query.example_execution_time.toLocaleString()}ms</span>
                                        )}
                                        {query.execution_time! > 0 && !query.error && (
                                            <span className="text-xs bg-gray-800 px-2 py-1 rounded flex items-center gap-1"><Clock className="w-3 h-3" />{query.execution_time!.toLocaleString()}ms</span>
                                        )}
                                    </div>

                                    {!query.error && (
                                        <div className="flex gap-2">
                                            <div className="flex items-center">
                                                {/* View mode toggles */}
                                                {(['table', 'json', 'visualization'] as const).map((mode, i) => {
                                                    const icons = [<Table className="w-4 h-4" />, <Braces className="w-4 h-4" />, <BarChart3 className="w-4 h-4" />];
                                                    const labels = ['Table view', 'JSON view', 'Visualization view'];
                                                    return (
                                                        <span key={mode} className="flex items-center">
                                                            {i > 0 && <div className="w-px h-4 bg-gray-700 mx-2" />}
                                                            <button
                                                                onClick={() => { if (userId && userName) analyticsService.trackResultViewToggle(chatId, queryId, mode, userId, userName); setViewMode(mode); setTimeout(() => window.scrollTo(window.scrollX, window.scrollY), 0); }}
                                                                className={`p-1 md:p-2 rounded ${viewMode === mode ? 'bg-gray-700' : 'hover:bg-gray-800'} hover-tooltip-messagetile`}
                                                                data-tooltip={labels[i]} title={labels[i]}
                                                            >
                                                                {icons[i]}
                                                            </button>
                                                        </span>
                                                    );
                                                })}

                                                {/* Download dropdown */}
                                                <div className="w-px h-4 bg-gray-700 mx-2" />
                                                <div className="relative download-dropdown-container">
                                                    <button onClick={() => setOpenDownloadMenu(openDownloadMenu === queryId ? null : queryId)} className="p-1 md:p-2 rounded hover:bg-gray-800 flex items-center gap-1 hover-tooltip-messagetile" data-tooltip="Download data" title="Download data">
                                                        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="w-4 h-4"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><polyline points="7 10 12 15 17 10" /><line x1="12" y1="15" x2="12" y2="3" /></svg>
                                                        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="w-3 h-3"><polyline points="6 9 12 15 18 9" /></svg>
                                                    </button>
                                                    {openDownloadMenu === queryId && (
                                                        <div className="absolute right-0 mt-2 w-64 bg-gray-800 border border-gray-700 rounded-lg shadow-lg z-50" style={{ top: '100%' }}>
                                                            <div className="p-2 text-xs text-gray-400 border-b border-gray-700">
                                                                {(() => {
                                                                    const pages = pageDataCacheRef.current[queryId] || {};
                                                                    const n = Object.keys(pages).length;
                                                                    const total = Object.values(pages).reduce((acc: number, p: any) => acc + (Array.isArray(p?.data) ? p.data.length : 0), 0);
                                                                    return `This will export ${total} records from ${n} fetched ${n === 1 ? 'page' : 'pages'}.`;
                                                                })()}
                                                            </div>
                                                            <div className="py-1">
                                                                <button onClick={() => { onExportData(queryId, 'csv'); setOpenDownloadMenu(null); }} className="block w-full text-left px-4 py-2 text-sm text-white hover:bg-gray-700">Export as CSV</button>
                                                                <button onClick={() => { onExportData(queryId, 'json'); setOpenDownloadMenu(null); }} className="block w-full text-left px-4 py-2 text-sm text-white hover:bg-gray-700">Export as JSON</button>
                                                                {query.visualization && (
                                                                    <button onClick={() => { onExportVisualization(queryId); setOpenDownloadMenu(null); }} className="block w-full text-left px-4 py-2 text-sm text-white hover:bg-gray-700">Export Visualization</button>
                                                                )}
                                                            </div>
                                                        </div>
                                                    )}
                                                </div>

                                                {/* Rollback */}
                                                {shouldShowRollback && (
                                                    <>
                                                        <div className="w-px h-4 bg-gray-700 mx-2" />
                                                        {!queryState.isExecuting ? (
                                                            <button onClick={() => { if (userId && userName) analyticsService.trackRollbackClick(chatId, queryId, userId, userName); setShowRollbackConfirm(true); setTimeout(() => window.scrollTo(window.scrollX, window.scrollY), 0); }} className="p-2 hover:bg-gray-800 rounded text-yellow-400 hover:text-yellow-300 hover-tooltip-messagetile" data-tooltip="Rollback changes">
                                                                <History className="w-4 h-4" />
                                                            </button>
                                                        ) : (
                                                            <button onClick={handleCancelExecution} className="p-2 hover:bg-gray-800 rounded transition-colors text-red-500 hover:text-red-400 hover-tooltip-messagetile" data-tooltip="Cancel query" title="Cancel query">
                                                                <XCircle className="w-4 h-4" />
                                                            </button>
                                                        )}
                                                        <div className="w-px h-4 bg-gray-700 mx-2" />
                                                    </>
                                                )}

                                                {/* Copy result JSON */}
                                                <button onClick={() => { navigator.clipboard.writeText(JSON.stringify(resultToShow, null, 2)); toast('Copied to clipboard!', { ...toastStyle, icon: '📋' }); if (userId && userName) analyticsService.trackResultCopyClick(chatId, queryId, userId, userName); }} className="p-2 hover:bg-gray-800 rounded text-white hover:text-gray-200 hover-tooltip-messagetile" data-tooltip="Copy JSON" title="Copy JSON">
                                                    <Copy className="w-4 h-4" />
                                                </button>
                                            </div>
                                        </div>
                                    )}
                                </div>

                                {/* Result body */}
                                {!isResultMinimized && (
                                    <>
                                        {query.error ? (
                                            <div className="bg-neo-error/10 text-neo-error p-4 rounded-lg mb-6">
                                                <div className="font-bold mb-2">{searchQuery ? highlightSearchText(query.error.code, searchQuery) : query.error.code}</div>
                                                {query.error.message !== query.error.details && <div className="mb-2">{searchQuery ? highlightSearchText(query.error.message, searchQuery) : query.error.message}</div>}
                                                {query.error.details && <div className="text-sm opacity-80 border-t border-neo-error/20 pt-2 mt-2">{searchQuery ? highlightSearchText(query.error.details, searchQuery) : query.error.details}</div>}
                                            </div>
                                        ) : (
                                            <div className="px-0">
                                                <div className="text-green-400 pb-6 w-full">
                                                    {shouldShowExampleResult ? (
                                                        viewMode === 'table' ? (
                                                            resultToShow
                                                                ? <QueryResultTable data={parseResults(resultToShow)} nonTechMode={message.non_tech_mode} searchQuery={searchQuery} dateColumns={dateColumns} setDateColumns={setDateColumns} expandedCells={expandedCells} setExpandedCells={setExpandedCells} expandedNodesRef={expandedNodesRef} />
                                                                : <div className="text-gray-500">{message.non_tech_mode ? 'No Data found' : 'No example data available'}</div>
                                                        ) : viewMode === 'json' ? (
                                                            resultToShow
                                                                ? <pre className="overflow-x-auto whitespace-pre-wrap rounded-md"><ColoredJsonView data={parseResults(resultToShow)} nonTechMode={message.non_tech_mode} /></pre>
                                                                : <div className="text-gray-500">{message.non_tech_mode ? 'No Data found' : 'No example data available'}</div>
                                                        ) : (
                                                            <VisualizationPanel chatId={chatId} messageId={message.id} query={query} userId={userId} userName={userName} initialVisualization={query.visualization} onVisualizationSaved={onVisualizationSaved} />
                                                        )
                                                    ) : (
                                                        resultToShow
                                                            ? renderActiveResult()
                                                            : <div className="text-gray-500">{message.non_tech_mode ? 'No Data found' : 'No data to display'}</div>
                                                    )}
                                                </div>
                                            </div>
                                        )}
                                    </>
                                )}
                            </div>
                        )}
                    </div>
                )}
            </div>

            {/* Modals */}
            {showRollbackConfirm && (
                <RollbackConfirmationModal
                    onConfirm={() => { setShowRollbackConfirm(false); onRollback(queryId); }}
                    onCancel={() => setShowRollbackConfirm(false)}
                />
            )}
            {showCriticalConfirm && (
                <ConfirmationModal
                    title="Critical Query"
                    message="This query may affect important data. Are you sure you want to proceed?"
                    onConfirm={async () => {
                        setShowCriticalConfirm(false);
                        if (queryToExecute) { await onExecuteQuery(queryToExecute); setQueryToExecute(null); }
                    }}
                    onCancel={() => { setShowCriticalConfirm(false); setQueryToExecute(null); }}
                />
            )}
        </div>
    );
}
