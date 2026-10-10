import { Link } from 'react-router-dom';
import { ChevronRight, ExternalLink, Server } from 'lucide-react';
import type { PluginInfo } from '../../../types';
import { Badge, EmptyState } from '../../../ui';
import { methodTone } from './PluginBits';
import { PLUGIN_API_DOCS } from './manifest';

const HOOK_LABELS: Record<string, string> = {
  record: 'Record hooks',
  auth: 'Auth hooks',
  storage: 'Storage hooks',
  realtime: 'Realtime hooks',
  email: 'Email hooks',
};

export function hasServerPlugins(info: PluginInfo | null) {
  return !!info && (info.plugins.length > 0 || info.hooks.length > 0 || info.routes.length > 0);
}

/** Registered Go plugins, their hooks grouped by type, and custom routes. */
export function ServerPlugins({ info, showNames = true }: { info: PluginInfo | null; showNames?: boolean }) {
  if (!hasServerPlugins(info) || !info) {
    return (
      <EmptyState
        icon={<Server className="w-5 h-5" />}
        title="No server plugins in this build"
        description="Go plugins register hooks and routes at startup. Add one in code and redeploy to see it here."
        action={
          <a href={PLUGIN_API_DOCS} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 text-[13px] font-medium text-accent hover:text-accent-2">
            Go plugin API <ExternalLink className="w-3.5 h-3.5" aria-hidden />
          </a>
        }
        className="py-10"
      />
    );
  }

  const groups = new Map<string, PluginInfo['hooks']>();
  for (const h of info.hooks) {
    const g = groups.get(h.type) ?? [];
    g.push(h);
    groups.set(h.type, g);
  }

  return (
    <div className="flex flex-col">
      {showNames && info.plugins.length > 0 && (
        <ul className="m-0 p-0 list-none">
          {info.plugins.map((name) => (
            <li key={name} className="border-b border-line">
              <Link
                to={`/plugins/${encodeURIComponent(`server:${name}`)}`}
                className="flex items-center gap-3 px-5 py-3 hover:bg-hover"
              >
                <span className="w-8 h-8 rounded-md bg-sunken text-ink-2 inline-flex items-center justify-center font-mono text-[11px] font-semibold" aria-hidden>
                  Go
                </span>
                <span className="flex-1 min-w-0 flex flex-col">
                  <span className="font-mono text-[13.5px] text-ink truncate">{name}</span>
                  <span className="text-[12.5px] text-ink-3">Registered at startup</span>
                </span>
                <Badge tone="ok" dot>
                  Running
                </Badge>
                <ChevronRight className="w-4 h-4 text-ink-3" aria-hidden />
              </Link>
            </li>
          ))}
        </ul>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 gap-x-8 gap-y-5 px-5 py-4">
        <div className="flex flex-col gap-3 min-w-0">
          <h4 className="m-0 text-[12px] font-semibold uppercase tracking-[.06em] text-ink-3">Hooks · {info.hooks.length}</h4>
          {info.hooks.length === 0 && <p className="m-0 text-sm text-ink-3">No hooks registered.</p>}
          {[...groups.entries()].map(([type, hooks]) => (
            <div key={type} className="flex flex-col gap-1">
              <span className="text-[13px] font-medium text-ink">{HOOK_LABELS[type] ?? `${type} hooks`}</span>
              <ul className="m-0 p-0 list-none flex flex-col">
                {hooks.map((h, i) => (
                  <li key={i} className="flex items-center gap-2 py-1 text-[13px] text-ink-2 border-b border-line last:border-b-0 min-w-0">
                    <code className="font-mono text-[12.5px] truncate">
                      {h.type}
                      {h.action ? `:${h.action}` : ''}
                    </code>
                    {h.collection && <span className="text-ink-3 truncate">on {h.collection}</span>}
                    {h.count > 1 && <span className="ml-auto text-[12px] text-ink-3 tabular-nums">×{h.count}</span>}
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
        <div className="flex flex-col gap-3 min-w-0">
          <h4 className="m-0 text-[12px] font-semibold uppercase tracking-[.06em] text-ink-3">Routes · {info.routes.length}</h4>
          {info.routes.length === 0 && <p className="m-0 text-sm text-ink-3">No custom routes.</p>}
          <ul className="m-0 p-0 list-none flex flex-col">
            {info.routes.map((r, i) => (
              <li key={i} className="flex items-center gap-2 py-1 border-b border-line last:border-b-0 min-w-0">
                <Badge tone={methodTone(r.method)} className="font-mono !text-[11px]">
                  {r.method}
                </Badge>
                <code className="font-mono text-[12.5px] text-ink-2 truncate">{r.path}</code>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </div>
  );
}
