/**
 * Pure utility functions shared across query result components.
 * No React dependencies — safe to import anywhere.
 */

// ─── Cursor helpers ───────────────────────────────────────────────────────────

/**
 * Extract the cursor value from a result record, handling field-name aliasing
 * (e.g. cursor_field="createdAt" but the column header is "Created At").
 */
export function extractCursorValue(
    record: Record<string, any>,
    cursorField: string,
): string | null {
    if (record[cursorField] != null) return String(record[cursorField]);
    const norm = (s: string) => s.toLowerCase().replace(/[\s_]/g, '');
    const target = norm(cursorField);
    for (const key of Object.keys(record)) {
        if (norm(key) === target && record[key] != null) return String(record[key]);
    }
    return null;
}

// ─── Result parsing ───────────────────────────────────────────────────────────

/**
 * Normalise the diverse shapes the backend can return into a plain array.
 */
export function parseResults(result: any, nonTechMode?: boolean): any[] {
    if (!result) return [];

    if (Array.isArray(result)) return result;

    if (result && typeof result === 'object') {
        if ('results' in result) {
            if (result.results === null && nonTechMode) return [];
            if (Array.isArray(result.results)) return result.results;
        }
        if ('rowsAffected' in result || 'message' in result) return [result];
        return [result];
    }

    return [result];
}

/**
 * Slice a flat array into a specific 25-record page (1- or 2-indexed offset page).
 * Used when the backend returns 50 records at once.
 */
export function sliceIntoPages(data: any[], pageSize: number, currentPage: number): any[] {
    const startIndex = currentPage % 2 === 1 ? 0 : pageSize;
    return data.slice(startIndex, startIndex + pageSize);
}

// ─── String deduplication ─────────────────────────────────────────────────────

/**
 * Remove duplicate semicolon-separated statements from a SQL/query string.
 */
export function removeDuplicateQueries(query: string): string {
    const queries = query
        .split(';')
        .map(q => q.trim())
        .filter(q => q.length > 0);
    return Array.from(new Set(queries)).join(';\n');
}

/**
 * Remove duplicate paragraphs or full-content duplicates from an LLM response.
 */
export function removeDuplicateContent(content: string): string {
    if (!content) return '';

    // Check for exact whole-content duplication
    const contentLength = content.length;
    if (contentLength > 20) {
        const halfPoint = Math.floor(contentLength / 2);
        for (let offset = -10; offset <= 10; offset++) {
            const splitPoint = halfPoint + offset;
            if (splitPoint <= 0 || splitPoint >= contentLength) continue;
            const firstPart = content.substring(0, splitPoint).trim();
            const secondPart = content.substring(splitPoint).trim();
            if (secondPart.startsWith(firstPart.substring(0, Math.min(20, firstPart.length)))) {
                return firstPart;
            }
        }
    }

    // Paragraph-level deduplication
    const paragraphs = content.split(/\n\n+/);
    const uniqueParagraphs: string[] = [];
    const seen = new Set<string>();
    for (const paragraph of paragraphs) {
        const trimmed = paragraph.trim();
        if (!trimmed) continue;
        const key = trimmed.toLowerCase();
        if (!seen.has(key)) {
            seen.add(key);
            uniqueParagraphs.push(paragraph);
        }
    }
    return uniqueParagraphs.join('\n\n');
}

// ─── Date helpers ─────────────────────────────────────────────────────────────

/**
 * Returns true when the value looks like an ISO-8601 datetime string.
 */
export function isDateString(value: any): boolean {
    return typeof value === 'string' && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/.test(value);
}

/**
 * Format an ISO datetime string in a friendly, localised way.
 */
export function formatDateString(dateStr: string, useFriendlyFormat: boolean): string {
    if (!useFriendlyFormat) return dateStr;
    try {
        const date = new Date(dateStr);
        if (isNaN(date.getTime())) return dateStr;
        return date.toLocaleString('en-US', {
            month: 'short',
            day: 'numeric',
            year: 'numeric',
            hour: 'numeric',
            minute: '2-digit',
            hour12: true,
        });
    } catch {
        return dateStr;
    }
}

/**
 * Format the timestamp shown inside a message bubble.
 */
export function formatMessageTime(message: { created_at: string; updated_at?: string; is_edited?: boolean }): string {
    const dateString =
        message.is_edited && message.updated_at ? message.updated_at : message.created_at;
    const date = new Date(dateString);
    return date.toLocaleTimeString('en-US', {
        hour: 'numeric',
        minute: 'numeric',
        hour12: true,
    });
}
