import {
    BarChart,
    Bar,
    Cell,
    ResponsiveContainer,
    XAxis,
    YAxis,
    CartesianGrid,
    Tooltip,
    Legend,
    Brush,
} from 'recharts';
import { CHART_COLORS, TOOLTIP_STYLE, legendFormatter, tooltipLabelFormatter, axisTickFormatter } from './widgetConstants';

interface BarChartWidgetProps {
    chartData: Record<string, unknown>[];
    numericKeys: string[];
    categoryKey: string | undefined;
}

export default function BarChartWidget({ chartData, numericKeys, categoryKey }: BarChartWidgetProps) {
    const showBrush = chartData.length > 12;
    const singleSeries = numericKeys.length === 1;
    return (
        <div className="w-full outline-none focus:outline-none" tabIndex={-1}>
            <div style={{ height: showBrush ? 360 : 330 }}>
                <ResponsiveContainer width="100%" height="100%">
                    <BarChart data={chartData} margin={{ top: 5, right: 10, left: 0, bottom: 5 }}>
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
                        {!singleSeries && (
                            <Legend wrapperStyle={{ fontSize: 14, paddingTop: 10, paddingLeft: 8, paddingRight: 8 }} iconSize={14} formatter={legendFormatter} />
                        )}
                        {singleSeries ? (
                            <Bar key={numericKeys[0]} dataKey={numericKeys[0]} radius={[4, 4, 0, 0]}>
                                {chartData.map((_entry, index) => (
                                    <Cell key={`cell-${index}`} fill={CHART_COLORS[index % CHART_COLORS.length]} />
                                ))}
                            </Bar>
                        ) : (
                            numericKeys.map((key, i) => (
                                <Bar key={key} dataKey={key} name={key} fill={CHART_COLORS[i % CHART_COLORS.length]} radius={[4, 4, 0, 0]} />
                            ))
                        )}
                        {showBrush && (
                            <Brush
                                dataKey={categoryKey}
                                height={28}
                                stroke="#047857"
                                fill="#f9fafb"
                                travellerWidth={10}
                                tickFormatter={() => ''}
                                startIndex={Math.max(0, chartData.length - 15)}
                            />
                        )}
                    </BarChart>
                </ResponsiveContainer>
            </div>
            {showBrush && <p className="text-xs text-gray-400 text-center mt-1">Drag the handle above to select a range</p>}
        </div>
    );
}
