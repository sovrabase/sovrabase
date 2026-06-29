import { useState, useEffect } from 'react';
import { Webhook, ToggleLeft, ToggleRight, Plus, Loader2, Trash2 } from 'lucide-react';
import { api, formatDate } from '../api';
import type { Webhook as WebhookType } from '../types';
import Modal from '../components/Modal';
import { useToast } from '../components/Toast';

interface Props { projectId: string; apiKey?: string; }

export default function WebhooksTab({ projectId }: Props) {
  const [webhooks, setWebhooks] = useState<WebhookType[]>([]);
  const [loading, setLoading] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({ name: '', url: '', events: '', enabled: 'true' });
  const { showToast } = useToast();

  const load = () => {
    setLoading(true);
    api<{ data: WebhookType[] }>(`/admin/projects/${encodeURIComponent(projectId)}/webhooks`)
      .then((res) => setWebhooks(res.data || []))
      .finally(() => setLoading(false));
  };

  useEffect(() => { load(); }, [projectId]);

  const handleDelete = async (webhookId: string) => {
    if (!confirm('Delete this webhook?')) return;
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/webhooks/${webhookId}`, { method: 'DELETE' });
      showToast('Webhook deleted', 'success');
      load();
    } catch (err) {
      showToast((err as Error).message, 'error');
    }
  };

  const save = async () => {
    if (!form.url.trim()) { showToast('URL is required', 'error'); return; }
    setSaving(true);
    try {
      const events = form.events.trim() ? form.events.split(',').map((s) => s.trim()).filter(Boolean).join(',') : '';
      await api<unknown>(`/admin/projects/${encodeURIComponent(projectId)}/webhooks`, {
        method: 'POST',
        body: JSON.stringify({
          url: form.url.trim(),
          events,
          enabled: form.enabled === 'true',
        }),
      });
      showToast('Webhook created', 'success');
      setShowModal(false);
      setForm({ name: '', url: '', events: '', enabled: 'true' });
      load();
    } catch (err) {
      showToast((err as Error).message, 'error');
    } finally { setSaving(false); }
  };

  const truncateUrl = (url: string) => url.length > 50 ? url.slice(0, 47) + '...' : url;
  const inputCls = 'w-full px-3 py-2 rounded-lg bg-bg-input border border-border text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent transition-colors text-sm';

  if (loading) {
    return <div className="flex items-center gap-2 py-10 text-text-muted"><Loader2 className="w-4 h-4 animate-spin" /> Loading webhooks...</div>;
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-text-primary">Webhooks ({webhooks.length})</h2>
        <button onClick={() => setShowModal(true)} className="flex items-center gap-2 px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover transition-colors">
          <Plus className="w-4 h-4" /> Add Webhook
        </button>
      </div>

      {webhooks.length === 0 ? (
        <div className="flex flex-col items-center py-16 text-text-muted gap-3"><Webhook className="w-10 h-10" /><p>No webhooks</p></div>
      ) : (
        <div className="border border-border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-bg-input text-text-muted text-xs uppercase">
              <tr>
                <th className="text-left px-4 py-3 font-medium">Name</th>
                <th className="text-left px-4 py-3 font-medium">URL</th>
                <th className="text-left px-4 py-3 font-medium">Events</th>
                <th className="text-left px-4 py-3 font-medium">Status</th>
                <th className="text-left px-4 py-3 font-medium">Created</th>
                <th className="text-left px-4 py-3 font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {webhooks.map((w) => (
                <tr key={w.id} className="hover:bg-bg-input/50 transition-colors">
                  <td className="px-4 py-3 font-medium text-text-primary">{w.name}</td>
                  <td className="px-4 py-3 text-text-secondary font-mono text-xs" title={w.url}>{truncateUrl(w.url)}</td>
                  <td className="px-4 py-3"><div className="flex gap-1 flex-wrap">{w.events?.map((ev, i) => (<span key={i} className="inline-flex px-2 py-0.5 bg-accent/10 text-accent rounded-full text-xs font-mono">{ev}</span>))}</div></td>
                  <td className="px-4 py-3">{w.enabled ? (<span className="inline-flex items-center gap-1 text-xs text-success"><ToggleRight className="w-4 h-4" /> Active</span>) : (<span className="inline-flex items-center gap-1 text-xs text-text-muted"><ToggleLeft className="w-4 h-4" /> Disabled</span>)}</td>
                  <td className="px-4 py-3 text-text-secondary text-xs">{formatDate(w.created_at)}</td>
                  <td className="px-4 py-3">
                    <button
                      onClick={() => handleDelete(w.id)}
                      className="p-1.5 rounded-md text-text-muted hover:text-error hover:bg-error/10 transition-colors"
                      title="Delete webhook"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <Modal isOpen={showModal} onClose={() => setShowModal(false)} title="Add Webhook" size="md">
        <div className="space-y-4">
          <div>
            <label className="text-text-secondary text-sm font-medium">Name</label>
            <input type="text" value={form.name} onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))} className={inputCls} placeholder="my-webhook" autoFocus />
          </div>
          <div>
            <label className="text-text-secondary text-sm font-medium">Callback URL</label>
            <input type="text" value={form.url} onChange={(e) => setForm((f) => ({ ...f, url: e.target.value }))} className={inputCls} placeholder="https://your-app.com/api/webhook" />
          </div>
          <div>
            <label className="text-text-secondary text-sm font-medium">Events <span className="text-text-muted text-xs">(comma-separated, empty = all)</span></label>
            <input type="text" value={form.events} onChange={(e) => setForm((f) => ({ ...f, events: e.target.value }))} className={inputCls} placeholder="insert,update,delete" />
          </div>
          <div>
            <label className="text-text-secondary text-sm font-medium">Enabled</label>
            <select value={form.enabled} onChange={(e) => setForm((f) => ({ ...f, enabled: e.target.value }))} className={inputCls}>
              <option value="true">Yes</option>
              <option value="false">No (paused)</option>
            </select>
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <button onClick={() => setShowModal(false)} className="px-4 py-2 rounded-lg text-text-secondary text-sm hover:bg-bg-input transition-colors">Cancel</button>
            <button onClick={save} disabled={saving} className="px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover disabled:opacity-50 transition-colors">
              {saving ? 'Creating...' : 'Create Webhook'}
            </button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
