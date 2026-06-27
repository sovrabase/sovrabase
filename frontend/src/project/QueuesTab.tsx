import { useState, useEffect } from 'react';
import { Layers, Loader2, RefreshCw, Trash2, Unlock } from 'lucide-react';
import { api, formatDate } from '../api';
import { useToast } from '../components/Toast';
import type { QueueMessage } from '../types';

interface Props { projectId: string; apiKey?: string; }

const STATUS_COLORS: Record<string, string> = {
  pending: 'bg-yellow-500/10 text-yellow-400 border-yellow-500/20',
  processing: 'bg-blue-500/10 text-blue-400 border-blue-500/20',
  completed: 'bg-success/10 text-success border-success/20',
  failed: 'bg-danger/10 text-danger border-danger/20',
};

export default function QueuesTab({ projectId }: Props) {
  const [messages, setMessages] = useState<QueueMessage[]>([]);
  const [loading, setLoading] = useState(true);
  const { showToast } = useToast();

  const load = () => {
    setLoading(true);
    api<{ data: QueueMessage[] }>(`/admin/projects/${encodeURIComponent(projectId)}/queues`)
      .then((res) => setMessages(res.data || []))
      .finally(() => setLoading(false));
  };

  useEffect(() => { load(); }, [projectId]);

  const unstickAll = async () => {
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/queues/unstick`, { method: 'POST' });
      showToast('All in-flight messages returned to visible', 'success');
      load();
    } catch (err) { showToast((err as Error).message, 'error'); }
  };

  const purgeQueue = async () => {
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/queues/purge`, { method: 'POST' });
      showToast('Queue purged', 'success');
      load();
    } catch (err) { showToast((err as Error).message, 'error'); }
  };

  const formatPayload = (payload: unknown) => {
    const str = typeof payload === 'object' ? JSON.stringify(payload) : String(payload ?? '');
    return str.length > 60 ? str.slice(0, 57) + '...' : str;
  };

  const statusClass = (status: string) => STATUS_COLORS[status] || 'bg-bg-input border-border text-text-muted';

  if (loading) {
    return <div className="flex items-center gap-2 py-10 text-text-muted"><Loader2 className="w-4 h-4 animate-spin" /> Loading queues...</div>;
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-text-primary">Queue Messages ({messages.length})</h2>
        <div className="flex gap-2">
          <button onClick={load} className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-bg-input border border-border text-text-secondary text-xs hover:text-text-primary transition-colors">
            <RefreshCw className="w-3.5 h-3.5" /> Refresh
          </button>
          <button onClick={unstickAll} className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-yellow-500/10 border border-yellow-500/20 text-yellow-400 text-xs hover:bg-yellow-500/20 transition-colors" title="Force all in-flight messages back to visible">
            <Unlock className="w-3.5 h-3.5" /> Unstick
          </button>
          <button onClick={purgeQueue} className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-danger/10 border border-danger/20 text-danger text-xs hover:bg-danger/20 transition-colors" title="Delete all queue messages">
            <Trash2 className="w-3.5 h-3.5" /> Purge
          </button>
        </div>
      </div>

      {messages.length === 0 ? (
        <div className="flex flex-col items-center py-16 text-text-muted gap-3"><Layers className="w-10 h-10" /><p>No queue messages</p></div>
      ) : (
        <div className="border border-border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-bg-input text-text-muted text-xs uppercase">
              <tr>
                <th className="text-left px-4 py-3 font-medium">ID</th>
                <th className="text-left px-4 py-3 font-medium">Queue</th>
                <th className="text-left px-4 py-3 font-medium">Payload</th>
                <th className="text-left px-4 py-3 font-medium">Status</th>
                <th className="text-left px-4 py-3 font-medium">Created</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {messages.map((m) => (
                <tr key={m.id} className="hover:bg-bg-input/50 transition-colors">
                  <td className="px-4 py-3 font-mono text-text-primary text-xs">{m.id.slice(0, 12)}...</td>
                  <td className="px-4 py-3 font-medium text-text-primary">{m.queue}</td>
                  <td className="px-4 py-3 text-text-secondary text-xs font-mono" title={typeof m.payload === 'object' ? JSON.stringify(m.payload) : String(m.payload)}>{formatPayload(m.payload)}</td>
                  <td className="px-4 py-3"><span className={`inline-flex px-2 py-0.5 border rounded-full text-xs font-medium capitalize ${statusClass(m.status)}`}>{m.status}</span></td>
                  <td className="px-4 py-3 text-text-secondary text-xs">{formatDate(m.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
