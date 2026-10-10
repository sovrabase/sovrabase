import type { ReactNode } from 'react';
import { Card } from '../../../ui';

/** One numbered step of an automation (trigger, action…). */
export function Step({ n, eyebrow, title, children }: { n: number; eyebrow: string; title: ReactNode; children: ReactNode }) {
  return (
    <Card padded={false}>
      <div className="flex items-center gap-3 px-5 pt-4">
        <span className="w-6 h-6 rounded-[7px] bg-ink text-bg text-[12px] font-bold flex items-center justify-center shrink-0" aria-hidden>
          {n}
        </span>
        <div className="flex flex-col min-w-0">
          <span className="text-[12px] font-semibold uppercase tracking-[.06em] text-ink-3">{eyebrow}</span>
          <h2 className="m-0 text-[15px] font-semibold text-ink">{title}</h2>
        </div>
      </div>
      <div className="px-5 pt-4 pb-5 flex flex-col gap-4">{children}</div>
    </Card>
  );
}

export function Join() {
  return <div className="w-0.5 h-5 bg-line-2 mx-auto" aria-hidden />;
}

/** Main column + side panel; stacks on narrow screens. */
export function BuilderLayout({ main, side }: { main: ReactNode; side?: ReactNode }) {
  return (
    <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_340px] items-start">
      <div className="flex flex-col min-w-0">{main}</div>
      {side && <div className="flex flex-col gap-4 min-w-0">{side}</div>}
    </div>
  );
}
