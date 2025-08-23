import { Send, Mic, Square } from 'lucide-react';
import { FormEvent, useState, useEffect } from 'react';
import analyticsService from '../../services/analyticsService';
import VoiceMode from './VoiceMode';

// Note: Recommendations UI moved to ChatWindow

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
    prefillText?: string;
    onConsumePrefill?: () => void;
}

export default function MessageInput({ isConnected, onSendMessage, isExpanded, chatId, userId, userName, isVoiceMode, onVoiceModeChange, voiceSteps: externalVoiceSteps, currentVoiceStep: externalCurrentVoiceStep, onResetVoiceSteps, isStreaming, onCancelStream, prefillText, onConsumePrefill }: MessageInputProps) {
    const [input, setInput] = useState('');
    const [isLoadingRecommendations] = useState(false);
    const [isVoiceActive, setIsVoiceActive] = useState(false);
    const [voiceCancelSeq, setVoiceCancelSeq] = useState(0);

    // Apply prefill from ChatWindow chips
    useEffect(() => {
        if (prefillText != null && prefillText !== '') {
            setInput(prefillText);
            onConsumePrefill?.();
        }
    }, [prefillText]);

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

    // Dice-driven recommendations removed; chips now rendered in ChatWindow

    // Chip click now handled in ChatWindow via prefill

    // Legacy tooltip handlers removed

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
                {/* Recommendations moved to ChatWindow */}

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
                                isConnected
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
                            disabled={!isConnected || isVoiceActive}
                        />
                        {/* Microphone button */}
                        <button
                            type="button"
                            onClick={handleVoiceToggle}
                            disabled={!isConnected}
                            className={`
                        absolute right-3 top-2.5
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

                        {/* Dice button removed: recommendations now in ChatWindow */}
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