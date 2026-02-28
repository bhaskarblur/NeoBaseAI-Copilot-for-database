import axios from './axiosConfig';
import { KnowledgeBaseResponse, UpdateKnowledgeBaseRequest } from '../types/knowledgeBase';

const API_URL = import.meta.env.VITE_API_URL;

interface KBApiResponse {
  success: boolean;
  data: KnowledgeBaseResponse;
  error?: string;
}

const knowledgeBaseService = {
  /**
   * Get the knowledge base for a chat.
   * Returns the existing KB or a default empty one.
   */
  async getKnowledgeBase(chatId: string): Promise<KnowledgeBaseResponse> {
    const response = await axios.get<KBApiResponse>(`${API_URL}/chats/${chatId}/knowledge-base`);
    if (!response.data.success) {
      throw new Error(response.data.error || 'Failed to fetch knowledge base');
    }
    return response.data.data;
  },

  /**
   * Update (create or replace) the knowledge base for a chat.
   * This triggers async vectorization on the backend.
   */
  async updateKnowledgeBase(chatId: string, request: UpdateKnowledgeBaseRequest): Promise<KnowledgeBaseResponse> {
    const response = await axios.put<KBApiResponse>(`${API_URL}/chats/${chatId}/knowledge-base`, request);
    if (!response.data.success) {
      throw new Error(response.data.error || 'Failed to update knowledge base');
    }
    return response.data.data;
  },
};

export default knowledgeBaseService;
