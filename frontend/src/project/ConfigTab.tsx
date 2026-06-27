import { useState, useEffect } from 'react';
import { Plus, Trash2, Globe, Lock, Loader2, Settings } from 'lucide-react';
import { api } from '../api';
import type { ConfigEntry } from '../types';
import Modal from '../components/Modal';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { useToast } from '../components/Toast';

interface Props {
  projectId: string;
  apiKey?: string;
}

const TYPES = ['string', 'number', 'boolean', 'json'] as const;

export default function ConfigTab({ projectId }: Props) {
  const { showToast } = useToast();
  const [entries, setEntries] = useState<ConfigEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [addOpen, setAddOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const loadEntries = () => {
    setLoading(true);
    api<ConfigEntry[]>(`/admin/projects/${encodeURIComponent(projectId)}/config`)
      .then(setEntries)
      .finally(() => setLoading(false));
  };

  useEffect(() => { loadEntries(); }, [projectId]);

  const handleAdd = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const form = e.currentTarget;
    const key = (form.elements.namedItem('key') as HTMLInputElement).value.trim();
    const value = (form.elements.namedItem('value') as HTMLInputElement).value.trim();
    const type = (form.elements.namedItem('type') as HTMLSelectElement).value;
    const pub = (form.elements.namedItem('public') as HTMLInputElement).checked;
    if (!key) return;
    setSubmitting(true);
    try {
      let parsedValue: unknown = value;
      if (type === 'number') parsedValue = Number(value);
      else if (type === 'boolean') parsedValue = value.toLowerCase() === 'true';
      else if (type === 'json') parsedValue = JSON.parse(value);

      await api(`/admin/projects/${encodeURIComponent(projectId)}/config`, {
        method: 'POST',
        body: JSON.stringify({ key, value: parsedValue, type, public: pub }),
      });
      showToast('Config entry added', 'success');
      setAddOpen(false);
      loadEntries();
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setSubmitting(true);
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/config/${encodeURIComponent(deleteTarget)}`, { method: 'DELETE' });
      setEntries((prev) => prev.filter((e) => e.key !== deleteTarget));
      showToast('Config entry deleted', 'success');
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setSubmitting(false);
      setDeleteTarget(null);
    }
  };

  const formatValue = (entry: ConfigEntry) => {
    const str = typeof entry.value === 'object' ? JSON.stringify(entry.value) : String(entry.value);
    return str.length > 40 ? str.slice(0, 40) + '...' : str;
  };

  if (loading) {
    return <div className="flex items-center gap-2 py-10 text-text-muted"><Loader2 className="w-4 h-4 animate-spin" /> Loading config...</div>;
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-text-primary">Config Entries ({entries.length})</h2>
        <button onClick={() => setAddOpen(true)} className="flex items-center gap-2 px-4 py-2 bg-accent text-white rounded-md hover:bg-accent-hover transition-colors text-sm font-medium">
          <Plus className="w-4 h-4" /> Add Entry
        </button>
      </div>

      {entries.length === 0 ? (
        <div className="flex flex-col items-center py-16 text-text-muted gap-3">
          <Settings className="w-10 h-10" />
          <p>No config entries yet</p>
        </div>
      ) : (
        <div className="border border-border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-bg-input text-text-muted text-xs uppercase">
              <tr>
                <th className="text-left px-4 py-3 font-medium">Key</th>
                <th className="text-left px-4 py-3 font-medium">Value</th>
                <th className="text-left px-4 py-3 font-medium">Type</th>
                <th className="text-left px-4 py-3 font-medium">Visibility</th>
                <th className="text-right px-4 py-3 font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {entries.map((e) => (
                <tr key={e.key} className="hover:bg-bg-input/50 transition-colors">
                  <td className="px-4 py-3 font-mono text-text-primary text-xs">{e.key}</td>
                  <td className="px-4 py-3 text-text-secondary text-xs" title={typeof e.value === 'object' ? JSON.stringify(e.value) : String(e.value)}>
                    {formatValue(e)}
                  </td>
                  <td className="px-4 py-3">
                    <span className="inline-flex px-2 py-0.5 bg-bg-input border border-border rounded text-xs text-text-secondary font-mono">{e.type || 'string'}</span>
                  </td>
                  <td className="px-4 py-3">
                    {e.public ? (
                      <span className="inline-flex items-center gap-1 text-xs text-success"><Globe className="w-3 h-3" /> Public</span>
                    ) : (
                      <span className="inline-flex items-center gap-1 text-xs text-text-muted"><Lock className="w-3 h-3" /> Private</span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <button onClick={() => setDeleteTarget(e.key)} className="p-1.5 rounded hover:bg-danger/10 text-text-secondary hover:text-danger transition-colors" title="Delete">
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Add Modal */}
      <Modal open={addOpen} onClose={() => setAddOpen(false)} title="Add Config Entry">
        <form onSubmit={handleAdd} className="space-y-4">
          <div>
            <label className="block text-sm text-text-secondary mb-1">Key</label>
            <input name="key" required placeholder="my.config.key" className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm placeholder:text-text-muted focus:outline-none focus:border-accent" />
          </div>
          <div>
            <label className="block text-sm text-text-secondary mb-1">Value</label>
            <input name="value" required placeholder="config value" className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm placeholder:text-text-muted focus:outline-none focus:border-accent" />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm text-text-secondary mb-1">Type</label>
              <select name="type" defaultValue="string" className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent">
                {TYPES.map((t) => (<option key={t} value={t}>{t}</option>))}
              </select>
            </div>
            <div className="flex items-end pb-1">
              <label className="flex items-center gap-2 cursor-pointer">
                <input type="checkbox" name="public" className="accent-accent w-4 h-4" defaultChecked />
                <span className="text-sm text-text-secondary">Public</span>
              </label>
            </div>
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <button type="button" onClick={() => setAddOpen(false)} className="px-4 py-2 text-sm text-text-secondary hover:text-text-primary transition-colors">Cancel</button>
            <button type="submit" disabled={submitting} className="px-4 py-2 bg-accent text-white rounded-md text-sm font-medium hover:bg-accent-hover transition-colors disabled:opacity-50">
              {submitting ? 'Adding...' : 'Add'}
            </button>
          </div>
        </form>
      </Modal>

      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDelete}
        title="Delete Config Entry"
        message={`Delete config key "${deleteTarget}"? This cannot be undone.`}
        loading={submitting}
      />
    </div>
  );
}
