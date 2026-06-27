import { useState, useEffect, useCallback } from 'react';
import { RefreshCw, Trash2, Loader2 } from 'lucide-react';
import { api, formatDate } from '../api';

interface LogEntry {
  timestamp: string;
  method: string;
  path: string;
  status: number;
  duration: string;
  ip?: string;
}

interface Props { projectId: string; }

const METHOD_COLORS: Record<string, string> = {
  GET: 'bg-success/10 text-success',
  POST: 'bg-accent/10 text-accent',
  PUT: 'bg-yellow-500/10 text-yellow-500',
  PATCH: 'bg-yellow-500/10 text-yellow-500',
  DELETE: 'bg-danger/10 text-danger',
};

export default function LogsTab({ projectId }: Props) {
  const [entries, setEntries] = useState<LogEntry[]>([]);
  const [loading, setLoading] = useState(true);

  const load = useCallback(() => {
    setLoading(true);
    api<LogEntry[]>(`/admin/projects/${encodeURIComponent(projectId)}/logs`)
      .then((d) => setEntries(Array.isArray(d) ? d : []))
      .catch(() => setEntries([]))
      .finally(() => setLoading(false));
  }, [projectId]);

  useEffect(() => { load(); }, [load]);

  const clearLogs = async () => {
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/logs`, { method: 'DELETE' });
      setEntries([]);
    } catch { /* ignore */ }
  };

  const methodColor = (m: string) => METHOD_COLORS[m] || 'bg-bg-input text-text-secondary';
  const statusColor = (s: number) => s < 300 ? 'text-success' : s < 400 ? 'text-yellow-500' : 'text-danger';

  // Compute metrics from entries
  const totalRequests = entries.length;
  const successRate = totalRequests > 0 ? Math.round((entries.filter((e) => e.status < 400).length / totalRequests) * 100) : 100;
  const errorRate = 100 - successRate;

  if (loading) {
    return <div className="flex items-center gap-2 py-10 text-text-muted"><Loader2 className="w-4 h-4 animate-spin" /> Loading logs...</div>;
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-text-primary">Logs ({entries.length})</h2>
        <div className="flex gap-2">
          <button onClick={load} className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-bg-input border border-border text-text-secondary text-xs hover:text-text-primary transition-colors">
            <RefreshCw className="w-3.5 h-3.5" /> Refresh
          </button>
          <button onClick={clearLogs} className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-danger/10 border border-danger/20 text-danger text-xs hover:bg-danger/20 transition-colors">
            <Trash2 className="w-3.5 h-3.5" /> Clear
          </button>
        </div>
      </div>

      {/* Metrics computed from entries */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        {[
          { label: 'Total Requests', value: totalRequests.toLocaleString(), color: 'text-text-primary' },
          { label: 'Success Rate', value: `${successRate}%`, color: 'text-success' },
          { label: 'Error Rate', value: `${errorRate}%`, color: 'text-danger' },
          { label: 'Unique Paths', value: new Set(entries.map((e) => e.path)).size.toString(), color: 'text-text-primary' },
        ].map((m) => (
          <div key={m.label} className="bg-bg-card border border-border rounded-lg p-4">
            <div className="text-text-muted text-xs mb-1">{m.label}</div>
            <p className={`text-xl font-bold ${m.color}`}>{m.value}</p>
          </div>
        ))}
      </div>

      {/* Logs Table */}
      {entries.length === 0 ? (
        <div className="flex flex-col items-center py-16 text-text-muted gap-3">
          <p>No logs recorded yet</p>
        </div>
      ) : (
        <div className="border border-border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-bg-input text-text-muted text-xs uppercase">
              <tr>
                <th className="text-left px-4 py-3 font-medium">Timestamp</th>
                <th className="text-left px-4 py-3 font-medium w-20">Method</th>
                <th className="text-left px-4 py-3 font-medium">Path</th>
                <th className="text-left px-4 py-3 font-medium w-20">Status</th>
                <th className="text-right px-4 py-3 font-medium w-24">Duration</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border font-mono">
              {entries.map((e, i) => (
                <tr key={i} className="hover:bg-bg-input/50 transition-colors">
                  <td className="px-4 py-2 text-text-secondary text-xs whitespace-nowrap">{formatDate(e.timestamp)}</td>
                  <td className="px-4 py-2">
                    <span className={`inline-flex px-2 py-0.5 rounded text-xs font-bold ${methodColor(e.method)}`}>{e.method}</span>
                  </td>
                  <td className="px-4 py-2 text-text-primary text-xs truncate max-w-md" title={e.path}>{e.path}</td>
                  <td className={`px-4 py-2 text-xs font-bold ${statusColor(e.status)}`}>{e.status}</td>
                  <td className="px-4 py-2 text-text-secondary text-xs text-right">{e.duration}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
