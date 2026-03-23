import {
    BarChart,
    Bar,
    Cell,
    ResponsiveContainer,
    XAxis,
    YAxis,
    CartesianGrid,
    Tooltip,
} from 'recharts';
import { Widget } from '../../../types/dashboard';
import { CHART_COLORS, TOOLTIP_STYLE } from './widgetConstants';

interface HistogramWidgetProps {
    widget: Widget;
    data: Record<string, unknown>[];
}

export default function HistogramWidget({ widget, data }: HistogramWidgetProps) {
    const config = widget.histogram_config;
    if (!config || data.length === 0) return null;

    const values = data
        .map(row => row[config.value_column] as number)
        .filter(v => typeof v === 'number' && !isNaN(v));

    if (values.length === 0) {
        return (
            <div className="flex items-center justify-center h-64">
                <span className="text-gray-500">No numeric data for histogram</span>
            </div>
        );
    }

    const minVal = Math.min(...values);
    const maxVal = Math.max(...values);
    const range = maxVal - minVal;

    const bucketCount = config.bucket_count || 10;
    const bucketSize = config.bucket_size || range / bucketCount;
    const buckets: { range: string; count: number; start: number; end: number }[] = [];

    for (let i = 0; i < bucketCount; i++) {
        const start = minVal + i * bucketSize;
        const end = start + bucketSize;
        const count = values.filter(v => v >= start && (i === bucketCount - 1 ? v <= end : v < end)).length;
        buckets.push({
            range: `${start.toFixed(config.decimal_places ?? 0)}-${end.toFixed(config.decimal_places ?? 0)}`,
            count,
            start,
            end,
        });
    }

    const mean = config.show_mean ? values.reduce((a, b) => a + b, 0) / values.length : undefined;
    const median = config.show_median
        ? (() => {
              const sorted = [...values].sort((a, b) => a - b);
              const mid = Math.floor(sorted.length / 2);
              return sorted.length % 2 === 0 ? (sorted[mid - 1] + sorted[mid]) / 2 : sorted[mid];
          })()
        : undefined;

    const showStats = mean !== undefined || median !== undefined;
    const chartHeight = bucketCount <= 6 ? 280 : bucketCount <= 10 ? 360 : 390;

    return (
        <div className="w-full min-h-[320px] outline-none focus:outline-none" tabIndex={-1}>
            <div style={{ height: chartHeight }}>
                <ResponsiveContainer width="100%" height="100%">
                    <BarChart data={buckets} margin={{ top: 24, right: 10, left: 0, bottom: 5 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="#e5e7eb" />
                        <XAxis
                            dataKey="range"
                            tick={{ fontSize: 12, fill: '#6b7280' }}
                            tickLine={false}
                            axisLine={{ stroke: '#d1d5db' }}
                            angle={-45}
                            textAnchor="end"
                            height={60}
                        />
                        <YAxis
                            label={{ value: 'Frequency', angle: -90, position: 'insideLeft', style: { fontSize: 13, fill: '#6b7280' } }}
                            tick={{ fontSize: 13, fill: '#6b7280' }}
                            tickLine={false}
                            axisLine={{ stroke: '#d1d5db' }}
                        />
                        <Tooltip {...TOOLTIP_STYLE} formatter={(value: number) => [value, 'Count']} />
                        <Bar dataKey="count" radius={[4, 4, 0, 0]}>
                            {buckets.map((_entry, index) => (
                                <Cell key={`cell-${index}`} fill={CHART_COLORS[index % CHART_COLORS.length]} />
                            ))}
                        </Bar>
                    </BarChart>
                </ResponsiveContainer>
            </div>
            {showStats && (
                <div className="flex gap-4 justify-center mt-4 mb-3 text-sm">
                    {mean !== undefined && (
                        <div className="flex items-center gap-1.5">
                            <div className="w-3 h-3 rounded-full bg-blue-500" />
                            <span className="text-gray-600 font-medium">
                                Mean: <span className="font-bold text-black">{mean.toFixed(config.decimal_places ?? 1)}</span>
                            </span>
                        </div>
                    )}
                    {median !== undefined && (
                        <div className="flex items-center gap-1.5">
                            <div className="w-3 h-3 rounded-full bg-purple-500" />
                            <span className="text-gray-600 font-medium">
                                Median: <span className="font-bold text-black">{median.toFixed(config.decimal_places ?? 1)}</span>
                            </span>
                        </div>
                    )}
                </div>
            )}
        </div>
    );
}
