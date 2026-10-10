import { Area, AreaChart, Bar, BarChart, CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';

const axis = { fontSize: 11, fill: 'var(--sv-ink-3)' };

function Tip({ active, payload, label, format, name }: {
  active?: boolean;
  payload?: Array<{ value?: number | string }>;
  label?: string | number;
  format: (v: number) => string;
  name: string;
}) {
  if (!active || !payload?.length) return null;
  return (
    <div className="rounded-md border border-line bg-surface shadow-pop px-2.5 py-1.5 text-[12px]">
      <div className="text-ink-3">{label}</div>
      <div className="text-ink font-medium tabular-nums">
        {name}: {format(Number(payload[0].value ?? 0))}
      </div>
    </div>
  );
}

/** One measure over time. One series, one axis, accent colour. */
export function SeriesChart({
  data,
  x,
  y,
  name,
  kind = 'line',
  format = (v) => String(v),
  height = 150,
}: {
  data: object[];
  x: string;
  y: string;
  name: string;
  kind?: 'line' | 'bar' | 'area';
  format?: (v: number) => string;
  height?: number;
}) {
  const common = {
    data,
    margin: { top: 6, right: 4, bottom: 0, left: 0 },
  };
  const grid = <CartesianGrid vertical={false} stroke="var(--sv-line)" />;
  const xa = <XAxis dataKey={x} tick={axis} tickLine={false} axisLine={{ stroke: 'var(--sv-line)' }} minTickGap={24} />;
  const ya = <YAxis tick={axis} tickLine={false} axisLine={false} width={48} tickFormatter={(v: number) => format(v)} />;
  const tip = <Tooltip cursor={{ stroke: 'var(--sv-line-2)', fill: 'var(--sv-hover)' }} content={<Tip format={format} name={name} />} />;
  return (
    <div style={{ height }} role="img" aria-label={`${name} chart`}>
      <ResponsiveContainer width="100%" height="100%">
        {kind === 'bar' ? (
          <BarChart {...common} barCategoryGap={2}>
            {grid}
            {xa}
            {ya}
            {tip}
            <Bar dataKey={y} name={name} fill="var(--sv-accent)" radius={[4, 4, 0, 0]} maxBarSize={18} isAnimationActive={false} />
          </BarChart>
        ) : kind === 'area' ? (
          <AreaChart {...common}>
            {grid}
            {xa}
            {ya}
            {tip}
            <Area type="monotone" dataKey={y} name={name} stroke="var(--sv-accent)" strokeWidth={2} fill="var(--sv-accent-soft)" isAnimationActive={false} />
          </AreaChart>
        ) : (
          <LineChart {...common}>
            {grid}
            {xa}
            {ya}
            {tip}
            <Line type="monotone" dataKey={y} name={name} stroke="var(--sv-accent)" strokeWidth={2} dot={false} activeDot={{ r: 4 }} isAnimationActive={false} />
          </LineChart>
        )}
      </ResponsiveContainer>
    </div>
  );
}
