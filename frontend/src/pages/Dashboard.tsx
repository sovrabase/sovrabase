import { useEffect } from 'react';
import { FolderKanban, HardDrive, Globe, Cloud, Activity, ArrowUp, ArrowDown, Loader2, AlertTriangle } from 'lucide-react';
import { useDashboard } from '../store';
import { formatBytes } from '../api';
import { StatCard } from '../components/StatCard';

const regionFlags: Record<string, string> = {
  'us-east-1': '\u{1F1FA}\u{1F1F8}', 'us-west-1': '\u{1F1FA}\u{1F1F8}',
  'eu-west-1': '\u{1F1EA}\u{1F1FA}', 'eu-central-1': '\u{1F1EA}\u{1F1FA}',
  'ap-southeast-1': '\u{1F1F8}\u{1F1EC}', 'ap-northeast-1': '\u{1F1EF}\u{1F1F5}',
  'sa-east-1': '\u{1F1E7}\u{1F1F7}',
};

export default function Dashboard() {
  const { stats, usage, replication, loading, error, loadDashboard } = useDashboard();

  useEffect(() => {
    loadDashboard();
  }, [loadDashboard]);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-24">
        <Loader2 className="w-8 h-8 text-accent animate-spin" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center py-24 gap-3 text-text-muted">
        <AlertTriangle className="w-10 h-10 text-danger" />
        <p className="text-text-secondary">{error}</p>
        <button
          onClick={loadDashboard}
          className="mt-2 px-4 py-2 rounded-lg bg-accent text-white text-sm hover:bg-accent-hover transition-colors"
        >
          Retry
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <h1 className="text-2xl font-bold text-text-primary">Dashboard</h1>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-5">
        <StatCard
          icon={<FolderKanban className="w-5 h-5" />}
          label="Projects"
          value={stats?.projects ?? '—'}
        />
        <StatCard
          icon={<HardDrive className="w-5 h-5" />}
          label="Storage Used"
          value={stats?.storage_bytes != null ? formatBytes(stats.storage_bytes) : '—'}
        />
        <StatCard
          icon={<Globe className="w-5 h-5" />}
          label="Region"
          value={stats?.region ?? '—'}
          subtitle={stats?.version ? `v${stats.version}` : undefined}
        />
      </div>

      {replication && (
        <div className="bg-bg-card border border-border rounded-xl p-6">
          <h2 className="flex items-center gap-2 text-lg font-semibold text-text-primary mb-4">
            <Cloud className="w-5 h-5 text-accent" />
            Replication
          </h2>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <div>
              <span className="text-text-muted text-sm">Role</span>
              <p className="text-text-primary font-medium capitalize">{replication.role}</p>
            </div>
            <div>
              <span className="text-text-muted text-sm">Status</span>
              <p className="flex items-center gap-1.5 mt-0.5">
                <span className={`w-2 h-2 rounded-full ${replication.active ? 'bg-success' : 'bg-danger'}`} />
                <span className="text-text-primary font-medium">
                  {replication.active ? 'Active' : 'Inactive'}
                </span>
              </p>
            </div>
            <div>
              <span className="text-text-muted text-sm">Peers</span>
              <p className="text-text-primary font-medium">{replication.peers}</p>
            </div>
          </div>
        </div>
      )}

      {usage && (
        <div className="bg-bg-card border border-border rounded-xl p-6">
          <h2 className="flex items-center gap-2 text-lg font-semibold text-text-primary mb-4">
            <Activity className="w-5 h-5 text-accent" />
            Usage
          </h2>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <div>
              <span className="text-text-muted text-sm">Total Requests</span>
              <p className="text-text-primary font-medium text-xl mt-1">
                {usage.total_requests?.toLocaleString() ?? '—'}
              </p>
            </div>
            <div>
              <span className="text-text-muted text-sm flex items-center gap-1">
                <ArrowUp className="w-3 h-3" /> Bandwidth Up
              </span>
              <p className="text-text-primary font-medium text-xl mt-1">
                {usage.total_bandwidth_up != null ? formatBytes(usage.total_bandwidth_up) : '—'}
              </p>
            </div>
            <div>
              <span className="text-text-muted text-sm flex items-center gap-1">
                <ArrowDown className="w-3 h-3" /> Bandwidth Down
              </span>
              <p className="text-text-primary font-medium text-xl mt-1">
                {usage.total_bandwidth_down != null ? formatBytes(usage.total_bandwidth_down) : '—'}
              </p>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
