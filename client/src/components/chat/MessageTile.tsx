import { useCallback, useEffect, useRef, useState } from 'react';
import { Copy, Cpu, Pencil, Pin, Send, X } from 'lucide-react';
import toast from 'react-hot-toast';
import { useStream } from '../../contexts/StreamContext';
import analyticsService from '../../services/analyticsService';
import chatService from '../../services/chatService';
import { Message, QueryResult } from '../../types/query';
import { QueryState } from '../../types/messageTile';
import { formatMessageTime, removeDuplicateContent, removeDuplicateQueries } from '../../utils/queryUtils';
import { highlightSearchText } from '../../utils/highlightSearch';
import { useQueryOperations } from '../../hooks/useQueryOperations';
import LoadingSteps from './LoadingSteps';
import MarkdownRenderer from './MarkdownRenderer';
import QueryBlock from './QueryBlock';

// ─── Toast style ──────────────────────────────────────────────────────────────
const toastStyle = {
    style: {
        background: '#000', color: '#fff', border: '4px solid #000', borderRadius: '12px',
        boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)', padding: '12px 24px',
        fontSize: '14px', fontWeight: '500',
    },
    position: 'bottom-center' as const,
    duration: 2000,
};

// ─── Props ────────────────────────────────────────────────────────────────────
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
    searchResultRefs?: React.MutableRefObject<{ [key: string]: HTMLElement | null }>;
    buttonCallback?: (action: string, label?: string) => void;
    userId?: string;
    userName?: string;
    onPinMessage?: (messageId: string, isPinned: boolean) => Promise<void>;
}

// ─── Component ────────────────────────────────────────────────────────────────
export default function MessageTile({
    chatId, message, setMessage,
    onEdit, editingMessageId, editInput, setEditInput,
    onSaveEdit, onCancelEdit,
    queryStates, setQueryStates, queryTimeouts,
    checkSSEConnection, isFirstMessage,
    onQueryUpdate, onEditQuery,
    searchQuery, searchResultRefs,
    buttonCallback, userId, userName, onPinMessage,
}: MessageTileProps) {
    const { streamId } = useStream();

    // Expansion state for nested JSON cells — shared across all QueryBlocks in this message
    const expandedNodesRef = useRef<Record<string, boolean>>({});

    // Streaming animation state
    const [streamingQueryIndex, setStreamingQueryIndex] = useState<number>(-1);
    const [isDescriptionStreaming] = useState(false);
    const [isQueryStreaming] = useState(false);
    const [currentDescription, setCurrentDescription] = useState('');
    const [currentQuery, setCurrentQuery] = useState('');

    // All query state + handlers live in the hook
    const ops = useQueryOperations({
        chatId, message, streamId, queryStates, setQueryStates,
        queryTimeouts, setMessage, checkSSEConnection, onQueryUpdate,
        userId, userName,
    });

    // ── Streaming effects ─────────────────────────────────────────────────────
    useEffect(() => {
        if (!message.queries || !message.is_streaming) return;
        setStreamingQueryIndex(0);
        for (let i = 0; i < message.queries.length; i++) {
            const query = message.queries[i];
            if (!query || !query.id) continue;
            setStreamingQueryIndex(i);
            setCurrentDescription(query.description);
            setCurrentQuery(removeDuplicateQueries(query.query));
            ops.setEditedQueryTexts(prev => ({ ...prev, [query.id]: removeDuplicateQueries(query.query || '') }));
            if (message.queries) message.queries[i].is_streaming = false;
        }
    }, [message.queries, message.is_streaming]); // eslint-disable-line react-hooks/exhaustive-deps

    useEffect(() => {
        if (!message.is_streaming && message.queries && message.queries.length > 0) {
            setStreamingQueryIndex(-1);
        }
    }, [message.is_streaming, message.queries]);

    // ── Persist visualization on message ─────────────────────────────────────
    const handleVisualizationSaved = useCallback((queryId: string, vizData: any) => {
        setMessage({
            ...message,
            queries: message.queries?.map(q =>
                q.id === queryId ? { ...q, visualization: vizData, visualization_id: vizData.visualization_id } : q,
            ) || [],
        });
    }, [message, setMessage]);

    // ── Pin / copy ────────────────────────────────────────────────────────────
    const handlePinMessage = async () => {
        try {
            if (onPinMessage) {
                await onPinMessage(message.id, !message.is_pinned);
            } else {
                if (message.is_pinned) {
                    await chatService.unpinMessage(chatId, message.id);
                    setMessage({ ...message, is_pinned: false, pinned_at: undefined });
                    toast('Message unpinned', { ...toastStyle, icon: '📌' });
                } else {
                    await chatService.pinMessage(chatId, message.id);
                    setMessage({ ...message, is_pinned: true, pinned_at: new Date().toISOString() });
                    toast('Message pinned', { ...toastStyle, icon: '📌' });
                }
            }
        } catch {
            toast.error('Failed to update pin status', {
                ...toastStyle,
                style: { ...toastStyle.style, background: '#ff4444', border: '4px solid #cc0000' },
            });
        }
    };

    const handleCopyMessage = () => {
        navigator.clipboard.writeText(removeDuplicateContent(message.content));
        toast('Copied to clipboard!', { ...toastStyle, icon: '📋' });
        if (userId && userName) analyticsService.trackMessageCopyClick(chatId, message.id, message.type, userId, userName);
    };

    // ── Action button bar ─────────────────────────────────────────────────────
    const ActionBar = ({ side }: { side: 'user' | 'assistant' }) => (
        <div className={`absolute ${side === 'user' ? 'right-0' : 'left-0 right-0'} -bottom-9 md:-bottom-10 flex gap-1 ${side === 'assistant' ? 'items-center' : ''} z-[5]`}>
            <button
                onClick={handleCopyMessage}
                className="-translate-y-1/2 p-1.5 md:p-2 group-hover:opacity-100 transition-colors hover:bg-neo-gray rounded-lg flex-shrink-0 border-0 bg-white/80 backdrop-blur-sm hover-tooltip-messagetile"
                data-tooltip="Copy message"
                title="Copy message"
            >
                <Copy className="w-4 h-4 text-gray-800" />
            </button>
            <button
                onClick={handlePinMessage}
                className="-translate-y-1/2 p-1.5 md:p-2 group-hover:opacity-100 transition-colors hover:bg-neo-gray rounded-lg flex-shrink-0 border-0 bg-white/80 backdrop-blur-sm hover-tooltip-messagetile"
                data-tooltip={message.is_pinned ? 'Unpin message' : 'Pin message'}
                title={message.is_pinned ? 'Unpin message' : 'Pin message'}
            >
                <Pin className={`w-4 h-4 rotate-45 ${message.is_pinned ? 'text-black fill-black' : 'text-gray-800'}`} />
            </button>
            {side === 'user' && onEdit && (
                <button
                    onClick={e => {
                        e.preventDefault();
                        e.stopPropagation();
                        if (userId && userName) analyticsService.trackMessageEditClick(chatId, message.id, userId, userName);
                        onEdit(message.id);
                        setTimeout(() => window.scrollTo(window.scrollX, window.scrollY), 0);
                    }}
                    className="-translate-y-1/2 p-1.5 md:p-2 group-hover:opacity-100 hover:bg-neo-gray transition-colors rounded-lg flex-shrink-0 border-0 bg-white/80 backdrop-blur-sm hover-tooltip-messagetile"
                    data-tooltip="Edit message"
                    title="Edit message"
                >
                    <Pencil className="w-4 h-4 text-gray-800" />
                </button>
            )}
        </div>
    );

    // ── Render ────────────────────────────────────────────────────────────────
    return (
        <div
            className={`py-4 md:py-6 ${isFirstMessage ? 'first:pt-0' : ''} w-full relative`}
            ref={el => { if (searchResultRefs && el) searchResultRefs.current[`msg-${message.id}`] = el; }}
        >
            <div className={`group flex items-center relative ${message.type === 'user' ? 'justify-end' : 'justify-start'} w-full`}>
                {message.type === 'user' && <ActionBar side="user" />}
                {message.type === 'assistant' && <ActionBar side="assistant" />}

                <div className={`
                    message-bubble inline-block relative
                    ${message.type === 'user'
                        ? editingMessageId === message.id ? 'w-[95%] sm:w-[85%] md:w-[75%]' : 'w-fit max-w-[95%] sm:max-w-[85%] md:max-w-[75%]'
                        : 'w-fit max-w-[95%] sm:max-w-[85%] md:max-w-[75%]'}
                    ${message.type === 'user' ? 'message-bubble-user' : 'message-bubble-ai'}
                `}>
                    <div className={`${editingMessageId === message.id ? 'w-full min-w-full' : 'w-auto min-w-0'} ${message.queries?.length ? 'min-w-full' : ''}`}>
                        <div className="relative">
                            {message.content.length === 0 && message.loading_steps && message.loading_steps.length > 0 && (
                                <div className={`${message.content ? 'animate-fade-up-out absolute w-full' : ''} text-gray-700`}>
                                    <LoadingSteps steps={message.loading_steps.map((step, i) => ({
                                        text: step.text,
                                        done: i !== message.loading_steps!.length - 1,
                                    }))} />
                                </div>
                            )}

                            {editingMessageId === message.id ? (
                                <div className="w-full">
                                    <textarea
                                        value={editInput}
                                        onChange={e => {
                                            e.preventDefault();
                                            e.stopPropagation();
                                            setEditInput(e.target.value);
                                            setTimeout(() => window.scrollTo(window.scrollX, window.scrollY), 0);
                                        }}
                                        className="neo-input w-full text-lg min-h-[42px] resize-y py-2 px-3 leading-normal whitespace-pre-wrap"
                                        rows={Math.min(Math.max(editInput.split('\n').length, Math.ceil(editInput.length / 50)), 10)}
                                        autoFocus
                                    />
                                    <div className="flex gap-2 mt-3">
                                        <button
                                            onClick={() => { onCancelEdit(); setTimeout(() => window.scrollTo(window.scrollX, window.scrollY), 0); }}
                                            className="neo-button-secondary flex-1 flex items-center justify-center gap-2"
                                        >
                                            <X className="w-4 h-4" /><span>Cancel</span>
                                        </button>
                                        <button
                                            onClick={() => onSaveEdit(message.id, editInput)}
                                            className="neo-button flex-1 flex items-center justify-center gap-2"
                                        >
                                            <Send className="w-4 h-4" /><span>Send</span>
                                        </button>
                                    </div>
                                </div>
                            ) : (
                                <div className={message.loading_steps ? 'animate-fade-in' : ''}>
                                    <div className="flex flex-col gap-1">
                                        <div className="flex items-flex-start justify-between gap-3">
                                            <div className="flex-1">
                                                {message.type === 'user' ? (
                                                    <p className="text-lg whitespace-pre-wrap break-words">
                                                        {searchQuery
                                                            ? highlightSearchText(removeDuplicateContent(message.content), searchQuery)
                                                            : removeDuplicateContent(message.content)}
                                                    </p>
                                                ) : (
                                                    <MarkdownRenderer
                                                        markdown={removeDuplicateContent(message.content)}
                                                        searchQuery={searchQuery}
                                                    />
                                                )}
                                            </div>
                                        </div>
                                        {message.is_edited && message.type === 'user' && (
                                            <span className="text-xs text-gray-600 italic">(edited)</span>
                                        )}
                                    </div>

                                    {message.queries && message.queries.length > 0 && (
                                        <div className="min-w-full">
                                            {message.queries.map((query: QueryResult, index: number) => {
                                                if (!query || !query.id) return null;
                                                return (
                                                    <div key={query.id}>
                                                        <QueryBlock
                                                            chatId={chatId}
                                                            message={message}
                                                            query={query}
                                                            index={index}
                                                            isMessageStreaming={message.is_streaming || false}
                                                            streamingQueryIndex={streamingQueryIndex}
                                                            currentDescription={currentDescription}
                                                            currentQuery={currentQuery}
                                                            isDescriptionStreaming={isDescriptionStreaming}
                                                            isQueryStreaming={isQueryStreaming}
                                                            queryState={queryStates[query.id] || { isExecuting: false, isExample: false }}
                                                            queryResult={ops.queryResults[query.id]}
                                                            isResultMinimized={ops.minimizedResults[query.id] || false}
                                                            isEditingQuery={ops.editingQueries[query.id] || false}
                                                            editedQueryText={ops.editedQueryTexts[query.id] || removeDuplicateQueries(query.query || '')}
                                                            dateColumns={ops.dateColumns}
                                                            setDateColumns={ops.setDateColumns}
                                                            expandedCells={ops.expandedCells}
                                                            setExpandedCells={ops.setExpandedCells}
                                                            expandedNodesRef={expandedNodesRef}
                                                            pageDataCacheRef={ops.pageDataCacheRef}
                                                            openDownloadMenu={ops.openDownloadMenu}
                                                            setOpenDownloadMenu={ops.setOpenDownloadMenu}
                                                            searchQuery={searchQuery}
                                                            searchResultRefs={searchResultRefs}
                                                            userId={userId}
                                                            userName={userName}
                                                            onSetIsEditingQuery={(qid, val) => ops.setEditingQueries(prev => ({ ...prev, [qid]: val }))}
                                                            onSetEditedQueryText={(qid, val) => ops.setEditedQueryTexts(prev => ({ ...prev, [qid]: val }))}
                                                            onEditQuery={onEditQuery}
                                                            onExecuteQuery={ops.executeQuery}
                                                            onRollback={ops.handleRollback}
                                                            onPageChange={ops.handlePageChange}
                                                            onExportData={ops.handleExportData}
                                                            onExportVisualization={ops.handleExportVisualization}
                                                            onToggleResultMinimize={ops.toggleResultMinimize}
                                                            onQueryUpdate={onQueryUpdate}
                                                            setQueryStates={setQueryStates}
                                                            abortControllerRef={ops.abortControllerRef}
                                                            queryTimeouts={queryTimeouts}
                                                            onVisualizationSaved={handleVisualizationSaved}
                                                        />
                                                    </div>
                                                );
                                            })}
                                        </div>
                                    )}

                                    {message.action_buttons && message.action_buttons.length > 0 && (
                                        <div className="flex flex-wrap gap-3 mt-4">
                                            {message.action_buttons.map(button => (
                                                <button
                                                    key={button.id}
                                                    onClick={() => buttonCallback ? buttonCallback(button.action, button.label) : undefined}
                                                    className={button.isPrimary ? 'neo-button' : 'neo-button-secondary'}
                                                >
                                                    {button.label}
                                                </button>
                                            ))}
                                        </div>
                                    )}

                                    {message.type === 'assistant' && message.content.length > 0 && message.id !== 'welcome-message' && (
                                        <div className="mt-4 group/tooltip">
                                            <div className="text-sm text-gray-700 flex flex-col sm:flex-row sm:flex-wrap sm:items-center gap-x-1">
                                                <span className="inline-flex items-center w-full sm:w-auto">
                                                    <svg className="w-4 h-4 mr-1 text-gray-700" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                                                    </svg>
                                                    This response was generated
                                                </span>
                                                <span className="inline-flex items-center gap-x-1 w-full sm:w-auto">
                                                    <span>in</span>
                                                    <div className="relative inline-block">
                                                        <span className={`font-semibold px-2 py-0.5 rounded cursor-help ${message.non_tech_mode ? 'bg-green-100 text-green-700' : 'bg-yellow-100 text-yellow-700'}`}>
                                                            {message.non_tech_mode ? 'Non-Technical' : 'Technical'}
                                                        </span>
                                                        <div className="absolute bottom-full left-1/2 transform -translate-x-1/2 mb-2 px-3 py-2 bg-gray-900 text-white text-sm rounded-lg opacity-0 invisible group-hover/tooltip:visible group-hover/tooltip:opacity-100 transition-all duration-200 w-64 z-50 pointer-events-none">
                                                            {message.non_tech_mode
                                                                ? 'Non-Technical Mode: Generates easy to understand explanations and queries without technical jargon.'
                                                                : 'Technical Mode: Generates raw queries and detailed explanations suitable for technical users.'}
                                                            <div className="absolute top-full left-1/2 transform -translate-x-1/2 -mt-1">
                                                                <div className="border-4 border-transparent border-t-gray-900" />
                                                            </div>
                                                        </div>
                                                    </div>
                                                    <span>Mode.</span>
                                                </span>
                                                <span className="inline-flex items-center gap-x-1 w-full sm:w-auto">
                                                    <span>You may change it from settings.</span>
                                                    {/* <button
                                                        onClick={() => buttonCallback?.('open_settings')}
                                                        className="text-blue-600 hover:text-blue-700 underline text-sm"
                                                    >
                                                        here
                                                    </button> */}
                                                </span>
                                            </div>
                                        </div>
                                    )}

                                    {message.type === 'assistant' && message.llm_model_name && (
                                        <div className="flex flex-row gap-1.5 mt-2 items-center">
                                            <Cpu className="w-4 h-4 text-gray-700" />
                                            <span className="text-sm text-gray-700 whitespace-nowrap flex-shrink-0">
                                                {message.llm_model_name}
                                            </span>
                                        </div>
                                    )}
                                </div>
                            )}
                        </div>

                        <div className={`text-[12px] text-gray-500 mt-1 ${message.type === 'user' ? 'text-right' : 'text-left'}`}>
                            {formatMessageTime(message)}
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}
