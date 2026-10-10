import { adminProjectPath } from '../../../api';

/** FileInfo from the server: no name field, the name is the last path segment. */
export interface StorageFile {
  path: string;
  bucket?: string;
  size?: number;
  content_type?: string;
  created_at?: string;
  updated_at?: string;
  url?: string;
}

export const fileName = (p: string) => p.slice(p.lastIndexOf('/') + 1);

const encPath = (p: string) => p.split('/').map(encodeURIComponent).join('/');

export function bucketPath(projectId: string, bucket: string, suffix = '') {
  return adminProjectPath(projectId, `/storage/buckets/${encodeURIComponent(bucket)}${suffix}`);
}

export function filePath(projectId: string, bucket: string, path: string) {
  return bucketPath(projectId, bucket, `/files/${encPath(path)}`);
}

function authHeaders(): Record<string, string> {
  const token = localStorage.getItem('sovrabase_admin_token');
  return token ? { Authorization: `Bearer ${token}` } : {};
}

async function failText(res: Response) {
  try {
    const j = await res.json();
    return j.error || j.message || res.statusText;
  } catch {
    return res.statusText;
  }
}

/** Multipart upload; `path` places the file inside a folder. */
export async function uploadFile(projectId: string, bucket: string, file: File, path: string) {
  const form = new FormData();
  form.append('file', file);
  form.append('path', path);
  const res = await fetch(bucketPath(projectId, bucket, '/files'), { method: 'POST', headers: authHeaders(), body: form });
  if (!res.ok) throw new Error(await failText(res));
}

/** Download through the admin API (works for private buckets too). */
export async function fetchBlob(projectId: string, bucket: string, path: string): Promise<Blob> {
  const res = await fetch(filePath(projectId, bucket, path), { headers: authHeaders() });
  if (!res.ok) throw new Error(await failText(res));
  return res.blob();
}

export function kindOf(ct?: string): 'image' | 'text' | 'video' | 'audio' | 'pdf' | 'other' {
  if (!ct) return 'other';
  if (ct.startsWith('image/')) return 'image';
  if (ct.startsWith('video/')) return 'video';
  if (ct.startsWith('audio/')) return 'audio';
  if (ct === 'application/pdf') return 'pdf';
  if (ct.startsWith('text/') || ct === 'application/json' || ct === 'application/xml' || ct === 'application/javascript') return 'text';
  return 'other';
}

export function extLabel(path: string, ct?: string) {
  const n = fileName(path);
  const i = n.lastIndexOf('.');
  if (i > 0 && n.length - i <= 6) return n.slice(i + 1).toUpperCase();
  return (ct?.split('/')[1] ?? 'FILE').slice(0, 4).toUpperCase();
}
