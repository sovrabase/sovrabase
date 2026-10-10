import { api, adminProjectPath, ApiError } from '../../../api';

export interface McpTool {
  name: string;
  description: string;
  scope: string;
  write: boolean;
  enabled: boolean;
}

export interface McpConfig {
  enabled: boolean;
  endpoint: string;
  disabled_tools: string[];
  tools: McpTool[];
}

export interface McpToken {
  id: string;
  name: string;
  prefix: string;
  scopes: string[];
  created_at: string;
  last_used_at: string | null;
  /** Only present in the response that created it. */
  token?: string;
}

export interface McpCall {
  at: string;
  token_name: string;
  tool: string;
  ok: boolean;
  duration_ms: number;
  error: string;
}

export const SCOPES = [
  { value: 'read', label: 'read', description: 'List collections, query records, list buckets and files' },
  { value: 'write', label: 'write', description: 'Insert, update and delete records' },
  { value: 'schema', label: 'schema', description: 'Create collections and indexes' },
  { value: 'functions', label: 'functions', description: 'List and invoke deployed functions' },
  { value: 'logs', label: 'logs', description: 'Read recent requests and errors' },
];

export const SCOPE_TEXT: Record<string, string> = {
  read: 'Read data',
  write: 'Change records',
  schema: 'Change the schema',
  functions: 'Functions',
  logs: 'Logs',
};

const p = (projectId: string, s: string) => adminProjectPath(projectId, `/mcp${s}`);

export const getConfig = (id: string) => api<McpConfig>(p(id, ''));
export const putConfig = (id: string, body: Partial<Pick<McpConfig, 'enabled' | 'disabled_tools'>>) =>
  api<McpConfig>(p(id, ''), { method: 'PUT', body: JSON.stringify(body) });
export const listTokens = (id: string) => api<McpToken[]>(p(id, '/tokens'));
export const createToken = (id: string, name: string, scopes: string[]) =>
  api<McpToken>(p(id, '/tokens'), { method: 'POST', body: JSON.stringify({ name, scopes }) });
export const listCalls = (id: string, limit = 50) => api<McpCall[]>(p(id, `/calls?limit=${limit}`));

/** DELETE answers 204 with no body; api() then fails to parse JSON, which still means success. */
export async function revokeToken(id: string, tokenId: string) {
  try {
    await api(p(id, `/tokens/${encodeURIComponent(tokenId)}`), { method: 'DELETE' });
  } catch (e) {
    if (e instanceof ApiError) throw e;
  }
}

export const isNotFound = (e: unknown) => e instanceof ApiError && e.status === 404;

/* ---------- Client snippets ---------- */

export type ClientKey = 'claude-code' | 'claude-desktop' | 'cursor' | 'json';

export const CLIENTS: Array<{ value: ClientKey; label: string; where: string }> = [
  { value: 'claude-code', label: 'Claude Code', where: 'Run this in a terminal.' },
  { value: 'claude-desktop', label: 'Claude Desktop', where: 'Add to claude_desktop_config.json. Desktop starts local servers, so mcp-remote bridges to the HTTP endpoint.' },
  { value: 'cursor', label: 'Cursor', where: 'Add to ~/.cursor/mcp.json, or .cursor/mcp.json in a repository.' },
  { value: 'json', label: 'Other JSON', where: 'For any client that supports remote HTTP servers.' },
];

export function snippet(client: ClientKey, url: string, token: string): string {
  const auth = `Authorization: Bearer ${token}`;
  switch (client) {
    case 'claude-code':
      return `claude mcp add --transport http sovrabase ${url} \\\n  --header "${auth}"`;
    case 'claude-desktop':
      return JSON.stringify(
        { mcpServers: { sovrabase: { command: 'npx', args: ['-y', 'mcp-remote', url, '--header', auth] } } },
        null,
        2,
      );
    case 'cursor':
      return JSON.stringify({ mcpServers: { sovrabase: { url, headers: { Authorization: `Bearer ${token}` } } } }, null, 2);
    default:
      return JSON.stringify(
        { mcpServers: { sovrabase: { type: 'http', url, headers: { Authorization: `Bearer ${token}` } } } },
        null,
        2,
      );
  }
}

export function ago(iso: string | null | undefined): string {
  if (!iso) return 'Never used';
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return iso;
  const s = Math.max(0, Math.round((Date.now() - t) / 1000));
  if (s < 60) return 'Just now';
  if (s < 3600) return `${Math.round(s / 60)} min ago`;
  if (s < 86400) return `${Math.round(s / 3600)} h ago`;
  return new Date(iso).toLocaleDateString(undefined, { day: 'numeric', month: 'short' });
}

export function clock(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  const today = new Date().toDateString() === d.toDateString();
  return today
    ? d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' })
    : d.toLocaleString(undefined, { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
}
