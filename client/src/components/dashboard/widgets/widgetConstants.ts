import { Activity, BarChart3, Gauge, Grid3x3, PieChart, Table2, TrendingUp } from 'lucide-react';
import React from 'react';
import { WidgetType } from '../../../types/dashboard';

// Balanced yellow / green / amber palette
export const CHART_COLORS = ['#047857', '#f59e0b', '#10b981', '#d97706', '#14b8a6', '#FBBF24', '#059669', '#b45309'];

export const WIDGET_TYPE_ICONS: Record<WidgetType, React.ReactNode> = {
    stat: React.createElement(TrendingUp, { className: 'w-5 h-5' }),
    line: React.createElement(TrendingUp, { className: 'w-5 h-5' }),
    bar: React.createElement(BarChart3, { className: 'w-5 h-5' }),
    area: React.createElement(BarChart3, { className: 'w-5 h-5' }),
    pie: React.createElement(PieChart, { className: 'w-5 h-5' }),
    table: React.createElement(Table2, { className: 'w-5 h-5' }),
    combo: React.createElement(BarChart3, { className: 'w-5 h-5' }),
    gauge: React.createElement(Gauge, { className: 'w-5 h-5' }),
    bar_gauge: React.createElement(Activity, { className: 'w-5 h-5' }),
    heatmap: React.createElement(Grid3x3, { className: 'w-5 h-5' }),
    histogram: React.createElement(BarChart3, { className: 'w-5 h-5' }),
};

/* Grafana-style loading bar keyframes via inline style tag */
export const GRAFANA_LOADING_STYLE = `
@keyframes grafana-loading-bar {
  0% { transform: translateX(-100%); }
  50% { transform: translateX(0%); }
  100% { transform: translateX(200%); }
}
`;

// Common tooltip style for all charts — white text on dark background
export const TOOLTIP_STYLE = {
    contentStyle: {
        backgroundColor: '#1a1a1a',
        border: '2px solid #333',
        borderRadius: 8,
        padding: '10px 14px',
        boxShadow: '0 4px 12px rgba(0,0,0,0.3)',
    },
    labelStyle: { color: '#fff', fontWeight: 800 as const, fontSize: 14, marginBottom: 4 },
    itemStyle: { color: '#e5e7eb', fontSize: 14, padding: '2px 0' },
    cursor: { fill: 'rgba(255, 215, 0, 0.08)' },
};

// Capitalize first letter of each word for legend labels
export const legendFormatter = (value: string) => {
    if (!value || value.length === 0 || value.trim() === '') return 'No Label';
    return value.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
};

// Try to parse any date-like string into short human-readable format for axes
export const tryFormatDate = (value: unknown): string => {
    if (typeof value !== 'string') return String(value ?? '');
    if (/^\d{4}-\d{2}/.test(value) || /^\d{4}\/\d{2}/.test(value)) {
        try {
            const d = new Date(value);
            if (!isNaN(d.getTime())) {
                return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: '2-digit' });
            }
        } catch { /* fall through */ }
    }
    return value;
};

// Tooltip label formatter — capitalize + human-readable date
export const tooltipLabelFormatter = (label: string | number) => {
    if (label == null) return '';
    const s = String(label);
    const dateFormatted = tryFormatDate(s);
    if (dateFormatted !== s) return dateFormatted;
    return s.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
};

// Chart axis date tick formatter
export const axisTickFormatter = (value: string | number) => tryFormatDate(value);

// Date detection & formatting helpers
export const isDateString = (value: unknown): boolean =>
    typeof value === 'string' && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/.test(value);

export const formatDateValue = (dateStr: string, friendly: boolean): string => {
    if (!friendly) return dateStr;
    try {
        const d = new Date(dateStr);
        if (isNaN(d.getTime())) return dateStr;
        return d.toLocaleString('en-US', {
            month: 'short', day: 'numeric', year: 'numeric',
            hour: 'numeric', minute: '2-digit', hour12: true,
        });
    } catch { return dateStr; }
};

// Helper: format stat values
export function formatStatValue(
    value: number,
    format?: string,
    prefix?: string,
    suffix?: string,
    decimalPlaces?: number,
): string {
    const dp = decimalPlaces ?? 0;
    let formatted: string;
    switch (format) {
        case 'currency':
            formatted = value.toLocaleString(undefined, { minimumFractionDigits: dp, maximumFractionDigits: dp });
            return `${prefix || '$'}${formatted}${suffix || ''}`;
        case 'percentage':
            formatted = value.toFixed(dp);
            return `${prefix || ''}${formatted}%${suffix || ''}`;
        case 'duration':
            if (value >= 3600) return `${(value / 3600).toFixed(1)}h`;
            if (value >= 60) return `${(value / 60).toFixed(1)}m`;
            return `${value.toFixed(0)}s`;
        default:
            if (value >= 1_000_000) formatted = `${(value / 1_000_000).toFixed(1)}M`;
            else if (value >= 1_000) formatted = `${(value / 1_000).toFixed(1)}K`;
            else formatted = value.toLocaleString(undefined, { minimumFractionDigits: dp, maximumFractionDigits: dp });
            return `${prefix || ''}${formatted}${suffix || ''}`;
    }
}

// Helper: relative time
export function formatRelativeTime(dateString: string): string {
    const now = Date.now();
    const then = new Date(dateString).getTime();
    const diff = Math.floor((now - then) / 1000);
    if (diff < 10) return 'Just now';
    if (diff < 60) return `${diff}s ago`;
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
    if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
    return `${Math.floor(diff / 86400)}d ago`;
}
