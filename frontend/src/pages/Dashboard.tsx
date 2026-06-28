import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { FolderKanban, HardDrive, Globe, Cloud, Activity, ArrowUp, ArrowDown, Loader2, AlertTriangle, Zap, Copy, Check, Cpu } from 'lucide-react';
import { useDashboard } from '../store';
import { formatBytes } from '../api';
import { StatCard } from '../components/StatCard';

// regionFlags removed (unused)

type QuickTab = 'create' | 'flutter' | 'rest';

const quickTabs: { key: QuickTab; label: string }[] = [
  { key: 'create', label: 'Create Project' },
  { key: 'flutter', label: 'Flutter SDK' },
  { key: 'rest', label: 'REST API' },
];

const flutterSnippet = `import 'package:sovrabase_client/sovrabase_client.dart';

final client = SovrabaseClient(
  projectId: 'YOUR_PROJECT_ID',
  apiKey: 'YOUR_API_KEY',
);

// Fetch a collection
final docs = await client.from('todos').find();`;

const restExamples = (origin: string) => [
  { label: 'List collections', cmd: `curl -H "Authorization: Bearer API_KEY" \\\n  "${origin}/api/rest/v1/collections"` },
  { label: 'Create document', cmd: `curl -X POST \\\n  -H "Authorization: Bearer API_KEY" \\\n  -H "Content-Type: application/json" \\\n  -d '{"title":"Hello"}' \\\n  "${origin}/api/rest/v1/collections/todos/documents"` },
];

export default function Dashboard() {
  const { stats, usage, replication, loading, error, loadDashboard } = useDashboard();
  const navigate = useNavigate();
  const [quickTab, setQuickTab] = useState<QuickTab>('create');
  const [copied, setCopied] = useState<number | null>(null);

  useEffect(() => {
    loadDashboard();
  }, [loadDashboard]);

  const copyText = (text: string, idx: number) => {
    navigator.clipboard.writeText(text);
    setCopied(idx);
    setTimeout(() => setCopied(null), 2000);
  };

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

      <div className="grid grid-cols-1 md:grid-cols-4 gap-5">
        <StatCard
          icon={<FolderKanban className="w-5 h-5" />}
          label="Projects"
          value={stats?.projects ?? '—'}
        />
        <StatCard
          icon={<Cpu className="w-5 h-5" />}
          label="Memory Usage"
          value={stats?.memory_bytes != null ? formatBytes(stats.memory_bytes) : '—'}
        />
        <StatCard
          icon={<HardDrive className="w-5 h-5" />}
          label="Storage Used"
          value={stats?.storage_bytes != null ? formatBytes(stats.storage_bytes) : '—'}
          subtitle={stats?.max_storage_bytes != null ? `of ${formatBytes(stats.max_storage_bytes)}` : undefined}
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

      {/* Quick Start */}
      <div className="bg-bg-card border border-border rounded-xl p-6">
        <h2 className="flex items-center gap-2 text-lg font-semibold text-text-primary mb-4">
          <Zap className="w-5 h-5 text-accent" />
          Quick Start
        </h2>

        <div className="flex gap-1 border-b border-border mb-5">
          {quickTabs.map((qt) => (
            <button
              key={qt.key}
              onClick={() => { setQuickTab(qt.key); setCopied(null); }}
              className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                quickTab === qt.key
                  ? 'border-accent text-accent'
                  : 'border-transparent text-text-muted hover:text-text-secondary'
              }`}
            >
              {qt.label}
            </button>
          ))}
        </div>

        {quickTab === 'create' && (
          <div className="space-y-3">
            <p className="text-text-secondary text-sm">Create a project to get your API keys, database, and storage bucket — everything you need to start building.</p>
            <button
              onClick={() => navigate('/projects')}
              className="inline-flex items-center gap-2 px-5 py-2.5 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover transition-colors"
            >
              Create your first project
            </button>
          </div>
        )}

        {quickTab === 'flutter' && (
          <div className="relative">
            <div className="bg-bg-input rounded-lg p-4 overflow-x-auto">
              <pre className="text-text-secondary text-sm font-mono leading-relaxed whitespace-pre">{flutterSnippet}</pre>
            </div>
            <button
              onClick={() => copyText(flutterSnippet, 0)}
              className="absolute top-3 right-3 p-1.5 rounded-md bg-bg-card border border-border text-text-muted hover:text-text-primary transition-colors"
              title="Copy"
            >
              {copied === 0 ? <Check className="w-4 h-4 text-success" /> : <Copy className="w-4 h-4" />}
            </button>
          </div>
        )}

        {quickTab === 'rest' && (
          <div className="space-y-4">
            {restExamples(window.location.origin).map((ex, idx) => (
              <div key={idx} className="relative">
                <p className="text-text-secondary text-sm font-medium mb-1.5">{ex.label}</p>
                <div className="bg-bg-input rounded-lg p-4 overflow-x-auto">
                  <pre className="text-text-secondary text-sm font-mono leading-relaxed whitespace-pre">{ex.cmd}</pre>
                </div>
                <button
                  onClick={() => copyText(ex.cmd, idx + 1)}
                  className="absolute top-7 right-3 p-1.5 rounded-md bg-bg-card border border-border text-text-muted hover:text-text-primary transition-colors"
                  title="Copy"
                >
                  {copied === idx + 1 ? <Check className="w-4 h-4 text-success" /> : <Copy className="w-4 h-4" />}
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
