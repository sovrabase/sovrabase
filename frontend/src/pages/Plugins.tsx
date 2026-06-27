import { useEffect, useState } from 'react';
import { Loader2, Puzzle, Zap, Route, Check } from 'lucide-react';
import { api } from '../api';
import { usePlugins } from '../store';

interface ConfigField { key: string; label: string; type: string; required: boolean; }
interface IntegrationDef { id: string; name: string; description: string; category: string; icon: string; color: string; config_fields: ConfigField[]; }

const CATEGORY_LABELS: Record<string, string> = {
  payments: 'Payments', email: 'Email', sms: 'SMS', notifications: 'Notifications', search: 'Search', analytics: 'Analytics',
};

const hookColors: Record<string, string> = {
  record: 'bg-[#5b5bff]/15 text-[#5b5bff]',
  auth: 'bg-[#f59e0b]/15 text-[#f59e0b]',
  storage: 'bg-[#22c55e]/15 text-[#22c55e]',
  realtime: 'bg-[#ef4444]/15 text-[#ef4444]',
  email: 'bg-[#8b5cf6]/15 text-[#8b5cf6]',
};
const methodColors: Record<string, string> = {
  GET: 'bg-[#22c55e]/15 text-[#22c55e]', POST: 'bg-[#5b5bff]/15 text-[#5b5bff]',
  PUT: 'bg-[#f59e0b]/15 text-[#f59e0b]', DELETE: 'bg-[#ef4444]/15 text-[#ef4444]',
};
const otherHookColor = 'bg-[#6b7280]/15 text-[#6b7280]';
const otherMethodColor = 'bg-[#6b7280]/15 text-[#6b7280]';

export default function Plugins() {
  const { plugins, loading, loadPlugins } = usePlugins();
  const [catalog, setCatalog] = useState<IntegrationDef[]>([]);
  const [catLoading, setCatLoading] = useState(true);

  useEffect(() => {
    loadPlugins();
    api<{ integrations: IntegrationDef[] }>('/admin/integrations/catalog')
      .then((d) => setCatalog(d.integrations || []))
      .catch(() => {})
      .finally(() => setCatLoading(false));
  }, [loadPlugins]);

  const categories = [...new Set(catalog.map((c) => c.category))].sort();
  const hasSystemPlugins = plugins && (
    (plugins.plugins?.length ?? 0) > 0 ||
    (plugins.hooks?.length ?? 0) > 0 ||
    (plugins.routes?.length ?? 0) > 0
  );

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-bold text-text-primary">Integrations</h1>
        <p className="text-text-muted text-sm mt-1">Enable third-party services for your projects. Configure per-project in each project's Integrations tab.</p>
      </div>

      {/* Integration Catalog */}
      {catLoading ? (
        <div className="flex justify-center py-12"><Loader2 className="w-6 h-6 text-accent animate-spin" /></div>
      ) : (
        categories.map((cat) => (
          <section key={cat}>
            <h2 className="text-text-muted text-xs uppercase tracking-wider mb-3 font-medium">{CATEGORY_LABELS[cat] || cat}</h2>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
              {catalog.filter((c) => c.category === cat).map((def) => (
                <div key={def.id} className="bg-bg-card border border-border rounded-lg p-4 flex items-start gap-3">
                  <span className="w-10 h-10 rounded-lg flex items-center justify-center text-base font-bold text-white shrink-0" style={{ backgroundColor: def.color }}>{def.icon}</span>
                  <div className="min-w-0">
                    <h4 className="text-text-primary text-sm font-semibold">{def.name}</h4>
                    <p className="text-text-muted text-xs mt-0.5 line-clamp-2">{def.description}</p>
                    <div className="flex flex-wrap gap-1 mt-2">
                      {def.config_fields.slice(0, 3).map((f) => (
                        <span key={f.key} className="px-1.5 py-0.5 bg-bg-input text-text-muted rounded text-[10px] font-mono">{f.key}</span>
                      ))}
                      {def.config_fields.length > 3 && <span className="px-1.5 py-0.5 bg-bg-input text-text-muted rounded text-[10px]">+{def.config_fields.length - 3}</span>}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </section>
        ))
      )}

      {/* System Hooks (developer section — only shows if Go plugins are registered) */}
      {loading ? (
        <div className="flex justify-center py-8"><Loader2 className="w-5 h-5 text-accent animate-spin" /></div>
      ) : hasSystemPlugins && plugins ? (
        <>
          <div className="border-t border-border pt-6">
            <div className="flex items-center gap-2 mb-1">
              <Puzzle className="w-5 h-5 text-text-muted" />
              <h2 className="text-lg font-semibold text-text-primary">System Hooks</h2>
            </div>
            <p className="text-text-muted text-sm mb-4">Internal Go plugins registered on the server. These are developer-facing and require code changes to configure.</p>
          </div>

          {(plugins.plugins?.length ?? 0) > 0 && (
            <section>
              <h3 className="flex items-center gap-2 text-sm font-semibold text-text-primary mb-3"><Check className="w-4 h-4 text-success" /> Registered Plugins ({plugins.plugins?.length ?? 0})</h3>
              <div className="flex flex-wrap gap-2">
                {plugins.plugins?.map((name) => (
                  <span key={name} className="inline-flex items-center gap-2 px-3 py-1.5 bg-bg-card border border-border rounded-lg text-text-primary text-sm">
                    <span className="w-2 h-2 rounded-full bg-success" /> {name}
                  </span>
                ))}
              </div>
            </section>
          )}

          {(plugins.hooks?.length ?? 0) > 0 && (
            <section>
              <h3 className="flex items-center gap-2 text-sm font-semibold text-text-primary mb-3"><Zap className="w-4 h-4 text-accent" /> Active Hooks ({plugins.hooks?.length ?? 0})</h3>
              <div className="flex flex-wrap gap-2">
                {plugins.hooks?.map((h, i) => (
                  <span key={i} className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded text-xs font-mono ${hookColors[h.type] || otherHookColor}`}>
                    {h.type}{h.action && `:${h.action}`}{h.collection && ` (${h.collection})`} {h.count > 1 && <span className="opacity-60">x{h.count}</span>}
                  </span>
                ))}
              </div>
            </section>
          )}

          {(plugins.routes?.length ?? 0) > 0 && (
            <section>
              <h3 className="flex items-center gap-2 text-sm font-semibold text-text-primary mb-3"><Route className="w-4 h-4 text-accent" /> Custom Routes ({plugins.routes?.length ?? 0})</h3>
              <div className="flex flex-wrap gap-2">
                {plugins.routes?.map((r, i) => (
                  <span key={i} className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded text-xs font-mono ${methodColors[r.method] || otherMethodColor}`}>
                    {r.method} {r.path}
                  </span>
                ))}
              </div>
            </section>
          )}
        </>
      ) : null}
    </div>
  );
}
