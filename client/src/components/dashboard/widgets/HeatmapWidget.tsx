import { Widget } from '../../../types/dashboard';

interface HeatmapWidgetProps {
    widget: Widget;
    data: Record<string, unknown>[];
}

export default function HeatmapWidget({ widget, data }: HeatmapWidgetProps) {
    const config = widget.heatmap_config;
    if (!config || data.length === 0) return null;

    const xValues = Array.from(new Set(data.map(row => String(row[config.x_axis_column]))));
    const yValues = Array.from(new Set(data.map(row => String(row[config.y_axis_column]))));

    const values = data.map(row => row[config.value_column] as number).filter(v => typeof v === 'number');
    const minValue = Math.min(...values);
    const maxValue = Math.max(...values);

    const getColorForValue = (value: number) => {
        const normalized = (value - minValue) / (maxValue - minValue);
        if (config.color_scheme === 'green-red') {
            const r = Math.round(255 * normalized);
            const g = Math.round(255 * (1 - normalized));
            return `rgb(${r}, ${g}, 0)`;
        } else if (config.color_scheme === 'blue-yellow') {
            return normalized > 0.5
                ? `rgb(255, 255, ${Math.round(255 * (1 - normalized) * 2)})`
                : `rgb(${Math.round(255 * normalized * 2)}, ${Math.round(255 * normalized * 2)}, 255)`;
        } else {
            const gray = Math.round(255 * normalized);
            return `rgb(${gray}, ${gray}, ${gray})`;
        }
    };

    const getValueForCell = (x: string, y: string): number | null => {
        const row = data.find(
            r => String(r[config.x_axis_column]) === x && String(r[config.y_axis_column]) === y
        );
        return row ? (row[config.value_column] as number) : null;
    };

    return (
        <div className="min-h-[280px] w-full flex flex-col justify-center py-4 outline-none focus:outline-none" tabIndex={-1}>
            <div className="overflow-x-auto px-2">
                <div className="inline-block min-w-full">
                    <div className="flex">
                        {/* Y-axis labels */}
                        <div className="flex flex-col justify-around pr-2 text-sm font-semibold text-gray-600">
                            <div className="h-6" />
                            {yValues.map((y, i) => (
                                <div key={i} className="h-10 flex items-center text-sm">{y}</div>
                            ))}
                        </div>
                        {/* Grid */}
                        <div className="flex-1">
                            {/* X-axis labels */}
                            <div className="flex mb-1">
                                {xValues.map((x, i) => (
                                    <div key={i} className="flex-1 text-center text-sm font-semibold text-gray-600 px-1">
                                        {x.length > 8 ? x.slice(0, 8) + '...' : x}
                                    </div>
                                ))}
                            </div>
                            {/* Cells */}
                            {yValues.map((y, yIdx) => (
                                <div key={yIdx} className="flex gap-1 mb-1">
                                    {xValues.map((x, xIdx) => {
                                        const value = getValueForCell(x, y);
                                        return (
                                            <div
                                                key={xIdx}
                                                className="flex-1 h-10 rounded border border-gray-300 flex items-center justify-center text-base font-semibold"
                                                style={{
                                                    backgroundColor: value !== null ? getColorForValue(value) : '#f3f4f6',
                                                    color: value !== null && value > (maxValue - minValue) / 2 ? '#fff' : '#000',
                                                }}
                                                title={value !== null ? `${x}, ${y}: ${value}` : 'No data'}
                                            >
                                                {config.show_values && value !== null ? value.toFixed(0) : ''}
                                            </div>
                                        );
                                    })}
                                </div>
                            ))}
                        </div>
                    </div>
                </div>
            </div>
            {config.show_legend && (
                <div className="mt-6 flex items-center justify-center gap-2 px-2">
                    <span className="text-sm text-gray-600 font-medium">Low</span>
                    <div className="flex h-4 w-32 rounded">
                        {Array.from({ length: 10 }).map((_, i) => (
                            <div
                                key={i}
                                className="flex-1"
                                style={{ backgroundColor: getColorForValue(minValue + (maxValue - minValue) * (i / 9)) }}
                            />
                        ))}
                    </div>
                    <span className="text-sm text-gray-600 font-medium">High</span>
                </div>
            )}
        </div>
    );
}
