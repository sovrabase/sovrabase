import type { ReactNode } from 'react';
import { Database, Globe, KeyRound, Radio, Send, ShieldCheck, Webhook, Zap } from 'lucide-react';
import { Badge, cx } from '../../../ui';
import type { Tone } from '../../../ui';
import type { Adds, CatalogEntry, Permission, PermissionKind } from './manifest';

/** Square letter tile for a plugin. Neutral on purpose: colour is kept for state. */
export function PluginIcon({ def, size = 'md' }: { def: Pick<CatalogEntry, 'icon' | 'name'>; size?: 'sm' | 'md' | 'lg' }) {
  const s = { sm: 'w-7 h-7 text-[12px] rounded-[7px]', md: 'w-10 h-10 text-[15px] rounded-[10px]', lg: 'w-12 h-12 text-[18px] rounded-[12px]' }[size];
  return (
    <span className={cx('shrink-0 inline-flex items-center justify-center font-semibold bg-accent-soft text-accent-2', s)} aria-hidden>
      {def.icon || def.name.charAt(0)}
    </span>
  );
}

const permIcon: Record<PermissionKind, ReactNode> = {
  network: <Globe className="w-4 h-4" />,
  secret: <KeyRound className="w-4 h-4" />,
  webhook: <Webhook className="w-4 h-4" />,
  events: <Send className="w-4 h-4" />,
  data: <ShieldCheck className="w-4 h-4" />,
};

export function PermissionList({ items, granted }: { items: Permission[]; granted?: boolean }) {
  return (
    <ul className="m-0 p-0 list-none flex flex-col gap-3.5">
      {items.map((p) => (
        <li key={p.title} className="flex items-start gap-3">
          <span
            className={cx(
              'w-8 h-8 shrink-0 rounded-md inline-flex items-center justify-center',
              p.kind === 'data' ? 'bg-ok-soft text-ok' : 'bg-sunken text-ink-2',
            )}
            aria-hidden
          >
            {permIcon[p.kind]}
          </span>
          <span className="flex-1 min-w-0 flex flex-col">
            <span className="text-sm font-medium text-ink">{p.title}</span>
            <span className="text-[12.5px] leading-[18px] text-ink-3">{p.detail}</span>
          </span>
          {granted && p.kind !== 'data' && <Badge tone="ok">Granted</Badge>}
        </li>
      ))}
    </ul>
  );
}

function Group({ icon, title, children }: { icon: ReactNode; title: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-2">
      <h4 className="m-0 flex items-center gap-1.5 text-[12px] font-semibold uppercase tracking-[.06em] text-ink-3">
        {icon}
        {title}
      </h4>
      {children}
    </div>
  );
}

/** Actions, triggers and routes a plugin adds to a project. */
export function AddsList({ adds, triggersAside }: { adds: Adds; triggersAside?: ReactNode }) {
  const none = !adds.actions.length && !adds.triggers.length && !adds.routes.length;
  if (none) return <p className="m-0 text-sm text-ink-3">This plugin only stores settings.</p>;
  return (
    <div className="flex flex-col gap-5">
      {adds.actions.length > 0 && (
        <Group icon={<Zap className="w-3.5 h-3.5" aria-hidden />} title="Actions">
          <ul className="m-0 p-0 list-none flex flex-col gap-1.5">
            {adds.actions.map((a) => (
              <li key={a.name} className="flex flex-wrap items-baseline gap-x-2 text-sm">
                <span className="text-ink font-medium">{a.label}</span>
                <code className="font-mono text-[12px] text-ink-3">{a.name}</code>
                {a.input && <span className="text-[12.5px] text-ink-3 basis-full">{a.input}</span>}
              </li>
            ))}
          </ul>
          <p className="m-0 text-[12.5px] text-ink-3">
            Your app calls them with <code className="font-mono">POST /api/v1/integrations/…/action</code>, so secret keys stay on the server.
          </p>
        </Group>
      )}
      {adds.triggers.length > 0 && (
        <Group icon={<Radio className="w-3.5 h-3.5" aria-hidden />} title="Triggers">
          <div className="flex flex-wrap gap-1.5">
            {adds.triggers.map((t) => (
              <Badge key={t}>{t}</Badge>
            ))}
          </div>
          {triggersAside}
        </Group>
      )}
      {adds.routes.length > 0 && (
        <Group icon={<Database className="w-3.5 h-3.5" aria-hidden />} title="Route">
          {adds.routes.map((r) => (
            <div key={r.path} className="flex flex-col gap-0.5">
              <code className="font-mono text-[12.5px] text-ink-2 break-all">
                <span className="font-semibold text-accent-2">{r.method}</span> {r.path}
              </code>
              <span className="text-[12.5px] text-ink-3">{r.note}</span>
            </div>
          ))}
        </Group>
      )}
    </div>
  );
}

const METHOD_TONES: Record<string, Tone> = { GET: 'ok', POST: 'accent', PUT: 'warn', PATCH: 'warn', DELETE: 'danger' };
export const methodTone = (m: string): Tone => METHOD_TONES[m.toUpperCase()] ?? 'neutral';
