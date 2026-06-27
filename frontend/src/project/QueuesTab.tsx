import { useState, useEffect } from 'react';
import { Layers, Loader2, RefreshCw, Trash2, Unlock } from 'lucide-react';
import { api } from '../api';
import { useToast } from '../components/Toast';

interface QueueInfo { name: string; visible: number; in_flight: number; total: number; }
interface Props { projectId: string; }

export default function QueuesTab({ projectId }: Props) {
  const [queues, setQueues] = useState<QueueInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const { showToast } = useToast();

  const load = () => {
    setLoading(true);
    api<{ data: QueueInfo[]; count: number }>(`/admin/projects/${encodeURIComponent(projectId)}/queues`)
      .then((d) => setQueues(d.data || []))
      .catch(() => setQueues([]))
      .finally(() => setLoading(false));
  };

  useEffect(() => { load(); }, [projectId]);

  const unstick = async (queueName: string) => {
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/queues/${encodeURIComponent(queueName)}/make-visible`, { method: 'POST' });
      showToast(`Queue "${queueName}" unstuck`, 'success');
      load();
    } catch (err) { showToast((err as Error).message, 'error'); }
  };

  const purgeQueue = async (queueName: string) => {
    if (!confirm(`Purge all messages from "${queueName}"?`)) return;
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/queues/purge`, { method: 'POST' });
      showToast(`Queue "${queueName}" purged`, 'success');
      load();
    } catch (err) { showToast((err as Error).message, 'error'); }
  };

  if (loading) {
    return <div className="flex items-center gap-2 py-10 text-text-muted"><Loader2 className="w-4 h-4 animate-spin" /> Loading queues...</div>;
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-text-primary">Queues ({queues.length})</h2>
        <button onClick={load} className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-bg-input border border-border text-text-secondary text-xs hover:text-text-primary transition-colors">
          <RefreshCw className="w-3.5 h-3.5" /> Refresh
        </button>
      </div>

      {queues.length === 0 ? (
        <div className="flex flex-col items-center py-16 text-text-muted gap-3">
          <Layers className="w-10 h-10" />
          <p>No queues yet</p>
        </div>
      ) : (
        <div className="border border-border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-bg-input text-text-muted text-xs uppercase">
              <tr>
                <th className="text-left px-4 py-3 font-medium">Queue Name</th>
                <th className="text-right px-4 py-3 font-medium">Visible</th>
                <th className="text-right px-4 py-3 font-medium">In Flight</th>
                <th className="text-right px-4 py-3 font-medium">Total</th>
                <th className="text-right px-4 py-3 font-medium w-32">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {queues.map((q) => (
                <tr key={q.name} className="hover:bg-bg-input/50 transition-colors">
                  <td className="px-4 py-3 font-mono text-text-primary text-sm font-semibold">{q.name}</td>
                  <td className="px-4 py-3 text-right text-success text-sm">{q.visible.toLocaleString()}</td>
                  <td className="px-4 py-3 text-right text-yellow-500 text-sm">{q.in_flight.toLocaleString()}</td>
                  <td className="px-4 py-3 text-right text-text-primary text-sm font-medium">{q.total.toLocaleString()}</td>
                  <td className="px-4 py-3 text-right">
                    <div className="flex items-center justify-end gap-1">
                      <button onClick={() => unstick(q.name)} className="p-1.5 rounded text-yellow-500 hover:bg-yellow-500/10" title="Make visible">
                        <Unlock className="w-3.5 h-3.5" />
                      </button>
                      <button onClick={() => purgeQueue(q.name)} className="p-1.5 rounded text-danger hover:bg-danger/10" title="Purge">
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
