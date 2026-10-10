import { useId, useMemo, useState, type KeyboardEvent, type ReactNode } from 'react';
import { Check, Copy, Plus, Search, X } from 'lucide-react';
import { cx } from './cx';
import { controlClass, Input } from './Field';

/* ---------- TokenInput: a list of strings typed as chips (replaces "a,b,c" text) ---------- */

export interface TokenInputProps {
  value: string[];
  onChange: (v: string[]) => void;
  placeholder?: string;
  /** Suggestions shown under the box while typing. */
  suggestions?: string[];
  /** Return an error message to reject a token. */
  validate?: (token: string) => string | null;
  mono?: boolean;
  id?: string;
}

export function TokenInput({ value, onChange, placeholder, suggestions, validate, mono, id }: TokenInputProps) {
  const [draft, setDraft] = useState('');
  const [err, setErr] = useState<string | null>(null);
  const autoId = useId();
  const inputId = id ?? autoId;

  const add = (raw: string) => {
    const parts = raw
      .split(/[,\n]/)
      .map((s) => s.trim())
      .filter(Boolean);
    const next = [...value];
    for (const p of parts) {
      const e = validate?.(p);
      if (e) {
        setErr(e);
        return;
      }
      if (!next.includes(p)) next.push(p);
    }
    setErr(null);
    onChange(next);
    setDraft('');
  };

  const onKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if ((e.key === 'Enter' || e.key === ',' || e.key === 'Tab') && draft.trim()) {
      e.preventDefault();
      add(draft);
    } else if (e.key === 'Backspace' && !draft && value.length) {
      onChange(value.slice(0, -1));
    }
  };

  const open = suggestions?.filter((s) => !value.includes(s) && s.toLowerCase().includes(draft.toLowerCase())) ?? [];

  return (
    <div className="flex flex-col gap-1.5">
      <div
        className={cx(
          'flex flex-wrap items-center gap-1.5 min-h-9 px-2 py-1 rounded-md border bg-surface',
          'focus-within:border-accent focus-within:ring-3 focus-within:ring-accent/20',
          err ? 'border-danger' : 'border-line-2',
        )}
        onClick={() => document.getElementById(inputId)?.focus()}
      >
        {value.map((t) => (
          <span
            key={t}
            className={cx(
              'inline-flex items-center gap-1 h-6 pl-2 pr-1 rounded-[6px] bg-sunken text-ink-2 text-[13px]',
              mono && 'font-mono text-[12px]',
            )}
          >
            {t}
            <button
              type="button"
              aria-label={`Remove ${t}`}
              className="w-4 h-4 inline-flex items-center justify-center rounded hover:bg-line-2"
              onClick={() => onChange(value.filter((v) => v !== t))}
            >
              <X className="w-3 h-3" />
            </button>
          </span>
        ))}
        <input
          id={inputId}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={onKey}
          onBlur={() => draft.trim() && add(draft)}
          onPaste={(e) => {
            const text = e.clipboardData.getData('text');
            if (/[,\n]/.test(text)) {
              e.preventDefault();
              add(text);
            }
          }}
          placeholder={value.length ? '' : placeholder}
          className={cx('flex-1 min-w-[80px] h-7 bg-transparent outline-none text-sm text-ink placeholder:text-ink-3', mono && 'font-mono text-[13px]')}
        />
      </div>
      {err && <p className="text-[12.5px] text-danger">{err}</p>}
      {draft && open.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {open.slice(0, 8).map((s) => (
            <button
              key={s}
              type="button"
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => add(s)}
              className="h-6 px-2 rounded-[6px] border border-dashed border-line-2 text-[12.5px] text-ink-2 hover:border-accent hover:text-accent"
            >
              + {s}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

/* ---------- ChipSelect: pick several from a fixed set (events, scopes, methods) ---------- */

export interface ChipOption {
  value: string;
  label?: ReactNode;
  description?: string;
}

export function ChipSelect({
  options,
  value,
  onChange,
  single,
  'aria-label': ariaLabel,
}: {
  options: ChipOption[];
  value: string[];
  onChange: (v: string[]) => void;
  single?: boolean;
  'aria-label'?: string;
}) {
  return (
    <div className="flex flex-wrap gap-1.5" role="group" aria-label={ariaLabel}>
      {options.map((o) => {
        const on = value.includes(o.value);
        return (
          <button
            key={o.value}
            type="button"
            aria-pressed={on}
            title={o.description}
            onClick={() =>
              onChange(single ? [o.value] : on ? value.filter((v) => v !== o.value) : [...value, o.value])
            }
            className={cx(
              'inline-flex items-center gap-1.5 h-[30px] px-3 rounded-full border text-[13px] font-medium transition-colors',
              on ? 'bg-accent-soft border-accent text-accent-2' : 'bg-surface border-line-2 text-ink-2 hover:border-ink-3',
            )}
          >
            {on && <Check className="w-3.5 h-3.5" aria-hidden />}
            {o.label ?? o.value}
          </button>
        );
      })}
    </div>
  );
}

/* ---------- Segmented control ---------- */

export function Segmented<T extends string>({
  options,
  value,
  onChange,
  size = 'md',
  'aria-label': ariaLabel,
}: {
  options: Array<{ value: T; label: ReactNode; count?: number }>;
  value: T;
  onChange: (v: T) => void;
  size?: 'sm' | 'md';
  'aria-label'?: string;
}) {
  return (
    <div className="inline-flex flex-wrap p-[3px] bg-sunken rounded-[9px] gap-0.5" role="radiogroup" aria-label={ariaLabel}>
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          role="radio"
          aria-checked={value === o.value}
          onClick={() => onChange(o.value)}
          className={cx(
            'inline-flex items-center gap-1.5 rounded-[7px] font-medium transition-colors',
            size === 'sm' ? 'h-6 px-2 text-[12.5px]' : 'h-7 px-3 text-[13px]',
            value === o.value ? 'bg-surface text-ink shadow-[0_1px_2px_rgba(13,16,23,.12)]' : 'text-ink-2 hover:text-ink',
          )}
        >
          {o.label}
          {o.count !== undefined && <span className="text-ink-3 text-[12px]">{o.count}</span>}
        </button>
      ))}
    </div>
  );
}

/* ---------- KeyValueEditor: headers, env vars, metadata ---------- */

export interface KV {
  key: string;
  value: string;
}

export function KeyValueEditor({
  value,
  onChange,
  keyPlaceholder = 'KEY',
  valuePlaceholder = 'value',
  addLabel = 'Add row',
  secretValues,
}: {
  value: KV[];
  onChange: (v: KV[]) => void;
  keyPlaceholder?: string;
  valuePlaceholder?: string;
  addLabel?: string;
  secretValues?: boolean;
}) {
  const set = (i: number, patch: Partial<KV>) => onChange(value.map((r, j) => (j === i ? { ...r, ...patch } : r)));
  const dupes = useMemo(() => {
    const seen = new Set<string>();
    const d = new Set<string>();
    value.forEach((r) => (seen.has(r.key) && r.key ? d.add(r.key) : seen.add(r.key)));
    return d;
  }, [value]);
  return (
    <div className="flex flex-col gap-2">
      {value.map((r, i) => (
        <div key={i} className="flex gap-2 items-start">
          <Input
            mono
            aria-label="Key"
            placeholder={keyPlaceholder}
            value={r.key}
            aria-invalid={dupes.has(r.key) || undefined}
            onChange={(e) => set(i, { key: e.target.value })}
            className="flex-[2]"
          />
          <Input
            mono
            aria-label="Value"
            type={secretValues ? 'password' : 'text'}
            placeholder={valuePlaceholder}
            value={r.value}
            onChange={(e) => set(i, { value: e.target.value })}
            className="flex-[3]"
          />
          <button
            type="button"
            aria-label="Remove row"
            className="w-9 h-9 shrink-0 inline-flex items-center justify-center rounded-md text-ink-3 hover:text-danger hover:bg-danger-soft"
            onClick={() => onChange(value.filter((_, j) => j !== i))}
          >
            <X className="w-4 h-4" />
          </button>
        </div>
      ))}
      {dupes.size > 0 && <p className="text-[12.5px] text-danger">Each key can appear once.</p>}
      <button
        type="button"
        onClick={() => onChange([...value, { key: '', value: '' }])}
        className="self-start inline-flex items-center gap-1.5 h-[30px] px-2.5 rounded-[7px] text-[13px] font-medium text-ink-2 hover:bg-sunken"
      >
        <Plus className="w-3.5 h-3.5" /> {addLabel}
      </button>
    </div>
  );
}

export function kvFromRecord(r: Record<string, string> | undefined | null): KV[] {
  return Object.entries(r ?? {}).map(([key, value]) => ({ key, value }));
}
export function kvToRecord(rows: KV[]): Record<string, string> {
  const out: Record<string, string> = {};
  rows.forEach((r) => r.key.trim() && (out[r.key.trim()] = r.value));
  return out;
}

/* ---------- ScheduleInput: cron with presets and a plain-language reading ---------- */

const PRESETS: Array<{ label: string; cron: string }> = [
  { label: 'Every minute', cron: '* * * * *' },
  { label: 'Every 5 min', cron: '*/5 * * * *' },
  { label: 'Hourly', cron: '0 * * * *' },
  { label: 'Daily 03:00', cron: '0 3 * * *' },
  { label: 'Mondays 09:00', cron: '0 9 * * 1' },
  { label: 'Monthly', cron: '0 0 1 * *' },
];

const DAYS = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];

/** Best-effort plain-language reading of a 5-field cron expression. */
export function describeCron(expr: string): { ok: boolean; text: string } {
  const p = expr.trim().split(/\s+/);
  if (p.length !== 5) return { ok: false, text: 'Use five fields: minute hour day month weekday.' };
  const [mi, h, dom, mon, dow] = p;
  const valid = p.every((f) => /^(\*|\d+)(-\d+)?(\/\d+)?(,(\*|\d+)(-\d+)?(\/\d+)?)*$/.test(f));
  if (!valid) return { ok: false, text: 'That does not look like a cron expression.' };
  const two = (n: string) => n.padStart(2, '0');
  if (p.every((f) => f === '*')) return { ok: true, text: 'Every minute' };
  if (/^\*\/\d+$/.test(mi) && h === '*' && dom === '*' && mon === '*' && dow === '*')
    return { ok: true, text: `Every ${mi.slice(2)} minutes` };
  if (/^\d+$/.test(mi) && h === '*' && dom === '*' && mon === '*' && dow === '*')
    return { ok: true, text: `Every hour at minute ${mi}` };
  if (/^\d+$/.test(mi) && /^\d+$/.test(h)) {
    const at = `${two(h)}:${two(mi)}`;
    if (dom === '*' && mon === '*' && dow === '*') return { ok: true, text: `Every day at ${at} (server time)` };
    if (dom === '*' && mon === '*' && /^\d$/.test(dow)) return { ok: true, text: `Every ${DAYS[+dow % 7]} at ${at}` };
    if (/^\d+$/.test(dom) && mon === '*' && dow === '*') return { ok: true, text: `Day ${dom} of every month at ${at}` };
  }
  return { ok: true, text: `Custom schedule: ${expr.trim()}` };
}

export function ScheduleInput({ value, onChange, id }: { value: string; onChange: (v: string) => void; id?: string }) {
  const d = describeCron(value);
  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap gap-1.5">
        {PRESETS.map((p) => (
          <button
            key={p.cron}
            type="button"
            onClick={() => onChange(p.cron)}
            className={cx(
              'h-[26px] px-2.5 rounded-full border text-[12.5px] font-medium',
              value.trim() === p.cron ? 'bg-accent-soft border-accent text-accent-2' : 'border-line-2 text-ink-2 hover:border-ink-3',
            )}
          >
            {p.label}
          </button>
        ))}
      </div>
      <Input id={id} mono value={value} onChange={(e) => onChange(e.target.value)} placeholder="*/5 * * * *" aria-invalid={!d.ok || undefined} />
      <p className={cx('text-[12.5px] leading-[18px]', d.ok ? 'text-ink-2' : 'text-danger')}>{d.text}</p>
    </div>
  );
}

/* ---------- SearchInput ---------- */

export function SearchInput({
  value,
  onChange,
  placeholder = 'Search',
  className,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  className?: string;
}) {
  return (
    <div className={cx('relative', className)}>
      <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-ink-3 pointer-events-none" aria-hidden />
      <input
        type="search"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        aria-label={placeholder}
        className={cx(controlClass, 'pl-8')}
      />
    </div>
  );
}

/* ---------- CopyButton ---------- */

export function CopyButton({ text, label = 'Copy', className }: { text: string; label?: string; className?: string }) {
  const [done, setDone] = useState(false);
  return (
    <button
      type="button"
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(text);
          setDone(true);
          setTimeout(() => setDone(false), 1400);
        } catch {
          /* clipboard blocked; nothing to do */
        }
      }}
      className={cx(
        'inline-flex items-center gap-1.5 h-[30px] px-2.5 rounded-[7px] border border-line-2 bg-surface text-[13px] font-medium text-ink-2 hover:bg-sunken',
        className,
      )}
    >
      {done ? <Check className="w-3.5 h-3.5 text-ok" /> : <Copy className="w-3.5 h-3.5" />}
      {done ? 'Copied' : label}
    </button>
  );
}

