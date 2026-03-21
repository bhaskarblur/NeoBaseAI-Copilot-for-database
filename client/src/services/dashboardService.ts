import axios from 'axios';
import {
  Dashboard,
  DashboardListItem,
  CreateDashboardRequest,
  UpdateDashboardRequest,
  RegenerateDashboardRequest,
  AddWidgetRequest,
  EditWidgetRequest,
  CreateFromBlueprintsRequest,
  Widget,
  ValidateImportRequest,
  ValidateImportResponse,
  ImportDashboardRequest,
  ImportDashboardResponse,
} from '../types/dashboard';

const API_URL = import.meta.env.VITE_API_URL;

export const dashboardService = {
  // === Dashboard CRUD ===

  /**
   * Create a new dashboard via AI generation
   */
  createDashboard: async (
    chatId: string,
    request: CreateDashboardRequest
  ): Promise<Dashboard> => {
    const response = await axios.post(
      `${API_URL}/chats/${chatId}/dashboards`,
      request
    );
    return response.data.data;
  },

  /**
   * Get a dashboard with all its widgets
   */
  getDashboard: async (
    chatId: string,
    dashboardId: string
  ): Promise<Dashboard> => {
    const response = await axios.get(
      `${API_URL}/chats/${chatId}/dashboards/${dashboardId}`
    );
    return response.data.data;
  },

  /**
   * List all dashboards for a chat
   */
  listDashboards: async (
    chatId: string
  ): Promise<DashboardListItem[]> => {
    const response = await axios.get(
      `${API_URL}/chats/${chatId}/dashboards`
    );
    return response.data.data || [];
  },

  /**
   * Update dashboard metadata (name, refresh interval, layout, etc.)
   */
  updateDashboard: async (
    chatId: string,
    dashboardId: string,
    request: UpdateDashboardRequest
  ): Promise<Dashboard> => {
    const response = await axios.patch(
      `${API_URL}/chats/${chatId}/dashboards/${dashboardId}`,
      request
    );
    return response.data.data;
  },

  /**
   * Delete a dashboard and all its widgets
   */
  deleteDashboard: async (
    chatId: string,
    dashboardId: string
  ): Promise<void> => {
    await axios.delete(
      `${API_URL}/chats/${chatId}/dashboards/${dashboardId}`
    );
  },

  // === Widget Operations ===

  /**
   * Add a new widget via AI prompt
   */
  addWidget: async (
    chatId: string,
    dashboardId: string,
    request: AddWidgetRequest
  ): Promise<Widget> => {
    const response = await axios.post(
      `${API_URL}/chats/${chatId}/dashboards/${dashboardId}/widgets`,
      request
    );
    return response.data.data;
  },

  /**
   * Edit an existing widget via AI prompt
   */
  editWidget: async (
    chatId: string,
    dashboardId: string,
    widgetId: string,
    request: EditWidgetRequest
  ): Promise<Widget> => {
    const response = await axios.post(
      `${API_URL}/chats/${chatId}/dashboards/${dashboardId}/widgets/${widgetId}/edit`,
      request
    );
    return response.data.data;
  },

  /**
   * Delete a widget from a dashboard
   */
  deleteWidget: async (
    chatId: string,
    dashboardId: string,
    widgetId: string
  ): Promise<void> => {
    await axios.delete(
      `${API_URL}/chats/${chatId}/dashboards/${dashboardId}/widgets/${widgetId}`
    );
  },

  // === AI Operations ===

  /**
   * Trigger AI to generate dashboard blueprint suggestions
   * Results are delivered via SSE events
   */
  generateBlueprints: async (
    chatId: string,
    streamId: string,
    prompt?: string,
    signal?: AbortSignal
  ): Promise<void> => {
    await axios.post(
      `${API_URL}/chats/${chatId}/dashboards/suggest-templates?stream_id=${streamId}`,
      prompt ? { prompt } : undefined,
      signal ? { signal } : undefined
    );
  },

  /**
   * Create dashboards from selected blueprints
   * Results are delivered via SSE events
   */
  createFromBlueprints: async (
    chatId: string,
    streamId: string,
    request: CreateFromBlueprintsRequest,
    signal?: AbortSignal
  ): Promise<void> => {
    await axios.post(
      `${API_URL}/chats/${chatId}/dashboards/create-from-blueprints?stream_id=${streamId}`,
      request,
      signal ? { signal } : undefined
    );
  },

  /**
   * Regenerate an existing dashboard
   * Results are delivered via SSE events
   */
  regenerateDashboard: async (
    chatId: string,
    dashboardId: string,
    streamId: string,
    request: RegenerateDashboardRequest,
    signal?: AbortSignal
  ): Promise<void> => {
    await axios.post(
      `${API_URL}/chats/${chatId}/dashboards/${dashboardId}/regenerate?stream_id=${streamId}`,
      request,
      signal ? { signal } : undefined
    );
  },

  // === Data Refresh ===

  /**
   * Trigger a manual refresh of all dashboard widgets
   * Data is delivered via SSE events per widget
   */
  refreshDashboard: async (
    chatId: string,
    dashboardId: string,
    streamId: string
  ): Promise<void> => {
    await axios.post(
      `${API_URL}/chats/${chatId}/dashboards/${dashboardId}/refresh?stream_id=${streamId}`
    );
  },

  /**
   * Trigger a refresh of a single widget
   * Data is delivered via SSE events
   * @param cursor - Optional cursor value for pagination
   */
  refreshWidget: async (
    chatId: string,
    dashboardId: string,
    widgetId: string,
    streamId: string,
    cursor?: string
  ): Promise<void> => {
    const params = new URLSearchParams({ stream_id: streamId });
    if (cursor) {
      params.append('cursor', cursor);
    }
    await axios.post(
      `${API_URL}/chats/${chatId}/dashboards/${dashboardId}/widgets/${widgetId}/refresh?${params.toString()}`
    );
  },

  // === Import/Export ===

  /**
   * Export a dashboard to portable JSON format
   * Returns JSON that can be downloaded and imported elsewhere
   */
  exportDashboard: async (
    chatId: string,
    dashboardId: string
  ): Promise<Blob> => {
    const response = await axios.get(
      `${API_URL}/chats/${chatId}/dashboards/${dashboardId}/export`,
      { responseType: 'blob' }
    );
    return response.data;
  },

  /**
   * Validate import JSON and get connection mapping suggestions
   */
  validateImport: async (
    chatId: string,
    request: ValidateImportRequest
  ): Promise<ValidateImportResponse> => {
    const response = await axios.post(
      `${API_URL}/chats/${chatId}/dashboards/import/validate`,
      request
    );
    return response.data.data;
  },

  /**
   * Import a dashboard from export JSON
   */
  importDashboard: async (
    chatId: string,
    request: ImportDashboardRequest
  ): Promise<ImportDashboardResponse> => {
    const response = await axios.post(
      `${API_URL}/chats/${chatId}/dashboards/import`,
      request
    );
    return response.data.data;
  },
};
