import { useState, useEffect, useRef, useCallback } from 'react';
import { FileText, Filter, RefreshCw, Loader2 } from 'lucide-react';
import { api, formatDate } from '../api';
import type { LogEntry } from '../types';

interface Props {
  projectId: string;
  apiKey?: string;
}

const LEVEL_COLORS: Record<string, string> = {
  error: 'text-danger',
  warn: 'text-yellow-400',
  info: 'text-blue-400',
  debug: 'text-text-muted',
  trace: 'text-text-muted',
};

const LEVELS = ['all', 'error', 'warn', 'info', 'debug', 'trace'] as const;

export default function LogsTab({ projectId }: Props) {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [levelFilter, setLevelFilter] = useState<string>('all');
  const [autoRefresh, setAutoRefresh] = useState(false);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const loadLogs = useCallback(() => {
    api<LogEntry[]>(`/admin/projects/${encodeURIComponent(projectId)}/logs`)
      .then(setLogs)
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [projectId]);

  useEffect(() => {
    setLoading(true);
    loadLogs();
  }, [loadLogs]);

  useEffect(() => {
    if (autoRefresh) {
      intervalRef.current = setInterval(loadLogs, 5000);
    } else if (intervalRef.current) {
      clearInterval(intervalRef.current);
      intervalRef.current = null;
    }
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [autoRefresh, loadLogs]);

  const filtered = levelFilter === 'all' ? logs : logs.filter((l) => l.level === levelFilter);

  const levelColor = (level: string) => LEVEL_COLORS[level] || 'text-text-secondary';

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between flex-wrap gap-3">
        <h2 className="text-lg font-semibold text-text-primary">Logs ({logs.length})</h2>
        <div className="flex items-center gap-3">
          {/* Level Filter */}
          <div className="flex items-center gap-2">
            <Filter className="w-4 h-4 text-text-muted" />
            <select
              value={levelFilter}
              onChange={(e) => setLevelFilter(e.target.value)}
              className="bg-bg-input border border-border rounded px-2 py-1 text-text-primary text-xs focus:outline-none focus:border-accent"
            >
              {LEVELS.map((l) => (
                <option key={l} value={l}>{l === 'all' ? 'All Levels' : l.toUpperCase()}</option>
              ))}
            </select>
          </div>
          {/* Auto-Refresh Toggle */}
          <button
            onClick={() => setAutoRefresh(!autoRefresh)}
            className={`flex items-center gap-1 px-3 py-1 rounded border text-xs font-medium transition-colors ${
              autoRefresh
                ? 'bg-accent/10 border-accent text-accent'
                : 'bg-bg-input border-border text-text-secondary hover:text-text-primary'
            }`}
          >
            <RefreshCw className={`w-3 h-3 ${autoRefresh ? 'animate-spin' : ''}`} />
            {autoRefresh ? 'Auto' : 'Realtime'}
          </button>
        </div>
      </div>

      {loading ? (
        <div className="flex items-center gap-2 py-10 text-text-muted"><Loader2 className="w-4 h-4 animate-spin" /> Loading logs...</div>
      ) : filtered.length === 0 ? (
        <div className="flex flex-col items-center py-16 text-text-muted gap-3">
          <FileText className="w-10 h-10" />
          <p>{logs.length === 0 ? 'No logs' : 'No logs match filter'}</p>
        </div>
      ) : (
        <div className="border border-border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-bg-input text-text-muted text-xs uppercase">
              <tr>
                <th className="text-left px-4 py-3 font-medium w-40">Timestamp</th>
                <th className="text-left px-4 py-3 font-medium w-20">Level</th>
                <th className="text-left px-4 py-3 font-medium">Message</th>
                <th className="text-left px-4 py-3 font-medium w-32">Source</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border font-mono">
              {filtered.map((log, i) => (
                <tr key={i} className="hover:bg-bg-input/50 transition-colors">
                  <td className="px-4 py-2 text-text-secondary text-xs whitespace-nowrap">{formatDate(log.timestamp)}</td>
                  <td className="px-4 py-2">
                    <span className={`text-xs font-bold uppercase ${levelColor(log.level)}`}>{log.level}</span>
                  </td>
                  <td className="px-4 py-2 text-text-primary text-xs">{log.message}</td>
                  <td className="px-4 py-2 text-text-muted text-xs">{log.source || '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
