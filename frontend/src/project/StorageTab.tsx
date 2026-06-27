import { useState, useEffect } from 'react';
import { HardDrive, File, Download, Loader2, ChevronRight } from 'lucide-react';
import { api, formatBytes, formatDate } from '../api';
import type { Bucket, StorageFile } from '../types';

interface Props {
  projectId: string;
  apiKey?: string;
}

export default function StorageTab({ projectId, apiKey }: Props) {
  const [buckets, setBuckets] = useState<Bucket[]>([]);
  const [loading, setLoading] = useState(true);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [files, setFiles] = useState<Record<string, StorageFile[]>>({});
  const [filesLoading, setFilesLoading] = useState(false);

  useEffect(() => {
    setLoading(true);
    api<{ buckets: Bucket[] }>(`/admin/storage/buckets`)
      .then((data) => setBuckets(data.buckets || []))
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  const toggleBucket = async (name: string) => {
    if (expanded === name) {
      setExpanded(null);
      return;
    }
    setExpanded(name);
    if (!files[name]) {
      setFilesLoading(true);
      try {
        const data = await api<{ files: StorageFile[] }>(`/admin/storage/${encodeURIComponent(name)}/files`);
        setFiles((prev) => ({ ...prev, [name]: data.files || [] }));
      } catch { /* ignore */ }
      setFilesLoading(false);
    }
  };

  if (loading) {
    return <div className="flex items-center gap-2 py-10 text-text-muted"><Loader2 className="w-4 h-4 animate-spin" /> Loading buckets...</div>;
  }

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold text-text-primary">Buckets ({buckets.length})</h2>

      {buckets.length === 0 ? (
        <div className="flex flex-col items-center py-16 text-text-muted gap-3">
          <HardDrive className="w-10 h-10" />
          <p>No buckets</p>
        </div>
      ) : (
        <div className="border border-border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-bg-input text-text-muted text-xs uppercase">
              <tr>
                <th className="text-left px-4 py-3 font-medium w-8"></th>
                <th className="text-left px-4 py-3 font-medium">Name</th>
                <th className="text-left px-4 py-3 font-medium">Files</th>
                <th className="text-left px-4 py-3 font-medium">Total Size</th>
                <th className="text-left px-4 py-3 font-medium">Created</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {buckets.map((b) => (
                <>
                  <tr
                    key={b.name}
                    className="hover:bg-bg-input/50 transition-colors cursor-pointer"
                    onClick={() => toggleBucket(b.name)}
                  >
                    <td className="px-4 py-3">
                      <ChevronRight className={`w-4 h-4 text-text-muted transition-transform ${expanded === b.name ? 'rotate-90' : ''}`} />
                    </td>
                    <td className="px-4 py-3 font-medium text-text-primary">{b.name}</td>
                    <td className="px-4 py-3 text-text-secondary">{b.file_count ?? '—'}</td>
                    <td className="px-4 py-3 text-text-secondary">{b.total_size != null ? formatBytes(b.total_size) : '—'}</td>
                    <td className="px-4 py-3 text-text-secondary text-xs">{formatDate(b.created_at)}</td>
                  </tr>
                  {expanded === b.name && (
                    <tr className="bg-bg-input/30">
                      <td colSpan={5} className="px-8 py-4">
                        {filesLoading ? (
                          <div className="flex items-center gap-2 py-4 text-text-muted"><Loader2 className="w-4 h-4 animate-spin" /> Loading files...</div>
                        ) : (files[b.name] || []).length === 0 ? (
                          <p className="text-text-muted text-sm py-2">No files in this bucket</p>
                        ) : (
                          <table className="w-full text-xs">
                            <thead className="text-text-muted">
                              <tr>
                                <th className="text-left py-2 font-medium">Name</th>
                                <th className="text-left py-2 font-medium">Type</th>
                                <th className="text-left py-2 font-medium">Size</th>
                                <th className="text-left py-2 font-medium">Updated</th>
                                <th className="text-right py-2 font-medium">Download</th>
                              </tr>
                            </thead>
                            <tbody>
                              {(files[b.name] || []).map((f) => (
                                <tr key={f.path} className="border-t border-border/50">
                                  <td className="py-2 text-text-primary font-mono">{f.name}</td>
                                  <td className="py-2 text-text-muted">{f.content_type || '—'}</td>
                                  <td className="py-2 text-text-secondary">{f.size != null ? formatBytes(f.size) : '—'}</td>
                                  <td className="py-2 text-text-secondary">{formatDate(f.updated_at)}</td>
                                  <td className="py-2 text-right">
                                    {f.url && (
                                      <a href={f.url} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 text-accent hover:text-accent-hover transition-colors">
                                        <Download className="w-3 h-3" /> DL
                                      </a>
                                    )}
                                  </td>
                                </tr>
                              ))}
                            </tbody>
                          </table>
                        )}
                      </td>
                    </tr>
                  )}
                </>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
