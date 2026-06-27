import { useState, useEffect, useRef, useCallback } from 'react';
import { Plus, HardDrive, FolderOpen, FileText, Upload, Download, Eye, Trash2, Loader2 } from 'lucide-react';
import { api, formatBytes, formatDate } from '../api';
import Modal from '../components/Modal';
import { useToast } from '../components/Toast';

interface BucketInfo { name: string; file_count?: number; total_size?: number; }
interface StorageFile { name: string; path: string; size?: number; content_type?: string; updated_at?: string; url?: string; }
interface Props { projectId: string; }

export default function StorageTab({ projectId }: Props) {
  const { showToast } = useToast();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [buckets, setBuckets] = useState<BucketInfo[]>([]);
  const [loadingBuckets, setLoadingBuckets] = useState(true);
  const [selectedBucket, setSelectedBucket] = useState<string | null>(null);
  const [files, setFiles] = useState<StorageFile[]>([]);
  const [loadingFiles, setLoadingFiles] = useState(false);
  const [showNewBucket, setShowNewBucket] = useState(false);
  const [newBucketName, setNewBucketName] = useState('');
  const [creatingBucket, setCreatingBucket] = useState(false);
  const [previewFile, setPreviewFile] = useState<StorageFile | null>(null);
  const [previewContent, setPreviewContent] = useState<string | null>(null);

  useEffect(() => {
    setLoadingBuckets(true);
    api<{ buckets: BucketInfo[] }>(`/admin/projects/${encodeURIComponent(projectId)}/storage/buckets`).then((d) => setBuckets(d.buckets || [])).catch(() => {}).finally(() => setLoadingBuckets(false));
  }, [projectId]);

  const loadFiles = useCallback(async (bucketName: string) => {
    setLoadingFiles(true);
    try { const d = await api<{ files: StorageFile[] }>(`/admin/projects/${encodeURIComponent(projectId)}/storage/buckets/${encodeURIComponent(bucketName)}/files`); setFiles(d.files || []); } catch { setFiles([]); }
    setLoadingFiles(false);
  }, [projectId]);

  const selectBucket = (name: string) => { setSelectedBucket(name); loadFiles(name); };

  const createBucket = async () => {
    if (!newBucketName.trim()) return;
    setCreatingBucket(true);
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/storage/buckets`, { method: 'POST', body: JSON.stringify({ name: newBucketName.trim() }) });
      showToast(`Bucket "${newBucketName.trim()}" created`, 'success');
      setShowNewBucket(false); setNewBucketName('');
      const d = await api<{ buckets: BucketInfo[] }>(`/admin/projects/${encodeURIComponent(projectId)}/storage/buckets`);
      setBuckets(d.buckets || []);
    } catch (e: unknown) { showToast((e as Error).message || 'Failed', 'error'); }
    setCreatingBucket(false);
  };

  const deleteBucket = async (name: string) => {
    if (!confirm(`Delete bucket "${name}" and all files?`)) return;
    try { await api(`/admin/projects/${encodeURIComponent(projectId)}/storage/buckets/${encodeURIComponent(name)}`, { method: 'DELETE' }); showToast(`Bucket "${name}" deleted`, 'success'); setSelectedBucket(null); setFiles([]); setBuckets((p) => p.filter((b) => b.name !== name)); } catch (e: unknown) { showToast((e as Error).message || 'Failed', 'error'); }
  };

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file || !selectedBucket) return;
    const form = new FormData(); form.append('file', file);
    try {
      const token = localStorage.getItem('sovrabase_admin_token');
      const headers: Record<string, string> = {};
      if (token) headers['Authorization'] = `Bearer ${token}`;
      const res = await fetch(`/admin/projects/${encodeURIComponent(projectId)}/storage/buckets/${encodeURIComponent(selectedBucket)}/files`, { method: 'POST', headers, body: form });
      if (!res.ok) throw new Error(await res.text());
      showToast(`File "${file.name}" uploaded`, 'success');
      loadFiles(selectedBucket);
    } catch (err: unknown) { showToast((err as Error).message || 'Upload failed', 'error'); }
    if (fileInputRef.current) fileInputRef.current.value = '';
  };

  const deleteFile = async (filePath: string) => {
    if (!confirm('Delete this file?') || !selectedBucket) return;
    try { await api(`/admin/projects/${encodeURIComponent(projectId)}/storage/buckets/${encodeURIComponent(selectedBucket)}/files/${encodeURIComponent(filePath)}`, { method: 'DELETE' }); showToast('File deleted', 'success'); setFiles((p) => p.filter((f) => f.path !== filePath)); } catch (e: unknown) { showToast((e as Error).message || 'Failed', 'error'); }
  };

  const preview = async (file: StorageFile) => {
    setPreviewFile(file); setPreviewContent(null);
    const ct = file.content_type || '';
    if (ct.startsWith('image/')) { setPreviewContent(file.url || ''); }
    else if (ct.startsWith('text/') || ct === 'application/json') {
      if (file.url) { try { const res = await fetch(file.url); setPreviewContent(await res.text()); } catch { setPreviewContent('Failed to load preview.'); } }
    } else { setPreviewContent('Preview not available for this file type.'); }
  };

  const isImage = (ct?: string) => ct?.startsWith('image/');
  const isText = (ct?: string) => ct?.startsWith('text/') || ct === 'application/json';

  if (loadingBuckets) return <div className="flex items-center gap-2 py-10 text-text-muted"><Loader2 className="w-4 h-4 animate-spin" /> Loading buckets...</div>;

  return (
    <div className="flex gap-0 h-[calc(100vh-280px)] min-h-[500px] border border-border rounded-lg overflow-hidden">
      {/* Left: Buckets (200px) */}
      <div className="w-[200px] shrink-0 border-r border-border flex flex-col bg-bg-card">
        <div className="px-4 py-3 border-b border-border flex items-center justify-between">
          <h3 className="text-text-primary font-semibold text-sm">Buckets</h3>
          <button onClick={() => { setNewBucketName(''); setShowNewBucket(true); }} className="p-1 rounded hover:bg-bg-input text-text-muted hover:text-text-primary" title="Create"><Plus className="w-4 h-4" /></button>
        </div>
        <div className="flex-1 overflow-y-auto">
          {buckets.length === 0 ? (
            <div className="flex flex-col items-center py-12 text-text-muted gap-2"><HardDrive className="w-6 h-6" /><p className="text-xs">No buckets</p></div>
          ) : buckets.map((b) => (
            <button key={b.name} onClick={() => selectBucket(b.name)} className={`w-full text-left px-4 py-2.5 flex items-center justify-between transition-colors ${selectedBucket === b.name ? 'bg-accent/10 border-r-2 border-accent' : 'hover:bg-bg-input/50'}`}>
              <span className="text-text-primary text-sm truncate">{b.name}</span>
              <span className="text-text-muted text-xs ml-2 shrink-0">{b.file_count ?? 0}</span>
            </button>
          ))}
        </div>
      </div>

      {/* Right: File browser (flex) */}
      <div className="flex-1 flex flex-col bg-bg-card">
        {!selectedBucket ? (
          <div className="flex flex-col items-center justify-center flex-1 text-text-muted gap-3"><HardDrive className="w-10 h-10" /><p className="text-sm">Select a bucket to browse files</p></div>
        ) : (
          <>
            <div className="px-4 py-3 border-b border-border flex items-center justify-between">
              <h3 className="text-text-primary font-semibold text-sm flex items-center gap-2"><FolderOpen className="w-4 h-4 text-accent" />{selectedBucket}<span className="text-text-muted text-xs font-normal">({files.length} files)</span></h3>
              <div className="flex items-center gap-2">
                <input ref={fileInputRef} type="file" onChange={handleUpload} className="hidden" />
                <button onClick={() => fileInputRef.current?.click()} className="flex items-center gap-1 px-3 py-1.5 bg-accent text-white rounded-lg text-xs font-medium hover:opacity-90"><Upload className="w-3.5 h-3.5" /> Upload File</button>
                <button onClick={() => deleteBucket(selectedBucket)} className="flex items-center gap-1 px-3 py-1.5 bg-bg-input border border-border rounded-lg text-text-muted text-xs hover:text-danger"><Trash2 className="w-3.5 h-3.5" /> Delete Bucket</button>
              </div>
            </div>
            <div className="flex-1 overflow-y-auto">
              {loadingFiles ? (
                <div className="flex items-center gap-2 py-12 justify-center text-text-muted"><Loader2 className="w-4 h-4 animate-spin" /> Loading files...</div>
              ) : files.length === 0 ? (
                <div className="flex flex-col items-center py-16 text-text-muted gap-3"><FileText className="w-10 h-10" /><p className="text-sm">No files in this bucket</p></div>
              ) : (
                <table className="w-full text-sm">
                  <thead className="bg-bg-input text-text-muted text-xs uppercase sticky top-0"><tr><th className="text-left px-4 py-3 font-medium">Name</th><th className="text-left px-4 py-3 font-medium w-24">Size</th><th className="text-left px-4 py-3 font-medium w-32">Type</th><th className="text-left px-4 py-3 font-medium w-44">Updated</th><th className="text-right px-4 py-3 font-medium w-32">Actions</th></tr></thead>
                  <tbody className="divide-y divide-border">
                    {files.map((f) => (
                      <tr key={f.path} className="hover:bg-bg-input/50 transition-colors">
                        <td className="px-4 py-2.5 text-text-primary text-xs font-mono truncate max-w-[200px]">{f.name}</td>
                        <td className="px-4 py-2.5 text-text-secondary text-xs">{f.size != null ? formatBytes(f.size) : '—'}</td>
                        <td className="px-4 py-2.5 text-text-muted text-xs truncate max-w-[120px]">{f.content_type || '—'}</td>
                        <td className="px-4 py-2.5 text-text-secondary text-xs whitespace-nowrap">{formatDate(f.updated_at)}</td>
                        <td className="px-4 py-2.5 text-right">
                          <div className="flex items-center justify-end gap-1">
                            {f.url && <a href={f.url} target="_blank" rel="noopener noreferrer" className="p-1.5 rounded text-text-muted hover:text-accent hover:bg-bg-input" title="Download"><Download className="w-3.5 h-3.5" /></a>}
                            <button onClick={() => preview(f)} className="p-1.5 rounded text-text-muted hover:text-accent hover:bg-bg-input" title="Preview"><Eye className="w-3.5 h-3.5" /></button>
                            <button onClick={() => deleteFile(f.path)} className="p-1.5 rounded text-text-muted hover:text-danger hover:bg-bg-input" title="Delete"><Trash2 className="w-3.5 h-3.5" /></button>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          </>
        )}
      </div>

      {/* Create Bucket Modal */}
      <Modal isOpen={showNewBucket} onClose={() => setShowNewBucket(false)} title="Create Bucket" size="sm">
        <div className="space-y-4">
          <input type="text" value={newBucketName} onChange={(e) => setNewBucketName(e.target.value)} placeholder="e.g. avatars, uploads" className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm placeholder:text-text-muted focus:outline-none focus:border-accent" autoFocus onKeyDown={(e) => e.key === 'Enter' && createBucket()} />
          <div className="flex justify-end gap-2"><button onClick={() => setShowNewBucket(false)} className="px-4 py-2 border border-border rounded-lg text-text-secondary text-sm hover:text-text-primary">Cancel</button><button onClick={createBucket} disabled={creatingBucket || !newBucketName.trim()} className="flex items-center gap-1 px-4 py-2 bg-accent text-white rounded-lg text-sm font-medium hover:opacity-90 disabled:opacity-50">{creatingBucket ? <Loader2 className="w-4 h-4 animate-spin" /> : <Plus className="w-4 h-4" />} Create</button></div>
        </div>
      </Modal>

      {/* Preview Modal */}
      <Modal isOpen={!!previewFile} onClose={() => { setPreviewFile(null); setPreviewContent(null); }} title={previewFile?.name || 'Preview'} size="lg">
        {previewFile && isImage(previewFile.content_type) && previewContent ? (
          <img src={previewContent} alt={previewFile.name} className="max-w-full max-h-[70vh] rounded-lg mx-auto" />
        ) : previewFile && isText(previewFile.content_type) && previewContent ? (
          <pre className="bg-bg-input border border-border rounded-lg p-4 text-text-primary text-xs font-mono overflow-auto max-h-[70vh] whitespace-pre-wrap">{previewContent}</pre>
        ) : (
          <div className="flex flex-col items-center py-12 text-text-muted gap-3"><FileText className="w-10 h-10" /><p className="text-sm">{previewContent || 'Preview not available'}</p></div>
        )}
      </Modal>
    </div>
  );
}
