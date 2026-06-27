import { useState, useEffect } from 'react';
import { BarChart3, TrendingUp, Download, Upload, Loader2 } from 'lucide-react';
import { api, formatBytes } from '../api';
import type { AnalyticsSummary } from '../types';

interface Props {
  projectId: string;
  apiKey?: string;
}

export default function AnalyticsTab({ projectId }: Props) {
  const [analytics, setAnalytics] = useState<AnalyticsSummary | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    api<AnalyticsSummary>(`/admin/projects/${encodeURIComponent(projectId)}/usage`)
      .then(setAnalytics)
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [projectId]);

  if (loading) {
    return <div className="flex items-center gap-2 py-10 text-text-muted"><Loader2 className="w-4 h-4 animate-spin" /> Loading analytics...</div>;
  }

  if (!analytics) {
    return (
      <div className="flex flex-col items-center py-16 text-text-muted gap-3">
        <BarChart3 className="w-10 h-10" />
        <p>No analytics data available</p>
      </div>
    );
  }

  const maxEndpoint = analytics.requests_by_endpoint
    ? Math.max(...Object.values(analytics.requests_by_endpoint), 1)
    : 1;

  return (
    <div className="space-y-6">
      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="bg-bg-card border border-border rounded-lg p-4">
          <div className="flex items-center gap-2 text-text-muted text-sm mb-1">
            <TrendingUp className="w-4 h-4 text-accent" /> Total Requests
          </div>
          <p className="text-2xl font-bold text-text-primary">{analytics.total_requests?.toLocaleString() ?? '—'}</p>
          {analytics.period && <p className="text-text-muted text-xs mt-1">{analytics.period}</p>}
        </div>
        <div className="bg-bg-card border border-border rounded-lg p-4">
          <div className="flex items-center gap-2 text-text-muted text-sm mb-1">
            <Upload className="w-4 h-4 text-accent" /> Upload
          </div>
          <p className="text-2xl font-bold text-text-primary">{analytics.bandwidth_up != null ? formatBytes(analytics.bandwidth_up) : '—'}</p>
        </div>
        <div className="bg-bg-card border border-border rounded-lg p-4">
          <div className="flex items-center gap-2 text-text-muted text-sm mb-1">
            <Download className="w-4 h-4 text-accent" /> Download
          </div>
          <p className="text-2xl font-bold text-text-primary">{analytics.bandwidth_down != null ? formatBytes(analytics.bandwidth_down) : '—'}</p>
        </div>
      </div>

      {/* Requests by Endpoint */}
      {analytics.requests_by_endpoint && Object.keys(analytics.requests_by_endpoint).length > 0 && (
        <div className="bg-bg-card border border-border rounded-lg p-5">
          <h3 className="text-text-primary font-semibold mb-4">Requests by Endpoint</h3>
          <div className="space-y-3">
            {Object.entries(analytics.requests_by_endpoint)
              .sort(([, a], [, b]) => b - a)
              .map(([endpoint, count]) => (
                <div key={endpoint}>
                  <div className="flex items-center justify-between text-sm mb-1">
                    <span className="text-text-primary font-mono text-xs">{endpoint}</span>
                    <span className="text-text-secondary text-xs">{count.toLocaleString()}</span>
                  </div>
                  <div className="w-full bg-bg-input rounded-full h-2">
                    <div
                      className="bg-accent rounded-full h-2 transition-all"
                      style={{ width: `${Math.max((count / maxEndpoint) * 100, 2)}%` }}
                    />
                  </div>
                </div>
              ))}
          </div>
        </div>
      )}
    </div>
  );
}
