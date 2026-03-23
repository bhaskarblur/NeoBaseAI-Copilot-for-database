import { useCallback, useEffect, useRef, useState } from 'react';
import toast from 'react-hot-toast';
import { EventSourcePolyfill } from 'event-source-polyfill';
import { useStream } from '../contexts/StreamContext';
import chatService from '../services/chatService';
import { Message, QueryResult, LoadingStep } from '../types/query';

interface UseSSEConnectionParams {
    selectedConnectionId: string | undefined;
    messages: Message[];
    temporaryMessage: Message | null;
    setMessages: React.Dispatch<React.SetStateAction<Message[]>>;
    setTemporaryMessage: (msg: Message | null) => void;
    setRecoRefreshToken: React.Dispatch<React.SetStateAction<number>>;
    onConnectionStatusChange: (chatId: string, isConnected: boolean, from: string) => void;
}

/** Encapsulates EventSource setup, teardown, and all SSE event handling. */
export function useSSEConnection({
    selectedConnectionId,
    messages,
    temporaryMessage,
    setMessages,
    setTemporaryMessage,
    setRecoRefreshToken,
    onConnectionStatusChange,
}: UseSSEConnectionParams) {
    const { streamId, setStreamId, generateStreamId } = useStream();
    const [eventSource, setEventSource] = useState<EventSourcePolyfill | null>(null);
    const [isSSEReconnecting, setIsSSEReconnecting] = useState(false);
    const [isSettingUpSSE, setIsSettingUpSSE] = useState(false);
    const streamingMessageTimeouts = useRef<Record<string, NodeJS.Timeout>>({});

    // Keep a stable ref so SSE handlers don't need `messages` in their effect deps
    const messagesRef = useRef(messages);
    useEffect(() => { messagesRef.current = messages; }, [messages]);

    // ── Typing animation helpers ───────────────────────────────────────────────
    const animateTyping = useCallback(async (text: string, messageId: string) => {
        setMessages(prev => prev.map(msg =>
            msg.id === messageId
                ? { ...msg, content: '', is_streaming: true, action_buttons: msg.action_buttons }
                : msg,
        ));
        const words = text.split(' ');
        for (const word of words) {
            await new Promise(resolve => setTimeout(resolve, 5 + Math.random() * 5));
            setMessages(prev => prev.map(msg =>
                msg.id === messageId
                    ? { ...msg, content: msg.content + (msg.content ? ' ' : '') + word, action_buttons: msg.action_buttons }
                    : msg,
            ));
        }
        setMessages(prev => prev.map(msg =>
            msg.id === messageId ? { ...msg, is_streaming: false, action_buttons: msg.action_buttons } : msg,
        ));
    }, [setMessages]);

    const animateQueryTyping = useCallback(async (messageId: string, queries: QueryResult[], nonTechMode = false) => {
        if (!queries?.length) return;

        if (nonTechMode) {
            setMessages(prev => prev.map(msg =>
                msg.id === messageId
                    ? { ...msg, queries, is_streaming: false, action_buttons: msg.action_buttons }
                    : msg,
            ));
            return;
        }

        setMessages(prev => prev.map(msg =>
            msg.id === messageId
                ? { ...msg, queries: queries.map(q => ({ ...q, query: '' })), is_streaming: true, action_buttons: msg.action_buttons }
                : msg,
        ));

        for (const query of queries) {
            for (const word of query.query.split(' ')) {
                await new Promise(resolve => setTimeout(resolve, 5 + Math.random() * 5));
                setMessages(prev => prev.map(msg => {
                    if (msg.id !== messageId) return msg;
                    const updatedQueries = [...(msg.queries || [])];
                    const idx = updatedQueries.findIndex(q => q.id === query.id);
                    if (idx !== -1) {
                        updatedQueries[idx] = {
                            ...updatedQueries[idx],
                            query: updatedQueries[idx].query + (updatedQueries[idx].query ? ' ' : '') + word,
                        };
                    }
                    return { ...msg, queries: updatedQueries, action_buttons: msg.action_buttons };
                }));
            }
        }
        setMessages(prev => prev.map(msg =>
            msg.id === messageId ? { ...msg, is_streaming: false, action_buttons: msg.action_buttons } : msg,
        ));
    }, [setMessages]);

    // ── SSE setup ─────────────────────────────────────────────────────────────
    const setupSSEConnection = useCallback(async (chatId: string): Promise<string> => {
        if (isSettingUpSSE) return streamId || '';

        try {
            setIsSettingUpSSE(true);
            if (eventSource) { eventSource.close(); setEventSource(null); }

            let localStreamId = streamId;
            if (!localStreamId) { localStreamId = generateStreamId(); setStreamId(localStreamId); }

            await new Promise(resolve => setTimeout(resolve, 100));

            const sse = new EventSourcePolyfill(
                `${import.meta.env.VITE_API_URL}/chats/${chatId}/stream?stream_id=${localStreamId}`,
                {
                    withCredentials: true,
                    headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` },
                },
            );

            sse.onopen = () => { setIsSSEReconnecting(true); };
            sse.onmessage = (event) => {
                try {
                    const data = JSON.parse((event as any).data);
                    if (data.event === 'db-connected') onConnectionStatusChange(chatId, true, 'app-sse-connection');
                    else if (data.event === 'db-disconnected') onConnectionStatusChange(chatId, false, 'app-sse-connection');
                } catch { /* ignore parse errors on basic handler */ }
            };
            sse.onerror = () => {
                setTimeout(() => {
                    if (!isSSEReconnecting) {
                        onConnectionStatusChange(chatId, false, 'sse-error');
                        if (sse.readyState === EventSource.CLOSED) setEventSource(null);
                    } else {
                        setIsSSEReconnecting(false);
                    }
                }, 100);
            };

            setEventSource(sse);
            return localStreamId;
        } catch (error) {
            console.error('Failed to setup SSE connection:', error);
            toast.error('Failed to setup SSE connection');
            throw error;
        } finally {
            setIsSettingUpSSE(false);
        }
    }, [eventSource, streamId, generateStreamId, setStreamId, onConnectionStatusChange, isSettingUpSSE, isSSEReconnecting]);

    const checkSSEConnection = useCallback(async () => {
        if (eventSource?.readyState === EventSource.OPEN) return;
        await setupSSEConnection(selectedConnectionId || '');
    }, [eventSource, selectedConnectionId, setupSSEConnection]);

    // ── Full SSE message handler ───────────────────────────────────────────────
    useEffect(() => {
        if (!eventSource) return;

        const handleSSEMessage = async (_this: EventSource, e: any) => {
            try {
                const response = JSON.parse(e.data);

                switch (response.event) {
                    case 'db-connected':
                        if (selectedConnectionId) onConnectionStatusChange(selectedConnectionId, true, 'app-sse-connection');
                        break;

                    case 'db-disconnected':
                        if (selectedConnectionId) onConnectionStatusChange(selectedConnectionId, false, 'app-sse-connection');
                        break;

                    case 'system-message':
                        if (response.data) {
                            const sysMsg: Message = {
                                id: response.data.message_id,
                                type: 'assistant',
                                content: response.data.content,
                                action_buttons: response.data.action_buttons || [],
                                queries: [],
                                is_loading: false,
                                loading_steps: [],
                                is_streaming: false,
                                created_at: response.data.created_at || new Date().toISOString(),
                            };
                            setMessages(prev => [...prev.filter(m => !m.is_streaming), sysMsg]);
                            setTemporaryMessage(null);
                        }
                        break;

                    case 'ai-response-step': {
                        const isFirst = !temporaryMessage || (temporaryMessage.loading_steps?.length ?? 0) <= 1;
                        await new Promise(resolve => setTimeout(resolve, isFirst ? 500 : 200));
                        if (streamId && streamingMessageTimeouts.current[streamId]) {
                            clearTimeout(streamingMessageTimeouts.current[streamId]);
                            delete streamingMessageTimeouts.current[streamId];
                        }
                        setMessages(prev => {
                            const candidates = prev.filter(m => m.is_streaming);
                            const streaming = candidates[candidates.length - 1];
                            if (!streaming) return prev;
                            if (streaming.loading_steps != null && streaming.loading_steps.length > 0 && response.data === 'NeoBase is analyzing your request..') return prev;
                            const updated: Message = {
                                ...streaming,
                                loading_steps: [
                                    ...(streaming.loading_steps || []).map((s: LoadingStep) => ({ ...s, done: true })),
                                    { text: response.data, done: false },
                                ],
                            };
                            return prev.map(m => m.id === streaming.id ? updated : m);
                        });
                        break;
                    }

                    case 'ai-response':
                        if (response.data) {
                            const isEdited = response.data.user_message_id &&
                                messagesRef.current.some(m => m.id === response.data.user_message_id && m.is_edited);
                            const existingIdx = isEdited
                                ? messagesRef.current.findIndex(m => m.type === 'assistant' && m.user_message_id === response.data.user_message_id)
                                : -1;

                            if (isEdited && existingIdx !== -1) {
                                const existing = messagesRef.current[existingIdx];
                                setMessages(prev => prev.map((m, i) => i !== existingIdx ? m : {
                                    ...m,
                                    content: '',
                                    action_buttons: response.data.action_buttons,
                                    queries: response.data.non_tech_mode
                                        ? response.data.queries || []
                                        : (response.data.queries || []).map((q: QueryResult) => ({ ...q, query: '' })),
                                    is_loading: false, loading_steps: [],
                                    is_streaming: !response.data.non_tech_mode,
                                    user_message_id: response.data.user_message_id,
                                    updated_at: new Date().toISOString(),
                                    action_at: response.data.action_at,
                                    non_tech_mode: response.data.non_tech_mode,
                                    llm_model: response.data.llm_model,
                                    llm_model_name: response.data.llm_model_name,
                                }));
                                await animateTyping(response.data.content, existing.id);
                                if (response.data.queries?.length) await animateQueryTyping(existing.id, response.data.queries, response.data.non_tech_mode);
                                setMessages(prev => prev.map((m, i) => i !== existingIdx ? m : { ...m, is_streaming: false, action_buttons: response.data.action_buttons, action_at: response.data.action_at, updated_at: new Date().toISOString() }));
                            } else {
                                const baseMsg: Message = {
                                    id: response.data.id,
                                    type: 'assistant',
                                    content: '',
                                    action_buttons: response.data.action_buttons,
                                    queries: response.data.non_tech_mode
                                        ? response.data.queries || []
                                        : (response.data.queries || []).map((q: QueryResult) => ({ ...q, query: '' })),
                                    is_loading: false, loading_steps: [],
                                    is_streaming: !response.data.non_tech_mode,
                                    created_at: new Date().toISOString(),
                                    user_message_id: response.data.user_message_id,
                                    action_at: response.data.action_at,
                                    non_tech_mode: response.data.non_tech_mode,
                                    llm_model: response.data.llm_model,
                                    llm_model_name: response.data.llm_model_name,
                                };
                                setMessages(prev => [...prev.filter(m => !m.is_streaming), baseMsg]);
                                await animateTyping(response.data.content, response.data.id);
                                if (response.data.queries?.length) await animateQueryTyping(response.data.id, response.data.queries, response.data.non_tech_mode);
                                setMessages(prev => prev.map(m => m.id !== response.data.id ? m : { ...m, is_streaming: false, action_buttons: response.data.action_buttons, action_at: response.data.action_at, updated_at: new Date().toISOString() }));
                            }
                            setRecoRefreshToken(p => p + 1);
                        }
                        setTemporaryMessage(null);
                        break;

                    case 'ai-response-error': {
                        const errData = typeof response.data === 'object' ? response.data : { error: response.data };
                        setMessages(prev => [{
                            id: `error-${Date.now()}`,
                            type: 'assistant',
                            content: errData.error || response.data,
                            queries: [], action_buttons: [], is_loading: false, loading_steps: [], is_streaming: false,
                            llm_model: errData.llm_model, llm_model_name: errData.llm_model_name,
                            created_at: new Date().toISOString(),
                        }, ...prev.filter(m => !m.is_streaming)]);
                        setTemporaryMessage(null);
                        setRecoRefreshToken(p => p + 1);
                        break;
                    }

                    case 'response-cancelled': {
                        const cancelMsg: Message = {
                            id: `cancelled-${Date.now()}`, type: 'assistant', content: '',
                            queries: [], is_loading: false, loading_steps: [], is_streaming: false,
                            created_at: new Date().toISOString(),
                        };
                        setMessages(prev => [cancelMsg, ...prev.filter(m => !m.is_streaming)]);
                        await animateTyping(response.data, cancelMsg.id);
                        setTemporaryMessage(null);
                        setMessages(prev => prev.map(m => ({ ...m, is_streaming: false })));
                        break;
                    }

                    case 'dashboard-blueprints':
                    case 'dashboard-generation-progress':
                    case 'dashboard-generation-complete':
                    case 'dashboard-widget-data':
                    case 'dashboard-widget-error':
                        globalThis.dispatchEvent(new CustomEvent(response.event, { detail: response.data }));
                        break;
                }
            } catch (error) {
                console.error('Failed to parse SSE message:', error);
            }
        };

        const handler = (e: any) => handleSSEMessage(eventSource, e);
        (eventSource as any).onmessage = handler;
        return () => { (eventSource as any).onmessage = null; };
    }, [eventSource, temporaryMessage, selectedConnectionId, streamId]);

    // Cleanup on unmount
    useEffect(() => {
        return () => {
            if (eventSource) { eventSource.close(); }
        };
    }, [eventSource]);

    // Timeout helpers for send + edit handlers
    const scheduleStreamTimeout = useCallback((msgId: string) => {
        if (!streamId) return;
        if (streamingMessageTimeouts.current[streamId]) clearTimeout(streamingMessageTimeouts.current[streamId]);
        streamingMessageTimeouts.current[streamId] = setTimeout(() => {
            setMessages(prev => {
                const streaming = prev.find(m => m.is_streaming);
                if (!streaming || (streaming.loading_steps?.length ?? 0) >= 2) return prev;
                const timeoutMsg: Message = {
                    ...streaming, is_streaming: false, is_loading: false, loading_steps: [],
                    content: 'There seems to be a timeout in processing your message, please try again.',
                    type: 'assistant',
                    action_buttons: [{ id: msgId, label: 'Try Again', action: 'try_again', isPrimary: true }],
                };
                return prev.map(m => m.id === streaming.id ? timeoutMsg : m);
            });
            setTemporaryMessage(null);
        }, 10000);
    }, [streamId, setMessages, setTemporaryMessage]);

    const clearStreamTimeout = useCallback(() => {
        if (streamId && streamingMessageTimeouts.current[streamId]) {
            clearTimeout(streamingMessageTimeouts.current[streamId]);
            delete streamingMessageTimeouts.current[streamId];
        }
    }, [streamId]);

    return {
        eventSource,
        setEventSource,
        streamId,
        setStreamId,
        generateStreamId,
        isSSEReconnecting,
        isSettingUpSSE,
        setupSSEConnection,
        checkSSEConnection,
        scheduleStreamTimeout,
        clearStreamTimeout,
    };
}
