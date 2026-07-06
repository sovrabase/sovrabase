import { api } from '../../../api';
import { useMobileDashboard } from '../context';

export default function MobileStoragePage() {
  const {
    showToast, buckets, selectedBucket, setSelectedBucket, files,
    loadingBuckets, loadingFiles, setActiveModal, setModalData, loadBuckets, loadFiles,
    projectId
  } = useMobileDashboard();
  return (
                  <div className="space-y-stack-md animate-fade-slide">
                    {selectedBucket ? (
                      /* Bucket Files View */
                      <div className="space-y-4">
                        <div className="flex justify-between items-center bg-surface-container-lowest/30 p-2 border border-outline-variant/35 rounded-lg">
                          <button 
                            onClick={() => setSelectedBucket(null)}
                            className="flex items-center gap-1 text-[10px] text-primary hover:text-accent font-bold uppercase"
                          >
                            <span className="material-symbols-outlined text-[14px]">arrow_back</span> Buckets
                          </button>
                          <span className="text-[10px] font-bold text-on-surface truncate max-w-[130px] font-mono">
                            {selectedBucket}
                          </span>
                          <button 
                            onClick={() => {
                              setModalData({ fileName: '', fileContent: 'Hello Sovrabase storage!' });
                              setActiveModal('upload_file');
                            }}
                            className="flex items-center gap-0.5 text-primary hover:bg-primary/10 px-2 py-0.5 rounded text-[9px] font-bold"
                          >
                            <span className="material-symbols-outlined text-[12px]">upload</span> UPLOAD
                          </button>
                        </div>
    
                        <div className="space-y-2">
                          {loadingFiles ? (
                            <div className="text-center py-6 text-on-surface-variant/50 text-xs">
                              <span className="material-symbols-outlined animate-spin text-[16px] mr-1.5 align-middle">sync</span> Loading files...
                            </div>
                          ) : files.length === 0 ? (
                            <div className="text-center py-10 bg-surface-container/20 border border-dashed border-outline-variant rounded-xl text-xs text-on-surface-variant/40">
                              No files found in this bucket.
                            </div>
                          ) : (
                            files.map((file, idx) => (
                              <div key={idx} className="bg-surface-container-low border border-outline-variant/60 rounded-xl p-stack-md flex justify-between items-center">
                                <div className="flex items-center gap-2.5 min-w-0">
                                  <span className="material-symbols-outlined text-primary text-[18px]">description</span>
                                  <div className="min-w-0">
                                    <p className="text-xs font-bold text-on-surface truncate max-w-[150px] font-mono">{file.name || file.path}</p>
                                    <p className="text-[9px] text-on-surface-variant/60 font-code-xs">
                                      {file.size ? `${(file.size / 1024).toFixed(1)} KB` : 'Unknown size'} • {file.contentType || 'binary'}
                                    </p>
                                  </div>
                                </div>
                                <button
                                  onClick={() => {
                                    if (confirm(`Delete file "${file.name || file.path}"?`)) {
                                      api(`/admin/projects/${encodeURIComponent(projectId)}/storage/buckets/${encodeURIComponent(selectedBucket)}/files/${encodeURIComponent(file.name || file.path)}`, {
                                        method: 'DELETE'
                                      }).then(() => {
                                        showToast('File deleted', 'success');
                                        loadFiles(selectedBucket);
                                      }).catch(() => showToast('Delete failed', 'error'));
                                    }
                                  }}
                                  className="text-on-surface-variant hover:text-error transition-colors p-1"
                                >
                                  <span className="material-symbols-outlined text-[16px]">delete</span>
                                </button>
                              </div>
                            ))
                          )}
                        </div>
                      </div>
                    ) : (
                      /* Buckets List */
                      <div className="space-y-4">
                        <div className="flex justify-between items-center">
                          <span className="text-on-surface-variant font-label-caps text-[10px] tracking-widest">{buckets.length} BUCKETS</span>
                          <button 
                            onClick={() => {
                              setModalData({ bucketName: '' });
                              setActiveModal('create_bucket');
                            }}
                            className="flex items-center gap-1 text-primary hover:bg-primary/10 px-2.5 py-1 rounded transition-colors text-[9px] font-bold font-label-caps"
                          >
                            <span className="material-symbols-outlined text-[14px]">create_new_folder</span> CREATE BUCKET
                          </button>
                        </div>
    
                        <div className="space-y-2">
                          {loadingBuckets ? (
                            <div className="text-center py-6 text-on-surface-variant/50 text-xs">
                              <span className="material-symbols-outlined animate-spin text-[16px] mr-1.5 align-middle">sync</span> Loading buckets...
                            </div>
                          ) : buckets.length === 0 ? (
                            <div className="text-center py-12 bg-surface-container/20 border border-dashed border-outline-variant rounded-xl text-xs text-on-surface-variant/40 p-6">
                              No storage buckets configured.
                            </div>
                          ) : (
                            buckets.map((b) => (
                              <div 
                                key={b.name} 
                                className="bg-surface-container border border-outline-variant rounded-xl overflow-hidden hover:bg-surface-container-high transition-colors"
                              >
                                <div className="p-stack-md flex justify-between items-center">
                                  <button 
                                    onClick={() => {
                                      setSelectedBucket(b.name);
                                      loadFiles(b.name);
                                    }}
                                    className="flex items-center gap-3 text-left focus:outline-none flex-1"
                                  >
                                    <span className="material-symbols-outlined text-primary text-[22px]">folder</span>
                                    <div>
                                      <p className="text-xs font-bold text-on-surface font-mono">{b.name}</p>
                                      <p className="text-[9px] text-on-surface-variant/60">Tap to browse S3-ready files</p>
                                    </div>
                                  </button>
                                  <button
                                    onClick={() => {
                                      if (confirm(`Delete storage bucket "${b.name}"?`)) {
                                        api(`/admin/projects/${encodeURIComponent(projectId)}/storage/buckets/${encodeURIComponent(b.name)}`, {
                                          method: 'DELETE'
                                        }).then(() => {
                                          showToast('Bucket deleted', 'success');
                                          loadBuckets();
                                        }).catch(() => showToast('Failed to delete bucket', 'error'));
                                      }
                                    }}
                                    className="text-on-surface-variant hover:text-error transition-colors p-1.5"
                                  >
                                    <span className="material-symbols-outlined text-[16px]">delete</span>
                                  </button>
                                </div>
                              </div>
                            ))
                          )}
                        </div>
                      </div>
                    )}
                  </div>
  );
}
