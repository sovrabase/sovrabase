import type { ReactNode } from 'react';
import { cx } from './cx';

export interface Column<T> {
  key: string;
  header: ReactNode;
  cell: (row: T) => ReactNode;
  /** CSS width, e.g. "120px" or "1fr". Defaults to 1fr. */
  width?: string;
  align?: 'left' | 'right';
  /** Hide on phones; the first two visible columns always show as the card title. */
  hideOnMobile?: boolean;
}

/**
 * Table that turns into a list of cards on phones.
 * Rows are clickable when `onRowClick` is given (Enter/Space work too).
 */
export function DataTable<T>({
  columns,
  rows,
  rowKey,
  onRowClick,
  selectedKey,
  empty,
  className,
}: {
  columns: Column<T>[];
  rows: T[];
  rowKey: (row: T) => string;
  onRowClick?: (row: T) => void;
  selectedKey?: string | null;
  empty?: ReactNode;
  className?: string;
}) {
  if (!rows.length && empty) return <>{empty}</>;
  const template = columns.map((c) => c.width ?? 'minmax(0,1fr)').join(' ');
  return (
    <div className={cx('w-full', className)}>
      {/* Desktop */}
      <div className="hidden md:block overflow-x-auto" role="table">
        <div
          role="row"
          className="grid gap-4 px-5 h-9 items-center border-b border-line text-[12px] font-semibold uppercase tracking-[.05em] text-ink-3"
          style={{ gridTemplateColumns: template }}
        >
          {columns.map((c) => (
            <div key={c.key} role="columnheader" className={cx('truncate', c.align === 'right' && 'text-right')}>
              {c.header}
            </div>
          ))}
        </div>
        {rows.map((r) => {
          const k = rowKey(r);
          return (
            <div
              key={k}
              role="row"
              tabIndex={onRowClick ? 0 : undefined}
              onClick={onRowClick ? () => onRowClick(r) : undefined}
              onKeyDown={
                onRowClick
                  ? (e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        onRowClick(r);
                      }
                    }
                  : undefined
              }
              className={cx(
                'grid gap-4 px-5 min-h-[48px] py-2 items-center border-b border-line last:border-b-0 text-sm',
                onRowClick && 'cursor-pointer hover:bg-hover',
                selectedKey === k && 'bg-accent-soft/60',
              )}
              style={{ gridTemplateColumns: template }}
            >
              {columns.map((c) => (
                <div key={c.key} role="cell" className={cx('min-w-0', c.align === 'right' && 'text-right')}>
                  {c.cell(r)}
                </div>
              ))}
            </div>
          );
        })}
      </div>
      {/* Phone: cards */}
      <ul className="md:hidden m-0 p-0 list-none">
        {rows.map((r) => {
          const k = rowKey(r);
          const vis = columns.filter((c) => !c.hideOnMobile);
          const [a, b, ...rest] = vis;
          return (
            <li
              key={k}
              onClick={onRowClick ? () => onRowClick(r) : undefined}
              className={cx('flex flex-col gap-1 px-4 py-3 border-b border-line last:border-b-0', onRowClick && 'active:bg-hover cursor-pointer')}
            >
              <div className="flex items-center justify-between gap-3 min-w-0">
                <div className="min-w-0 font-medium">{a?.cell(r)}</div>
                {b && <div className="shrink-0">{b.cell(r)}</div>}
              </div>
              {rest.length > 0 && (
                <div className="flex flex-wrap gap-x-3 gap-y-0.5 text-[12.5px] text-ink-3">
                  {rest.map((c) => (
                    <span key={c.key} className="min-w-0">
                      {c.cell(r)}
                    </span>
                  ))}
                </div>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}
