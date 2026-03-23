import { useCallback, useEffect, useRef, useState } from 'react';
import toast from 'react-hot-toast';
import { Message } from '../types/query';

type UpdateSource = 'api' | 'new' | 'query' | null;

interface UseScrollManagementParams {
    messages: Message[];
    isLoadingMessages: boolean;
    viewMode: 'chats' | 'pinned';
}

export function useScrollManagement({
    messages,
    isLoadingMessages,
    viewMode,
}: UseScrollManagementParams) {
    const chatContainerRef = useRef<HTMLDivElement>(null);
    const messagesEndRef = useRef<HTMLDivElement>(null);
    const isScrollingRef = useRef(false);
    const scrollPositionRef = useRef<number>(0);
    const scrollTimeoutRef = useRef<NodeJS.Timeout | null>(null);
    const scrollPositions = useRef<{ chats: number; pinned: number }>({ chats: 0, pinned: 0 });
    const messageUpdateSource = useRef<UpdateSource>(null);
    const isLoadingOldMessages = useRef(false);
    const isInitialLoad = useRef(true);
    const wasStreamingRef = useRef(false);
    const [showScrollButton, setShowScrollButton] = useState(false);

    const scrollToBottom = useCallback((force = false) => {
        const container = chatContainerRef.current;
        if (!container) return;

        isScrollingRef.current = true;
        if (scrollTimeoutRef.current) clearTimeout(scrollTimeoutRef.current);

        const performScroll = () => {
            requestAnimationFrame(() => {
                requestAnimationFrame(() => {
                    const maxScrollTop = container.scrollHeight - container.clientHeight;
                    container.scrollTop = maxScrollTop;
                    scrollPositionRef.current = container.scrollTop;

                    const isAtBottom = Math.abs(maxScrollTop - container.scrollTop) < 1;
                    if (!isAtBottom && force) {
                        let retryCount = 0;
                        const retryScroll = () => {
                            const top = container.scrollHeight - container.clientHeight;
                            container.scrollTop = top;
                            scrollPositionRef.current = container.scrollTop;
                            if (Math.abs(top - container.scrollTop) > 1 && ++retryCount < 5) {
                                setTimeout(retryScroll, 100 * retryCount);
                            }
                        };
                        setTimeout(retryScroll, 50);
                    }
                    scrollTimeoutRef.current = setTimeout(() => { isScrollingRef.current = false; }, 150);
                });
            });
        };
        performScroll();
    }, []);

    const preserveScroll = useCallback((container: HTMLDivElement | null, callback: () => void) => {
        if (!container) { callback(); return; }

        const oldHeight = container.scrollHeight;
        const oldScroll = container.scrollTop;
        const wasAtBottom = container.scrollHeight - container.scrollTop - container.clientHeight < 10;

        isScrollingRef.current = true;
        if (scrollTimeoutRef.current) clearTimeout(scrollTimeoutRef.current);

        callback();

        requestAnimationFrame(() => {
            if (wasAtBottom) {
                container.scrollTop = container.scrollHeight;
            } else {
                container.scrollTop = oldScroll + (container.scrollHeight - oldHeight);
            }
            scrollPositionRef.current = container.scrollTop;
            scrollTimeoutRef.current = setTimeout(() => { isScrollingRef.current = false; }, 100);
        });
    }, []);

    const handleQueryUpdate = useCallback((callback: () => void) => {
        messageUpdateSource.current = 'query';
        const container = chatContainerRef.current;
        if (!container) {
            callback();
            setTimeout(() => { messageUpdateSource.current = null; }, 100);
            return;
        }

        const oldScrollTop = container.scrollTop;
        const oldScrollHeight = container.scrollHeight;
        const wasAtBottom = oldScrollHeight - oldScrollTop - container.clientHeight < 10;

        callback();

        requestAnimationFrame(() => {
            requestAnimationFrame(() => {
                const newScrollHeight = container.scrollHeight;
                if (wasAtBottom) {
                    container.scrollTop = newScrollHeight - container.clientHeight;
                } else {
                    container.scrollTop = oldScrollTop;
                }
                scrollPositionRef.current = container.scrollTop;
            });
        });

        setTimeout(() => { messageUpdateSource.current = null; }, 200);
    }, []);

    // Scroll event listener
    useEffect(() => {
        const container = chatContainerRef.current;
        if (!container) return;

        const handleScroll = () => {
            if (isScrollingRef.current) return;
            const { scrollTop, scrollHeight, clientHeight } = container;
            scrollPositionRef.current = scrollTop;
            scrollPositions.current[viewMode] = scrollTop;
            setShowScrollButton(scrollHeight - scrollTop - clientHeight >= 10);
        };

        container.addEventListener('scroll', handleScroll);
        return () => container.removeEventListener('scroll', handleScroll);
    }, [viewMode]);

    // MutationObserver for scroll-on-new-content
    useEffect(() => {
        const container = chatContainerRef.current;
        if (!container) return;

        const observer = new MutationObserver(() => {
            if (isScrollingRef.current) return;
            const { scrollTop, scrollHeight, clientHeight } = container;
            setShowScrollButton(scrollHeight - scrollTop - clientHeight >= 10);

            if (messageUpdateSource.current === 'query' ||
                messageUpdateSource.current === 'api' ||
                isLoadingOldMessages.current ||
                isLoadingMessages) {
                return;
            }

            if (messageUpdateSource.current === 'new') {
                requestAnimationFrame(() => {
                    container.scrollTop = container.scrollHeight - container.clientHeight;
                    scrollPositionRef.current = container.scrollTop;
                });
            }
        });

        observer.observe(container, { childList: true, subtree: true, characterData: true });
        return () => observer.disconnect();
    }, [messages, isLoadingMessages]);

    // Scroll when new user message is added
    useEffect(() => {
        if (messageUpdateSource.current === 'api' ||
            messageUpdateSource.current === 'query' ||
            isLoadingOldMessages.current) return;

        const lastMessage = messages[messages.length - 1];
        if (lastMessage?.type === 'user' && messageUpdateSource.current === 'new') {
            setTimeout(() => scrollToBottom(true), 50);
        }
    }, [messages, scrollToBottom]);

    // Streaming completion toast
    useEffect(() => {
        const isStreaming = messages.some(m => m.is_streaming);
        const container = chatContainerRef.current;

        if (container && wasStreamingRef.current && !isStreaming) {
            const { scrollTop, scrollHeight, clientHeight } = container;
            if (scrollHeight - scrollTop - clientHeight >= 10) {
                toast('Assistant response completed!', {
                    icon: '✅',
                    style: {
                        background: '#000', color: '#fff', border: '4px solid #000',
                        borderRadius: '12px', boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
                        padding: '12px 24px', fontSize: '14px', fontWeight: '500',
                    },
                    position: 'bottom-center',
                    duration: 2000,
                });
            }
        }

        wasStreamingRef.current = isStreaming;
    }, [messages]);

    return {
        chatContainerRef,
        messagesEndRef,
        showScrollButton,
        scrollPositions,
        messageUpdateSource,
        isLoadingOldMessages,
        isInitialLoad,
        isScrollingRef,
        scrollPositionRef,
        scrollTimeoutRef,
        scrollToBottom,
        preserveScroll,
        handleQueryUpdate,
    };
}
