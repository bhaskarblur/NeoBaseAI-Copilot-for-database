import { useCallback, useState } from 'react';
import toast from 'react-hot-toast';
import { useStream } from '../contexts/StreamContext';
import axios from '../services/axiosConfig';

interface UseConnectionActionsParams {
    chatId: string;
    checkSSEConnection: () => Promise<void>;
    onCloseConnection: () => void;
    onConnectionStatusChange?: (chatId: string, isConnected: boolean, from: string) => void;
}

export function useConnectionActions({
    chatId,
    checkSSEConnection,
    onCloseConnection,
    onConnectionStatusChange,
}: UseConnectionActionsParams) {
    const [isConnecting, setIsConnecting] = useState(false);
    const { streamId, generateStreamId } = useStream();

    const checkConnectionStatus = useCallback(async (id: string) => {
        try {
            const response = await axios.get(
                `${import.meta.env.VITE_API_URL}/chats/${id}/connection-status`,
                {
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': `Bearer ${localStorage.getItem('token')}`,
                    },
                },
            );
            return response.data;
        } catch {
            return false;
        }
    }, []);

    const connectToDatabase = useCallback(async (id: string, sid: string) => {
        try {
            const response = await axios.post(
                `${import.meta.env.VITE_API_URL}/chats/${id}/connect`,
                { stream_id: sid },
                {
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': `Bearer ${localStorage.getItem('token')}`,
                    },
                },
            );
            return response.data;
        } catch (error: any) {
            throw error?.response?.data?.error ?? error;
        }
    }, []);

    const handleReconnect = useCallback(async () => {
        try {
            setIsConnecting(true);
            const sid = streamId || generateStreamId();
            await checkSSEConnection();
            await new Promise(resolve => setTimeout(resolve, 200));
            const status = await checkConnectionStatus(chatId);
            if (!status?.is_connected) {
                await connectToDatabase(chatId, sid);
            }
            onConnectionStatusChange?.(chatId, true, 'chat-window-reconnect');
        } catch (error) {
            console.error('Failed to reconnect to data source:', error);
            onConnectionStatusChange?.(chatId, false, 'chat-window-reconnect');
            toast.error('Failed to reconnect to data source: ' + error, {
                style: {
                    background: '#ff4444', color: '#fff',
                    border: '4px solid #cc0000', borderRadius: '12px',
                    boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)', padding: '12px 24px',
                },
            });
        } finally {
            setIsConnecting(false);
        }
    }, [chatId, streamId, generateStreamId, checkSSEConnection, onConnectionStatusChange, checkConnectionStatus, connectToDatabase]);

    const handleDisconnect = useCallback(async () => {
        try {
            await onCloseConnection();
        } catch (error) {
            console.error('Failed to disconnect:', error);
        }
    }, [onCloseConnection]);

    return {
        isConnecting,
        setIsConnecting,
        handleReconnect,
        handleDisconnect,
        connectToDatabase,
        checkConnectionStatus,
    };
}
