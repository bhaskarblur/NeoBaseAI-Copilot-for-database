import { useCallback, useRef, useState } from 'react';
import { Message } from '../types/query';

interface UseMessageSearchParams {
    messages: Message[];
}

export function useMessageSearch({ messages }: UseMessageSearchParams) {
    const [showSearch, setShowSearch] = useState(false);
    const [searchQuery, setSearchQuery] = useState('');
    const [searchResults, setSearchResults] = useState<string[]>([]);
    const [currentSearchIndex, setCurrentSearchIndex] = useState(0);
    const searchResultRefs = useRef<{ [key: string]: HTMLElement | null }>({});

    const scrollToSearchResult = useCallback((index: number, results?: string[]) => {
        const list = results ?? searchResults;
        if (list.length === 0 || index < 0 || index >= list.length) return;
        const element = searchResultRefs.current[list[index]];
        if (element) {
            element.scrollIntoView({ behavior: 'smooth', block: 'center' });
        }
    }, [searchResults]);

    const performSearch = useCallback((query: string) => {
        setSearchQuery(query);
        if (!query.trim()) {
            setSearchResults([]);
            setCurrentSearchIndex(0);
            return;
        }

        const results: string[] = [];
        const lowerQuery = query.toLowerCase();

        messages.forEach(message => {
            let hit = false;
            if (message.content.toLowerCase().includes(lowerQuery)) hit = true;
            message.queries?.forEach(q => {
                if (q.query.toLowerCase().includes(lowerQuery)) hit = true;
                if (q.error) {
                    const { message: msg, code, details } = q.error;
                    if ((msg && msg.toLowerCase().includes(lowerQuery)) ||
                        (code && code.toLowerCase().includes(lowerQuery)) ||
                        (details && details.toLowerCase().includes(lowerQuery))) {
                        hit = true;
                    }
                }
                if (q.description && q.description.toLowerCase().includes(lowerQuery)) hit = true;
            });
            if (hit) results.push(`msg-${message.id}`);
        });

        setSearchResults(results);
        setCurrentSearchIndex(0);
        if (results.length > 0) scrollToSearchResult(0, results);
    }, [messages, scrollToSearchResult]);

    const navigateSearchUp = useCallback(() => {
        if (searchResults.length === 0) return;
        const newIndex = currentSearchIndex < searchResults.length - 1 ? currentSearchIndex + 1 : 0;
        setCurrentSearchIndex(newIndex);
        scrollToSearchResult(newIndex);
    }, [currentSearchIndex, searchResults, scrollToSearchResult]);

    const navigateSearchDown = useCallback(() => {
        if (searchResults.length === 0) return;
        const newIndex = currentSearchIndex > 0 ? currentSearchIndex - 1 : searchResults.length - 1;
        setCurrentSearchIndex(newIndex);
        scrollToSearchResult(newIndex);
    }, [currentSearchIndex, searchResults, scrollToSearchResult]);

    const handleToggleSearch = useCallback(() => {
        setShowSearch(prev => {
            const next = !prev;
            if (!next) {
                setSearchResults([]);
                setCurrentSearchIndex(0);
            }
            return next;
        });
    }, []);

    // Re-run search when search panel opens with an existing query
    const openSearch = useCallback(() => {
        setShowSearch(true);
        if (searchQuery) performSearch(searchQuery);
    }, [searchQuery, performSearch]);

    return {
        showSearch, setShowSearch,
        searchQuery, setSearchQuery,
        searchResults, setSearchResults,
        currentSearchIndex, setCurrentSearchIndex,
        searchResultRefs,
        performSearch,
        scrollToSearchResult,
        navigateSearchUp,
        navigateSearchDown,
        handleToggleSearch,
        openSearch,
    };
}
