import { useState } from 'react';
import { Link, X } from 'lucide-react';

interface ConnectionMappingModalProps {
  validationResult: {
    valid: boolean;
    errors?: string[];
    warnings?: string[];
    requiredConnections?: {
      name: string;
      type: string;
      usedBy: string[];
      suggestions?: string[];
    }[];
  };
  availableConnections: { id: string; name: string; type: string }[];
  onSubmit: (mappings: Record<string, string>, skipInvalid: boolean) => void;
  onCancel: () => void;
  onBack: () => void;
}

export function ConnectionMappingModal({
  validationResult,
  availableConnections,
  onSubmit,
  onCancel,
  onBack,
}: Readonly<ConnectionMappingModalProps>) {
  const [mappings, setMappings] = useState<Record<string, string>>({});
  const [skipInvalid, setSkipInvalid] = useState(false);

  const handleMappingChange = (sourceName: string, targetId: string) => {
    setMappings((prev) => ({
      ...prev,
      [sourceName]: targetId,
    }));
  };

  const handleAutoMap = () => {
    const newMappings: Record<string, string> = {};
    validationResult.requiredConnections?.forEach((required) => {
      // Try to find exact name match first
      const exactMatch = availableConnections.find(
        (conn) => conn.name === required.name && conn.type === required.type
      );
      if (exactMatch) {
        newMappings[required.name] = exactMatch.id;
      } else {
        // Try to find type match
        const typeMatch = availableConnections.find((conn) => conn.type === required.type);
        if (typeMatch) {
          newMappings[required.name] = typeMatch.id;
        }
      }
    });
    setMappings(newMappings);
  };

  const canSubmit = () => {
    if (!validationResult.requiredConnections) return true;
    if (skipInvalid) return true;
    return validationResult.requiredConnections.every((req) => mappings[req.name]);
  };

  return (
    <div className="fixed inset-0 z-[110] flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
      <div className="bg-white neo-border rounded-lg w-full max-w-3xl max-h-[80vh] flex flex-col">
        {/* Header */}
        <div className="flex justify-between items-center p-6 border-b-4 border-black">
          <div className="flex items-center gap-3">
            <div className="flex items-center justify-center">
              <Link className="w-5 h-5 text-black" />
            </div>
            <div>
              <h2 className="text-2xl font-bold">Map Connections</h2>
              <p className="text-sm text-gray-500 mt-0.5">
                Map imported connections to your existing data sources
              </p>
            </div>
          </div>
          <button
            onClick={onCancel}
            className="hover:bg-neo-gray rounded-lg p-2 transition-colors"
          >
            <X className="w-6 h-6" />
          </button>
        </div>

        {/* Body - Scrollable */}
        <div className="p-6 space-y-4 overflow-y-auto flex-1">
          {/* Warnings/Errors */}
          {validationResult.errors && validationResult.errors.length > 0 && (
            <div className="px-4 py-3 bg-red-50 border-2 border-red-300 rounded-lg">
              <p className="text-sm font-semibold text-red-600 mb-2">Errors:</p>
              <ul className="text-sm text-red-600 space-y-1">
                {validationResult.errors.map((error, idx) => (
                  <li key={idx}>• {error}</li>
                ))}
              </ul>
            </div>
          )}

          {validationResult.warnings && validationResult.warnings.length > 0 && (
            <div className="px-4 py-3 bg-yellow-50 border-2 border-yellow-300 rounded-lg">
              <p className="text-sm font-semibold text-yellow-600 mb-2">Warnings:</p>
              <ul className="text-sm text-yellow-600 space-y-1">
                {validationResult.warnings.map((warning, idx) => (
                  <li key={idx}>• {warning}</li>
                ))}
              </ul>
            </div>
          )}

          {/* Connection Mappings */}
          {validationResult.requiredConnections && validationResult.requiredConnections.length > 0 ? (
            <>
              <div className="flex justify-between items-center">
                <h3 className="text-lg font-bold">Required Connections</h3>
                <button
                  onClick={handleAutoMap}
                  className="text-sm px-3 py-1.5 border-2 border-black rounded-lg hover:bg-gray-100 font-semibold"
                >
                  Auto-Map by Name
                </button>
              </div>

              <div className="space-y-4">
                {validationResult.requiredConnections.map((required) => {
                  const matchingSuggestions = availableConnections.filter(
                    (conn) => conn.type === required.type
                  );

                  return (
                    <div
                      key={required.name}
                      className="p-4 border-2 border-gray-300 rounded-lg space-y-3"
                    >
                      <div>
                        <p className="font-bold text-black">{required.name}</p>
                        <p className="text-sm text-gray-500">
                          Type: <span className="font-semibold">{required.type}</span>
                        </p>
                        <p className="text-xs text-gray-400 mt-1">
                          Used by: {required.usedBy.join(', ')}
                        </p>
                      </div>

                      <div>
                        <label className="block text-sm font-bold text-black mb-2">
                          Map to Connection:
                        </label>
                        <select
                          value={mappings[required.name] || ''}
                          onChange={(e) => handleMappingChange(required.name, e.target.value)}
                          className="w-full px-3 py-2 border-2 border-black rounded-lg focus:outline-none focus:ring-2 focus:ring-black focus:ring-offset-1"
                        >
                          <option value="">-- Select Connection --</option>
                          {matchingSuggestions.length > 0 ? (
                            matchingSuggestions.map((conn) => (
                              <option key={conn.id} value={conn.id}>
                                {conn.name} ({conn.type})
                              </option>
                            ))
                          ) : (
                            <option disabled>No matching connections available</option>
                          )}
                        </select>
                      </div>
                    </div>
                  );
                })}
              </div>

              {/* Skip Invalid Option */}
              <div className="flex items-start gap-2 p-4 bg-gray-50 border-2 border-gray-300 rounded-lg">
                <input
                  type="checkbox"
                  id="skipInvalid"
                  checked={skipInvalid}
                  onChange={(e) => setSkipInvalid(e.target.checked)}
                  className="mt-1 w-4 h-4"
                />
                <label htmlFor="skipInvalid" className="text-sm flex-1">
                  <span className="font-semibold">Skip widgets with unmapped connections</span>
                  <p className="text-gray-500 mt-0.5">
                    Widgets that reference unmapped connections will be skipped during import
                  </p>
                </label>
              </div>
            </>
          ) : (
            <div className="px-4 py-3 bg-green-50 border-2 border-green-300 rounded-lg">
              <p className="text-sm text-green-600">
                ✓ All connections are available. No mapping required.
              </p>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex gap-4 p-6 border-t-4 border-black bg-gray-50/50">
          <button
            onClick={onBack}
            className="neo-button-secondary px-6"
          >
            Back
          </button>
          <button
            onClick={() => onSubmit(mappings, skipInvalid)}
            disabled={!canSubmit()}
            className="neo-button bg-black text-white flex-1 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            Import Dashboard
          </button>
          <button onClick={onCancel} className="neo-button-secondary px-6">
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}
