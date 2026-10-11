import { useState, useEffect, useCallback } from 'react';
import { api, adminProjectPath } from '../api';

export interface ProjectUsage {
  enabled?: boolean;
  api_requests?: number;
  db_reads_total?: number;
  db_writes_total?: number;
  bandwidth_up?: number;
  bandwidth_down?: number;
  total_storage_bytes?: number;
  database_bytes?: number;
  file_storage_bytes?: number;
  realtime_connections?: number;
  requests_by_method?: Record<string, number>;
}

const USAGE_POLL_MS = 3000;

export function useProjectUsage(projectId: string, poll = true) {
  const [usage, setUsage] = useState<ProjectUsage | null>(null);

  const refresh = useCallback(() => {
    return api<ProjectUsage>(adminProjectPath(projectId, '/usage'))
      .then(setUsage)
      .catch(() => setUsage(null));
  }, [projectId]);

  useEffect(() => {
    refresh();
    if (!poll) return;
    const timer = setInterval(refresh, USAGE_POLL_MS);
    return () => clearInterval(timer);
  }, [refresh, poll]);

  return { usage, refresh };
}
