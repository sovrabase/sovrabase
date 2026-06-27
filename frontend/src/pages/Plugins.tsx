import { useEffect } from 'react';
import { Loader2, Puzzle, Zap, Route } from 'lucide-react';
import { usePlugins } from '../store';
import type { HookInfo, RouteInfo } from '../types';

const hookColors: Record<string, string> = {
  record: 'bg-[#5b5bff]/15 text-[#5b5bff]',
  auth: 'bg-[#f59e0b]/15 text-[#f59e0b]',
  storage: 'bg-[#22c55e]/15 text-[#22c55e]',
  realtime: 'bg-[#ef4444]/15 text-[#ef4444]',
  email: 'bg-[#8b5cf6]/15 text-[#8b5cf6]',
};

const methodColors: Record<string, string> = {
  GET: 'bg-[#22c55e]/15 text-[#22c55e]',
  POST: 'bg-[#5b5bff]/15 text-[#5b5bff]',
  PUT: 'bg-[#f59e0b]/15 text-[#f59e0b]',
  DELETE: 'bg-[#ef4444]/15 text-[#ef4444]',
  PATCH: 'bg-[#8b5cf6]/15 text-[#8b5cf6]',
};

const otherHookColor = 'bg-[#6b7280]/15 text-[#6b7280]';
const otherMethodColor = 'bg-[#6b7280]/15 text-[#6b7280]';

export default function Plugins() {
  const { plugins, loading, loadPlugins } = usePlugins();

  useEffect(() => {
    loadPlugins();
  }, [loadPlugins]);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-24">
        <Loader2 className="w-8 h-8 text-accent animate-spin" />
      </div>
    );
  }

  if (!plugins) {
    return (
      <div className="flex flex-col items-center justify-center py-20 text-text-muted gap-3">
        <Puzzle className="w-12 h-12 text-text-muted/40" />
        <p className="text-text-secondary text-lg">No plugin data available</p>
        <button onClick={loadPlugins} className="mt-2 px-4 py-2 rounded-lg bg-accent text-white text-sm hover:bg-accent-hover transition-colors">
          Retry
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <h1 className="text-2xl font-bold text-text-primary">Plugins</h1>

      {/* Registered Plugins */}
      <section>
        <h2 className="flex items-center gap-2 text-lg font-semibold text-text-primary mb-4">
          <Puzzle className="w-5 h-5 text-accent" />
          Registered Plugins
        </h2>
        {plugins.plugins.length === 0 ? (
          <p className="text-text-muted text-sm py-4">No plugins registered</p>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {plugins.plugins.map((name) => (
              <div key={name} className="flex items-center gap-3 bg-bg-card border border-border rounded-lg px-4 py-3">
                <span className="w-2 h-2 rounded-full bg-success shrink-0" />
                <span className="text-text-primary text-sm font-medium">{name}</span>
                <span className="ml-auto text-[10px] font-bold uppercase tracking-wider px-2 py-0.5 rounded bg-accent/15 text-accent">
                  Active
                </span>
              </div>
            ))}
          </div>
        )}
      </section>

      {/* Active Hooks */}
      <section>
        <h2 className="flex items-center gap-2 text-lg font-semibold text-text-primary mb-4">
          <Zap className="w-5 h-5 text-accent" />
          Active Hooks
        </h2>
        {plugins.hooks.length === 0 ? (
          <p className="text-text-muted text-sm py-4">No active hooks</p>
        ) : (
          <div className="bg-bg-card border border-border rounded-xl overflow-hidden">
            <table className="w-full">
              <thead>
                <tr className="border-b border-border">
                  {['Type', 'Action', 'Collection', 'Count'].map((h) => (
                    <th key={h} className="text-left text-text-muted text-xs font-medium uppercase tracking-wider px-6 py-3">
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {plugins.hooks.map((hook, i) => (
                  <tr key={i} className="border-b border-border/50">
                    <td className="px-6 py-3">
                      <span className={`text-xs font-medium px-2 py-0.5 rounded ${hookColors[hook.type] || otherHookColor}`}>
                        {hook.type}
                      </span>
                    </td>
                    <td className="px-6 py-3 text-text-primary text-sm">{hook.action}</td>
                    <td className="px-6 py-3 text-text-secondary text-sm font-mono">{hook.collection}</td>
                    <td className="px-6 py-3 text-text-secondary text-sm">{hook.count}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {/* Custom Routes */}
      <section>
        <h2 className="flex items-center gap-2 text-lg font-semibold text-text-primary mb-4">
          <Route className="w-5 h-5 text-accent" />
          Custom Routes
        </h2>
        {plugins.routes.length === 0 ? (
          <p className="text-text-muted text-sm py-4">No custom routes registered</p>
        ) : (
          <div className="bg-bg-card border border-border rounded-xl overflow-hidden">
            <table className="w-full">
              <thead>
                <tr className="border-b border-border">
                  <th className="text-left text-text-muted text-xs font-medium uppercase tracking-wider px-6 py-3">Method</th>
                  <th className="text-left text-text-muted text-xs font-medium uppercase tracking-wider px-6 py-3">Path</th>
                </tr>
              </thead>
              <tbody>
                {plugins.routes.map((route, i) => (
                  <tr key={i} className="border-b border-border/50">
                    <td className="px-6 py-3">
                      <span className={`text-xs font-bold px-2 py-0.5 rounded ${methodColors[route.method] || otherMethodColor}`}>
                        {route.method}
                      </span>
                    </td>
                    <td className="px-6 py-3 text-text-primary text-sm font-mono">{route.path}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
