import { useState, useEffect } from 'react';
import { Clock, ToggleLeft, ToggleRight, Plus, Loader2, Play, Trash2, Pencil, History } from 'lucide-react';
import { api, formatDate } from '../api';
import type { CronJob, CronJobExecution, ServerlessFunction } from '../types';
import Modal from '../components/Modal';
import { useToast } from '../components/Toast';

interface Props { projectId: string; apiKey?: string; }

const emptyForm = () => ({ name: '', schedule: '', method: 'POST', url: '', body: '', enabled: 'true', functionName: '' });

export default function CronTab({ projectId, apiKey }: Props) {
  const [jobs, setJobs] = useState<CronJob[]>([]);
  const [functions, setFunctions] = useState<ServerlessFunction[]>([]);
  const [loading, setLoading] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState(emptyForm());
  const { showToast } = useToast();
  const [invoking, setInvoking] = useState<Record<string, 'loading' | 'success' | 'error'>>({});
  const [historyJob, setHistoryJob] = useState<CronJob | null>(null);
  const [executions, setExecutions] = useState<CronJobExecution[]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);

  const jobUrl = (job: CronJob) => job.url || job.endpoint || '';

  const load = () => {
    setLoading(true);
    Promise.all([
      api<{ data: CronJob[] }>(`/admin/projects/${encodeURIComponent(projectId)}/cron`),
      api<{ data: ServerlessFunction[] }>(`/admin/projects/${encodeURIComponent(projectId)}/functions`).catch(() => ({ data: [] })),
    ])
      .then(([cronRes, fnRes]) => {
        setJobs(cronRes.data || []);
        setFunctions((fnRes.data || []).filter((f) => f.trigger === 'http'));
      })
      .finally(() => setLoading(false));
  };

  useEffect(() => { load(); }, [projectId]);

  const applyFunctionUrl = (name: string) => {
    if (!name) return;
    const origin = window.location.origin.replace(/:\d+$/, ':6070');
    const key = apiKey ? `?project_key=${encodeURIComponent(apiKey)}` : '';
    setForm((f) => ({ ...f, functionName: name, url: `${origin}/api/v1/functions/${name}${key}` }));
  };

  const openCreate = () => {
    setEditingId(null);
    setForm(emptyForm());
    setShowModal(true);
  };

  const openEdit = (job: CronJob) => {
    setEditingId(job.id);
    setForm({
      name: job.name,
      schedule: job.schedule,
      method: job.method || 'POST',
      url: jobUrl(job),
      body: job.body ? (typeof job.body === 'string' ? job.body : JSON.stringify(job.body, null, 2)) : '',
      enabled: job.enabled ? 'true' : 'false',
      functionName: '',
    });
    setShowModal(true);
  };

  const save = async () => {
    if (!form.name.trim() || !form.schedule.trim() || !form.url.trim()) {
      showToast('Name, schedule, and URL are required', 'error');
      return;
    }
    setSaving(true);
    try {
      const body: Record<string, unknown> = {
        name: form.name.trim(),
        schedule: form.schedule.trim(),
        method: form.method,
        url: form.url.trim(),
        enabled: form.enabled === 'true',
      };
      if (form.body.trim()) {
        try { body.body = JSON.parse(form.body); } catch { body.body = form.body; }
      }
      if (editingId) {
        await api(`/admin/projects/${encodeURIComponent(projectId)}/cron/${encodeURIComponent(editingId)}`, {
          method: 'PUT',
          body: JSON.stringify({ ...body, id: editingId }),
        });
        showToast('Cron job updated', 'success');
      } else {
        await api(`/admin/projects/${encodeURIComponent(projectId)}/cron`, {
          method: 'POST',
          body: JSON.stringify(body),
        });
        showToast('Cron job created', 'success');
      }
      setShowModal(false);
      setForm(emptyForm());
      load();
    } catch (err) {
      showToast((err as Error).message, 'error');
    } finally {
      setSaving(false);
    }
  };

  const openHistory = async (job: CronJob) => {
    setHistoryJob(job);
    setHistoryLoading(true);
    try {
      const res = await api<{ data: CronJobExecution[] }>(
        `/admin/projects/${encodeURIComponent(projectId)}/cron/${encodeURIComponent(job.id)}/executions`,
      );
      setExecutions(res.data || []);
    } catch {
      setExecutions([]);
    } finally {
      setHistoryLoading(false);
    }
  };

  const handleInvoke = async (job: CronJob) => {
    const endpoint = jobUrl(job);
    if (!endpoint) { showToast('No URL configured', 'error'); return; }
    setInvoking((prev) => ({ ...prev, [job.id]: 'loading' }));
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 8000);
    try {
      const init: RequestInit = {
        method: job.method || 'POST',
        headers: { 'Content-Type': 'application/json' },
        signal: controller.signal,
      };
      if (job.body && init.method !== 'GET') {
        init.body = typeof job.body === 'string' ? job.body : JSON.stringify(job.body);
      }
      const res = await fetch(endpoint, init);
      clearTimeout(timeoutId);
      if (!res.ok) throw new Error(`HTTP ${res.status}: ${res.statusText}`);
      setInvoking((prev) => ({ ...prev, [job.id]: 'success' }));
      showToast(`Triggered "${job.name}" — OK`, 'success');
    } catch (err: unknown) {
      clearTimeout(timeoutId);
      setInvoking((prev) => ({ ...prev, [job.id]: 'error' }));
      const msg = (err as Error).name === 'AbortError' ? 'Request timed out (8s)' : (err as Error).message;
      showToast(`Trigger failed: ${msg}`, 'error');
    } finally {
      setTimeout(() => setInvoking((prev) => {
        const next = { ...prev };
        delete next[job.id];
        return next;
      }), 2000);
    }
  };

  const handleDelete = async (jobId: string) => {
    if (!confirm('Delete this cron job?')) return;
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/cron/${jobId}`, { method: 'DELETE' });
      showToast('Cron job deleted', 'success');
      load();
    } catch (err) {
      showToast((err as Error).message, 'error');
    }
  };

  const inputCls = 'w-full px-3 py-2 rounded-lg bg-bg-input border border-border text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent transition-colors text-sm';

  if (loading) {
    return <div className="flex items-center gap-2 py-10 text-text-muted"><Loader2 className="w-4 h-4 animate-spin" /> Loading cron jobs...</div>;
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-text-primary">Cron Jobs ({jobs.length})</h2>
        <button onClick={openCreate} className="flex items-center gap-2 px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover transition-colors">
          <Plus className="w-4 h-4" /> Add Job
        </button>
      </div>

      {jobs.length === 0 ? (
        <div className="flex flex-col items-center py-16 text-text-muted gap-3">
          <Clock className="w-10 h-10" />
          <p>No cron jobs</p>
        </div>
      ) : (
        <div className="border border-border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-bg-input text-text-muted text-xs uppercase">
              <tr>
                <th className="text-left px-4 py-3 font-medium">Name</th>
                <th className="text-left px-4 py-3 font-medium">Schedule</th>
                <th className="text-left px-4 py-3 font-medium">URL</th>
                <th className="text-left px-4 py-3 font-medium">Status</th>
                <th className="text-left px-4 py-3 font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {jobs.map((job) => (
                <tr key={job.id} className="hover:bg-bg-input/50 transition-colors">
                  <td className="px-4 py-3 font-medium text-text-primary">{job.name}</td>
                  <td className="px-4 py-3"><span className="inline-flex px-2 py-0.5 bg-bg-input border border-border rounded text-xs font-mono text-accent">{job.schedule}</span></td>
                  <td className="px-4 py-3 text-text-secondary font-mono text-xs max-w-[200px] truncate" title={jobUrl(job)}>{jobUrl(job) || '—'}</td>
                  <td className="px-4 py-3">
                    {job.enabled ? (
                      <span className="inline-flex items-center gap-1 text-xs text-success"><ToggleRight className="w-4 h-4" /> Active</span>
                    ) : (
                      <span className="inline-flex items-center gap-1 text-xs text-text-muted"><ToggleLeft className="w-4 h-4" /> Disabled</span>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-1">
                      {invoking[job.id] === 'loading' ? (
                        <Loader2 className="w-4 h-4 animate-spin text-accent" />
                      ) : invoking[job.id] === 'success' ? (
                        <span className="text-success text-xs font-medium">OK</span>
                      ) : invoking[job.id] === 'error' ? (
                        <span className="text-error text-xs font-medium">Fail</span>
                      ) : (
                        <button onClick={() => handleInvoke(job)} className="p-1.5 rounded-md text-text-muted hover:text-accent hover:bg-accent/10" title="Trigger now">
                          <Play className="w-4 h-4" />
                        </button>
                      )}
                      <button onClick={() => openEdit(job)} className="p-1.5 rounded-md text-text-muted hover:text-accent hover:bg-accent/10" title="Edit">
                        <Pencil className="w-4 h-4" />
                      </button>
                      <button onClick={() => openHistory(job)} className="p-1.5 rounded-md text-text-muted hover:text-accent hover:bg-accent/10" title="History">
                        <History className="w-4 h-4" />
                      </button>
                      <button onClick={() => handleDelete(job.id)} className="p-1.5 rounded-md text-text-muted hover:text-error hover:bg-error/10" title="Delete">
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <Modal isOpen={showModal} onClose={() => setShowModal(false)} title={editingId ? 'Edit Scheduled Job' : 'Add Scheduled Job'} size="md">
        <div className="space-y-4">
          <div>
            <label className="text-text-secondary text-sm font-medium">Job Name</label>
            <input type="text" value={form.name} onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))} className={inputCls} placeholder="e.g. daily-cleanup" autoFocus />
          </div>
          <div>
            <label className="text-text-secondary text-sm font-medium">Schedule <span className="text-text-muted text-xs">(cron)</span></label>
            <input type="text" value={form.schedule} onChange={(e) => setForm((f) => ({ ...f, schedule: e.target.value }))} className={inputCls} placeholder="*/5 * * * *" />
          </div>
          {functions.length > 0 && (
            <div>
              <label className="text-text-secondary text-sm font-medium">Call HTTP function (optional)</label>
              <select
                value={form.functionName}
                onChange={(e) => applyFunctionUrl(e.target.value)}
                className={inputCls}
              >
                <option value="">— custom URL —</option>
                {functions.map((fn) => (
                  <option key={fn.id} value={fn.name}>{fn.name}</option>
                ))}
              </select>
            </div>
          )}
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-text-secondary text-sm font-medium">HTTP Method</label>
              <select value={form.method} onChange={(e) => setForm((f) => ({ ...f, method: e.target.value }))} className={inputCls}>
                <option value="POST">POST</option>
                <option value="GET">GET</option>
                <option value="PUT">PUT</option>
                <option value="DELETE">DELETE</option>
              </select>
            </div>
            <div>
              <label className="text-text-secondary text-sm font-medium">Enabled</label>
              <select value={form.enabled} onChange={(e) => setForm((f) => ({ ...f, enabled: e.target.value }))} className={inputCls}>
                <option value="true">Yes</option>
                <option value="false">No (paused)</option>
              </select>
            </div>
          </div>
          <div>
            <label className="text-text-secondary text-sm font-medium">Callback URL</label>
            <input type="text" value={form.url} onChange={(e) => setForm((f) => ({ ...f, url: e.target.value }))} className={inputCls} placeholder="https://..." />
          </div>
          <div>
            <label className="text-text-secondary text-sm font-medium">Request Body (JSON, optional)</label>
            <textarea value={form.body} onChange={(e) => setForm((f) => ({ ...f, body: e.target.value }))} rows={3} className={inputCls + ' resize-y font-mono text-xs'} placeholder='{"key":"value"}' />
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <button onClick={() => setShowModal(false)} className="px-4 py-2 rounded-lg text-text-secondary text-sm hover:bg-bg-input transition-colors">Cancel</button>
            <button onClick={save} disabled={saving} className="px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover disabled:opacity-50 transition-colors">
              {saving ? 'Saving...' : editingId ? 'Update' : 'Create Job'}
            </button>
          </div>
        </div>
      </Modal>

      <Modal isOpen={!!historyJob} onClose={() => { setHistoryJob(null); setExecutions([]); }} title={`History: ${historyJob?.name || ''}`} size="md">
        {historyLoading ? (
          <div className="flex items-center gap-2 py-6 text-text-muted justify-center"><Loader2 className="w-4 h-4 animate-spin" /> Loading...</div>
        ) : executions.length === 0 ? (
          <p className="text-text-muted text-sm text-center py-6">No executions recorded</p>
        ) : (
          <table className="w-full text-xs">
            <thead className="text-text-muted uppercase">
              <tr>
                <th className="text-left py-2">Start</th>
                <th className="text-left py-2">Status</th>
                <th className="text-left py-2">Duration</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {executions.map((ex, i) => {
                const dur = ex.start_time && ex.end_time
                  ? new Date(ex.end_time).getTime() - new Date(ex.start_time).getTime()
                  : 0;
                return (
                  <tr key={i}>
                    <td className="py-2 text-text-secondary">{formatDate(ex.start_time)}</td>
                    <td className={`py-2 ${ex.status === 'success' ? 'text-success' : 'text-danger'}`}>{ex.status}</td>
                    <td className="py-2 font-mono">{dur} ms</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </Modal>
    </div>
  );
}
