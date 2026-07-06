import { api } from '../../../api';
import { useMobileDashboard } from '../context';

export default function MobileConfigPage() {
  const {
    showToast, projectDetail, setProjectDetail, savingConfig, setSavingConfig,
    configEntries, loadingConfigEntries, setActiveModal, setModalData, loadProjectConfig, loadConfigEntries,
    projectId
  } = useMobileDashboard();
  return (
    <>
      <div className="space-y-stack-md animate-fade-slide bg-surface-container border border-outline-variant p-stack-md rounded-xl">
                    <div className="border-b border-outline-variant/30 pb-2">
                      <h3 className="text-xs font-bold text-on-surface uppercase tracking-widest">Project Credentials</h3>
                    </div>
                    
                    {projectDetail ? (
                      <div className="space-y-3.5">
                        <div>
                          <span className="text-[8px] font-bold text-on-surface-variant uppercase tracking-wider block mb-1">CORS Allowed Origins</span>
                          <input 
                            type="text"
                            value={projectDetail.allow_origins || ''}
                            onChange={(e) => setProjectDetail({ ...projectDetail, allow_origins: e.target.value })}
                            className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                            placeholder="e.g. *, http://localhost:3000"
                          />
                        </div>
    
                        <div>
                          <span className="text-[8px] font-bold text-on-surface-variant uppercase tracking-wider block mb-1">Storage Quota (Bytes)</span>
                          <input 
                            type="number"
                            value={projectDetail.storage_quota || 0}
                            onChange={(e) => setProjectDetail({ ...projectDetail, storage_quota: Number(e.target.value) })}
                            className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                          />
                        </div>
    
                        <div>
                          <span className="text-[8px] font-bold text-on-surface-variant uppercase tracking-wider block mb-0.5">Project API Client Key</span>
                          <div className="flex gap-2 items-center bg-surface-container-lowest/50 border border-outline-variant/30 px-2 py-1.5 rounded-lg select-all">
                            <span className="font-code-xs text-[10px] text-primary truncate flex-1 font-mono">{projectDetail.api_key || 'No key generated'}</span>
                            <button 
                              onClick={() => {
                                navigator.clipboard.writeText(projectDetail.api_key || '');
                                showToast('API Key copied', 'success');
                              }}
                              className="text-on-surface-variant hover:text-primary active:scale-90 transition-all"
                            >
                              <span className="material-symbols-outlined text-[16px]">content_copy</span>
                            </button>
                          </div>
                        </div>
    
                        <div className="pt-2">
                          <button
                            onClick={() => {
                              setSavingConfig(true);
                              api(`/admin/projects/${encodeURIComponent(projectId)}`, {
                                method: 'PUT',
                                body: JSON.stringify({
                                  allow_origins: projectDetail.allow_origins,
                                  storage_quota: Number(projectDetail.storage_quota)
                                })
                              }).then(() => {
                                showToast('Project configuration updated', 'success');
                                loadProjectConfig();
                              }).catch(err => showToast(err.message || 'Save failed', 'error'))
                                .finally(() => setSavingConfig(false));
                            }}
                            disabled={savingConfig}
                            className="w-full bg-primary text-on-primary py-2 rounded-lg font-bold text-xs active:scale-[0.98] transition-all"
                          >
                            {savingConfig ? 'Saving...' : 'Save Configuration'}
                          </button>
                        </div>
                      </div>
                    ) : (
                      <div className="text-center py-6 text-on-surface-variant/40 text-xs">
                        Failed to load project details.
                      </div>
                    )}
      </div>

      <div className="space-y-stack-md animate-fade-slide bg-surface-container border border-outline-variant p-stack-md rounded-xl">
                    <div className="flex justify-between items-center border-b border-outline-variant/30 pb-2">
                      <h3 className="text-xs font-bold text-on-surface uppercase tracking-widest">Config Entries</h3>
                      <button
                        onClick={() => {
                          setModalData({ configKey: '', configValue: '', configType: 'string', configPublic: 'false' });
                          setActiveModal('add_config');
                        }}
                        className="flex items-center gap-1 text-primary hover:bg-primary/10 px-2 py-0.5 rounded text-[9px] font-bold active:scale-95 transition-all"
                      >
                        <span className="material-symbols-outlined text-[14px]">add</span> ADD ENTRY
                      </button>
                    </div>
    
                    {loadingConfigEntries ? (
                      <div className="text-center py-6 text-on-surface-variant/50 text-xs">
                        <span className="material-symbols-outlined animate-spin text-[16px] mr-1.5 align-middle">sync</span> Loading entries...
                      </div>
                    ) : configEntries.length === 0 ? (
                      <div className="text-center py-10 bg-surface-container/20 border border-dashed border-outline-variant rounded-xl text-xs text-on-surface-variant/40">
                        No config entries defined.
                      </div>
                    ) : (
                      <div className="space-y-2.5">
                        {configEntries.map((entry: any, idx: number) => (
                          <div key={entry.key || idx} className="bg-surface-container-low border border-outline-variant/60 rounded-lg p-3 flex justify-between items-start gap-2">
                            <div className="min-w-0 flex-1">
                              <div className="flex items-center gap-1.5 mb-1 flex-wrap">
                                <span className="text-xs font-bold text-on-surface font-mono truncate">{entry.key}</span>
                                <span className={`text-[8px] font-bold px-1.5 py-0.5 rounded border font-mono ${
                                  entry.type === 'string' ? 'bg-primary/10 text-primary border-primary/20' :
                                  entry.type === 'number' ? 'bg-secondary/10 text-secondary border-secondary/20' :
                                  entry.type === 'boolean' ? 'bg-tertiary/10 text-tertiary border-tertiary/20' :
                                  'bg-surface-container-high text-on-surface-variant border-outline-variant/40'
                                }`}>
                                  {entry.type}
                                </span>
                                <span className={`text-[8px] font-bold px-1.5 py-0.5 rounded border font-mono ${
                                  entry.public ? 'bg-success/10 text-success border-success/20' : 'bg-error/10 text-error border-error/20'
                                }`}>
                                  {entry.public ? 'PUBLIC' : 'PRIVATE'}
                                </span>
                              </div>
                              <p className="text-[10px] text-on-surface-variant/70 font-mono truncate max-w-[200px]">
                                {typeof entry.value === 'string' ? entry.value : JSON.stringify(entry.value)}
                              </p>
                            </div>
                            <button
                              onClick={() => {
                                if (confirm(`Delete config entry "${entry.key}"?`)) {
                                  api(`/admin/projects/${encodeURIComponent(projectId)}/config/${encodeURIComponent(entry.key)}`, {
                                    method: 'DELETE'
                                  }).then(() => {
                                    showToast('Entry deleted', 'success');
                                    loadConfigEntries();
                                  }).catch(() => showToast('Failed to delete', 'error'));
                                }
                              }}
                              className="text-on-surface-variant hover:text-error transition-colors p-1 shrink-0 active:scale-90"
                            >
                              <span className="material-symbols-outlined text-[16px]">delete</span>
                            </button>
                          </div>
                        ))}
                      </div>
                    )}
      </div>
    </>
  );
}
