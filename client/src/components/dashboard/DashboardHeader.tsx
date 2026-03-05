import {
  ChevronDown,
  Clock,
  MoreHorizontal,
  Plus,
  RefreshCcw,
  Sparkles,
  Trash2,
  X,
} from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import {
  Dashboard,
  DashboardListItem,
  REFRESH_INTERVAL_OPTIONS,
} from '../../types/dashboard';

interface DashboardHeaderProps {
  dashboardList: DashboardListItem[];
  activeDashboard: Dashboard | null;
  isConnected: boolean;
  isRefreshing: boolean;
  onSelectDashboard: (id: string) => void;
  onRefreshDashboard: () => void;
  onCancelRefresh?: () => void;
  onRefreshIntervalChange: (interval: number) => void;
  onNewDashboard: () => void;
  onAddWidget: () => void;
  onRegenerateDashboard: () => void;
  onDeleteDashboard: () => void;
}

export default function DashboardHeader({
  dashboardList,
  activeDashboard,
  isConnected,
  isRefreshing,
  onSelectDashboard,
  onRefreshDashboard,
  onCancelRefresh,
  onRefreshIntervalChange,
  onNewDashboard,
  onAddWidget,
  onRegenerateDashboard,
  onDeleteDashboard,
}: Readonly<DashboardHeaderProps>) {
  const [showSelector, setShowSelector] = useState(false);
  const [selectorPosition, setSelectorPosition] = useState<{ top: number; left: number } | null>(null);
  const [showRefreshMenu, setShowRefreshMenu] = useState(false);
  const [refreshMenuPosition, setRefreshMenuPosition] = useState<{ top: number; right: number } | null>(null);
  const [showMoreMenu, setShowMoreMenu] = useState(false);
  const [moreMenuPosition, setMoreMenuPosition] = useState<{ top: number; left: number } | null>(null);

  const selectorBtnRef = useRef<HTMLButtonElement>(null);
  const refreshBtnRef = useRef<HTMLButtonElement>(null);
  const moreMenuBtnRef = useRef<HTMLButtonElement>(null);

  // Close dropdowns on outside click
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      const target = e.target as HTMLElement;
      if (showSelector && !target.closest('.dashboard-selector-dropdown') && !target.closest('.dashboard-selector-btn')) {
        setShowSelector(false);
        setSelectorPosition(null);
      }
      if (showRefreshMenu && !target.closest('.dashboard-refresh-dropdown') && !target.closest('.dashboard-refresh-btn')) {
        setShowRefreshMenu(false);
        setRefreshMenuPosition(null);
      }
      if (showMoreMenu && !target.closest('.dashboard-more-dropdown') && !target.closest('.dashboard-more-btn')) {
        setShowMoreMenu(false);
        setMoreMenuPosition(null);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [showSelector, showRefreshMenu, showMoreMenu]);

  const handleToggleSelector = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (showSelector) {
      setShowSelector(false);
      setSelectorPosition(null);
      return;
    }
    const rect = e.currentTarget.getBoundingClientRect();
    setSelectorPosition({ top: rect.bottom + 8, left: rect.left });
    setShowSelector(true);
  };

  const handleToggleRefreshMenu = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (showRefreshMenu) {
      setShowRefreshMenu(false);
      setRefreshMenuPosition(null);
      return;
    }
    const rect = e.currentTarget.getBoundingClientRect();
    setRefreshMenuPosition({ top: rect.bottom + 8, right: window.innerWidth - rect.right });
    setShowRefreshMenu(true);
  };

  const handleToggleMoreMenu = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (showMoreMenu) {
      setShowMoreMenu(false);
      setMoreMenuPosition(null);
      return;
    }
    const rect = e.currentTarget.getBoundingClientRect();
    setMoreMenuPosition({
      top: rect.bottom + 8,
      left: rect.right - 214,
    });
    setShowMoreMenu(true);
  };

  // Filter out Manual from the refresh interval options
  const refreshOptions = REFRESH_INTERVAL_OPTIONS.filter((o) => o.value !== 0);

  const currentRefreshLabel =
    REFRESH_INTERVAL_OPTIONS.find(
      (o) => o.value === (activeDashboard?.refresh_interval ?? 0)
    )?.label ?? 'Off';

  return (
    <>
      <div className="flex items-center justify-between px-4 py-3 border-b-2 border-black backdrop-blur-sm flex-shrink-0">
        <div className="flex items-center gap-3 min-w-0">
          {/* Dashboard Selector Dropdown */}
          <div className="relative">
            <button
              ref={selectorBtnRef}
              onClick={handleToggleSelector}
              className="
                dashboard-selector-btn flex items-center gap-2 px-4 py-2
                bg-white border-2 border-black rounded-lg
                shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]
                hover:shadow-[1px_1px_0px_0px_rgba(0,0,0,1)]
                hover:translate-x-[1px] hover:translate-y-[1px]
                transition-all duration-100
                max-w-[140px] md:max-w-[280px]
              "
            >
              <span className="text-base font-bold text-black truncate">
                {activeDashboard?.name ?? 'Select Dashboard'}
              </span>
              <ChevronDown className="w-5 h-5 text-black flex-shrink-0" />
            </button>
          </div>
        </div>

        {/* Right side controls */}
        <div className="flex items-center gap-2.5 flex-shrink-0">
          {/* Manual refresh — hidden on mobile, shown in more menu instead */}
          <div className="relative group hidden md:flex items-center gap-1">
            {!isRefreshing ? (
              <button
                onClick={onRefreshDashboard}
                className="
                  p-2.5 bg-white border-2 border-black rounded-lg
                  shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]
                  hover:shadow-[1px_1px_0px_0px_rgba(0,0,0,1)]
                  hover:translate-x-[1px] hover:translate-y-[1px]
                  transition-all duration-100
                "
              >
                <RefreshCcw className="w-5 h-5 text-black" />
              </button>
            ) : onCancelRefresh ? (
              <button
                onClick={onCancelRefresh}
                className="
                  p-2.5 bg-white border-2 border-black rounded-lg
                  shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]
                  hover:shadow-[1px_1px_0px_0px_rgba(0,0,0,1)]
                  hover:translate-x-[1px] hover:translate-y-[1px]
                  hover:bg-red-50
                  transition-all duration-100
                "
                title="Cancel refresh"
              >
                <X className="w-5 h-5 text-red-500" />
              </button>
            ) : (
              <button
                disabled
                className="
                  p-2.5 bg-white border-2 border-black rounded-lg
                  shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]
                  opacity-50 cursor-not-allowed
                "
              >
                <RefreshCcw className="w-5 h-5 text-black animate-spin" />
              </button>
            )}
            {!isRefreshing && (
              <div className="absolute invisible opacity-0 group-hover:visible group-hover:opacity-100 transition-opacity duration-200 bottom-[-35px] left-1/2 transform -translate-x-1/2 bg-black text-white text-xs py-1 px-2 rounded whitespace-nowrap z-50 before:content-[''] before:absolute before:top-[-5px] before:left-1/2 before:transform before:-translate-x-1/2 before:border-[5px] before:border-transparent before:border-b-black">
                Refresh now
              </div>
            )}
          </div>

          {/* Refresh interval selector — hidden on mobile */}
          <div className="relative group hidden md:block">
            <button
              ref={refreshBtnRef}
              onClick={handleToggleRefreshMenu}
              className="
                dashboard-refresh-btn flex items-center gap-1.5 px-3 py-2.5
                text-sm font-bold text-black
                bg-white border-2 border-black rounded-lg
                shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]
                hover:shadow-[1px_1px_0px_0px_rgba(0,0,0,1)]
                hover:translate-x-[1px] hover:translate-y-[1px]
                transition-all duration-100
              "
            >
              <Clock className="w-5 h-5" />
              <span className="text-sm font-bold">{currentRefreshLabel}</span>
            </button>
            {!showRefreshMenu && (
              <div className="absolute invisible opacity-0 group-hover:visible group-hover:opacity-100 transition-opacity duration-200 bottom-[-35px] left-1/2 transform -translate-x-1/2 bg-black text-white text-xs py-1 px-2 rounded whitespace-nowrap z-50 before:content-[''] before:absolute before:top-[-5px] before:left-1/2 before:transform before:-translate-x-1/2 before:border-[5px] before:border-transparent before:border-b-black">
                Auto-refresh interval
              </div>
            )}
          </div>

          {/* More menu */}
          <div className="relative group">
            <button
              ref={moreMenuBtnRef}
              onClick={handleToggleMoreMenu}
              className="
                dashboard-more-btn p-2.5 bg-white border-2 border-black rounded-lg
                shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]
                hover:shadow-[1px_1px_0px_0px_rgba(0,0,0,1)]
                hover:translate-x-[1px] hover:translate-y-[1px]
                transition-all duration-100
              "
            >
              <MoreHorizontal className="w-5 h-5 text-black" />
            </button>
            {!showMoreMenu && (
              <div className="absolute invisible opacity-0 group-hover:visible group-hover:opacity-100 transition-opacity duration-200 bottom-[-35px] left-1/2 transform -translate-x-1/2 bg-black text-white text-xs py-1 px-2 rounded whitespace-nowrap z-50 before:content-[''] before:absolute before:top-[-5px] before:left-1/2 before:transform before:-translate-x-1/2 before:border-[5px] before:border-transparent before:border-b-black">
                More options
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Dashboard Selector Dropdown (fixed) */}
      {showSelector && selectorPosition && (
        <div
          className="dashboard-selector-dropdown fixed w-80 bg-white border-4 border-black rounded-lg shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] z-[100] py-1 max-h-64 overflow-y-auto"
          style={{ top: `${selectorPosition.top}px`, left: `${selectorPosition.left}px` }}
          onClick={(e) => e.stopPropagation()}
        >
          {dashboardList.map((d) => (
            <button
              key={d.id}
              onClick={() => {
                onSelectDashboard(d.id);
                setShowSelector(false);
                setSelectorPosition(null);
              }}
              className={`
                w-full text-left px-4 py-2.5 font-medium
                hover:bg-gray-200 transition-colors
                ${activeDashboard?.id === d.id ? 'bg-[#FFDB58]' : ''}
              `}
            >
              <div className="text-base font-bold text-black truncate">{d.name}</div>
              {d.description && (
                <div className="text-sm text-gray-500 truncate mt-0.5">{d.description}</div>
              )}
            </button>
          ))}
          <div className="border-t-2 border-gray-200 mt-1 pt-1 mx-2">
            <button
              onClick={() => {
                setShowSelector(false);
                setSelectorPosition(null);
                onNewDashboard();
              }}
              className="w-full text-left px-3 py-2.5 text-sm font-bold text-black hover:bg-gray-200 flex items-center gap-2 rounded-lg"
            >
              <Plus className="w-5 h-5" />
              New Dashboard
            </button>
          </div>
        </div>
      )}

      {/* Refresh Interval Dropdown (fixed) */}
      {showRefreshMenu && refreshMenuPosition && (
        <div
          className="dashboard-refresh-dropdown fixed w-36 bg-white border-4 border-black rounded-lg shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] z-[100]"
          style={{ top: `${refreshMenuPosition.top}px`, right: `${refreshMenuPosition.right}px` }}
          onClick={(e) => e.stopPropagation()}
        >
          <div className="px-4 py-2 border-b-2 border-gray-200">
            <span className="text-xs font-bold text-gray-400 uppercase tracking-wider">Auto-refresh</span>
          </div>
          <div className="py-1">
            <button
              onClick={() => {
                onRefreshIntervalChange(0);
                setShowRefreshMenu(false);
                setRefreshMenuPosition(null);
              }}
              className={`
                w-full text-left px-4 py-2 text-sm font-semibold
                hover:bg-gray-200 transition-colors
                ${(activeDashboard?.refresh_interval ?? 0) === 0 ? 'bg-[#FFDB58] font-bold' : ''}
              `}
            >
              Off
            </button>
            <div className="h-px bg-gray-200 mx-2" />
            {refreshOptions.map((opt) => (
              <button
                key={opt.value}
                onClick={() => {
                  onRefreshIntervalChange(opt.value);
                  setShowRefreshMenu(false);
                  setRefreshMenuPosition(null);
                }}
                className={`
                  w-full text-left px-4 py-2 text-sm font-semibold
                  hover:bg-gray-200 transition-colors
                  ${activeDashboard?.refresh_interval === opt.value ? 'bg-[#FFDB58] font-bold' : ''}
                `}
              >
                {opt.label}
              </button>
            ))}
          </div>
        </div>
      )}
      {showMoreMenu && moreMenuPosition && (
        <div
          className="dashboard-more-dropdown fixed w-52 bg-white border-4 border-black rounded-lg shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] z-[100]"
          style={{ top: `${moreMenuPosition.top}px`, left: `${moreMenuPosition.left}px` }}
          onClick={(e) => e.stopPropagation()}
        >
          <div className="py-1">
            {/* Refresh — visible only on mobile */}
            <button
              onClick={() => {
                setShowMoreMenu(false);
                setMoreMenuPosition(null);
                onRefreshDashboard();
              }}
              className="flex items-center w-full text-left px-4 py-2 text-sm font-semibold text-black hover:bg-gray-200 transition-colors md:hidden"
            >
              <RefreshCcw className="w-4 h-4 mr-2 text-black" />
              Refresh Now
            </button>
            <div className="h-px bg-gray-200 mx-2 md:hidden" />
            <button
              onClick={() => {
                setShowMoreMenu(false);
                setMoreMenuPosition(null);
                onAddWidget();
              }}
              className="flex items-center w-full text-left px-4 py-2 text-sm font-semibold text-black hover:bg-gray-200 transition-colors"
            >
              <Plus className="w-4 h-4 mr-2 text-black" />
              Add Widget
            </button>
            <div className="h-px bg-gray-200 mx-2" />
            <button
              onClick={() => {
                setShowMoreMenu(false);
                setMoreMenuPosition(null);
                onRegenerateDashboard();
              }}
              className="flex items-center w-full text-left px-4 py-2 text-sm font-semibold text-black hover:bg-gray-200 transition-colors"
            >
              <Sparkles className="w-4 h-4 mr-2 text-black" />
              Regenerate
            </button>
            <div className="h-px bg-gray-200 mx-2" />
            <button
              onClick={() => {
                setShowMoreMenu(false);
                setMoreMenuPosition(null);
                onDeleteDashboard();
              }}
              className="flex items-center w-full text-left px-4 py-2 text-sm font-semibold text-red-500 hover:bg-neo-error hover:text-white transition-colors"
            >
              <Trash2 className="w-4 h-4 mr-2" />
              Delete Dashboard
            </button>
          </div>
        </div>
      )}
    </>
  );
}
