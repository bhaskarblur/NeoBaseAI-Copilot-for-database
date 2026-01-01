import { Send, Square, ChevronDown, Cpu } from 'lucide-react';
import { FormEvent, useState, useEffect, useRef, useMemo } from 'react';
import analyticsService from '../../services/analyticsService';
import { LLMModel, CategorizedLLMModels } from '../../types/chat';
import axios from 'axios';

// Note: Recommendations UI moved to ChatWindow

interface MessageInputProps {
    isConnected: boolean;
    onSendMessage: (content: string) => Promise<void>;
    isExpanded: boolean;
    isDisabled?: boolean;
    chatId?: string;
    userId?: string;
    userName?: string;
    isStreaming?: boolean;
    onCancelStream?: () => void;
    prefillText?: string;
    onConsumePrefill?: () => void;
    onModelChange?: (modelId: string) => void;
    selectedModel?: string;
}

export default function MessageInput({ 
    isConnected, 
    onSendMessage, 
    isExpanded, 
    chatId, 
    userId, 
    userName, 
    isStreaming, 
    onCancelStream, 
    prefillText, 
    onConsumePrefill,
    onModelChange,
    selectedModel
}: MessageInputProps) {
    const [input, setInput] = useState('');
    const [isLoadingRecommendations] = useState(false);
    const [llmModels, setLlmModels] = useState<LLMModel[]>([]);
    const [showModelDropdown, setShowModelDropdown] = useState(false);
    const [isLoadingModels, setIsLoadingModels] = useState(false);
    const modelDropdownRef = useRef<HTMLDivElement>(null);

    // Categorize models by provider
    const categorizedModels = useMemo(() => {
        const categorized: CategorizedLLMModels = {};
        
        llmModels.forEach(model => {
            if (!categorized[model.provider]) {
                categorized[model.provider] = [];
            }
            categorized[model.provider].push(model);
        });

        return categorized;
    }, [llmModels]);

    // Get provider order (Gemini first, then OpenAI, then others alphabetically)
    const providerOrder = useMemo(() => {
        return Object.keys(categorizedModels).sort((a, b) => {
            const providerPriority: { [key: string]: number } = {
                'gemini': 0,
                'openai': 1,
            };
            return (providerPriority[a] ?? 99) - (providerPriority[b] ?? 99);
        });
    }, [categorizedModels]);

    // Fetch available LLM models
    useEffect(() => {
        const fetchModels = async () => {
            try {
                setIsLoadingModels(true);
                const response = await axios.get(`${import.meta.env.VITE_API_URL}/llm-models`);
                if (response.data.success && response.data.data.models) {
                    setLlmModels(response.data.data.models);
                }
            } catch (error) {
                console.error('Failed to fetch LLM models:', error);
            } finally {
                setIsLoadingModels(false);
            }
        };

        fetchModels();
    }, []);

    // Apply prefill from ChatWindow chips
    useEffect(() => {
        if (prefillText != null && prefillText !== '') {
            setInput(prefillText);
            onConsumePrefill?.();
        }
    }, [prefillText]);

    // Close dropdown when clicking outside
    useEffect(() => {
        const handleClickOutside = (event: MouseEvent) => {
            if (modelDropdownRef.current && !modelDropdownRef.current.contains(event.target as Node)) {
                setShowModelDropdown(false);
            }
        };

        document.addEventListener('mousedown', handleClickOutside);
        return () => document.removeEventListener('mousedown', handleClickOutside);
    }, []);

    const handleSubmit = (e: FormEvent) => {
        e.preventDefault();
        if (input.trim()) {
            // Track message submit
            if (chatId && userId && userName) {
                analyticsService.trackMessageSubmit(chatId, input.trim().length, userId, userName);
            }
            onSendMessage(input.trim());
            setInput('');
        }
    };

    const handleModelChange = (modelId: string) => {
        onModelChange?.(modelId);
        setShowModelDropdown(false);
    };

    const currentModel = selectedModel && llmModels.find(m => m.id === selectedModel);

    // Get provider display name and icon
    const getProviderInfo = (provider: string) => {
        const providerInfo: { [key: string]: { name: string; color: string; bgColor: string; borderColor: string } } = {
            'openai': { name: 'OpenAI', color: 'text-green-700', bgColor: 'bg-green-50', borderColor: 'border-green-300' },
            'gemini': { name: 'Google Gemini', color: 'text-blue-700', bgColor: 'bg-blue-50', borderColor: 'border-blue-300' },
        };
        return providerInfo[provider] || { name: provider, color: 'text-gray-700', bgColor: 'bg-gray-50', borderColor: 'border-gray-300' };
    };

    // Dice-driven recommendations removed; chips now rendered in ChatWindow

    // Chip click now handled in ChatWindow via prefill


    return (
        <form
            onSubmit={handleSubmit}
            className={`
            fixed -bottom-1.5 left-0 right-0 p-4
            bg-white border-t-4 border-black 
            transition-all duration-300
            z-[10]
            ${isExpanded
                    ? `
                    [@media(min-width:1024px)_and_(max-width:1279px)]:ml-[20rem]
                    [@media(min-width:1280px)_and_(max-width:1439px)]:ml-[20rem]
                    [@media(min-width:1440px)_and_(max-width:1700px)]:ml-[18rem]
                    ml-2
                `
                    : 'md:left-[5rem]'
                }
        `}
        >
            <div className="max-w-5xl mx-auto chat-input-1440 relative">
                {/* Recommendations moved to ChatWindow */}

                <div className="flex gap-4 justify-center relative items-end">
                    <div className="relative flex-1">
                        {/* Input container with model selector inside */}
                        <div className="relative">
                            <textarea
                                value={input}
                                onChange={(e) => setInput(e.target.value)}
                                onKeyDown={(e) => {
                                    if (e.key === 'Enter' && !e.shiftKey) {
                                        e.preventDefault();
                                        handleSubmit(e);
                                    }
                                }}
                                placeholder={
                                    isConnected
                                        ? "Ask about your data.."
                                        : "You are not connected to your database..."
                                }
                                className="
                        neo-input 
                        w-full
                        min-h-[52px]
                        resize-y
                        py-3
                        px-4
                        pr-16
                        leading-normal
                        whitespace-pre-wrap
                      "
                                rows={Math.min(
                                    Math.max(
                                        input.split('\n').length,
                                        Math.ceil(input.length / 50)
                                    ),
                                    5
                                )}
                                disabled={!isConnected}
                            />

                            {/* Model Selector - Inside textarea on right edge */}
                            <div ref={modelDropdownRef} className="absolute right-3 top-1/2 transform -translate-y-1/2">
                                <button
                                    type="button"
                                    onClick={() => setShowModelDropdown(!showModelDropdown)}
                                    disabled={isLoadingModels || llmModels.length === 0}
                                    className={`
                                        flex items-center gap-1.5 px-2 py-1.5
                                        bg-transparent hover:bg-gray-100 disabled:opacity-50
                                        transition-colors duration-200 rounded
                                        ${showModelDropdown ? 'bg-gray-100' : ''}
                                    `}
                                    title={currentModel ? `Selected: ${currentModel.displayName}` : 'Select AI Model'}
                                >
                                    {isLoadingModels ? (
                                        <span className="w-4 h-4 border-2 border-gray-400 border-t-transparent rounded-full animate-spin" />
                                    ) : (
                                        <>
                                        <div className='-mt-2 -mr-1 flex flex-row gap-1'>
                                            <Cpu className="w-4 h-4 text-green-700 flex-shrink-0" />
                                            <ChevronDown className={`w-3.5 h-3.5 mt-0.5 text-gray-600 flex-shrink-0 transition-transform ${showModelDropdown ? 'rotate-180' : ''}`} />
                                        </div>
                                        </>
                                    )}
                                </button>

                                {/* Model Dropdown Menu - positioned above textarea */}
                                {showModelDropdown && llmModels.length > 0 && (
                                    <div className="
                                        absolute -right-20 md:right-0 bottom-full mb-7 md:mb-3
                                        bg-white border-2 border-black
                                        shadow-[4px_4px_0px_rgba(0,0,0,1)]
                                        w-[340px] max-h-[380px] overflow-hidden flex flex-col
                                    ">
                                        {/* Dropdown Header */}
                                        <div className="px-4 py-3 border-b-2 border-black bg-gray-100">
                                            <h3 className="text-sm font-bold uppercase tracking-widest text-black">
                                                Select AI Model
                                            </h3>
                                            <p className="text-xs text-gray-700 mt-1">
                                                {llmModels.length} models available
                                            </p>
                                        </div>

                                        {/* Models List */}
                                        <div className="overflow-y-auto flex-1 bg-white">
                                            {/* Show Selected Model at Top */}
                                            {currentModel && (
                                                <>
                                                    <button
                                                        type="button"
                                                        onClick={() => handleModelChange(currentModel.id)}
                                                        className="
                                                            w-full text-left px-4 py-2.5
                                                            border-b-2 border-green-600
                                                            bg-green-100 hover:bg-green-200
                                                            transition-colors duration-150
                                                            flex items-start justify-between gap-3
                                                        "
                                                    >
                                                        <div className="flex-1 min-w-0">
                                                            <span className="font-bold text-sm text-green-900">
                                                                {currentModel.displayName}
                                                            </span>
                                                        </div>
                                                        <div className="flex-shrink-0 pt-1">
                                                            <div className="w-5 h-5 bg-green-500 rounded-full flex items-center justify-center border border-green-600">
                                                                <svg className="w-3 h-3 text-white" fill="currentColor" viewBox="0 0 20 20">
                                                                    <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
                                                                </svg>
                                                            </div>
                                                        </div>
                                                    </button>
                                                </>
                                            )}

                                            {providerOrder.map((provider, providerIndex) => {
                                                const models = categorizedModels[provider];
                                                const providerInfo = getProviderInfo(provider);
                                                
                                                return (
                                                    <div key={provider}>
                                                        {/* Provider Section Header */}
                                                        {providerIndex > 0 && <div className="border-t border-gray-200" />}
                                                        <div className="px-4 py-3 border-b border-gray-200 bg-gray-50">
                                                            <h4 className="text-xs font-bold uppercase tracking-wider text-black">
                                                                {providerInfo.name}
                                                            </h4>
                                                        </div>

                                                        {/* Models for this provider */}
                                                        {models.map((model, modelIndex) => (
                                                            <button
                                                                key={model.id}
                                                                type="button"
                                                                onClick={() => handleModelChange(model.id)}
                                                                className={`
                                                                    w-full text-left px-4 py-2.5
                                                                    border-b border-gray-200
                                                                    transition-colors duration-150
                                                                    flex items-start justify-between gap-3
                                                                    ${modelIndex === models.length - 1 && providerIndex === providerOrder.length - 1 ? 'last:border-b-0' : ''}
                                                                    ${selectedModel === model.id 
                                                                        ? 'bg-white' 
                                                                        : 'bg-transparent hover:bg-gray-200'
                                                                    }
                                                                `}
                                                            >
                                                                <div className="flex-1 min-w-0">
                                                                    <span className={`${selectedModel === model.id ? 'font-bold' : 'font-medium'} text-sm text-black`}>
                                                                        {model.displayName}
                                                                    </span>
                                                                </div>

                                                                {/* Active Indicator - Tick */}
                                                                {selectedModel === model.id && (
                                                                    <div className="flex-shrink-0 pt-1">
                                                                        <div className="w-5 h-5 bg-green-500 rounded-full flex items-center justify-center border border-green-700">
                                                                            <svg className="w-3 h-3 text-white" fill="currentColor" viewBox="0 0 20 20">
                                                                                <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
                                                                            </svg>
                                                                        </div>
                                                                    </div>
                                                                )}
                                                            </button>
                                                        ))}
                                                    </div>
                                                );
                                            })}
                                        </div>
                                    </div>
                                )}
                            </div>
                        </div>
                    </div>

                    <button
                        type={isStreaming ? "button" : "submit"}
                        onClick={isStreaming ? () => { onCancelStream?.(); } : undefined}
                        className={`neo-button px-4 md:px-6 self-end mb-1.5`}
                        disabled={(!isConnected || isLoadingRecommendations) && !isStreaming}
                        title={isStreaming ? "Cancel request" : "Send message"}
                    >
                        {isStreaming ? (
                            <Square className="w-6 h-8 fill-current" />
                        ) : (
                            <Send className="w-6 h-8" />
                        )}
                    </button>
                </div>
            </div>
        </form>
    );
}