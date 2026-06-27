import { useState, useEffect } from 'react';
import { Clock, ToggleLeft, ToggleRight, Plus, Loader2, X } from 'lucide-react';
import { api, formatDate } from '../api';
import type { CronJob } from '../types';
import Modal from '../components/Modal';
import { useToast } from '../components/Toast';

interface Props { projectId: string; apiKey?: string; }

export default function CronTab({ projectId }: Props) {
  const [jobs, setJobs] = useState<CronJob[]>([]);
  const [loading, setLoading] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({ name: '', schedule: '', method: 'POST', url: '', body: '', enabled: 'true' });
  const { showToast } = useToast();

  const load = () => {
    setLoading(true);
    api<CronJob[]>(`/admin/projects/${encodeURIComponent(projectId)}/cron`)
      .then(setJobs)
      .finally(() => setLoading(false));
  };

  useEffect(() => { load(); }, [projectId]);

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
      await api<unknown>(`/admin/projects/${encodeURIComponent(projectId)}/cron`, {
        method: 'POST',
        body: JSON.stringify(body),
      });
      showToast('Cron job created', 'success');
      setShowModal(false);
      setForm({ name: '', schedule: '', method: 'POST', url: '', body: '', enabled: 'true' });
      load();
    } catch (err) {
      showToast((err as Error).message, 'error');
    } finally {
      setSaving(false);
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
        <button onClick={() => setShowModal(true)} className="flex items-center gap-2 px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover transition-colors">
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
                <th className="text-left px-4 py-3 font-medium">Endpoint</th>
                <th className="text-left px-4 py-3 font-medium">Status</th>
                <th className="text-left px-4 py-3 font-medium">Last Run</th>
                <th className="text-left px-4 py-3 font-medium">Next Run</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {jobs.map((job) => (
                <tr key={job.id} className="hover:bg-bg-input/50 transition-colors">
                  <td className="px-4 py-3 font-medium text-text-primary">{job.name}</td>
                  <td className="px-4 py-3"><span className="inline-flex px-2 py-0.5 bg-bg-input border border-border rounded text-xs font-mono text-accent">{job.schedule}</span></td>
                  <td className="px-4 py-3 text-text-secondary font-mono text-xs">{job.endpoint || '—'}</td>
                  <td className="px-4 py-3">
                    {job.enabled ? (
                      <span className="inline-flex items-center gap-1 text-xs text-success"><ToggleRight className="w-4 h-4" /> Active</span>
                    ) : (
                      <span className="inline-flex items-center gap-1 text-xs text-text-muted"><ToggleLeft className="w-4 h-4" /> Disabled</span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-text-secondary text-xs">{formatDate(job.last_run)}</td>
                  <td className="px-4 py-3 text-text-secondary text-xs">{job.next_run ? formatDate(job.next_run) : '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <Modal isOpen={showModal} onClose={() => setShowModal(false)} title="Add Scheduled Job" size="md">
        <div className="space-y-4">
          <div>
            <label className="text-text-secondary text-sm font-medium">Job Name</label>
            <input type="text" value={form.name} onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))} className={inputCls} placeholder="e.g. daily-cleanup" autoFocus />
          </div>
          <div>
            <label className="text-text-secondary text-sm font-medium">Schedule <span className="text-text-muted text-xs">(cron: min hour dom month dow)</span></label>
            <input type="text" value={form.schedule} onChange={(e) => setForm((f) => ({ ...f, schedule: e.target.value }))} className={inputCls} placeholder="*/5 * * * *" />
            <p className="text-text-muted text-xs mt-1">Examples: <code>*/5 * * * *</code> (every 5min) · <code>0 * * * *</code> (hourly) · <code>0 2 * * *</code> (daily 2am) · <code>0 0 * * 0</code> (weekly)</p>
          </div>
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
            <input type="text" value={form.url} onChange={(e) => setForm((f) => ({ ...f, url: e.target.value }))} className={inputCls} placeholder="https://your-app.com/api/cleanup" />
          </div>
          <div>
            <label className="text-text-secondary text-sm font-medium">Request Body (JSON, optional)</label>
            <textarea value={form.body} onChange={(e) => setForm((f) => ({ ...f, body: e.target.value }))} rows={3} className={inputCls + ' resize-y font-mono text-xs'} placeholder='{"key":"value"}' />
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <button onClick={() => setShowModal(false)} className="px-4 py-2 rounded-lg text-text-secondary text-sm hover:bg-bg-input transition-colors">Cancel</button>
            <button onClick={save} disabled={saving} className="px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover disabled:opacity-50 transition-colors">
              {saving ? 'Creating...' : 'Create Job'}
            </button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
