import { useCallback, useState } from 'react';
import toast from 'react-hot-toast';
import { useNavigate } from 'react-router-dom';
import axios from '../services/axiosConfig';
import chatService from '../services/chatService';
import { Chat, ChatSettings, Connection } from '../types/chat';
import { Message } from '../types/query';

const ERROR_TOAST = {
    style: {
        background: '#ff4444', color: '#fff',
        border: '4px solid #cc0000', borderRadius: '12px',
        boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
        padding: '12px 24px', fontSize: '16px', fontWeight: '500', zIndex: 9999,
    },
    duration: 4000,
    icon: '⚠️',
};

const SUCCESS_TOAST = {
    style: {
        background: '#000', color: '#fff', border: '4px solid #000', borderRadius: '12px',
        boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)', padding: '12px 24px',
        fontSize: '14px', fontWeight: '500',
    },
};

interface UseChatActionsParams {
    chats: Chat[];
    setChats: React.Dispatch<React.SetStateAction<Chat[]>>;
    selectedConnection: Chat | undefined;
    setSelectedConnection: (chat: Chat | undefined) => void;
    setMessages: React.Dispatch<React.SetStateAction<Message[]>>;
    setSuccessMessage: (msg: string | null) => void;
    connectionStatuses: Record<string, boolean>;
    handleConnectionStatusChange: (chatId: string, isConnected: boolean, from: string) => void;
    streamId: string | null;
    generateStreamId: () => string;
    eventSource: any | null;
    setEventSource: (es: any | null) => void;
    setupSSEConnection: (chatId: string) => Promise<string>;
}

export function useChatActions({
    chats,
    setChats,
    selectedConnection,
    setSelectedConnection,
    setMessages,
    setSuccessMessage,
    connectionStatuses,
    handleConnectionStatusChange,
    streamId,
    generateStreamId,
    eventSource,
    setEventSource,
    setupSSEConnection,
}: UseChatActionsParams) {
    const navigate = useNavigate();
    const [isSelectingConnection, setIsSelectingConnection] = useState(false);
    const [newlyCreatedChat, setNewlyCreatedChat] = useState<Chat | null>(null);
    const [, setShowSelectTablesModal] = useState(false);

    const handleAddConnection = useCallback(async (
        connection: Connection,
        settings: ChatSettings,
    ): Promise<{ success: boolean; error?: string; chatId?: string }> => {
        try {
            const newChat = await chatService.createChat(connection, settings);
            setChats(prev => [...prev, newChat]);
            setSuccessMessage('Connection added successfully!');
            return { success: true, chatId: newChat.id };
        } catch (error: any) {
            toast.error(error.message, ERROR_TOAST);
            return { success: false, error: error.message };
        }
    }, [setChats, setSuccessMessage]);

    const handleDeleteConnection = useCallback(async (id: string) => {
        setChats(prev => prev.filter(c => c.id !== id));
        if (selectedConnection?.id === id) {
            setSelectedConnection(undefined);
            setMessages([]);
            navigate('/');
        }
        setSuccessMessage('Connection deleted successfully');
    }, [selectedConnection, setChats, setSelectedConnection, setMessages, navigate, setSuccessMessage]);

    const handleEditConnection = useCallback(async (
        id: string,
        data: Connection,
        settings: ChatSettings,
    ): Promise<{ success: boolean; error?: string; updatedChat?: Chat }> => {
        const loadingId = toast.loading('Updating connection...', {
            style: { background: '#000', color: '#fff', borderRadius: '12px', border: '4px solid #000' },
        });

        try {
            const credentialsChanged = data && selectedConnection && (
                selectedConnection.connection.database !== data.database ||
                selectedConnection.connection.host !== data.host ||
                selectedConnection.connection.port !== data.port ||
                selectedConnection.connection.username !== data.username
            );

            const response = await chatService.editChat(id, data, settings);
            if (response) {
                setChats(prev => prev.map(c => c.id === id ? response : c));
                if (selectedConnection?.id === id) setSelectedConnection(response);

                if (credentialsChanged && streamId) {
                    try {
                        await chatService.disconnectFromConnection(id, streamId);
                        await new Promise(resolve => setTimeout(resolve, 1000));
                        await chatService.connectToConnection(id, streamId);
                        handleConnectionStatusChange(id, true, 'edit-connection');
                    } catch {
                        toast.error('Failed to reconnect. Please reconnect manually.');
                    }
                } else if (credentialsChanged) {
                    toast.error('Connection details updated. Please reconnect to the data source.');
                    handleConnectionStatusChange(id, false, 'edit-connection-no-stream');
                }

                toast.success('Connection updated successfully!', SUCCESS_TOAST);
                toast.dismiss(loadingId);
                return { success: true, updatedChat: response };
            }

            toast.dismiss(loadingId);
            return { success: false, error: 'Failed to update connection' };
        } catch (error: any) {
            toast.dismiss(loadingId);
            return { success: false, error: error.message || 'Failed to update connection' };
        }
    }, [selectedConnection, setChats, setSelectedConnection, streamId, handleConnectionStatusChange]);

    const handleSelectConnection = useCallback(async (id: string) => {
        if (id === selectedConnection?.id) return;
        const currentId = selectedConnection?.id;
        let connection = chats.find(c => c.id === id);
        if (!connection) return;

        setIsSelectingConnection(true);
        if (currentId !== id) setMessages([]);

        try {
            connection = await chatService.getChat(id);
        } catch {
            // use cached
        }

        setSelectedConnection(connection);
        navigate(`/chat/${id}`);

        const isAlreadyConnected = connectionStatuses[id];
        if (isAlreadyConnected) {
            handleConnectionStatusChange(id, true, 'app-select-connection');
        } else {
            const status = await chatService.checkConnectionStatus(id);
            if (status) {
                handleConnectionStatusChange(id, true, 'app-select-connection');
            } else {
                const sid = streamId || generateStreamId();
                await chatService.connectToConnection(id, sid);
                handleConnectionStatusChange(id, true, 'app-select-connection');
            }
        }

        if (currentId !== id) {
            await setupSSEConnection(id);
        }

        setTimeout(() => setIsSelectingConnection(false), 100);
    }, [
        chats, connectionStatuses, selectedConnection, streamId,
        generateStreamId, setMessages, setSelectedConnection, navigate,
        handleConnectionStatusChange, setupSSEConnection,
    ]);

    const handleCloseConnection = useCallback(async () => {
        setIsSelectingConnection(true);
        navigate('/');

        if (eventSource) {
            eventSource.close();
            setEventSource(null);
            await chatService.disconnectFromConnection(selectedConnection?.id || '', streamId || '');
            handleConnectionStatusChange(selectedConnection?.id || '', false, 'app-close-connection');
        }

        if (selectedConnection) {
            handleConnectionStatusChange(selectedConnection.id, false, 'app-close-connection');
        }

        setMessages([]);
        setSelectedConnection(undefined);
        setTimeout(() => setIsSelectingConnection(false), 200);
    }, [eventSource, selectedConnection, streamId, setEventSource, setMessages, setSelectedConnection, navigate, handleConnectionStatusChange]);

    const handleDuplicateConnection = useCallback(async (chatId: string) => {
        try {
            const duplicated = await chatService.getChat(chatId);
            setChats(prev => prev.some(c => c.id === duplicated.id) ? prev : [...prev, duplicated]);
            setSelectedConnection(duplicated);
            setSuccessMessage('Chat duplicated successfully!');

            try {
                await setupSSEConnection(chatId);
                handleConnectionStatusChange(chatId, true, 'duplicate-connection');
            } catch {
                toast('Chat duplicated, but connection could not be established automatically.', {
                    style: { background: '#ffcc00', color: '#000', border: '4px solid #e6b800', borderRadius: '12px', padding: '12px 24px', boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)' },
                    icon: '⚠️',
                });
            }
        } catch (error: any) {
            toast.error(`Failed to duplicate chat: ${error.message}`, ERROR_TOAST);
        }
    }, [setChats, setSelectedConnection, setSuccessMessage, setupSSEConnection, handleConnectionStatusChange]);

    const handleUpdateSelectedCollections = useCallback(async (chatId: string, selectedCollections: string): Promise<void> => {
        const loadingId = toast.loading('Updating selected tables...', {
            style: { background: '#000', color: '#fff', borderRadius: '12px', border: '4px solid #000' },
        });

        try {
            await chatService.updateSelectedCollections(chatId, selectedCollections);
            setChats(prev => prev.map(c => c.id === chatId ? { ...c, selected_collections: selectedCollections } : c));
            if (selectedConnection?.id === chatId) {
                setSelectedConnection({ ...selectedConnection, selected_collections: selectedCollections });
            }
            toast.dismiss(loadingId);

            if (newlyCreatedChat?.id === chatId) {
                setShowSelectTablesModal(false);
                setNewlyCreatedChat(null);
                await handleSelectConnection(chatId);
            }
        } catch (error: any) {
            toast.dismiss(loadingId);
            toast.error(error.message || 'Failed to update tables selection');
            throw error;
        }
    }, [selectedConnection, setChats, setSelectedConnection, newlyCreatedChat, handleSelectConnection]);

    const handleClearChat = useCallback(async () => {
        try {
            await axios.delete(`${import.meta.env.VITE_API_URL}/chats/${selectedConnection?.id}/messages`, {
                withCredentials: true,
                headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` },
            });
            setMessages([]);
        } catch (error: any) {
            toast.error(error.message, ERROR_TOAST);
        }
    }, [selectedConnection, setMessages]);

    return {
        isSelectingConnection,
        newlyCreatedChat,
        setNewlyCreatedChat,
        handleAddConnection,
        handleDeleteConnection,
        handleEditConnection,
        handleSelectConnection,
        handleCloseConnection,
        handleDuplicateConnection,
        handleUpdateSelectedCollections,
        handleClearChat,
    };
}
