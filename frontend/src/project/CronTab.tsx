import { useState, useEffect } from 'react';
import { Clock, ToggleLeft, ToggleRight, Loader2 } from 'lucide-react';
import { api, formatDate } from '../api';
import type { CronJob } from '../types';

interface Props {
  projectId: string;
  apiKey?: string;
}

export default function CronTab({ projectId }: Props) {
  const [jobs, setJobs] = useState<CronJob[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    api<CronJob[]>(`/admin/projects/${encodeURIComponent(projectId)}/cron`)
      .then(setJobs)
      .finally(() => setLoading(false));
  }, [projectId]);

  if (loading) {
    return <div className="flex items-center gap-2 py-10 text-text-muted"><Loader2 className="w-4 h-4 animate-spin" /> Loading cron jobs...</div>;
  }

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold text-text-primary">Cron Jobs ({jobs.length})</h2>

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
                  <td className="px-4 py-3">
                    <span className="inline-flex px-2 py-0.5 bg-bg-input border border-border rounded text-xs font-mono text-accent">{job.schedule}</span>
                  </td>
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
    </div>
  );
}
