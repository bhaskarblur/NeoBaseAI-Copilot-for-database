import { FileUpload } from '../types/chat';
import uploadService from '../services/uploadService';

/**
 * Uploads an array of files to a given chat.
 * Throws on the first failed upload.
 */
export const uploadFilesToChat = async (chatId: string, fileUploads: FileUpload[]): Promise<void> => {
  return uploadService.uploadFilesToChat(chatId, fileUploads);
};
