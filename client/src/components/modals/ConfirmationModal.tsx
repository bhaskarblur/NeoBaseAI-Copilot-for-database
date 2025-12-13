import { AlertTriangle, Loader2, X } from 'lucide-react';
import { useState } from 'react';
interface ConfirmationModalProps {
  themeColor?: string;
  buttonText?: string;
  icon?: React.ReactNode;
  title: string;
  message: string;
  onConfirm: () => Promise<void>;
  onCancel: () => void;
  zIndex?: string;
}

export default function ConfirmationModal({
  title,
  message,
  onConfirm,
  onCancel,
  themeColor = 'neo-error',
  icon = <AlertTriangle className={`w-6 h-6 text-${themeColor}`} />,
  buttonText = 'Confirm',
  zIndex = 'z-[110]',
}: ConfirmationModalProps) {
  const [isLoading, setIsLoading] = useState(false);
  return (
    <div className={`fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4 ${zIndex}`}>
      <div className="bg-white neo-border rounded-lg w-full max-w-md">
        <div className="flex justify-between items-center p-6 border-b-4 border-black">
          <div className="flex items-center gap-3">
            {icon}
            <h2 className="text-2xl font-bold">{title}</h2>
          </div>
          <button
            onClick={onCancel}
            className="hover:bg-neo-gray rounded-lg p-2 transition-colors"
          >
            <X className="w-6 h-6" />
          </button>
        </div>

        <div className="p-6">
          <p className="text-gray-600 mb-6">{message}</p>

          <div className="flex gap-4">
            <button
              onClick={async () => {
                setIsLoading(true);
                await onConfirm();
                setIsLoading(false);
              }}
              disabled={isLoading}
              className={`neo-border bg-${themeColor} disabled:opacity-50 disabled:cursor-not-allowed text-white px-4 py-2 font-bold text-base transition-all hover:translate-y-[-2px] hover:shadow-[6px_6px_0px_0px_rgba(0,0,0,1)] active:translate-y-[0px] active:shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] flex-1`}
            >
              {isLoading ? (
                <div className="flex items-center justify-center gap-2">
                  <Loader2 className="w-4 h-4 animate-spin" />
                  <span>Please Wait...</span>
                </div>
              ) : (
                buttonText
              )}
            </button>
            <button
              onClick={onCancel}
              className="neo-button-secondary flex-1"
            >
              Cancel
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}