import { X } from 'lucide-react';
import { useEffect, useRef } from 'react';

interface RecommendationTooltipProps {
    isVisible: boolean;
    onClose: () => void;
    onDiceClick: () => void;
}

export default function RecommendationTooltip({ isVisible, onClose, onDiceClick }: RecommendationTooltipProps) {
    const tooltipRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        const handleClickOutside = (event: MouseEvent) => {
            if (tooltipRef.current && !tooltipRef.current.contains(event.target as Node)) {
                onClose();
            }
        };

        if (isVisible) {
            document.addEventListener('mousedown', handleClickOutside);
        }

        return () => {
            document.removeEventListener('mousedown', handleClickOutside);
        };
    }, [isVisible, onClose]);

    const handleCloseClick = (e: React.MouseEvent) => {
        e.stopPropagation();
        onClose();
    };

    if (!isVisible) return null;

    return (
        <div
            ref={tooltipRef}
            className="
                absolute bottom-10 right-0 
                bg-white border-2 border-black
                px-3 md:px-4 py-3 rounded-lg shadow-lg
                w-60 md:w-80 z-99
                animate-fade-in text-left
            "
            style={{
                animation: 'fadeIn 0.3s ease-in-out'
            }}
        >
            {/* Arrow pointing to dice button */}
            <div className="absolute bottom-0 right-3 w-0 h-0 border-l-8 border-r-8 border-t-8 border-transparent border-t-black transform translate-y-full"></div>
            
            <div className="flex items-start gap-3">
                <div className="flex-1">
                    <p className="text-base font-bold text-black">
                        Not Sure What To Ask?
                    </p>
                    <p className="text-sm text-gray-600 mt-1">
                        Try some recommendations by us.
                    </p>
                </div>
                
                <div className="flex gap-2 -mr-2.5 -mt-2">
                    <button
                        onClick={handleCloseClick}
                        className="
                            p-0.5 
                            hover:bg-gray-100 
                            rounded transition-colors duration-200
                            border-2 border-transparent hover:border-gray-300
                        "
                    >
                        <X className="w-4 h-4 text-gray-600" />
                    </button>
                </div>
            </div>
        </div>
    );
} 