import { ArrowDown, Loader2, RefreshCcw, XCircle } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import toast from 'react-hot-toast';
import { useStream } from '../../contexts/StreamContext';
import axios from '../../services/axiosConfig';
import chatService from '../../services/chatService';
import analyticsService from '../../services/analyticsService';
import { Chat, Connection } from '../../types/chat';
import { transformBackendMessage } from '../../types/messages';
import ConfirmationModal from '../modals/ConfirmationModal';
import ConnectionModal, { ModalTab } from '../modals/ConnectionModal';
import ChatHeader from './ChatHeader';
import MessageInput from './MessageInput';
import MessageTile from './MessageTile';
import SearchBar from './SearchBar';
import { Message } from './types';
import { ChatSettings } from '../../types/chat';
interface ChatWindowProps {
  chat: Chat;
  isExpanded: boolean;
  messages: Message[];
  setMessages: React.Dispatch<React.SetStateAction<Message[]>>;
  onSendMessage: (message: string) => Promise<void>;
  onEditMessage: (id: string, content: string) => void;
  onClearChat: () => void;
  onCloseConnection: () => void;
  onEditConnection?: (id: string, connection: Connection, settings: ChatSettings) => Promise<{ success: boolean, error?: string }>;
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
}

interface QueryState {
  isExecuting: boolean;
  isExample: boolean;
}

type UpdateSource = 'api' | 'new' | 'query' | null;


const formatDateDivider = (dateString: string) => {
  const date = new Date(dateString);
  const today = new Date();
  const yesterday = new Date(today);
  yesterday.setDate(yesterday.getDate() - 1);

  if (date.toDateString() === today.toDateString()) {
    return 'Today';
  } else if (date.toDateString() === yesterday.toDateString()) {
    return 'Yesterday';
  }
  return date.toLocaleDateString('en-US', {
    month: 'long',
    day: 'numeric',
    year: 'numeric'
  });
};

const groupMessagesByDate = (messages: Message[]) => {
  const groups: { [key: string]: Message[] } = {};

  // Sort messages by date, oldest first
  const sortedMessages = [...messages].sort((a, b) =>
    new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
  );

  sortedMessages.forEach(message => {
    const date = new Date(message.created_at).toDateString();
    if (!groups[date]) {
      groups[date] = [];
    }
    groups[date].push(message);
  });

  // Convert to array and sort by date
  const sortedEntries = Object.entries(groups).sort((a, b) =>
    new Date(a[0]).getTime() - new Date(b[0]).getTime()
  );

  return Object.fromEntries(sortedEntries);
};

export default function ChatWindow({
  chat,
  onEditMessage,
  isExpanded,
  messages,
  setMessages,
  onSendMessage,
  onClearChat,
  onCloseConnection,
  onEditConnection,
  onConnectionStatusChange,
  isConnected,
  onCancelStream,
  onRefreshSchema,
  onCancelRefreshSchema,
  checkSSEConnection,
  onUpdateSelectedCollections,
  onEditConnectionFromChatWindow,
  userId,
  userName
}: ChatWindowProps) {
  const queryTimeouts = useRef<Record<string, NodeJS.Timeout>>({});
  const [editingMessageId, setEditingMessageId] = useState<string | null>(null);
  const [editInput, setEditInput] = useState('');
  const [showClearConfirm, setShowClearConfirm] = useState(false);
  const [showRefreshSchema, setShowRefreshSchema] = useState(false);
  const [showCloseConfirm, setShowCloseConfirm] = useState(false);
  const [showScrollButton, setShowScrollButton] = useState(false);
  const [queryStates, setQueryStates] = useState<Record<string, QueryState>>({});
  const [isConnecting, setIsConnecting] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const chatContainerRef = useRef<HTMLDivElement>(null);
  const [showEditConnection, setShowEditConnection] = useState(false);
  const [openWithSettingsTab, setOpenWithSettingsTab] = useState(false);
  const { streamId, generateStreamId } = useStream();
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(true);
  const [isLoadingMessages, setIsLoadingMessages] = useState(false);
  const pageSize = 25; // Messages per page
  const loadingRef = useRef<HTMLDivElement>(null);
  const [isMessageSending, setIsMessageSending] = useState(false);
  const isLoadingOldMessages = useRef(false);
  const messageUpdateSource = useRef<UpdateSource>(null);
  const isInitialLoad = useRef(true);
  const scrollPositionRef = useRef<number>(0);
  const isScrollingRef = useRef(false);
  const scrollTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const [showSearch, setShowSearch] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState<string[]>([]);
  const [currentSearchIndex, setCurrentSearchIndex] = useState(0);
  const searchResultRefs = useRef<{ [key: string]: HTMLElement | null }>({});
  const [showEditQueryConfirm, setShowEditQueryConfirm] = useState<{
    show: boolean;
    messageId: string | null;
    queryId: string | null;
    query: string | null;
  }>({
    show: false,
    messageId: null,
    queryId: null,
    query: null
  });
  const wasStreamingRef = useRef<boolean>(false);
  const currentChatIdRef = useRef<string | null>(null);

  // Search functionality
  const performSearch = useCallback((query: string) => {
    setSearchQuery(query);
    
    if (!query.trim()) {
      setSearchResults([]);
      setCurrentSearchIndex(0);
      return;
    }

    const results: string[] = [];
    const lowerQuery = query.toLowerCase();
    const messageMatches = new Set<string>(); // Track which messages have matches

    messages.forEach(message => {
      let messageHasMatch = false;
      
      // Search in message content
      if (message.content.toLowerCase().includes(lowerQuery)) {
        messageHasMatch = true;
      }

      // Search in queries
      message.queries?.forEach((query) => {
        if (query.query.toLowerCase().includes(lowerQuery)) {
          messageHasMatch = true;
        }
        // Search in error messages
        if (query.error) {
          if ((query.error.message && query.error.message.toLowerCase().includes(lowerQuery)) ||
              (query.error.code && query.error.code.toLowerCase().includes(lowerQuery)) ||
              (query.error.details && query.error.details.toLowerCase().includes(lowerQuery))) {
            messageHasMatch = true;
          }
        }
        // Search in description (explanation)
        if (query.description && query.description.toLowerCase().includes(lowerQuery)) {
          messageHasMatch = true;
        }
      });
      
      // Add only one result per message
      if (messageHasMatch) {
        results.push(`msg-${message.id}`);
        messageMatches.add(message.id);
      }
    });

    setSearchResults(results);
    setCurrentSearchIndex(0);
    
    // Scroll to first result if found
    if (results.length > 0) {
      scrollToSearchResult(0, results);
    }
  }, [messages]);

  const scrollToSearchResult = useCallback((index: number, results?: string[]) => {
    const searchList = results || searchResults;
    if (searchList.length === 0 || index < 0 || index >= searchList.length) return;

    const resultId = searchList[index];
    const element = searchResultRefs.current[resultId];
    
    if (element) {
      element.scrollIntoView({
        behavior: 'smooth',
        block: 'center'
      });
    }
  }, [searchResults]);

  const navigateSearchUp = useCallback(() => {
    if (searchResults.length === 0) return;
    // Since messages are displayed newest at bottom, "up" should go to previous (older) result
    const newIndex = currentSearchIndex < searchResults.length - 1 ? currentSearchIndex + 1 : 0;
    setCurrentSearchIndex(newIndex);
    scrollToSearchResult(newIndex);
  }, [currentSearchIndex, searchResults, scrollToSearchResult]);

  const navigateSearchDown = useCallback(() => {
    if (searchResults.length === 0) return;
    // Since messages are displayed newest at bottom, "down" should go to next (newer) result
    const newIndex = currentSearchIndex > 0 ? currentSearchIndex - 1 : searchResults.length - 1;
    setCurrentSearchIndex(newIndex);
    scrollToSearchResult(newIndex);
  }, [currentSearchIndex, searchResults, scrollToSearchResult]);

  const handleToggleSearch = useCallback(() => {
    setShowSearch(prev => !prev);
    if (showSearch) {
      // Don't clear search query, just clear results
      setSearchResults([]);
      setCurrentSearchIndex(0);
    } else {
      // Re-run search when opening if there's a query
      if (searchQuery) {
        performSearch(searchQuery);
      }
    }
  }, [showSearch, searchQuery, performSearch]);

  useEffect(() => {
    if (isConnected) {
      setIsConnecting(false);
    }
  }, [isConnected]);

  const setMessage = (message: Message) => {
    console.log('setMessage called with message:', message);
    setMessages(prev => prev.map(m => m.id === message.id ? message : m));
  };

  const scrollToBottom = (origin: string, force: boolean = false) => {
    const chatContainer = chatContainerRef.current;
    if (!chatContainer) {
      return;
    }

    isScrollingRef.current = true;
    if (scrollTimeoutRef.current) {
      clearTimeout(scrollTimeoutRef.current);
    }

    // Use multiple animation frames to ensure content is fully rendered
    const performScroll = () => {
      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          // Ensure we scroll to the absolute bottom
          const maxScrollTop = chatContainer.scrollHeight - chatContainer.clientHeight;
          chatContainer.scrollTop = maxScrollTop;
          scrollPositionRef.current = chatContainer.scrollTop;

          // Verify we're actually at the bottom and retry if needed
          const currentScrollTop = chatContainer.scrollTop;
          const isAtBottom = Math.abs(maxScrollTop - currentScrollTop) < 1;
          
          if (!isAtBottom && force) {
            let retryCount = 0;
            const retryScroll = () => {
              const newMaxScrollTop = chatContainer.scrollHeight - chatContainer.clientHeight;
              chatContainer.scrollTop = newMaxScrollTop;
              scrollPositionRef.current = chatContainer.scrollTop;
              
              const stillNotAtBottom = Math.abs(newMaxScrollTop - chatContainer.scrollTop) > 1;
              retryCount++;
              
              if (stillNotAtBottom && retryCount < 5) {
                setTimeout(retryScroll, 100 * retryCount);
              }
            };
            setTimeout(retryScroll, 50);
          }

          scrollTimeoutRef.current = setTimeout(() => {
            isScrollingRef.current = false;
          }, 150);
        });
      });
    };

    performScroll();
  };

  const preserveScroll = (chatContainer: HTMLDivElement | null, callback: () => void) => {
    if (!chatContainer) return callback();

    // Store current scroll position
    const oldHeight = chatContainer.scrollHeight;
    const oldScroll = chatContainer.scrollTop;
    const wasAtBottom = chatContainer.scrollHeight - chatContainer.scrollTop - chatContainer.clientHeight < 10;

    // Set scrolling flag
    isScrollingRef.current = true;

    // Clear any pending scroll timeout
    if (scrollTimeoutRef.current) {
      clearTimeout(scrollTimeoutRef.current);
    }

    // Execute state update
    callback();

    // Use RAF for smooth animation frame
    requestAnimationFrame(() => {
      if (wasAtBottom) {
        chatContainer.scrollTop = chatContainer.scrollHeight;
      } else {
        const newHeight = chatContainer.scrollHeight;
        const heightDiff = newHeight - oldHeight;
        chatContainer.scrollTop = oldScroll + heightDiff;
      }

      // Store the final position
      scrollPositionRef.current = chatContainer.scrollTop;

      // Clear scrolling flag after a short delay
      scrollTimeoutRef.current = setTimeout(() => {
        isScrollingRef.current = false;
      }, 100);
    });
  };

  useEffect(() => {
    const chatContainer = chatContainerRef.current;
    if (!chatContainer) return;

    const handleScroll = () => {
      if (isScrollingRef.current) return;

      const { scrollTop, scrollHeight, clientHeight } = chatContainer;
      const isAtBottom = scrollHeight - scrollTop - clientHeight < 10;

      scrollPositionRef.current = scrollTop;
      setShowScrollButton(!isAtBottom);
    };

    chatContainer.addEventListener('scroll', handleScroll);
    return () => chatContainer.removeEventListener('scroll', handleScroll);
  }, []);

  useEffect(() => {
    const chatContainer = chatContainerRef.current;
    if (!chatContainer) return;

    const observer = new MutationObserver(() => {
      if (isScrollingRef.current) return;

      const { scrollTop, scrollHeight, clientHeight } = chatContainer;
      const isAtBottom = scrollHeight - scrollTop - clientHeight < 10;

      setShowScrollButton(!isAtBottom);

      // Skip auto-scroll during query updates, edits, or API updates
      if (messageUpdateSource.current === 'query' || 
          messageUpdateSource.current === 'api' ||
          isLoadingOldMessages.current ||
          isLoadingMessages) {
        return;
      }

      // Remove auto-scroll behavior - let users control scrolling manually
      // Only auto-scroll for new user messages
      const shouldAutoScroll = messageUpdateSource.current === 'new';

      if (shouldAutoScroll) {
        requestAnimationFrame(() => {
          const maxScrollTop = chatContainer.scrollHeight - chatContainer.clientHeight;
          chatContainer.scrollTop = maxScrollTop;
          scrollPositionRef.current = chatContainer.scrollTop;
        });
      }
    });

    observer.observe(chatContainer, {
      childList: true,
      subtree: true,
      characterData: true
    });

    return () => observer.disconnect();
  }, [messages, isLoadingMessages]);

  const handleCloseConfirm = useCallback(() => {
    setShowCloseConfirm(false);
  }, []);

  const handleReconnect = useCallback(async () => {
    try {
      setIsConnecting(true);
      let currentStreamId = streamId;

      // Generate new streamId if not available
      if (!currentStreamId) {
        currentStreamId = generateStreamId();
      }

      // Check if the connection is already established
      const connectionStatus = await checkConnectionStatus(chat.id, currentStreamId);
      if (!connectionStatus) {
        await connectToDatabase(chat.id, currentStreamId);
      }
      console.log('connectionStatus', connectionStatus);
      onConnectionStatusChange?.(chat.id, true, 'chat-window-reconnect');
    } catch (error) {
      console.error('Failed to reconnect to database:', error);
      onConnectionStatusChange?.(chat.id, false, 'chat-window-reconnect');
      toast.error('Failed to reconnect to database:' + error, {
        style: {
          background: '#ff4444',
          color: '#fff',
          border: '4px solid #cc0000',
          borderRadius: '12px',
          boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
          padding: '12px 24px',
        }
      });
    } finally {
      setIsConnecting(false);
    }
  }, [chat.id, streamId, generateStreamId, onConnectionStatusChange]);

  const checkConnectionStatus = async (chatId: string, streamId: string) => {
    try {
      const response = await axios.get(
        `${import.meta.env.VITE_API_URL}/chats/${chatId}/connection-status`,
        {
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${localStorage.getItem('token')}`
          }
        }
      );
      return response.data;
    } catch (error) {
      console.error('Failed to check connection status:', error);
      return false;
    }
  };

  const handleDisconnect = useCallback(async () => {
    try {
      await onCloseConnection();
      handleCloseConfirm();
    } catch (error) {
      console.error('Failed to disconnect:', error);
      toast.error('Failed to disconnect from database');
    }
  }, [onCloseConnection, handleCloseConfirm]);

  const handleEditMessage = (id: string) => {
    // Set message source to prevent auto-scroll
    messageUpdateSource.current = 'query';
    
    const message = messages.find(m => m.id === id);
    if (message) {
      setEditingMessageId(id);
      setEditInput(message.content);
    }
    
    // Reset after a delay
    setTimeout(() => {
      messageUpdateSource.current = null;
    }, 200);
  };

  const handleCancelEdit = () => {
    // Set message source to prevent auto-scroll
    messageUpdateSource.current = 'query';
    
    setEditingMessageId(null);
    setEditInput('');
    
    // Reset after a delay
    setTimeout(() => {
      messageUpdateSource.current = null;
    }, 200);
  };

  const connectToDatabase = async (chatId: string, streamId: string) => {
    try {
      const response = await axios.post(
        `${import.meta.env.VITE_API_URL}/chats/${chatId}/connect`,
        { stream_id: streamId },
        {
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${localStorage.getItem('token')}`
          }
        }
      );
      return response.data;
    } catch (error: any) {
      console.error('Failed to connect to database:', error.response.data);
      throw error.response.data.error;
    }
  };

  const handleSendMessage = useCallback(async (content: string) => {
    try {
      // Track message sent event
      if (chat?.id) {
        analyticsService.trackMessageSent(chat.id, content.length, userId || '', userName || '');
      }

      await onSendMessage(content);
    } catch (error) {
      console.error('Failed to send message:', error);
    }
  }, [chat?.id, onSendMessage]);

  const handleSaveEdit = useCallback((id: string, content: string) => {
    if (content.trim()) {
      // Set message source to prevent auto-scroll
      messageUpdateSource.current = 'query';
      
      // Find the message and its index
      const messageIndex = messages.findIndex(msg => msg.id === id);
      if (messageIndex === -1) return;

      // Get the edited message and the next message (AI response)
      const editedMessage = messages[messageIndex];
      const aiResponse = messages[messageIndex + 1];

      // Track message edit event
      if (chat?.id) {
        analyticsService.trackMessageEdited(chat.id, id, userId || '', userName || '');
      }

      onEditMessage(id, content);
      setMessages(prev => {
        const updated = [...prev];
        // Update the edited message
        updated[messageIndex] = { ...editedMessage, content: content.trim() };
        // Keep the AI response if it exists
        if (aiResponse && aiResponse.type === 'assistant') {
          updated[messageIndex + 1] = aiResponse;
        }
        return updated;
      });
      
      // Reset after a delay
      setTimeout(() => {
        messageUpdateSource.current = null;
      }, 200);
    }
    setEditingMessageId(null);
    setEditInput('');
  }, [messages, setMessages, chat?.id, onEditMessage]);

  const fetchMessages = useCallback(async (page: number) => {
    if (!chat?.id || isLoadingMessages) return;

    try {
      console.log('Fetching messages for chat:', chat.id, 'page:', page);
      setIsLoadingMessages(true);
      isLoadingOldMessages.current = page > 1;
      messageUpdateSource.current = 'api';

      const response = await chatService.getMessages(chat.id, page, pageSize);

      if (response.success) {
        const newMessages = response.data.messages.map(transformBackendMessage);
        console.log('Received messages:', newMessages.length, 'for page:', page);

        if (page === 1) {
          // For initial load, set messages and scroll to bottom
          setMessages(newMessages);
          if (isInitialLoad.current) {
            // Use multiple timeouts to ensure DOM is fully updated and all images/content loaded
            setTimeout(() => {
              scrollToBottom('initial-load', true);
              // Double-check after a longer delay to handle lazy-loaded content
              setTimeout(() => {
                scrollToBottom('initial-load-verification', true);
                isInitialLoad.current = false;
              }, 300);
            }, 100);
          }
        } else {
          // For pagination, preserve scroll position
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
  }, [chat?.id, pageSize]);

  // Update intersection observer effect
  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting &&
          hasMore &&
          !isLoadingMessages) {
          console.log('Loading more messages, current page:', page);
          setPage(prev => prev + 1);
          fetchMessages(page + 1);  // Fetch next page immediately
        }
      },
      {
        root: null,
        rootMargin: '100px',  // Start loading before element is visible
        threshold: 0.1
      }
    );

    if (loadingRef.current) {
      observer.observe(loadingRef.current);
    }

    return () => observer.disconnect();
  }, [hasMore, isLoadingMessages, page, fetchMessages, chat?.id]);

  // Keep only necessary effects
  useEffect(() => {
    if (chat?.id && chat.id !== currentChatIdRef.current) {
      // Update current chat ID
      currentChatIdRef.current = chat.id;

      // Close search when changing chats
      if (showSearch) {
        setShowSearch(false);
        setSearchQuery('');
        setSearchResults([]);
        setCurrentSearchIndex(0);
      }

      // Reset all scroll-related state
      isInitialLoad.current = true;
      isScrollingRef.current = false;
      scrollPositionRef.current = 0;
      messageUpdateSource.current = null;
      isLoadingOldMessages.current = false;
      
      // Clear any pending timeouts
      if (scrollTimeoutRef.current) {
        clearTimeout(scrollTimeoutRef.current);
      }
      
      setPage(1);
      setHasMore(true);
      setMessages([]);
      
      // Immediate scroll for welcome message or empty state
      setTimeout(() => {
        scrollToBottom('chat-mounted', true);
      }, 50);
      fetchMessages(1);
    }
  }, [chat?.id, fetchMessages, showSearch]);

  // Update the message update effect with better timing control
  useEffect(() => {
    // Skip effect if source is API, query operations, or loading old messages
    if (messageUpdateSource.current === 'api' ||
      messageUpdateSource.current === 'query' ||
      isLoadingOldMessages.current) {
      return;
    }

    // Always scroll for new user messages (when user sends a message)
    const chatContainer = chatContainerRef.current;
    if (!chatContainer) return;
    
    const { scrollTop, scrollHeight, clientHeight } = chatContainer;
    const isAtBottom = scrollHeight - scrollTop - clientHeight < 10;
    
    const lastMessage = messages[messages.length - 1];
    const shouldScrollForNewMessage = lastMessage?.type === 'user' && messageUpdateSource.current === 'new';

    if (shouldScrollForNewMessage) {
      // Use timeout to ensure proper timing after state updates
      setTimeout(() => {
        scrollToBottom('new-message', true);
      }, 50);
    }
  }, [messages]);

  // Effect to track streaming state and show toast when completed
  useEffect(() => {
    const hasStreamingMessage = messages.some(m => m.is_streaming);
    const chatContainer = chatContainerRef.current;
    
    if (chatContainer) {
      const { scrollTop, scrollHeight, clientHeight } = chatContainer;
      const isAtBottom = scrollHeight - scrollTop - clientHeight < 10;
      
      // Check if streaming just stopped (was streaming before, not streaming now)
      if (wasStreamingRef.current && !hasStreamingMessage && !isAtBottom) {
        toast('Assistant response completed!', {
          icon: '✅',
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
        });
      }
    }
    
    // Update the streaming state for next comparison
    wasStreamingRef.current = hasStreamingMessage;
  }, [messages]);

  // Update handleMessageSubmit to be more explicit
  const handleMessageSubmit = async (content: string) => {
    try {
      messageUpdateSource.current = 'new';
      await handleSendMessage(content);
      // Force scroll after message is sent
      setTimeout(() => {
        scrollToBottom('user-message-sent', true);
      }, 100);
    } finally {
      setTimeout(() => {
        messageUpdateSource.current = null;
      }, 300);
    }
  };

  // Add this function to handle query-related updates
  const handleQueryUpdate = (callback: () => void) => {
    messageUpdateSource.current = 'query';
    const chatContainer = chatContainerRef.current;

    if (!chatContainer) {
      callback();
      setTimeout(() => {
        messageUpdateSource.current = null;
      }, 100);
      return;
    }

    // Store exact scroll state before update
    const oldScrollTop = chatContainer.scrollTop;
    const oldScrollHeight = chatContainer.scrollHeight;
    const distanceFromBottom = oldScrollHeight - oldScrollTop - chatContainer.clientHeight;
    const wasAtBottom = distanceFromBottom < 10;

    // Execute the update
    callback();

    // Preserve scroll position after update
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        const newScrollHeight = chatContainer.scrollHeight;
        const heightDiff = newScrollHeight - oldScrollHeight;
        
        if (wasAtBottom) {
          // If user was at bottom, keep them at bottom
          chatContainer.scrollTop = newScrollHeight - chatContainer.clientHeight;
        } else {
          // Otherwise, maintain their distance from top
          chatContainer.scrollTop = oldScrollTop;
          
          // If content expanded above current position, adjust
          if (heightDiff > 0 && oldScrollTop > 0) {
            chatContainer.scrollTop = oldScrollTop;
          }
        }
        
        scrollPositionRef.current = chatContainer.scrollTop;
      });
    });

    // Reset message source after a delay
    setTimeout(() => {
      messageUpdateSource.current = null;
    }, 200);
  };

  const handleEditQuery = async (messageId: string, queryId: string, query: string) => {
    setShowEditQueryConfirm({
      show: true,
      messageId,
      queryId,
      query
    });
  };

  const handleConfirmQueryEdit = async () => {
    if (!showEditQueryConfirm.messageId || !showEditQueryConfirm.queryId || !showEditQueryConfirm.query) return;

    try {
      const response = await chatService.editQuery(
        chat.id,
        showEditQueryConfirm.messageId,
        showEditQueryConfirm.queryId,
        showEditQueryConfirm.query
      );

      if (response.success) {
        preserveScroll(chatContainerRef.current, () => {
          setMessages(prev => prev.map(msg => {
            if (msg.id === showEditQueryConfirm.messageId) {
              return {
                ...msg,
                queries: msg.queries?.map(q =>
                  q.id === showEditQueryConfirm.queryId
                    ? {
                      ...q,
                      query: showEditQueryConfirm.query!,
                      is_edited: true,
                      original_query: q.query
                    }
                    : q
                )
              };
            }
            return msg;
          }));
        });
        toast.success('Query updated successfully');
      }
    } catch (error) {
      console.error('Failed to edit query:', error);
      toast.error('Failed to update query: ' + error);
    } finally {
      setShowEditQueryConfirm({ show: false, messageId: null, queryId: null, query: null });
    }
  };

  // @Deprecated the below logic for now & testing the new one
  const _handleFixErrorAction = (message: Message) => {
    // Find the user message that this AI response is replying to
    const userMessageId = message.user_message_id;
    if (!userMessageId) {
      toast.error("Could not find the original message to fix");
      return;
    }

    // Find the user message in the messages array
    const userMessage = messages.find(m => m.id === userMessageId);
    if (!userMessage) {
      toast.error("Could not find the original message to fix");
      return;
    }

    // Collect all queries with errors
    const queriesWithErrors = message.queries?.filter(q => q.error) || [];
    if (queriesWithErrors.length === 0) {
      toast.error("No errors found to fix");
      // Remove the "Fix Error" action button from the message
      setMessages(prev => prev.map(msg => {
        if (msg.id === userMessageId) {
          return {
            ...msg,
            action_buttons: msg.action_buttons?.filter(b => b.action !== "fix_error")
          };
        }
        return msg;
      }));
      return;
    }

    // Create the error message content
    let fixErrorContent = userMessage.content + "\n\nFix Errors:\n";
    queriesWithErrors.forEach(query => {
      fixErrorContent += `Query: '${query.query}' faced an error: '${query.error?.message || "Unknown error"}'.\n`;
    });

    // Edit the user message to include the error message
    onEditMessage(userMessageId, fixErrorContent);
  };

  // @Deprecated the below logic for now & testing the new one
  const _handleFixRollbackErrorAction = (message: Message) => {
    // Find the user message that this AI response is replying to
    const userMessageId = message.user_message_id;
    if (!userMessageId) {
      toast.error("Could not find the original message to fix");
      return;
    }

    // Find the user message in the messages array
    const userMessage = messages.find(m => m.id === userMessageId);
    if (!userMessage) {
      toast.error("Could not find the original message to fix");
      return;
    }

    // Collect all queries with errors
    const queriesWithErrors = message.queries?.filter(q => q.error) || [];
    if (queriesWithErrors.length === 0) {
      toast.error("No errors found to fix");
      // Remove the "Fix Error" action button from the message
      setMessages(prev => prev.map(msg => {
        if (msg.id === userMessageId) {
          return {
            ...msg,
            action_buttons: msg.action_buttons?.filter(b => b.action !== "fix_rollback_error")
          };
        }
        return msg;
      }));
      return;
    }

    // Create the error message content
    let fixRollbackErrorContent = userMessage.content + "\n\nFix Rollback Errors:\n";
    queriesWithErrors.forEach(query => {
      fixRollbackErrorContent += `Query: '${query.rollback_query != null && query.rollback_query != "" ? query.rollback_query : query.rollback_dependent_query}' faced an error: '${query.error?.message || "Unknown error"}'.\n`;
    });

    // Edit the user message to include the error message
    onEditMessage(userMessageId, fixRollbackErrorContent);
  }

  const handleFixErrorAction = (message: Message) => {

    const queriesWithErrors = message.queries?.filter(q => q.error) || [];
    if (queriesWithErrors.length === 0) {
      toast.error("No errors found to fix");
      return;
    }

    // Create the error message content
    let fixErrorContent = "Fix Errors:\n";
    queriesWithErrors.forEach(query => {
      fixErrorContent += `Query: '${query.query}' faced an error: '${query.error?.message || "Unknown error"}'.\n`;
    });

    // Edit the user message to include the error message
    onSendMessage(fixErrorContent);
  };

  // New logic for fixing rollback errors
  const handleFixRollbackErrorAction = (message: Message) => {

    const queriesWithErrors = message.queries?.filter(q => q.error) || [];
    if (queriesWithErrors.length === 0) {
      toast.error("No errors found to fix");
      return;
    }
    // Create the error message content
    let fixRollbackErrorContent = "Fix Rollback Errors:";
    queriesWithErrors.forEach(query => {
      fixRollbackErrorContent += `Query: '${query.rollback_query != null && query.rollback_query != "" ? query.rollback_query : query.rollback_dependent_query}' faced an error: '${query.error?.message || "Unknown error"}'.\n`;
    });

    // Edit the user message to include the error message
    onSendMessage(fixRollbackErrorContent);
  }

  const handleConfirmClearChat = useCallback(async () => {
    // Track chat cleared event
    if (chat?.id) {
      analyticsService.trackChatCleared(chat.id, userId || '', userName || '');
    }

    await onClearChat();
    setShowClearConfirm(false);
  }, [chat?.id, onClearChat]);

  const handleCancelStreamClick = useCallback(() => {
    // Track query cancelled event
    if (chat?.id) {
      analyticsService.trackQueryCancelled(chat.id, userId || '', userName || '');
    }

    onCancelStream();
  }, [chat?.id, onCancelStream]);

  const handleConfirmRefreshSchema = useCallback(async () => {
    // Track schema refreshed event
    if (chat?.id) {
      analyticsService.trackSchemaRefreshed(chat.id, chat.connection.database, userId || '', userName || '');
    }

    await onRefreshSchema();
    setShowRefreshSchema(false);
  }, [chat?.id, chat?.connection.database, onRefreshSchema]);

  const handleCancelRefreshSchema = useCallback(async () => {
    // Track schema refresh cancelled event
    if (chat?.id) {
      analyticsService.trackSchemaCancelled(chat.id, chat.connection.database, userId || '', userName || '');
    }

    await onCancelRefreshSchema();
    setShowRefreshSchema(false);
  }, [chat?.id, chat?.connection.database, onCancelRefreshSchema]);

  return (
    <div className={`
      flex-1 
      flex 
      flex-col 
      h-screen 
      max-h-screen
      overflow-hidden
      transition-all 
      duration-300 
      ${isExpanded ? 'md:ml-80' : 'md:ml-20'}
    `}>
      <div className="relative">
        <ChatHeader
          chat={chat}
          isConnecting={isConnecting}
          isConnected={isConnected}
          onClearChat={() => setShowClearConfirm(true)}
          onEditConnection={() => {
            if (onEditConnectionFromChatWindow) {
              onEditConnectionFromChatWindow();
            } else {
              setShowEditConnection(true);
            }
          }}
          onShowCloseConfirm={() => setShowCloseConfirm(true)}
          onReconnect={handleReconnect}
          setShowRefreshSchema={() => setShowRefreshSchema(true)}
          onToggleSearch={handleToggleSearch}
        />
        
        {showSearch && (
          <SearchBar
            onSearch={performSearch}
            onClose={handleToggleSearch}
            onNavigateUp={navigateSearchUp}
            onNavigateDown={navigateSearchDown}
            currentResultIndex={currentSearchIndex}
            totalResults={searchResults.length}
            initialQuery={searchQuery}
          />
        )}
      </div>

      <div
        ref={chatContainerRef}
        data-chat-container
        className="
          flex-1 
          overflow-y-auto 
          bg-[#FFDB58]/10 
          relative 
          scroll-smooth 
          pb-24 
          md:pb-32 
          mt-16
          md:mt-0
          flex-shrink
        "
      >
        <div
          ref={loadingRef}
          className="h-20 flex items-center justify-center"
        >
          {isLoadingMessages && (
            <div className="flex items-center justify-center gap-2">
              <Loader2 className="w-4 h-4 animate-spin" />
              <span className="text-sm text-gray-600">Loading older messages...</span>
            </div>
          )}
        </div>

        <div
          className={`
            max-w-5xl 
            mx-auto
            px-4
            pt-16
            md:pt-0
            md:px-2
            xl:px-0
            transition-all 
            duration-300
            ${isExpanded
              ? 'md:ml-6 lg:ml-6 xl:mx-8 [@media(min-width:1760px)]:ml-[4rem] [@media(min-width:1920px)]:ml-[8.4rem]'
              : 'md:ml-[19rem] xl:mx-auto'
            }
          `}
        >
          {Object.entries(groupMessagesByDate(messages)).map(([date, dateMessages], index) => (
            <div key={date}>
              <div className={`flex items-center justify-center ${index === 0 ? 'mb-4' : 'my-6'}`}>
                <div className="
                  px-4 
                  py-2
                  bg-white 
                  text-sm 
                  font-medium 
                  text-black
                  border-2
                  border-black
                  shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]
                  rounded-full
                ">
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
                  onQueryUpdate={handleQueryUpdate}
                  onEditQuery={handleEditQuery}
                  userId={userId || ''}
                  userName={userName || ''}
                  searchQuery={showSearch ? searchQuery : ''}
                  isSearchResult={showSearch && searchResults.some(r => r === `msg-${message.id}`)}
                  isCurrentSearchResult={showSearch && searchResults[currentSearchIndex] === `msg-${message.id}`}
                  searchResultRefs={searchResultRefs}
                  buttonCallback={(action) => {
                    if (action === "refresh_schema") {
                      setShowRefreshSchema(true);
                    } else if (action === "fix_error") {
                      // Handle fix_error action
                      handleFixErrorAction(message);
                    } else if (action === "fix_rollback_error") {
                      // Handle fix_rollback_error action
                      handleFixRollbackErrorAction(message);
                    } else if (action === "open_settings") {
                      setOpenWithSettingsTab(true);
                      setShowEditConnection(true);
                    } else {
                      console.log(`Action not implemented: ${action}`);
                      toast.error(`There is no available action for this button: ${action}`);
                    }
                  }}
                />
              ))}
            </div>
          ))}
          {messages.length === 0 && (
            <div className="flex flex-col items-center justify-center h-full">
              <div className="
                  px-4 
                  py-2
                  bg-white 
                  text-sm 
                  font-medium 
                  text-black
                  border-2
                  border-black
                  shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]
                  rounded-full
                ">
                {formatDateDivider(new Date().toISOString())}
              </div>
              <MessageTile
                key={"welcome-message"}
                checkSSEConnection={checkSSEConnection}
                chatId={chat.id}
                message={{
                  id: "welcome-message",
                  type: "assistant",
                  content: `Hi ${userName || 'There'}! I am your Data Copilot. You can ask me anything about your data and i will understand your request & respond with data. You can start by asking me what's in this database or try recommendations.`,
                  queries: [],
                  action_buttons: [],
                  created_at: new Date().toISOString(),
                  updated_at: new Date().toISOString(),
                }}
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
                isFirstMessage={false}
                onQueryUpdate={handleQueryUpdate}
                onEditQuery={handleEditQuery}
                userId={userId || ''}
                userName={userName || ''}
                searchQuery={showSearch ? searchQuery : ''}
                isSearchResult={false}
                isCurrentSearchResult={false}
                searchResultRefs={searchResultRefs}
                buttonCallback={(action) => {
                  if (action === "refresh_schema") {
                    setShowRefreshSchema(true);
                  } else if (action === "open_settings") {
                    setOpenWithSettingsTab(true);
                    setShowEditConnection(true);
                  }
                }}
              />
            </div>
          )}
        </div>
        <div ref={messagesEndRef} />

        {messages.some(m => m.is_streaming) && (
          <div className="
            fixed 
            bottom-[88px]  // Position it above message input
            left-1/2 
            -translate-x-1/2 
            z-50
          ">
            <button
              onClick={handleCancelStreamClick}
              className="
                neo-border
                px-3
                py-2
                flex
                items-center
                gap-1.5
                bg-white
                text-sm
                font-medium
                hover:bg-red-50
                active:translate-y-[1px]
                active:shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]
              "
            >
              <XCircle className="w-3.5 h-3.5" />
              <span>Cancel Request</span>
            </button>
          </div>
        )}

        {showScrollButton && (
          <button
            onClick={() => scrollToBottom('scroll-button', true)}
            className="fixed bottom-24 right-4 md:right-8 p-3 bg-black text-white rounded-full shadow-lg hover:bg-gray-800 transition-all neo-border z-40"
            title="Scroll to bottom"
          >
            <ArrowDown className="w-6 h-6" />
          </button>
        )}
      </div>

      <MessageInput
        isConnected={isConnected}
        onSendMessage={handleMessageSubmit}
        isExpanded={isExpanded}
        isDisabled={isMessageSending}
        chatId={chat.id}
        userId={userId || ''}
        userName={userName || ''}
      />

      {showRefreshSchema && (
        <ConfirmationModal
          icon={<RefreshCcw className="w-6 h-6 text-black" />}
          themeColor="black"
          title="Refresh Knowledge Base"
          buttonText="Refresh"
          message="This action will refetch the schema from the database and update the knowledge base. This may take a few minutes depending on the size of the database."
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
          onConfirm={handleDisconnect}
          onCancel={() => setShowCloseConfirm(false)}
        />
      )}

      {showEditConnection && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/50">
          <ConnectionModal
            initialData={chat}
            initialTab={openWithSettingsTab ? 'settings' : undefined}
            onClose={() => {
              setShowEditConnection(false);
              setOpenWithSettingsTab(false);
            }}
            onEdit={async (data, autoExecuteQuery) => {
              const result = await onEditConnection?.(chat.id, data!, autoExecuteQuery!);
              return { success: result?.success || false, error: result?.error };
            }}
            onSubmit={async (data, autoExecuteQuery) => {
              const result = await onEditConnection?.(chat.id, data, autoExecuteQuery);
              return { success: result?.success || false, error: result?.error };
            }}
            onUpdateSelectedCollections={onUpdateSelectedCollections}
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