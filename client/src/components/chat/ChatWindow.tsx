import { ArrowDown, Loader2, Pin, RefreshCcw } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import toast from 'react-hot-toast';
import { useStream } from '../../contexts/StreamContext';
import axios from '../../services/axiosConfig';
import chatService from '../../services/chatService';
import analyticsService from '../../services/analyticsService';
import { Chat, Connection } from '../../types/chat';
import { Message } from '../../types/query';
import { ChatSettings } from '../../types/chat';
import { DashboardViewMode } from '../../types/dashboard';
import { useMessageSearch } from '../../hooks/useMessageSearch';
import { useScrollManagement } from '../../hooks/useScrollManagement';
import { useMessageFetching } from '../../hooks/useMessageFetching';
import { useConnectionActions } from '../../hooks/useConnectionActions';
import ConfirmationModal from '../modals/ConfirmationModal';
import ConnectionModal from '../modals/ConnectionModal';
import ChatHeader from './ChatHeader';
import MessageInput from './MessageInput';
import MessageTile from './MessageTile';
import SearchBar from './SearchBar';
import DashboardView from '../dashboard/DashboardView';

// ─── Helpers ──────────────────────────────────────────────────────────────────
const formatDateDivider = (dateString: string) => {
    const date = new Date(dateString);
    const today = new Date();
    const yesterday = new Date(today);
    yesterday.setDate(yesterday.getDate() - 1);
    if (date.toDateString() === today.toDateString()) return 'Today';
    if (date.toDateString() === yesterday.toDateString()) return 'Yesterday';
    return date.toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric' });
};

const groupMessagesByDate = (messages: Message[]) => {
    const groups: { [key: string]: Message[] } = {};
    const sorted = [...messages].sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime());
    sorted.forEach(message => {
        const date = new Date(message.created_at).toDateString();
        if (!groups[date]) groups[date] = [];
        groups[date].push(message);
    });
    return Object.fromEntries(
        Object.entries(groups).sort((a, b) => new Date(a[0]).getTime() - new Date(b[0]).getTime()),
    );
};

// ─── Props ────────────────────────────────────────────────────────────────────
interface ChatWindowProps {
    chat: Chat;
    isExpanded: boolean;
    messages: Message[];
    setMessages: React.Dispatch<React.SetStateAction<Message[]>>;
    onSendMessage: (message: string, llmModel?: string) => Promise<void>;
    onEditMessage: (id: string, content: string) => void;
    onClearChat: () => void;
    onCloseConnection: () => void;
    onEditConnection?: (id: string, connection: Connection, settings: ChatSettings) => Promise<{ success: boolean; error?: string; updatedChat?: Chat }>;
    onConnectionStatusChange?: (chatId: string, isConnected: boolean, from: string) => void;
    isConnected: boolean;
    onCancelStream: () => void;
    onRefreshSchema: () => Promise<void>;
    onCancelRefreshSchema: () => void;
    checkSSEConnection: () => Promise<void>;
    onUpdateSelectedCollections?: (chatId: string, selectedCollections: string) => Promise<void>;
    onEditConnectionFromChatWindow?: () => void;
    userId?: string;
    userName?: string;
    recoVersion?: number;
    llmModels?: any[];
    isLoadingModels?: boolean;
}

interface QueryState {
    isExecuting: boolean;
    isExample: boolean;
}

// ─── Component ────────────────────────────────────────────────────────────────
export default function ChatWindow({
    chat, onEditMessage, isExpanded,
    messages, setMessages,
    onSendMessage, onClearChat, onCloseConnection,
    onEditConnection, onConnectionStatusChange,
    isConnected, onCancelStream,
    onRefreshSchema, onCancelRefreshSchema,
    checkSSEConnection,
    onUpdateSelectedCollections, onEditConnectionFromChatWindow,
    userId, userName, recoVersion,
    llmModels = [], isLoadingModels = false,
}: ChatWindowProps) {
    const { streamId } = useStream();
    const queryTimeouts = useRef<Record<string, NodeJS.Timeout>>({});

    // ── Local UI state ────────────────────────────────────────────────────────
    const [editingMessageId, setEditingMessageId] = useState<string | null>(null);
    const [editInput, setEditInput] = useState('');
    const [showClearConfirm, setShowClearConfirm] = useState(false);
    const [showRefreshSchema, setShowRefreshSchema] = useState(false);
    const [showCloseConfirm, setShowCloseConfirm] = useState(false);
    const [queryStates, setQueryStates] = useState<Record<string, QueryState>>({});
    const [showEditConnection, setShowEditConnection] = useState(false);
    const [openWithSettingsTab, setOpenWithSettingsTab] = useState(false);
    const [_isMessageSending] = useState(false);
    const [viewMode, setViewMode] = useState<'chats' | 'pinned'>('chats');
    const [dashboardViewMode, setDashboardViewMode] = useState<DashboardViewMode>('chat');
    const [selectedLLMModel, setSelectedLLMModel] = useState<string | undefined>(undefined);
    const [recommendations, setRecommendations] = useState<string[]>([]);
    const [isLoadingRecommendations, setIsLoadingRecommendations] = useState(false);
    const [inputPrefill, setInputPrefill] = useState<string | null>(null);
    const [_shimmerTexts, setShimmerTexts] = useState<string[]>([]);
    const lastRecoKeyRef = useRef<string | null>(null);
    const [showEditQueryConfirm, setShowEditQueryConfirm] = useState<{
        show: boolean; messageId: string | null; queryId: string | null; query: string | null;
    }>({ show: false, messageId: null, queryId: null, query: null });

    // ── Search ────────────────────────────────────────────────────────────────
    const search = useMessageSearch({ messages });

    // isLoadingMessages is hoisted here so both scroll + fetching hooks share it
    const [isLoadingMessages, setIsLoadingMessages] = useState(false);

    // ── Scroll management ─────────────────────────────────────────────────────
    const scroll = useScrollManagement({ messages, isLoadingMessages, viewMode });

    // ── Message fetching ──────────────────────────────────────────────────────
    const fetching = useMessageFetching({
        chatId: chat.id,
        pageSize: 25,
        setMessages,
        scrollToBottom: scroll.scrollToBottom,
        preserveScroll: scroll.preserveScroll,
        chatContainerRef: scroll.chatContainerRef,
        messageUpdateSource: scroll.messageUpdateSource,
        isLoadingOldMessages: scroll.isLoadingOldMessages,
        isInitialLoad: scroll.isInitialLoad,
        viewMode,
        showSearch: search.showSearch,
        setShowSearch: search.setShowSearch,
        setSearchQuery: search.setSearchQuery,
        setSearchResults: search.setSearchResults,
        setCurrentSearchIndex: search.setCurrentSearchIndex,
        isLoadingMessages,
        setIsLoadingMessages,
    });

    // ── Connection actions ────────────────────────────────────────────────────
    const conn = useConnectionActions({
        chatId: chat.id,
        checkSSEConnection,
        onCloseConnection,
        onConnectionStatusChange,
    });

    // ── LLM model ─────────────────────────────────────────────────────────────
    useEffect(() => {
        if (chat?.preferred_llm_model) { setSelectedLLMModel(chat.preferred_llm_model); return; }
        if (llmModels?.length > 0) {
            const def = llmModels.find((m: any) => m.default === true);
            if (def) setSelectedLLMModel(def.id);
        }
    }, [chat?.id, chat?.preferred_llm_model, llmModels]);

    useEffect(() => {
        if (isConnected) conn.setIsConnecting(false);
    }, [isConnected, conn]);

    // ── Recommendations ───────────────────────────────────────────────────────
    useEffect(() => {
        if (!chat?.id) return;
        let cancelled = false;
        const key = `${chat.id}-${recoVersion}`;
        if (lastRecoKeyRef.current === key) return;

        setIsLoadingRecommendations(true);
        setShimmerTexts([
            'This is a placeholder - good data',
            'This is a placeholder for the recommendations',
            'This also is a placeholder - shows data ',
            'This is a placeholder for the recommendations very good',
        ]);

        (async () => {
            try {
                lastRecoKeyRef.current = key;
                const resp = await chatService.getQueryRecommendations(chat.id, streamId || undefined);
                if (!cancelled) {
                    setRecommendations(resp.success && resp.data?.recommendations
                        ? resp.data.recommendations.map((r: any) => r.text)
                        : []);
                }
            } catch {
                if (!cancelled) setRecommendations([]);
            } finally {
                if (!cancelled) setIsLoadingRecommendations(false);
            }
        })();
        return () => { cancelled = true; };
    }, [recoVersion, chat?.id, streamId]);

    // ── Restore scroll on view mode switch ───────────────────────────────────
    useEffect(() => {
        if (viewMode === 'pinned' && chat?.id && fetching.pinnedMessages.length === 0) {
            fetching.fetchPinnedMessages();
        }
        setTimeout(() => {
            if (scroll.chatContainerRef.current) {
                scroll.chatContainerRef.current.scrollTop = scroll.scrollPositions.current[viewMode];
            }
        }, 50);
    }, [viewMode, chat?.id]); // eslint-disable-line react-hooks/exhaustive-deps

    // ── Message helpers ───────────────────────────────────────────────────────
    const setMessage = useCallback((message: Message) => {
        setMessages(prev => prev.map(m => m.id === message.id ? message : m));
        if (message.is_pinned !== undefined) fetching.fetchPinnedMessages();
    }, [setMessages, fetching]);

    const handleEditMessage = useCallback((id: string) => {
        scroll.messageUpdateSource.current = 'query';
        const msg = messages.find(m => m.id === id);
        if (msg) { setEditingMessageId(id); setEditInput(msg.content); }
        setTimeout(() => { scroll.messageUpdateSource.current = null; }, 200);
    }, [messages, scroll.messageUpdateSource]);

    const handleCancelEdit = useCallback(() => {
        scroll.messageUpdateSource.current = 'query';
        setEditingMessageId(null);
        setEditInput('');
        setTimeout(() => { scroll.messageUpdateSource.current = null; }, 200);
    }, [scroll.messageUpdateSource]);

    const handleLLMModelChange = useCallback(async (modelId: string) => {
        setSelectedLLMModel(modelId);
        try {
            await axios.patch(
                `${import.meta.env.VITE_API_URL}/chats/${chat.id}`,
                { preferred_llm_model: modelId },
                { withCredentials: true, headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` } },
            );
            if (chat) chat.preferred_llm_model = modelId;
        } catch (error) {
            console.error('Failed to save preferred LLM model:', error);
        }
    }, [chat]);

    const handleSendMessage = useCallback(async (content: string) => {
        if (chat?.id) analyticsService.trackMessageSent(chat.id, content.length, userId || '', userName || '');
        await onSendMessage(content, selectedLLMModel);
    }, [chat?.id, onSendMessage, selectedLLMModel, userId, userName]);

    const handleSaveEdit = useCallback((id: string, content: string) => {
        if (!content.trim()) { setEditingMessageId(null); setEditInput(''); return; }
        scroll.messageUpdateSource.current = 'query';
        setRecommendations([]);
        const idx = messages.findIndex(m => m.id === id);
        if (idx === -1) return;
        if (chat?.id) analyticsService.trackMessageEdited(chat.id, id, userId || '', userName || '');
        onEditMessage(id, content);
        setMessages(prev => {
            const upd = [...prev];
            upd[idx] = { ...upd[idx], content: content.trim() };
            return upd;
        });
        setTimeout(() => { scroll.messageUpdateSource.current = null; }, 200);
        setEditingMessageId(null);
        setEditInput('');
    }, [messages, setMessages, chat?.id, onEditMessage, userId, userName, scroll.messageUpdateSource]);

    const handleMessageSubmit = useCallback(async (content: string) => {
        scroll.messageUpdateSource.current = 'new';
        await handleSendMessage(content);
        setTimeout(() => scroll.scrollToBottom(true), 100);
        setTimeout(() => { scroll.messageUpdateSource.current = null; }, 300);
    }, [handleSendMessage, scroll]);

    const handleEditQuery = useCallback((messageId: string, queryId: string, query: string) => {
        setShowEditQueryConfirm({ show: true, messageId, queryId, query });
    }, []);

    const handleConfirmQueryEdit = useCallback(async () => {
        const { messageId, queryId, query } = showEditQueryConfirm;
        if (!messageId || !queryId || !query) return;
        try {
            const response = await chatService.editQuery(chat.id, messageId, queryId, query);
            if (response.success) {
                scroll.preserveScroll(scroll.chatContainerRef.current, () => {
                    setMessages(prev => prev.map(msg => {
                        if (msg.id !== messageId) return msg;
                        return { ...msg, queries: msg.queries?.map(q => q.id === queryId ? { ...q, query: query!, is_edited: true, original_query: q.query } : q) };
                    }));
                });
                toast.success('Query updated successfully');
            }
        } catch (error) {
            toast.error('Failed to update query: ' + error);
        } finally {
            setShowEditQueryConfirm({ show: false, messageId: null, queryId: null, query: null });
        }
    }, [showEditQueryConfirm, chat.id, setMessages, scroll]);

    const handleFixErrorAction = useCallback((message: Message) => {
        const errs = message.queries?.filter(q => q.error) || [];
        if (!errs.length) { toast.error('No errors found to fix'); return; }
        let content = 'Fix Errors:\n';
        errs.forEach(q => { content += `Query: '${q.query}' faced an error: '${q.error?.message || 'Unknown error'}'.\n`; });
        onSendMessage(content);
    }, [onSendMessage]);

    const handleFixRollbackErrorAction = useCallback((message: Message) => {
        const errs = message.queries?.filter(q => q.error) || [];
        if (!errs.length) { toast.error('No errors found to fix'); return; }
        let content = 'Fix Rollback Errors:';
        errs.forEach(q => { content += `Query: '${q.rollback_query || q.rollback_dependent_query}' faced an error: '${q.error?.message || 'Unknown error'}'.\n`; });
        onSendMessage(content);
    }, [onSendMessage]);

    const handleButtonCallback = useCallback((message: Message, action: string, label?: string) => {
        if (action === 'refresh_schema') { setShowRefreshSchema(true); }
        else if (action === 'fix_error') { handleFixErrorAction(message); }
        else if (action === 'fix_rollback_error') { handleFixRollbackErrorAction(message); }
        else if (action === 'try_again') {
            const userMsg = messages.find(m => m.id === message.user_message_id || (m.type === 'user' && m.created_at < message.created_at));
            if (userMsg) handleSendMessage(userMsg.content);
            else toast.error('Could not find original message to retry');
        } else if (action === 'open_settings') { setOpenWithSettingsTab(true); setShowEditConnection(true); }
        else { handleSendMessage(`${label}`); }
    }, [messages, handleSendMessage, handleFixErrorAction, handleFixRollbackErrorAction]);

    const handleWelcomeButtonCallback = useCallback((action: string) => {
        if (action === 'refresh_schema') { setShowRefreshSchema(true); }
        else if (action === 'try_again') {
            const idx = messages.findIndex(m => m.is_streaming);
            if (idx > 0 && messages[idx - 1]?.type === 'user') handleSendMessage(messages[idx - 1].content);
            else toast.error('Could not find original message to retry');
        } else if (action === 'open_settings') { setOpenWithSettingsTab(true); setShowEditConnection(true); }
    }, [messages, handleSendMessage]);

    const handleConfirmClearChat = useCallback(async () => {
        if (chat?.id) analyticsService.trackChatCleared(chat.id, userId || '', userName || '');
        await onClearChat();
        fetching.setPinnedMessages([]);
        setShowClearConfirm(false);
    }, [chat?.id, onClearChat, userId, userName, fetching]);

    const handleCancelStreamClick = useCallback(() => {
        if (chat?.id) analyticsService.trackQueryCancelled(chat.id, userId || '', userName || '');
        onCancelStream();
    }, [chat?.id, onCancelStream, userId, userName]);

    const handleConfirmRefreshSchema = useCallback(async () => {
        if (chat?.id) analyticsService.trackSchemaRefreshed(chat.id, chat.connection.database, userId || '', userName || '');
        await onRefreshSchema();
        setShowRefreshSchema(false);
    }, [chat?.id, chat?.connection.database, onRefreshSchema, userId, userName]);

    const handleCancelRefreshSchema = useCallback(async () => {
        if (chat?.id) analyticsService.trackSchemaCancelled(chat.id, chat.connection.database, userId || '', userName || '');
        await onCancelRefreshSchema();
        setShowRefreshSchema(false);
    }, [chat?.id, chat?.connection.database, onCancelRefreshSchema, userId, userName]);

    // ── Render ────────────────────────────────────────────────────────────────
    const displayMessages = viewMode === 'chats' ? messages : fetching.pinnedMessages;
    const dateGroups = groupMessagesByDate(displayMessages);

    return (
        <div className={`flex-1 flex flex-col h-screen max-h-screen overflow-hidden transition-all duration-300 ${isExpanded ? 'md:ml-80' : 'md:ml-20'}`}>
            <div className="relative">
                <ChatHeader
                    chat={chat}
                    isConnecting={conn.isConnecting}
                    isConnected={isConnected}
                    onClearChat={() => setShowClearConfirm(true)}
                    onEditConnection={() => onEditConnectionFromChatWindow ? onEditConnectionFromChatWindow() : setShowEditConnection(true)}
                    onShowCloseConfirm={() => setShowCloseConfirm(true)}
                    onReconnect={conn.handleReconnect}
                    setShowRefreshSchema={() => setShowRefreshSchema(true)}
                    onToggleSearch={search.handleToggleSearch}
                    viewMode={viewMode}
                    onViewModeChange={setViewMode}
                    dashboardViewMode={dashboardViewMode}
                    onDashboardViewModeChange={setDashboardViewMode}
                />
                {search.showSearch && (
                    <SearchBar
                        onSearch={search.performSearch}
                        onClose={search.handleToggleSearch}
                        onNavigateUp={search.navigateSearchUp}
                        onNavigateDown={search.navigateSearchDown}
                        currentResultIndex={search.currentSearchIndex}
                        totalResults={search.searchResults.length}
                        initialQuery={search.searchQuery}
                    />
                )}
            </div>

            <div className="relative flex-1 min-h-0">
                {/* Dashboard View */}
                <div className={`absolute inset-0 flex flex-col transition-opacity duration-200 ease-in-out ${dashboardViewMode === 'dashboard' ? 'opacity-100 z-[2]' : 'opacity-0 z-0 pointer-events-none'}`}>
                    <DashboardView chatId={chat.id} streamId={streamId} isConnected={isConnected} onReconnect={conn.handleReconnect} />
                </div>

                {/* Chat View */}
                <div className={`absolute inset-0 flex flex-col transition-opacity duration-200 ease-in-out ${dashboardViewMode === 'chat' ? 'opacity-100 z-[2]' : 'opacity-0 z-0 pointer-events-none'}`}>
                    <div
                        ref={scroll.chatContainerRef}
                        data-chat-container
                        className={`flex-1 overflow-y-auto overflow-x-hidden bg-[#FFDB58]/10 relative scroll-smooth ${viewMode === 'chats' ? 'pb-24 md:pb-32' : 'pb-8'} -mt-6 md:mt-0 flex-shrink`}
                    >
                        {viewMode === 'chats' ? (
                            <div ref={fetching.loadingRef} className="h-20 flex items-center justify-center">
                                {fetching.isLoadingMessages && (
                                    <div className="flex items-center justify-center gap-2">
                                        <Loader2 className="w-4 h-4 animate-spin" />
                                        <span className="text-sm text-gray-600">Loading more messages...</span>
                                    </div>
                                )}
                            </div>
                        ) : <div className="h-20" />}

                        <div className={`max-w-5xl mx-auto px-4 pt-16 md:pt-0 md:px-2 xl:px-0 transition-all duration-300 ${isExpanded ? 'md:ml-6 lg:ml-6 xl:mx-8 [@media(min-width:1760px)]:ml-[4rem] [@media(min-width:1920px)]:ml-[8.4rem]' : 'md:ml-[19rem] xl:mx-auto'}`}>
                            {Object.entries(dateGroups).map(([date, dateMessages], groupIndex) => (
                                <div key={date}>
                                    <div className={`flex items-center justify-center ${groupIndex === 0 ? 'mb-4' : 'my-6'}`}>
                                        <div className="px-4 py-2 bg-white text-sm font-medium text-black border-2 border-black shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] rounded-full">
                                            {formatDateDivider(date)}
                                        </div>
                                    </div>

                                    {dateMessages.map((message, index) => (
                                        <MessageTile
                                            key={message.id}
                                            checkSSEConnection={checkSSEConnection}
                                            chatId={chat.id}
                                            message={message}
                                            setMessage={setMessage}
                                            onEdit={handleEditMessage}
                                            editingMessageId={editingMessageId}
                                            editInput={editInput}
                                            setEditInput={setEditInput}
                                            onSaveEdit={handleSaveEdit}
                                            onCancelEdit={handleCancelEdit}
                                            queryStates={queryStates}
                                            setQueryStates={setQueryStates}
                                            queryTimeouts={queryTimeouts}
                                            isFirstMessage={index === 0}
                                            onPinMessage={(msgId, shouldPin) => fetching.handlePinMessage(msgId, shouldPin, messages)}
                                            onQueryUpdate={scroll.handleQueryUpdate}
                                            onEditQuery={handleEditQuery}
                                            userId={userId || ''}
                                            userName={userName || ''}
                                            searchQuery={search.showSearch ? search.searchQuery : ''}
                                            searchResultRefs={search.searchResultRefs}
                                            buttonCallback={(action, label) => handleButtonCallback(message, action, label)}
                                        />
                                    ))}

                                    {/* Inline recommendations after the last AI message in the last date group */}
                                    {viewMode === 'chats' && (() => {
                                        const isLastGroup = groupIndex === Object.keys(dateGroups).length - 1;
                                        const isStreaming = messages.length > 0 && messages[messages.length - 1].is_streaming === true;
                                        if (!isLastGroup || isStreaming || isLoadingRecommendations || recommendations.length === 0) return null;
                                        const hasAI = dateMessages.some(m => m.type === 'assistant');
                                        if (!hasAI) return null;
                                        return (
                                            <div className="mt-6 md:mt-4 mb-6 md:mb-2">
                                                <div className="flex items-center gap-2 mb-3">
                                                    <span className="text-sm text-gray-600 font-medium">You may try asking:</span>
                                                </div>
                                                <div className="flex flex-wrap gap-2 items-center">
                                                    {recommendations.slice(0, 4).map((text, idx) => (
                                                        <button
                                                            key={`${text}-${idx}`}
                                                            onClick={async () => {
                                                                analyticsService.trackRecommendationChipClick(chat.id, text, userId || '', userName || '');
                                                                setRecommendations([]);
                                                                await handleMessageSubmit(text);
                                                            }}
                                                            className="inline-flex items-center gap-2 px-3 py-2 bg-gray-100 hover:bg-gray-200 border-2 border-gray-300 hover:border-gray-400 rounded-full text-sm font-medium text-black transition-all duration-200 max-w-base truncate"
                                                            title={text}
                                                        >
                                                            <span className="truncate">{text}</span>
                                                        </button>
                                                    ))}
                                                </div>
                                            </div>
                                        );
                                    })()}
                                </div>
                            ))}

                            {displayMessages.length === 0 && (
                                <div className="flex flex-col items-center justify-center h-full">
                                    <div className="px-4 py-2 bg-white text-sm font-medium text-black border-2 border-black shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] rounded-full">
                                        {formatDateDivider(new Date().toISOString())}
                                    </div>
                                    {viewMode === 'chats' ? (
                                        <>
                                            <MessageTile
                                                key="welcome-message"
                                                checkSSEConnection={checkSSEConnection}
                                                chatId={chat.id}
                                                message={{
                                                    id: 'welcome-message',
                                                    type: 'assistant',
                                                    content: `Hi ${userName || 'There'}! I am your Data Copilot. You can ask me anything about your data and i will understand your request & respond. You can start by asking me what all data is stored or try recommendations.`,
                                                    queries: [],
                                                    action_buttons: [],
                                                    created_at: new Date().toISOString(),
                                                    updated_at: new Date().toISOString(),
                                                }}
                                                setMessage={setMessage}
                                                onEdit={handleEditMessage}
                                                editingMessageId={editingMessageId}
                                                onPinMessage={(msgId, shouldPin) => fetching.handlePinMessage(msgId, shouldPin, messages)}
                                                editInput={editInput}
                                                setEditInput={setEditInput}
                                                onSaveEdit={handleSaveEdit}
                                                onCancelEdit={handleCancelEdit}
                                                queryStates={queryStates}
                                                setQueryStates={setQueryStates}
                                                queryTimeouts={queryTimeouts}
                                                isFirstMessage={false}
                                                onQueryUpdate={scroll.handleQueryUpdate}
                                                onEditQuery={handleEditQuery}
                                                userId={userId || ''}
                                                userName={userName || ''}
                                                searchQuery={search.showSearch ? search.searchQuery : ''}
                                                searchResultRefs={search.searchResultRefs}
                                                buttonCallback={handleWelcomeButtonCallback}
                                            />
                                            {!isLoadingRecommendations && recommendations.length > 0 && (
                                                <div className="mt-6 md:mt-4 mb-6 md:mb-2">
                                                    <div className="flex items-center gap-2 mb-3">
                                                        <span className="text-sm text-gray-600 font-medium">You may try asking:</span>
                                                    </div>
                                                    <div className="flex flex-wrap gap-2 items-center">
                                                        {recommendations.slice(0, 4).map((text, idx) => (
                                                            <button
                                                                key={`${text}-${idx}`}
                                                                onClick={async () => {
                                                                    analyticsService.trackRecommendationChipClick(chat.id, text, userId || '', userName || '');
                                                                    setRecommendations([]);
                                                                    await handleMessageSubmit(text);
                                                                }}
                                                                className="inline-flex items-center gap-2 px-3 py-2 bg-gray-100 hover:bg-gray-200 border-2 border-gray-300 hover:border-gray-400 rounded-full text-sm font-medium text-black transition-all duration-200 max-w-base truncate"
                                                                title={text}
                                                            >
                                                                <span className="truncate">{text}</span>
                                                            </button>
                                                        ))}
                                                    </div>
                                                </div>
                                            )}
                                        </>
                                    ) : (
                                        <div className="text-center text-gray-600 mt-40">
                                            <Pin className="w-12 h-12 mx-auto mb-4 text-gray-400 rotate-45" />
                                            <p className="text-lg font-medium">No Pinned Messages</p>
                                            <p className="text-sm mt-2">Pin frequently asked or important messages to access them quickly</p>
                                        </div>
                                    )}
                                </div>
                            )}
                        </div>
                        <div ref={scroll.messagesEndRef} />

                        {scroll.showScrollButton && (
                            <button
                                onClick={() => scroll.scrollToBottom(true)}
                                className="fixed bottom-28 right-4 md:right-6 p-3 bg-black text-white rounded-full shadow-lg hover:bg-gray-800 transition-all neo-border z-40"
                                title="Scroll to bottom"
                            >
                                <ArrowDown className="w-6 h-6" />
                            </button>
                        )}
                    </div>
                </div>
            </div>

            {viewMode === 'chats' && dashboardViewMode === 'chat' && (
                <MessageInput
                    isConnected={isConnected}
                    onSendMessage={handleMessageSubmit}
                    isExpanded={isExpanded}
                    isDisabled={_isMessageSending}
                    chatId={chat.id}
                    userId={userId || ''}
                    userName={userName || ''}
                    isStreaming={messages.some(m => m.is_streaming)}
                    onCancelStream={handleCancelStreamClick}
                    prefillText={inputPrefill || ''}
                    onConsumePrefill={() => setInputPrefill(null)}
                    onModelChange={handleLLMModelChange}
                    selectedModel={selectedLLMModel}
                    llmModels={llmModels}
                    isLoadingModels={isLoadingModels}
                />
            )}

            {showRefreshSchema && (
                <ConfirmationModal
                    icon={<RefreshCcw className="w-6 h-6 text-black" />}
                    themeColor="black"
                    title="Refresh Knowledge Base"
                    buttonText="Refresh"
                    message="This action will refetch the schema from the data source and update the knowledge base. This may take 2-3 minutes depending on the size of your data."
                    onConfirm={handleConfirmRefreshSchema}
                    onCancel={handleCancelRefreshSchema}
                />
            )}

            {showClearConfirm && (
                <ConfirmationModal
                    title="Clear Chat"
                    message="Are you sure you want to clear all chat messages? This action cannot be undone."
                    onConfirm={handleConfirmClearChat}
                    onCancel={() => setShowClearConfirm(false)}
                />
            )}

            {showCloseConfirm && (
                <ConfirmationModal
                    title="Disconnect Connection"
                    message="Are you sure you want to disconnect from this database? You can reconnect anytime."
                    onConfirm={conn.handleDisconnect}
                    onCancel={() => setShowCloseConfirm(false)}
                />
            )}

            {showEditConnection && (
                <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/50">
                    <ConnectionModal
                        initialData={chat}
                        initialTab={openWithSettingsTab ? 'settings' : undefined}
                        onClose={updatedChat => {
                            setShowEditConnection(false);
                            setOpenWithSettingsTab(false);
                            if (updatedChat && onEditConnection) onEditConnection(chat.id, updatedChat.connection, updatedChat.settings);
                        }}
                        onEdit={async (data, autoExecuteQuery) => {
                            const result = await onEditConnection?.(chat.id, data!, autoExecuteQuery!);
                            return { success: result?.success || false, error: result?.error, updatedChat: result?.success ? result.updatedChat : undefined };
                        }}
                        onSubmit={async (data, autoExecuteQuery) => {
                            const result = await onEditConnection?.(chat.id, data, autoExecuteQuery);
                            return { success: result?.success || false, error: result?.error };
                        }}
                        onUpdateSelectedCollections={onUpdateSelectedCollections}
                        onRefreshSchema={handleConfirmRefreshSchema}
                    />
                </div>
            )}

            {showEditQueryConfirm.show && (
                <ConfirmationModal
                    title="Edit Query"
                    message="Are you sure you want to edit this query? This may affect the execution results."
                    onConfirm={handleConfirmQueryEdit}
                    onCancel={() => setShowEditQueryConfirm({ show: false, messageId: null, queryId: null, query: null })}
                />
            )}
        </div>
    );
}
