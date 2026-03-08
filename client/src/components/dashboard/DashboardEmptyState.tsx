import { BarChart3, Bot, Sparkles, Table2, TrendingUp, PieChart } from 'lucide-react';

interface DashboardEmptyStateProps {
  onExploreSuggestions: () => void;
  onCreateWithAI: () => void;
  onImportDashboard?: () => void;
}

export default function DashboardEmptyState({
  onExploreSuggestions,
  onCreateWithAI,
  onImportDashboard,
}: Readonly<DashboardEmptyStateProps>) {
  return (
    <div className="flex flex-col items-center justify-center h-full px-4 py-12 select-none">
      {/* Animated widget mockup illustration */}
      <div className="relative w-[400px] h-80 mb-12">
        {/* Background grid dots */}
        <div className="absolute inset-0 opacity-10">
          <div
            className="w-full h-full"
            style={{
              backgroundImage: 'radial-gradient(circle, #000 1px, transparent 1px)',
              backgroundSize: '20px 20px',
            }}
          />
        </div>

        {/* Mock widget: Bar chart card */}
        <div
          className="absolute -top-2 left-0 w-48 h-36 bg-white border-3 border-black rounded-xl shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] p-4 animate-float-slow"
        >
          <div className="flex items-center gap-2 mb-3">
            <BarChart3 className="w-4.5 h-4.5 text-black" />
            <span className="text-sm font-bold text-black truncate">Revenue</span>
          </div>
          <div className="flex items-end gap-2 h-16">
            <div className="w-5 bg-[#FFD700] border border-black rounded-sm" style={{ height: '40%' }} />
            <div className="w-5 bg-[#FFD700] border border-black rounded-sm" style={{ height: '65%' }} />
            <div className="w-5 bg-[#FFD700] border border-black rounded-sm" style={{ height: '45%' }} />
            <div className="w-5 bg-[#FFD700] border border-black rounded-sm" style={{ height: '80%' }} />
            <div className="w-5 bg-[#FFD700] border border-black rounded-sm" style={{ height: '55%' }} />
            <div className="w-5 bg-[#FFD700] border border-black rounded-sm" style={{ height: '90%' }} />
          </div>
        </div>

        {/* Mock widget: Stat card - Users */}
        <div
          className="absolute top-2 right-0 w-44 h-32 bg-white border-3 border-black rounded-xl shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] p-4 animate-float-medium"
        >
          <div className="flex items-center gap-2 mb-2">
            <TrendingUp className="w-4.5 h-4.5 text-green-600" />
            <span className="text-sm font-bold text-black">Users</span>
          </div>
          <div className="text-3xl font-black text-black">12.4K</div>
          <div className="text-xs text-green-600 font-bold mt-1">+24.5%</div>
        </div>

        {/* Mock widget: Pie chart card */}
        <div
          className="absolute -bottom-2 left-0 w-44 h-40 bg-white border-3 border-black rounded-xl shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] p-4 animate-float-fast"
        >
          <div className="flex items-center gap-2 mb-3">
            <PieChart className="w-4.5 h-4.5 text-black" />
            <span className="text-sm font-bold text-black">Categories</span>
          </div>
          <div className="flex flex-col justify-center items-center gap-2">
            <div className="relative w-16 h-16">
              <svg viewBox="0 0 36 36" className="w-16 h-16">
                <circle cx="18" cy="18" r="15" fill="none" stroke="#FFD700" strokeWidth="5" strokeDasharray="40 60" strokeDashoffset="25" />
                <circle cx="18" cy="18" r="15" fill="none" stroke="#7CFC00" strokeWidth="5" strokeDasharray="25 75" strokeDashoffset="85" />
                <circle cx="18" cy="18" r="15" fill="none" stroke="#FF6B6B" strokeWidth="5" strokeDasharray="35 65" strokeDashoffset="60" />
              </svg>
            </div>
            <p className="text-xs text-gray-600 font-bold mt-1">Data distribution</p>
          </div>
        </div>

        {/* Mock widget: Table preview */}
        <div
          className="absolute bottom-2 right-2 w-44 h-36 bg-white border-3 border-black rounded-xl shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] p-4 animate-float-slow"
          style={{ animationDelay: '0.5s' }}
        >
          <div className="flex items-center gap-2 mb-3">
            <Table2 className="w-4.5 h-4.5 text-black" />
            <span className="text-sm font-bold text-black">Orders</span>
          </div>
          <div className="space-y-2">
            <div className="flex gap-1.5">
              <div className="h-2.5 bg-gray-300 rounded-full flex-1" />
              <div className="h-2.5 bg-gray-300 rounded-full flex-1" />
              <div className="h-2.5 bg-gray-300 rounded-full w-10" />
            </div>
            <div className="flex gap-1.5">
              <div className="h-2.5 bg-gray-200 rounded-full flex-1" />
              <div className="h-2.5 bg-gray-200 rounded-full flex-1" />
              <div className="h-2.5 bg-gray-200 rounded-full w-10" />
            </div>
            <div className="flex gap-1.5">
              <div className="h-2.5 bg-gray-100 rounded-full flex-1" />
              <div className="h-2.5 bg-gray-100 rounded-full flex-1" />
              <div className="h-2.5 bg-gray-100 rounded-full w-10" />
            </div>
          </div>
        </div>
      </div>

      {/* Title & subtitle */}
      <h2 className="text-2xl font-black text-black mb-2">See Your Data Come Alive</h2>
      <p className="text-base text-gray-600 text-center max-w-md mb-8 leading-relaxed">
        AI builds insightful dashboards for you to visualize data in real-time via charts, stats, tables and more.
      </p>

      {/* CTA Buttons */}
      <div className="flex flex-col sm:flex-row gap-5">
        <button
          onClick={onExploreSuggestions}
          className="
            neo-button inline-flex items-center justify-center gap-2 py-3 px-5
          "
        >
          <Sparkles className="w-4 h-4" />
          Try Recommendations
        </button>
        <button
          onClick={onCreateWithAI}
          className="neo-button-secondary inline-flex items-center justify-center gap-2 py-3 px-5"
        >
          <Bot className="w-4 h-4" />
          Customize With AI
        </button>
      </div>

      {/* Import Link */}
      {onImportDashboard && (
        <p className="text-sm text-gray-600 mt-6">
          Or{' '}
          <button
            onClick={onImportDashboard}
            className="text-green-600 hover:text-green-700 transition-colors"
          >
            Import a dashboard
          </button>
          {' '}(.json)
        </p>
      )}

      {/* Floating animation keyframes */}
      <style>{`
        @keyframes float-slow {
          0%, 100% { transform: translateY(0px); }
          50% { transform: translateY(-6px); }
        }
        @keyframes float-medium {
          0%, 100% { transform: translateY(0px); }
          50% { transform: translateY(-8px); }
        }
        @keyframes float-fast {
          0%, 100% { transform: translateY(0px); }
          50% { transform: translateY(-5px); }
        }
        .animate-float-slow {
          animation: float-slow 3s ease-in-out infinite;
        }
        .animate-float-medium {
          animation: float-medium 2.5s ease-in-out infinite;
          animation-delay: 0.3s;
        }
        .animate-float-fast {
          animation: float-fast 2s ease-in-out infinite;
          animation-delay: 0.6s;
        }
      `}</style>
    </div>
  );
}
