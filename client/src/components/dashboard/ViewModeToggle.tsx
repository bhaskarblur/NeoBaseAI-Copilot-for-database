import { LayoutDashboard, MessageSquare } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { DashboardViewMode } from '../../types/dashboard';

interface ViewModeToggleProps {
  mode: DashboardViewMode;
  onChange: (mode: DashboardViewMode) => void;
  dashboardCount?: number;
}

export default function ViewModeToggle({
  mode,
  onChange,
  dashboardCount = 0,
}: Readonly<ViewModeToggleProps>) {
  const chatRef = useRef<HTMLButtonElement>(null);
  const dashRef = useRef<HTMLButtonElement>(null);
  const [indicator, setIndicator] = useState({ left: 0, width: 0 });

  useEffect(() => {
    const activeRef = mode === 'chat' ? chatRef : dashRef;
    if (activeRef.current) {
      setIndicator({
        left: activeRef.current.offsetLeft,
        width: activeRef.current.offsetWidth,
      });
    }
  }, [mode, dashboardCount]);

  return (
    <div
      className="
        relative inline-flex items-center
        bg-white border-2 border-black rounded-lg
        shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]
        p-1
      "
    >
      {/* Sliding indicator */}
      <div
        className="absolute top-1 bottom-1 bg-black rounded-md transition-all duration-300 ease-in-out"
        style={{ left: indicator.left, width: indicator.width }}
      />

      {/* Chat tab */}
      <button
        ref={chatRef}
        onClick={() => onChange('chat')}
        className={`
          relative z-10 flex items-center gap-2 px-3 md:px-4 py-1.5 rounded-md
          text-sm font-bold transition-colors duration-300
          ${mode === 'chat' ? 'text-white' : 'text-gray-500 hover:text-black'}
        `}
      >
        <MessageSquare className="w-4 h-4" />
        <span className="hidden md:inline">Chat</span>
      </button>

      {/* Dashboard tab */}
      <button
        ref={dashRef}
        onClick={() => onChange('dashboard')}
        className={`
          relative z-10 flex items-center gap-2 px-3 md:px-4 py-1.5 rounded-md
          text-sm font-bold transition-colors duration-300
          ${mode === 'dashboard' ? 'text-white' : 'text-gray-500 hover:text-black'}
        `}
      >
        <LayoutDashboard className="w-4 h-4" />
        <span className="hidden md:inline">Dashboard</span>
        {dashboardCount > 0 && mode !== 'dashboard' && (
          <span className="
            ml-0.5 px-1.5 py-0.5
            bg-[#FFD700] text-black
            text-[10px] font-black
            rounded-md
            border border-black
            leading-none
          ">
            {dashboardCount}
          </span>
        )}
      </button>
    </div>
  );
}
