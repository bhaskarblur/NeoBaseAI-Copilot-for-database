import axios from 'axios';
import { AlertCircle, ArrowLeft, ArrowRight, Braces, Clock, Copy, History, Loader, Pencil, Play, RefreshCcw, Send, Table, X, XCircle } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import toast from 'react-hot-toast';
import { useStream } from '../../contexts/StreamContext';
import chatService from '../../services/chatService';
import ConfirmationModal from '../modals/ConfirmationModal';
import RollbackConfirmationModal from '../modals/RollbackConfirmationModal';
import LoadingSteps from './LoadingSteps';
import { Message, QueryResult } from './types';
import MarkdownRenderer from './MarkdownRenderer';
import { formatActionAt } from '../../utils/message';
import analyticsService from '../../services/analyticsService';
import { highlightSearchText } from '../../utils/highlightSearch';

interface QueryState {
    isExecuting: boolean;
    isExample: boolean;
}

interface QueryResultState {
    data: any;
    loading: boolean;
    error: string | null;
    currentPage: number;
    pageSize: number;
    totalRecords: number | null;
}

interface PageData {
    data: any[];
    totalRecords: number;
}

interface MessageTileProps {
    chatId: string;
    message: Message;
    setMessage: (message: Message) => void;
    checkSSEConnection: () => Promise<void>;
    onEdit?: (id: string) => void;
    editingMessageId: string | null;
    editInput: string;
    setEditInput: (input: string) => void;
    onSaveEdit: (id: string, content: string) => void;
    onCancelEdit: () => void;
    queryStates: Record<string, QueryState>;
    setQueryStates: React.Dispatch<React.SetStateAction<Record<string, QueryState>>>;
    queryTimeouts: React.MutableRefObject<Record<string, NodeJS.Timeout>>;
    isFirstMessage?: boolean;
    onQueryUpdate: (callback: () => void) => void;
    onEditQuery: (id: string, queryId: string, query: string) => void;
    searchQuery?: string;
    isSearchResult?: boolean;
    isCurrentSearchResult?: boolean;
    searchResultRefs?: React.MutableRefObject<{ [key: string]: HTMLElement | null }>;
    buttonCallback?: (action: string) => void;
    userId?: string;
    userName?: string;
}

const toastStyle = {
    style: {
        background: '#000',
        color: '#fff',
        border: '4px solid #000',
        borderRadius: '12px',
        boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
        padding: '12px 24px',
        fontSize: '14px',
        fontWeight: '500',
    },
    position: 'bottom-center' as const,
    duration: 2000,
};

const formatMessageTime = (message: Message) => {
    // Use updated_at if available and message is edited, otherwise use created_at
    const dateString = message.is_edited && message.updated_at ? message.updated_at : message.created_at;
    const date = new Date(dateString);
    return date.toLocaleTimeString('en-US', {
        hour: 'numeric',
        minute: 'numeric',
        hour12: true
    });
};

const DEFAULT_PAGE_SIZE = 25; // Set page size to 25 items per page

export default function MessageTile({
    chatId,
    message,
    setMessage,
    onEdit,
    editingMessageId,
    editInput,
    setEditInput,
    onSaveEdit,
    onCancelEdit,
    queryStates,
    setQueryStates,
    queryTimeouts,
    checkSSEConnection,
    isFirstMessage,
    onQueryUpdate,
    onEditQuery,
    buttonCallback,
    userId,
    userName,
    searchQuery,
    isSearchResult,
    isCurrentSearchResult,
    searchResultRefs,
}: MessageTileProps) {
    const { streamId } = useStream();
    const [viewModes, setViewModes] = useState<Record<string, 'table' | 'json'>>({});
    const [showCriticalConfirm, setShowCriticalConfirm] = useState(false);
    const [queryToExecute, setQueryToExecute] = useState<string | null>(null);
    const [rollbackState, setRollbackState] = useState<{
        show: boolean;
        queryId: string | null;
    }>({ show: false, queryId: null });
    const [streamingQueryIndex, setStreamingQueryIndex] = useState<number>(-1);
    const [isDescriptionStreaming, setIsDescriptionStreaming] = useState(false);
    const [isQueryStreaming, setIsQueryStreaming] = useState(false);
    const [currentDescription, setCurrentDescription] = useState('');
    const [currentQuery, setCurrentQuery] = useState('');
    const abortControllerRef = useRef<Record<string, AbortController>>({});
    const [queryResults, setQueryResults] = useState<Record<string, QueryResultState>>({});
    const pageDataCacheRef = useRef<Record<string, Record<number, PageData>>>({});
    const [editingQueries, setEditingQueries] = useState<Record<string, boolean>>({});
    const [editedQueryTexts, setEditedQueryTexts] = useState<Record<string, string>>({});
    // Add state for tracking which download dropdown is open
    const [openDownloadMenu, setOpenDownloadMenu] = useState<string | null>(null);
    // Add state for date format preference - initialize as empty object
    const [dateColumns, setDateColumns] = useState<Record<string, boolean>>({});
    // Store expanded state for nested JSON fields
    const expandedNodesRef = useRef<Record<string, boolean>>({});
    const [minimizedResults, setMinimizedResults] = useState<Record<string, boolean>>({});
    const [expandedCells, setExpandedCells] = useState<Record<string, boolean>>({});

    useEffect(() => {
        try {
            const key = `minimize-result-state-${chatId}`;
            const storedStateJSON = localStorage.getItem(key);
            if (storedStateJSON) {
                const allStoredStates = JSON.parse(storedStateJSON);
                const messageState = allStoredStates[message.id];
                if (messageState) {
                    setMinimizedResults(messageState);
                }
            }
        } catch (e) {
            console.error("Failed to parse minimized state from localStorage", e);
        }
    }, [chatId, message.id]);

    const toggleResultMinimize = (queryId: string) => {
        const newMinimizedState = !(minimizedResults[queryId] || false);
        const updatedMessageState = { ...minimizedResults, [queryId]: newMinimizedState };
        
        // Track result minimize toggle
        if (userId && userName) {
            analyticsService.trackResultMinimizeToggle(chatId, queryId, newMinimizedState, userId, userName);
        }
        
        setMinimizedResults(updatedMessageState);

        try {
            const key = `minimize-result-state-${chatId}`;
            const storedStateJSON = localStorage.getItem(key);
            const allStoredStates = storedStateJSON ? JSON.parse(storedStateJSON) : {};
            allStoredStates[message.id] = updatedMessageState;
            localStorage.setItem(key, JSON.stringify(allStoredStates));
        } catch (e) {
            console.error("Failed to save minimized state to localStorage", e);
        }
    };
    
    // Close dropdown when clicking outside
    useEffect(() => {
        const handleClickOutside = (event: MouseEvent) => {
            if (openDownloadMenu) {
                const target = event.target as HTMLElement;
                if (!target.closest('.download-dropdown-container')) {
                    setOpenDownloadMenu(null);
                }
            }
        };
        
        document.addEventListener('mousedown', handleClickOutside);
        return () => {
            document.removeEventListener('mousedown', handleClickOutside);
        };
    }, [openDownloadMenu]);

    // Ensure message is valid to prevent errors
    if (!message) {
        console.error('MessageTile received invalid message');
        return null;
    }

    useEffect(() => {
        const streamQueries = async () => {
            if (!message.queries || !message.is_streaming) return;

            // Set streaming index to 0 to start showing the first query
            setStreamingQueryIndex(0);
            
            // Just set the content immediately without streaming
            for (let i = 0; i < message.queries.length; i++) {
                const query = message.queries[i];
                if (!query || !query.id) continue;
                
                setStreamingQueryIndex(i);
                setCurrentDescription(query.description);
                setCurrentQuery(removeDuplicateQueries(query.query));

                // Initialize the edited query text for this query
                setEditedQueryTexts(prev => ({
                    ...prev,
                    [query.id]: removeDuplicateQueries(query.query || '')
                }));

                // Keep the existing query state management
                if (message.queries) {
                    message.queries[i].is_streaming = false;
                }
            }
        };

        streamQueries();
    }, [message.queries, message.is_streaming]);

    // Add a separate effect to handle when streaming is complete
    useEffect(() => {
        if (!message.is_streaming && message.queries && message.queries.length > 0) {
            // Reset streaming index when streaming is complete
            setStreamingQueryIndex(-1);
        }
    }, [message.is_streaming, message.queries]);

    // Initialize editedQueryTexts when queries change
    useEffect(() => {
        if (message.queries) {
            const initialQueryTexts: Record<string, string> = {};
            message.queries.forEach(query => {
                if (query && query.id) {
                    initialQueryTexts[query.id] = removeDuplicateQueries(query.query || '');
                }
            });
            if (Object.keys(initialQueryTexts).length > 0) {
                setEditedQueryTexts(prev => {
                    const newState = { ...prev };
                    // Only set values that don't already exist or have changed
                    Object.entries(initialQueryTexts).forEach(([id, text]) => {
                        if (!prev[id]) {
                            newState[id] = text;
                        }
                    });
                    return newState;
                });
            }
        }
    }, [message.queries]);

    useEffect(() => {
        if (message.queries) {
            const initialStates: Record<string, QueryResultState> = {};
            message.queries.forEach(query => {
                if (!queryResults[query.id]) {
                    const resultArray = parseResults(query.execution_result || []);
                    const totalRecords = query.pagination?.total_records_count || resultArray.length;

                    // For initial data, always show first page (25 records)
                    const pageData = resultArray.slice(0, 25);

                    // Cache both pages from initial 50 records if available
                    if (!pageDataCacheRef.current[query.id]) {
                        pageDataCacheRef.current[query.id] = {};
                    }

                    if (resultArray.length > 0) {
                        // Cache first page (1-25)
                        pageDataCacheRef.current[query.id][1] = {
                            data: resultArray.slice(0, 25),
                            totalRecords
                        };

                        // Cache second page (26-50) if it exists
                        if (resultArray.length > 25) {
                            pageDataCacheRef.current[query.id][2] = {
                                data: resultArray.slice(25, 50),
                                totalRecords
                            };
                        }
                    }

                    initialStates[query.id] = {
                        data: pageData,
                        loading: false,
                        error: null,
                        currentPage: 1,
                        pageSize: 25,
                        totalRecords: totalRecords
                    };
                }
            });
            if (Object.keys(initialStates).length > 0) {
                setQueryResults(prev => ({ ...prev, ...initialStates }));
            }
        }
    }, [message.queries]);

    const handleCopyToClipboard = (text: string, context?: { type: 'message' | 'query' | 'result', messageId?: string, queryId?: string }) => {
        navigator.clipboard.writeText(text);
        toast('Copied to clipboard!', {
            ...toastStyle,
            icon: '📋',
        });

        // Track copy action
        if (userId && userName && context) {
            if (context.type === 'message' && context.messageId) {
                analyticsService.trackMessageCopyClick(chatId, context.messageId, message.type, userId, userName);
            } else if (context.type === 'query' && context.queryId) {
                analyticsService.trackQueryCopyClick(chatId, context.queryId, userId, userName);
            } else if (context.type === 'result' && context.queryId) {
                analyticsService.trackResultCopyClick(chatId, context.queryId, userId, userName);
            }
        }
    };

    const handleExecuteQuery = async (queryId: string) => {
        const query = message.queries?.find(q => q.id === queryId);
        if (!query) return;

        // Track query execute click
        if (userId && userName) {
            analyticsService.trackQueryExecuteClick(chatId, queryId, userId, userName);
        }

        if (query.is_critical) {
            setQueryToExecute(queryId);
            setShowCriticalConfirm(true);
            return;
        }
        executeQuery(queryId);
    };

    const executeQuery = async (queryId: string) => {
        const query = message.queries?.find(q => q.id === queryId);
        if (!query) return;

        // Clear any existing timeout
        if (queryTimeouts.current[queryId]) {
            clearTimeout(queryTimeouts.current[queryId]);
            delete queryTimeouts.current[queryId];
        }

        // Create new AbortController for this query
        abortControllerRef.current[queryId] = new AbortController();

        onQueryUpdate(() => {
            setQueryStates(prev => ({
                ...prev,
                [queryId]: { isExecuting: true, isExample: false }
            }));
        });

        try {
            await checkSSEConnection();
            const response = await chatService.executeQuery(
                chatId,
                message.id,
                queryId,
                streamId || '',
                abortControllerRef.current[queryId]
            );

            console.log('executeQuery response', response?.success);
            if (response?.success) {

                const fullData = parseResults(response.data.execution_result);
                const totalRecords = response.data.total_records_count;

                // Slice the data into pages of 25
                const pageData = sliceIntoPages(fullData, DEFAULT_PAGE_SIZE, 1);

                // Cache both pages from the API response
                const basePage = Math.floor((1 - 1) / 2) * 2 + 1;
                const firstHalf = sliceIntoPages(fullData, DEFAULT_PAGE_SIZE, 1);
                const secondHalf = sliceIntoPages(fullData, DEFAULT_PAGE_SIZE, 2);

                pageDataCacheRef.current[queryId][basePage] = {
                    data: firstHalf,
                    totalRecords
                };
                pageDataCacheRef.current[queryId][basePage + 1] = {
                    data: secondHalf,
                    totalRecords
                };
                onQueryUpdate(() => {
                    if (!response.data.execution_result) {
                        console.log('response.data.execution_result is empty', response.data.execution_result);
                    }
                    setMessage({
                        ...message,
                        queries: message.queries?.map(q => q.id === queryId ? {
                            ...q,
                            is_executed: response.data.is_executed,
                            is_rolled_back: response.data.is_rolled_back,
                            execution_result: !response.data.execution_result || 
                                (typeof response.data.execution_result === 'object' && 
                                Object.keys(response.data.execution_result).length === 0) 
                                ? null 
                                : response.data.execution_result,
                            execution_time: response.data.execution_time,
                            action_at: response.data.action_at,
                            error: response.data.error,
                            total_records_count: response.data.total_records_count,
                            pagination: {
                                ...q.pagination,
                                total_records_count: response.data.total_records_count,
                            }
                        } : q),
                        // Always update action buttons, setting to empty array if not in response
                        action_buttons: response.data.action_buttons || []
                    });

                    setQueryResults(prev => ({
                        ...prev,
                        [queryId]: {
                            ...prev[queryId],
                            data: pageData,
                            loading: false,
                            error: null,
                            currentPage: 1,
                            pageSize: DEFAULT_PAGE_SIZE,
                            totalRecords: totalRecords
                        }
                    }));
                });

                toast('Query executed!', {
                    ...toastStyle,
                    icon: '✅',
                });
            }
        } catch (error: any) {
            // Only show error if not aborted
            if (error.name !== 'AbortError') {
                console.log('error', error.message);
                toast.error("Query execution failed: " + error);
            }
        } finally {
            onQueryUpdate(() => {
                setQueryStates(prev => ({
                    ...prev,
                    [queryId]: { isExecuting: false, isExample: !query.is_executed }
                }));
            });
            // Clean up abort controller
            delete abortControllerRef.current[queryId];
        }
    };

    const handleRollback = async (queryId: string) => {
        const queryIndex = message.queries?.findIndex(q => q.id === queryId) ?? -1;
        if (queryIndex === -1) return;

        try {
            // Close the dialog immediately
            setRollbackState({ show: false, queryId: null });
            
            // Create new AbortController for this query
            abortControllerRef.current[queryId] = new AbortController();
            onQueryUpdate(() => {
                setQueryStates(prev => ({
                    ...prev,
                    [queryId]: { isExecuting: true, isExample: true }
                }));
            });

            await checkSSEConnection();
            const response = await chatService.rollbackQuery(chatId, message.id, queryId, streamId || '', abortControllerRef.current[queryId]);
            console.log('rolledBack', response);

            if (response?.success) {
                // Update the message with the new rolled back data
                const updatedMessage = {
                    ...message,
                    queries: message.queries?.map(q => q.id === queryId ? {
                        ...q,
                        is_executed: true,
                        is_rolled_back: response?.data?.is_rolled_back,
                        execution_result: response?.data?.execution_result,
                        execution_time: response?.data?.execution_time,
                        action_at: response?.data?.action_at,
                        error: response?.data?.error,
                    } : q),
                    // Always update action buttons, setting to empty array if not in response
                    action_buttons: response?.data?.action_buttons || []
                };
                
                setMessage(updatedMessage);
                
                // Also update the query results state to show the new data
                if (response?.data?.execution_result) {
                    // Store the raw execution result
                    const executionResult = response.data.execution_result;
                    
                    // Calculate total records based on the type of execution result
                    let totalRecords = 1;
                    if (Array.isArray(executionResult)) {
                        totalRecords = executionResult.length;
                    } else if (executionResult && typeof executionResult === 'object') {
                        if ('results' in executionResult && Array.isArray((executionResult as any).results)) {
                            totalRecords = (executionResult as any).results.length;
                        }
                    }
                    
                    // Update the query results state with the new data
                    setQueryResults(prev => ({
                        ...prev,
                        [queryId]: {
                            ...prev[queryId],
                            data: executionResult,
                            loading: false,
                            error: null,
                            currentPage: 1,
                            totalRecords: totalRecords
                        }
                    }));
                    
                    // Update the page data cache
                    if (!pageDataCacheRef.current[queryId]) {
                        pageDataCacheRef.current[queryId] = {};
                    }
                    
                    pageDataCacheRef.current[queryId][1] = {
                        data: executionResult,
                        totalRecords: totalRecords
                    };
                }
                
                toast('Changes reverted', {
                    ...toastStyle,
                    icon: '↺',
                });
                
                onQueryUpdate(() => {
                    if (message.queries) {
                        message.queries[queryIndex].is_rolled_back = response?.data?.is_rolled_back;
                        message.queries[queryIndex].execution_time = response?.data?.execution_time;
                        message.queries[queryIndex].error = response?.data?.error;
                        message.queries[queryIndex].execution_result = response?.data?.execution_result;
                    }
                });
            }
        } catch (error: any) {
            toast.error(error.message);
        } finally {
            onQueryUpdate(() => {
                setQueryStates(prev => ({
                    ...prev,
                    [queryId]: { isExecuting: false, isExample: true, isRolledBack: false }
                }));
            });
            delete abortControllerRef.current[queryId];
        }
    };

    // Format date string in a user-friendly way
    const formatDateString = (dateStr: string, useFriendlyFormat: boolean): string => {
        if (!useFriendlyFormat) return dateStr; // Return raw ISO format
        
        try {
            const date = new Date(dateStr);
            // Check if date is valid
            if (isNaN(date.getTime())) return dateStr;
            
            // Format as "Apr 26, 2025, 06:46 PM"
            return date.toLocaleString('en-US', {
                month: 'short',
                day: 'numeric',
                year: 'numeric',
                hour: 'numeric',
                minute: '2-digit',
                hour12: true
            });
        } catch (e) {
            // Fallback to original string if parsing fails
            return dateStr;
        }
    };
    
    // Small toggle component for date format switching
    const DateFormatToggle = ({ column, className = "" }: { column: string, className?: string }) => {
        return (
            <button 
                onClick={(e) => {
                    e.stopPropagation();
                    e.preventDefault();
                    setDateColumns(prev => ({
                        ...prev,
                        [column]: !prev[column]
                    }));
                }}
                className={`inline-flex items-center text-xs px-1.5 py-0.5 ml-2 bg-gray-700 hover:bg-gray-600 rounded-sm text-gray-300 ${className}`}
                title="Toggle date format"
            >
                <svg xmlns="http://www.w3.org/2000/svg" width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="mr-1">
                    <rect x="3" y="4" width="18" height="18" rx="2" ry="2"></rect>
                    <line x1="16" y1="2" x2="16" y2="6"></line>
                    <line x1="8" y1="2" x2="8" y2="6"></line>
                    <line x1="3" y1="10" x2="21" y2="10"></line>
                </svg>
                {dateColumns[column] ? "ISO" : "Human"}
            </button>
        );
    };

    // Check if a value is a date string
    const isDateString = (value: any): boolean => {
        return typeof value === 'string' && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/.test(value);
    };

    // Initialize date columns to true (human format) by default
    // This needs to be at component level, not inside the render function
    useEffect(() => {
        const initializeDateColumns = () => {
            if (!message.queries || message.queries.length === 0) return;
            
            // Find all date columns across all queries
            const newDateColumns: Record<string, boolean> = {};
            
            message.queries.forEach(query => {
                if (!query.execution_result && !query.example_result) return;
                
                const result = query.execution_result || query.example_result;
                const data = parseResults(result);
                if (!data || data.length === 0 || !data[0]) return;
                
                const columns = Object.keys(data[0]);
                columns.forEach(column => {
                    // Check if this is a date column by examining first few rows
                    for (let i = 0; i < Math.min(data.length, 5); i++) {
                        if (isDateString(data[i][column])) {
                            // Only set if not already set by user
                            if (dateColumns[column] === undefined) {
                                newDateColumns[column] = true; // Default to human format
                            }
                            break;
                        }
                    }
                });
            });
            
            // Set all the new date columns if any found
            if (Object.keys(newDateColumns).length > 0) {
                setDateColumns(prev => ({
                    ...prev,
                    ...newDateColumns
                }));
            }
        };
        
        initializeDateColumns();
    }, [message.queries]); // Only re-run when queries change

    // Component to render nested JSON data in a collapsible/expandable way
    const NestedJsonCell = ({ data }: { data: any }) => {
        // Generate a stable ID for this field to track its expanded state
        const getFieldId = (): string => {
            let idString = '';
            if (typeof data === 'object' && data !== null) {
                // Try to use id field if available
                if ('id' in data) {
                    idString = `obj-${data.id}`;
                } else if (Array.isArray(data)) {
                    // For arrays, use length and hash of first few items
                    idString = `arr-${data.length}-${JSON.stringify(data.slice(0, 2)).split('').reduce((a, b) => (a * 31 + b.charCodeAt(0)) & 0xFFFFFFFF, 0)}`;
                } else {
                    // For other objects, use hash of keys and some values
                    const keys = Object.keys(data).sort().join(',');
                    const firstFewValues = Object.keys(data).slice(0, 2).map(key => data[key]);
                    idString = `obj-${keys}-${JSON.stringify(firstFewValues).split('').reduce((a, b) => (a * 31 + b.charCodeAt(0)) & 0xFFFFFFFF, 0)}`;
                }
            }
            return `field-${idString.replace(/[^a-zA-Z0-9-]/g, '')}`;
        };
        
        const fieldId = getFieldId();
        const [isExpanded, setIsExpanded] = useState(() => {
            // Initialize from the ref if available
            return expandedNodesRef.current[fieldId] || false;
        });
        const expandButtonRef = useRef<HTMLDivElement>(null);
        const expandedContentRef = useRef<HTMLDivElement>(null);
        
        // Ensure expansion state persists across renders
        useEffect(() => {
            // Update the ref when state changes
            expandedNodesRef.current[fieldId] = isExpanded;
            
            // Use a data attribute on the DOM element for extra persistence
            if (expandButtonRef.current) {
                expandButtonRef.current.setAttribute('data-expanded', isExpanded ? 'true' : 'false');
            }
            
            // Set display style directly to ensure it stays
            if (expandedContentRef.current) {
                expandedContentRef.current.style.display = isExpanded ? 'block' : 'none';
            }
        }, [isExpanded, fieldId]);
        
        // When the component mounts, check for saved expansion state
        useEffect(() => {
            const savedExpanded = expandedNodesRef.current[fieldId];
            if (savedExpanded !== undefined && savedExpanded !== isExpanded) {
                setIsExpanded(savedExpanded);
            }
            
            // Also check the DOM attribute as a fallback
            if (expandButtonRef.current) {
                const domExpanded = expandButtonRef.current.getAttribute('data-expanded') === 'true';
                if (domExpanded !== isExpanded) {
                    setIsExpanded(domExpanded);
                }
            }
        }, [fieldId]);
        
        // Determine if the data is expandable (object or array with items)
        const isExpandable = 
            (typeof data === 'object' && data !== null) && 
            (Array.isArray(data) ? data.length > 0 : Object.keys(data).length > 0);
        
        // If not expandable, render as simple value
        if (!isExpandable) {
            if (data === null) return <span className="text-gray-400">null</span>;
            if (data === undefined) return <span className="text-gray-400">undefined</span>;
            if (typeof data === 'boolean') return <span className="text-purple-400">{String(data)}</span>;
            if (typeof data === 'number') return <span className="text-cyan-400">{data}</span>;
            if (typeof data === 'string') {
                // Check if it's a date string
                if (isDateString(data)) {
                    return <span className="text-yellow-300">{data}</span>;
                }
                return <span className="text-green-400">"{data}"</span>;
            }
            // Fallback
            return <span>{String(data)}</span>;
        }
        
        // Handle toggle with persistence
        const handleToggleClick = () => {
            const newExpandedState = !isExpanded;
            console.log('toggleExpand called, current state:', isExpanded, 'new state:', newExpandedState, 'fieldId:', fieldId);
            
            // Update both React state and the ref
            setIsExpanded(newExpandedState);
            expandedNodesRef.current[fieldId] = newExpandedState;
            
            // Also update DOM directly
            if (expandedContentRef.current) {
                expandedContentRef.current.style.display = newExpandedState ? 'block' : 'none';
            }
            if (expandButtonRef.current) {
                expandButtonRef.current.setAttribute('data-expanded', newExpandedState ? 'true' : 'false');
            }
        };
        
        const renderExpandedContent = () => {
            if (Array.isArray(data)) {
                return (
                    <div className="pl-4 mt-2 space-y-1 border-l-2 border-gray-700 pt-1">
                        {data.map((item, index) => (
                            <div key={index} className="mb-2">
                                <span className="text-gray-400 mr-1">[{index}]:</span>
                                <NestedJsonCell data={item} />
                            </div>
                        ))}
                    </div>
                );
            } else {
                return (
                    <div className="pl-4 mt-2 space-y-1 border-l-2 border-gray-700 pt-1">
                        {Object.entries(data).map(([key, value]) => (
                            <div key={key} className="mb-2">
                                <span className="text-gray-400 mr-1">{key}:</span>
                                <NestedJsonCell data={value} />
                            </div>
                        ))}
                    </div>
                );
            }
        };
        
        // Get a more user-friendly preview content
        const getPreviewContent = () => {
            if (Array.isArray(data)) {
                const itemCount = data.length;
                return `${itemCount} item${itemCount !== 1 ? 's' : ''} in list`;
            } else {
                // For objects, try to show a more descriptive preview
                const keys = Object.keys(data);
                const keyCount = keys.length;
                
                // Try to detect what kind of object this might be
                if ('id' in data && ('name' in data || 'title' in data)) {
                    const nameField = data.name || data.title;
                    return typeof nameField === 'string' 
                        ? `${nameField} (${keyCount} properties)` 
                        : `Details with ${keyCount} properties`;
                }
                
                // Show some of the keys as a preview
                const previewKeys = keys.slice(0, 2);
                if (previewKeys.length > 0) {
                    return `View: ${previewKeys.join(', ')}${keys.length > 2 ? '...' : ''}`;
                }
                
                return `${keyCount} propert${keyCount !== 1 ? 'ies' : 'y'}`;
            }
        };
        
        return (
            <div 
                className={`nested-json min-w-[160px] ${isExpanded ? 'mt-2' : ''}`} 
                style={{ position: 'relative', zIndex: 5 }}
                data-field-id={fieldId}
            >
                <div 
                    ref={expandButtonRef}
                    className="cursor-pointer flex items-center transition-colors"
                    tabIndex={0}
                    role="button"
                    aria-expanded={isExpanded}
                    data-expanded={isExpanded ? 'true' : 'false'}
                    onClick={(e) => {
                        e.preventDefault();
                        e.stopPropagation();
                        handleToggleClick();
                    }}
                    onKeyDown={(e) => {
                        if (e.key === 'Enter' || e.key === ' ') {
                            e.preventDefault();
                            handleToggleClick();
                        }
                    }}
                >
                    <span className="mr-2 text-white font-medium">
                        {isExpanded ? 
                            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="inline-block">
                                <path d="m18 15-6-6-6 6"/>
                            </svg> : 
                            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="inline-block">
                                <path d="m6 9 6 6 6-6"/>
                            </svg>
                        }
                    </span>
                    <span className="text-blue-400 font-medium">{getPreviewContent()}</span>
                </div>
                <div 
                    ref={expandedContentRef} 
                    style={{ display: isExpanded ? 'block' : 'none' }}
                    data-expanded-content={fieldId}
                >
                    {renderExpandedContent()}
                </div>
            </div>
        );
    };
    
    const toggleCellExpansion = (cellId: string) => {
        setExpandedCells(prev => ({ ...prev, [cellId]: !prev[cellId] }));
    };

    const renderCellValue = (value: any, column: string, rowId: string): JSX.Element => {
        const cellId = `${rowId}-${column}`;
        const isExpanded = expandedCells[cellId];

        if (value === null) {
            // Show "No Data found" in non-tech mode
            // Show "-" in non-tech mode for null values in table cells
            return message.non_tech_mode ? 
                <span className="text-gray-400">-</span> : 
                <span className="text-yellow-400">null</span>;
        }
        if (value === undefined) {
            // Show "-" in non-tech mode for undefined values in table cells
            return message.non_tech_mode ? 
                <span className="text-gray-400">-</span> : 
                <span className="text-yellow-400">undefined</span>;
        }

        if (typeof value === 'object' && value !== null) {
            return <NestedJsonCell data={value} />;
        }

        if (typeof value === 'number') {
            return <span className="text-cyan-400">{value}</span>;
        }

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
                const truncatedText = isExpanded ? value : value.substring(0, maxLength);    
                return (
                    <span onClick={() => toggleCellExpansion(cellId)} className="cursor-pointer">
                        <span className="text-green-400">"{truncatedText}</span>
                        {!isExpanded ? (
                             <span className="text-green-400">..<br></br><span className="text-cyan-400">Show More</span></span> )
                             : (
                                <span className="text-cyan-400"><br></br>Show Less</span>
                             )
                            }
                    </span>
                );
            }

            return <span className="text-green-400">"{value}"</span>;
        }

        if (typeof value === 'boolean') {
            return <span className="text-purple-400">{String(value)}</span>;
        }
        
        // Fallback
        return <span>{String(value)}</span>;
    };

    const renderTableView = (data: any[]) => {
        if (!data || data.length === 0) {
            // Show "No Data found" in non-tech mode
            const displayMessage = message.non_tech_mode ? "No Data found" : "No data to display";
            return <div className="text-gray-500">{displayMessage}</div>;
        }

        const columns = Object.keys(data[0]);
        
        // Detect date columns
        const dateColumnList = columns.filter(column => {
            // Check the first few rows to see if this column contains date strings
            for (let i = 0; i < Math.min(data.length, 5); i++) {
                if (isDateString(data[i][column])) {
                    return true;
                }
            }
            return false;
        });

        return (
            <div className="overflow-x-auto">
                <table className="w-full text-left border-collapse">
                    <thead>
                        <tr>
                            {columns.map(column => (
                                <th key={column} className="py-2 px-4 bg-gray-800 border-b border-gray-700 text-gray-300 font-mono">
                                    <div className="flex items-center">
                                        <span>{column}</span>
                                        {dateColumnList.includes(column) && (
                                            <DateFormatToggle column={column} />
                                        )}
                                    </div>
                                </th>
                            ))}
                        </tr>
                    </thead>
                    <tbody>
                        {data.map((row, i) => (
                            <tr key={i} className="border-b border-gray-700">
                                {columns.map(column => {
                                    const isComplexObject = 
                                        typeof row[column] === 'object' && 
                                        row[column] !== null && 
                                        Object.keys(row[column]).length > 0;
                                    
                                    const isTooLong = row[column] != null && row[column].length > 140;
                                    const isDateColumn = dateColumnList.includes(column);
                                    
                                    return (
                                        <td 
                                            key={column} 
                                            className={`py-2 px-4 ${isComplexObject ? 'min-w-[300px]' : ''} ${isDateColumn ? 'min-w-[200px] whitespace-nowrap' : ''} ${isTooLong ? 'min-w-[480px]' : ''}`}
                                        >
                                            {renderCellValue(row[column], column, row.id || i.toString())}
                                        </td>
                                    );
                                })}
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>
        );
    };

    const renderQueryResult = (result: any, queryId: string) => {
        if (!result) {
            return <div className="text-gray-500">No results available</div>;
        }

        const query = message.queries?.find(q => q.id === queryId);
        if (!query) return null;

        const state = queryResults[query.id];
        if (!state) return null;

        // Parse the data - handle both array and object with results property
        const parsedData = parseResults(state.data);
        const totalRecords = state.totalRecords || parsedData.length;
        const showPagination = totalRecords > state.pageSize;

        // Don't slice the data here since we're getting paginated data from API
        const currentPageData = parsedData;

        return (
            <div className="query-results">
                {totalRecords > 50 && (
                    <div className="text-gray-300 mb-4">
                        The result contains total <b className="text-yellow-500">{totalRecords}</b> records.
                    </div>
                )}

                {state.loading ? (
                    <div className="flex justify-center p-4">
                        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-white"></div>
                    </div>
                ) : (
                    <>
                        {state.error && (
                            <div className="text-red-500 py-2 mb-2">
                                Error in fetching results: {state.error}
                            </div>
                        )}

                        {(viewModes[queryId] || 'table') === 'table' ? (
                            currentPageData.length > 0 ? (
                                renderTableView(currentPageData)
                            ) : (
                                <div className="text-gray-500">{message.non_tech_mode ? "No Data found" : "No data to display"}</div>
                            )
                        ) : (
                            <pre className="overflow-x-auto whitespace-pre-wrap">
                                {renderColoredJson(currentPageData)}
                            </pre>
                        )}

                        {showPagination && (
                            <div className="flex justify-center mt-6">
                                <div className="flex items-center gap-4 bg-gray-800 rounded-lg p-1.5">
                                    <button
                                        onClick={() => handlePageChange(query.id, state.currentPage - 1)}
                                        disabled={state.currentPage === 1}
                                        className="
                                            flex items-center justify-center
                                            w-8 h-8
                                            rounded
                                            transition-colors
                                            disabled:opacity-40
                                            disabled:cursor-not-allowed
                                            enabled:hover:bg-gray-700
                                            enabled:active:bg-gray-600
                                        "
                                        title="Previous page"
                                    >
                                        <ArrowLeft className="w-4 h-4" />
                                    </button>

                                    <div className="flex items-center gap-2 text-sm font-mono">
                                        <span className="text-gray-400">Page</span>
                                        <span className="bg-gray-700 rounded px-2 py-1 min-w-[2rem] text-center">
                                            {state.currentPage}
                                        </span>
                                        <span className="text-gray-400">of</span>
                                        <span className="bg-gray-700 rounded px-2 py-1 min-w-[2rem] text-center">
                                            {Math.max(1, Math.ceil(totalRecords / state.pageSize))}
                                        </span>
                                    </div>

                                    <button
                                        onClick={() => handlePageChange(query.id, state.currentPage + 1)}
                                        disabled={state.currentPage >= Math.ceil(totalRecords / state.pageSize)}
                                        className="
                                            flex items-center justify-center
                                            w-8 h-8
                                            rounded
                                            transition-colors
                                            disabled:opacity-40
                                            disabled:cursor-not-allowed
                                            enabled:hover:bg-gray-700
                                            enabled:active:bg-gray-600
                                        "
                                        title="Next page"
                                    >
                                        <ArrowRight className="w-4 h-4" />
                                    </button>
                                </div>
                            </div>
                        )}
                    </>
                )}
            </div>
        );
    };

    const sliceIntoPages = (data: any[], pageSize: number, currentPage: number): any[] => {
        const startIndex = (currentPage % 2 === 1) ? 0 : pageSize;
        return data.slice(startIndex, startIndex + pageSize);
    };

    const handlePageChange = useCallback(async (queryId: string, page: number) => {
        const query = message.queries?.find(q => q.id === queryId);
        if (!query) return;

        const state = queryResults[queryId];
        const newOffset = (page - 1) * state.pageSize;

        // Initialize page data cache for this query if it doesn't exist
        if (!pageDataCacheRef.current[queryId]) {
            pageDataCacheRef.current[queryId] = {};
        }

        setQueryResults(prev => ({
            ...prev,
            [queryId]: { ...prev[queryId], loading: true, error: null }
        }));

        try {
            // Wrap state updates in preserveScroll
            if (newOffset < 50 && query.execution_result) {
                const resultArray = parseResults(query.execution_result);
                const totalRecords = query.pagination?.total_records_count || resultArray.length;

                // Calculate the slice for current page
                const startIndex = (page - 1) * state.pageSize;
                const endIndex = Math.min(startIndex + state.pageSize, resultArray.length);
                const pageData = resultArray.slice(startIndex, endIndex);

                // Cache the page data
                pageDataCacheRef.current[queryId][page] = {
                    data: pageData,
                    totalRecords
                };

                onQueryUpdate(() => {
                    setQueryResults(prev => ({
                        ...prev,
                        [queryId]: {
                            ...prev[queryId],
                            loading: false,
                            currentPage: page,
                            data: pageData,
                            error: null,
                            totalRecords
                        }
                    }));
                });
                return;
            }

            if (pageDataCacheRef.current[queryId][page]) {
                const cachedData = pageDataCacheRef.current[queryId][page];
                onQueryUpdate(() => {
                    setQueryResults(prev => ({
                        ...prev,
                        [queryId]: {
                            ...prev[queryId],
                            loading: false,
                            currentPage: page,
                            data: cachedData.data,
                            error: null,
                            totalRecords: cachedData.totalRecords
                        }
                    }));
                });
                return;
            }

            // For remote pagination - fetch new pages
            const apiPage = Math.ceil(page / 2); // Convert UI page to API page (each API call gets 50 records)
            const response = await axios.post(`${import.meta.env.VITE_API_URL}/chats/${chatId}/queries/results`, {
                message_id: message.id,
                query_id: queryId,
                stream_id: streamId,
                offset: (apiPage - 1) * 50 // Adjust offset to fetch 50 records at a time
            });

            // Get the results array from the response
            const responseData = response.data.data;
            const fullData = parseResults(responseData.execution_result);
            const totalRecords = responseData.total_records_count;

            // Slice the data into pages of 25
            const pageData = sliceIntoPages(fullData, state.pageSize, page % 2);

            // Cache both pages from the API response
            const basePage = Math.floor((page - 1) / 2) * 2 + 1;
            const firstHalf = sliceIntoPages(fullData, state.pageSize, 1);
            const secondHalf = sliceIntoPages(fullData, state.pageSize, 2);

            pageDataCacheRef.current[queryId][basePage] = {
                data: firstHalf,
                totalRecords
            };
            pageDataCacheRef.current[queryId][basePage + 1] = {
                data: secondHalf,
                totalRecords
            };

            // Wrap the final state update
            onQueryUpdate(() => {
                setQueryResults(prev => ({
                    ...prev,
                    [queryId]: {
                        ...prev[queryId],
                        loading: false,
                        currentPage: page,
                        data: pageData,
                        error: null,
                        totalRecords
                    }
                }));
            });

        } catch (error: any) {
            console.error('Error fetching results:', error);
            setQueryResults(prev => ({
                ...prev,
                [queryId]: {
                    ...prev[queryId],
                    loading: false,
                    error: error.response?.data?.error || 'Failed to fetch results',
                    data: prev[queryId].data
                }
            }));
        }
    }, [message.queries, queryResults, streamId, chatId, message.id, onQueryUpdate]);

    // Update parseResults to better handle the API response format
    const parseResults = (result: any): any[] => {
        if (!result) return [];

        if (Array.isArray(result)) {
            return result;
        }

        if (result && typeof result === 'object') {
            if ('results' in result) {
                // Handle null results in non-tech mode
                if (result.results === null && message.non_tech_mode) {
                    return [];
                }
                if (Array.isArray(result.results)) {
                    return result.results;
                }
            }
            if ('rowsAffected' in result || 'message' in result) {
                // For DML queries that return rowsAffected or message
                return [result];
            }
            // If no results property found, try to convert the object itself to array
            return [result];
        }

        // For primitive values or other types
        return [result];
    };

    // Function to render colored JSON syntax highlighting
    const renderColoredJson = (data: any, indent = 0): JSX.Element => {
        const indentStr = '  '.repeat(indent);
        
        if (data === null) {
            // Show "No Data found" for null in non-tech mode
            if (message.non_tech_mode) {
                return <span className="text-gray-400">No Data found</span>;
            }
            return <span className="text-yellow-400">null</span>;
        }
        
        if (data === undefined) {
            return <span className="text-yellow-400">undefined</span>;
        }
        
        if (typeof data === 'boolean') {
            return <span className="text-purple-400">{String(data)}</span>;
        }
        
        if (typeof data === 'number') {
            return <span className="text-cyan-400">{data}</span>;
        }
        
        if (typeof data === 'string') {
            // Check if it's a date string
            if (isDateString(data)) {
                return <span className="text-yellow-300">"{data}"</span>;
            }
            return <span className="text-green-400">"{data}"</span>;
        }
        
        if (Array.isArray(data)) {
            if (data.length === 0) {
                return <span>[]</span>;
            }
            
            return (
                <span>
                    <span>[</span>
                    <div style={{ marginLeft: 20 }}>
                        {data.map((item, index) => (
                            <div key={index}>
                                {renderColoredJson(item, indent + 1)}
                                {index < data.length - 1 && <span>,</span>}
                            </div>
                        ))}
                    </div>
                    <span>{indentStr}]</span>
                </span>
            );
        }
        
        if (typeof data === 'object') {
            const keys = Object.keys(data);
            
            if (keys.length === 0) {
                return <span>{'{}'}</span>;
            }
            
            return (
                <span>
                    <span>{'{'}</span>
                    <div style={{ marginLeft: 20 }}>
                        {keys.map((key, index) => (
                            <div key={key}>
                                <span className="text-blue-400">"{key}"</span>
                                <span>: </span>
                                {renderColoredJson(data[key], indent + 1)}
                                {index < keys.length - 1 && <span>,</span>}
                            </div>
                        ))}
                    </div>
                    <span>{indentStr}{'}'}</span>
                </span>
            );
        }
        
        return <span>{String(data)}</span>;
    };

    const renderQuery = (isMessageStreaming: boolean, query: QueryResult, index: number) => {
        // Ensure query is valid before proceeding
        if (!query) {
            console.error('renderQuery called with invalid query');
            return null;
        }

        if (isMessageStreaming && streamingQueryIndex !== -1 && index !== streamingQueryIndex) {
            return null;
        }
        const isEditingQuery = editingQueries[query.id] || false;
        const editedQueryText = editedQueryTexts[query.id] || removeDuplicateQueries(query.query || '');

        const setIsEditingQuery = (value: boolean) => {
            setEditingQueries(prev => ({ ...prev, [query.id]: value }));
        };
        
        const setEditedQueryText = (value: string) => {
            setEditedQueryTexts(prev => ({ ...prev, [query.id]: value }));
        };

        // Get query state or initialize it
        const queryState = queryStates[query.id] || { isExecuting: false, isExample: false };
        const isResultMinimized = minimizedResults[query.id] || false;

        const queryId = query.id;
        const shouldShowExampleResult = !query.is_executed && !query.is_rolled_back;
        const resultToShow = shouldShowExampleResult ? query.example_result : query.execution_result;
        const isCurrentlyStreaming = !isMessageStreaming && streamingQueryIndex === index;

        const shouldShowRollback = query.can_rollback && ((query.rollback_query != null && query.rollback_query != "") || (query.rollback_dependent_query != null && query.rollback_dependent_query != "")) &&
            query.is_executed &&
            !query.is_rolled_back &&
            !query.error;

        return (
            <div>
                <p className="mb-4 mt-4 font-base text-base"
                    ref={el => {
                        if (searchResultRefs && el) {
                            searchResultRefs.current[`explanation-${message.id}-${index}`] = el;
                        }
                    }}>
                    <span className="text-black font-semibold">Explanation:</span> {searchQuery 
                        ? highlightSearchText(
                            isCurrentlyStreaming && isDescriptionStreaming
                                ? currentDescription
                                : query.description,
                            searchQuery
                        )
                        : (isCurrentlyStreaming && isDescriptionStreaming
                            ? currentDescription
                            : query.description)
                    }
                </p>
                <div 
                    key={index} 
                    className="mt-4 bg-black text-white rounded-lg font-mono text-sm overflow-hidden w-full" 
                    style={{ minWidth: '100%' }}
                    ref={el => {
                        if (searchResultRefs && el) {
                            searchResultRefs.current[`query-${message.id}-${index}`] = el;
                        }
                    }}>
                    <div className="flex flex-wrap items-center justify-between gap-2 mb-4 px-4 pt-4">
                        <div className="flex justify-between items-center md:justify-centre gap-2">
                            <span>{message.non_tech_mode ? `Action ${index + 1}:` : `Query ${index + 1}:`}</span>
                            {query.is_edited && (
                                <span className="text-xs bg-gray-500/20 text-gray-300 px-2 py-0.5 rounded">
                                    Edited
                                </span>
                            )}
                            {query.is_rolled_back ? (
                                <span className="text-xs bg-yellow-500/20 text-yellow-300 px-2 py-0.5 rounded">
                                    Rolled Back on {query.action_at != null ? `${formatActionAt(query.action_at)}` : ''}
                                </span>
                            ) : query.is_executed && query.action_at != null ? (
                                <span className="w-[60%] md:w-auto text-xs bg-green-500/20 text-green-300 px-2 py-0.5 rounded">
                                    Executed on {formatActionAt(query.action_at)}
                                </span>
                            ) : (
                                <span className="text-xs bg-blue-500/20 text-blue-300 px-2 py-0.5 rounded">
                                    Execute Manually
                                </span>
                            )}
                        </div>
                        <div className="flex items-center">
                            {(
                                !queryState.isExecuting && !query.is_executed && (
                                    <>
                                        <button
                                            onClick={(e) => {
                                                e.preventDefault();
                                                e.stopPropagation();
                                                
                                                // Track query edit click
                                                if (userId && userName) {
                                                    analyticsService.trackQueryEditClick(chatId, query.id, userId, userName);
                                                }
                                                
                                                setIsEditingQuery(true);
                                            }}
                                            className="p-2 hover:bg-gray-800 rounded transition-colors text-yellow-400 hover:text-yellow-300 hover-tooltip-messagetile"
                                            data-tooltip="Edit query"
                                            title="Edit query"
                                        >
                                            <Pencil className="w-4 h-4" />
                                        </button>
                                        <div className="w-px h-4 bg-gray-700 mx-2" />
                                    </>
                                )
                            )}

                            {queryState.isExecuting ? (
                                <button
                                    onClick={(e) => {
                                        e.preventDefault();
                                        e.stopPropagation();

                                        // Track query cancel click
                                        if (userId && userName) {
                                            analyticsService.trackQueryCancelClick(chatId, queryId, userId, userName);
                                        }

                                        // Abort the API call if it's in progress
                                        if (abortControllerRef.current[queryId]) {
                                            abortControllerRef.current[queryId].abort();
                                            delete abortControllerRef.current[queryId];
                                        }

                                        // Clear any timeouts
                                        if (queryTimeouts.current[queryId]) {
                                            clearTimeout(queryTimeouts.current[queryId]);
                                            delete queryTimeouts.current[queryId];
                                        }

                                        // Update state
                                        onQueryUpdate(() => {
                                            setQueryStates(prev => ({
                                                ...prev,
                                                [queryId]: { isExecuting: false, isExample: !query.is_executed }
                                            }));
                                        });

                                        setTimeout(() => {
                                            window.scrollTo(window.scrollX, window.scrollY);
                                        }, 0);
                                        toast.error('Query cancelled', toastStyle);
                                    }}
                                    className="p-2 hover:bg-gray-800 rounded transition-colors text-red-500 hover:text-red-400 hover-tooltip-messagetile"
                                    data-tooltip="Cancel query"
                                    title="Cancel query"
                                >
                                    <XCircle className="w-4 h-4" />
                                </button>
                            ) : (
                                <button
                                    onClick={() => handleExecuteQuery(queryId)}
                                    className="p-2 text-red-500 hover:text-red-400 hover:bg-gray-800 rounded transition-colors hover-tooltip-messagetile"
                                    data-tooltip={query.is_executed ? "Rerun query" : "Execute query"}
                                    title={query.is_executed ? "Rerun query" : "Execute query"}
                                >
                                    {query.is_executed
                                        ? <RefreshCcw className="w-4 h-4" />
                                        : <Play className="w-4 h-4" />}
                                </button>
                            )}
                            <div className="w-px h-4 bg-gray-700 mx-2" />
                            <button
                                onClick={() => handleCopyToClipboard(query.query, { type: 'query', queryId: query.id })}
                                className="p-2 hover:bg-gray-800 rounded text-white hover:text-gray-200 hover-tooltip-messagetile"
                                data-tooltip="Copy query"
                                title="Copy query"
                            >
                                <Copy className="w-4 h-4" />
                            </button>
                        </div>
                    </div>
                    {isEditingQuery ? (
                        <div className="px-4 pb-4 border-t border-gray-700 pt-4">
                            <textarea
                                value={editedQueryText}
                                onChange={(e) => setEditedQueryText(e.target.value)}
                                className="w-full bg-gray-900 text-white p-3 rounded-none 
                                       border-4 border-gray-600 font-mono text-sm min-h-[120px]
                                       focus:outline-none focus:border-neo-gray shadow-[4px_4px_0px_0px_rgba(75,85,99,1)]"
                            />
                            <div className="flex justify-end gap-3 mt-4">
                                <button
                                    onClick={() => {
                                        setIsEditingQuery(false);
                                        setEditedQueryText(removeDuplicateQueries(query.query || ''));
                                    }}
                                    className="font-semibold px-4 py-2 bg-gray-800 text-white border-2 border-gray-600
                                                  hover:bg-gray-700 transition-colors shadow-[2px_2px_0px_0px_rgba(75,85,99,1)]
                                                  active:translate-y-[1px] active:shadow-[1px_1px_0px_0px_rgba(75,85,99,1)]"
                                >
                                    Cancel
                                </button>
                                <button
                                    onClick={() => {
                                        setIsEditingQuery(false);
                                        onEditQuery(message.id, queryId, editedQueryText);
                                    }}
                                    className="font-semibold px-4 py-2 bg-yellow-400 text-black border-2 border-black
                                                  hover:bg-yellow-300 transition-colors shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]
                                                  active:translate-y-[1px] active:shadow-[1px_1px_0px_0px_rgba(0,0,0,1)]"
                                >
                                    Save Changes
                                </button>
                            </div>
                        </div>
                    ) : (
                        !message.non_tech_mode ? (
                            <pre className={`
                        text-sm overflow-x-auto p-4 border-t border-gray-700
                            ${isCurrentlyStreaming && isQueryStreaming ? 'animate-pulse duration-300' : ''}
                        `}>
                                <code className="whitespace-pre-wrap break-words">
                                    {searchQuery 
                                        ? highlightSearchText(
                                            isCurrentlyStreaming && isQueryStreaming
                                                ? removeDuplicateQueries(currentQuery)
                                                : removeDuplicateQueries(query.query),
                                            searchQuery
                                        )
                                        : (isCurrentlyStreaming && isQueryStreaming
                                            ? removeDuplicateQueries(currentQuery)
                                            : removeDuplicateQueries(query.query))
                                    }
                                </code>
                            </pre>
                        ) : (
                            <div>
                            </div>
                        )
                    )}
                    {(query.execution_result || query.example_result || query.error || queryState.isExecuting) && (
                        <div 
                            className="border-t border-gray-700 mt-2 w-full"
                            ref={el => {
                                if (searchResultRefs && el) {
                                    searchResultRefs.current[`result-${message.id}-${index}`] = el;
                                }
                            }}>
                            {queryState.isExecuting ? (
                                <div className="flex items-center justify-center p-8">
                                    <Loader className="w-8 h-8 animate-spin text-gray-400" />
                                    <span className="ml-3 text-gray-400">Executing  query...</span>
                                </div>
                            ) : (
                                <div className="mt-3 px-4 pt-4 w-full">
                                    <div className="flex flex-wrap items-center justify-between gap-2 mb-4">
                                        <div className="flex items-center gap-2 text-gray-400">
                                            {query.error ? (
                                                <span className="text-neo-error font-medium flex items-center gap-2">
                                                    <AlertCircle className="w-4 h-4" />
                                                    Error
                                                </span>
                                            ) : (
                                                <div className="flex items-center gap-2">
                                                    <button
                                                        onClick={() => toggleResultMinimize(query.id)}
                                                        className="p-1 hover:bg-gray-800 rounded text-white hover:text-gray-200"
                                                        title={isResultMinimized ? "Expand" : "Collapse"}
                                                        
                                                    >
                                                        {isResultMinimized ? (
                                                            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="m6 9 6 6 6-6"/></svg>
                                                        ) : (
                                                            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="m18 15-6-6-6 6"/></svg>
                                                        )}
                                                    </button>
                                                    <span>
                                                        {shouldShowExampleResult ? 'Example Result:' : query.is_rolled_back ? 'Rolled Back Result:' : 'Result:'}
                                                    </span>
                                                </div>
                                            )}
                                            {query.example_execution_time && !query.execution_time && !query.is_executed && !query.error && (
                                                <span className="text-xs bg-gray-800 px-2 py-1 rounded flex items-center gap-1">
                                                    <Clock className="w-3 h-3" />
                                                    {query.example_execution_time.toLocaleString()}ms
                                                </span>
                                            )}

                                            {query.execution_time! > 0 && !query.error && (
                                                <span className="text-xs bg-gray-800 px-2 py-1 rounded flex items-center gap-1">
                                                    <Clock className="w-3 h-3" />
                                                    {query.execution_time!.toLocaleString()}ms
                                                </span>
                                            )}
                                        </div>
                                        {!query.error && <div className="flex gap-2">
                                            <div className="flex items-center">
                                                <button
                                                    onClick={(e) => {
                                                        e.preventDefault();
                                                        e.stopPropagation();
                                                        
                                                        // Track view mode toggle
                                                        if (userId && userName) {
                                                            analyticsService.trackResultViewToggle(chatId, query.id, 'table', userId, userName);
                                                        }
                                                        
                                                        setViewModes(prev => ({ ...prev, [query.id]: 'table' }));
                                                        setTimeout(() => {
                                                            window.scrollTo(window.scrollX, window.scrollY);
                                                        }, 0);
                                                    }}
                                                    className={`p-1 md:p-2 rounded ${(viewModes[query.id] || 'table') === 'table' ? 'bg-gray-700' : 'hover:bg-gray-800'} hover-tooltip-messagetile`}
                                                    data-tooltip="Table view"
                                                    title="Table view"
                                                >
                                                    <Table className="w-4 h-4" />
                                                </button>
                                                <div className="w-px h-4 bg-gray-700 mx-2" />
                                                <button
                                                    onClick={(e) => {
                                                        e.preventDefault();
                                                        e.stopPropagation();
                                                        
                                                        // Track view mode toggle
                                                        if (userId && userName) {
                                                            analyticsService.trackResultViewToggle(chatId, query.id, 'json', userId, userName);
                                                        }
                                                        
                                                        setViewModes(prev => ({ ...prev, [query.id]: 'json' }));
                                                        setTimeout(() => {
                                                            window.scrollTo(window.scrollX, window.scrollY);
                                                        }, 0);
                                                    }}
                                                    className={`p-1 md:p-2 rounded ${(viewModes[query.id] || 'table') === 'json' ? 'bg-gray-700' : 'hover:bg-gray-800'} hover-tooltip-messagetile`}
                                                    data-tooltip="JSON view"
                                                    title="JSON view"
                                                >
                                                    <Braces className="w-4 h-4" />
                                                </button>
                                                <div className="w-px h-4 bg-gray-700 mx-2" />
                                                
                                                {/* Download Dropdown */}
                                                <div className="relative download-dropdown-container">
                                                    <button
                                                        onClick={(e) => {
                                                            e.preventDefault();
                                                            e.stopPropagation();
                                                            setOpenDownloadMenu(openDownloadMenu === query.id ? null : query.id);
                                                        }}
                                                        className="p-1 md:p-2 rounded hover:bg-gray-800 flex items-center gap-1 hover-tooltip-messagetile"
                                                        data-tooltip="Download data"
                                                        title="Download data"
                                                    >
                                                        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="w-4 h-4">
                                                            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path>
                                                            <polyline points="7 10 12 15 17 10"></polyline>
                                                            <line x1="12" y1="15" x2="12" y2="3"></line>
                                                        </svg>
                                                        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="w-3 h-3">
                                                            <polyline points="6 9 12 15 18 9"></polyline>
                                                        </svg>
                                                    </button>
                                                    {openDownloadMenu === query.id && (
                                                        <div 
                                                            className="absolute right-0 mt-2 w-64 bg-gray-800 border border-gray-700 rounded-lg shadow-lg z-50 dropdown-menu"
                                                            style={{
                                                                bottom: 'auto',
                                                                top: '100%',
                                                                maxHeight: '300px',
                                                                overflowY: 'auto'
                                                            }}
                                                        >
                                                            <div className="p-2 text-xs text-gray-400 border-b border-gray-700">
                                                                {(() => {
                                                                    const cachedPages = pageDataCacheRef.current[query.id] || {};
                                                                    const pageCount = Object.keys(cachedPages).length;
                                                                    let totalRecords = 0;
                                                                    
                                                                    // Calculate total records across all cached pages
                                                                    Object.values(cachedPages).forEach((page: any) => {
                                                                        if (page.data && Array.isArray(page.data)) {
                                                                            totalRecords += page.data.length;
                                                                        }
                                                                    });
                                                                    
                                                                    return `This will export ${totalRecords} records from ${pageCount} fetched ${pageCount === 1 ? 'page' : 'pages'}.`;
                                                                })()}
                                                            </div>
                                                            <div className="py-1">
                                                                <button 
                                                                    onClick={(e) => {
                                                                        e.preventDefault();
                                                                        e.stopPropagation();
                                                                        handleExportData(query.id, 'csv');
                                                                        setOpenDownloadMenu(null);
                                                                    }}
                                                                    className="block w-full text-left px-4 py-2 text-sm text-white hover:bg-gray-700"
                                                                >
                                                                    Export as CSV
                                                                </button>
                                                                <button 
                                                                    onClick={(e) => {
                                                                        e.preventDefault();
                                                                        e.stopPropagation();
                                                                        handleExportData(query.id, 'json');
                                                                        setOpenDownloadMenu(null);
                                                                    }}
                                                                    className="block w-full text-left px-4 py-2 text-sm text-white hover:bg-gray-700"
                                                                >
                                                                    Export as JSON
                                                                </button>
                                                            </div>
                                                        </div>
                                                    )}
                                                </div>
                                                
                                                <div className="w-px h-4 bg-gray-700 mx-2" />
                                                {shouldShowRollback && (
                                                    !queryState.isExecuting ? (
                                                        <>
                                                            <button
                                                                onClick={(e) => {
                                                                    e.preventDefault();
                                                                    e.stopPropagation();
                                                                    
                                                                    // Track rollback click
                                                                    if (userId && userName) {
                                                                        analyticsService.trackRollbackClick(chatId, queryId, userId, userName);
                                                                    }
                                                                    
                                                                    setRollbackState({ show: true, queryId });
                                                                    setTimeout(() => {
                                                                    window.scrollTo(window.scrollX, window.scrollY);
                                                                }, 0);
                                                            }}
                                                            className="p-2 hover:bg-gray-800 rounded text-yellow-400 hover:text-yellow-300 hover-tooltip-messagetile"
                                                            data-tooltip="Rollback changes"
                                                            disabled={queryState.isExecuting}
                                                        >
                                                            <History className="w-4 h-4" />
                                                        </button>
                                                        <div className="w-px h-4 bg-gray-700 mx-2" />
                                                        </>
                                                    ) : (
                                                        <>
                                                            <button
                                                                onClick={(e) => {
                                                                    e.preventDefault();
                                                                e.stopPropagation();

                                                                // Abort the API call if it's in progress
                                                                if (abortControllerRef.current[queryId]) {
                                                                    abortControllerRef.current[queryId].abort();
                                                                    delete abortControllerRef.current[queryId];
                                                                }

                                                                // Clear any timeouts
                                                                if (queryTimeouts.current[queryId]) {
                                                                    clearTimeout(queryTimeouts.current[queryId]);
                                                                    delete queryTimeouts.current[queryId];
                                                                }

                                                                setRollbackState({ show: false, queryId: null });
                                                                // Update state
                                                                onQueryUpdate(() => {
                                                                    setQueryStates(prev => ({
                                                                        ...prev,
                                                                        [queryId]: { isExecuting: false, isExample: !query.is_executed }
                                                                    }));
                                                                });

                                                                setTimeout(() => {
                                                                    window.scrollTo(window.scrollX, window.scrollY);
                                                                }, 0);
                                                                toast.error('Query cancelled', toastStyle);
                                                            }}
                                                            className="p-2 hover:bg-gray-800 rounded transition-colors text-red-500 hover:text-red-400 hover-tooltip-messagetile"
                                                            data-tooltip="Cancel query"
                                                            title="Cancel query"
                                                        >
                                                            <XCircle className="w-4 h-4" />
                                                        </button>
                                                        <div className="w-px h-4 bg-gray-700 mx-2" />
                                                        </>
                                                    )
                                                )}
                                                <button
                                                    onClick={() => handleCopyToClipboard(JSON.stringify(resultToShow, null, 2), { type: 'result', queryId: query.id })}
                                                    className="p-2 hover:bg-gray-800 rounded text-white hover:text-gray-200 hover-tooltip-messagetile"
                                                    data-tooltip="Copy result"
                                                    title="Copy result"
                                                >
                                                    <Copy className="w-4 h-4" />
                                                </button>
                                            </div>
                                        </div>}
                                    </div>
                                    {!isResultMinimized && (
                                        <>
                                            {query.error ? (
                                                <div className="bg-neo-error/10 text-neo-error p-4 rounded-lg mb-6">
                                                    <div className="font-bold mb-2">{searchQuery ? highlightSearchText(query.error.code, searchQuery) : query.error.code}</div>
                                                    {query.error.message != query.error.details && <div className="mb-2">{searchQuery ? highlightSearchText(query.error.message, searchQuery) : query.error.message}</div>}
                                                    {query.error.details && (
                                                        <div className="text-sm opacity-80 border-t border-neo-error/20 pt-2 mt-2">
                                                            {searchQuery ? highlightSearchText(query.error.details, searchQuery) : query.error.details}
                                                        </div>
                                                    )}
                                                </div>
                                            ) : (
                                                <div className="px-0">
                                                    <div className={`
                                            text-green-400 pb-6 w-full
                                                    ${!query.example_result && !query.error ? '' : ''}
                                        `}>
                                                        {(viewModes[query.id] || 'table') === 'table' ? (
                                                            <div className="w-full">
                                                                {shouldShowExampleResult ? (
                                                                    resultToShow ? renderTableView(parseResults(resultToShow)) : (
                                                                        <div className="text-gray-500">{message.non_tech_mode ? "No Data found" : "No example data available"}</div>
                                                                    )
                                                                ) : (
                                                                    resultToShow ? (
                                                                        renderQueryResult(resultToShow, query.id)
                                                                    ) : (
                                                                        <div className="text-gray-500">{message.non_tech_mode ? "No Data found" : "No data to display"}</div>
                                                                    )
                                                                )}
                                                            </div>
                                                        ) : (
                                                            <div className="w-full">
                                                                {shouldShowExampleResult ? (
                                                                    resultToShow ? (
                                                                        <pre className="overflow-x-auto whitespace-pre-wrap rounded-md">
                                                                            {renderColoredJson(parseResults(resultToShow))}
                                                                        </pre>
                                                                    ) : (
                                                                        <div className="text-gray-500">{message.non_tech_mode ? "No Data found" : "No example data available"}</div>
                                                                    )
                                                                ) : (
                                                                    resultToShow ? (
                                                                        renderQueryResult(resultToShow, query.id)
                                                                    ) : (
                                                                        <div className="text-gray-500">{message.non_tech_mode ? "No Data found" : "No data to display"}</div>
                                                                    )
                                                                )}
                                                            </div>
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
            </div>
        );
    };

    // Add a helper function to remove duplicate queries
    const removeDuplicateQueries = (query: string): string => {
        // Split by semicolon and trim each query
        const queries = query.split(';')
            .map(q => q.trim())
            .filter(q => q.length > 0);

        // Remove duplicates while preserving order
        const uniqueQueries = Array.from(new Set(queries));

        // Join back with semicolons
        return uniqueQueries.join(';\n');
    };
    
    // Remove duplicate content from the message
    const removeDuplicateContent = (content: string): string => {
        if (!content) return '';
        
        // Check for exact duplication of the entire content
        const contentLength = content.length;
        if (contentLength > 20) { // Only check for longer content
            const halfPoint = Math.floor(contentLength / 2);
            
            // Try different split points around the middle
            for (let offset = -10; offset <= 10; offset++) {
                const splitPoint = halfPoint + offset;
                if (splitPoint <= 0 || splitPoint >= contentLength) continue;
                
                const firstPart = content.substring(0, splitPoint).trim();
                const secondPart = content.substring(splitPoint).trim();
                
                // If the second part starts with the same text as the first part
                if (secondPart.startsWith(firstPart.substring(0, Math.min(20, firstPart.length)))) {
                    return firstPart;
                }
            }
        }
        
        // If not an exact duplication, handle sentence by sentence
        const sentences = content.split(/(?<=[.!?])\s+/);
        const uniqueSentences: string[] = [];
        const seen = new Set<string>();
        
        for (const sentence of sentences) {
            const trimmed = sentence.trim();
            // Skip empty sentences and check for duplicates (case insensitive)
            if (trimmed && !seen.has(trimmed.toLowerCase())) {
                seen.add(trimmed.toLowerCase());
                uniqueSentences.push(sentence);
            }
        }
        
        return uniqueSentences.join(' ');
    };

    // Add this function to handle data export
    const handleExportData = (queryId: string, format: 'csv' | 'xlsx' | 'json') => {
        const query = message.queries?.find(q => q.id === queryId);
        if (!query) return;
        
        // Get all cached data for this query from pageDataCacheRef
        const cachedPages = pageDataCacheRef.current[queryId] || {};
        const pageNumbers = Object.keys(cachedPages).map(Number).sort((a, b) => a - b);
        
        // Combine all data from all cached pages
        let allData: any[] = [];
        if (pageNumbers.length > 0) {
            // Collect data from all cached pages
            pageNumbers.forEach(pageNum => {
                const pageData = cachedPages[pageNum]?.data || [];
                allData = [...allData, ...pageData];
            });
        } else {
            // Fallback to current query results or execution result if no cached pages
            const queryResult = queryResults[queryId];
            allData = queryResult?.data || parseResults(query.is_executed ? query.execution_result : query.example_result);
        }
        
        // Remove duplicates (in case pages overlap)
        const seen = new Set();
        allData = allData.filter(item => {
            // Create a string key from the item to check for duplicates
            const key = JSON.stringify(item);
            if (seen.has(key)) return false;
            seen.add(key);
            return true;
        });
        
        if (!allData || allData.length === 0) {
            toast.error('No data available to export', toastStyle);
            return;
        }
        
        try {
            // Convert data to the requested format
            let content: string = '';
            let fileName: string = `query-${queryId}-export`;
            let mimeType: string = '';
            
            if (format === 'json') {
                content = JSON.stringify(allData, null, 2);
                fileName += '.json';
                mimeType = 'application/json';
            } else if (format === 'csv') {
                // Get headers from the first object
                const headers = Object.keys(allData[0]);
                
                // Create CSV header row
                content = headers.join(',') + '\n';
                
                // Add data rows
                allData.forEach((row: any) => {
                    const rowValues = headers.map(header => {
                        const value = row[header];
                        // Handle different types of values
                        if (value === null || value === undefined) return '';
                        if (typeof value === 'object') return JSON.stringify(value).replace(/"/g, '""');
                        return typeof value === 'string' ? `"${value.replace(/"/g, '""')}"` : value;
                    });
                    content += rowValues.join(',') + '\n';
                });
                
                fileName += '.csv';
                mimeType = 'text/csv';
            }
            
            // Create a blob and download link
            const blob = new Blob([content], { type: mimeType });
            const url = URL.createObjectURL(blob);
            const link = document.createElement('a');
            link.href = url;
            link.download = fileName;
            document.body.appendChild(link);
            link.click();
            
            // Clean up
            setTimeout(() => {
                document.body.removeChild(link);
                URL.revokeObjectURL(url);
            }, 100);
            
            // Track data export
            if (userId && userName) {
                analyticsService.trackDataExport(chatId, queryId, format, allData.length, userId, userName);
            }

            toast(`Exported ${allData.length} records as ${format.toUpperCase()}`, {
                ...toastStyle,
                icon: '📥',
            });
        } catch (error) {
            console.error('Error exporting data:', error);
            toast.error(`Failed to export data: ${error}`, toastStyle);
        }
    };

    return (
        <div 
            className={`
                py-4 md:py-6
                ${isFirstMessage ? 'first:pt-0' : ''}
                w-full
                relative
              `}
            ref={el => {
                if (searchResultRefs && el) {
                    searchResultRefs.current[`msg-${message.id}`] = el;
                }
            }}>
            <div className={`
        group flex items-center relative
        ${message.type === 'user' ? 'justify-end' : 'justify-start'}
        w-full
      `}>
                {message.type === 'user' && (
                    <div className="
            absolute 
            right-0 
            -bottom-9
            md:-bottom-10 
            flex 
            gap-1
            z-[5]

          ">
                        <button
                            onClick={() => handleCopyToClipboard(removeDuplicateContent(message.content), { type: 'message', messageId: message.id })}
                            className="
                -translate-y-1/2
                p-1.5
                md:p-2 
                group-hover:opacity-100 
                transition-colors
                hover:bg-neo-gray
                rounded-lg
                flex-shrink-0
                border-0
                bg-white/80
                backdrop-blur-sm
                hover-tooltip-messagetile
              "
                            data-tooltip="Copy message"
                            title="Copy message"
                        >
                            <Copy className="w-4 h-4 text-gray-800" />
                        </button>
                        {onEdit && (
                            <button
                                onClick={(e) => {
                                    e.preventDefault();
                                    e.stopPropagation();
                                    
                                    // Track message edit click
                                    if (userId && userName) {
                                        analyticsService.trackMessageEditClick(chatId, message.id, userId, userName);
                                    }
                                    
                                    onEdit(message.id);
                                    setTimeout(() => {
                                        window.scrollTo(window.scrollX, window.scrollY);
                                    }, 0);
                                }}
                                className="
                  -translate-y-1/2
                  p-1.5
                  md:p-2
                  group-hover:opacity-100 
                  hover:bg-neo-gray
                  transition-colors
                  rounded-lg
                  flex-shrink-0
                  border-0
                  bg-white/80
                  backdrop-blur-sm
                  hover-tooltip-messagetile
                "
                                data-tooltip="Edit message"
                                title="Edit message"
                            >
                                <Pencil className="w-4 h-4 text-gray-800" />
                            </button>
                        )}
                    </div>
                )}
                {message.type === 'assistant' && (
                    <div className="
            absolute 
            left-0 
            -bottom-9
            md:-bottom-10 
            flex 
            gap-1
            z-[5]
          ">
                        <button
                            onClick={() => handleCopyToClipboard(removeDuplicateContent(message.content), { type: 'message', messageId: message.id })}
                            className="
                -translate-y-1/2
                p-1.5
                md:p-2 
                group-hover:opacity-100 
                transition-colors
                hover:bg-neo-gray
                rounded-lg
                flex-shrink-0
                border-0
                bg-white/80
                backdrop-blur-sm
                hover-tooltip-messagetile
              "
                            data-tooltip="Copy message"
                            title="Copy message"
                        >
                            <Copy className="w-4 h-4 text-gray-800" />
                        </button>
                    </div>
                )}
                <div className={`
    message-bubble
    inline-block
        relative
    ${message.type === 'user' ? (
                        editingMessageId === message.id
                            ? 'w-[95%] sm:w-[85%] md:w-[75%]'
                            : 'w-fit max-w-[95%] sm:max-w-[85%] md:max-w-[75%]'
                    ) : 'w-fit max-w-[95%] sm:max-w-[85%] md:max-w-[75%]'}
    ${message.type === 'user'
        ? 'message-bubble-user'
        : 'message-bubble-ai'
    }
`}>
                    <div className={`
        ${editingMessageId === message.id ? 'w-full min-w-full' : 'w-auto min-w-0'}
        ${message.queries?.length ? 'min-w-full' : ''}
    `}>
                        <div className="relative">
                            {message.content.length === 0 && message.loading_steps && message.loading_steps.length > 0 && (
                                <div className={`
                                    ${message.content ? 'animate-fade-up-out absolute w-full' : ''}
                                        text-gray-700
                                    `}>
                                    <LoadingSteps
                                        steps={message.loading_steps.map((step, index) => ({
                                            text: step.text,
                                            done: index !== message.loading_steps!.length - 1
                                        }))}
                                    />
                                </div>
                            )}

                            {editingMessageId === message.id ? (
                                <div className='w-full'>
                                    <textarea
                                        value={editInput}
                                        onChange={(e) => {
                                            e.preventDefault();
                                            e.stopPropagation();
                                            setEditInput(e.target.value);
                                            setTimeout(() => {
                                                window.scrollTo(window.scrollX, window.scrollY);
                                            }, 0);
                                        }}
                                        className="neo-input w-full text-lg min-h-[42px] resize-y py-2 px-3 leading-normal whitespace-pre-wrap"
                                        rows={Math.min(
                                            Math.max(
                                                editInput.split('\n').length,
                                                Math.ceil(editInput.length / 50)
                                            ),
                                            10
                                        )}
                                        autoFocus
                                    />
                                    <div className="flex gap-2 mt-3">
                                        <button
                                            onClick={() => {
                                                onCancelEdit();
                                                setTimeout(() => {
                                                    window.scrollTo(window.scrollX, window.scrollY);
                                                }, 0);
                                            }}
                                            className="neo-button-secondary flex-1 flex items-center justify-center gap-2"
                                        >
                                            <X className="w-4 h-4" />
                                            <span>Cancel</span>
                                        </button>
                                        <button
                                            onClick={() => onSaveEdit(message.id, editInput)}
                                            className="neo-button flex-1 flex items-center justify-center gap-2"
                                        >
                                            <Send className="w-4 h-4" />
                                            <span>Send</span>
                                        </button>
                                    </div>
                                </div>
                            ) :
                            // Message content
                             (
                                <div className={message.loading_steps ? 'animate-fade-in' : ''}>
                                 <div className='flex flex-col gap-1'>
                                    {message.type === 'user' ? (
                                        <p className='text-lg whitespace-pre-wrap break-words'>
                                            {searchQuery ? highlightSearchText(removeDuplicateContent(message.content), searchQuery) : removeDuplicateContent(message.content)}
                                        </p>) :
                                    (   <MarkdownRenderer 
                                            markdown={removeDuplicateContent(message.content)}
                                            searchQuery={searchQuery}
                                        />
                                    )
                                    }
                                    {message.is_edited && message.type === 'user' && (
                                        <span className="text-xs text-gray-600 italic">
                                            (edited)
                                        </span>
                                    )}
                                    </div>
                                    {message.queries && message.queries.length > 0 && (
                                        <div className="min-w-full">
                                            {message.queries.map((query: QueryResult, index: number) => {
                                                // Ensure query is valid before rendering
                                                if (!query || !query.id) {
                                                    console.error('Invalid query in message', message.id, index);
                                                    return null;
                                                }
                                                
                                                return (
                                                    <div key={query.id || `query-${index}`}>
                                                        {renderQuery(message.is_streaming || false, query, index)}
                                                    </div>
                                                );
                                            })}
                                        </div>
                                    )}
                                    
                                    {message.action_buttons && message.action_buttons.length > 0 && (
                                        <div className="flex flex-wrap gap-3 mt-4">
                                            {message.action_buttons.map((button) => (
                                                <button
                                                    key={button.id}
                                                    onClick={() => {
                                                        if (buttonCallback) {
                                                            buttonCallback(button.action);
                                                        } else {
                                                            console.log(`Action button clicked: ${button.action}`);
                                                        }
                                                    }}
                                                    className={button.isPrimary ? "neo-button" : "neo-button-secondary"}
                                                >
                                                    {button.label}
                                                </button>
                                            ))}
                                        </div>
                                    )}
                                    
                                    {message.type === 'assistant' && message.content.length > 0 && message.id !== 'welcome-message' && (
                                        <div className="mt-4 group/tooltip">
                                            <p className="text-sm text-gray-700 flex items-center gap-1">
                                                <span className="inline-flex items-center">
                                                    <svg className="w-4 h-4 mr-1 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
                                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                                                    </svg>
                                                    This response was generated in 
                                                </span>
                                                <div className="relative inline-block">
                                                    <span 
                                                        className={`font-semibold px-2 py-0.5 rounded cursor-help ${message.non_tech_mode ? 'bg-green-100 text-green-700' : 'bg-yellow-100 text-yellow-700'}`}
                                                    >
                                                        {message.non_tech_mode ? 'Non-Technical' : 'Technical'}
                                                    </span>
                                                    <div className="absolute bottom-full left-1/2 transform -translate-x-1/2 mb-2 px-3 py-2 bg-gray-900 text-white text-sm rounded-lg opacity-0 invisible group-hover/tooltip:visible group-hover/tooltip:opacity-100 transition-all duration-200 w-64 z-50 pointer-events-none">
                                                        {message.non_tech_mode 
                                                            ? 'Non-Technical Mode: Generates simple, natural language queries that are easy to understand. Perfect for users who want to interact with data without knowing database syntax.'
                                                            : 'Technical Mode: Generates raw database queries with full syntax. Ideal for developers and technical users who want direct control over database operations.'
                                                        }
                                                        <div className="absolute top-full left-1/2 transform -translate-x-1/2 -mt-1">
                                                            <div className="border-4 border-transparent border-t-gray-900"></div>
                                                        </div>
                                                    </div>
                                                </div>
                                                Mode. You can change it for this chat from
                                                <button 
                                                    onClick={() => {
                                                        if (buttonCallback) {
                                                            buttonCallback('open_settings');
                                                        }
                                                    }}
                                                    className="text-blue-600 hover:text-blue-700 underline text-sm"
                                                >
                                                    here
                                                </button>
                                            </p>
                                        </div>
                                    )}
                        
                                </div>
                            )}
                        </div>

                        <div className={`
                              text-[12px] text-gray-500 mt-1
                              ${message.type === 'user' ? 'text-right' : 'text-left'}
                            `}>
                            {formatMessageTime(message)}
                        </div>
                    </div>
                </div>
            </div>

            {rollbackState.show && rollbackState.queryId && (
                <RollbackConfirmationModal
                    onConfirm={() => handleRollback(rollbackState.queryId!)}
                    onCancel={() => setRollbackState({ show: false, queryId: null })}
                />
            )}

            {showCriticalConfirm && (
                <ConfirmationModal
                    title="Critical Query"
                    message="This query may affect important data. Are you sure you want to proceed?"
                    onConfirm={async () => {
                        setShowCriticalConfirm(false);
                        if (queryToExecute !== null) {
                            executeQuery(queryToExecute);
                            setQueryToExecute(null);
                        }
                    }}
                    onCancel={() => {
                        setShowCriticalConfirm(false);
                        setQueryToExecute(null);
                    }}
                />
            )}
        </div>
    );
}