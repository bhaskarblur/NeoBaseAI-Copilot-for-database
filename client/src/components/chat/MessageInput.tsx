import { Send, Square } from 'lucide-react';
import { FormEvent, useState, useEffect } from 'react';
import analyticsService from '../../services/analyticsService';

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
}

export default function MessageInput({ isConnected, onSendMessage, isExpanded, chatId, userId, userName, isStreaming, onCancelStream, prefillText, onConsumePrefill }: MessageInputProps) {
    const [input, setInput] = useState('');
    const [isLoadingRecommendations] = useState(false);

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

                        {/* Dice button removed: recommendations now in ChatWindow */}
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