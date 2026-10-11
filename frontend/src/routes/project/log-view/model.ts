/** One line of the project's request log, as GET /admin/projects/{id}/logs returns it. */
export interface RequestLog {
  timestamp: string;
  method?: string;
  path?: string;
  status?: number;
  duration?: string;
  ip?: string;
  email?: string;
  collection?: string;
  doc_id?: string;
  /** Present when the server tags the entry (e.g. "mcp"); otherwise derived from the path. */
  source?: string;
  level?: string;
  message?: string;
  [k: string]: unknown;
}

export type Level = 'info' | 'warn' | 'error';
export type Source = 'api' | 'database' | 'auth' | 'storage' | 'functions' | 'admin' | 'mcp';

export const SOURCES: Array<{ value: Source; label: string }> = [
  { value: 'database', label: 'Database' },
  { value: 'auth', label: 'Auth' },
  { value: 'storage', label: 'Storage' },
  { value: 'functions', label: 'Functions' },
  { value: 'mcp', label: 'MCP' },
  { value: 'admin', label: 'Console' },
  { value: 'api', label: 'Other API' },
];

export function levelOf(e: RequestLog): Level {
  const l = (e.level ?? '').toLowerCase();
  if (l.startsWith('err') || l === 'fatal') return 'error';
  if (l.startsWith('warn')) return 'warn';
  if (l) return 'info';
  const s = e.status ?? 200;
  if (s >= 500) return 'error';
  if (s >= 400) return 'warn';
  return 'info';
}

export function sourceOf(e: RequestLog): Source {
  const src = (e.source ?? '').toLowerCase();
  if (src) {
    if (src === 'collections' || src === 'db') return 'database';
    if ((SOURCES.map((s) => s.value) as string[]).includes(src)) return src as Source;
  }
  const p = e.path ?? '';
  if (p === '/mcp' || p.startsWith('/mcp/') || p.includes('/mcp')) return 'mcp';
  if (p.startsWith('/admin/')) return 'admin';
  if (p.startsWith('/auth/') || p.startsWith('/api/v1/auth')) return 'auth';
  if (p.startsWith('/api/v1/collections') || p.startsWith('/api/v1/batch') || e.collection) return 'database';
  if (p.startsWith('/api/v1/storage')) return 'storage';
  if (p.startsWith('/api/v1/functions')) return 'functions';
  return 'api';
}

export function sourceLabel(s: Source) {
  return SOURCES.find((x) => x.value === s)?.label ?? s;
}

/** "21:44:02.118" in local time. */
export function clock(iso: string) {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  const pad = (n: number, w = 2) => String(n).padStart(w, '0');
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}.${pad(d.getMilliseconds(), 3)}`;
}

export function dayOf(iso: string) {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? '' : d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

export function summary(e: RequestLog) {
  if (e.message) return e.message;
  return `${e.method ?? ''} ${e.path ?? ''}`.trim();
}
