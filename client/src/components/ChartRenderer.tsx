import React from 'react';
import {
  LineChart, Line, BarChart, Bar, AreaChart, Area, PieChart, Pie, ScatterChart, Scatter,
  XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer,
  Cell, ReferenceLine
} from 'recharts';
import { ChartConfiguration } from '../types/visualization';
import { PencilRuler, RefreshCcw, Clock } from 'lucide-react';
import TooltipComponent from './ui/Tooltip';

interface ChartRendererProps {
  config: ChartConfiguration;
  data: Record<string, any>[];
  isLoading?: boolean;
  error?: string;
  onError?: (error: string) => void;
  onRetry?: () => void;
  onRegenerate?: () => void;
  updatedAt?: string; // Timestamp when the chart was last generated
}

// ============================================
// CENTRALIZED CHART STYLING CONFIGURATION
// ============================================
// This allows easy customization of all charts in one place
const CHART_THEME = {
  // Colors - use consistent palette across all charts
  colors: {
    primary: ['#FFD700', '#7CFC00', '#32CD32', '#90EE90', '#ADFF2F', '#F0E68C', '#FFFACD'],
    secondary: ['#FF6B35', '#004E89', '#1B9CFC', '#F7B801', '#13D9D9'],
    rainbow: ['#FF6B6B', '#4ECDC4', '#45B7D1', '#FFA07A', '#98D8C8', '#F7DC6F', '#BB8FCE'],
    vibrant: ['#FF006E', '#FB5607', '#FFBE0B', '#8338EC', '#3A86FF', '#06FFA5'],
    heatmap: ['#fee5d9', '#fcae91', '#fb6a4a', '#de2d26', '#a50f15'],
    waterfall: {
      increase: '#4ECDC4',
      decrease: '#FF6B6B',
      total: '#FFD700',
    },
  },
  // Axis styling
  axis: {
    stroke: '#fff',
    fontSize: 13,
    fontWeight: 400,
    tickColor: '#fff',
    labelFontSize: 14,
    labelFontWeight: 'bold' as const,
    labelColor: '#fff',
  },
  // Grid styling
  grid: {
    stroke: '#505050ff',
    strokeDasharray: '3 3',
  },
  // Legend styling
  legend: {
    fontSize: 14,
    fontWeight: 700,
    color: '#fff',
    paddingBottom: 40,
  },
  // Chart dimensions
  dimensions: {
    height: 400,
    marginTop: 20,
    marginRight: 15,
    marginLeft: 15,
    marginBottom: 0,
    xAxisHeight: 75,
    scatterMarginLeft: 60,
  },
  // Tooltip styling
  tooltip: {
    backgroundColor: '#000',
    borderColor: '#666',
    borderRadius: 8,
    padding: 12,
  },
  // Animation
  animation: {
    enabled: true,
    isAnimationActive: true,
  },
} as const;

const CustomTooltip = ({ active, payload, label }: any) => {
  if (active && payload && payload.length) {
    return (
      <div 
        className="bg-black text-white p-3 rounded-lg shadow-2xl border border-gray-500 backdrop-blur-sm"
        style={{
          backgroundColor: CHART_THEME.tooltip.backgroundColor,
          borderColor: CHART_THEME.tooltip.borderColor,
          borderRadius: CHART_THEME.tooltip.borderRadius,
          padding: CHART_THEME.tooltip.padding,
        }}
      >
        {label && (
          <p className="text-xs font-bold text-white mb-2">
            {typeof label === 'object' ? JSON.stringify(label) : label}
          </p>
        )}
        {payload.map((entry: any, index: number) => (
          <p key={index} style={{ color: entry.color }} className="text-xs font-medium">
            <span className="font-semibold">{entry.name}:</span>{' '}
            {typeof entry.value === 'number' ? entry.value.toLocaleString() : entry.value}
          </p>
        ))}
      </div>
    );
  }
  return null;
};

const ChartLoadingState = () => (
  <div className="w-full h-96 flex items-center justify-center bg-gray-900 rounded-lg">
    <div className="flex flex-col items-center gap-3">
      <div className="w-8 h-8 border-4 border-yellow-400 border-t-transparent rounded-full animate-spin"></div>
      <p className="text-gray-300 text-sm">Rendering visualization...</p>
    </div>
  </div>
);

const ChartErrorState = ({ error, onRetry }: { error: string; onRetry?: () => void }) => (
  <div className="w-full h-48 flex items-center justify-center bg-gray-900 rounded-lg">
    <div className="flex flex-col items-center gap-3 max-w-md">
      <p className="text-red-400 font-semibold text-center text-sm">An Error Occurred</p>
      <p className="text-red-300 text-sm text-center">{error}</p>
      {onRetry && (
        <button
          onClick={onRetry}
          className="px-4 py-2 mt-3 bg-yellow-500 text-gray-900 font-semibold rounded hover:bg-yellow-400 transition-colors"
        >
          <PencilRuler className="w-4 h-4 inline-block mr-2" />
          Regenerate
        </button>
      )}
    </div>
  </div>
);

const ChartHeader = ({ config, onRegenerate, updatedAt }: { config: ChartConfiguration; onRegenerate?: () => void; updatedAt?: string }) => {
  const formatTimestamp = (timestamp: string): string => {
    try {
      const date = new Date(timestamp);
      return date.toLocaleString('en-US', {
        month: 'short',
        day: 'numeric',
        year: 'numeric',
        hour: 'numeric',
        minute: '2-digit',
        hour12: true
      });
    } catch (e) {
      return timestamp;
    }
  };

  return (
    <div className="mb-4 pb-2 border-b border-gray-700/30">
      <div className="flex items-start justify-between">
        <div className="flex-1">
          <h3 className="text-base font-bold text-white mb-1">{config.title}</h3>
          {config.description && (
            <p className="text-sm text-gray-400 mb-3">{config.description}</p>
          )}
          {updatedAt && (
            <div className="flex items-center gap-1.5 text-xs text-gray-400">
              <Clock className="w-3.5 h-3.5" />
              <span>{formatTimestamp(updatedAt)}</span>
            </div>
          )}
        </div>
        {onRegenerate && (
          <TooltipComponent content="This is used to regenerate the graph with latest data.">
            <button
              onClick={onRegenerate}
              className="p-2 hover:bg-gray-800 rounded transition-colors text-gray-400 hover:text-gray-200 flex items-center gap-2 whitespace-nowrap ml-4 flex-shrink-0"
            >
              <RefreshCcw className="w-4 h-4" />
              <span className="text-sm font-semibold">Regenerate</span>
            </button>
          </TooltipComponent>
        )}
      </div>
    </div>
  );
};

// Line Chart Component
const LineChartComponent: React.FC<ChartRendererProps> = ({ config, data }) => {
  const colors = CHART_THEME.colors.primary; // Use yellow/green theme by default
  
  // Filter out null/undefined dates
  const cleanData = data.filter(item => {
    const dateKey = config.chart_render.x_axis.data_key;
    return item[dateKey] != null && item[dateKey] !== 'null' && item[dateKey] !== '';
  });

  return (
    <ResponsiveContainer width="100%" height={CHART_THEME.dimensions.height}>
      <LineChart
        data={cleanData}
        margin={{
          top: CHART_THEME.dimensions.marginTop,
          right: CHART_THEME.dimensions.marginRight,
          left: CHART_THEME.dimensions.marginLeft + 20,
          bottom: CHART_THEME.dimensions.marginBottom + 15,
        }}
      >
        {config.chart_render.features.grid && (
          <CartesianGrid strokeDasharray={CHART_THEME.grid.strokeDasharray} stroke={CHART_THEME.grid.stroke} />
        )}
        <XAxis
          dataKey={config.chart_render.x_axis.data_key}
          stroke={CHART_THEME.axis.stroke}
          style={{ fontSize: CHART_THEME.axis.fontSize, fontWeight: CHART_THEME.axis.fontWeight, fill: CHART_THEME.axis.stroke }}
          angle={config.chart_render.x_axis.data_key === 'date' ? -45 : 0}
          height={config.chart_render.x_axis.data_key === 'date' ? CHART_THEME.dimensions.xAxisHeight : 60}
          textAnchor="end"
          tick={{ fill: CHART_THEME.axis.tickColor }}
          label={{
            value: config.chart_render.x_axis.label,
            position: 'bottom' as const,
            offset: -20,
            fontSize: CHART_THEME.axis.labelFontSize,
            fontWeight: CHART_THEME.axis.labelFontWeight,
            fill: CHART_THEME.axis.labelColor,
          }}
        />
        {config.chart_render.y_axis && (
          <YAxis
            stroke={CHART_THEME.axis.stroke}
            style={{ fontSize: CHART_THEME.axis.fontSize, fontWeight: CHART_THEME.axis.fontWeight, fill: CHART_THEME.axis.stroke }}
            tick={{ fill: CHART_THEME.axis.tickColor }}
            label={{
              value: config.chart_render.y_axis.label,
              angle: -90,
              position: 'insideLeft' as const,
              offset: -20,
              fontSize: CHART_THEME.axis.labelFontSize,
              fontWeight: CHART_THEME.axis.labelFontWeight,
              fill: CHART_THEME.axis.labelColor,
            }}
          />
        )}
        {config.chart_render.features.tooltip && <Tooltip content={<CustomTooltip />} />}
        {config.chart_render.features.legend && (
          <Legend
            wrapperStyle={{
              fontSize: CHART_THEME.legend.fontSize,
              fontWeight: CHART_THEME.legend.fontWeight,
              paddingBottom: CHART_THEME.legend.paddingBottom,
              color: CHART_THEME.legend.color,
            }}
            verticalAlign="top"
            layout="horizontal"
          />
        )}
        {config.chart_render.series?.map((series, idx) => (
          <Line
            key={series.data_key}
            type={(series.type as 'monotone' | 'natural' | 'stepAfter') || 'monotone'}
            dataKey={series.data_key}
            name={series.name}
            stroke={series.stroke || colors[idx % colors.length]}
            strokeWidth={2}
            dot={{ fill: series.stroke || colors[idx % colors.length], r: 3 }}
            activeDot={{ r: 5, fill: '#FFFF00' }}
            isAnimationActive={CHART_THEME.animation.isAnimationActive}
          />
        ))}
      </LineChart>
    </ResponsiveContainer>
  );
};

// Bar Chart Component
const BarChartComponent: React.FC<ChartRendererProps> = ({ config, data }) => {
  const colors = CHART_THEME.colors.primary;
  
  // Filter out null/undefined dates
  const cleanData = data.filter(item => {
    const dateKey = config.chart_render.x_axis.data_key;
    return item[dateKey] != null && item[dateKey] !== 'null' && item[dateKey] !== '';
  });

  return (
    <ResponsiveContainer width="100%" height={CHART_THEME.dimensions.height}>
      <BarChart
        data={cleanData}
        margin={{
          top: CHART_THEME.dimensions.marginTop,
          right: CHART_THEME.dimensions.marginRight,
          left: CHART_THEME.dimensions.marginLeft,
          bottom: CHART_THEME.dimensions.marginBottom,
        }}
      >
        {config.chart_render.features.grid && (
          <CartesianGrid strokeDasharray={CHART_THEME.grid.strokeDasharray} stroke={CHART_THEME.grid.stroke} />
        )}
        <XAxis
          dataKey={config.chart_render.x_axis.data_key}
          stroke={CHART_THEME.axis.stroke}
          style={{ fontSize: CHART_THEME.axis.fontSize, fontWeight: CHART_THEME.axis.fontWeight, fill: CHART_THEME.axis.stroke }}
          angle={0}
          textAnchor="end"
          height={CHART_THEME.dimensions.xAxisHeight}
          tick={{ fill: CHART_THEME.axis.tickColor, dx: 25, dy: 10 }}
          label={{
            value: config.chart_render.x_axis.label,
            position: 'bottom' as const,
            offset: -25,
            fontSize: CHART_THEME.axis.labelFontSize,
            fontWeight: CHART_THEME.axis.labelFontWeight,
            fill: CHART_THEME.axis.labelColor,
          }}
        />
        {config.chart_render.y_axis && (
          <YAxis
            stroke={CHART_THEME.axis.stroke}
            style={{ fontSize: CHART_THEME.axis.fontSize, fontWeight: CHART_THEME.axis.fontWeight, fill: CHART_THEME.axis.stroke }}
            tick={{ fill: CHART_THEME.axis.tickColor }}
            label={{
              value: config.chart_render.y_axis.label,
              angle: -90,
              position: 'insideLeft' as const,
              offset: -4,
              dy: 20,
              fontSize: CHART_THEME.axis.labelFontSize,
              fontWeight: CHART_THEME.axis.labelFontWeight,
              fill: CHART_THEME.axis.labelColor,
            }}
          />
        )}
        {config.chart_render.features.tooltip && <Tooltip content={<CustomTooltip />} />}
        {config.chart_render.features.legend && (
          <Legend
            wrapperStyle={{
              fontSize: CHART_THEME.legend.fontSize,
              fontWeight: CHART_THEME.legend.fontWeight,
              paddingBottom: CHART_THEME.legend.paddingBottom,
              color: CHART_THEME.legend.color,
            }}
            verticalAlign="top"
            layout="horizontal"
          />
        )}
        {config.chart_render.series?.map((series, idx) => (
          <Bar
            key={series.data_key}
            dataKey={series.data_key}
            name={series.name}
            fill={series.fill || colors[idx % colors.length]}
            isAnimationActive={CHART_THEME.animation.isAnimationActive}
            radius={[10, 10, 0, 0]}
          />
        ))}
      </BarChart>
    </ResponsiveContainer>
  );
};

// Area Chart Component
const AreaChartComponent: React.FC<ChartRendererProps> = ({ config, data }) => {
  const colors = CHART_THEME.colors.primary;
  
  // Filter out null/undefined dates
  const cleanData = data.filter(item => {
    const dateKey = config.chart_render.x_axis.data_key;
    return item[dateKey] != null && item[dateKey] !== 'null' && item[dateKey] !== '';
  });

  return (
    <ResponsiveContainer width="100%" height={CHART_THEME.dimensions.height}>
      <AreaChart
        data={cleanData}
        margin={{
          top: CHART_THEME.dimensions.marginTop,
          right: CHART_THEME.dimensions.marginRight,
          left: CHART_THEME.dimensions.marginLeft,
          bottom: CHART_THEME.dimensions.marginBottom,
        }}
      >
        {config.chart_render.features.grid && (
          <CartesianGrid strokeDasharray={CHART_THEME.grid.strokeDasharray} stroke={CHART_THEME.grid.stroke} />
        )}
        <XAxis
          dataKey={config.chart_render.x_axis.data_key}
          stroke={CHART_THEME.axis.stroke}
          style={{ fontSize: CHART_THEME.axis.fontSize, fontWeight: CHART_THEME.axis.fontWeight, fill: CHART_THEME.axis.stroke }}
          angle={-45}
          textAnchor="end"
          height={CHART_THEME.dimensions.xAxisHeight}
          tick={{ fill: CHART_THEME.axis.tickColor }}
          label={{
            value: config.chart_render.x_axis.label,
            position: 'bottom' as const,
            offset: 15,
            fontSize: CHART_THEME.axis.labelFontSize,
            fontWeight: CHART_THEME.axis.labelFontWeight,
            fill: CHART_THEME.axis.labelColor,
          }}
        />
        {config.chart_render.y_axis && (
          <YAxis
            stroke={CHART_THEME.axis.stroke}
            style={{ fontSize: CHART_THEME.axis.fontSize, fontWeight: CHART_THEME.axis.fontWeight, fill: CHART_THEME.axis.stroke }}
            tick={{ fill: CHART_THEME.axis.tickColor }}
            label={{
              value: config.chart_render.y_axis.label,
              angle: -90,
              position: 'insideLeft' as const,
              offset: 10,
              fontSize: CHART_THEME.axis.labelFontSize,
              fontWeight: CHART_THEME.axis.labelFontWeight,
              fill: CHART_THEME.axis.labelColor,
            }}
          />
        )}
        {config.chart_render.features.tooltip && <Tooltip content={<CustomTooltip />} />}
        {config.chart_render.features.legend && (
          <Legend
            wrapperStyle={{
              fontSize: CHART_THEME.legend.fontSize,
              fontWeight: CHART_THEME.legend.fontWeight,
              paddingBottom: CHART_THEME.legend.paddingBottom,
              color: CHART_THEME.legend.color,
            }}
            verticalAlign="top"
            layout="horizontal"
          />
        )}
        {config.chart_render.series?.map((series, idx) => (
          <Area
            key={series.data_key}
            type={(series.type as 'monotone' | 'natural' | 'stepAfter') || 'monotone'}
            dataKey={series.data_key}
            name={series.name}
            stroke={series.stroke || colors[idx % colors.length]}
            fill={series.fill || colors[idx % colors.length]}
            fillOpacity={0.4}
            isAnimationActive={CHART_THEME.animation.isAnimationActive}
          />
        ))}
      </AreaChart>
    </ResponsiveContainer>
  );
};

// Pie Chart Component
const PieChartComponent: React.FC<ChartRendererProps> = ({ config, data }) => {
  const colors = CHART_THEME.colors.primary;
  const pieConfig = config.chart_render.pie;

  if (!pieConfig) return <ChartErrorState error="Pie configuration missing" />;

  return (
    <ResponsiveContainer width="100%" height={CHART_THEME.dimensions.height}>
      <PieChart>
        <Pie
          data={data}
          dataKey={pieConfig.data_key}
          nameKey={pieConfig.name_key}
          cx="50%"
          cy="50%"
          outerRadius={100}
          innerRadius={pieConfig.inner_radius || 0}
          paddingAngle={2}
          label={({ name, value }) => `${name}: ${value.toLocaleString()}`}
        >
          {data.map((_, idx) => (
            <Cell key={`cell-${idx}`} fill={colors[idx % colors.length]} />
          ))}
        </Pie>
        {config.chart_render.features.tooltip && <Tooltip content={<CustomTooltip />} />}
        {config.chart_render.features.legend && (
          <Legend
            wrapperStyle={{
              fontSize: CHART_THEME.legend.fontSize,
              fontWeight: CHART_THEME.legend.fontWeight,
              color: CHART_THEME.legend.color,
            }}
          />
        )}
      </PieChart>
    </ResponsiveContainer>
  );
};

// Scatter Chart Component
const ScatterChartComponent: React.FC<ChartRendererProps> = ({ config, data }) => {
  const colors = CHART_THEME.colors.primary;

  return (
    <ResponsiveContainer width="100%" height={CHART_THEME.dimensions.height}>
      <ScatterChart
        margin={{
          top: CHART_THEME.dimensions.marginTop,
          right: CHART_THEME.dimensions.marginRight,
          left: CHART_THEME.dimensions.scatterMarginLeft,
          bottom: CHART_THEME.dimensions.marginBottom,
        }}
      >
        {config.chart_render.features.grid && (
          <CartesianGrid strokeDasharray={CHART_THEME.grid.strokeDasharray} stroke={CHART_THEME.grid.stroke} />
        )}
        <XAxis
          type="number"
          dataKey={config.chart_render.x_axis.data_key}
          name={config.chart_render.x_axis.label}
          stroke={CHART_THEME.axis.stroke}
          style={{ fontSize: CHART_THEME.axis.fontSize, fontWeight: CHART_THEME.axis.fontWeight, fill: CHART_THEME.axis.stroke }}
          tick={{ fill: CHART_THEME.axis.tickColor }}
          label={{
            value: config.chart_render.x_axis.label,
            position: 'bottom' as const,
            offset: 15,
            fontSize: CHART_THEME.axis.labelFontSize,
            fontWeight: CHART_THEME.axis.labelFontWeight,
            fill: CHART_THEME.axis.labelColor,
          }}
        />
        {config.chart_render.y_axis && (
          <YAxis
            type="number"
            dataKey={config.chart_render.y_axis.data_key}
            name={config.chart_render.y_axis.label}
            stroke={CHART_THEME.axis.stroke}
            style={{ fontSize: CHART_THEME.axis.fontSize, fontWeight: CHART_THEME.axis.fontWeight, fill: CHART_THEME.axis.stroke }}
            tick={{ fill: CHART_THEME.axis.tickColor }}
            label={{
              value: config.chart_render.y_axis.label,
              angle: -90,
              position: 'insideLeft' as const,
              offset: 10,
              fontSize: CHART_THEME.axis.labelFontSize,
              fontWeight: CHART_THEME.axis.labelFontWeight,
              fill: CHART_THEME.axis.labelColor,
            }}
          />
        )}
        {config.chart_render.features.tooltip && <Tooltip content={<CustomTooltip />} />}
        {config.chart_render.features.legend && (
          <Legend
            wrapperStyle={{
              fontSize: CHART_THEME.legend.fontSize,
              fontWeight: CHART_THEME.legend.fontWeight,
              paddingBottom: CHART_THEME.legend.paddingBottom,
              color: CHART_THEME.legend.color,
            }}
            verticalAlign="top"
            layout="horizontal"
          />
        )}
        {config.chart_render.series?.map((series, idx) => (
          <Scatter
            key={series.data_key}
            dataKey={series.data_key}
            name={series.name}
            data={data}
            fill={series.fill || colors[idx % colors.length]}
            isAnimationActive={CHART_THEME.animation.isAnimationActive}
          />
        ))}
      </ScatterChart>
    </ResponsiveContainer>
  );
};

// Heatmap Component (using table with color intensity gradient)
const HeatmapComponent: React.FC<ChartRendererProps> = ({ config, data }) => {
  const heatmapConfig = config.chart_render.heatmap;
  if (!heatmapConfig) return <ChartErrorState error="Heatmap configuration missing" />;

  const colors = heatmapConfig.colors || CHART_THEME.colors.heatmap;

  // Group data by y_key for heatmap display
  const heatmapData: Record<string, any> = {};
  data.forEach((row) => {
    const yVal = row[heatmapConfig.y_key];
    if (!heatmapData[yVal]) {
      heatmapData[yVal] = { name: yVal };
    }
    heatmapData[yVal][row[heatmapConfig.x_key]] = row[heatmapConfig.value_key];
  });

  const xValues = [...new Set(data.map((d) => d[heatmapConfig.x_key]))].sort();
  const maxValue = Math.max(...data.map((d) => d[heatmapConfig.value_key]));

  const getColor = (value: number) => {
    const ratio = value / maxValue;
    const colorIndex = Math.floor(ratio * (colors.length - 1));
    return colors[colorIndex];
  };

  return (
    <div className="w-full overflow-x-auto">
      <table className="w-full text-xs border-collapse">
        <thead>
          <tr>
            <th className="bg-gray-800 text-white p-2 border border-gray-700 text-left font-bold">{heatmapConfig.y_key}</th>
            {xValues.map((xVal) => (
              <th key={xVal} className="bg-gray-800 text-white p-2 border border-gray-700 text-center font-bold">{xVal}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {data.reduce((rows: any[], row) => {
            const yVal = row[heatmapConfig.y_key];
            const xVal = row[heatmapConfig.x_key];
            const value = row[heatmapConfig.value_key];
            const existingRow = rows.find((r) => r.y === yVal);
            if (existingRow) {
              existingRow.values[xVal] = value;
            } else {
              rows.push({ y: yVal, values: { [xVal]: value } });
            }
            return rows;
          }, []).map((row) => (
            <tr key={row.y}>
              <td className="bg-gray-900 text-white p-2 border border-gray-700 font-semibold">{row.y}</td>
              {xValues.map((xVal) => (
                <td
                  key={xVal}
                  className="p-2 border border-gray-700 text-center font-bold text-white"
                  style={{ backgroundColor: getColor(row.values[xVal] || 0) }}
                >
                  {row.values[xVal] || '-'}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
};

// Funnel Component
const FunnelComponent: React.FC<ChartRendererProps> = ({ config, data }) => {
  const funnelConfig = config.chart_render.funnel;
  if (!funnelConfig) return <ChartErrorState error="Funnel configuration missing" />;

  const colors = funnelConfig.colors || CHART_THEME.colors.secondary;
  const maxValue = Math.max(...data.map((d) => d[funnelConfig.value_key]));

  return (
    <div className="w-full space-y-2 p-4">
      {data.map((row, idx) => {
        const stage = row[funnelConfig.stage_key];
        const value = row[funnelConfig.value_key];
        const percentage = ((value / maxValue) * 100).toFixed(1);
        const color = colors[idx % colors.length];

        return (
          <div key={stage} className="space-y-1">
            <div className="flex justify-between text-xs font-semibold">
              <span className="text-white">{stage}</span>
              <span className="text-gray-400">{value.toLocaleString()} ({percentage}%)</span>
            </div>
            <div className="w-full bg-gray-800 rounded-lg overflow-hidden h-8">
              <div
                className="h-full rounded-lg flex items-center justify-start pl-3 text-white text-xs font-bold transition-all"
                style={{ width: `${percentage}%`, backgroundColor: color }}
              >
                {percentage}%
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
};

// Bubble Chart Component
const BubbleChartComponent: React.FC<ChartRendererProps> = ({ config, data }) => {
  const bubbleConfig = config.chart_render.bubble;
  if (!bubbleConfig) return <ChartErrorState error="Bubble configuration missing" />;

  const colors = bubbleConfig.colors || CHART_THEME.colors.vibrant;
  const getColor = (idx: number) => colors[idx % colors.length];

  return (
    <ResponsiveContainer width="100%" height={CHART_THEME.dimensions.height}>
      <ScatterChart
        margin={{
          top: CHART_THEME.dimensions.marginTop,
          right: CHART_THEME.dimensions.marginRight,
          left: CHART_THEME.dimensions.scatterMarginLeft,
          bottom: CHART_THEME.dimensions.marginBottom,
        }}
        data={data}
      >
        {config.chart_render.features.grid && (
          <CartesianGrid strokeDasharray={CHART_THEME.grid.strokeDasharray} stroke={CHART_THEME.grid.stroke} />
        )}
        <XAxis
          type="number"
          dataKey={bubbleConfig.x_key}
          name={bubbleConfig.x_key}
          stroke={CHART_THEME.axis.stroke}
          style={{ fontSize: CHART_THEME.axis.fontSize, fontWeight: CHART_THEME.axis.fontWeight, fill: CHART_THEME.axis.stroke }}
          tick={{ fill: CHART_THEME.axis.tickColor }}
          label={{
            value: config.chart_render.x_axis?.label || bubbleConfig.x_key,
            position: 'bottom' as const,
            offset: 15,
            fontSize: CHART_THEME.axis.labelFontSize,
            fontWeight: CHART_THEME.axis.labelFontWeight,
            fill: CHART_THEME.axis.labelColor,
          }}
        />
        <YAxis
          type="number"
          dataKey={bubbleConfig.y_key}
          name={bubbleConfig.y_key}
          stroke={CHART_THEME.axis.stroke}
          style={{ fontSize: CHART_THEME.axis.fontSize, fontWeight: CHART_THEME.axis.fontWeight, fill: CHART_THEME.axis.stroke }}
          tick={{ fill: CHART_THEME.axis.tickColor }}
          label={{
            value: config.chart_render.y_axis?.label || bubbleConfig.y_key,
            angle: -90,
            position: 'insideLeft' as const,
            offset: 10,
            fontSize: CHART_THEME.axis.labelFontSize,
            fontWeight: CHART_THEME.axis.labelFontWeight,
            fill: CHART_THEME.axis.labelColor,
          }}
        />
        {config.chart_render.features.tooltip && <Tooltip content={<CustomTooltip />} />}
        {config.chart_render.features.legend && (
          <Legend
            wrapperStyle={{
              fontSize: CHART_THEME.legend.fontSize,
              fontWeight: CHART_THEME.legend.fontWeight,
              paddingBottom: CHART_THEME.legend.paddingBottom,
              color: CHART_THEME.legend.color,
            }}
            verticalAlign="top"
            layout="horizontal"
          />
        )}
        <Scatter
          name="Bubbles"
          data={data}
          fill={colors[0]}
          dataKey={bubbleConfig.size_key}
          isAnimationActive={CHART_THEME.animation.isAnimationActive}
        >
          {data.map((_, idx) => (
            <Cell key={`bubble-${idx}`} fill={getColor(idx)} />
          ))}
        </Scatter>
      </ScatterChart>
    </ResponsiveContainer>
  );
};

// Waterfall Component
const WaterfallComponent: React.FC<ChartRendererProps> = ({ config, data }) => {
  const waterfallConfig = config.chart_render.waterfall;
  if (!waterfallConfig) return <ChartErrorState error="Waterfall configuration missing" />;

  const colors = CHART_THEME.colors.waterfall;
  let runningTotal = 0;
  
  const waterfallData = data.map((row, idx) => {
    const value = row[waterfallConfig.value_key];
    const start = runningTotal;
    const isTotal = idx === data.length - 1; // Last item is total
    const isIncrease = value >= 0;
    
    runningTotal += value;

    return {
      category: row[waterfallConfig.category_key],
      value: isTotal ? runningTotal : Math.abs(value),
      start: isTotal ? 0 : start,
      end: isTotal ? runningTotal : runningTotal,
      type: isTotal ? 'total' : (isIncrease ? 'increase' : 'decrease'),
      isTotal,
      isIncrease,
      fill: isTotal ? colors.total : (isIncrease ? colors.increase : colors.decrease),
    };
  });

  return (
    <ResponsiveContainer width="100%" height={CHART_THEME.dimensions.height}>
      <BarChart 
        data={waterfallData} 
        margin={{ 
          top: CHART_THEME.dimensions.marginTop, 
          right: CHART_THEME.dimensions.marginRight, 
          left: CHART_THEME.dimensions.marginLeft, 
          bottom: CHART_THEME.dimensions.marginBottom 
        }}
      >
        {config.chart_render.features.grid && (
          <CartesianGrid strokeDasharray={CHART_THEME.grid.strokeDasharray} stroke={CHART_THEME.grid.stroke} />
        )}
        <XAxis
          dataKey="category"
          stroke={CHART_THEME.axis.stroke}
          style={{ fontSize: CHART_THEME.axis.fontSize, fontWeight: CHART_THEME.axis.fontWeight, fill: CHART_THEME.axis.stroke }}
          angle={-45}
          textAnchor="end"
          height={CHART_THEME.dimensions.xAxisHeight}
          tick={{ fill: CHART_THEME.axis.tickColor }}
          label={{ 
            value: config.chart_render.x_axis?.label || 'Categories', 
            position: 'bottom' as const, 
            offset: 15, 
            fontSize: CHART_THEME.axis.labelFontSize, 
            fontWeight: CHART_THEME.axis.labelFontWeight, 
            fill: CHART_THEME.axis.labelColor 
          }}
        />
        <YAxis 
          stroke={CHART_THEME.axis.stroke} 
          style={{ fontSize: CHART_THEME.axis.fontSize, fontWeight: CHART_THEME.axis.fontWeight, fill: CHART_THEME.axis.stroke }}
          tick={{ fill: CHART_THEME.axis.tickColor }}
          label={{ 
            value: config.chart_render.y_axis?.label || 'Value', 
            angle: -90, 
            position: 'insideLeft' as const, 
            offset: 10, 
            fontSize: CHART_THEME.axis.labelFontSize, 
            fontWeight: CHART_THEME.axis.labelFontWeight, 
            fill: CHART_THEME.axis.labelColor 
          }}
        />
        {config.chart_render.features.tooltip && <Tooltip content={<CustomTooltip />} />}
        {config.chart_render.features.legend && (
          <Legend 
            wrapperStyle={{ 
              fontSize: CHART_THEME.legend.fontSize, 
              fontWeight: CHART_THEME.legend.fontWeight, 
              paddingBottom: CHART_THEME.legend.paddingBottom, 
              color: CHART_THEME.legend.color 
            }} 
            verticalAlign="top" 
            layout="horizontal" 
          />
        )}
        {/* Waterfall bars */}
        <Bar 
          dataKey="value" 
          fill={colors.increase}
          radius={[6, 6, 0, 0]}
          isAnimationActive={CHART_THEME.animation.isAnimationActive}
        >
          {waterfallData.map((entry, idx) => (
            <Cell key={`waterfall-${idx}`} fill={entry.fill} />
          ))}
        </Bar>
        {/* Connector lines between bars */}
        {waterfallData.map((entry, idx) => {
          if (idx < waterfallData.length - 1) {
            return (
              <ReferenceLine 
                key={`ref-${idx}`}
                x={entry.category}
                stroke={CHART_THEME.axis.stroke}
                strokeDasharray="3 3"
                opacity={0.3}
              />
            );
          }
          return null;
        })}
      </BarChart>
    </ResponsiveContainer>
  );
};

// Main Chart Renderer Component
const ChartRenderer: React.FC<ChartRendererProps> = ({
  config,
  data,
  isLoading = false,
  error,
  onRetry,
  onRegenerate,
  updatedAt
}) => {
  const isEmpty = !data || data.length === 0;

  if (isLoading) return <ChartLoadingState />;
  if (error) return <ChartErrorState error={error} onRetry={onRetry} />;
  if (isEmpty) return <ChartErrorState error="No data available for visualization" onRetry={onRetry} />;

  const renderChart = () => {
    switch (config.chart_type) {
      case 'line':
        return <LineChartComponent config={config} data={data} />;
      case 'bar':
        return <BarChartComponent config={config} data={data} />;
      case 'pie':
        return <PieChartComponent config={config} data={data} />;
      case 'area':
        return <AreaChartComponent config={config} data={data} />;
      case 'scatter':
        return <ScatterChartComponent config={config} data={data} />;
      case 'heatmap':
        return <HeatmapComponent config={config} data={data} />;
      case 'funnel':
        return <FunnelComponent config={config} data={data} />;
      case 'bubble':
        return <BubbleChartComponent config={config} data={data} />;
      case 'waterfall':
        return <WaterfallComponent config={config} data={data} />;
      default:
        return <ChartErrorState error={`Unknown chart type: ${config.chart_type}`} />;
    }
  };

  return (
    <div className="w-full bg-gray-950 rounded-lg p-5">
      <ChartHeader config={config} onRegenerate={onRegenerate} updatedAt={updatedAt} />
      <div className="bg-gray-900 rounded-lg p-4 overflow-x-auto overflow-y-auto" style={{ maxHeight: '600px' }}>
        {renderChart()}
      </div>
    </div>
  );
};

export default ChartRenderer;
