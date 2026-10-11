import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';

export interface Point {
  label: string;
  value: number;
  /** Longer label for the tooltip. */
  title?: string;
}

const axisTick = { fill: 'var(--sv-ink-3)', fontSize: 11.5 };

function ChartTooltip({ active, payload, unit }: { active?: boolean; payload?: Array<{ payload: Point }>; unit: string }) {
  if (!active || !payload?.length) return null;
  const p = payload[0].payload;
  return (
    <div className="px-3 py-2 rounded-md border border-line bg-surface shadow-pop text-[12.5px]">
      <div className="text-ink-3">{p.title ?? p.label}</div>
      <div className="text-ink font-semibold tabular-nums">
        {p.value.toLocaleString()} {unit}
      </div>
    </div>
  );
}

/** Single-series bar chart: accent bars, recessive grid, hover tooltip. */
export function SingleBarChart({
  data,
  unit,
  height = 220,
  ariaLabel,
  tickInterval = 0,
}: {
  data: Point[];
  unit: string;
  height?: number;
  ariaLabel: string;
  tickInterval?: number | 'preserveStartEnd';
}) {
  return (
    <div role="img" aria-label={ariaLabel} style={{ height }}>
      <ResponsiveContainer width="100%" height="100%">
        <BarChart data={data} margin={{ top: 8, right: 4, bottom: 0, left: -12 }} barCategoryGap={2}>
          <CartesianGrid vertical={false} stroke="var(--sv-line)" strokeDasharray="0" />
          <XAxis dataKey="label" tick={axisTick} tickLine={false} axisLine={{ stroke: 'var(--sv-line)' }} interval={tickInterval} />
          <YAxis tick={axisTick} tickLine={false} axisLine={false} allowDecimals={false} width={48} />
          <Tooltip cursor={{ fill: 'var(--sv-sunken)' }} content={<ChartTooltip unit={unit} />} isAnimationActive={false} />
          <Bar dataKey="value" fill="var(--sv-accent)" radius={[4, 4, 0, 0]} maxBarSize={36} isAnimationActive={false} />
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
