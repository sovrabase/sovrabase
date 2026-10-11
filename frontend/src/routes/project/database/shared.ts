import { adminProjectPath } from '../../../api';
import type { DatabaseDocument } from '../../../types';

export type Doc = DatabaseDocument;

/** Keys the server manages: shown, never edited, stripped before writes. */
export const PROTECTED_KEYS = new Set(['_id', '_createdAt', '_updatedAt']);

/** Collections the server refuses to clear or drop. */
export function isReserved(name: string) {
  return name.startsWith('_');
}

export function colPath(projectId: string, collection: string, suffix = '') {
  return adminProjectPath(projectId, `/collections/${encodeURIComponent(collection)}${suffix}`);
}

export function stripProtected(d: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(d)) if (!PROTECTED_KEYS.has(k)) out[k] = v;
  return out;
}

/** Short one-line rendering of any value for table cells. */
export function preview(v: unknown): string {
  if (v === null || v === undefined) return '';
  if (typeof v === 'string') return v;
  if (typeof v === 'number' || typeof v === 'boolean') return String(v);
  if (Array.isArray(v)) return `[${v.length}]`;
  return '{…}';
}

/** The most common user fields across a page of records, for table columns. */
export function pickColumns(docs: Doc[], max = 4): string[] {
  const count = new Map<string, number>();
  for (const d of docs.slice(0, 200))
    for (const k of Object.keys(d)) if (!k.startsWith('_')) count.set(k, (count.get(k) ?? 0) + 1);
  return [...count.entries()].sort((a, b) => b[1] - a[1]).slice(0, max).map(([k]) => k);
}

export function allFields(docs: Doc[]): string[] {
  const s = new Set<string>();
  for (const d of docs.slice(0, 500)) for (const k of Object.keys(d)) if (k !== '_id') s.add(k);
  return [...s].sort();
}
