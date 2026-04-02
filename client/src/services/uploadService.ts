import { FileUpload } from '../types/chat';

const API_URL = import.meta.env.VITE_API_URL;

const uploadService = {
  async getTablePreviewData(
    chatId: string,
    tableName: string,
    page: number,
    pageSize: number
  ): Promise<{ rows: any[]; total_rows: number }> {
    const response = await fetch(
      `${API_URL}/upload/${chatId}/tables/${tableName}?page=${page}&pageSize=${pageSize}`,
      { headers: { Authorization: `Bearer ${localStorage.getItem('token')}` } }
    );
    if (!response.ok) throw new Error('Failed to load table preview data');
    const data = await response.json();
    const responseData = data.data || data;
    return { rows: responseData.rows || [], total_rows: responseData.total_rows };
  },

  async downloadTableData(
    chatId: string,
    tableName: string,
    format: string,
    rowIds?: string[]
  ): Promise<Blob> {
    let url = `${API_URL}/upload/${chatId}/tables/${tableName}/download?format=${format}`;
    if (rowIds && rowIds.length > 0) {
      url += `&rowIds=${rowIds.join(',')}`;
    }
    const response = await fetch(url, {
      headers: {
        Authorization: `Bearer ${localStorage.getItem('token')}`,
        Accept:
          format === 'xlsx'
            ? 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
            : 'text/csv',
      },
    });
    if (!response.ok) throw new Error('Download failed');
    return response.blob();
  },

  async deleteTableRow(
    chatId: string,
    tableName: string,
    rowId: string | number
  ): Promise<boolean> {
    const response = await fetch(
      `${API_URL}/upload/${chatId}/tables/${tableName}/rows/${rowId}`,
      {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
      }
    );
    return response.ok;
  },

  async deleteTable(chatId: string, tableName: string): Promise<boolean> {
    const response = await fetch(`${API_URL}/upload/${chatId}/tables/${tableName}`, {
      method: 'DELETE',
      headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
    });
    return response.ok;
  },

  async uploadFilesToChat(chatId: string, fileUploads: FileUpload[]): Promise<void> {
    for (const fileUpload of fileUploads) {
      if (!fileUpload.file) {
        console.error('No file object found for upload:', fileUpload.filename);
        continue;
      }

      const body = new FormData();
      body.append('file', fileUpload.file);
      body.append('tableName', fileUpload.tableName || '');
      body.append('mergeStrategy', fileUpload.mergeStrategy || 'replace');

      if (fileUpload.mergeOptions) {
        body.append('ignoreCase', String(fileUpload.mergeOptions.ignoreCase ?? true));
        body.append('trimWhitespace', String(fileUpload.mergeOptions.trimWhitespace ?? true));
        body.append('handleNulls', fileUpload.mergeOptions.handleNulls || 'empty');
        body.append('addNewColumns', String(fileUpload.mergeOptions.addNewColumns ?? true));
        body.append('dropMissingColumns', String(fileUpload.mergeOptions.dropMissingColumns ?? false));
        body.append('updateExisting', String(fileUpload.mergeOptions.updateExisting ?? true));
        body.append('insertNew', String(fileUpload.mergeOptions.insertNew ?? true));
        body.append('deleteMissing', String(fileUpload.mergeOptions.deleteMissing ?? false));
      }

      const response = await fetch(`${API_URL}/upload/${chatId}/file`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
        body,
      });

      if (!response.ok) {
        const errorData = await response.json();
        console.error('Upload error response:', errorData);
        throw new Error(
          errorData.error || `Failed to upload file: ${response.status} ${response.statusText}`
        );
      }
    }
  },
};

export default uploadService;
