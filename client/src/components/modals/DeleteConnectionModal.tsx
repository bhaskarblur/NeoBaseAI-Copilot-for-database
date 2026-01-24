import { AlertTriangle, Loader2, X } from 'lucide-react';
import { useState } from 'react';

interface DeleteConnectionModalProps {
  connectionName: string;
  chatId: string;
  onConfirm: (chatId: string) => Promise<void>;
  onCancel: () => void;
}

export default function DeleteConnectionModal({
  connectionName,
  chatId,
  onConfirm,
  onCancel,
}: DeleteConnectionModalProps) {
  const [confirmText, setConfirmText] = useState('');
  const [isDeleting, setIsDeleting] = useState(false);
  const isConfirmValid = confirmText === connectionName;

  const handleDelete = async () => {
    if (!isConfirmValid) return;
    setIsDeleting(true);
    try {
      await onConfirm(chatId);
    } catch (error) {
      console.error('Failed to delete chat:', error);
    } finally {
      setIsDeleting(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4 z-50">
      <div className="bg-white neo-border rounded-lg w-full max-w-md">
        <div className="flex justify-between items-center p-6 border-b-4 border-black">
          <div className="flex items-center gap-3">
            <AlertTriangle className="w-6 h-6 text-neo-error" />
            <h2 className="text-2xl font-bold">Delete Chat</h2>
          </div>
          <button
            onClick={onCancel}
            className="hover:bg-neo-gray rounded-lg p-2 transition-colors"
            disabled={isDeleting}
          >
            <X className="w-6 h-6" />
          </button>
        </div>

        <div className="p-6">
          <p className="text-gray-600 mb-6">
            Are you sure you want to delete the chat <strong>{connectionName}</strong>?
            This action cannot be undone.
          </p>

          <div className="mb-6">
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Type <span className="font-mono bg-gray-100 px-2 py-1 rounded">{connectionName}</span> to confirm:
            </label>
            <input
              type="text"
              value={confirmText}
              onChange={(e) => setConfirmText(e.target.value)}
              className="neo-input w-full"
              placeholder={`Type ${connectionName} to confirm`}
              autoFocus
              disabled={isDeleting}
            />
          </div>

          <div className="flex gap-4">
            <button
              onClick={handleDelete}
              disabled={!isConfirmValid || isDeleting}
              className={`neo-border px-4 py-2 font-bold text-base transition-all flex-1 ${isConfirmValid && !isDeleting
                ? 'bg-neo-error text-white hover:translate-y-[-2px] hover:shadow-[6px_6px_0px_0px_rgba(0,0,0,1)] active:translate-y-[0px] active:shadow-[4px_4px_0px_0px_rgba(0,0,0,1)]'
                : 'bg-gray-200 text-gray-400 cursor-not-allowed'
                }`}
            >
              {isDeleting ? (
                <div className="flex items-center justify-center gap-2">
                  <Loader2 className="w-4 h-4 animate-spin" />
                  <span>Deleting...</span>
                </div>
              ) : (
                'Delete Chat'
              )}
            </button>
            <button
              onClick={onCancel}
              className="neo-button-secondary flex-1"
              disabled={isDeleting}
            >
              Cancel
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}