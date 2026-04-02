import { Trash2 } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import toast from 'react-hot-toast';
import { dashboardService } from '../services/dashboardService';
import {
  Dashboard,
  DashboardBlueprint,
  DashboardBlueprintEvent,
  DashboardGenerationCompleteEvent as DashboardGenCompleteEvent,
  DashboardGenerationProgressEvent,
  DashboardListItem,
  DashboardWidgetDataEvent,
  DASHBOARD_SSE_EVENTS,
} from '../types/dashboard';

// Module-level cache to prevent re-fetching on tab toggle
const _listCache = new Map<string, { list: DashboardListItem[]; activeDashId?: string }>();
const _dashCache = new Map<string, Dashboard>();

interface UseDashboardProps {
  chatId: string;
  streamId: string | null;
  isConnected: boolean;
  onReconnect?: () => Promise<void>;
}

export function useDashboard({ chatId, streamId, isConnected, onReconnect }: UseDashboardProps) {
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
  const WIDGET_LOADING_TIMEOUT_MS = 30_000;

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
      await new Promise((r) => setTimeout(r, 150));
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

  const loadDashboard = async (dashboardId: string) => {
    try {
      setIsLoadingDashboard(true);
      const dashboard = await dashboardService.getDashboard(chatId, dashboardId);
      setActiveDashboard(dashboard);
      _dashCache.set(dashboardId, dashboard);
      const cached = _listCache.get(chatId);
      if (cached) _listCache.set(chatId, { ...cached, activeDashId: dashboardId });

      hasFetchedOnceRef.current = null;

      const hasUnfetched = dashboard.widgets.some((w) => !w.data && !w.is_loading && !w.error);
      if (hasUnfetched && streamId && isConnected) {
        hasFetchedOnceRef.current = dashboardId;
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

  // Reset state + load list when chatId changes
  useEffect(() => {
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
  // eslint-disable-next-line react-hooks/exhaustive-deps
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
  // eslint-disable-next-line react-hooks/exhaustive-deps
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
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // =============================================
  // Handlers
  // =============================================

  const handleRefreshDashboard = useCallback(async (silent = false, retryCount = 0) => {
    if (!activeDashboard) return;

    const connected = await ensureSSEConnection(silent);
    if (!connected) return;
    const sid = streamIdRef.current;
    if (!sid) return;

    setActiveDashboard((prev) => {
      if (!prev) return null;
      return {
        ...prev,
        widgets: prev.widgets.map((w) => ({
          ...w,
          is_loading: true,
          error: undefined,
          page_data_cache: undefined,
          cursor_stack: undefined,
          current_page: 1,
          next_cursor: null,
          has_more: false,
        })),
      };
    });
    activeDashboard.widgets.forEach((w) => startLoadingTimeout(w.id));

    setIsRefreshing(true);
    try {
      await dashboardService.refreshDashboard(chatId, activeDashboard.id, sid);
      if (!silent) toast.success('Dashboard refreshing...', { icon: '✅' });
    } catch (err: any) {
      console.error('Failed to refresh dashboard:', err);

      const errorMsg = err?.response?.data?.error || err?.message || '';
      const isSSENotFound = errorMsg.includes('SSE stream not found');
      const isDBNotConnected = errorMsg.includes('database connection not found') || errorMsg.includes('connection not found');

      if ((isSSENotFound || isDBNotConnected) && onReconnect && retryCount < 3) {
        clearAllLoadingTimeouts();
        setIsRefreshing(false);

        if (!silent) toast.loading('Establishing connections...', { id: 'reconnect-refresh', duration: 8000 });

        try {
          await onReconnect();
          await new Promise((r) => setTimeout(r, 300));
          if (!silent) toast.success('Connected! Refreshing...', { id: 'reconnect-refresh', icon: '✅' });
          handleRefreshDashboard(silent, retryCount + 1);
          return;
        } catch (reconnectErr) {
          console.error('Failed to reconnect:', reconnectErr);
          if (!silent) toast.error('Connection failed. Please try manually.', { id: 'reconnect-refresh' });
        }
      }

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
    if (hasFetchedOnceRef.current === dashId) return;
    const hasUnfetchedWidgets = activeDashboard.widgets.some(
      (w) => !w.data && !w.is_loading && !w.error
    );
    if (!hasUnfetchedWidgets) return;
    hasFetchedOnceRef.current = dashId;
    handleRefreshDashboard(true);
  }, [activeDashboard?.id, activeDashboard?.widgets, streamId, isConnected, handleRefreshDashboard]);

  const handleRefreshWidget = useCallback(async (widgetId: string, retryCount = 0, cursor?: string) => {
    if (!activeDashboard) return;

    const connected = await ensureSSEConnection();
    if (!connected) return;
    const sid = streamIdRef.current;
    if (!sid) return;

    setIndividuallyRefreshingWidgets((prev) => new Set(prev).add(widgetId));
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

      const errorMsg = err?.response?.data?.error || err?.message || '';
      const isSSENotFound = errorMsg.includes('SSE stream not found');
      const isDBNotConnected = errorMsg.includes('database connection not found') || errorMsg.includes('connection not found');

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
          await new Promise((r) => setTimeout(r, 300));
          toast.success('Connected! Refreshing widget...', { id: 'reconnect-widget', icon: '✅' });
          handleRefreshWidget(widgetId, retryCount + 1, cursor);
          return;
        } catch (reconnectErr) {
          console.error('Failed to reconnect:', reconnectErr);
          toast.error('Connection failed. Please try manually.', { id: 'reconnect-widget' });
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

  const handleWidgetNextPage = useCallback((widgetId: string) => {
    if (!activeDashboard) return;

    const widget = activeDashboard.widgets.find(w => w.id === widgetId);
    if (!widget || !widget.table_config?.cursor_field || !widget.has_more || widget.is_loading) return;

    const cursorToUse = widget.next_cursor ?? null;
    const currentPage = widget.current_page || 1;
    const nextPage = currentPage + 1;
    const currentData = widget.data;
    const cachedNextData = widget.page_data_cache?.[nextPage];

    if (cachedNextData) {
      setActiveDashboard((prev) => {
        if (!prev) return null;
        return {
          ...prev,
          widgets: prev.widgets.map((w) =>
            w.id === widgetId ? {
              ...w,
              current_page: nextPage,
              cursor_stack: [...(w.cursor_stack ?? [null]), cursorToUse],
              data: cachedNextData.data,
              next_cursor: cachedNextData.next_cursor,
              has_more: cachedNextData.has_more,
              page_data_cache: {
                ...(w.page_data_cache ?? {}),
                [currentPage]: { data: currentData ?? [], next_cursor: cursorToUse, has_more: true },
              },
            } : w
          ),
        };
      });
      return;
    }

    handleRefreshWidget(widgetId, 0, cursorToUse || undefined);

    setActiveDashboard((prev) => {
      if (!prev) return null;
      return {
        ...prev,
        widgets: prev.widgets.map((w) =>
          w.id === widgetId ? {
            ...w,
            current_page: nextPage,
            cursor_stack: [...(w.cursor_stack ?? [null]), cursorToUse],
            page_data_cache: {
              ...(w.page_data_cache ?? {}),
              [currentPage]: { data: currentData ?? [], next_cursor: cursorToUse, has_more: true },
            },
          } : w
        ),
      };
    });
  }, [activeDashboard, handleRefreshWidget]);

  const handleWidgetPreviousPage = useCallback((widgetId: string) => {
    if (!activeDashboard) return;

    const widget = activeDashboard.widgets.find(w => w.id === widgetId);
    if (!widget || !widget.table_config?.cursor_field || widget.is_loading) return;

    const currentStack = widget.cursor_stack ?? [null];
    if (currentStack.length <= 1) return;

    const newStack = currentStack.slice(0, -1);
    const currentPage = widget.current_page || 1;
    const prevPage = Math.max(1, currentPage - 1);
    const cachedData = widget.page_data_cache?.[prevPage];
    const poppedCursor = currentStack[currentStack.length - 1];

    if (cachedData) {
      const currentData = widget.data;
      setActiveDashboard((prev) => {
        if (!prev) return null;
        return {
          ...prev,
          widgets: prev.widgets.map((w) =>
            w.id === widgetId ? {
              ...w,
              current_page: prevPage,
              cursor_stack: newStack,
              data: cachedData.data,
              next_cursor: poppedCursor,
              has_more: true,
              page_data_cache: {
                ...(w.page_data_cache ?? {}),
                [currentPage]: { data: currentData ?? [], next_cursor: w.next_cursor ?? null, has_more: w.has_more ?? false },
              },
            } : w
          ),
        };
      });
    } else {
      const prevCursor = newStack[newStack.length - 1];
      setActiveDashboard((prev) => {
        if (!prev) return null;
        return {
          ...prev,
          widgets: prev.widgets.map((w) =>
            w.id === widgetId ? { ...w, current_page: prevPage, cursor_stack: newStack } : w
          ),
        };
      });
      handleRefreshWidget(widgetId, 0, prevCursor ?? undefined);
    }
  }, [activeDashboard, handleRefreshWidget]);

  const handleManualRefreshWidget = useCallback((widgetId: string) => {
    setActiveDashboard((prev) => {
      if (!prev) return null;
      return {
        ...prev,
        widgets: prev.widgets.map((w) =>
          w.id === widgetId ? {
            ...w,
            page_data_cache: undefined,
            cursor_stack: undefined,
            current_page: 1,
            next_cursor: null,
            has_more: false,
          } : w
        ),
      };
    });
    handleRefreshWidget(widgetId);
  }, [handleRefreshWidget]);

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
            next_cursor: event.next_cursor ?? null,
            has_more: event.has_more,
            page_data_cache: w.page_data_cache,
          };
        }),
      };
    });
  }, [clearLoadingTimeout]);

  const isReconnectingRef = useRef(false);

  const handleWidgetErrorUpdate = useCallback((widgetId: string, error: string) => {
    clearLoadingTimeout(widgetId);
    setIndividuallyRefreshingWidgets((prev) => {
      const next = new Set(prev);
      next.delete(widgetId);
      return next;
    });
    if (error.toLowerCase().includes('no connection found') && onReconnect && !isReconnectingRef.current) {
      isReconnectingRef.current = true;
      console.log('[Dashboard] Connection lost — auto-reconnecting...');
      toast.loading('Reconnecting to database...', { id: 'dashboard-reconnect', duration: 5000 });

      onReconnect().then(() => {
        toast.success('Reconnected! Refreshing dashboard...', { id: 'dashboard-reconnect', icon: '✅' });
        if (activeDashboard && streamId) {
          dashboardService.refreshDashboard(chatId, activeDashboard.id, streamId).catch(() => {});
        }
      }).catch(() => {
        toast.error('Failed to reconnect. Please try manually.', { id: 'dashboard-reconnect', icon: '❌' });
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
      toast.success('Widget removed', { icon: <Trash2 size={18} className='text-red-500' /> });
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
      console.error('Failed to generate suggestions:', err);
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
      console.error('Failed to create dashboards:', err);
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
      setActiveDashboard((prev) => {
        if (!prev) return null;
        return { ...prev, widgets: [...prev.widgets, { ...widget, data: undefined, is_loading: true, error: undefined }] };
      });
      setShowAddWidgetModal(false);
      toast.success(`Widget "${widget.title}" added!`);

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
    setShowImportModal(true);
  };

  const handleImportSubmit = async (jsonContent: string) => {
    setImportJsonContent(jsonContent);
    setIsImporting(true);

    try {
      const result = await dashboardService.importDashboard(chatId, {
        json: jsonContent,
        mappings: {},
        options: {
          skipInvalidWidgets: false,
          autoCreateConnections: false,
        },
      });

      setShowImportModal(false);
      setImportJsonContent('');

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

  return {
    // State
    isLoadingList,
    dashboardList,
    activeDashboard,
    isLoadingDashboard,
    isRefreshing,
    individuallyRefreshingWidgets,
    blueprints,
    showBlueprintPicker,
    selectedBlueprintIndices,
    isCreatingFromBlueprints,
    generationProgress,
    generationContext,
    showPromptModal,
    showAddWidgetModal,
    isAddingWidget,
    showRegenerateModal,
    showDeleteDashboardConfirm,
    showImportModal,
    isImporting,
    importJsonContent,
    editingWidgetId,
    isEditingWidget,
    // Setters (for inline handlers in render)
    setSelectedBlueprintIndices,
    setShowBlueprintPicker,
    setShowPromptModal,
    setShowAddWidgetModal,
    setEditingWidgetId,
    setShowRegenerateModal,
    setShowDeleteDashboardConfirm,
    setShowImportModal,
    // Navigation / loading
    loadDashboard,
    // Dashboard actions
    handleRefreshDashboard,
    handleCancelDashboardRefresh,
    handleRefreshIntervalChange,
    handleNewDashboard,
    handleRegenerateDashboard,
    handleDeleteDashboard,
    handleExportDashboard,
    handleImportDashboard,
    handleImportSubmit,
    // Widget actions
    handleManualRefreshWidget,
    handleCancelWidgetRefresh,
    handleWidgetNextPage,
    handleWidgetPreviousPage,
    handleDeleteWidget,
    handleEditWidget,
    handleAddWidget,
    // AI / blueprint actions
    handleExploreSuggestions,
    handleCreateFromBlueprints,
    handleCreateWithAI,
    handleSubmitAIPrompt,
    handleCancelGeneration,
  };
}
