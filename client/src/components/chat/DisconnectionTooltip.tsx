import { X } from 'lucide-react';
import { useEffect, useRef } from 'react';

interface DisconnectionTooltipProps {
    isVisible: boolean;
    onClose: () => void;
}

export default function DisconnectionTooltip({ isVisible, onClose }: DisconnectionTooltipProps) {
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
                absolute top-16 right-0 
                bg-white border-2 border-black
                px-3 md:px-4 py-3 rounded-lg shadow-lg
                w-60 md:w-72 z-50
                animate-fade-in text-left
            "
            style={{
                animation: 'fadeIn 0.3s ease-in-out'
            }}
        >
            {/* Arrow pointing up to reconnect button */}
            <div className="absolute top-0 right-3 w-0 h-0 border-l-8 border-r-8 border-b-8 border-transparent border-b-black transform -translate-y-full"></div>
            
            <div className="flex items-start gap-3">
                <div className="flex-1">
                    <p className="text-base font-bold text-black">
                        You're Disconnected
                    </p>
                    <p className="text-sm text-gray-600 mt-1">
                        Click the reconnect button above to reconnect to the data source.
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