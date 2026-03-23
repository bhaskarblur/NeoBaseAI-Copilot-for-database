import {
    AreaChart,
    Area,
    ResponsiveContainer,
    XAxis,
    YAxis,
    CartesianGrid,
    Tooltip,
    Legend,
    Brush,
} from 'recharts';
import { CHART_COLORS, TOOLTIP_STYLE, legendFormatter, tooltipLabelFormatter, axisTickFormatter } from './widgetConstants';

interface AreaChartWidgetProps {
    chartData: Record<string, unknown>[];
    numericKeys: string[];
    categoryKey: string | undefined;
}

export default function AreaChartWidget({ chartData, numericKeys, categoryKey }: AreaChartWidgetProps) {
    const showBrush = chartData.length > 15;
    return (
        <div className="w-full outline-none focus:outline-none" tabIndex={-1}>
            <div style={{ height: showBrush ? 340 : 280 }}>
                <ResponsiveContainer width="100%" height="100%">
                    <AreaChart data={chartData} margin={{ top: 5, right: 10, left: 0, bottom: 5 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="#e5e7eb" />
                        {categoryKey && (
                            <XAxis
                                dataKey={categoryKey}
                                tick={{ fontSize: 13, fill: '#6b7280' }}
                                tickLine={false}
                                axisLine={{ stroke: '#d1d5db' }}
                                tickFormatter={(val) => {
                                    const formatted = axisTickFormatter(val);
                                    return (!formatted || formatted.trim() === '') ? 'No Label' : formatted;
                                }}
                            />
                        )}
                        <YAxis tick={{ fontSize: 13, fill: '#6b7280' }} tickLine={false} axisLine={{ stroke: '#d1d5db' }} />
                        <Tooltip
                            {...TOOLTIP_STYLE}
                            labelFormatter={tooltipLabelFormatter}
                            formatter={(value: number | string, name: string) => [value, legendFormatter(name)]}
                        />
                        <Legend wrapperStyle={{ fontSize: 14, paddingTop: 10, paddingLeft: 8, paddingRight: 8 }} iconSize={14} formatter={legendFormatter} />
                        {numericKeys.map((key, i) => (
                            <Area
                                key={key}
                                type="monotone"
                                dataKey={key}
                                name={key}
                                stroke={CHART_COLORS[i % CHART_COLORS.length]}
                                fill={CHART_COLORS[i % CHART_COLORS.length]}
                                fillOpacity={0.15}
                                strokeWidth={2.5}
                                activeDot={{ r: 5, strokeWidth: 2, stroke: '#000' }}
                            />
                        ))}
                        {showBrush && (
                            <Brush
                                dataKey={categoryKey}
                                height={28}
                                stroke="#047857"
                                fill="#f9fafb"
                                travellerWidth={10}
                                tickFormatter={() => ''}
                                startIndex={Math.max(0, chartData.length - 20)}
                            />
                        )}
                    </AreaChart>
                </ResponsiveContainer>
            </div>
            {showBrush && <p className="text-xs text-gray-400 text-center mt-1">Drag the handler above to select a range</p>}
        </div>
    );
}
