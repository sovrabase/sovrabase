import { api } from '../../../api';
import { useMobileDashboard } from '../context';

export default function MobileCronPage() {
  const {
    showToast, cronJobs, loadingJobs, invokingStates, setActiveModal,
    setModalData, loadCronJobs, handleInvoke, projectId
  } = useMobileDashboard();
  return (
                  <div className="space-y-4 animate-fade-slide">
                    <div className="flex justify-between items-end pb-1 border-b border-outline-variant/20">
                      <h2 className="font-label-caps text-[10px] text-on-surface-variant uppercase tracking-widest font-bold">Active Scheduled Tasks ({cronJobs.length})</h2>
                    </div>
    
                    <div className="space-y-3.5">
                      {loadingJobs ? (
                        <div className="text-center py-6 text-on-surface-variant/50 text-xs">
                          <span className="material-symbols-outlined animate-spin text-[16px]">sync</span> Loading jobs...
                        </div>
                      ) : cronJobs.length === 0 ? (
                        <div className="text-center py-10 bg-surface-container/20 border border-dashed border-outline-variant rounded-xl text-xs text-on-surface-variant/40">
                          No cron jobs created yet.
                        </div>
                      ) : (
                        cronJobs.map(job => {
                          const isLoader = invokingStates[job.id] === 'loading';
                          
                          return (
                            <div key={job.id} className="bg-surface-container border border-outline-variant rounded-xl overflow-hidden p-stack-md flex justify-between items-center gap-2">
                              <div className="min-w-0">
                                <div className="flex items-center gap-1.5">
                                  <h4 className="text-xs font-bold text-on-surface font-mono">{job.name}</h4>
                                  <span className="bg-primary/10 text-primary text-[8px] font-bold px-1.5 py-0.5 rounded font-mono">{job.schedule}</span>
                                </div>
                                <p className="text-[9px] text-on-surface-variant/60 truncate max-w-[170px] font-mono mt-0.5">{job.url}</p>
                              </div>
                              
                              <div className="flex items-center gap-1 bg-surface-container-low px-1.5 py-1 rounded-lg border border-outline-variant/30 shrink-0">
                                <button
                                  disabled={isLoader}
                                  onClick={() => handleInvoke(job)}
                                  className="text-primary hover:bg-primary/10 p-1 rounded active:scale-90 transition-all"
                                >
                                  <span className="material-symbols-outlined text-[16px]">{isLoader ? 'sync' : 'play_arrow'}</span>
                                </button>
                                <button
                                  onClick={() => {
                                    if (confirm('Delete cron job?')) {
                                      api(`/admin/projects/${encodeURIComponent(projectId)}/cron/${encodeURIComponent(job.id)}`, { method: 'DELETE' })
                                        .then(() => { showToast('Job deleted', 'success'); loadCronJobs(); })
                                        .catch(() => showToast('Failed to delete', 'error'));
                                    }
                                  }}
                                  className="text-on-surface-variant hover:text-error p-1 rounded active:scale-90 transition-all"
                                >
                                  <span className="material-symbols-outlined text-[16px]">delete</span>
                                </button>
                              </div>
                            </div>
                          );
                        })
                      )}
                    </div>
    
                    <div className="pt-2">
                      <button 
                        onClick={() => {
                          setModalData({ cronName: '', cronSchedule: '*/5 * * * *', cronUrl: '' });
                          setActiveModal('create_cron');
                        }}
                        className="w-full bg-primary text-on-primary py-2 rounded-lg font-bold text-xs active:scale-[0.98] transition-all"
                      >
                        Add scheduled task
                      </button>
                    </div>
                  </div>
  );
}
