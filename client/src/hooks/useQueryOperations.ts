import axios from 'axios';
import { useCallback, useEffect, useRef, useState } from 'react';
import toast from 'react-hot-toast';
import analyticsService from '../services/analyticsService';
import chatService from '../services/chatService';
import { Message, QueryResult } from '../types/query';
import { PageData, QueryResultState, QueryState } from '../types/messageTile';
import {
    extractCursorValue,
    isDateString,
    parseResults,
    sliceIntoPages,
} from '../utils/queryUtils';

const DEFAULT_PAGE_SIZE = 25;

const toastStyle = {
    style: {
        background: '#000', color: '#fff', border: '4px solid #000', borderRadius: '12px',
        boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)', padding: '12px 24px',
        fontSize: '14px', fontWeight: '500',
    },
    position: 'bottom-center' as const,
    duration: 2000,
};

interface UseQueryOperationsParams {
    chatId: string;
    message: Message;
    streamId: string | null;
    queryStates: Record<string, QueryState>;
    setQueryStates: React.Dispatch<React.SetStateAction<Record<string, QueryState>>>;
    queryTimeouts: React.MutableRefObject<Record<string, NodeJS.Timeout>>;
    setMessage: (message: Message) => void;
    checkSSEConnection: () => Promise<void>;
    onQueryUpdate: (callback: () => void) => void;
    userId?: string;
    userName?: string;
}

export interface QueryOperationsReturn {
    queryResults: Record<string, QueryResultState>;
    setQueryResults: React.Dispatch<React.SetStateAction<Record<string, QueryResultState>>>;
    pageDataCacheRef: React.MutableRefObject<Record<string, Record<number, PageData | undefined>>>;
    expandedCells: Record<string, boolean>;
    setExpandedCells: React.Dispatch<React.SetStateAction<Record<string, boolean>>>;
    dateColumns: Record<string, boolean>;
    setDateColumns: React.Dispatch<React.SetStateAction<Record<string, boolean>>>;
    minimizedResults: Record<string, boolean>;
    toggleResultMinimize: (queryId: string) => void;
    openDownloadMenu: string | null;
    setOpenDownloadMenu: React.Dispatch<React.SetStateAction<string | null>>;
    editingQueries: Record<string, boolean>;
    setEditingQueries: React.Dispatch<React.SetStateAction<Record<string, boolean>>>;
    editedQueryTexts: Record<string, string>;
    setEditedQueryTexts: React.Dispatch<React.SetStateAction<Record<string, string>>>;
    abortControllerRef: React.MutableRefObject<Record<string, AbortController>>;
    handleExecuteQuery: (queryId: string) => Promise<void>;
    executeQuery: (queryId: string) => Promise<void>;
    handleRollback: (queryId: string) => Promise<void>;
    handlePageChange: (queryId: string, page: number) => Promise<void>;
    handleExportData: (queryId: string, format: 'csv' | 'json') => void;
    handleExportVisualization: (queryId: string) => Promise<void>;
}

export function useQueryOperations({
    chatId,
    message,
    streamId,
    queryStates,
    setQueryStates,
    queryTimeouts,
    setMessage,
    checkSSEConnection,
    onQueryUpdate,
    userId,
    userName,
}: UseQueryOperationsParams): QueryOperationsReturn {
    const [queryResults, setQueryResults] = useState<Record<string, QueryResultState>>({});
    const pageDataCacheRef = useRef<Record<string, Record<number, PageData | undefined>>>({});
    const abortControllerRef = useRef<Record<string, AbortController>>({});

    const [expandedCells, setExpandedCells] = useState<Record<string, boolean>>({});
    const [dateColumns, setDateColumns] = useState<Record<string, boolean>>({});
    const [minimizedResultsRaw, setMinimizedResults] = useState<Record<string, boolean>>({});
    const [openDownloadMenu, setOpenDownloadMenu] = useState<string | null>(null);
    const [editingQueries, setEditingQueries] = useState<Record<string, boolean>>({});
    const [editedQueryTexts, setEditedQueryTexts] = useState<Record<string, string>>({});

    // ── Persist / restore minimized state ────────────────────────────────────
    useEffect(() => {
        try {
            const key = `minimize-result-state-${chatId}`;
            const storedJSON = localStorage.getItem(key);
            if (storedJSON) {
                const all = JSON.parse(storedJSON);
                const messageState = all[message.id];
                if (messageState) setMinimizedResults(messageState);
            }
        } catch { /* non-critical */ }
    }, [chatId, message.id]);

    const toggleResultMinimize = useCallback((queryId: string) => {
        const newState = !(minimizedResultsRaw[queryId] || false);
        const updated = { ...minimizedResultsRaw, [queryId]: newState };
        if (userId && userName) {
            analyticsService.trackResultMinimizeToggle(chatId, queryId, newState, userId, userName);
        }
        setMinimizedResults(updated);
        try {
            const key = `minimize-result-state-${chatId}`;
            const all = JSON.parse(localStorage.getItem(key) || '{}');
            all[message.id] = updated;
            localStorage.setItem(key, JSON.stringify(all));
        } catch { /* non-critical */ }
    }, [chatId, message.id, minimizedResultsRaw, userId, userName]);

    // ── Initialise queryResults from message.queries ──────────────────────────
    useEffect(() => {
        if (!message.queries) return;
        const initialStates: Record<string, QueryResultState> = {};
        message.queries.forEach(query => {
            if (queryResults[query.id]) return; // already initialised
            const resultArray = parseResults(query.execution_result || []);
            const totalRecords = query.pagination?.total_records_count ?? resultArray.length;
            const pageData = resultArray.slice(0, DEFAULT_PAGE_SIZE);
            const cursorField = query.pagination?.cursor_field;
            const isCursorBased = !!cursorField;
            let nextCursor: string | null = null;
            let hasMore = false;
            if (isCursorBased && resultArray.length > 0) {
                const lastRecord = resultArray[resultArray.length - 1];
                if (lastRecord) nextCursor = extractCursorValue(lastRecord, cursorField!);
                const numericTotal = typeof totalRecords === 'number' ? totalRecords : null;
                hasMore = resultArray.length >= 50 || (numericTotal != null && numericTotal > Math.min(resultArray.length, DEFAULT_PAGE_SIZE));
            }

            if (!pageDataCacheRef.current[query.id]) pageDataCacheRef.current[query.id] = {};
            if (resultArray.length > 0) {
                pageDataCacheRef.current[query.id][1] = {
                    data: resultArray.slice(0, DEFAULT_PAGE_SIZE), totalRecords,
                    ...(isCursorBased ? { nextCursor, hasMore, cursor: null } : {}),
                };
                if (resultArray.length > DEFAULT_PAGE_SIZE) {
                    pageDataCacheRef.current[query.id][2] = {
                        data: resultArray.slice(DEFAULT_PAGE_SIZE, DEFAULT_PAGE_SIZE * 2), totalRecords,
                        ...(isCursorBased ? { nextCursor, hasMore, cursor: null } : {}),
                    };
                }
            }

            initialStates[query.id] = {
                data: pageData, loading: false, error: null,
                currentPage: 1, pageSize: DEFAULT_PAGE_SIZE,
                totalRecords, cursor: null, nextCursor, hasMore,
            };
        });
        if (Object.keys(initialStates).length > 0) {
            setQueryResults(prev => ({ ...prev, ...initialStates }));
        }
    }, [message.queries]); // eslint-disable-line react-hooks/exhaustive-deps

    // Initialise date columns once
    useEffect(() => {
        if (!message.queries || message.queries.length === 0) return;
        const newDateColumns: Record<string, boolean> = {};
        message.queries.forEach(query => {
            const result = query.execution_result || query.example_result;
            const data = parseResults(result);
            if (!data || data.length === 0 || !data[0]) return;
            Object.keys(data[0]).forEach(column => {
                for (let i = 0; i < Math.min(data.length, 5); i++) {
                    if (isDateString(data[i][column])) {
                        if (dateColumns[column] === undefined) newDateColumns[column] = true;
                        break;
                    }
                }
            });
        });
        if (Object.keys(newDateColumns).length > 0) {
            setDateColumns(prev => ({ ...prev, ...newDateColumns }));
        }
    }, [message.queries]); // eslint-disable-line react-hooks/exhaustive-deps

    // Initialise edited query texts
    useEffect(() => {
        if (!message.queries) return;
        setEditedQueryTexts(prev => {
            const next = { ...prev };
            message.queries!.forEach(q => {
                if (q && q.id && !prev[q.id]) {
                    next[q.id] = q.query || '';
                }
            });
            return next;
        });
    }, [message.queries]);

    // Close download dropdown on outside click
    useEffect(() => {
        const handler = (e: MouseEvent) => {
            if (openDownloadMenu && !(e.target as HTMLElement).closest('.download-dropdown-container')) {
                setOpenDownloadMenu(null);
            }
        };
        document.addEventListener('mousedown', handler);
        return () => document.removeEventListener('mousedown', handler);
    }, [openDownloadMenu]);

    // ── Execute query ─────────────────────────────────────────────────────────
    const executeQuery = useCallback(async (queryId: string) => {
        const query = message.queries?.find(q => q.id === queryId);
        if (!query) return;

        if (queryTimeouts.current[queryId]) {
            clearTimeout(queryTimeouts.current[queryId]);
            delete queryTimeouts.current[queryId];
        }
        abortControllerRef.current[queryId] = new AbortController();
        onQueryUpdate(() => setQueryStates(prev => ({ ...prev, [queryId]: { isExecuting: true, isExample: false } })));

        try {
            await checkSSEConnection();
            const response = await chatService.executeQuery(
                chatId, message.id, queryId, streamId || '',
                abortControllerRef.current[queryId],
            );

            if (response?.success) {
                const fullData = parseResults(response.data.execution_result);
                const totalRecords = response.data.total_records_count;
                const isCursorQuery = query.pagination?.cursor_field != null;
                let derivedNextCursor: string | null = null;
                let derivedHasMore = false;
                if (isCursorQuery && fullData.length > 0) {
                    const lastRecord = fullData[fullData.length - 1];
                    derivedNextCursor = extractCursorValue(lastRecord, query.pagination!.cursor_field!);
                    derivedHasMore = fullData.length > DEFAULT_PAGE_SIZE ||
                        (totalRecords != null && totalRecords > DEFAULT_PAGE_SIZE);
                }

                const pageData = isCursorQuery
                    ? fullData.slice(0, DEFAULT_PAGE_SIZE)
                    : sliceIntoPages(fullData, DEFAULT_PAGE_SIZE, 1);

                if (!pageDataCacheRef.current[queryId]) pageDataCacheRef.current[queryId] = {};
                if (isCursorQuery) {
                    pageDataCacheRef.current[queryId][1] = { data: pageData, totalRecords, nextCursor: derivedNextCursor, hasMore: derivedHasMore, cursor: null };
                    if (fullData.length > DEFAULT_PAGE_SIZE) {
                        pageDataCacheRef.current[queryId][2] = {
                            data: fullData.slice(DEFAULT_PAGE_SIZE, DEFAULT_PAGE_SIZE * 2), totalRecords,
                            nextCursor: derivedNextCursor,
                            hasMore: fullData.length >= DEFAULT_PAGE_SIZE * 2,
                            cursor: null,
                        };
                    }
                } else {
                    pageDataCacheRef.current[queryId][1] = { data: sliceIntoPages(fullData, DEFAULT_PAGE_SIZE, 1), totalRecords };
                    pageDataCacheRef.current[queryId][2] = { data: sliceIntoPages(fullData, DEFAULT_PAGE_SIZE, 2), totalRecords };
                }

                onQueryUpdate(() => {
                    setMessage({
                        ...message,
                        queries: message.queries?.map(q => q.id === queryId ? {
                            ...q,
                            is_executed: response.data.is_executed,
                            is_rolled_back: response.data.is_rolled_back,
                            execution_result: (!response.data.execution_result || (typeof response.data.execution_result === 'object' && Object.keys(response.data.execution_result).length === 0)) ? null : response.data.execution_result,
                            execution_time: response.data.execution_time,
                            action_at: response.data.action_at,
                            error: response.data.error,
                            total_records_count: response.data.total_records_count,
                            pagination: { ...q.pagination, total_records_count: response.data.total_records_count },
                        } : q),
                        action_buttons: response.data.action_buttons || [],
                    });
                    setQueryResults(prev => ({
                        ...prev,
                        [queryId]: {
                            ...prev[queryId],
                            data: pageData, loading: false, error: null,
                            currentPage: 1, pageSize: DEFAULT_PAGE_SIZE,
                            totalRecords, cursor: null, nextCursor: derivedNextCursor, hasMore: derivedHasMore,
                        },
                    }));
                });

                toast('Query executed!', { ...toastStyle, icon: '✅' });
            }
        } catch (error: any) {
            if (error.name !== 'AbortError') {
                toast.error('Query execution failed: ' + error);
            }
        } finally {
            onQueryUpdate(() => setQueryStates(prev => ({
                ...prev,
                [queryId]: { isExecuting: false, isExample: !query.is_executed },
            })));
            delete abortControllerRef.current[queryId];
        }
    }, [chatId, message, streamId, queryTimeouts, setQueryStates, setMessage, checkSSEConnection, onQueryUpdate]);

    const handleExecuteQuery = useCallback(async (queryId: string) => {
        const query = message.queries?.find(q => q.id === queryId);
        if (!query) return;
        if (userId && userName) analyticsService.trackQueryExecuteClick(chatId, queryId, userId, userName);
        // Critical confirm is handled by the caller (QueryBlock) before reaching here
        await executeQuery(queryId);
    }, [message.queries, executeQuery, chatId, userId, userName]);

    // ── Rollback ──────────────────────────────────────────────────────────────
    const handleRollback = useCallback(async (queryId: string) => {
        const queryIndex = message.queries?.findIndex(q => q.id === queryId) ?? -1;
        if (queryIndex === -1) return;

        abortControllerRef.current[queryId] = new AbortController();
        onQueryUpdate(() => setQueryStates(prev => ({ ...prev, [queryId]: { isExecuting: true, isExample: true } })));

        try {
            await checkSSEConnection();
            const response = await chatService.rollbackQuery(chatId, message.id, queryId, streamId || '', abortControllerRef.current[queryId]);

            if (response?.success) {
                const updatedMessage = {
                    ...message,
                    queries: message.queries?.map(q => q.id === queryId ? {
                        ...q,
                        is_executed: true,
                        is_rolled_back: response.data?.is_rolled_back,
                        execution_result: response.data?.execution_result,
                        execution_time: response.data?.execution_time,
                        action_at: response.data?.action_at,
                        error: response.data?.error,
                    } : q),
                    action_buttons: response.data?.action_buttons || [],
                };
                setMessage(updatedMessage);

                if (response.data?.execution_result) {
                    const execResult = response.data.execution_result;
                    let total = 1;
                    if (Array.isArray(execResult)) total = execResult.length;
                    else if (execResult && typeof execResult === 'object' && 'results' in execResult && Array.isArray(execResult.results)) total = execResult.results.length;

                    setQueryResults(prev => ({
                        ...prev,
                        [queryId]: { ...prev[queryId], data: execResult, loading: false, error: null, currentPage: 1, totalRecords: total, cursor: null, nextCursor: null, hasMore: false },
                    }));
                    if (!pageDataCacheRef.current[queryId]) pageDataCacheRef.current[queryId] = {};
                    pageDataCacheRef.current[queryId][1] = { data: execResult, totalRecords: 1 };
                }

                toast('Changes reverted', { ...toastStyle, icon: '↺' });
                onQueryUpdate(() => {
                    if (message.queries?.[queryIndex]) {
                        message.queries[queryIndex].is_rolled_back = response.data?.is_rolled_back;
                        message.queries[queryIndex].execution_time = response.data?.execution_time;
                        message.queries[queryIndex].error = response.data?.error;
                        message.queries[queryIndex].execution_result = response.data?.execution_result;
                    }
                });
            }
        } catch (error: any) {
            toast.error(error.message);
        } finally {
            onQueryUpdate(() => setQueryStates(prev => ({ ...prev, [queryId]: { isExecuting: false, isExample: true } })));
            delete abortControllerRef.current[queryId];
        }
    }, [chatId, message, streamId, setQueryStates, setMessage, checkSSEConnection, onQueryUpdate]);

    // ── Page change ───────────────────────────────────────────────────────────
    const handlePageChange = useCallback(async (queryId: string, page: number) => {
        const query = message.queries?.find(q => q.id === queryId);
        if (!query) return;

        const state = queryResults[queryId];
        const useCursorPagination = query.pagination?.cursor_field != null;

        if (!pageDataCacheRef.current[queryId]) pageDataCacheRef.current[queryId] = {};

        setQueryResults(prev => ({ ...prev, [queryId]: { ...prev[queryId], loading: true, error: null } }));

        try {
            if (useCursorPagination) {
                const cachedData = pageDataCacheRef.current[queryId][page];
                if (cachedData) {
                    onQueryUpdate(() => setQueryResults(prev => ({
                        ...prev,
                        [queryId]: {
                            ...prev[queryId], loading: false, currentPage: page,
                            data: cachedData.data, error: null, totalRecords: cachedData.totalRecords,
                            nextCursor: cachedData.nextCursor ?? null, hasMore: cachedData.hasMore ?? false, cursor: cachedData.cursor ?? null,
                        } satisfies QueryResultState,
                    })));
                    return;
                }

                if (page < state.currentPage) {
                    // Going backward without cache — nothing to do (should not happen with caching in place)
                    setQueryResults(prev => ({ ...prev, [queryId]: { ...prev[queryId], loading: false } }));
                    return;
                }

                const response = await axios.post(
                    `${import.meta.env.VITE_API_URL}/chats/${chatId}/queries/results`,
                    { message_id: message.id, query_id: queryId, stream_id: streamId, cursor: state.nextCursor },
                );

                const responseData = response.data.data;
                const fullBatch = parseResults(responseData.execution_result);
                const pageData = fullBatch.slice(0, DEFAULT_PAGE_SIZE);
                const nextPageData = fullBatch.length > DEFAULT_PAGE_SIZE ? fullBatch.slice(DEFAULT_PAGE_SIZE) : null;
                const totalRecords = responseData.total_records_count || state.totalRecords;

                let batchEndCursor: string | null = responseData.next_cursor || null;
                if (!batchEndCursor && fullBatch.length > 0 && query.pagination?.cursor_field) {
                    const lastRecord = fullBatch[fullBatch.length - 1];
                    if (lastRecord) batchEndCursor = extractCursorValue(lastRecord, query.pagination.cursor_field);
                }

                const hasMoreBeyondBatch = responseData.has_more ||
                    (totalRecords != null && totalRecords > 0 && (page + 1) * state.pageSize < totalRecords);
                const hasMoreForCurrentPage = fullBatch.length > 0 && (nextPageData !== null || hasMoreBeyondBatch);

                pageDataCacheRef.current[queryId][page] = {
                    data: pageData, totalRecords, nextCursor: null, hasMore: hasMoreForCurrentPage, cursor: state.nextCursor,
                };
                if (nextPageData) {
                    pageDataCacheRef.current[queryId][page + 1] = {
                        data: nextPageData, totalRecords, nextCursor: batchEndCursor, hasMore: hasMoreBeyondBatch, cursor: state.nextCursor,
                    };
                }
                const prevPage = page - 1;
                const prevCached = pageDataCacheRef.current[queryId][prevPage];
                if (prevPage >= 1 && prevCached) {
                    pageDataCacheRef.current[queryId][prevPage] = { ...prevCached, nextCursor: state.nextCursor, hasMore: true };
                }

                onQueryUpdate(() => setQueryResults(prev => ({
                    ...prev,
                    [queryId]: {
                        ...prev[queryId], loading: false, currentPage: page, data: pageData, error: null,
                        totalRecords, nextCursor: null, hasMore: hasMoreForCurrentPage, cursor: state.nextCursor ?? null,
                    } satisfies QueryResultState,
                })));
                return;
            }

            // ── Offset-based pagination ──────────────────────────────────────
            const newOffset = (page - 1) * state.pageSize;

            if (newOffset < 50 && query.execution_result) {
                const resultArray = parseResults(query.execution_result);
                const total = query.pagination?.total_records_count || resultArray.length;
                const startIndex = (page - 1) * state.pageSize;
                const pd = resultArray.slice(startIndex, startIndex + state.pageSize);
                pageDataCacheRef.current[queryId][page] = { data: pd, totalRecords: total };
                onQueryUpdate(() => setQueryResults(prev => ({
                    ...prev,
                    [queryId]: { ...prev[queryId], loading: false, currentPage: page, data: pd, error: null, totalRecords: total, cursor: null, nextCursor: null, hasMore: false },
                })));
                return;
            }

            const cached = pageDataCacheRef.current[queryId][page];
            if (cached) {
                onQueryUpdate(() => setQueryResults(prev => ({
                    ...prev,
                    [queryId]: { ...prev[queryId], loading: false, currentPage: page, data: cached.data, error: null, totalRecords: cached.totalRecords, cursor: null, nextCursor: null, hasMore: false },
                })));
                return;
            }

            const apiPage = Math.ceil(page / 2);
            const response = await axios.post(
                `${import.meta.env.VITE_API_URL}/chats/${chatId}/queries/results`,
                { message_id: message.id, query_id: queryId, stream_id: streamId, offset: (apiPage - 1) * 50 },
            );
            const responseData = response.data.data;
            const fullData = parseResults(responseData.execution_result);
            const totalRecords = responseData.total_records_count;
            const pd = sliceIntoPages(fullData, state.pageSize, page % 2);
            const base = Math.floor((page - 1) / 2) * 2 + 1;
            pageDataCacheRef.current[queryId][base] = { data: sliceIntoPages(fullData, state.pageSize, 1), totalRecords };
            pageDataCacheRef.current[queryId][base + 1] = { data: sliceIntoPages(fullData, state.pageSize, 2), totalRecords };

            onQueryUpdate(() => setQueryResults(prev => ({
                ...prev,
                [queryId]: { ...prev[queryId], loading: false, currentPage: page, data: pd, error: null, totalRecords, cursor: null, nextCursor: null, hasMore: false },
            })));

        } catch (error: any) {
            setQueryResults(prev => ({
                ...prev,
                [queryId]: { ...prev[queryId], loading: false, error: error.response?.data?.error || 'Failed to fetch results', data: prev[queryId].data },
            }));
        }
    }, [chatId, message, streamId, queryResults, onQueryUpdate]);

    // ── Export ────────────────────────────────────────────────────────────────
    const handleExportData = useCallback((queryId: string, format: 'csv' | 'json') => {
        const query = message.queries?.find(q => q.id === queryId);
        if (!query) return;

        const cachedPages = pageDataCacheRef.current[queryId] || {};
        const pageNumbers = Object.keys(cachedPages).map(Number).sort((a, b) => a - b);
        let allData: any[] = [];
        if (pageNumbers.length > 0) {
            pageNumbers.forEach(pn => { allData = [...allData, ...(cachedPages[pn]?.data || [])]; });
        } else {
            allData = queryResults[queryId]?.data || parseResults(query.is_executed ? query.execution_result : query.example_result);
        }

        const seen = new Set<string>();
        allData = allData.filter(item => {
            const key = JSON.stringify(item);
            if (seen.has(key)) return false;
            seen.add(key);
            return true;
        });

        if (!allData.length) { toast.error('No data available to export', toastStyle); return; }

        try {
            let content = '';
            let fileName = `query-${queryId}-export`;
            let mimeType = '';

            if (format === 'json') {
                content = JSON.stringify(allData, null, 2);
                fileName += '.json'; mimeType = 'application/json';
            } else {
                const headers = Object.keys(allData[0]);
                content = headers.join(',') + '\n';
                allData.forEach((row: any) => {
                    content += headers.map(h => {
                        const v = row[h];
                        if (v === null || v === undefined) return '';
                        if (typeof v === 'object') return JSON.stringify(v).replace(/"/g, '""');
                        return typeof v === 'string' ? `"${v.replace(/"/g, '""')}"` : v;
                    }).join(',') + '\n';
                });
                fileName += '.csv'; mimeType = 'text/csv';
            }

            const blob = new Blob([content], { type: mimeType });
            const url = URL.createObjectURL(blob);
            const link = document.createElement('a');
            link.href = url; link.download = fileName;
            document.body.appendChild(link); link.click();
            setTimeout(() => { document.body.removeChild(link); URL.revokeObjectURL(url); }, 100);

            if (userId && userName) analyticsService.trackDataExport(chatId, queryId, format, allData.length, userId, userName);
            toast(`Exported ${allData.length} records as ${format.toUpperCase()}`, { ...toastStyle, icon: '📥' });
        } catch (error) {
            toast.error(`Failed to export data: ${error}`, toastStyle);
        }
    }, [chatId, message.queries, queryResults, userId, userName]);

    const handleExportVisualization = useCallback(async (queryId: string) => {
        try {
            const el = document.querySelector(`[data-query-id="${queryId}"] .recharts-wrapper`);
            if (!el) { toast.error('Visualization not found or loaded', toastStyle); return; }
            const { toPng } = await import('html-to-image');
            toast.loading('Exporting visualization...', toastStyle);
            const dataUrl = await toPng(el as HTMLElement, { quality: 1.0, pixelRatio: 2, backgroundColor: '#1f2937' });
            const link = document.createElement('a');
            link.download = `visualization-query-${queryId}-${new Date().toISOString().split('T')[0]}.png`;
            link.href = dataUrl; link.click();
            if (userId && userName) analyticsService.trackDataExport(chatId, queryId, 'png-visualization', 1, userId, userName);
            toast.dismiss();
            toast.success('Visualization exported successfully!', { ...toastStyle, icon: '📊' });
        } catch (error) {
            toast.dismiss();
            toast.error('Failed to export visualization', toastStyle);
        }
    }, [chatId, userId, userName]);

    return {
        queryResults, setQueryResults,
        pageDataCacheRef,
        expandedCells, setExpandedCells,
        dateColumns, setDateColumns,
        minimizedResults: minimizedResultsRaw,
        toggleResultMinimize,
        openDownloadMenu, setOpenDownloadMenu,
        editingQueries, setEditingQueries,
        editedQueryTexts, setEditedQueryTexts,
        abortControllerRef,
        handleExecuteQuery,
        executeQuery,
        handleRollback,
        handlePageChange,
        handleExportData,
        handleExportVisualization,
    };
}
