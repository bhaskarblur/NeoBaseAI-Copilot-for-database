import { Send, X, LucideDices, Mic, Square } from 'lucide-react';
import { FormEvent, useState, useEffect } from 'react';
import toast from 'react-hot-toast';
import chatService from '../../services/chatService';
import RecommendationTooltip from './RecommendationTooltip';
import { recommendationStorage } from '../../utils/recommendationStorage';
import analyticsService from '../../services/analyticsService';
import VoiceMode from './VoiceMode';

interface Recommendation {
    text: string;
}

interface MessageInputProps {
    isConnected: boolean;
    onSendMessage: (content: string) => Promise<void>;
    isExpanded: boolean;
    isDisabled?: boolean;
    chatId?: string;
    userId?: string;
    userName?: string;
    isVoiceMode?: boolean;
    onVoiceModeChange?: (isActive: boolean) => void;
    voiceSteps?: string[];
    currentVoiceStep?: string;
    onResetVoiceSteps?: () => void;
    isStreaming?: boolean;
    onCancelStream?: () => void;
}

export default function MessageInput({ isConnected, onSendMessage, isExpanded, chatId, userId, userName, isVoiceMode, onVoiceModeChange, voiceSteps: externalVoiceSteps, currentVoiceStep: externalCurrentVoiceStep, onResetVoiceSteps, isStreaming, onCancelStream }: MessageInputProps) {
    const [input, setInput] = useState('');
    const [isLoadingRecommendations, setIsLoadingRecommendations] = useState(false);
    const [recommendations, setRecommendations] = useState<Recommendation[]>([]);
    const [showTooltip, setShowTooltip] = useState(false);
    const [isVoiceActive, setIsVoiceActive] = useState(false);
    const [voiceCancelSeq, setVoiceCancelSeq] = useState(0);

    // Check if tooltip should be shown for new chats
    useEffect(() => {
        if (chatId && isConnected && !recommendationStorage.hasShownTooltip(chatId)) {
            // Show tooltip after a short delay to ensure UI is ready
            const timer = setTimeout(() => {
                setShowTooltip(true);
            }, 1500);
            
            return () => clearTimeout(timer);
        }
    }, [chatId, isConnected]);

    const handleSubmit = (e: FormEvent) => {
        e.preventDefault();
        if (input.trim()) {
            // Track message submit
            if (chatId && userId && userName) {
                analyticsService.trackMessageSubmit(chatId, input.trim().length, userId, userName);
            }
            onSendMessage(input.trim());
            setInput('');
            setRecommendations([]); // Clear recommendations after sending
        }
    };

    const handleGetRecommendations = async () => {
        if (!chatId || !isConnected || isLoadingRecommendations) return;

        // Track dice click
        if (userId && userName) {
            analyticsService.trackRecommendationDiceClick(chatId, userId, userName);
        }

        // Hide tooltip when dice is clicked
        if (showTooltip) {
            setShowTooltip(false);
            recommendationStorage.markTooltipAsShown(chatId);
        }

        // If recommendations are already shown, hide them
        if (recommendations.length > 0) {
            setRecommendations([]);
            return;
        }

        setIsLoadingRecommendations(true);
        try {
            const data = await chatService.getQueryRecommendations(chatId);

            if (data.success && data.data?.recommendations) {
                setRecommendations(data.data.recommendations);
            } else {
                console.error('Failed to get recommendations:', data);
                toast.error('Failed to get query recommendations', {
                    style: {
                        background: '#ff4444',
                        color: '#fff',
                        border: '4px solid #cc0000',
                        borderRadius: '12px',
                        fontSize: '16px',
                        fontWeight: 'bold',
                        padding: '16px',
                    },
                });
            }
        } catch (error: any) {
            console.error('Error fetching recommendations:', error);
            toast.error('Failed to get query recommendations: ' + (error.message || 'Unknown error'), {
                style: {
                    background: '#ff4444',
                    color: '#fff',
                    border: '4px solid #cc0000',
                    borderRadius: '12px',
                    fontSize: '16px',
                    fontWeight: 'bold',
                    padding: '16px',
                },
            });
        } finally {
            setIsLoadingRecommendations(false);
        }
    };

    const handleChipClick = (recommendationText: string, index: number) => {
        // Track recommendation chip click
        if (chatId && userId && userName) {
            analyticsService.trackRecommendationChipClick(chatId, recommendationText, userId, userName);
        }
        
        setInput(recommendationText);
        // Remove only the clicked recommendation from the list
        setRecommendations(prev => prev.filter((_, i) => i !== index));
    };

    const handleTooltipClose = () => {
        // Track tooltip close
        if (chatId && userId && userName) {
            analyticsService.trackTooltipClose(chatId, userId, userName);
        }
        
        setShowTooltip(false);
        if (chatId) {
            recommendationStorage.markTooltipAsShown(chatId);
        }
    };

    const handleTooltipDiceClick = () => {
        setShowTooltip(false);
        if (chatId) {
            recommendationStorage.markTooltipAsShown(chatId);
        }
        handleGetRecommendations();
    };

    const handleVoiceToggle = () => {
        if (!isVoiceActive) {
            setIsVoiceActive(true);
            onVoiceModeChange?.(true);
        } else {
            setIsVoiceActive(false);
            onVoiceModeChange?.(false);
            // Increment cancel sequence to tell VoiceMode to fully release mic
            setVoiceCancelSeq(s => s + 1);
        }
    };

    // Voice mode effects
    useEffect(() => {
        if (isVoiceMode && !isVoiceActive) {
            setIsVoiceActive(true);
        } else if (!isVoiceMode && isVoiceActive) {
            setIsVoiceActive(false);
        }
    }, [isVoiceMode, isVoiceActive]);


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
                {/* Recommendations chips */}
                {recommendations.length > 0 && (
                    <div className="mb-3 flex flex-wrap gap-2 items-center">
                        <span className="text-sm text-gray-600 font-medium">💡 Try asking:</span>
                        {recommendations.map((rec, index) => (
                            <button
                                key={index}
                                onClick={() => handleChipClick(rec.text, index)}
                                className="
                                    inline-flex items-center gap-2 px-3 py-2 
                                    bg-gray-100 hover:bg-gray-200 
                                    border-2 border-gray-300 hover:border-gray-400
                                    rounded-full text-sm font-medium text-black
                                    transition-all duration-200
                                    max-w-base truncate
                                "
                                title={rec.text}
                            >
                                <span className="truncate">{rec.text}</span>
                            </button>
                        ))}

                    </div>
                )}

                <div className="flex gap-4 justify-center relative">
                    <div className="relative flex-1">
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
                                isLoadingRecommendations
                                    ? "Recommending queries for you..."
                                    : isConnected
                                        ? "Ask what you want.."
                                        : "You are not connected to your database..."
                            }
                            className="
                    neo-input 
                    w-full
                    min-h-[52px]
                    resize-y
                    py-3
                    px-4
                    pr-24
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
                            disabled={!isConnected || isLoadingRecommendations || isVoiceActive}
                        />
                        {/* Microphone button */}
                        <button
                            type="button"
                            onClick={handleVoiceToggle}
                            disabled={!isConnected}
                            className={`
                        absolute right-12 top-2.5
                        p-2 rounded-lg
                        hover:bg-gray-100 
                        disabled:opacity-50 disabled:cursor-not-allowed
                        transition-colors duration-200
                        flex items-center justify-center
                        hover-tooltip
                        z-10
                        ${isVoiceActive ? 'bg-neo-gray text-green-500' : 'text-gray-600'}
                    `}
                            data-tooltip={isVoiceActive ? "Exit voice mode" : "Enter voice mode"}
                            aria-label={isVoiceActive ? "Exit voice mode" : "Enter voice mode"}
                        >
                            {isVoiceActive ? (
                                <Mic className="w-5 h-5 text-green-500" />
                            ) : (
                                <Mic className="w-5 h-5" />
                            )}
                        </button>

                        {/* Dice button */}
                        <button
                            type="button"
                            onClick={handleGetRecommendations}
                            disabled={!isConnected || isLoadingRecommendations || isVoiceActive}
                            className={`
                        absolute right-3 top-2.5
                        p-2 rounded-lg
                        hover:bg-gray-100 
                        disabled:opacity-50 disabled:cursor-not-allowed
                        transition-colors duration-200
                        flex items-center justify-center
                        hover-tooltip
                        ${recommendations.length > 0 ? 'bg-gray-200' : ''}
                    `}
                            data-tooltip={recommendations.length > 0 ? "Hide recommendations" : "Get query recommendations"}
                        >
                            {recommendations.length > 0 ? (
                                <X className={`w-5 h-5 text-gray-600`} />
                            ) : (
                                <LucideDices
                                    className={`w-5 h-5 text-gray-600 ${isLoadingRecommendations ? 'animate-spin' : ''}`}
                                />
                            )}
                            
                            {/* Recommendation Tooltip positioned relative to dice button */}
                            <RecommendationTooltip
                                isVisible={showTooltip}
                                onClose={handleTooltipClose}
                                onDiceClick={handleTooltipDiceClick}
                            />
                        </button>
                    </div>
                    <button
                        type={isStreaming ? "button" : "submit"}
                        onClick={isStreaming ? () => { onCancelStream?.(); setVoiceCancelSeq(s => s + 1); } : undefined}
                        className={`neo-button px-4 md:px-6 self-end mb-1.5`}
                        disabled={(!isConnected || isLoadingRecommendations || isVoiceActive) && !isStreaming}
                        title={isStreaming ? "Cancel request" : "Send message"}
                    >
                        {isStreaming ? (
                            <Square className="w-6 h-8 fill-current" />
                        ) : (
                            <Send className="w-6 h-8" />
                        )}
                    </button>

                    {/* Voice Mode Component - expands upward from input field top */}
                    <div className={`absolute bottom-full left-0 right-0 z-50 transition-all duration-600 ease-out origin-bottom ${
                        isVoiceActive 
                            ? 'opacity-100 scale-y-100 translate-y-0' 
                            : 'opacity-0 scale-y-0 translate-y-4 pointer-events-none'
                    }`}>
                        <VoiceMode
                            isOpen={isVoiceActive}
                            onClose={handleVoiceToggle}
                            onSendMessage={onSendMessage}
                            voiceSteps={externalVoiceSteps}
                            currentVoiceStep={externalCurrentVoiceStep}
                            onResetVoiceSteps={onResetVoiceSteps}
                            resetOnCancelKey={voiceCancelSeq}
                        />
                    </div>
                </div>
            </div>
        </form>
    );
}