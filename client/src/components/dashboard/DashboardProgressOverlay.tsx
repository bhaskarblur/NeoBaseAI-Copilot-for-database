import { Loader2, X } from 'lucide-react';
import { DashboardGenerationProgressEvent } from '../../types/dashboard';

interface DashboardProgressOverlayProps {
  progress: DashboardGenerationProgressEvent;
  generationContext: 'blueprints' | 'creating' | 'custom';
  onCancel: () => void;
}

export default function DashboardProgressOverlay({
  progress,
  generationContext,
  onCancel,
}: Readonly<DashboardProgressOverlayProps>) {
  const title =
    generationContext === 'creating'
      ? 'Preparing the Selected Dashboards'
      : generationContext === 'custom'
        ? 'Preparing Customized Dashboard'
        : 'Curating Recommended Dashboards';

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 backdrop-blur-sm">
      <div className="
        bg-white border-4 border-black rounded-2xl
        shadow-[8px_8px_0px_0px_rgba(0,0,0,1)]
        w-full max-w-md mx-4 p-8 text-center
      ">
        <Loader2 className="w-10 h-10 animate-spin text-black mx-auto mb-5" />

        <h3 className="font-black text-xl text-black mb-2">
          {title}
        </h3>
        <p className="text-base text-gray-500 mb-6">
          {progress.message || 'Analyzing your data and building widgets\u2026'}
        </p>

        {/* Progress bar */}
        <div className="w-full h-3 bg-gray-100 border-2 border-black rounded-full overflow-hidden mb-2">
          <div
            className="h-full bg-[#FFD700] transition-all duration-500 ease-out"
            style={{ width: `${progress.progress}%` }}
          />
        </div>
        <p className="text-base text-gray-500 mb-6">{progress.progress}%</p>

        {/* Cancel button */}
        <button
          onClick={onCancel}
          className="neo-button-secondary inline-flex items-center justify-center gap-2 text-base"
        >
          <X className="w-5 h-5" />
          Cancel
        </button>
      </div>
    </div>
  );
}
