import {
    PieChart,
    Pie,
    Cell,
    ResponsiveContainer,
    Tooltip,
    Legend,
} from 'recharts';
import { CHART_COLORS, TOOLTIP_STYLE, legendFormatter } from './widgetConstants';

interface PieChartWidgetProps {
    data: Record<string, unknown>[];
}

export default function PieChartWidget({ data }: PieChartWidgetProps) {
    const nameKey = data.length > 0 ? Object.keys(data[0]).find((k) => typeof data[0][k] !== 'number') : 'name';
    const valueKey = data.length > 0 ? Object.keys(data[0]).find((k) => typeof data[0][k] === 'number') : 'value';

    const heightClass = data.length >= 10
        ? 'h-[580px] md:h-[380px]'
        : data.length >= 8
            ? 'h-[360px] md:h-[310px]'
            : data.length >= 6
                ? 'h-[320px] md:h-[290px]'
                : data.length >= 3
                    ? 'h-[260px]'
                    : 'h-[250px]';

    return (
        <div className={`${heightClass} w-full outline-none focus:outline-none`} tabIndex={-1}>
            <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                    <Pie
                        data={data}
                        dataKey={valueKey || 'value'}
                        nameKey={nameKey || 'name'}
                        cx="50%"
                        cy="48%"
                        outerRadius={75}
                        innerRadius={38}
                        strokeWidth={2}
                        stroke="#000"
                    >
                        {data.map((_entry, index) => (
                            <Cell key={`cell-${index}`} fill={CHART_COLORS[index % CHART_COLORS.length]} />
                        ))}
                    </Pie>
                    <Tooltip
                        contentStyle={TOOLTIP_STYLE.contentStyle}
                        labelStyle={TOOLTIP_STYLE.labelStyle}
                        itemStyle={TOOLTIP_STYLE.itemStyle}
                        formatter={(value: number | string, name: string) => [value, legendFormatter(name)]}
                    />
                    <Legend
                        wrapperStyle={{ fontSize: 14, paddingBottom: 12, paddingTop: 12, paddingLeft: 12, paddingRight: 12 }}
                        iconSize={14}
                        formatter={legendFormatter}
                    />
                </PieChart>
            </ResponsiveContainer>
        </div>
    );
}
