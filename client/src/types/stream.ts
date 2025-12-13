export interface StreamResponse {
    event: 'ai-response' | 'ai-response-step' | 'ai-response-error' | 'db-connected' |
    'db-disconnected' | 'sse-connected' | 'response-cancelled' | 'query-results' |
    'rollback-executed' | 'query-execution-failed' | 'rollback-query-failed' | 'system-message';
    data?: any;
} 