import { Eraser, ListRestart, Loader, Pencil, PlugZap, RefreshCw, Search } from 'lucide-react';
import { useCallback, useMemo, useState, useEffect } from 'react';
import { Chat } from '../../types/chat';
import analyticsService from '../../services/analyticsService';
import DatabaseLogo from '../icons/DatabaseLogos';
import DisconnectionTooltip from './DisconnectionTooltip';

interface ChatHeaderProps {
    chat: Chat;
    isConnecting: boolean;
    isConnected: boolean;
    onClearChat: () => void;
    onEditConnection: () => void;
    onShowCloseConfirm: () => void;
    onReconnect: () => void;
    setShowRefreshSchema: (show: boolean) => void;
    onToggleSearch: () => void;
}

export default function ChatHeader({
    chat,
    isConnecting = true,
    isConnected,
    onClearChat,
    onEditConnection,
    onShowCloseConfirm,
    onReconnect,
    setShowRefreshSchema,
    onToggleSearch,
}: ChatHeaderProps) {
    const [showDisconnectionTooltip, setShowDisconnectionTooltip] = useState(false);

    // Show tooltip when connection is lost
    useEffect(() => {
        if (!isConnected && !isConnecting) {
            // Show tooltip after a short delay
            const timer = setTimeout(() => {
                setShowDisconnectionTooltip(true);
            }, 1000);
            
            return () => clearTimeout(timer);
        } else {
            setShowDisconnectionTooltip(false);
        }
    }, [isConnected, isConnecting]);

    const connectionStatus = useMemo(() => {
        if (isConnecting) {
            return (
                <span className="text-yellow-600 text-sm font-medium bg-yellow-100 px-2 py-1 rounded flex items-center gap-2">
                    <Loader className="w-3 h-3 animate-spin" />
                    Connecting...
                </span>
            );
        }
        return isConnected ? (
            <span className="text-emerald-700 text-sm font-medium bg-emerald-100 px-2 py-1 rounded">
                Connected
            </span>
        ) : (
            <span className="text-neo-error text-sm font-medium bg-neo-error/10 px-2 py-1 rounded">
                Disconnected
            </span>
        );
    }, [isConnecting, isConnected]);

    // Wrap handlers with analytics events
    const handleClearChat = useCallback(() => {
        // Track clear chat click event
        analyticsService.trackEvent('clear_chat_clicked', { 
            chatId: chat.id,
            connectionName: chat.connection.database
        });
        
        onClearChat();
    }, [chat.id, chat.connection.database, onClearChat]);

    const handleEditConnection = useCallback(() => {
        // Track edit connection click event
        analyticsService.trackEvent('edit_connection_clicked', { 
            chatId: chat.id,
            connectionName: chat.connection.database
        });
        
        onEditConnection();
    }, [chat.id, chat.connection.database, onEditConnection]);

    const handleShowCloseConfirm = useCallback(() => {
        // Track disconnect click event
        analyticsService.trackEvent('disconnect_clicked', { 
            chatId: chat.id,
            connectionName: chat.connection.database
        });
        
        onShowCloseConfirm();
    }, [chat.id, chat.connection.database, onShowCloseConfirm]);

    const handleReconnect = useCallback(() => {
        // Track reconnect click event
        analyticsService.trackEvent('reconnect_clicked', { 
            chatId: chat.id,
            connectionName: chat.connection.database
        });
        
        onReconnect();
    }, [chat.id, chat.connection.database, onReconnect]);

    const handleShowRefreshSchema = useCallback(() => {
        // Track refresh schema click event
        analyticsService.trackEvent('refresh_schema_clicked', { 
            chatId: chat.id,
            connectionName: chat.connection.database
        });
        
        setShowRefreshSchema(true);
    }, [chat.id, chat.connection.database, setShowRefreshSchema]);

    return (
        <div className="fixed top-0 left-0 right-0 md:relative md:left-auto md:right-auto bg-white border-b-4 border-black h-16 px-4 flex justify-between items-center mt-16 md:mt-0 z-20">
            <div className="flex items-center gap-2 overflow-hidden max-w-[60%]">
                <DatabaseLogo type={chat.connection.type as "postgresql" | "mysql" | "mongodb" | "redis" | "clickhouse" | "neo4j"} size={32} className="transition-transform hover:scale-110" />
                <h2 className="text-lg md:text-2xl font-bold truncate">{chat.connection.is_example_db ? "Sample Database" : chat.connection.database}</h2>
                {connectionStatus}
            </div>
            <div className="flex items-center gap-2">
                {/* Desktop buttons with borders */}
                <div className="relative group hidden md:block">
                    <button
                        onClick={onToggleSearch}
                        className="p-2 hover:bg-neo-gray rounded-lg transition-colors neo-border text-gray-800"
                        aria-label="Search messages"
                    >
                        <Search className="w-5 h-5" />
                    </button>
                    <div className="absolute invisible opacity-0 group-hover:visible group-hover:opacity-100 transition-opacity duration-200 bottom-[-35px] left-1/2 transform -translate-x-1/2 bg-black text-white text-xs py-1 px-2 rounded whitespace-nowrap z-50 before:content-[''] before:absolute before:top-[-5px] before:left-1/2 before:transform before:-translate-x-1/2 before:border-[5px] before:border-transparent before:border-b-black">
                        Search messages
                    </div>
                </div>

                <div className="relative group hidden md:block">
                    <button
                        onClick={handleShowRefreshSchema}
                        className="p-2 hover:bg-neo-gray rounded-lg transition-colors neo-border text-gray-800"
                        aria-label="Refresh Knowledge Base"
                    >
                        <ListRestart className="w-5 h-5" />
                    </button>
                    <div className="absolute invisible opacity-0 group-hover:visible group-hover:opacity-100 transition-opacity duration-200 bottom-[-35px] left-1/2 transform -translate-x-1/2 bg-black text-white text-xs py-1 px-2 rounded whitespace-nowrap z-50 before:content-[''] before:absolute before:top-[-5px] before:left-1/2 before:transform before:-translate-x-1/2 before:border-[5px] before:border-transparent before:border-b-black">
                        Refresh Knowledge Base
                    </div>
                </div>

                <div className="relative group hidden md:block">
                    <button
                        onClick={handleClearChat}
                        className="p-2 text-neo-error hover:bg-neo-error/10 rounded-lg transition-colors neo-border"
                        aria-label="Clear chat"
                    >
                        <Eraser className="w-5 h-5" />
                    </button>
                    <div className="absolute invisible opacity-0 group-hover:visible group-hover:opacity-100 transition-opacity duration-200 bottom-[-35px] left-1/2 transform -translate-x-1/2 bg-black text-white text-xs py-1 px-2 rounded whitespace-nowrap z-50 before:content-[''] before:absolute before:top-[-5px] before:left-1/2 before:transform before:-translate-x-1/2 before:border-[5px] before:border-transparent before:border-b-black">
                        Clear chat
                    </div>
                </div>

                {/* <div className="relative group hidden md:block">
                    <button
                        onClick={handleEditConnection}
                        className="p-2 hover:bg-neo-gray rounded-lg transition-colors neo-border text-gray-800"
                        aria-label="Edit connection"
                    >
                        <Pencil className="w-5 h-5" />
                    </button>
                    <div className="absolute invisible opacity-0 group-hover:visible group-hover:opacity-100 transition-opacity duration-200 bottom-[-35px] left-1/2 transform -translate-x-1/2 bg-black text-white text-xs py-1 px-2 rounded whitespace-nowrap z-50 before:content-[''] before:absolute before:top-[-5px] before:left-1/2 before:transform before:-translate-x-1/2 before:border-[5px] before:border-transparent before:border-b-black">
                        Edit connection
                    </div>
                </div> */}

                {isConnected ? (
                    <div className="relative group hidden md:block">
                        <button
                            onClick={handleShowCloseConfirm}
                            className="p-2 hover:bg-neo-gray rounded-lg transition-colors neo-border text-gray-800"
                            aria-label="Disconnect"
                        >
                            <PlugZap className="w-5 h-5" />
                        </button>
                        <div className="absolute invisible opacity-0 group-hover:visible group-hover:opacity-100 transition-opacity duration-200 bottom-[-35px] left-1/2 transform -translate-x-1/2 bg-black text-white text-xs py-1 px-2 rounded whitespace-nowrap z-50 before:content-[''] before:absolute before:top-[-5px] before:left-1/2 before:transform before:-translate-x-1/2 before:border-[5px] before:border-transparent before:border-b-black">
                            Disconnect
                        </div>
                    </div>
                ) : (
                    <div className="relative group hidden md:block">
                        <button
                            onClick={handleReconnect}
                            className="p-2 hover:bg-neo-gray rounded-lg transition-colors neo-border"
                            aria-label="Reconnect"
                        >
                            <RefreshCw className="w-5 h-5 text-gray-800" />
                        </button>
                        <div className="absolute invisible opacity-0 group-hover:visible group-hover:opacity-100 transition-opacity duration-200 bottom-[-35px] left-1/2 transform -translate-x-1/2 bg-black text-white text-xs py-1 px-2 rounded whitespace-nowrap z-50 before:content-[''] before:absolute before:top-[-5px] before:left-1/2 before:transform before:-translate-x-1/2 before:border-[5px] before:border-transparent before:border-b-black">
                            Reconnect
                        </div>
                        <DisconnectionTooltip
                            isVisible={showDisconnectionTooltip}
                            onClose={() => setShowDisconnectionTooltip(false)}
                        />
                    </div>
                )}

                {/* Mobile buttons without borders */}
                <div className="relative group md:hidden">
                    <button
                        onClick={onToggleSearch}
                        className="p-2 hover:bg-neo-gray rounded-lg transition-colors"
                        aria-label="Search messages"
                    >
                        <Search className="w-5 h-5" />
                    </button>
                    <div className="absolute invisible opacity-0 group-hover:visible group-hover:opacity-100 transition-opacity duration-200 bottom-[-35px] left-1/2 transform -translate-x-1/2 bg-black text-white text-xs py-1 px-2 rounded whitespace-nowrap z-50 before:content-[''] before:absolute before:top-[-5px] before:left-1/2 before:transform before:-translate-x-1/2 before:border-[5px] before:border-transparent before:border-b-black">
                        Search messages
                    </div>
                </div>

                <div className="relative group md:hidden">
                    <button
                        onClick={handleShowRefreshSchema}
                        className="p-2 hover:bg-neo-gray rounded-lg transition-colors"
                        aria-label="Refresh Knowledge Base"
                    >
                        <ListRestart className="w-5 h-5" />
                    </button>
                    <div className="absolute invisible opacity-0 group-hover:visible group-hover:opacity-100 transition-opacity duration-200 bottom-[-35px] left-1/2 transform -translate-x-1/2 bg-black text-white text-xs py-1 px-2 rounded whitespace-nowrap z-50 before:content-[''] before:absolute before:top-[-5px] before:left-1/2 before:transform before:-translate-x-1/2 before:border-[5px] before:border-transparent before:border-b-black">
                        Refresh Knowledge Base
                    </div>
                </div>

                <div className="relative group md:hidden">
                    <button
                        onClick={handleClearChat}
                        className="p-2 text-neo-error hover:bg-neo-error/10 rounded-lg transition-colors"
                        aria-label="Clear chat"
                    >
                        <Eraser className="w-5 h-5" />
                    </button>
                    <div className="absolute invisible opacity-0 group-hover:visible group-hover:opacity-100 transition-opacity duration-200 bottom-[-35px] left-1/2 transform -translate-x-1/2 bg-black text-white text-xs py-1 px-2 rounded whitespace-nowrap z-50 before:content-[''] before:absolute before:top-[-5px] before:left-1/2 before:transform before:-translate-x-1/2 before:border-[5px] before:border-transparent before:border-b-black">
                        Clear chat
                    </div>
                </div>

                {/* <div className="relative group md:hidden">
                    <button
                        onClick={handleEditConnection}
                        className="p-2 hover:bg-neo-gray rounded-lg transition-colors"
                        aria-label="Edit connection"
                    >
                        <Pencil className="w-5 h-5" />
                    </button>
                    <div className="absolute invisible opacity-0 group-hover:visible group-hover:opacity-100 transition-opacity duration-200 bottom-[-35px] left-1/2 transform -translate-x-1/2 bg-black text-white text-xs py-1 px-2 rounded whitespace-nowrap z-50 before:content-[''] before:absolute before:top-[-5px] before:left-1/2 before:transform before:-translate-x-1/2 before:border-[5px] before:border-transparent before:border-b-black">
                        Edit connection
                    </div>
                </div> */}

                {isConnected ? (
                    <div className="relative group md:hidden">
                        <button
                            onClick={handleShowCloseConfirm}
                            className="p-2 hover:bg-neo-gray rounded-lg transition-colors"
                            aria-label="Disconnect connection"
                        >
                            <PlugZap className="w-5 h-5 text-gray-800" />
                        </button>
                        <div className="absolute invisible opacity-0 group-hover:visible group-hover:opacity-100 transition-opacity duration-200 bottom-[-35px] left-1/2 transform -translate-x-1/2 bg-black text-white text-xs py-1 px-2 rounded whitespace-nowrap z-50 before:content-[''] before:absolute before:top-[-5px] before:left-1/2 before:transform before:-translate-x-1/2 before:border-[5px] before:border-transparent before:border-b-black">
                            Disconnect connection
                        </div>
                    </div>
                ) : (
                    <div className="relative group md:hidden">
                        <button
                            onClick={handleReconnect}
                            className="p-2 hover:bg-neo-gray rounded-lg transition-colors"
                            aria-label="Reconnect"
                        >
                            <RefreshCw className="w-5 h-5 text-gray-800" />
                        </button>
                        <div className="absolute invisible opacity-0 group-hover:visible group-hover:opacity-100 transition-opacity duration-200 bottom-[-35px] left-1/2 transform -translate-x-1/2 bg-black text-white text-xs py-1 px-2 rounded whitespace-nowrap z-50 before:content-[''] before:absolute before:top-[-5px] before:left-1/2 before:transform before:-translate-x-1/2 before:border-[5px] before:border-transparent before:border-b-black">
                            Reconnect
                        </div>
                        <DisconnectionTooltip
                            isVisible={showDisconnectionTooltip}
                            onClose={() => setShowDisconnectionTooltip(false)}
                        />
                    </div>
                )}
            </div>
        </div>
    );
}