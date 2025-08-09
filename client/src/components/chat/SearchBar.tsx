import { Search, X, ChevronUp, ChevronDown } from 'lucide-react';
import { useState, useRef, useEffect, useCallback } from 'react';

interface SearchBarProps {
    onSearch: (query: string) => void;
    onClose: () => void;
    onNavigateUp: () => void;
    onNavigateDown: () => void;
    currentResultIndex: number;
    totalResults: number;
    initialQuery?: string;
}

export default function SearchBar({
    onSearch,
    onClose,
    onNavigateUp,
    onNavigateDown,
    currentResultIndex,
    totalResults,
    initialQuery = ''
}: SearchBarProps) {
    const [searchQuery, setSearchQuery] = useState(initialQuery);
    const [isVisible, setIsVisible] = useState(false);
    const inputRef = useRef<HTMLInputElement>(null);
    const debounceTimerRef = useRef<NodeJS.Timeout | null>(null);

    useEffect(() => {
        // Trigger animation on mount
        setIsVisible(true);
        // Focus input after animation starts
        setTimeout(() => {
            inputRef.current?.focus();
        }, 100);
        
        // Cleanup debounce timer on unmount
        return () => {
            if (debounceTimerRef.current) {
                clearTimeout(debounceTimerRef.current);
            }
        };
    }, []);

    const handleSearchChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
        const query = e.target.value;
        setSearchQuery(query);
        
        // Clear existing timer
        if (debounceTimerRef.current) {
            clearTimeout(debounceTimerRef.current);
        }
        
        // Set new timer for debounced search
        debounceTimerRef.current = setTimeout(() => {
            onSearch(query);
        }, 300); // 300ms delay
    }, [onSearch]);

    const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
        if (e.key === 'Enter' && totalResults > 0) {
            if (e.shiftKey) {
                onNavigateUp();
            } else {
                onNavigateDown();
            }
        } else if (e.key === 'Escape') {
            handleClose();
        }
    };

    const handleClose = () => {
        setIsVisible(false);
        setTimeout(() => {
            onClose();
        }, 300); // Wait for animation to complete
    };

    return (
        <div className={`
            fixed md:absolute top-16 left-0 right-0 
            bg-white border-b border-gray-300 
            px-2 md:px-4 py-3 flex items-center gap-2 md:gap-4 z-[30] md:z-[19]
            transition-all duration-300 ease-in-out
            ${isVisible ? 'translate-y-0 opacity-100' : '-translate-y-full opacity-0'}
        `}>
            <div className="flex-1 min-w-0 flex items-center gap-2 px-3 md:px-4 py-2 bg-white neo-border rounded-lg">
                <Search className="w-5 h-5 text-gray-600 flex-shrink-0" />
                <input
                    ref={inputRef}
                    type="text"
                    value={searchQuery}
                    onChange={handleSearchChange}
                    onKeyDown={handleKeyDown}
                    placeholder="Search messages..."
                    className="flex-1 min-w-0 bg-transparent outline-none text-sm md:text-base"
                />
            </div>
            
            {searchQuery && (
                <div className="flex items-center gap-1 md:gap-3 flex-shrink-0">
                    {totalResults > 0 ? (
                        <>
                            <span className="text-xs md:text-sm font-medium whitespace-nowrap text-gray-700 hidden sm:inline">
                                {currentResultIndex + 1} of {totalResults}
                            </span>
                            <span className="text-xs font-medium text-gray-700 sm:hidden">
                                {currentResultIndex + 1}/{totalResults}
                            </span>
                            <div className="flex items-center gap-1 md:gap-2">
                                <ChevronUp 
                                    onClick={onNavigateUp}
                                    className="w-4 md:w-5 h-4 md:h-5 text-gray-600 hover:text-gray-900 cursor-pointer transition-colors"
                                    aria-label="Previous result"
                                />
                                <ChevronDown 
                                    onClick={onNavigateDown}
                                    className="w-4 md:w-5 h-4 md:h-5 text-gray-600 hover:text-gray-900 cursor-pointer transition-colors"
                                    aria-label="Next result"
                                />
                            </div>
                        </>
                    ) : (
                        <span className="text-xs md:text-sm text-gray-500">No results</span>
                    )}
                </div>
            )}
            
            <button
                onClick={handleClose}
                className="p-1.5 md:p-2 hover:bg-neo-gray rounded-lg transition-colors neo-border bg-white flex-shrink-0"
                aria-label="Close search"
            >
                <X className="w-4 md:w-5 h-4 md:h-5" />
            </button>
        </div>
    );
}