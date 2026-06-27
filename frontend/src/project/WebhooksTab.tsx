import { useState, useEffect } from 'react';
import { Webhook, ToggleLeft, ToggleRight, Loader2 } from 'lucide-react';
import { api, formatDate } from '../api';
import type { Webhook as WebhookType } from '../types';

interface Props {
  projectId: string;
  apiKey?: string;
}

export default function WebhooksTab({ projectId }: Props) {
  const [webhooks, setWebhooks] = useState<WebhookType[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    api<WebhookType[]>(`/admin/projects/${encodeURIComponent(projectId)}/webhooks`)
      .then(setWebhooks)
      .finally(() => setLoading(false));
  }, [projectId]);

  const truncateUrl = (url: string) => url.length > 50 ? url.slice(0, 47) + '...' : url;

  if (loading) {
    return <div className="flex items-center gap-2 py-10 text-text-muted"><Loader2 className="w-4 h-4 animate-spin" /> Loading webhooks...</div>;
  }

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold text-text-primary">Webhooks ({webhooks.length})</h2>

      {webhooks.length === 0 ? (
        <div className="flex flex-col items-center py-16 text-text-muted gap-3">
          <Webhook className="w-10 h-10" />
          <p>No webhooks</p>
        </div>
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
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {webhooks.map((w) => (
                <tr key={w.id} className="hover:bg-bg-input/50 transition-colors">
                  <td className="px-4 py-3 font-medium text-text-primary">{w.name}</td>
                  <td className="px-4 py-3 text-text-secondary font-mono text-xs" title={w.url}>{truncateUrl(w.url)}</td>
                  <td className="px-4 py-3">
                    <div className="flex gap-1 flex-wrap">
                      {w.events.map((ev, i) => (
                        <span key={i} className="inline-flex px-2 py-0.5 bg-accent/10 text-accent rounded-full text-xs font-mono">{ev}</span>
                      ))}
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    {w.enabled ? (
                      <span className="inline-flex items-center gap-1 text-xs text-success"><ToggleRight className="w-4 h-4" /> Active</span>
                    ) : (
                      <span className="inline-flex items-center gap-1 text-xs text-text-muted"><ToggleLeft className="w-4 h-4" /> Disabled</span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-text-secondary text-xs">{formatDate(w.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
