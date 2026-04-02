import {
  BarChart3,
  Check,
  FileJson,
  Loader2,
  PieChart,
  Plus,
  RefreshCcw,
  Send,
  Sparkles,
  Table2,
  TrendingUp,
  Upload,
  X,
  AreaChart,
  BarChart,
  LineChart,
  Bot,
  PercentCircle,
  LucideSparkles,
  BarChart3Icon,
} from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { DashboardBlueprint, WidgetType } from '../../types/dashboard';

// Widget type → icon mapping for blueprint cards
const WIDGET_TYPE_ICONS: Record<WidgetType, React.ElementType> = {
  stat: TrendingUp,
  line: LineChart,
  bar: BarChart,
  area: AreaChart,
  pie: PieChart,
  table: Table2,
  combo: BarChart3,
  gauge: PercentCircle,
  bar_gauge: PercentCircle,
  heatmap: LucideSparkles,
  histogram: BarChart3Icon,
};

// ============================================================================
// Blueprint Picker Modal
// ============================================================================
interface BlueprintPickerModalProps {
  blueprints: DashboardBlueprint[];
  selectedIndices: Set<number>;
  isCreating: boolean;
  onToggleSelection: (index: number) => void;
  onCreate: () => void;
  onClose: () => void;
}

export function BlueprintPickerModal({
  blueprints,
  selectedIndices,
  isCreating,
  onToggleSelection,
  onCreate,
  onClose,
}: Readonly<BlueprintPickerModalProps>) {
  return (
    <div className="fixed inset-0 z-[110] flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
      <div className="bg-white neo-border rounded-lg w-full max-w-2xl max-h-[80vh] overflow-hidden flex flex-col">
        {/* Header */}
        <div className="flex justify-between items-center p-6 border-b-4 border-black">
          <div className="flex items-center gap-3">
            <div className="flex items-center justify-center">
              <Sparkles className="w-5 h-5 text-black" />
            </div>
            <div>
              <h2 className="text-2xl font-bold">Choose Your Dashboards</h2>
              <p className="text-sm text-gray-500 mt-0.5">
                Select which dashboards to create. You can always add more later.
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="hover:bg-neo-gray rounded-lg p-2 transition-colors"
          >
            <X className="w-6 h-6" />
          </button>
        </div>

        {/* Blueprint Cards */}
        <div className="flex-1 overflow-y-auto p-6 space-y-4">
          {blueprints.map((bp) => {
            const isSelected = selectedIndices.has(bp.index);
            return (
              <button
                key={bp.index}
                onClick={() => onToggleSelection(bp.index)}
                className={`
                  w-full text-left p-5 rounded-xl border-3 transition-all duration-150
                  ${isSelected
                    ? 'border-2 border-black bg-[#FFDB58]/20'
                    : 'border-2 border-gray-200 bg-white hover:border-gray-400 hover:shadow-[2px_2px_0px_0px_rgba(0,0,0,0.1)]'}
                `}
              >
                <div className="flex items-start justify-between gap-4">
                  <div className="flex-1 min-w-0">
                    {/* Type badge */}
                    <span
                      className={`
                        inline-flex items-center gap-1 text-xs font-black uppercase px-2 py-1 rounded-md mb-2
                        ${isSelected ? 'bg-[#FFD700] text-black border border-black' : 'bg-gray-100 text-gray-600 border border-gray-200'}
                      `}
                    >
                      {bp.template_type}
                    </span>
                    {/* Name */}
                    <h3 className="font-black text-black text-lg mb-1">{bp.name}</h3>
                    {/* Description */}
                    <p className="text-sm text-gray-600 leading-relaxed line-clamp-2">
                      {bp.description}
                    </p>
                    {/* Widget chips with icons */}
                    <div className="flex flex-wrap gap-2 mt-3">
                      {bp.proposed_widgets.slice(0, 6).map((w, i) => {
                        const IconComp = WIDGET_TYPE_ICONS[w.widget_type] || BarChart3;
                        return (
                          <span
                            key={i}
                            className={`
                              inline-flex items-center gap-1.5 text-xs font-semibold px-2.5 py-1 rounded-lg
                              ${isSelected
                                ? 'bg-white text-black border border-black/20'
                                : 'bg-gray-50 text-gray-600 border border-gray-200'}
                            `}
                          >
                            <IconComp className="w-3 h-3" />
                            {w.title}
                          </span>
                        );
                      })}
                      {bp.proposed_widgets.length > 6 && (
                        <span className="text-xs font-semibold px-2.5 py-1 rounded-lg bg-gray-50 text-gray-400 border border-gray-200">
                          +{bp.proposed_widgets.length - 6} more
                        </span>
                      )}
                    </div>
                  </div>
                  {/* Checkbox */}
                  <div
                    className={`
                      w-7 h-7 rounded-lg border-3 flex items-center justify-center flex-shrink-0 mt-1
                      transition-all duration-150
                      ${isSelected
                        ? 'border-2 border-black bg-green-600'
                        : 'border-2 border-gray-400 bg-white'}
                    `}
                  >
                    {isSelected && <Check className="w-4 h-4 text-white" />}
                  </div>
                </div>
              </button>
            );
          })}
        </div>

        {/* Footer */}
        <div className="flex gap-4 p-6 border-t-4 border-black bg-gray-50/50">
          <button
            onClick={onCreate}
            disabled={selectedIndices.size === 0 || isCreating}
            className="neo-border bg-black disabled:opacity-50 disabled:cursor-not-allowed text-white px-4 py-2.5 font-bold text-base transition-all hover:translate-y-[-2px] hover:shadow-[6px_6px_0px_0px_rgba(0,0,0,1)] active:translate-y-[0px] active:shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] flex-1"
          >
            {isCreating ? (
              <div className="flex items-center justify-center gap-2">
                <Loader2 className="w-4 h-4 animate-spin" />
                <span>Creating...</span>
              </div>
            ) : (
              <div className="flex items-center justify-center gap-2">
                <Sparkles className="w-4 h-4" />
                <span>
                  Create{' '}
                  {selectedIndices.size > 1
                    ? `${selectedIndices.size} Dashboards`
                    : 'Dashboard'}
                </span>
              </div>
            )}
          </button>
          <button onClick={onClose} className="neo-button-secondary flex-1">
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}

// ============================================================================
// Create with AI Prompt Modal
// ============================================================================
interface PromptModalProps {
  onSubmit: (prompt: string) => void;
  onClose: () => void;
}

export function CreateDashboardPromptModal({
  onSubmit,
  onClose,
}: Readonly<PromptModalProps>) {
  const [prompt, setPrompt] = useState('');
  const inputRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    setTimeout(() => inputRef.current?.focus(), 100);
  }, []);

  return (
    <div className="fixed inset-0 z-[110] flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
      <div className="bg-white neo-border rounded-lg w-full max-w-lg">
        {/* Header */}
        <div className="flex justify-between items-center p-6 border-b-4 border-black">
          <div className="flex items-center gap-3">
            <div className="flex items-center justify-center">
              <Sparkles className="w-5 h-5 text-black" />
            </div>
            <div>
              <h2 className="text-2xl font-bold">Customize with AI</h2>
              <p className="text-sm text-gray-500 mt-0.5">Describe what dashboard you want to visualize</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="hover:bg-neo-gray rounded-lg p-2 transition-colors"
          >
            <X className="w-6 h-6" />
          </button>
        </div>

        {/* Body */}
        <div className="p-6">
          <label className="block text-base font-bold text-black mb-2">
            What Insights Do You Want To See?
          </label>
          <textarea
            ref={inputRef}
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            placeholder='e.g., "Show me daily revenue trends, top products, and customer growth"'
            className="
              w-full h-32 px-4 py-3 text-base
              border-2 border-black rounded-xl
              focus:outline-none focus:ring-2 focus:ring-black focus:ring-offset-1
              resize-none placeholder:text-gray-400
              shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]
            "
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey && prompt.trim()) {
                e.preventDefault();
                onSubmit(prompt.trim());
              }
            }}
          />
          <p className="text-sm text-gray-600 mt-2">
            The AI will generate a full dashboard with widgets, queries, and chart configurations.
          </p>
        </div>

        {/* Footer */}
        <div className="flex gap-4 p-6 border-t-4 border-black bg-gray-50/50">
          <button
            onClick={() => prompt.trim() && onSubmit(prompt.trim())}
            disabled={!prompt.trim()}
            className="neo-border bg-black disabled:opacity-50 disabled:cursor-not-allowed text-white px-4 py-2.5 font-bold text-base transition-all hover:translate-y-[-2px] hover:shadow-[6px_6px_0px_0px_rgba(0,0,0,1)] active:translate-y-[0px] active:shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] flex-1"
          >
            <div className="flex items-center justify-center gap-2">
              <Sparkles className="w-4 h-4" />
              <span>Generate</span>
            </div>
          </button>
          <button onClick={onClose} className="neo-button-secondary flex-1">
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}

// ============================================================================
// Add Widget Modal — ConnectionModal style
// ============================================================================
interface AddWidgetModalProps {
  isAdding: boolean;
  onSubmit: (prompt: string) => void;
  onClose: () => void;
}

export function AddWidgetModal({
  isAdding,
  onSubmit,
  onClose,
}: Readonly<AddWidgetModalProps>) {
  const [prompt, setPrompt] = useState('');
  const inputRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    setTimeout(() => inputRef.current?.focus(), 100);
  }, []);

  return (
    <div className="fixed inset-0 z-[110] flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
      <div className="bg-white neo-border rounded-lg w-full max-w-lg">
        {/* Header */}
        <div className="flex justify-between items-center p-6 border-b-4 border-black">
          <div className="flex items-center gap-3">
            <div className="flex items-center justify-center">
              <Plus className="w-5 h-5 text-black" />
            </div>
            <div>
              <h2 className="text-2xl font-bold">Add Widget</h2>
              <p className="text-sm text-gray-500 mt-0.5">Describe the widget you want to add</p>
            </div>
          </div>
          <button
            onClick={onClose}
            disabled={isAdding}
            className="hover:bg-neo-gray rounded-lg p-2 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <X className="w-6 h-6" />
          </button>
        </div>

        {/* Body */}
        <div className="p-6">
          <label className="block text-base font-bold text-black mb-2">
            Tell The AI What Widget To Add
          </label>
          <textarea
            ref={inputRef}
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            placeholder='e.g., "Show total revenue this month", "Top 10 users by orders", "Daily signups chart"'
            className="
              w-full h-32 px-4 py-3 text-base
              border-2 border-black rounded-xl
              focus:outline-none focus:ring-2 focus:ring-black focus:ring-offset-1
              resize-none placeholder:text-gray-400
              shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]
            "
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey && prompt.trim() && !isAdding) {
                e.preventDefault();
                onSubmit(prompt.trim());
              }
            }}
          />
          <p className="text-sm text-gray-600 mt-2">
            The AI will generate a widget with the query, chart type, and configuration automatically.
          </p>
        </div>

        {/* Footer */}
        <div className="flex gap-4 p-6 border-t-4 border-black bg-gray-50/50">
          <button
            onClick={() => prompt.trim() && !isAdding && onSubmit(prompt.trim())}
            disabled={!prompt.trim() || isAdding}
            className="neo-border bg-black disabled:opacity-50 disabled:cursor-not-allowed text-white px-4 py-2.5 font-bold text-base transition-all hover:translate-y-[-2px] hover:shadow-[6px_6px_0px_0px_rgba(0,0,0,1)] active:translate-y-[0px] active:shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] flex-1"
          >
            {isAdding ? (
              <div className="flex items-center justify-center gap-2">
                <Loader2 className="w-4 h-4 animate-spin" />
                <span>Adding Widget...</span>
              </div>
            ) : (
              <div className="flex items-center justify-center gap-2">
                <Plus className="w-4 h-4" />
                <span className="hidden sm:inline">Add Widget</span>
                <span className="sm:hidden">Add</span>
              </div>
            )}
          </button>
          <button 
            onClick={onClose} 
            disabled={isAdding}
            className="neo-button-secondary flex-1 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}

// ============================================================================
// Edit Widget Modal — ConnectionModal style
// ============================================================================
interface EditWidgetModalProps {
  widgetTitle: string;
  isEditing: boolean;
  onSubmit: (prompt: string) => void;
  onClose: () => void;
}

export function EditWidgetModal({
  widgetTitle,
  isEditing,
  onSubmit,
  onClose,
}: Readonly<EditWidgetModalProps>) {
  const [prompt, setPrompt] = useState('');
  const inputRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    setTimeout(() => inputRef.current?.focus(), 100);
  }, []);

  return (
    <div className="fixed inset-0 z-[110] flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
      <div className="bg-white neo-border rounded-lg w-full max-w-lg">
        {/* Header */}
        <div className="flex justify-between items-center p-6 border-b-4 border-black">
          <div className="flex items-center gap-3">
            <div className="flex items-center justify-center">
              <Sparkles className="w-5 h-5 text-black" />
            </div>
            <div>
              <h2 className="text-2xl font-bold">Edit Widget</h2>
              <p className="text-sm text-gray-500 mt-0.5">
                Editing: <span className="font-semibold text-black">{widgetTitle}</span>
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            disabled={isEditing}
            className="hover:bg-neo-gray rounded-lg p-2 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <X className="w-6 h-6" />
          </button>
        </div>

        {/* Body */}
        <div className="p-6">
          <label className="block text-base font-bold text-black mb-2">
            How Should This Widget Change?
          </label>
          <textarea
            ref={inputRef}
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            placeholder='e.g., "Change to a pie chart", "Show last 30 days instead of 7", "Add a trend line"'
            className="
              w-full h-32 px-4 py-3 text-base
              border-2 border-black rounded-xl
              focus:outline-none focus:ring-2 focus:ring-black focus:ring-offset-1
              resize-none placeholder:text-gray-400
              shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]
            "
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey && prompt.trim() && !isEditing) {
                e.preventDefault();
                onSubmit(prompt.trim());
              }
            }}
          />
        </div>

        {/* Footer */}
        <div className="flex gap-4 p-6 border-t-4 border-black bg-gray-50/50">
          <button
            onClick={() => prompt.trim() && !isEditing && onSubmit(prompt.trim())}
            disabled={!prompt.trim() || isEditing}
            className="neo-border bg-black disabled:opacity-50 disabled:cursor-not-allowed text-white px-4 py-2.5 font-bold text-base transition-all hover:translate-y-[-2px] hover:shadow-[6px_6px_0px_0px_rgba(0,0,0,1)] active:translate-y-[0px] active:shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] flex-1"
          >
            {isEditing ? (
              <div className="flex items-center justify-center gap-2">
                <Loader2 className="w-4 h-4 animate-spin" />
                <span>Updating...</span>
              </div>
            ) : (
              <div className="flex items-center justify-center gap-2">
                <Sparkles className="w-4 h-4" />
                <span className="hidden sm:inline">Update Widget</span>
                <span className="sm:hidden">Update</span>
              </div>
            )}
          </button>
          <button 
            onClick={onClose} 
            disabled={isEditing}
            className="neo-button-secondary flex-1 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}

// ============================================================================
// Regenerate Dashboard Modal
// ============================================================================
type RegenerateReason = 'try_another_variant' | 'schema_changed';

interface RegenerateDashboardModalProps {
  onSubmit: (reason: RegenerateReason, customInstructions?: string) => void;
  onClose: () => void;
}

const REGENERATE_OPTIONS: { value: RegenerateReason; label: string; description: string; icon: React.ReactNode }[] = [
  {
    value: 'try_another_variant',
    label: 'Try Another Alternative',
    description: 'Generate a fresh set of widgets with different metrics and visualizations',
    icon: <Sparkles className="w-5 h-5" />,
  },
  {
    value: 'schema_changed',
    label: 'Apply New Schema Changes',
    description: 'Rebuild dashboard based on updated database tables and columns',
    icon: <RefreshCcw className="w-5 h-5" />,
  },
];

export function RegenerateDashboardModal({
  onSubmit,
  onClose,
}: Readonly<RegenerateDashboardModalProps>) {
  const [selectedReason, setSelectedReason] = useState<RegenerateReason | null>(null);
  const [customInstructions, setCustomInstructions] = useState('');
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  return (
    <div className="fixed inset-0 z-[110] flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
      <div className="bg-white neo-border rounded-lg w-full max-w-lg">
        {/* Header */}
        <div className="flex justify-between items-center p-6 border-b-4 border-black">
          <div className="flex items-center gap-3">
            <div className="flex items-center justify-center">
              <RefreshCcw className="w-5 h-5 text-black" />
            </div>
            <div>
              <h2 className="text-2xl font-bold">Regenerate Dashboard</h2>
              <p className="text-sm text-gray-500 mt-0.5">Choose a reason for regeneration</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="hover:bg-neo-gray rounded-lg p-2 transition-colors"
          >
            <X className="w-6 h-6" />
          </button>
        </div>

        {/* Body */}
        <div className="p-6 space-y-4">
          <div className="space-y-3">
            <label className="block text-base font-bold text-black">
              Choose Regeneration Reason
            </label>
            {REGENERATE_OPTIONS.map((option) => (
              <button
                key={option.value}
                onClick={() => setSelectedReason(option.value)}
                className={`
                  w-full text-left p-4 rounded-xl border-2 transition-all
                  ${selectedReason === option.value
                    ? 'border-black bg-[#FFD700]/10 shadow-[3px_3px_0px_0px_rgba(0,0,0,1)]'
                    : 'border-gray-200 hover:border-black hover:shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]'
                  }
                `}
              >
                <div className="flex items-start gap-3">
                  <div className={`mt-0.5 ${selectedReason === option.value ? 'text-black' : 'text-gray-400'}`}>
                    {option.icon}
                  </div>
                  <div>
                    <div className="font-bold text-base">{option.label}</div>
                    <div className="text-sm text-gray-500 mt-0.5">{option.description}</div>
                  </div>
                </div>
              </button>
            ))}
          </div>

          {selectedReason && (
            <div>
              <label className="block text-base font-bold text-black mb-2">
                Any Instructions (Optional)
              </label>
              <textarea
                ref={textareaRef}
                value={customInstructions}
                onChange={(e) => setCustomInstructions(e.target.value)}
                placeholder='e.g., "Focus on user engagement metrics", "Include more time-series charts", "Add filters for date ranges"'
                className="
                  w-full h-24 px-4 py-3 text-base
                  border-2 border-black rounded-xl
                  focus:outline-none focus:ring-2 focus:ring-black focus:ring-offset-1
                  resize-none placeholder:text-gray-400
                  shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]
                "
              />
              <p className="text-sm text-gray-600 mt-1">
                Tell the AI how to customize the regenerated dashboard
              </p>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex gap-4 p-6 border-t-4 border-black bg-gray-50/50">
          <button
            onClick={() => selectedReason && onSubmit(selectedReason, customInstructions.trim() || undefined)}
            disabled={!selectedReason}
            className={`
              neo-border px-4 py-2.5 font-bold text-base transition-all flex-1
              ${selectedReason
                ? 'bg-black text-white hover:translate-y-[-2px] hover:shadow-[6px_6px_0px_0px_rgba(0,0,0,1)] active:translate-y-[0px] active:shadow-[4px_4px_0px_0px_rgba(0,0,0,1)]'
                : 'bg-gray-300 text-gray-500 cursor-not-allowed'
              }
            `}
          >
            <div className="flex items-center justify-center gap-2">
              <Sparkles className="w-4 h-4" />
              <span>Regenerate</span>
            </div>
          </button>
          <button onClick={onClose} className="neo-button-secondary flex-1">
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}

// ============================================================================
// Import Dashboard Modal
// ============================================================================
interface ImportDashboardModalProps {
  isImporting: boolean;
  onSubmit: (jsonContent: string, file?: File) => void;
  onClose: () => void;
}

export function ImportDashboardModal({
  isImporting,
  onSubmit,
  onClose,
}: Readonly<ImportDashboardModalProps>) {
  const [jsonContent, setJsonContent] = useState('');
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [error, setError] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const exampleSchema = `{
  "schemaVersion": "1.0.0",
  "dashboard": {
    "name": "My Dashboard",
    "description": "Dashboard description",
    "widgets": [...],
    "layout": [...]
  }
}`;

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      if (!file.name.endsWith('.json')) {
        setError('Please select a JSON file');
        setSelectedFile(null);
        return;
      }
      setSelectedFile(file);
      setError(null);
      
      // Read file and populate text area
      const reader = new FileReader();
      reader.onload = (event) => {
        const content = event.target?.result as string;
        setJsonContent(content);
      };
      reader.readAsText(file);
    }
  };

  const handleSubmit = async () => {
    setError(null);
    
    if (!jsonContent.trim()) {
      setError('Please paste JSON content or select a file');
      return;
    }
    
    try {
      JSON.parse(jsonContent); // Validate JSON
      onSubmit(jsonContent.trim(), selectedFile || undefined);
    } catch (err) {
      setError('Invalid JSON format');
    }
  };

  return (
    <div className="fixed inset-0 z-[110] flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
      <div className="bg-white neo-border rounded-lg w-full max-w-2xl">
        {/* Header */}
        <div className="flex justify-between items-center p-6 border-b-4 border-black">
          <div className="flex items-center gap-3">
            <div className="flex items-center justify-center">
              <Upload className="w-5 h-5 text-black" />
            </div>
            <div>
              <h2 className="text-2xl font-bold">Import Dashboard</h2>
              <p className="text-sm text-gray-500 mt-0.5">Import a dashboard from JSON file or paste content</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="hover:bg-neo-gray rounded-lg p-2 transition-colors"
          >
            <X className="w-6 h-6" />
          </button>
        </div>

        {/* Body */}
        <div className="p-6 space-y-4">
          {/* JSON Content Text Area - Always Visible */}
          <div>
            <label className="block text-base font-bold text-black mb-2">
              Paste Dashboard JSON
            </label>
            <textarea
              ref={textareaRef}
              value={jsonContent}
              onChange={(e) => {
                setJsonContent(e.target.value);
                setError(null);
              }}
              placeholder={exampleSchema}
              className="
                w-full h-48 px-4 py-3 text-sm font-mono
                border-2 border-black rounded-xl
                focus:outline-none focus:ring-2 focus:ring-black focus:ring-offset-1
                resize-none placeholder:text-gray-400
                shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]
              "
            />
          </div>

          {/* OR Divider */}
          <div className="flex items-center gap-4 my-6">
            <div className="flex-1 h-px bg-gray-300" />
            <span className="text-sm text-gray-500">OR</span>
            <div className="flex-1 h-px bg-gray-300" />
          </div>

          {/* File Upload Section - Always Visible */}
          <div>
            <label className="block text-base font-bold text-black mb-2">
              Upload JSON File
            </label>
            <input
              ref={fileInputRef}
              type="file"
              accept=".json,application/json"
              onChange={handleFileSelect}
              className="hidden"
            />
            <button
              onClick={() => fileInputRef.current?.click()}
              className="
                w-full py-8 px-4 border-2 border-dashed border-gray-300 rounded-xl
                hover:border-black hover:bg-gray-50 transition-all
                flex flex-col items-center justify-center gap-2
              "
            >
              <Upload className="w-8 h-8 text-gray-400" />
              <span className="text-base font-bold text-gray-700">
                Click to select a JSON file
              </span>
              <span className="text-sm text-gray-500">
                or drag and drop here
              </span>
            </button>

            {/* Selected File Info */}
            {selectedFile && (
              <div className="mt-3 px-4 py-3 bg-green-50 border-2 border-green-200 rounded-lg flex items-center gap-3">
                <FileJson className="w-5 h-5 text-green-600" />
                <div className="flex-1">
                  <p className="text-sm font-bold text-green-900">
                    {selectedFile.name}
                  </p>
                  <p className="text-xs text-green-600">
                    {(selectedFile.size / 1024).toFixed(2)} KB
                  </p>
                </div>
                <button
                  onClick={() => {
                    setSelectedFile(null);
                    setJsonContent('');
                    if (fileInputRef.current) {
                      fileInputRef.current.value = '';
                    }
                  }}
                  className="p-1 hover:bg-green-100 rounded transition-colors"
                >
                  <X className="w-4 h-4 text-green-600" />
                </button>
              </div>
            )}
          </div>

          {/* Error Message */}
          {error && (
            <div className="px-4 py-3 bg-red-50 border-2 border-red-300 rounded-lg">
              <p className="text-sm font-semibold text-red-600">{error}</p>
            </div>
          )}

          {/* Info Box */}
          <div className="px-4 py-3 bg-blue-50 border-2 border-blue-200 rounded-lg">
            <p className="text-sm text-gray-700">
              <span className="font-semibold">Note:</span> When you import a dashboard file, the system will try to map your data sources and widget queries to your existing database schema, incompatibility errors may occur if any.
            </p>
          </div>
        </div>

        {/* Footer */}
        <div className="flex gap-4 p-6 border-t-4 border-black bg-gray-50/50">
          <button
            onClick={handleSubmit}
            disabled={isImporting || !jsonContent.trim()}
            className="neo-border bg-black disabled:opacity-50 disabled:cursor-not-allowed text-white px-4 py-2.5 font-bold text-base transition-all hover:translate-y-[-2px] hover:shadow-[6px_6px_0px_0px_rgba(0,0,0,1)] active:translate-y-[0px] active:shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] flex-1"
          >
            {isImporting ? (
              <div className="flex items-center justify-center gap-2">
                <Loader2 className="w-4 h-4 animate-spin" />
                <span>Importing...</span>
              </div>
            ) : (
              <div className="flex items-center justify-center gap-2">
                <Upload className="w-4 h-4" />
                <span>Import Dashboard</span>
              </div>
            )}
          </button>
          <button onClick={onClose} className="neo-button-secondary flex-1">
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}
