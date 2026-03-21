import { Loader2, Trash2 } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import toast from 'react-hot-toast';
import { dashboardService } from '../../services/dashboardService';
import ConfirmationModal from '../modals/ConfirmationModal';
import {
  Dashboard,
  DashboardBlueprint,
  DashboardBlueprintEvent,
  DashboardGenerationCompleteEvent as DashboardGenCompleteEvent,
  DashboardGenerationProgressEvent,
  DashboardListItem,
  DashboardWidgetDataEvent,
  DASHBOARD_SSE_EVENTS,
} from '../../types/dashboard';
import DashboardEmptyState from './DashboardEmptyState';
import DashboardHeader from './DashboardHeader';
import {
  AddWidgetModal,
  BlueprintPickerModal,
  CreateDashboardPromptModal,
  EditWidgetModal,
  ImportDashboardModal,
  RegenerateDashboardModal,
} from './DashboardModals';
import DashboardProgressOverlay from './DashboardProgressOverlay';
import DashboardWidgetGrid from './DashboardWidgetGrid';

// Module-level cache to prevent re-fetching on tab toggle
const _listCache = new Map<string, { list: DashboardListItem[]; activeDashId?: string }>();
const _dashCache = new Map<string, Dashboard>();

interface DashboardViewProps {
  chatId: string;
  streamId: string | null;
  isConnected: boolean;
  onReconnect?: () => Promise<void>;
}

export default function DashboardView({
  chatId,
  streamId,
  isConnected,
  onReconnect,
}: Readonly<DashboardViewProps>) {
  // Dashboard list state
  const [dashboardList, setDashboardList] = useState<DashboardListItem[]>([]);
  const [isLoadingList, setIsLoadingList] = useState(true);
  const [activeDashboard, setActiveDashboard] = useState<Dashboard | null>(null);
  const [isLoadingDashboard, setIsLoadingDashboard] = useState(false);

  // UI state
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [individuallyRefreshingWidgets, setIndividuallyRefreshingWidgets] = useState<Set<string>>(new Set());

  // Blueprint / generation state
  const [blueprints, setBlueprints] = useState<DashboardBlueprint[]>([]);
  const [showBlueprintPicker, setShowBlueprintPicker] = useState(false);
  const [selectedBlueprintIndices, setSelectedBlueprintIndices] = useState<Set<number>>(new Set());
  const [generationProgress, setGenerationProgress] = useState<DashboardGenerationProgressEvent | null>(null);
  const [, setIsGeneratingBlueprints] = useState(false);
  const [isCreatingFromBlueprints, setIsCreatingFromBlueprints] = useState(false);
  const [generationContext, setGenerationContext] = useState<'blueprints' | 'creating' | 'custom'>('blueprints');
  const cancelledRef = useRef(false);
  const hasFetchedOnceRef = useRef<string | null>(null);

  // Modal visibility state
  const [showPromptModal, setShowPromptModal] = useState(false);
  const [showAddWidgetModal, setShowAddWidgetModal] = useState(false);
  const [isAddingWidget, setIsAddingWidget] = useState(false);
  const [showRegenerateModal, setShowRegenerateModal] = useState(false);
  const [showDeleteDashboardConfirm, setShowDeleteDashboardConfirm] = useState(false);

  // Import/Export state
  const [showImportModal, setShowImportModal] = useState(false);
  const [importJsonContent, setImportJsonContent] = useState<string>('');
  const [isImporting, setIsImporting] = useState(false);

  // Edit widget state
  const [editingWidgetId, setEditingWidgetId] = useState<string | null>(null);
  const [isEditingWidget, setIsEditingWidget] = useState(false);

  // Refs
  const refreshIntervalRef = useRef<NodeJS.Timeout | null>(null);
  const abortControllerRef = useRef<AbortController | null>(null);

  // Refs for latest prop values (needed for async operations after reconnect)
  const streamIdRef = useRef(streamId);
  useEffect(() => { streamIdRef.current = streamId; }, [streamId]);
  const isConnectedRef = useRef(isConnected);
  useEffect(() => { isConnectedRef.current = isConnected; }, [isConnected]);

  // Widget loading timeout tracking
  const widgetLoadingTimeoutsRef = useRef<Map<string, NodeJS.Timeout>>(new Map());
  const WIDGET_LOADING_TIMEOUT_MS = 30_000; // 30 seconds

  const clearLoadingTimeout = useCallback((widgetId: string) => {
    const existing = widgetLoadingTimeoutsRef.current.get(widgetId);
    if (existing) {
      clearTimeout(existing);
      widgetLoadingTimeoutsRef.current.delete(widgetId);
    }
  }, []);

  const clearAllLoadingTimeouts = useCallback(() => {
    widgetLoadingTimeoutsRef.current.forEach((t) => clearTimeout(t));
    widgetLoadingTimeoutsRef.current.clear();
  }, []);

  const startLoadingTimeout = useCallback((widgetId: string) => {
    clearLoadingTimeout(widgetId);
    const timeout = setTimeout(() => {
      widgetLoadingTimeoutsRef.current.delete(widgetId);
      setActiveDashboard((prev) => {
        if (!prev) return null;
        return {
          ...prev,
          widgets: prev.widgets.map((w) =>
            w.id === widgetId && w.is_loading
              ? { ...w, is_loading: false, error: 'Loading timed out. Try refreshing.' }
              : w
          ),
        };
      });
    }, WIDGET_LOADING_TIMEOUT_MS);
    widgetLoadingTimeoutsRef.current.set(widgetId, timeout);
  }, [clearLoadingTimeout]);

  // Cleanup timeouts on unmount
  useEffect(() => () => clearAllLoadingTimeouts(), [clearAllLoadingTimeouts]);

  // Try to ensure SSE connection, returns true if connected
  const ensureSSEConnection = useCallback(async (silent = false): Promise<boolean> => {
    if (streamIdRef.current && isConnectedRef.current) return true;
    if (!onReconnect) {
      if (!silent) toast.error('Connection not active. Please reconnect first.', { icon: '🔌' });
      return false;
    }
    try {
      if (!silent) toast.loading('Reconnecting...', { id: 'sse-ensure', duration: 8000 });
      await onReconnect();
      await new Promise((r) => setTimeout(r, 150)); // let React update props
      if (!silent) toast.success('Reconnected!', { id: 'sse-ensure' });
      return true;
    } catch {
      if (!silent) toast.error('Failed to reconnect.', { id: 'sse-ensure' });
      return false;
    }
  }, [onReconnect]);

  // Cancel handlers
  const handleCancelDashboardRefresh = useCallback(() => {
    clearAllLoadingTimeouts();
    setIsRefreshing(false);
    setActiveDashboard((prev) => {
      if (!prev) return null;
      return {
        ...prev,
        widgets: prev.widgets.map((w) => w.is_loading ? { ...w, is_loading: false } : w),
      };
    });
  }, [clearAllLoadingTimeouts]);

  const handleCancelWidgetRefresh = useCallback((widgetId: string) => {
    clearLoadingTimeout(widgetId);
    setIndividuallyRefreshingWidgets((prev) => {
      const next = new Set(prev);
      next.delete(widgetId);
      return next;
    });
    setActiveDashboard((prev) => {
      if (!prev) return null;
      return {
        ...prev,
        widgets: prev.widgets.map((w) =>
          w.id === widgetId && w.is_loading ? { ...w, is_loading: false } : w
        ),
      };
    });
  }, [clearLoadingTimeout]);

  // =============================================
  // Data Loading
  // =============================================

  useEffect(() => {
    // Reset all state when chatId changes
    setActiveDashboard(null);
    setDashboardList([]);
    setBlueprints([]);
    setShowBlueprintPicker(false);
    setGenerationProgress(null);
    setIsGeneratingBlueprints(false);
    setIsCreatingFromBlueprints(false);
    setShowPromptModal(false);
    setShowAddWidgetModal(false);
    setShowRegenerateModal(false);
    setShowDeleteDashboardConfirm(false);
    setEditingWidgetId(null);
    cancelledRef.current = false;
    hasFetchedOnceRef.current = null;

    const cached = _listCache.get(chatId);
    if (cached) {
      setDashboardList(cached.list);
      setIsLoadingList(false);
      if (cached.activeDashId) {
        const cachedDash = _dashCache.get(cached.activeDashId);
        if (cachedDash) {
          setActiveDashboard(cachedDash);
        } else {
          loadDashboard(cached.activeDashId);
        }
      }
      return;
    }
    loadDashboardList();
  }, [chatId]);

  // Auto-refresh timer
  useEffect(() => {
    if (refreshIntervalRef.current) {
      clearInterval(refreshIntervalRef.current);
      refreshIntervalRef.current = null;
    }

    if (activeDashboard && activeDashboard.refresh_interval > 0 && isConnected) {
      refreshIntervalRef.current = setInterval(() => {
        handleRefreshDashboard(true);
      }, activeDashboard.refresh_interval * 1000);
    }

    return () => {
      if (refreshIntervalRef.current) {
        clearInterval(refreshIntervalRef.current);
      }
    };
  }, [activeDashboard?.id, activeDashboard?.refresh_interval, isConnected]);

  // =============================================
  // SSE Event Listeners
  // =============================================

  useEffect(() => {
    const handleBlueprints = (e: Event) => {
      if (cancelledRef.current) return;
      const data = (e as CustomEvent).detail as DashboardBlueprintEvent;
      setBlueprints(data.blueprints);
      setSelectedBlueprintIndices(new Set(data.blueprints.map((b) => b.index)));
      setShowBlueprintPicker(true);
      setIsGeneratingBlueprints(false);
      setGenerationProgress(null);
    };

    const handleProgress = (e: Event) => {
      if (cancelledRef.current) return;
      const data = (e as CustomEvent).detail as DashboardGenerationProgressEvent;
      setGenerationProgress(data);
    };

    const handleComplete = (e: Event) => {
      const data = (e as CustomEvent).detail as DashboardGenCompleteEvent;
      setGenerationProgress(null);
      setIsCreatingFromBlueprints(false);

      setDashboardList((prev) => {
        const exists = prev.some((d) => d.id === data.dashboard_id);
        if (exists) return prev;
        const newList = [
          ...prev,
          {
            id: data.dashboard.id,
            name: data.dashboard.name,
            description: data.dashboard.description ?? '',
            template_type: data.dashboard.template_type ?? '',
            is_default: data.dashboard.is_default,
            widget_count: data.dashboard.widgets?.length ?? 0,
            created_at: data.dashboard.created_at,
            updated_at: data.dashboard.updated_at,
          },
        ];
        _listCache.set(chatId, { list: newList, activeDashId: data.dashboard.id });
        return newList;
      });

      setActiveDashboard(data.dashboard);
      _dashCache.set(data.dashboard.id, data.dashboard);
      toast.success(`Dashboard "${data.dashboard.name}" created!`);
    };

    const handleWidgetData = (e: Event) => {
      const data = (e as CustomEvent).detail as DashboardWidgetDataEvent;
      handleWidgetDataUpdate(data.widget_id, data);
    };

    const handleWidgetError = (e: Event) => {
      const data = (e as CustomEvent).detail as DashboardWidgetDataEvent;
      if (data.error) {
        handleWidgetErrorUpdate(data.widget_id, data.error);
      }
    };

    globalThis.addEventListener(DASHBOARD_SSE_EVENTS.BLUEPRINTS, handleBlueprints);
    globalThis.addEventListener(DASHBOARD_SSE_EVENTS.GENERATION_PROGRESS, handleProgress);
    globalThis.addEventListener(DASHBOARD_SSE_EVENTS.GENERATION_COMPLETE, handleComplete);
    globalThis.addEventListener(DASHBOARD_SSE_EVENTS.WIDGET_DATA, handleWidgetData);
    globalThis.addEventListener(DASHBOARD_SSE_EVENTS.WIDGET_ERROR, handleWidgetError);

    return () => {
      globalThis.removeEventListener(DASHBOARD_SSE_EVENTS.BLUEPRINTS, handleBlueprints);
      globalThis.removeEventListener(DASHBOARD_SSE_EVENTS.GENERATION_PROGRESS, handleProgress);
      globalThis.removeEventListener(DASHBOARD_SSE_EVENTS.GENERATION_COMPLETE, handleComplete);
      globalThis.removeEventListener(DASHBOARD_SSE_EVENTS.WIDGET_DATA, handleWidgetData);
      globalThis.removeEventListener(DASHBOARD_SSE_EVENTS.WIDGET_ERROR, handleWidgetError);
    };
  }, []);

  // =============================================
  // Handlers
  // =============================================

  const loadDashboardList = async () => {
    try {
      setIsLoadingList(true);
      const list = await dashboardService.listDashboards(chatId);
      setDashboardList(list);

      if (list.length > 0) {
        const defaultDash = list.find((d: DashboardListItem) => d.is_default) || list[0];
        await loadDashboard(defaultDash.id);
        _listCache.set(chatId, { list, activeDashId: defaultDash.id });
      } else {
        _listCache.set(chatId, { list, activeDashId: undefined });
      }
    } catch (err) {
      console.error('Failed to load dashboards:', err);
    } finally {
      setIsLoadingList(false);
    }
  };

  const loadDashboard = async (dashboardId: string) => {
    try {
      setIsLoadingDashboard(true);
      const dashboard = await dashboardService.getDashboard(chatId, dashboardId);
      setActiveDashboard(dashboard);
      _dashCache.set(dashboardId, dashboard);
      const cached = _listCache.get(chatId);
      if (cached) _listCache.set(chatId, { ...cached, activeDashId: dashboardId });

      // Reset fetch flag when switching dashboards
      hasFetchedOnceRef.current = null;

      // Immediately trigger data fetch for widgets that have no data
      const hasUnfetched = dashboard.widgets.some((w) => !w.data && !w.is_loading && !w.error);
      if (hasUnfetched && streamId && isConnected) {
        hasFetchedOnceRef.current = dashboardId;
        // Set unfetched widgets to loading state with timeouts
        const unfetchedIds = dashboard.widgets.filter((w) => !w.data && !w.is_loading && !w.error).map((w) => w.id);
        setActiveDashboard((prev) => {
          if (!prev) return null;
          return {
            ...prev,
            widgets: prev.widgets.map((w) =>
              unfetchedIds.includes(w.id) ? { ...w, is_loading: true } : w
            ),
          };
        });
        unfetchedIds.forEach((id) => startLoadingTimeout(id));
        dashboardService.refreshDashboard(chatId, dashboardId, streamId).catch(() => {});
      }
    } catch (err) {
      console.error('Failed to load dashboard:', err);
      toast.error('Failed to load dashboard');
    } finally {
      setIsLoadingDashboard(false);
    }
  };

  const handleRefreshDashboard = useCallback(async (silent = false, retryCount = 0) => {
    if (!activeDashboard) return;

    // Ensure SSE connection before refresh
    const connected = await ensureSSEConnection(silent);
    if (!connected) return;
    const sid = streamIdRef.current;
    if (!sid) return;

    // Set all widgets to loading state with timeouts
    setActiveDashboard((prev) => {
      if (!prev) return null;
      return {
        ...prev,
        widgets: prev.widgets.map((w) => ({ ...w, is_loading: true, error: undefined })),
      };
    });
    activeDashboard.widgets.forEach((w) => startLoadingTimeout(w.id));

    setIsRefreshing(true);
    try {
      await dashboardService.refreshDashboard(chatId, activeDashboard.id, sid);
      if (!silent) toast.success('Dashboard refreshing...', { icon: '✅' });
    } catch (err: any) {
      console.error('Failed to refresh dashboard:', err);
      
      // Check if error is about SSE not found or database connection
      const errorMsg = err?.response?.data?.error || err?.message || '';
      const isSSENotFound = errorMsg.includes('SSE stream not found');
      const isDBNotConnected = errorMsg.includes('database connection not found') || errorMsg.includes('connection not found');
      
      // If connection issues detected and we can reconnect
      if ((isSSENotFound || isDBNotConnected) && onReconnect && retryCount < 3) {
        clearAllLoadingTimeouts();
        setIsRefreshing(false);
        
        if (!silent) toast.loading('Establishing connections...', { id: 'reconnect-refresh', duration: 8000 });
        
        try {
          await onReconnect();
          // Wait for connections to fully establish
          await new Promise((r) => setTimeout(r, 300));
          if (!silent) toast.success('Connected! Refreshing...', { id: 'reconnect-refresh' });
          // Retry the refresh with incremented retry count
          handleRefreshDashboard(silent, retryCount + 1);
          return;
        } catch (reconnectErr) {
          console.error('Failed to reconnect:', reconnectErr);
          if (!silent) toast.error('Connection failed. Please try manually.', { id: 'reconnect-refresh' });
          // If reconnect failed, don't retry further
        }
      }
      
      // If we've exhausted retries or can't reconnect
      if (!silent && (isSSENotFound || isDBNotConnected)) {
        toast.error('Unable to connect. Please check your connection and try again.');
      } else if (!silent) {
        toast.error('Failed to refresh dashboard');
      }
      
      clearAllLoadingTimeouts();
      setActiveDashboard((prev) => {
        if (!prev) return null;
        return {
          ...prev,
          widgets: prev.widgets.map((w) => w.is_loading ? { ...w, is_loading: false, error: 'Failed to refresh' } : w),
        };
      });
    } finally {
      setIsRefreshing(false);
    }
  }, [activeDashboard, chatId, ensureSSEConnection, startLoadingTimeout, clearAllLoadingTimeouts, onReconnect]);

  // One-time auto-fetch: when a dashboard loads with unfetched widgets, trigger a silent refresh once
  useEffect(() => {
    if (!activeDashboard || !streamId || !isConnected) return;
    const dashId = activeDashboard.id;
    // Already fetched once for this dashboard
    if (hasFetchedOnceRef.current === dashId) return;
    // Check if any widget has no data and isn't loading
    const hasUnfetchedWidgets = activeDashboard.widgets.some(
      (w) => !w.data && !w.is_loading && !w.error
    );
    if (!hasUnfetchedWidgets) return;
    hasFetchedOnceRef.current = dashId;
    handleRefreshDashboard(true);
  }, [activeDashboard?.id, activeDashboard?.widgets, streamId, isConnected, handleRefreshDashboard]);

  const handleRefreshWidget = useCallback(async (widgetId: string, retryCount = 0, cursor?: string) => {
    if (!activeDashboard) return;

    // Ensure SSE connection before refresh
    const connected = await ensureSSEConnection();
    if (!connected) return;
    const sid = streamIdRef.current;
    if (!sid) return;

    // Mark widget as individually refreshing
    setIndividuallyRefreshingWidgets((prev) => new Set(prev).add(widgetId));

    // Set widget to loading state with timeout
    setActiveDashboard((prev) => {
      if (!prev) return null;
      return {
        ...prev,
        widgets: prev.widgets.map((w) =>
          w.id === widgetId ? { ...w, is_loading: true, error: undefined } : w
        ),
      };
    });
    startLoadingTimeout(widgetId);

    try {
      await dashboardService.refreshWidget(chatId, activeDashboard.id, widgetId, sid, cursor);
    } catch (err: any) {
      console.error('Failed to refresh widget:', err);
      
      // Check if error is about SSE not found or database connection
      const errorMsg = err?.response?.data?.error || err?.message || '';
      const isSSENotFound = errorMsg.includes('SSE stream not found');
      const isDBNotConnected = errorMsg.includes('database connection not found') || errorMsg.includes('connection not found');
      
      // If connection issues detected and we can reconnect
      if ((isSSENotFound || isDBNotConnected) && onReconnect && retryCount < 3) {
        clearLoadingTimeout(widgetId);
        setIndividuallyRefreshingWidgets((prev) => {
          const next = new Set(prev);
          next.delete(widgetId);
          return next;
        });
        
        toast.loading('Establishing connections...', { id: 'reconnect-widget', duration: 8000 });
        
        try {
          await onReconnect();
          // Wait for connections to fully establish
          await new Promise((r) => setTimeout(r, 300));
          toast.success('Connected! Refreshing widget...', { id: 'reconnect-widget' });
          // Retry the widget refresh with incremented retry count
          handleRefreshWidget(widgetId, retryCount + 1, cursor);
          return;
        } catch (reconnectErr) {
          console.error('Failed to reconnect:', reconnectErr);
          toast.error('Connection failed. Please try manually.', { id: 'reconnect-widget' });
          // If reconnect failed, don't retry further
        }
      }
      
      clearLoadingTimeout(widgetId);
      setIndividuallyRefreshingWidgets((prev) => {
        const next = new Set(prev);
        next.delete(widgetId);
        return next;
      });
      setActiveDashboard((prev) => {
        if (!prev) return null;
        return {
          ...prev,
          widgets: prev.widgets.map((w) =>
            w.id === widgetId ? { ...w, is_loading: false, error: 'Failed to refresh widget data' } : w
          ),
        };
      });
      toast.error('Failed to refresh widget');
    }
  }, [activeDashboard, chatId, ensureSSEConnection, startLoadingTimeout, clearLoadingTimeout, onReconnect]);

  // Handle widget pagination - Next Page
  const handleWidgetNextPage = useCallback((widgetId: string) => {
    if (!activeDashboard) return;
    
    const widget = activeDashboard.widgets.find(w => w.id === widgetId);
    if (!widget || !widget.table_config?.cursor_field || !widget.has_more || widget.is_loading) return;
    
    const cursorToUse = widget.next_cursor ?? null;
    
    // Refresh with the next cursor
    handleRefreshWidget(widgetId, 0, cursorToUse || undefined);
    
    // Push the cursor we're navigating to onto the stack and increment page
    // cursor_stack[0] = null (page 1, no cursor), cursor_stack[1] = cursor for page 2, etc.
    setActiveDashboard((prev) => {
      if (!prev) return null;
      return {
        ...prev,
        widgets: prev.widgets.map((w) =>
          w.id === widgetId ? {
            ...w,
            current_page: (w.current_page || 1) + 1,
            cursor_stack: [...(w.cursor_stack ?? [null]), cursorToUse],
          } : w
        ),
      };
    });
  }, [activeDashboard, handleRefreshWidget]);

  // Handle widget pagination - Previous Page (uses cursor stack for backward navigation)
  const handleWidgetPreviousPage = useCallback((widgetId: string) => {
    if (!activeDashboard) return;
    
    const widget = activeDashboard.widgets.find(w => w.id === widgetId);
    if (!widget || !widget.table_config?.cursor_field || widget.is_loading) return;
    
    // cursor_stack stores the cursor used to load each page.
    // To go back, pop the last entry and reload using the new top-of-stack cursor.
    const currentStack = widget.cursor_stack ?? [null];
    if (currentStack.length <= 1) return; // Already on first page
    
    const newStack = currentStack.slice(0, -1);
    const prevCursor = newStack[newStack.length - 1]; // null = first page (no cursor)
    
    // Update cursor stack and page number before triggering refresh
    setActiveDashboard((prev) => {
      if (!prev) return null;
      return {
        ...prev,
        widgets: prev.widgets.map((w) =>
          w.id === widgetId ? {
            ...w,
            current_page: Math.max(1, (w.current_page || 1) - 1),
            cursor_stack: newStack,
          } : w
        ),
      };
    });
    
    // Reload the page with the previous cursor (null = first page)
    handleRefreshWidget(widgetId, 0, prevCursor ?? undefined);
  }, [activeDashboard, handleRefreshWidget]);

  const handleRefreshIntervalChange = async (interval: number) => {
    if (!activeDashboard) return;
    try {
      await dashboardService.updateDashboard(chatId, activeDashboard.id, {
        refresh_interval: interval,
      });
      setActiveDashboard((prev) => prev ? { ...prev, refresh_interval: interval } : null);
    } catch {
      toast.error('Failed to update refresh interval');
    }
  };

  const handleDeleteDashboard = async () => {
    if (!activeDashboard) return;
    try {
      await dashboardService.deleteDashboard(chatId, activeDashboard.id);
      const deletedId = activeDashboard.id;
      setActiveDashboard(null);
      _dashCache.delete(deletedId);
      setDashboardList((prev) => {
        const newList = prev.filter((d) => d.id !== deletedId);
        _listCache.set(chatId, { list: newList, activeDashId: newList[0]?.id });
        return newList;
      });
      setShowDeleteDashboardConfirm(false);
      toast.success('Dashboard deleted', { icon: '🗑️' });

      const remaining = dashboardList.filter((d) => d.id !== deletedId);
      if (remaining.length > 0) {
        await loadDashboard(remaining[0].id);
      }
    } catch {
      toast.error('Failed to delete dashboard');
    }
  };

  const handleDeleteWidget = async (widgetId: string) => {
    if (!activeDashboard) return;
    try {
      await dashboardService.deleteWidget(chatId, activeDashboard.id, widgetId);
      setActiveDashboard((prev) => {
        if (!prev) return null;
        return {
          ...prev,
          widgets: prev.widgets.filter((w) => w.id !== widgetId),
          layout: prev.layout.filter((l) => l.widget_id !== widgetId),
        };
      });
      toast.success('Widget removed', { icon: <Trash2 size={18} /> });
    } catch {
      toast.error('Failed to delete widget');
    }
  };

  const handleEditWidget = async (widgetId: string, prompt: string) => {
    if (!activeDashboard) return;
    setIsEditingWidget(true);
    try {
      const updatedWidget = await dashboardService.editWidget(
        chatId,
        activeDashboard.id,
        widgetId,
        { prompt }
      );
      // Set widget with loading state so it shows the loading bar while data fetches
      setActiveDashboard((prev) => {
        if (!prev) return null;
        return {
          ...prev,
          widgets: prev.widgets.map((w) =>
            w.id === widgetId ? { ...updatedWidget, data: undefined, is_loading: true, error: undefined } : w
          ),
        };
      });
      setEditingWidgetId(null);
      toast.success('Widget updated!');

      // Trigger data fetch for the updated widget via SSE
      startLoadingTimeout(widgetId);
      if (streamId && isConnected) {
        dashboardService.refreshWidget(chatId, activeDashboard.id, widgetId, streamId).catch((err) => {
          console.error('Failed to refresh updated widget:', err);
          clearLoadingTimeout(widgetId);
          setActiveDashboard((prev) => {
            if (!prev) return null;
            return {
              ...prev,
              widgets: prev.widgets.map((w) =>
                w.id === widgetId ? { ...w, is_loading: false, error: 'Failed to load data for updated widget' } : w
              ),
            };
          });
        });
      }
    } catch {
      toast.error('Failed to update widget');
    } finally {
      setIsEditingWidget(false);
    }
  };

  const handleWidgetDataUpdate = useCallback((widgetId: string, event: DashboardWidgetDataEvent) => {
    clearLoadingTimeout(widgetId);
    setIndividuallyRefreshingWidgets((prev) => {
      const next = new Set(prev);
      next.delete(widgetId);
      return next;
    });
    setActiveDashboard((prev) => {
      if (!prev) return null;
      return {
        ...prev,
        widgets: prev.widgets.map((w) => {
          if (w.id !== widgetId) return w;
          return {
            ...w,
            data: event.data ?? undefined,
            is_loading: false,
            error: undefined,
            last_refreshed_at: new Date().toISOString(),
            // Persist cursor pagination state from SSE event
            next_cursor: event.next_cursor ?? null,
            has_more: event.has_more,
          };
        }),
      };
    });
  }, [clearLoadingTimeout]);

  // Track if we're already attempting a reconnect to avoid loops
  const isReconnectingRef = useRef(false);

  const handleWidgetErrorUpdate = useCallback((widgetId: string, error: string) => {
    clearLoadingTimeout(widgetId);
    setIndividuallyRefreshingWidgets((prev) => {
      const next = new Set(prev);
      next.delete(widgetId);
      return next;
    });
    // Detect "no connection found" error and auto-reconnect
    if (error.toLowerCase().includes('no connection found') && onReconnect && !isReconnectingRef.current) {
      isReconnectingRef.current = true;
      console.log('[Dashboard] Connection lost — auto-reconnecting...');
      toast.loading('Reconnecting to database...', { id: 'dashboard-reconnect', duration: 5000 });

      onReconnect().then(() => {
        toast.success('Reconnected! Refreshing dashboard...', { id: 'dashboard-reconnect' });
        // After reconnecting, retry the dashboard refresh
        if (activeDashboard && streamId) {
          dashboardService.refreshDashboard(chatId, activeDashboard.id, streamId).catch(() => {});
        }
      }).catch(() => {
        toast.error('Failed to reconnect. Please try manually.', { id: 'dashboard-reconnect' });
        // Still show the error on widgets
        setActiveDashboard((prev) => {
          if (!prev) return null;
          return {
            ...prev,
            widgets: prev.widgets.map((w) =>
              w.id === widgetId ? { ...w, is_loading: false, error } : w
            ),
          };
        });
      }).finally(() => {
        isReconnectingRef.current = false;
      });
      return;
    }

    setActiveDashboard((prev) => {
      if (!prev) return null;
      return {
        ...prev,
        widgets: prev.widgets.map((w) =>
          w.id === widgetId ? { ...w, is_loading: false, error } : w
        ),
      };
    });
  }, [onReconnect, activeDashboard, chatId, streamId, clearLoadingTimeout]);

  const ensureConnection = (): boolean => {
    if (!streamId || !isConnected) {
      toast.error('Connection is not active. Please reconnect first.');
      return false;
    }
    return true;
  };

  const handleExploreSuggestions = async () => {
    if (!ensureConnection()) return;
    cancelledRef.current = false;
    setIsGeneratingBlueprints(true);
    setGenerationContext('blueprints');
    setGenerationProgress({ status: 'generating', message: 'Analyzing your database...', progress: 5 });
    const controller = new AbortController();
    abortControllerRef.current = controller;
    try {
      await dashboardService.generateBlueprints(chatId, streamId!, undefined, controller.signal);
    } catch (err) {
      if (controller.signal.aborted) return;
      toast.error('Failed to generate suggestions');
      setIsGeneratingBlueprints(false);
      setGenerationProgress(null);
    }
  };

  const handleCreateFromBlueprints = async () => {
    if (!streamId || !isConnected || selectedBlueprintIndices.size === 0) return;

    setIsCreatingFromBlueprints(true);
    setShowBlueprintPicker(false);
    setGenerationContext('creating');
    setGenerationProgress({ status: 'generating', message: 'Building your dashboards...', progress: 10 });

    const controller = new AbortController();
    abortControllerRef.current = controller;
    try {
      await dashboardService.createFromBlueprints(chatId, streamId, {
        blueprint_indices: Array.from(selectedBlueprintIndices),
      }, controller.signal);
    } catch (err) {
      if (controller.signal.aborted) return;
      toast.error('Failed to create dashboards');
      setIsCreatingFromBlueprints(false);
      setGenerationProgress(null);
    }
  };

  const handleCreateWithAI = () => {
    if (!ensureConnection()) return;
    setShowPromptModal(true);
  };

  const handleSubmitAIPrompt = async (prompt: string) => {
    if (!streamId || !isConnected) return;

    setShowPromptModal(false);
    cancelledRef.current = false;
    setIsGeneratingBlueprints(true);
    setGenerationContext('custom');
    setGenerationProgress({ status: 'generating', message: 'Preparing your custom dashboard...', progress: 5 });
    const controller = new AbortController();
    abortControllerRef.current = controller;
    try {
      await dashboardService.generateBlueprints(chatId, streamId, prompt, controller.signal);
    } catch {
      if (controller.signal.aborted) return;
      toast.error('Failed to generate dashboard');
      setIsGeneratingBlueprints(false);
      setGenerationProgress(null);
    }
  };

  const handleAddWidget = async (prompt: string) => {
    if (!activeDashboard) return;
    setIsAddingWidget(true);
    try {
      const widget = await dashboardService.addWidget(chatId, activeDashboard.id, { prompt });
      // Add widget with loading state so it shows loading bar while data fetches
      setActiveDashboard((prev) => {
        if (!prev) return null;
        return { ...prev, widgets: [...prev.widgets, { ...widget, data: undefined, is_loading: true, error: undefined }] };
      });
      setShowAddWidgetModal(false);
      toast.success(`Widget "${widget.title}" added!`);

      // Trigger data fetch for the new widget via SSE
      startLoadingTimeout(widget.id);
      if (streamId && isConnected) {
        dashboardService.refreshWidget(chatId, activeDashboard.id, widget.id, streamId).catch((err) => {
          console.error('Failed to fetch data for new widget:', err);
          clearLoadingTimeout(widget.id);
          setActiveDashboard((prev) => {
            if (!prev) return null;
            return {
              ...prev,
              widgets: prev.widgets.map((w) =>
                w.id === widget.id ? { ...w, is_loading: false, error: 'Failed to load data for new widget' } : w
              ),
            };
          });
        });
      }
    } catch {
      toast.error('Failed to add widget');
    } finally {
      setIsAddingWidget(false);
    }
  };

  const handleRegenerateDashboard = async (reason: 'try_another_variant' | 'schema_changed', customInstructions?: string) => {
    if (!activeDashboard) return;
    
    if (!streamId || !isConnected) {
      toast.error('Connection not active. Please reconnect to regenerate dashboard.', {
        icon: '🔌',
        duration: 4000,
      });
      return;
    }
    
    setShowRegenerateModal(false);
    cancelledRef.current = false;
    setGenerationContext('creating');
    setGenerationProgress({ status: 'generating', message: 'Regenerating dashboard...', progress: 5 });
    
    const controller = new AbortController();
    abortControllerRef.current = controller;
    
    try {
      await dashboardService.regenerateDashboard(chatId, activeDashboard.id, streamId, {
        reason,
        custom_instructions: customInstructions,
      }, controller.signal);
    } catch {
      if (controller.signal.aborted) return;
      toast.error('Failed to regenerate dashboard');
      setGenerationProgress(null);
    }
  };

  const handleCancelGeneration = () => {
    cancelledRef.current = true;
    abortControllerRef.current?.abort();
    abortControllerRef.current = null;
    setGenerationProgress(null);
    setIsGeneratingBlueprints(false);
    setIsCreatingFromBlueprints(false);
    setBlueprints([]);
    setShowBlueprintPicker(false);
    toast.error('Generation cancelled', { icon: '✕' });
  };

  const handleNewDashboard = () => {
    if (!ensureConnection()) return;
    setShowPromptModal(true);
  };

  // Import/Export handlers
  const handleExportDashboard = async () => {
    if (!activeDashboard) return;
    
    try {
      const blob = await dashboardService.exportDashboard(chatId, activeDashboard.id);
      const url = globalThis.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `${activeDashboard.name.replaceAll(/\s+/g, '-').toLowerCase()}-export.json`;
      document.body.appendChild(a);
      a.click();
      globalThis.URL.revokeObjectURL(url);
      a.remove();
      toast.success('Dashboard exported successfully', { icon: '📥' });
    } catch (error) {
      console.error('Export failed:', error);
      toast.error('Failed to export dashboard');
    }
  };

  const handleImportDashboard = () => {
    console.log('Import dashboard clicked');
    setShowImportModal(true);
  };

  const handleImportSubmit = async (jsonContent: string) => {
    setImportJsonContent(jsonContent);
    setIsImporting(true);
    
    try {
      // Import with auto-mapping to current chat connection
      const result = await dashboardService.importDashboard(chatId, {
        json: jsonContent,
        mappings: {}, // Backend auto-maps to current chat's connection
        options: {
          skipInvalidWidgets: false,
          autoCreateConnections: false,
        },
      });
      
      setShowImportModal(false);
      setImportJsonContent('');
      
      // Reload dashboard list and switch to imported dashboard
      await loadDashboardList();
      await loadDashboard(result.dashboardId);
      
      const warnings = result.summary.warnings?.length || 0;
      
      if (warnings > 0) {
        toast.success(
          `Dashboard imported with ${result.summary.widgetsImported} widgets. Some queries may need regeneration.`,
          { icon: '⚠️', duration: 5000 }
        );
      } else {
        toast.success(
          `Dashboard imported successfully with ${result.summary.widgetsImported} widgets!`,
          { icon: '✅' }
        );
      }
    } catch (error) {
      console.error('Import failed:', error);
      toast.error('Failed to import dashboard');
    } finally {
      setIsImporting(false);
    }
  };

  // =============================================
  // Render
  // =============================================

  if (isLoadingList) {
    return (
      <div className="flex-1 flex items-center justify-center bg-[#FFDB58]/10 pt-16 md:pt-0">
        <div className="flex items-center gap-2">
          <Loader2 className="w-5 h-5 animate-spin text-black" />
          <span className="text-sm font-medium text-black">Loading dashboards...</span>
        </div>
      </div>
    );
  }

  const showEmptyState = dashboardList.length === 0 && !activeDashboard;

  if (showEmptyState && !generationProgress && !showBlueprintPicker && !showPromptModal) {
    return (
      <div className="flex-1 bg-[#FFDB58]/10 overflow-y-auto pt-16 md:pt-0">
        <DashboardEmptyState
          onExploreSuggestions={handleExploreSuggestions}
          onCreateWithAI={handleCreateWithAI}
          onImportDashboard={handleImportDashboard}
        />
        
        {/* Import Dashboard Modal */}
        {showImportModal && (
          <ImportDashboardModal
            isImporting={isImporting}
            onSubmit={handleImportSubmit}
            onClose={() => setShowImportModal(false)}
          />
        )}
      </div>
    );
  }

  const editingWidget = editingWidgetId
    ? activeDashboard?.widgets.find((w) => w.id === editingWidgetId)
    : null;

  return (
    <div className="flex-1 flex flex-col bg-[#FFDB58]/10 overflow-hidden pt-16 md:pt-0">
      {showEmptyState ? (
        <div className="flex-1 overflow-y-auto">
          <DashboardEmptyState
            onExploreSuggestions={handleExploreSuggestions}
            onCreateWithAI={handleCreateWithAI}
            onImportDashboard={handleImportDashboard}
          />
        </div>
      ) : (
        <>
          {/* Header */}
          <DashboardHeader
            dashboardList={dashboardList}
            activeDashboard={activeDashboard}
            isConnected={isConnected}
            isRefreshing={isRefreshing}
            onSelectDashboard={(id) => loadDashboard(id)}
            onRefreshDashboard={() => handleRefreshDashboard(false)}
            onCancelRefresh={handleCancelDashboardRefresh}
            onRefreshIntervalChange={handleRefreshIntervalChange}
            onNewDashboard={handleNewDashboard}
            onAddWidget={() => setShowAddWidgetModal(true)}
            onRegenerateDashboard={() => setShowRegenerateModal(true)}
            onDeleteDashboard={() => setShowDeleteDashboardConfirm(true)}
            onExportDashboard={handleExportDashboard}
            onImportDashboard={handleImportDashboard}
          />

          {/* Widget Grid */}
          <div className="flex-1 overflow-y-auto py-8">
            {isLoadingDashboard && (
              <div className="flex items-center justify-center h-64">
                <div className="flex items-center gap-2">
                  <Loader2 className="w-5 h-5 animate-spin text-black" />
                  <span className="text-sm font-medium text-black">Loading dashboard...</span>
                </div>
              </div>
            )}

            {!isLoadingDashboard && activeDashboard && (
              <DashboardWidgetGrid
                dashboard={activeDashboard}
                onDeleteWidget={handleDeleteWidget}
                onEditWidget={(widgetId) => setEditingWidgetId(widgetId)}
                onRefreshWidget={handleRefreshWidget}
                onCancelWidgetRefresh={handleCancelWidgetRefresh}
                onWidgetNextPage={handleWidgetNextPage}
                onWidgetPreviousPage={handleWidgetPreviousPage}
                individuallyRefreshingWidgets={individuallyRefreshingWidgets}
                onAddWidget={() => setShowAddWidgetModal(true)}
              />
            )}
          </div>
        </>
      )}

      {/* === MODALS === */}

      {/* Blueprint Picker */}
      {showBlueprintPicker && blueprints.length > 0 && (
        <BlueprintPickerModal
          blueprints={blueprints}
          selectedIndices={selectedBlueprintIndices}
          isCreating={isCreatingFromBlueprints}
          onToggleSelection={(index) => {
            setSelectedBlueprintIndices((prev) => {
              const next = new Set(prev);
              if (next.has(index)) next.delete(index);
              else next.add(index);
              return next;
            });
          }}
          onCreate={handleCreateFromBlueprints}
          onClose={() => setShowBlueprintPicker(false)}
        />
      )}

      {/* Create with AI Prompt */}
      {showPromptModal && (
        <CreateDashboardPromptModal
          onSubmit={handleSubmitAIPrompt}
          onClose={() => setShowPromptModal(false)}
        />
      )}

      {/* Add Widget */}
      {showAddWidgetModal && (
        <AddWidgetModal
          isAdding={isAddingWidget}
          onSubmit={handleAddWidget}
          onClose={() => setShowAddWidgetModal(false)}
        />
      )}

      {/* Edit Widget */}
      {editingWidgetId && editingWidget && (
        <EditWidgetModal
          widgetTitle={editingWidget.title}
          isEditing={isEditingWidget}
          onSubmit={(prompt) => handleEditWidget(editingWidgetId, prompt)}
          onClose={() => setEditingWidgetId(null)}
        />
      )}

      {/* Regenerate Dashboard */}
      {showRegenerateModal && (
        <RegenerateDashboardModal
          onSubmit={handleRegenerateDashboard}
          onClose={() => setShowRegenerateModal(false)}
        />
      )}

      {/* Delete Dashboard Confirmation */}
      {showDeleteDashboardConfirm && (
        <ConfirmationModal
          icon={<Trash2 className="w-6 h-6 text-neo-error" />}
          title="Delete Dashboard"
          message={`Are you sure you want to delete "${activeDashboard?.name ?? ''}"? All widgets will be permanently removed. This action cannot be undone.`}
          buttonText="Delete"
          onConfirm={handleDeleteDashboard}
          onCancel={() => setShowDeleteDashboardConfirm(false)}
          zIndex="z-[120]"
        />
      )}

      {/* Import Dashboard */}
      {showImportModal && (
        <ImportDashboardModal
          isImporting={isImporting}
          onSubmit={handleImportSubmit}
          onClose={() => setShowImportModal(false)}
        />
      )}

      {/* Generation Progress */}
      {generationProgress && (
        <DashboardProgressOverlay
          progress={generationProgress}
          generationContext={generationContext}
          onCancel={handleCancelGeneration}
        />
      )}
    </div>
  );
}
