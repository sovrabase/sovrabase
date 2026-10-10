import { useCallback, useEffect, useState } from 'react';
import { api, adminProjectPath } from '../../../api';
import type { Project } from '../../../types';

/** Full answer of GET /admin/projects/{id}: the project plus connection details. */
export interface ProjectDetail {
  project: Project & { allow_origins?: string; storage_quota?: number; owner_id?: string };
  api_key?: string;
  api_url?: string;
  allow_origins?: string;
  storage_quota?: number;
}

export function useProjectDetail(projectId: string) {
  const [detail, setDetail] = useState<ProjectDetail | null>(null);
  const [error, setError] = useState<string | null>(null);
  const load = useCallback(async () => {
    try {
      const d = await api<ProjectDetail>(adminProjectPath(projectId, ''));
      setDetail(d);
      setError(null);
    } catch (e) {
      setError((e as Error).message);
    }
  }, [projectId]);
  useEffect(() => {
    load();
  }, [load]);
  return { detail, error, reload: load };
}

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

/** Live usage counters, refreshed every few seconds while the page is open. */
export function useProjectUsage(projectId: string, pollMs = 3000) {
  const [usage, setUsage] = useState<ProjectUsage | null>(null);
  const refresh = useCallback(
    () =>
      api<ProjectUsage>(adminProjectPath(projectId, '/usage'))
        .then(setUsage)
        .catch(() => setUsage(null)),
    [projectId],
  );
  useEffect(() => {
    refresh();
    if (!pollMs) return;
    const t = setInterval(() => {
      if (document.visibilityState === 'visible') refresh();
    }, pollMs);
    return () => clearInterval(t);
  }, [refresh, pollMs]);
  return { usage, refresh };
}

/** Base URL apps call. The server reports host only, so the console origin gives the scheme. */
export function apiBase(): string {
  return typeof window !== 'undefined' ? window.location.origin : '';
}

export function timeAgo(iso?: string): string {
  if (!iso) return '';
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return iso;
  const s = Math.round((Date.now() - t) / 1000);
  if (s < 45) return 'just now';
  if (s < 3600) return `${Math.round(s / 60)} min ago`;
  if (s < 86400) return `${Math.round(s / 3600)} h ago`;
  if (s < 86400 * 7) return `${Math.round(s / 86400)} d ago`;
  return new Date(t).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' });
}

export function mask(key: string): string {
  if (key.length <= 12) return '•'.repeat(key.length);
  return `${key.slice(0, 4)}${'•'.repeat(8)}${key.slice(-4)}`;
}
