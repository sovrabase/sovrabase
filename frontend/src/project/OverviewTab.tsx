import { useState, useEffect } from 'react';
import { Copy, Eye, EyeOff, Save, Database, Globe, Settings, HardDrive, Check } from 'lucide-react';
import { api, formatBytes, formatDate } from '../api';
import { useToast } from '../components/Toast';
import type { Project } from '../types';

interface Props {
  projectId: string;
}

interface ProjectDetail {
  project: Project;
  api_key: string;
  api_url: string;
  allow_origins: string;
  storage_quota: number;
}

interface DbAnalysis {
  collections: { name: string; size: number; doc_count: number }[];
  metadata_overhead: number;
  indexes: { name: string; size: number }[];
  total_size: number;
}

const UNIT_MULTIPLIERS: Record<string, number> = {
  MB: 1_048_576,
  GB: 1_073_741_824,
  TB: 1_099_511_627_776,
};

function humanizeBytes(bytes: number): { val: number; unit: string } {
  if (bytes <= 0) return { val: 0, unit: 'MB' };
  const tb = bytes / UNIT_MULTIPLIERS.TB;
  const gb = bytes / UNIT_MULTIPLIERS.GB;
  const mb = bytes / UNIT_MULTIPLIERS.MB;
  if (tb >= 1 && Number.isInteger(tb)) return { val: tb, unit: 'TB' };
  if (gb >= 1 && Number.isInteger(gb)) return { val: gb, unit: 'GB' };
  return { val: Math.round(mb * 100) / 100, unit: 'MB' };
}

export default function OverviewTab({ projectId }: Props) {
  const { showToast } = useToast();

  const [detail, setDetail] = useState<ProjectDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [keyRevealed, setKeyRevealed] = useState(false);

  // Settings state
  const [allowOrigins, setAllowOrigins] = useState('*');
  const [quotaVal, setQuotaVal] = useState(100);
  const [quotaUnit, setQuotaUnit] = useState('MB');
  const [saving, setSaving] = useState(false);
  const [saveStatus, setSaveStatus] = useState<'idle' | 'success' | 'error'>('idle');

  // DB analysis
  const [analysis, setAnalysis] = useState<DbAnalysis | null>(null);
  const [analyzing, setAnalyzing] = useState(false);
  const [showAnalysis, setShowAnalysis] = useState(false);

  const fetchDetail = () => {
    setLoading(true);
    setError(null);
    api<ProjectDetail>(`/admin/projects/${encodeURIComponent(projectId)}`)
      .then((data) => {
        setDetail(data);
        setAllowOrigins(data.allow_origins || '*');
        const h = humanizeBytes(data.storage_quota ?? 0);
        setQuotaVal(h.val);
        setQuotaUnit(h.unit);
      })
      .catch((err) => setError((err as Error).message))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchDetail();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectId]);

  const copyKey = async () => {
    if (!detail?.api_key) return;
    try {
      await navigator.clipboard.writeText(detail.api_key);
      showToast('API key copied', 'success');
    } catch {
      showToast('Failed to copy', 'error');
    }
  };

  const handleSave = async () => {
    if (!detail) return;
    setSaving(true);
    setSaveStatus('idle');
    const storageQuota = quotaVal * (UNIT_MULTIPLIERS[quotaUnit] || UNIT_MULTIPLIERS.MB);
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}`, {
        method: 'PUT',
        body: JSON.stringify({
          allow_origins: allowOrigins,
          storage_quota: Math.round(storageQuota),
        }),
      });
      setSaveStatus('success');
      showToast('Settings saved', 'success');
      fetchDetail();
    } catch (err) {
      setSaveStatus('error');
      showToast((err as Error).message, 'error');
    } finally {
      setSaving(false);
      setTimeout(() => setSaveStatus('idle'), 3000);
    }
  };

  const runAnalysis = async () => {
    setAnalyzing(true);
    try {
      const data = await api<DbAnalysis>(`/admin/projects/${encodeURIComponent(projectId)}/db-analysis`);
      setAnalysis(data);
      setShowAnalysis(true);
    } catch (err) {
      showToast((err as Error).message, 'error');
    } finally {
      setAnalyzing(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20 text-text-muted">
        <div className="w-5 h-5 border-2 border-accent border-t-transparent rounded-full animate-spin mr-3" />
        Loading project...
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center py-20 gap-3">
        <p className="text-danger">{error}</p>
        <button onClick={fetchDetail} className="px-4 py-2 rounded-lg bg-accent text-white text-sm hover:bg-accent-hover transition-colors">
          Retry
        </button>
      </div>
    );
  }

  if (!detail) return null;

  const maskKey = (key: string) => {
    if (key.length <= 12) return '\u25CF'.repeat(key.length);
    return '\u25CF'.repeat(key.length - 4) + key.slice(-4);
  };

  return (
    <div className="space-y-6">
      {/* Top row: 2 columns */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* CARD 1 - Project Information */}
        <div className="bg-bg-card border border-border rounded-xl p-6 space-y-5">
          <h2 className="text-lg font-semibold text-text-primary flex items-center gap-2">
            <HardDrive className="w-5 h-5 text-accent" />
            Project Information
          </h2>

          <div className="space-y-3">
            <div>
              <p className="text-text-muted text-xs uppercase tracking-wider mb-1">Name</p>
              <p className="text-text-primary font-medium">{detail.project.name}</p>
            </div>
            <div>
              <p className="text-text-muted text-xs uppercase tracking-wider mb-1">Project ID</p>
              <code className="text-text-secondary text-sm font-mono">{detail.project.id}</code>
            </div>
            <div>
              <p className="text-text-muted text-xs uppercase tracking-wider mb-1">Status</p>
              <span className="inline-flex items-center gap-1.5">
                <span className={`w-2 h-2 rounded-full ${detail.project.is_online ? 'bg-success' : 'bg-text-muted'}`} />
                <span className="text-text-secondary text-sm">{detail.project.is_online ? 'Online' : 'Offline'}</span>
              </span>
            </div>
            <div>
              <p className="text-text-muted text-xs uppercase tracking-wider mb-1">Created</p>
              <p className="text-text-secondary text-sm">{formatDate(detail.project.created_at)}</p>
            </div>
          </div>

          {/* API Credentials subsection */}
          <div className="border-t border-border pt-4 space-y-3">
            <h3 className="text-sm font-semibold text-text-primary flex items-center gap-2">
              <KeyIcon className="w-4 h-4 text-accent" />
              API Credentials
            </h3>
            <div>
              <p className="text-text-muted text-xs uppercase tracking-wider mb-1">API URL</p>
              <code className="text-text-secondary text-sm font-mono break-all">{detail.api_url}</code>
            </div>
            <div>
              <p className="text-text-muted text-xs uppercase tracking-wider mb-1">API Key</p>
              {detail.api_key ? (
                <div className="flex items-center gap-2">
                  <code className="text-text-secondary text-sm font-mono break-all flex-1">
                    {keyRevealed ? detail.api_key : maskKey(detail.api_key)}
                    {!keyRevealed && (
                      <span className="text-text-muted ml-2 font-sans text-xs">(click to reveal)</span>
                    )}
                  </code>
                  <button
                    onClick={() => setKeyRevealed(!keyRevealed)}
                    className="p-1.5 rounded hover:bg-bg-input text-text-muted hover:text-text-primary transition-colors"
                    title={keyRevealed ? 'Hide' : 'Reveal'}
                  >
                    {keyRevealed ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                  </button>
                  <button
                    onClick={copyKey}
                    className="p-1.5 rounded hover:bg-bg-input text-text-muted hover:text-text-primary transition-colors"
                    title="Copy"
                  >
                    <Copy className="w-4 h-4" />
                  </button>
                </div>
              ) : (
                <p className="text-text-muted text-sm">No API key generated</p>
              )}
            </div>
          </div>
        </div>

        {/* CARD 2 - Project Settings */}
        <div className="bg-bg-card border border-border rounded-xl p-6 space-y-5">
          <h2 className="text-lg font-semibold text-text-primary flex items-center gap-2">
            <Settings className="w-5 h-5 text-accent" />
            Project Settings
          </h2>

          <div className="space-y-4">
            {/* Allowed Origins */}
            <div>
              <label className="flex items-center gap-2 text-text-secondary text-sm font-medium mb-1.5">
                <Globe className="w-4 h-4" />
                Allowed Origins (CORS)
              </label>
              <input
                type="text"
                value={allowOrigins}
                onChange={(e) => setAllowOrigins(e.target.value)}
                className="w-full px-3 py-2 rounded-lg bg-bg-input border border-border text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent transition-colors text-sm"
                placeholder="*"
              />
              <p className="text-text-muted text-xs mt-1">Comma-separated origins or * for all</p>
            </div>

            {/* Storage Quota */}
            <div>
              <label className="flex items-center gap-2 text-text-secondary text-sm font-medium mb-1.5">
                <Database className="w-4 h-4" />
                Storage Quota Limit
              </label>
              <div className="flex gap-2">
                <input
                  type="number"
                  value={quotaVal}
                  onChange={(e) => setQuotaVal(Math.max(0, Number(e.target.value)))}
                  className="flex-1 px-3 py-2 rounded-lg bg-bg-input border border-border text-text-primary focus:outline-none focus:border-accent transition-colors text-sm"
                  min={0}
                  step={quotaUnit === 'TB' ? 1 : quotaUnit === 'GB' ? 1 : 10}
                />
                <select
                  value={quotaUnit}
                  onChange={(e) => setQuotaUnit(e.target.value)}
                  className="px-3 py-2 rounded-lg bg-bg-input border border-border text-text-primary focus:outline-none focus:border-accent transition-colors text-sm"
                >
                  <option value="MB">MB</option>
                  <option value="GB">GB</option>
                  <option value="TB">TB</option>
                </select>
              </div>
            </div>

            {/* Save button + status */}
            <div className="flex items-center gap-3 pt-2">
              <button
                onClick={handleSave}
                disabled={saving}
                className="flex items-center gap-2 px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover disabled:opacity-50 transition-colors"
              >
                <Save className="w-4 h-4" />
                {saving ? 'Saving...' : 'Save Settings'}
              </button>
              {saveStatus === 'success' && (
                <span className="flex items-center gap-1 text-success text-sm">
                  <Check className="w-4 h-4" /> Saved
                </span>
              )}
              {saveStatus === 'error' && (
                <span className="text-danger text-sm">Failed to save</span>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* CARD 3 - Database Storage Analysis (full width) */}
      <div className="bg-bg-card border border-border rounded-xl p-6 space-y-4">
        <div>
          <h2 className="text-lg font-semibold text-text-primary flex items-center gap-2">
            <Database className="w-5 h-5 text-accent" />
            Database Storage Analysis
          </h2>
          <p className="text-text-muted text-sm mt-1">
            Analyze collection sizes, metadata overhead, and index usage across your project database.
          </p>
        </div>

        <button
          onClick={runAnalysis}
          disabled={analyzing}
          className="flex items-center gap-2 px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover disabled:opacity-50 transition-colors"
        >
          {analyzing ? (
            <>
              <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
              Analyzing...
            </>
          ) : (
            'Run Deep Analysis'
          )}
        </button>

        {showAnalysis && analysis && (
          <div className="border-t border-border pt-4 space-y-4">
            {/* Summary */}
            <div className="flex items-center gap-2 text-text-secondary text-sm">
              <Database className="w-4 h-4" />
              Total storage:{' '}
              <span className="font-semibold text-text-primary">{formatBytes(analysis.total_size)}</span>
            </div>

            {/* Collections breakdown */}
            {analysis.collections.length > 0 && (
              <div>
                <h4 className="text-sm font-medium text-text-primary mb-2">Collections</h4>
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b border-border">
                        <th className="text-left text-text-muted text-xs font-medium py-2 pr-4">Name</th>
                        <th className="text-left text-text-muted text-xs font-medium py-2 pr-4">Documents</th>
                        <th className="text-left text-text-muted text-xs font-medium py-2">Size</th>
                      </tr>
                    </thead>
                    <tbody>
                      {analysis.collections.map((c) => (
                        <tr key={c.name} className="border-b border-border/40">
                          <td className="py-2 pr-4 text-text-primary font-mono text-xs">{c.name}</td>
                          <td className="py-2 pr-4 text-text-secondary">{c.doc_count.toLocaleString()}</td>
                          <td className="py-2 text-text-secondary">{formatBytes(c.size)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}

            {/* Metadata overhead */}
            <div>
              <h4 className="text-sm font-medium text-text-primary mb-1">Metadata Overhead</h4>
              <p className="text-text-secondary text-sm">{formatBytes(analysis.metadata_overhead)}</p>
            </div>

            {/* Indexes */}
            {analysis.indexes.length > 0 && (
              <div>
                <h4 className="text-sm font-medium text-text-primary mb-2">Indexes</h4>
                <div className="space-y-1">
                  {analysis.indexes.map((idx) => (
                    <div key={idx.name} className="flex justify-between text-sm">
                      <code className="text-text-primary font-mono text-xs">{idx.name}</code>
                      <span className="text-text-muted">{formatBytes(idx.size)}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

function KeyIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4" />
    </svg>
  );
}
