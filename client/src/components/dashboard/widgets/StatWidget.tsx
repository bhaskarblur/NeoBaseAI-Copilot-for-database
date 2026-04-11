import { ArrowDown, ArrowUp } from 'lucide-react';
import { useState } from 'react';
import { Widget } from '../../../types/dashboard';
import { formatStatValue, formatStatValueFull } from './widgetConstants';

interface StatWidgetProps {
    widget: Widget;
    data: Record<string, unknown>[];
}

// Number of characters at which we consider the value "too long" to display unabbreviated
const ABBREV_THRESHOLD = 10;

export default function StatWidget({ widget, data }: StatWidgetProps) {
    const config = widget.stat_config;
    // abbreviated = true means show K/M/B format; false = full number
    const [abbreviated, setAbbreviated] = useState(false);

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

    const fullFormatted = formatStatValueFull(mainValue, config.format, config.prefix, config.suffix, config.decimal_places);
    const abbrevFormatted = formatStatValue(mainValue, config.format, config.prefix, config.suffix, config.decimal_places);

    // Auto-abbreviate if full value is too long (would overflow the card)
    const forceAbbrev = fullFormatted.length > ABBREV_THRESHOLD;
    const displayValue = (abbreviated || forceAbbrev) ? abbrevFormatted : fullFormatted;

    // Only show toggle when abbreviation is meaningful (i.e. they differ)
    const showToggle = fullFormatted !== abbrevFormatted;

    const changePercent = config.change_percentage;
    const isPositive = changePercent !== undefined && changePercent >= 0;
    const trendIsGood = config.trend_direction === 'up_is_good' ? isPositive : !isPositive;

    return (
        <div className="flex flex-col justify-center p-1">
            <div className="flex items-start gap-2">
                <div className="text-4xl font-black text-black leading-tight">{displayValue}</div>
                {showToggle && !forceAbbrev && (
                    <button
                        onClick={(e) => { e.stopPropagation(); setAbbreviated((v) => !v); }}
                        className="mt-1 inline-flex text-[10px] px-1.5 py-0.5 bg-gray-200 hover:bg-gray-300 rounded text-gray-600 font-medium transition-colors focus:outline-none flex-shrink-0"
                        title="Toggle number format"
                    >
                        {abbreviated ? 'Full' : 'K/M'}
                    </button>
                )}
            </div>
            {changePercent !== undefined && (
                <div className={`flex items-center gap-1 mt-2 text-base font-bold ${trendIsGood ? 'text-green-600' : 'text-red-600'}`}>
                    {isPositive ? <ArrowUp className="w-4 h-4" /> : <ArrowDown className="w-4 h-4" />}
                    <span>{Math.abs(changePercent).toFixed(1)}%</span>
                </div>
            )}
        </div>
    );
}
