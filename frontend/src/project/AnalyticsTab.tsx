import { useState, useEffect, useCallback } from 'react';
import { BarChart3, TrendingUp, Clock, RefreshCw, Loader2 } from 'lucide-react';
import { api } from '../api';

interface TopEvent {
  name: string;
  count: number;
}

interface HourlyBucket {
  hour: number;
  count: number;
}

interface AnalyticsData {
  total_events: number;
  top_events: TopEvent[];
  hourly: HourlyBucket[];
}

interface Props {
  projectId: string;
}

export default function AnalyticsTab({ projectId }: Props) {
  const [data, setData] = useState<AnalyticsData | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(() => {
    setLoading(true);
    api<AnalyticsData>(`/admin/projects/${encodeURIComponent(projectId)}/analytics`)
      .then(setData)
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [projectId]);

  useEffect(() => { load(); }, [load]);

  if (loading) {
    return (
      <div className="flex items-center gap-2 py-10 text-text-muted">
        <Loader2 className="w-4 h-4 animate-spin" /> Loading analytics...
      </div>
    );
  }

  if (!data) {
    return (
      <div className="flex flex-col items-center py-16 text-text-muted gap-3">
        <BarChart3 className="w-10 h-10" />
        <p>No analytics data available</p>
        <button onClick={load} className="flex items-center gap-1 px-3 py-1.5 bg-bg-input border border-border rounded text-text-secondary text-xs hover:text-text-primary transition-colors">
          <RefreshCw className="w-3 h-3" /> Retry
        </button>
      </div>
    );
  }

  const topEvent = data.top_events?.[0];
  const activeHours = data.hourly?.filter((h) => h.count > 0).length ?? 0;
  const maxCount = data.hourly?.length ? Math.max(...data.hourly.map((h) => h.count), 1) : 1;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-text-primary">Analytics</h2>
        <button
          onClick={load}
          className="flex items-center gap-1 px-3 py-1.5 bg-bg-input border border-border rounded text-text-secondary text-xs hover:text-text-primary transition-colors"
        >
          <RefreshCw className="w-3 h-3" /> Refresh
        </button>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="bg-bg-card border border-border rounded-lg p-4">
          <div className="flex items-center gap-2 text-text-muted text-sm mb-1">
            <TrendingUp className="w-4 h-4 text-accent" /> Total Events (24h)
          </div>
          <p className="text-2xl font-bold text-text-primary">
            {data.total_events?.toLocaleString() ?? '0'}
          </p>
        </div>
        <div className="bg-bg-card border border-border rounded-lg p-4">
          <div className="flex items-center gap-2 text-text-muted text-sm mb-1">
            <BarChart3 className="w-4 h-4 text-accent" /> Top Event
          </div>
          <p className="text-2xl font-bold text-text-primary">{topEvent?.name ?? '—'}</p>
          {topEvent && (
            <p className="text-text-muted text-xs mt-1">{topEvent.count.toLocaleString()} occurrences</p>
          )}
        </div>
        <div className="bg-bg-card border border-border rounded-lg p-4">
          <div className="flex items-center gap-2 text-text-muted text-sm mb-1">
            <Clock className="w-4 h-4 text-accent" /> Active Hours
          </div>
          <p className="text-2xl font-bold text-text-primary">{activeHours} / 24</p>
        </div>
      </div>

      {/* Top Events Table */}
      {data.top_events && data.top_events.length > 0 && (
        <div className="bg-bg-card border border-border rounded-lg p-5">
          <h3 className="text-text-primary font-semibold mb-4">Top Events</h3>
          <div className="border border-border rounded-lg overflow-hidden">
            <table className="w-full text-sm">
              <thead className="bg-bg-input text-text-muted text-xs uppercase">
                <tr>
                  <th className="text-left px-4 py-3 font-medium">Event Name</th>
                  <th className="text-right px-4 py-3 font-medium w-32">Count</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {data.top_events.map((e, i) => (
                  <tr key={i} className="hover:bg-bg-input/50 transition-colors">
                    <td className="px-4 py-2.5 text-text-primary font-mono text-xs">{e.name}</td>
                    <td className="px-4 py-2.5 text-text-secondary text-xs text-right">
                      <span className="inline-flex px-2 py-0.5 bg-accent/10 text-accent rounded-full font-medium">
                        {e.count.toLocaleString()}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Hourly Activity Chart */}
      {data.hourly && data.hourly.length > 0 && (
        <div className="bg-bg-card border border-border rounded-lg p-5">
          <h3 className="text-text-primary font-semibold mb-4">Hourly Activity</h3>
          <div className="space-y-1.5">
            {data.hourly.map((h) => (
              <div key={h.hour} className="flex items-center gap-3">
                <span className="w-8 text-right text-text-muted text-xs font-mono">
                  {h.hour.toString().padStart(2, '0')}
                </span>
                <div className="flex-1 bg-bg-input rounded-full h-5 overflow-hidden">
                  <div
                    className="bg-accent h-full rounded-full transition-all min-w-[4px]"
                    style={{ width: `${Math.max((h.count / maxCount) * 100, 0.5)}%` }}
                  />
                </div>
                <span className="w-16 text-left text-text-secondary text-xs font-mono">
                  {h.count > 0 ? h.count.toLocaleString() : ''}
                </span>
              </div>
            ))}
            <div className="flex items-center gap-3 pt-1">
              <span className="w-8" />
              <div className="flex-1 flex justify-between text-text-muted text-[10px]">
                <span>00</span><span>06</span><span>12</span><span>18</span><span>23</span>
              </div>
              <span className="w-16" />
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
