/**
 * Shared types for MessageTile and its extracted sub-components / hooks.
 */

export interface QueryState {
    isExecuting: boolean;
    isExample: boolean;
}

export interface QueryResultState {
    data: any;
    loading: boolean;
    error: string | null;
    currentPage: number;
    pageSize: number;
    totalRecords: number | null;
    /** Current cursor value used to load this page (cursor-based pagination). */
    cursor: string | null;
    /** Cursor for fetching the next page. */
    nextCursor: string | null;
    /** Whether more pages exist beyond the current one. */
    hasMore: boolean;
}

export interface PageData {
    data: any[];
    totalRecords: number;
    /** Cursor that loads the page after this one (cursor-based pagination). */
    nextCursor?: string | null;
    /** Whether more data exists beyond this page. */
    hasMore?: boolean;
    /** Cursor that was used to load this page. */
    cursor?: string | null;
}

export type QueryViewMode = 'table' | 'json' | 'visualization';
