import { api, adminProjectPath } from '../../../api';
import type { CronJob, CronJobExecution, ServerlessFunction } from '../../../types';

/* ---------- Shapes the backend returns ---------- */

export interface WebhookRow {
  id: string;
  url?: string;
  /** Stored as "insert,update" by the API; older rows may be an array. Empty = every event. */
  events?: string | string[] | null;
  enabled: boolean;
  created_at?: string;
}

export interface QueueInfo {
  name: string;
  visible: number;
  in_flight: number;
  total: number;
}

export interface QueueMessage {
  id: string;
  queue_name?: string;
  body?: unknown;
  created_at?: string;
  visible_at?: string;
  receive_count?: number;
}

export interface IntegrationDef {
  id: string;
  name: string;
  description?: string;
  category?: string;
  supports_triggers?: boolean;
}

export interface ProjectIntegration {
  id: string;
  config?: Record<string, unknown>;
  events?: string[];
  collections?: string[];
}

/* ---------- The unified list ---------- */

export type TriggerKind = 'schedule' | 'record' | 'incoming' | 'queue' | 'plugin';

export const TRIGGER_LABEL: Record<TriggerKind, string> = {
  schedule: 'Schedule',
  record: 'Record event',
  incoming: 'Incoming webhook',
  queue: 'Queue',
  plugin: 'Plugin',
};

export interface AutomationItem {
  key: string;
  kind: TriggerKind;
  name: string;
  trigger: string;
  action: string;
  status: 'on' | 'paused' | 'info';
  /** Where the row opens. */
  to: string;
  lastRun?: { at?: string; ok?: boolean; text?: string };
  /** Editable here (cron, webhook) or read-only (plugin rows). */
  editable: boolean;
}

/** Providers whose dashboards call back into Sovrabase. */
export const INCOMING_PROVIDERS = new Set(['stripe', 'paypal']);

export const RECORD_EVENTS = [
  { value: 'insert', label: 'Created' },
  { value: 'update', label: 'Updated' },
  { value: 'delete', label: 'Deleted' },
];

const PLUGIN_EVENT_LABEL: Record<string, string> = {
  'record:create': 'record created',
  'record:update': 'record updated',
  'record:delete': 'record deleted',
  'auth:signup': 'user signed up',
  'auth:signin': 'user signed in',
};

export function pluginEventLabel(e: string) {
  return PLUGIN_EVENT_LABEL[e] ?? e;
}

export function webhookEvents(w: WebhookRow): string[] {
  if (!w.events) return [];
  if (Array.isArray(w.events)) return w.events.filter(Boolean);
  return w.events
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean);
}

export function cronUrl(j: CronJob) {
  return j.url || j.endpoint || '';
}

export function shortUrl(u: string) {
  try {
    const x = new URL(u);
    return x.host + (x.pathname === '/' ? '' : x.pathname);
  } catch {
    return u;
  }
}

/** Public API origin, same rule the old Cron tab used. */
export function publicOrigin() {
  return window.location.origin.replace(/:\d+$/, ':6070');
}

/* ---------- Fetchers (the same endpoints the old tabs called) ---------- */

const enc = encodeURIComponent;

export const automationsApi = {
  listCron: (pid: string) => api<{ data: CronJob[] }>(adminProjectPath(pid, '/cron')).then((r) => r.data || []),
  cronExecutions: (pid: string, jobId: string) =>
    api<{ data: CronJobExecution[] }>(adminProjectPath(pid, `/cron/${enc(jobId)}/executions`)).then((r) => r.data || []),
  createCron: (pid: string, body: Record<string, unknown>) =>
    api<CronJob>(adminProjectPath(pid, '/cron'), { method: 'POST', body: JSON.stringify(body) }),
  updateCron: (pid: string, id: string, body: Record<string, unknown>) =>
    api<CronJob>(adminProjectPath(pid, `/cron/${enc(id)}`), { method: 'PUT', body: JSON.stringify({ ...body, id }) }),
  deleteCron: (pid: string, id: string) => api(adminProjectPath(pid, `/cron/${enc(id)}`), { method: 'DELETE' }),
  listFunctions: (pid: string) =>
    api<{ data: ServerlessFunction[] }>(adminProjectPath(pid, '/functions'))
      .then((r) => (r.data || []).filter((f) => f.trigger === 'http'))
      .catch(() => [] as ServerlessFunction[]),

  listWebhooks: (pid: string) => api<{ data: WebhookRow[] }>(adminProjectPath(pid, '/webhooks')).then((r) => r.data || []),
  createWebhook: (pid: string, body: { url: string; events: string; enabled: boolean }) =>
    api<WebhookRow>(adminProjectPath(pid, '/webhooks'), { method: 'POST', body: JSON.stringify(body) }),
  updateWebhook: (pid: string, id: string, body: Partial<{ url: string; events: string; enabled: boolean }>) =>
    api<WebhookRow>(adminProjectPath(pid, `/webhooks/${enc(id)}`), { method: 'PUT', body: JSON.stringify(body) }),
  deleteWebhook: (pid: string, id: string) => api(adminProjectPath(pid, `/webhooks/${enc(id)}`), { method: 'DELETE' }),

  listQueues: (pid: string) =>
    api<{ data: QueueInfo[]; count: number }>(adminProjectPath(pid, '/queues'))
      .then((r) => r.data || [])
      .catch(() => [] as QueueInfo[]),
  makeVisible: (pid: string, q: string) => api(adminProjectPath(pid, `/queues/${enc(q)}/make-visible`), { method: 'POST' }),
  purgeQueue: (pid: string, q: string) =>
    api(adminProjectPath(pid, '/queues/purge'), { method: 'POST', body: JSON.stringify({ queue: q }) }),

  integrations: async (pid: string) => {
    const [cat, proj] = await Promise.all([
      api<{ integrations: IntegrationDef[] }>('/admin/integrations/catalog').catch(() => ({ integrations: [] as IntegrationDef[] })),
      api<{ integrations: ProjectIntegration[] }>(adminProjectPath(pid, '/integrations')).catch(() => ({
        integrations: [] as ProjectIntegration[],
      })),
    ]);
    return { catalog: cat.integrations || [], enabled: proj.integrations || [] };
  },
};

/** Public queue API (send / receive / delete) — needs the project key, like an app would. */
export async function queuePublic<T>(apiKey: string, op: 'send' | 'receive' | 'delete', body: Record<string, unknown>): Promise<T> {
  const res = await fetch(`${publicOrigin()}/api/v1/queues/${op}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-Project-Key': apiKey },
    body: JSON.stringify(body),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error((data as { error?: string }).error || `HTTP ${res.status}`);
  return data as T;
}

export function timeAgo(iso?: string) {
  if (!iso) return '';
  const t = new Date(iso).getTime();
  if (!t || Number.isNaN(t)) return '';
  const s = Math.round((Date.now() - t) / 1000);
  if (s < 60) return 'just now';
  if (s < 3600) return `${Math.floor(s / 60)} min ago`;
  if (s < 86400) return `${Math.floor(s / 3600)} h ago`;
  return `${Math.floor(s / 86400)} d ago`;
}
