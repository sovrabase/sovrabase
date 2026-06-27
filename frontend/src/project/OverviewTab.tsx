import { useState, useEffect } from 'react';
import { Copy, Eye, EyeOff, Key, Calendar, HardDrive, Users, BarChart3, TrendingUp } from 'lucide-react';
import { api, formatBytes, formatDate } from '../api';
import type { Project } from '../types';
import { useToast } from '../components/Toast';

interface Props {
  projectId: string;
  apiKey?: string;
}

export default function OverviewTab({ projectId, apiKey }: Props) {
  const { show } = useToast();
  const [project, setProject] = useState<Project | null>(null);
  const [loading, setLoading] = useState(true);
  const [keyVisible, setKeyVisible] = useState(false);

  useEffect(() => {
    setLoading(true);
    api<Project>(`/admin/projects/${encodeURIComponent(projectId)}`)
      .then(setProject)
      .finally(() => setLoading(false));
  }, [projectId]);

  const copyKey = async () => {
    if (!apiKey) return;
    try {
      await navigator.clipboard.writeText(apiKey);
      show('API key copied to clipboard', 'success');
    } catch {
      show('Failed to copy', 'error');
    }
  };

  const maskKey = (key: string) => {
    if (key.length <= 8) return key;
    return key.slice(0, 8) + '•'.repeat(Math.min(24, key.length - 8));
  };

  if (loading) return <div className="py-10 text-text-muted">Loading...</div>;

  return (
    <div className="space-y-6">
      {/* Info Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {/* API Key */}
        <div className="bg-bg-card border border-border rounded-lg p-4">
          <div className="flex items-center gap-2 text-text-muted text-sm mb-2">
            <Key className="w-4 h-4" /> API Key
          </div>
          {apiKey ? (
            <div className="flex items-center gap-2">
              <code className="text-sm font-mono text-text-primary break-all flex-1">
                {keyVisible ? apiKey : maskKey(apiKey)}
              </code>
              <button onClick={() => setKeyVisible(!keyVisible)} className="p-1.5 rounded hover:bg-bg-input text-text-secondary hover:text-text-primary transition-colors" title={keyVisible ? 'Hide' : 'Reveal'}>
                {keyVisible ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
              </button>
              <button onClick={copyKey} className="p-1.5 rounded hover:bg-bg-input text-text-secondary hover:text-text-primary transition-colors" title="Copy">
                <Copy className="w-4 h-4" />
              </button>
            </div>
          ) : (
            <p className="text-text-muted text-sm">No API key generated</p>
          )}
        </div>

        {/* Project ID */}
        <div className="bg-bg-card border border-border rounded-lg p-4">
          <div className="flex items-center gap-2 text-text-muted text-sm mb-2">
            <HardDrive className="w-4 h-4" /> Project ID
          </div>
          <p className="text-text-primary font-mono text-sm">{project?.id || projectId}</p>
        </div>

        {/* Dates */}
        <div className="bg-bg-card border border-border rounded-lg p-4">
          <div className="flex items-center gap-2 text-text-muted text-sm mb-2">
            <Calendar className="w-4 h-4" /> Created / Updated
          </div>
          <p className="text-text-primary text-sm">
            {formatDate(project?.created_at)} / {formatDate(project?.updated_at)}
          </p>
        </div>
      </div>

      {/* Quick Stats */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="bg-bg-card border border-border rounded-lg p-4">
          <div className="flex items-center gap-2 text-text-muted text-sm mb-1">
            <BarChart3 className="w-4 h-4" /> Collections
          </div>
          <p className="text-2xl font-bold text-text-primary">{project?.collection_count ?? '—'}</p>
        </div>
        <div className="bg-bg-card border border-border rounded-lg p-4">
          <div className="flex items-center gap-2 text-text-muted text-sm mb-1">
            <HardDrive className="w-4 h-4" /> Buckets
          </div>
          <p className="text-2xl font-bold text-text-primary">{project?.bucket_count ?? '—'}</p>
        </div>
        <div className="bg-bg-card border border-border rounded-lg p-4">
          <div className="flex items-center gap-2 text-text-muted text-sm mb-1">
            <Users className="w-4 h-4" /> Members
          </div>
          <p className="text-2xl font-bold text-text-primary">{project?.member_count ?? '—'}</p>
        </div>
      </div>

      {/* Usage Summary */}
      <UsageCard projectId={projectId} />
    </div>
  );
}

function UsageCard({ projectId }: { projectId: string }) {
  const [usage, setUsage] = useState<{ total_requests?: number; bandwidth_up?: number; bandwidth_down?: number } | null>(null);

  useEffect(() => {
    api<{ total_requests?: number; bandwidth_up?: number; bandwidth_down?: number }>(`/admin/projects/${encodeURIComponent(projectId)}/usage`)
      .then(setUsage)
      .catch(() => {});
  }, [projectId]);

  if (!usage) return null;

  return (
    <div className="bg-bg-card border border-border rounded-lg p-4">
      <h3 className="text-text-primary font-semibold mb-3 flex items-center gap-2">
        <TrendingUp className="w-4 h-4 text-accent" /> Usage Summary
      </h3>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div>
          <p className="text-text-muted text-xs uppercase tracking-wider">Requests Today</p>
          <p className="text-text-primary text-lg font-semibold">{usage.total_requests?.toLocaleString() ?? '—'}</p>
        </div>
        <div>
          <p className="text-text-muted text-xs uppercase tracking-wider">Upload</p>
          <p className="text-text-primary text-lg font-semibold">{usage.bandwidth_up != null ? formatBytes(usage.bandwidth_up) : '—'}</p>
        </div>
        <div>
          <p className="text-text-muted text-xs uppercase tracking-wider">Download</p>
          <p className="text-text-primary text-lg font-semibold">{usage.bandwidth_down != null ? formatBytes(usage.bandwidth_down) : '—'}</p>
        </div>
      </div>
    </div>
  );
}
