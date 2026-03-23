import { ArrowDown, ArrowUp } from 'lucide-react';
import { Widget } from '../../../types/dashboard';
import { formatStatValue } from './widgetConstants';

interface StatWidgetProps {
    widget: Widget;
    data: Record<string, unknown>[];
}

export default function StatWidget({ widget, data }: StatWidgetProps) {
    const config = widget.stat_config;
    if (!config || data.length === 0) return null;

    const firstRow = data[0];
    const keys = Object.keys(firstRow);
    const mainValue = firstRow[keys[0]] as number;

    if (mainValue === null || mainValue === undefined) {
        return (
            <div className="flex flex-col justify-center p-1">
                <div className="text-4xl font-black text-gray-400">—</div>
                <div className="text-sm font-semibold text-gray-500 mt-1">{widget.title}</div>
            </div>
        );
    }

    const formattedValue = formatStatValue(mainValue, config.format, config.prefix, config.suffix, config.decimal_places);
    const changePercent = config.change_percentage;
    const isPositive = changePercent !== undefined && changePercent >= 0;
    const trendIsGood = config.trend_direction === 'up_is_good' ? isPositive : !isPositive;

    return (
        <div className="flex flex-col justify-center p-1">
            <div className="text-4xl font-black text-black">{formattedValue}</div>
            {changePercent !== undefined && (
                <div className={`flex items-center gap-1 mt-2 text-base font-bold ${trendIsGood ? 'text-green-600' : 'text-red-600'}`}>
                    {isPositive ? <ArrowUp className="w-4 h-4" /> : <ArrowDown className="w-4 h-4" />}
                    <span>{Math.abs(changePercent).toFixed(1)}%</span>
                </div>
            )}
        </div>
    );
}
