const RECOMMENDATION_TOOLTIP_KEY = 'neobase_recommendation_tooltip_shown';

export const recommendationStorage = {
    /**
     * Check if the recommendation tooltip has been shown for a specific chat
     */
    hasShownTooltip: (chatId: string): boolean => {
        try {
            const shownChats = JSON.parse(localStorage.getItem(RECOMMENDATION_TOOLTIP_KEY) || '[]');
            return shownChats.includes(chatId);
        } catch (error) {
            console.error('Error reading recommendation tooltip state:', error);
            return false;
        }
    },

    /**
     * Mark the recommendation tooltip as shown for a specific chat
     */
    markTooltipAsShown: (chatId: string): void => {
        try {
            const shownChats = JSON.parse(localStorage.getItem(RECOMMENDATION_TOOLTIP_KEY) || '[]');
            if (!shownChats.includes(chatId)) {
                shownChats.push(chatId);
                localStorage.setItem(RECOMMENDATION_TOOLTIP_KEY, JSON.stringify(shownChats));
            }
        } catch (error) {
            console.error('Error saving recommendation tooltip state:', error);
        }
    },

    /**
     * Clear all stored tooltip states (useful for testing or reset)
     */
    clearAllTooltipStates: (): void => {
        try {
            localStorage.removeItem(RECOMMENDATION_TOOLTIP_KEY);
        } catch (error) {
            console.error('Error clearing recommendation tooltip states:', error);
        }
    }
}; 