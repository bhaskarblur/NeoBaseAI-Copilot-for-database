import { Widget } from '../../../types/dashboard';

interface BarGaugeWidgetProps {
    widget: Widget;
    data: Record<string, unknown>[];
}

export default function BarGaugeWidget({ widget, data }: BarGaugeWidgetProps) {
    const config = widget.bar_gauge_config;
    if (!config || data.length === 0) return null;

    const min = config.min ?? 0;
    const max = config.max ?? 100;

    const getColor = (value: number) => {
        if (!config.thresholds || config.thresholds.length === 0) return '#047857';
        const sorted = [...config.thresholds].sort((a, b) => b.value - a.value);
        for (const threshold of sorted) {
            if (value >= threshold.value) return threshold.color;
        }
        return config.thresholds[0]?.color || '#047857';
    };

    return (
        <div className="min-h-[260px] w-full flex items-center py-4 outline-none focus:outline-none" tabIndex={-1}>
            <div className="w-full space-y-3 px-2">
                {data.slice(0, 8).map((row, idx) => {
                    const keys = Object.keys(row);
                    const label = keys.find(k => typeof row[k] !== 'number');
                    const valueKey = keys.find(k => typeof row[k] === 'number');
                    const value = valueKey ? (row[valueKey] as number) : 0;
                    const percentage = Math.min(100, Math.max(0, ((value - min) / (max - min)) * 100));
                    const color = getColor(value);
                    const displayValue = value.toFixed(config.decimal_places ?? 0);

                    return (
                        <div key={idx} className="space-y-2">
                            <div className="flex items-center justify-between text-base">
                                <span className="font-semibold text-gray-700">
                                    {label ? String(row[label]) : `Series ${idx + 1}`}
                                </span>
                                <span className="font-bold text-black">
                                    {displayValue}{config.unit || ''}
                                </span>
                            </div>
                            <div className="relative h-6 bg-gray-200 rounded-full overflow-hidden">
                                <div
                                    className="absolute inset-y-0 left-0 rounded-full transition-all duration-500"
                                    style={{
                                        width: `${percentage}%`,
                                        background: config.display_mode === 'gradient'
                                            ? `linear-gradient(to right, ${color}cc, ${color})`
                                            : color,
                                    }}
                                />
                                {config.display_mode === 'lcd' && (
                                    <div className="absolute inset-0 flex">
                                        {Array.from({ length: 20 }).map((_, i) => (
                                            <div key={i} className="flex-1 border-r border-white/30" />
                                        ))}
                                    </div>
                                )}
                            </div>
                        </div>
                    );
                })}
            </div>
        </div>
    );
}
