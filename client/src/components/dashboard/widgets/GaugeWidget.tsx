import { Widget } from '../../../types/dashboard';

interface GaugeWidgetProps {
    widget: Widget;
    data: Record<string, unknown>[];
}

export default function GaugeWidget({ widget, data }: GaugeWidgetProps) {
    const config = widget.gauge_config;
    if (!config || data.length === 0) return null;

    const firstRow = data[0];
    const keys = Object.keys(firstRow);
    const value = firstRow[keys[0]] as number;

    if (value === null || value === undefined) {
        return (
            <div className="flex items-center justify-center h-full min-h-[120px]">
                <div className="text-4xl font-black text-gray-400">—</div>
            </div>
        );
    }

    const min = config.min ?? 0;
    const max = config.max ?? 100;
    const percentage = Math.min(100, Math.max(0, ((value - min) / (max - min)) * 100));

    const getColor = () => {
        if (!config.thresholds || config.thresholds.length === 0) return '#047857';
        const sorted = [...config.thresholds].sort((a, b) => b.value - a.value);
        for (const threshold of sorted) {
            if (value >= threshold.value) return threshold.color;
        }
        return config.thresholds[0]?.color || '#047857';
    };

    const color = getColor();
    const displayValue = value.toFixed(config.decimal_places ?? 0);

    return (
        <div className="min-h-[240px] w-full flex flex-col items-center justify-center py-4 outline-none focus:outline-none" tabIndex={-1}>
            <div className="relative w-full max-w-[190px] aspect-square">
                {/* Background arc */}
                <svg className="w-full h-full transform -rotate-90" viewBox="0 0 100 100">
                    <circle
                        cx="50"
                        cy="50"
                        r="40"
                        fill="none"
                        stroke="#e5e7eb"
                        strokeWidth="8"
                        strokeDasharray="251.2"
                        strokeDashoffset="62.8"
                    />
                    {/* Value arc */}
                    <circle
                        cx="50"
                        cy="50"
                        r="40"
                        fill="none"
                        stroke={color}
                        strokeWidth="8"
                        strokeDasharray="251.2"
                        strokeDashoffset={62.8 + ((100 - percentage) / 100) * 188.4}
                        strokeLinecap="round"
                        style={{ transition: 'stroke-dashoffset 0.5s ease' }}
                    />
                </svg>
                {/* Center value */}
                <div className="absolute inset-0 flex flex-col items-center justify-center">
                    <div className="text-3xl font-black text-black">
                        {displayValue}{config.unit || ''}
                    </div>
                    <div className="text-sm text-gray-500 mt-1">
                        {min} - {max}
                    </div>
                </div>
            </div>
            {config.show_threshold && config.thresholds && config.thresholds.length > 0 && (
                <div className="flex gap-3 mt-3 flex-wrap justify-center px-2">
                    {config.thresholds.map((t, i) => (
                        <div key={i} className="flex items-center gap-1.5 text-xs">
                            <div className="w-3 h-3 rounded-full" style={{ backgroundColor: t.color }} />
                            <span className="text-gray-600 font-medium">{t.value}{config.unit || ''}</span>
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
}
