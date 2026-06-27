import { useState, useEffect } from 'react';
import { Layers, Loader2 } from 'lucide-react';
import { api, formatDate } from '../api';
import type { QueueMessage } from '../types';

interface Props {
  projectId: string;
  apiKey?: string;
}

const STATUS_COLORS: Record<string, string> = {
  pending: 'bg-yellow-500/10 text-yellow-400 border-yellow-500/20',
  processing: 'bg-blue-500/10 text-blue-400 border-blue-500/20',
  completed: 'bg-success/10 text-success border-success/20',
  failed: 'bg-danger/10 text-danger border-danger/20',
};

export default function QueuesTab({ projectId }: Props) {
  const [messages, setMessages] = useState<QueueMessage[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    api<QueueMessage[]>(`/admin/projects/${encodeURIComponent(projectId)}/queues`)
      .then(setMessages)
      .finally(() => setLoading(false));
  }, [projectId]);

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
      <h2 className="text-lg font-semibold text-text-primary">Queue Messages ({messages.length})</h2>

      {messages.length === 0 ? (
        <div className="flex flex-col items-center py-16 text-text-muted gap-3">
          <Layers className="w-10 h-10" />
          <p>No queue messages</p>
        </div>
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
                  <td className="px-4 py-3 text-text-secondary text-xs font-mono" title={typeof m.payload === 'object' ? JSON.stringify(m.payload) : String(m.payload)}>
                    {formatPayload(m.payload)}
                  </td>
                  <td className="px-4 py-3">
                    <span className={`inline-flex px-2 py-0.5 border rounded-full text-xs font-medium capitalize ${statusClass(m.status)}`}>
                      {m.status}
                    </span>
                  </td>
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
