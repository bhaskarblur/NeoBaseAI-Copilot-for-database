import { Copy, Loader2, X } from 'lucide-react';
import { useState } from 'react';

interface DuplicateChatModalProps {
  chatName: string;
  chatId: string;
  onConfirm: (chatId: string, duplicateMessages: boolean) => Promise<void>;
  onCancel: () => void;
}

export default function DuplicateChatModal({
  chatName,
  chatId,
  onConfirm,
  onCancel,
}: DuplicateChatModalProps) {
  const [isLoading, setIsLoading] = useState(false);
  const [duplicateMessages, setDuplicateMessages] = useState(false);

  return (
    <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4 z-50">
      <div className="bg-white neo-border rounded-lg w-full max-w-md">
        <div className="flex justify-between items-center p-6 border-b-4 border-black">
          <div className="flex items-center gap-3">
            <Copy className="w-6 h-6 text-green-500" />
            <h2 className="text-2xl font-bold">Duplicate Chat</h2>
          </div>
          <button
            onClick={onCancel}
            className="hover:bg-neo-gray rounded-lg p-2 transition-colors"
          >
            <X className="w-6 h-6" />
          </button>
        </div>

        <div className="p-6">
          <p className="text-gray-600 mb-6">
            Are you sure you want to duplicate "{chatName}"? This will create a new chat with the same database connection.
          </p>

          <div className="mb-6">
            <label className="flex items-center gap-2 cursor-pointer">
              <input 
                type="checkbox" 
                checked={duplicateMessages} 
                onChange={(e) => setDuplicateMessages(e.target.checked)}
                className="w-5 h-5 rounded border-2 border-black focus:ring-2 focus:ring-blue-500"
              />
              <span className="text-base">Duplicate all messages</span>
            </label>
            <p className="text-gray-500 text-sm mt-1 ml-7">
              All conversation history will be copied to the new chat.
            </p>
          </div>

          <div className="flex gap-4">
            <button
              onClick={async () => {
                setIsLoading(true);
                try {
                  await onConfirm(chatId, duplicateMessages);
                } finally {
                  setIsLoading(false);
                }
              }}
              className="neo-border bg-green-500 text-white px-4 py-2 font-bold text-base transition-all hover:translate-y-[-2px] hover:shadow-[6px_6px_0px_0px_rgba(0,0,0,1)] active:translate-y-[0px] active:shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] flex-1"
            >
              {isLoading ? (
                <div className="flex items-center justify-center gap-2">
                  <Loader2 className="w-4 h-4 animate-spin" />
                  <span>Duplicating...</span>
                </div>
              ) : (
                "Duplicate"
              )}
            </button>
            <button
              onClick={onCancel}
              className="neo-button-secondary flex-1"
              disabled={isLoading}
            >
              Cancel
            </button>
          </div>
        </div>
      </div>
    </div>
  );
} 