import { useCallback, useEffect, useRef, useState } from 'react';
import toast from 'react-hot-toast';
import chatService from '../services/chatService';
import { transformBackendMessage } from '../types/messages';
import { Message } from '../types/query';

const TOAST_STYLE = {
    style: {
        background: '#000', color: '#fff', border: '4px solid #000', borderRadius: '12px',
        boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)', padding: '12px 24px',
        fontSize: '14px', fontWeight: '500',
    },
    position: 'bottom-center' as const,
    duration: 2000,
};

type UpdateSource = 'api' | 'new' | 'query' | null;

interface UseMessageFetchingParams {
    chatId: string;
    pageSize: number;
    setMessages: React.Dispatch<React.SetStateAction<Message[]>>;
    scrollToBottom: (force?: boolean) => void;
    preserveScroll: (container: HTMLDivElement | null, callback: () => void) => void;
    chatContainerRef: React.RefObject<HTMLDivElement>;
    messageUpdateSource: React.MutableRefObject<UpdateSource>;
    isLoadingOldMessages: React.MutableRefObject<boolean>;
    isInitialLoad: React.MutableRefObject<boolean>;
    viewMode: 'chats' | 'pinned';
    showSearch: boolean;
    setShowSearch: (v: boolean) => void;
    setSearchQuery: (q: string) => void;
    setSearchResults: (r: string[]) => void;
    setCurrentSearchIndex: (i: number) => void;
    // Hoisted to ChatWindow so useScrollManagement can also receive it
    isLoadingMessages: boolean;
    setIsLoadingMessages: React.Dispatch<React.SetStateAction<boolean>>;
}

export function useMessageFetching({
    chatId,
    pageSize,
    setMessages,
    scrollToBottom,
    preserveScroll,
    chatContainerRef,
    messageUpdateSource,
    isLoadingOldMessages,
    isInitialLoad,
    viewMode,
    showSearch,
    setShowSearch,
    setSearchQuery,
    setSearchResults,
    setCurrentSearchIndex,
    isLoadingMessages,
    setIsLoadingMessages,
}: UseMessageFetchingParams) {
    const [page, setPage] = useState(1);
    const [hasMore, setHasMore] = useState(true);
    const [pinnedMessages, setPinnedMessages] = useState<Message[]>([]);

    const loadingRef = useRef<HTMLDivElement>(null);
    const pageRef = useRef(1);
    const hasMoreRef = useRef(true);
    const currentChatIdRef = useRef<string | null>(null);
    const currentFetchChatIdRef = useRef<string | null>(null);

    // Sync refs with state
    useEffect(() => { pageRef.current = page; }, [page]);
    useEffect(() => { hasMoreRef.current = hasMore; }, [hasMore]);

    const fetchPinnedMessages = useCallback(async () => {
        if (!chatId) return;
        try {
            const response = await chatService.getPinnedMessages(chatId);
            if (response.success) {
                setPinnedMessages(response.data.messages.map(transformBackendMessage));
            }
        } catch (error) {
            console.error('Failed to fetch pinned messages:', error);
            toast.error('Failed to load pinned messages');
        }
    }, [chatId]);

    const fetchMessages = useCallback(async (targetPage: number) => {
        if (!chatId || isLoadingMessages) return;

        const fetchingForChatId = chatId;
        currentFetchChatIdRef.current = fetchingForChatId;

        try {
            if (currentFetchChatIdRef.current !== fetchingForChatId) return;

            setIsLoadingMessages(true);
            isLoadingOldMessages.current = targetPage > 1;
            messageUpdateSource.current = 'api';

            const response = await chatService.getMessages(fetchingForChatId, targetPage, pageSize);

            if (currentFetchChatIdRef.current !== fetchingForChatId) return;

            if (response.success) {
                const newMessages = response.data.messages.map(transformBackendMessage);

                if (targetPage === 1) {
                    setMessages(newMessages);
                    if (isInitialLoad.current) {
                        setTimeout(() => {
                            scrollToBottom(true);
                            setTimeout(() => {
                                scrollToBottom(true);
                                isInitialLoad.current = false;
                            }, 300);
                        }, 100);
                    }
                } else {
                    preserveScroll(chatContainerRef.current, () => {
                        setMessages(prev => [...newMessages, ...prev]);
                    });
                }

                setHasMore(newMessages.length === pageSize);
            }
        } catch (error: any) {
            console.error('Failed to fetch messages:', error);
            toast.error('Failed to load messages');
        } finally {
            setTimeout(() => {
                messageUpdateSource.current = null;
                isLoadingOldMessages.current = false;
                setIsLoadingMessages(false);
            }, 200);
        }
    }, [chatId, isLoadingMessages, pageSize, setMessages, scrollToBottom, preserveScroll, chatContainerRef, messageUpdateSource, isLoadingOldMessages, isInitialLoad]);

    const handlePinMessage = useCallback(async (messageId: string, shouldPin: boolean, messages: Message[]) => {
        try {
            if (shouldPin) {
                await chatService.pinMessage(chatId, messageId);
                toast('Message pinned', { ...TOAST_STYLE, icon: '📌' });
            } else {
                await chatService.unpinMessage(chatId, messageId);
                toast('Message unpinned', { ...TOAST_STYLE, icon: '📌' });
            }

            const currentMessage = messages.find(m => m.id === messageId);
            if (!currentMessage) return;

            const updatedMessages = messages.map(msg => {
                if (msg.id === messageId) return { ...msg, is_pinned: shouldPin, pinned_at: shouldPin ? new Date().toISOString() : undefined };
                if (currentMessage.type === 'user' && msg.user_message_id === messageId) return { ...msg, is_pinned: shouldPin, pinned_at: shouldPin ? new Date().toISOString() : undefined };
                if (currentMessage.type === 'assistant' && currentMessage.user_message_id && msg.id === currentMessage.user_message_id) return { ...msg, is_pinned: shouldPin, pinned_at: shouldPin ? new Date().toISOString() : undefined };
                return msg;
            });

            setMessages(updatedMessages);

            if (shouldPin) {
                const newPinned = updatedMessages.filter(msg => msg.is_pinned && !pinnedMessages.some(pm => pm.id === msg.id));
                setPinnedMessages(prev => [...newPinned, ...prev]);
            } else {
                const unpinnedIds = new Set(updatedMessages.filter(m => !m.is_pinned).map(m => m.id));
                setPinnedMessages(prev => prev.filter(m => !unpinnedIds.has(m.id)));
            }
        } catch (error) {
            console.error('Failed to pin/unpin message:', error);
            toast.error('Failed to update pin status');
        }
    }, [chatId, pinnedMessages, setMessages]);

    // IntersectionObserver for infinite scroll
    useEffect(() => {
        const observer = new IntersectionObserver(
            entries => {
                if (entries[0].isIntersecting &&
                    hasMoreRef.current &&
                    !isLoadingMessages &&
                    !isInitialLoad.current &&
                    pageRef.current > 0 &&
                    viewMode === 'chats' &&
                    chatId &&
                    currentFetchChatIdRef.current === chatId) {
                    const nextPage = pageRef.current + 1;
                    setPage(nextPage);
                    fetchMessages(nextPage);
                }
            },
            { root: null, rootMargin: '100px', threshold: 0.1 },
        );

        if (loadingRef.current && viewMode === 'chats') {
            observer.observe(loadingRef.current);
        }
        return () => observer.disconnect();
    }, [isLoadingMessages, fetchMessages, chatId, viewMode, isInitialLoad]);

    // Reset + fetch on chat change
    useEffect(() => {
        if (!chatId || chatId === currentChatIdRef.current) return;

        currentChatIdRef.current = chatId;
        currentFetchChatIdRef.current = chatId;

        if (showSearch) {
            setShowSearch(false);
            setSearchQuery('');
            setSearchResults([]);
            setCurrentSearchIndex(0);
        }

        isInitialLoad.current = true;
        isLoadingOldMessages.current = false;
        messageUpdateSource.current = null;

        setPage(1);
        setHasMore(true);
        pageRef.current = 1;
        hasMoreRef.current = true;
        setMessages([]);
        setPinnedMessages([]);

        setTimeout(() => scrollToBottom(true), 50);
        fetchMessages(1);
        fetchPinnedMessages();
    // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [chatId]);

    // Restore scroll when switching between chats/pinned views
    useEffect(() => {
        // Always restore scroll on viewMode change
    }, [viewMode]); // handled in ChatWindow via scrollPositions ref

    return {
        page, setPage,
        hasMore, setHasMore,
        isLoadingMessages,
        pinnedMessages, setPinnedMessages,
        pageRef, hasMoreRef,
        loadingRef,
        currentChatIdRef,
        currentFetchChatIdRef,
        fetchMessages,
        fetchPinnedMessages,
        handlePinMessage,
    };
}
