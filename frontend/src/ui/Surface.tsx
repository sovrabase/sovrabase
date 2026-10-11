import type { ReactNode } from 'react';
import { Loader2 } from 'lucide-react';
import { cx } from './cx';

/* ---------- Page scaffolding ---------- */

/** Standard page body: consistent padding and max width. */
export function Page({ children, wide, className }: { children: ReactNode; wide?: boolean; className?: string }) {
  return (
    <div className={cx('px-4 md:px-7 pt-6 pb-24 md:pb-16 mx-auto w-full animate-fade', wide ? 'max-w-[1400px]' : 'max-w-[1180px]', className)}>
      {children}
    </div>
  );
}

/** Title row of a page: h1, one-line description, actions on the right. */
export function PageHeader({
  title,
  description,
  actions,
  meta,
}: {
  title: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  meta?: ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-start justify-between gap-4 mb-6">
      <div className="flex flex-col gap-1 min-w-0">
        <div className="flex items-center gap-2.5 flex-wrap">
          <h1 className="m-0 text-[22px] leading-7 font-semibold text-ink tracking-[-0.01em]">{title}</h1>
          {meta}
        </div>
        {description && <p className="m-0 text-sm text-ink-3 max-w-[720px]">{description}</p>}
      </div>
      {actions && <div className="flex items-center gap-2 flex-wrap">{actions}</div>}
    </div>
  );
}

/**
 * Settings-style section: title and explanation on the left, controls on the right.
 * Stacks on narrow screens.
 */
export function Section({
  title,
  description,
  children,
  last,
}: {
  title: ReactNode;
  description?: ReactNode;
  children: ReactNode;
  last?: boolean;
}) {
  return (
    <section className={cx('flex flex-wrap gap-x-7 gap-y-4 py-7', !last && 'border-b border-line')}>
      <div className="flex-[1_1_220px] flex flex-col gap-1.5">
        <h2 className="m-0 text-base font-semibold text-ink">{title}</h2>
        {description && <p className="m-0 text-[13px] leading-[19px] text-ink-3">{description}</p>}
      </div>
      <div className="flex-[3_1_460px] min-w-0 flex flex-col gap-4">{children}</div>
    </section>
  );
}

export function Card({ children, className, padded = true }: { children: ReactNode; className?: string; padded?: boolean }) {
  return <div className={cx('bg-surface border border-line rounded-lg', padded && 'p-5', className)}>{children}</div>;
}

export function CardHeader({
  title,
  description,
  actions,
  className,
}: {
  title: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  className?: string;
}) {
  return (
    <div className={cx('flex flex-wrap items-center justify-between gap-3 px-5 py-4', className)}>
      <div className="flex flex-col min-w-0">
        <h3 className="m-0 text-[15px] font-semibold text-ink">{title}</h3>
        {description && <p className="m-0 text-[12.5px] leading-[18px] text-ink-3">{description}</p>}
      </div>
      {actions && <div className="flex items-center gap-2">{actions}</div>}
    </div>
  );
}

/** A row inside a Card list: bordered top, flexible content. */
export function Row({ children, className, onClick }: { children: ReactNode; className?: string; onClick?: () => void }) {
  return (
    <div
      className={cx(
        'flex flex-wrap items-center gap-3 px-5 py-3 border-t border-line first:border-t-0',
        onClick && 'cursor-pointer hover:bg-hover',
        className,
      )}
      onClick={onClick}
    >
      {children}
    </div>
  );
}

/* ---------- Status ---------- */

export type Tone = 'neutral' | 'ok' | 'warn' | 'danger' | 'accent';

const tones: Record<Tone, string> = {
  neutral: 'bg-sunken text-ink-2',
  ok: 'bg-ok-soft text-ok',
  warn: 'bg-warn-soft text-warn',
  danger: 'bg-danger-soft text-danger',
  accent: 'bg-accent-soft text-accent-2',
};

export function Badge({ tone = 'neutral', dot, children, className }: { tone?: Tone; dot?: boolean; children: ReactNode; className?: string }) {
  return (
    <span className={cx('inline-flex items-center gap-1.5 h-[22px] px-2 rounded-full text-[12px] font-medium whitespace-nowrap', tones[tone], className)}>
      {dot && <span className="w-[7px] h-[7px] rounded-full bg-current shrink-0" aria-hidden />}
      {children}
    </span>
  );
}

export function Kbd({ children }: { children: ReactNode }) {
  return (
    <kbd className="font-mono text-[11px] leading-4 px-[5px] border border-line-2 border-b-2 rounded-[5px] text-ink-3 bg-surface">
      {children}
    </kbd>
  );
}

export function Spinner({ label = 'Loading' }: { label?: string }) {
  return (
    <div className="flex items-center justify-center gap-2 py-16 text-ink-3" role="status">
      <Loader2 className="w-5 h-5 animate-spin" aria-hidden />
      <span className="text-sm">{label}</span>
    </div>
  );
}

export function Skeleton({ className }: { className?: string }) {
  return <div className={cx('rounded-md bg-sunken animate-pulse', className)} aria-hidden />;
}

export function EmptyState({
  icon,
  title,
  description,
  action,
  className,
}: {
  icon?: ReactNode;
  title: ReactNode;
  description?: ReactNode;
  action?: ReactNode;
  className?: string;
}) {
  return (
    <div className={cx('flex flex-col items-center text-center gap-2 py-14 px-6', className)}>
      {icon && <div className="w-10 h-10 rounded-[10px] bg-sunken text-ink-3 flex items-center justify-center mb-1">{icon}</div>}
      <p className="m-0 text-[15px] font-semibold text-ink">{title}</p>
      {description && <p className="m-0 text-[13px] text-ink-3 max-w-[420px]">{description}</p>}
      {action && <div className="mt-3">{action}</div>}
    </div>
  );
}

/** Inline banner for errors, warnings or notes inside a page. */
export function Banner({ tone = 'neutral', title, children, action }: { tone?: Tone; title?: ReactNode; children?: ReactNode; action?: ReactNode }) {
  const border: Record<Tone, string> = {
    neutral: 'border-line',
    ok: 'border-ok/30',
    warn: 'border-warn/30',
    danger: 'border-danger/30',
    accent: 'border-accent/30',
  };
  return (
    <div className={cx('flex flex-wrap items-center gap-3 px-4 py-3 rounded-lg border', tones[tone], border[tone])} role={tone === 'danger' ? 'alert' : undefined}>
      <div className="flex-1 min-w-[200px] text-[13px] leading-[19px]">
        {title && <b className="block text-[13.5px]">{title}</b>}
        {children && <span className={title ? 'opacity-90' : ''}>{children}</span>}
      </div>
      {action}
    </div>
  );
}

/** Big number tile. */
export function Stat({ label, value, sub, className }: { label: ReactNode; value: ReactNode; sub?: ReactNode; className?: string }) {
  return (
    <div className={cx('flex flex-col gap-0.5', className)}>
      <span className="text-[12.5px] text-ink-3">{label}</span>
      <b className="text-[24px] leading-8 font-semibold text-ink tabular-nums">{value}</b>
      {sub && <span className="text-[12.5px] text-ink-3">{sub}</span>}
    </div>
  );
}

export function CodeBlock({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <pre className={cx('m-0 p-4 rounded-[10px] bg-code text-code-ink font-mono text-[12.5px] leading-5 overflow-x-auto whitespace-pre', className)}>
      {children}
    </pre>
  );
}

/* ---------- In-page tabs ---------- */

export function Tabs<T extends string>({
  tabs,
  value,
  onChange,
  className,
}: {
  tabs: Array<{ value: T; label: ReactNode; count?: number }>;
  value: T;
  onChange: (v: T) => void;
  className?: string;
}) {
  return (
    <div className={cx('flex gap-1 border-b border-line overflow-x-auto no-scrollbar', className)} role="tablist">
      {tabs.map((t) => (
        <button
          key={t.value}
          type="button"
          role="tab"
          aria-selected={value === t.value}
          onClick={() => onChange(t.value)}
          className={cx(
            'h-9 px-3 -mb-px border-b-2 text-[13.5px] font-medium whitespace-nowrap inline-flex items-center gap-1.5',
            value === t.value ? 'border-accent text-ink' : 'border-transparent text-ink-3 hover:text-ink',
          )}
        >
          {t.label}
          {t.count !== undefined && <span className="text-[12px] text-ink-3">{t.count}</span>}
        </button>
      ))}
    </div>
  );
}
