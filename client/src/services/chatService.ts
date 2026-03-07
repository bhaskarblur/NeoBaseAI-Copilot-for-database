import { Chat, Connection, TablesResponse, ChatSettings } from '../types/chat';
import { ExecuteQueryResponse, MessagesResponse, SendMessageResponse } from '../types/messages';
import axios from './axiosConfig';

const API_URL = import.meta.env.VITE_API_URL;

interface CreateChatResponse {
    success: boolean;
    data: Chat;
}

interface QueryRecommendationsResponse {
    success: boolean;
    data: {
        recommendations: Array<{
            text: string;
        }>;
    };
}

// No frontend tables caching — always fetch fresh from backend to avoid stale data.

const chatService = {
    async createChat(connection: Connection, settings: ChatSettings): Promise<Chat> {
        try {
            // Ensure we send the ssl_mode when it's present
            const connectionToSend = { ...connection };
            
            // Make sure ssl_mode is included if present
            if (connection.use_ssl && connection.ssl_mode) {
                connectionToSend.ssl_mode = connection.ssl_mode;
            }
            
            const response = await axios.post<CreateChatResponse>(`${API_URL}/chats`, {
                connection: connectionToSend,
                settings: {
                    auto_execute_query: settings.auto_execute_query,
                    share_data_with_ai: settings.share_data_with_ai,
                    non_tech_mode: settings.non_tech_mode
                }
            });

            if (!response.data.success) {
                throw new Error('Failed to create chat');
            }

            return response.data.data;
        } catch (error: any) {
            console.error('Create chat error:', error);
            throw new Error(error.response?.data?.error || 'Failed to create chat');
        }
    },
    
    async editChat(chatId: string, connection?: Connection, settings?: ChatSettings): Promise<Chat> {
        try {
            // Ensure we send the ssl_mode when it's present
            const connectionToSend = { ...connection };
            
            // Make sure ssl_mode is included if present
            if (connection?.use_ssl && connection?.ssl_mode) {
                connectionToSend.ssl_mode = connection.ssl_mode;
            }
            
            const payload: any = { connection: connection ? connectionToSend : undefined, settings: {
                auto_execute_query: settings?.auto_execute_query,
                share_data_with_ai: settings?.share_data_with_ai,
                non_tech_mode: settings?.non_tech_mode,
                auto_generate_visualization: settings?.auto_generate_visualization
            } };
            
            const response = await axios.patch<CreateChatResponse>(
                `${API_URL}/chats/${chatId}`,
                payload,
                {
                    withCredentials: true,
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': `Bearer ${localStorage.getItem('token')}`
                    }
                }
            );

            if (!response.data.success) {
                throw new Error('Failed to edit chat');
            }

            return response.data.data;
        } catch (error: any) {
            console.error('Edit chat error:', error);
            throw new Error(error.response?.data?.error || 'Failed to edit chat');
        }
    },
    async deleteChat(chatId: string): Promise<void> {
        try {
            const response = await axios.delete(`${API_URL}/chats/${chatId}`);

            if (!response.data.success && response.status !== 200) {
                throw new Error('Failed to delete chat');
            }
        } catch (error: any) {
            console.error('Delete chat error:', error);
            throw new Error(error.response?.data?.error || 'Failed to delete chat');
        }
    },

    async checkConnectionStatus(chatId: string): Promise<boolean> {
        try {
            const response = await axios.get(`${API_URL}/chats/${chatId}/connection-status`);
            return response.data.success;
        } catch (error: any) {
            console.error('Check connection status error:', error);
            return false;
        }
    },

    async connectToConnection(chatId: string, streamId: string): Promise<void> {
        try {
            const response = await axios.post(`${API_URL}/chats/${chatId}/connect`, { stream_id: streamId });
            return response.data.success;
        } catch (error: any) {
            console.error('Connect to connection error:', error);
            throw new Error(error.response?.data?.error || 'Failed to connect');
        }
    },

    async disconnectFromConnection(chatId: string, streamId: string): Promise<void> {
        try {
            const response = await axios.post(`${API_URL}/chats/${chatId}/disconnect`, {
                stream_id: streamId
            }, {
                withCredentials: true,
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('token')}`
                }
            });
            return response.data.success;
        } catch (error: any) {
            console.error('Disconnect from connection error:', error);
            throw new Error(error.response?.data?.error || 'Failed to disconnect from connection');
        }
    },

    async getMessages(chatId: string, page: number, perPage: number): Promise<MessagesResponse> {
        try {
            const response = await axios.get<MessagesResponse>(
                `${import.meta.env.VITE_API_URL}/chats/${chatId}/messages?page=${page}&page_size=${perPage}`,
                {
                    withCredentials: true,
                    headers: {
                        'Authorization': `Bearer ${localStorage.getItem('token')}`
                    }
                }
            );
            return response.data;
        } catch (error: any) {
            console.error('Get messages error:', error);
            throw new Error(error.response?.data?.error || 'Failed to get messages');
        }
    },
    async sendMessage(chatId: string, messageId: string, streamId: string, content: string, llmModel?: string): Promise<SendMessageResponse> {
        try {
            const response = await axios.post<SendMessageResponse>(
                `${API_URL}/chats/${chatId}/messages`,
                {
                    message_id: messageId,
                    stream_id: streamId,
                    content: content,
                    llm_model: llmModel
                },
                {
                    withCredentials: true,
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': `Bearer ${localStorage.getItem('token')}`
                    }
                }
            );
            return response.data
        } catch (error: any) {
            console.error('Send message error:', error);
            throw new Error(error.response?.data?.error || 'Failed to send message');
        }
    },
    async cancelStream(chatId: string, streamId: string): Promise<void> {
        try {
            await axios.post(
                `${import.meta.env.VITE_API_URL}/chats/${chatId}/stream/cancel?stream_id=${streamId}`,
                {},
                {
                    withCredentials: true,
                    headers: {
                        'Authorization': `Bearer ${localStorage.getItem('token')}`
                    }
                }
            );
        } catch (error: any) {
            console.error('Cancel stream error:', error);
            throw new Error(error.response?.data?.error || 'Failed to cancel stream');
        }
    },

    async executeQuery(chatId: string, messageId: string, queryId: string, streamId: string, controller: AbortController): Promise<ExecuteQueryResponse | undefined> {
        try {
            const response = await axios.post<ExecuteQueryResponse>(
                `${API_URL}/chats/${chatId}/queries/execute`,
                {
                    message_id: messageId,
                    query_id: queryId,
                    stream_id: streamId
                },
                {
                    signal: controller.signal,
                    withCredentials: true,
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': `Bearer ${localStorage.getItem('token')}`
                    }
                }
            );
            console.log('chatService executeQuery response', response);
            return response.data;
        } catch (error: any) {
            if (error.name === 'CanceledError' || error.name === 'AbortError') {
                return undefined;
            }
            console.error('Execute query error:', error);
            throw new Error(error.response?.data?.error || 'Failed to execute query');
        }
    },

    async rollbackQuery(chatId: string, messageId: string, queryId: string, streamId: string, controller: AbortController): Promise<ExecuteQueryResponse | undefined> {
        try {
            const response = await axios.post<ExecuteQueryResponse>(`${API_URL}/chats/${chatId}/queries/rollback`, {
                message_id: messageId,
                query_id: queryId,
                stream_id: streamId
            },
                {
                    signal: controller.signal,
                    withCredentials: true,
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': `Bearer ${localStorage.getItem('token')}`
                    }
                }
            );
            return response.data;
        } catch (error: any) {
            if (error.name === 'CanceledError' || error.name === 'AbortError') {
                return undefined;
            }
            console.error('Rollback query error:', error);
            throw new Error(error.response?.data?.error || 'Failed to rollback query');
        }
    },

    async refreshSchema(chatId: string, controller: AbortController): Promise<boolean> {
        try {
            const response = await axios.post(`${API_URL}/chats/${chatId}/refresh-schema`, {
                withCredentials: true,
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('token')}`
                }
            }, {
                signal: controller.signal,
            });
            return response.data.success;
        } catch (error: any) {
            if (error.name === 'CanceledError' || error.name === 'AbortError') {
                return false;
            }
            console.error('Refresh knowledge base error:', error);
            throw new Error(error.response?.data?.error || 'Failed to refresh knowledge base');
        }
    },

    async editQuery(
        chatId: string,
        messageId: string,
        queryId: string,
        query: string
    ): Promise<{ success: boolean; data?: any }> {
        try {
            const response = await axios.patch(
                `${import.meta.env.VITE_API_URL}/chats/${chatId}/queries/edit`,
                {
                    "message_id": messageId,
                    "query_id": queryId,
                    "query": query
                },
                {
                    withCredentials: true,
                }
            );
            return { success: true, data: response.data };
        } catch (error: any) {
            throw error.response?.data?.error || 'Failed to edit query';
        }
    },

    async updateSelectedCollections(chatId: string, selectedCollections: string): Promise<Chat> {
        try {
            const response = await axios.patch<CreateChatResponse>(
                `${API_URL}/chats/${chatId}`,
                {
                    selected_collections: selectedCollections
                },
                {
                    withCredentials: true,
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': `Bearer ${localStorage.getItem('token')}`
                    }
                }
            );

            if (!response.data.success) {
                throw new Error('Failed to update selected collections');
            }

            return response.data.data;
        } catch (error: any) {
            console.error('Update selected collections error:', error);
            throw new Error(error.response?.data?.error || 'Failed to update selected collections');
        }
    },

    // Add a method to get a single chat
    async getChat(chatId: string): Promise<Chat> {
        try {
            const response = await axios.get<{success: boolean, data: Chat}>(
                `${API_URL}/chats/${chatId}`,
                {
                    withCredentials: true,
                    headers: {
                        'Authorization': `Bearer ${localStorage.getItem('token')}`
                    }
                }
            );

            if (!response.data.success) {
                throw new Error('Failed to get chat');
            }

            return response.data.data;
        } catch (error: any) {
            console.error('Get chat error:', error);
            throw new Error(error.response?.data?.error || 'Failed to get chat');
        }
    },

      // Add a method to get a single chat
      async duplicateChat(chatId: string, duplicateMessages: boolean = false, duplicateDashboards: boolean = false): Promise<Chat> {
        try {
            const response = await axios.post<{success: boolean, data: Chat}>(
                `${API_URL}/chats/${chatId}/duplicate?duplicate_messages=${duplicateMessages}&duplicate_dashboards=${duplicateDashboards}`,
                {},  // Empty body
                {    // Proper options object with headers
                    withCredentials: true,
                    headers: {
                        'Authorization': `Bearer ${localStorage.getItem('token')}`,
                        'Content-Type': 'application/json'
                    }
                }
            );

            if (!response.data.success) {
                throw new Error('Failed to duplicate chat');
            }

            return response.data.data;
        } catch (error: any) {
            console.error('Duplicate chat error:', error);
            throw new Error(error.response?.data?.error || 'Failed to duplicate chat');
        }
    },

    async getTables(chatId: string): Promise<TablesResponse> {
        try {
            const response = await axios.get<{success: boolean, data: TablesResponse}>(
                `${API_URL}/chats/${chatId}/tables`,
                {
                    withCredentials: true,
                    headers: {
                        'Authorization': `Bearer ${localStorage.getItem('token')}`
                    },
                    timeout: 120000
                }
            );

            if (!response.data.success) {
                throw new Error('Failed to get tables');
            }

            return response.data.data;
        } catch (error: any) {
            console.error('Get tables error:', error);
            throw new Error(error.response?.data?.error || 'Failed to get tables');
        }
    },

    async updateAutoExecuteQuery(chatId: string, autoExecuteQuery: boolean): Promise<Chat> {
        try {
            const response = await axios.patch<CreateChatResponse>(
                `${API_URL}/chats/${chatId}`,
                {
                    auto_execute_query: autoExecuteQuery
                },
                {
                    withCredentials: true,
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': `Bearer ${localStorage.getItem('token')}`
                    }
                }
            );

            if (!response.data.success) {
                throw new Error('Failed to update auto execute query setting');
            }

            return response.data.data;
        } catch (error: any) {
            console.error('Update auto execute query error:', error);
            throw new Error(error.response?.data?.error || 'Failed to update auto execute query setting');
        }
    },

    async getQueryRecommendations(chatId: string, streamId?: string): Promise<QueryRecommendationsResponse> {
        try {
            const params = streamId ? { stream_id: streamId } : {};
            const response = await axios.get<QueryRecommendationsResponse>(
                `${API_URL}/chats/${chatId}/recommendations`,
                {
                    params,
                    withCredentials: true,
                    headers: {
                        'Authorization': `Bearer ${localStorage.getItem('token')}`
                    }
                }
            );
            return response.data;
        } catch (error: any) {
            console.error('Get query recommendations error:', error);
            throw new Error(error.response?.data?.error || 'Failed to get query recommendations');
        }
    },

    async pinMessage(chatId: string, messageId: string): Promise<{success: boolean}> {
        try {
            const response = await axios.post(
                `${API_URL}/chats/${chatId}/messages/${messageId}/pin`,
                {},
                {
                    withCredentials: true,
                    headers: {
                        'Authorization': `Bearer ${localStorage.getItem('token')}`
                    }
                }
            );
            return response.data;
        } catch (error: any) {
            console.error('Pin message error:', error);
            throw new Error(error.response?.data?.error || 'Failed to pin message');
        }
    },

    async unpinMessage(chatId: string, messageId: string): Promise<{success: boolean}> {
        try {
            const response = await axios.delete(
                `${API_URL}/chats/${chatId}/messages/${messageId}/pin`,
                {
                    withCredentials: true,
                    headers: {
                        'Authorization': `Bearer ${localStorage.getItem('token')}`
                    }
                }
            );
            return response.data;
        } catch (error: any) {
            console.error('Unpin message error:', error);
            throw new Error(error.response?.data?.error || 'Failed to unpin message');
        }
    },

    async getPinnedMessages(chatId: string): Promise<MessagesResponse> {
        try {
            const response = await axios.get(
                `${API_URL}/chats/${chatId}/messages/pinned`,
                {
                    withCredentials: true,
                    headers: {
                        'Authorization': `Bearer ${localStorage.getItem('token')}`
                    }
                }
            );
            return response.data;
        } catch (error: any) {
            console.error('Get pinned messages error:', error);
            throw new Error(error.response?.data?.error || 'Failed to get pinned messages');
        }
    },

    async generateVisualization(chatId: string, messageId: string, queryId: string): Promise<any> {
        try {
            const response = await axios.post(
                `${API_URL}/chats/${chatId}/messages/${messageId}/visualizations`,
                {
                    query_id: queryId
                },
                {
                    withCredentials: true,
                    headers: {
                        'Authorization': `Bearer ${localStorage.getItem('token')}`,
                        'Content-Type': 'application/json'
                    }
                }
            );
            return response.data;
        } catch (error: any) {
            console.error('Generate visualization error:', error);
            throw new Error(error.response?.data?.error || 'Failed to generate visualization');
        }
    },

    async getVisualizationData(chatId: string, messageId: string, queryId: string, limit: number = 500, offset: number = 0): Promise<any> {
        try {
            const response = await axios.post(
                `${API_URL}/chats/${chatId}/visualization-data`,
                {
                    message_id: messageId,
                    query_id: queryId,
                    limit,
                    offset
                },
                {
                    withCredentials: true,
                    headers: {
                        'Authorization': `Bearer ${localStorage.getItem('token')}`,
                        'Content-Type': 'application/json'
                    }
                }
            );
            return response.data;
        } catch (error: any) {
            console.error('Get visualization data error:', error);
            throw new Error(error.response?.data?.error || 'Failed to load visualization data');
        }
    }
};

export default chatService; 