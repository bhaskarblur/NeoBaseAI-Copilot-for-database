import { useEffect, useState } from 'react';
import { AlertCircle, BarChart3, Loader, PencilRuler, RefreshCcw } from 'lucide-react';
import toast from 'react-hot-toast';
import chatService from '../../services/chatService';
import ChartRenderer from '../ChartRenderer';
import { QueryResult } from '../../types/query';

interface VisualizationState {
    loading: boolean;
    error: string | null;
    isGenerating: boolean;
    notSupported?: boolean;
}

interface VisualizationPanelProps {
    chatId: string;
    messageId: string;
    query: QueryResult;
    userId?: string;
    userName?: string;
    /** Initial visualization value pulled from the message (may be undefined). */
    initialVisualization?: any;
    /** Called when a new visualization is saved so the parent can persist it on the message. */
    onVisualizationSaved?: (queryId: string, vizData: any) => void;
}

const toastStyle = {
    style: {
        background: '#000', color: '#fff', border: '4px solid #000', borderRadius: '12px',
        boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)', padding: '12px 24px',
        fontSize: '14px', fontWeight: '500',
    },
    position: 'bottom-center' as const,
    duration: 2000,
};

export default function VisualizationPanel({
    chatId,
    messageId,
    query,
    userId,
    userName,
    initialVisualization,
    onVisualizationSaved,
}: VisualizationPanelProps) {
    const [vizState, setVizState] = useState<VisualizationState>(() => {
        if (initialVisualization?.can_visualize === false) {
            return {
                loading: false,
                error: initialVisualization.reason || 'Visualization not supported for this data',
                isGenerating: false,
                notSupported: true,
            };
        }
        if (initialVisualization?.can_visualize) {
            return { loading: false, error: null, isGenerating: false };
        }
        return { loading: false, error: null, isGenerating: false };
    });

    const [visualization, setVisualization] = useState<any>(
        initialVisualization?.can_visualize && initialVisualization?.chart_data?.length > 0
            ? initialVisualization
            : null,
    );
    const [dataLoading, setDataLoading] = useState(false);
    const [dataError, setDataError] = useState('');

    // Sync when the parent message updates the query's visualization field
    useEffect(() => {
        if (!initialVisualization) return;
        if (initialVisualization.can_visualize === false) {
            setVizState({
                loading: false,
                error: initialVisualization.reason || 'Visualization not supported for this data',
                isGenerating: false,
                notSupported: true,
            });
        } else if (initialVisualization.can_visualize && initialVisualization.chart_data?.length > 0) {
            setVisualization(initialVisualization);
            setVizState({ loading: false, error: null, isGenerating: false });
        }
    }, [initialVisualization]);

    const handleGenerate = async () => {
        setVizState({ loading: false, error: null, isGenerating: true });
        try {
            const response = await chatService.generateVisualization(chatId, messageId, query.id);
            if (response?.success && response?.data) {
                if (response.data.can_visualize === false) {
                    const reason = response.data.reason || 'This query result cannot be visualized.';
                    setVizState({ loading: false, error: reason, isGenerating: false, notSupported: true });
                    return;
                }
                if (!response.data.chart_data || response.data.chart_data.length === 0) {
                    setVizState({ loading: false, error: 'No data available for visualization.', isGenerating: false });
                    return;
                }
                setVisualization(response.data);
                setVizState({ loading: false, error: null, isGenerating: false });
                onVisualizationSaved?.(query.id, response.data);
                toast('Visualization generated successfully!', { ...toastStyle, icon: '📊' });
            } else {
                const msg = response?.data?.reason || 'Failed to generate visualization.';
                setVizState({ loading: false, error: msg, isGenerating: false });
                toast.error(`Couldn't generate visualization: ${msg}`);
            }
        } catch (error: any) {
            const msg = error.message || 'Failed to generate visualization.';
            setVizState({ loading: false, error: msg, isGenerating: false });
            toast.error(`Visualization error: ${msg}`);
        }
    };

    const handleLoadData = async () => {
        if (!query.visualization) return;
        setDataLoading(true);
        setDataError('');
        try {
            const response = await chatService.getVisualizationData(chatId, messageId, query.id);
            if (response?.success && response?.data) {
                setVisualization((prev: any) => ({
                    ...prev,
                    chart_data: response.data.chart_data,
                    total_records: response.data.total_records,
                    returned_count: response.data.returned_count,
                    updated_at: response.data.updated_at,
                }));
            } else {
                setDataError(response?.data?.reason || 'Failed to load visualization data');
            }
        } catch (error: any) {
            setDataError(error.message || 'Failed to load visualization data');
        } finally {
            setDataLoading(false);
        }
    };

    // ── Rendering ──────────────────────────────────────────────────────────────

    if (vizState.isGenerating) {
        return (
            <div className="flex flex-row items-center justify-center py-12 gap-3">
                <Loader className="w-6 h-6 animate-spin text-gray-400" />
                <p className="text-gray-400">Generating visualization...</p>
            </div>
        );
    }

    if (vizState.error) {
        if (vizState.notSupported) {
            return (
                <div className="bg-gray-800/50 rounded-lg p-8 text-center border border-gray-700">
                    <div className="flex justify-center mb-4">
                        <AlertCircle className="w-8 h-8 text-gray-400" />
                    </div>
                    <div className="text-white font-semibold mb-2 text-base">Visualization Not Supported</div>
                    <p className="text-gray-400 text-sm mb-4">{vizState.error}</p>
                </div>
            );
        }
        return (
            <div className="bg-neo-error/10 rounded-lg p-8 text-center">
                <div className="text-neo-error font-semibold mb-3">Couldn't generate visualization</div>
                <p className="text-gray-400 mb-6">{vizState.error}</p>
                <button onClick={handleGenerate} className="px-4 py-2 bg-white text-gray-900 font-semibold rounded hover:bg-gray-200 transition-colors">
                    <RefreshCcw className="w-4 h-4 inline-block mr-2" />
                    Try Again
                </button>
            </div>
        );
    }

    if (visualization?.chart_data?.length > 0) {
        return (
            <div className="w-full" data-query-id={query.id}>
                <ChartRenderer
                    config={visualization.chart_configuration}
                    data={visualization.chart_data}
                    onRetry={handleGenerate}
                    onRegenerate={handleGenerate}
                    updatedAt={visualization.updated_at}
                />
            </div>
        );
    }

    // Visualization metadata exists but data not yet loaded
    if (query.visualization?.can_visualize && !visualization?.chart_data) {
        return (
            <div className="bg-green-900/20 rounded-lg p-8 text-center border border-green-700/40">
                <div className="mb-6">
                    <BarChart3 className="w-8 h-8 text-green-400 mx-auto mb-4" />
                    <p className="text-white text-base font-semibold mb-2">Visualization Ready</p>
                    <p className="text-gray-400 text-sm">Click the button below to load the visualization</p>
                </div>
                <button
                    onClick={handleLoadData}
                    disabled={dataLoading}
                    className="px-4 py-2 bg-green-600 text-white font-semibold rounded hover:bg-green-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                >
                    {dataLoading ? (
                        <><Loader className="w-4 h-4 inline-block mr-2 animate-spin" />Loading...</>
                    ) : (
                        <><PencilRuler className="w-4 h-4 inline-block mr-2" />Load Visualization</>
                    )}
                </button>
                {dataError && <div className="text-red-400 text-xs mt-3">{dataError}</div>}
            </div>
        );
    }

    // No visualization yet — prompt to generate
    return (
        <div className="bg-blue-900/20 rounded-lg p-8 text-center border border-blue-700/40">
            <div className="mb-6">
                <PencilRuler className="w-8 h-8 text-blue-400 mx-auto mb-4" />
                <p className="text-white text-base font-semibold mb-2">Visualize Your Query Results</p>
                <p className="text-gray-400 text-sm">
                    AI will analyze your data and generate the <br />most suitable visualization
                </p>
            </div>
            <button onClick={handleGenerate} className="px-4 py-2 bg-blue-600 text-white font-semibold rounded hover:bg-blue-700 transition-colors">
                <PencilRuler className="w-4 h-4 inline-block mr-2" />
                Generate Visualization
            </button>
        </div>
    );
}
