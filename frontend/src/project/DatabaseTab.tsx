import { useState, useEffect } from 'react';
import { Search, Database, Shield, FileText, Loader2, ChevronRight } from 'lucide-react';
import { api, formatDate } from '../api';
import type { Collection } from '../types';

interface Props {
  projectId: string;
  apiKey?: string;
}

export default function DatabaseTab({ projectId }: Props) {
  const [collections, setCollections] = useState<Collection[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [expanded, setExpanded] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    api<{ collections: Collection[] }>(`/admin/projects/${encodeURIComponent(projectId)}/db-analysis`)
      .then((data) => setCollections(data.collections || []))
      .finally(() => setLoading(false));
  }, [projectId]);

  const filtered = collections.filter((c) =>
    c.name.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return <div className="flex items-center gap-2 py-10 text-text-muted"><Loader2 className="w-4 h-4 animate-spin" /> Loading collections...</div>;
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <h2 className="text-lg font-semibold text-text-primary">Collections ({collections.length})</h2>
        <div className="relative flex-1 max-w-xs">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-muted" />
          <input
            type="text"
            placeholder="Filter collections..."
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            className="w-full bg-bg-input border border-border rounded-md pl-9 pr-3 py-2 text-text-primary text-sm placeholder:text-text-muted focus:outline-none focus:border-accent"
          />
        </div>
      </div>

      {filtered.length === 0 ? (
        <div className="flex flex-col items-center py-16 text-text-muted gap-3">
          <Database className="w-10 h-10" />
          <p>{collections.length === 0 ? 'No collections' : 'No collections match filter'}</p>
        </div>
      ) : (
        <div className="border border-border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-bg-input text-text-muted text-xs uppercase">
              <tr>
                <th className="text-left px-4 py-3 font-medium w-8"></th>
                <th className="text-left px-4 py-3 font-medium">Name</th>
                <th className="text-left px-4 py-3 font-medium">Schema</th>
                <th className="text-left px-4 py-3 font-medium">Documents</th>
                <th className="text-left px-4 py-3 font-medium">RLS Rules</th>
                <th className="text-left px-4 py-3 font-medium">Created</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {filtered.map((c) => (
                <>
                  <tr
                    key={c.name}
                    className="hover:bg-bg-input/50 transition-colors cursor-pointer"
                    onClick={() => setExpanded(expanded === c.name ? null : c.name)}
                  >
                    <td className="px-4 py-3">
                      <ChevronRight className={`w-4 h-4 text-text-muted transition-transform ${expanded === c.name ? 'rotate-90' : ''}`} />
                    </td>
                    <td className="px-4 py-3 font-medium text-text-primary">{c.name}</td>
                    <td className="px-4 py-3 text-text-secondary">
                      {c.schema ? Object.keys(c.schema).length : 0} columns
                    </td>
                    <td className="px-4 py-3 text-text-secondary">{c.doc_count ?? '—'}</td>
                    <td className="px-4 py-3">
                      {c.rls_rules && c.rls_rules.length > 0 ? (
                        <span className="inline-flex items-center gap-1 text-xs text-accent">
                          <Shield className="w-3 h-3" /> {c.rls_rules.length} rules
                        </span>
                      ) : (
                        <span className="text-text-muted text-xs">None</span>
                      )}
                    </td>
                    <td className="px-4 py-3 text-text-secondary text-xs">{formatDate(c.created_at)}</td>
                  </tr>
                  {expanded === c.name && (
                    <tr key={`${c.name}-details`} className="bg-bg-input/30">
                      <td colSpan={6} className="px-8 py-4">
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                          {c.schema && (
                            <div>
                              <h4 className="text-xs uppercase tracking-wider text-text-muted mb-2 flex items-center gap-1">
                                <FileText className="w-3 h-3" /> Schema
                              </h4>
                              <div className="space-y-1">
                                {Object.entries(c.schema).map(([key, type]) => (
                                  <div key={key} className="flex gap-2 text-xs">
                                    <span className="text-text-primary font-mono">{key}</span>
                                    <span className="text-text-muted">
                                      {typeof type === 'string' ? type : JSON.stringify(type)}
                                    </span>
                                  </div>
                                ))}
                              </div>
                            </div>
                          )}
                          {c.rls_rules && c.rls_rules.length > 0 && (
                            <div>
                              <h4 className="text-xs uppercase tracking-wider text-text-muted mb-2 flex items-center gap-1">
                                <Shield className="w-3 h-3" /> RLS Rules
                              </h4>
                              <div className="space-y-2">
                                {c.rls_rules.map((rule, i) => (
                                  <div key={i} className="bg-bg-card border border-border rounded p-2">
                                    <span className="text-xs font-mono text-accent">{rule.action}</span>
                                    <p className="text-xs text-text-muted mt-0.5 font-mono break-all">{rule.expression}</p>
                                  </div>
                                ))}
                              </div>
                            </div>
                          )}
                        </div>
                      </td>
                    </tr>
                  )}
                </>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
