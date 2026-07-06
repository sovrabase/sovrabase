import { api } from '../../../api';
import { useMobileDashboard } from '../context';

export default function MobileWebhooksPage() {
  const {
    showToast, webhooks, loadingWebhooks, setActiveModal, setModalData,
    loadWebhooks, projectId
  } = useMobileDashboard();
  return (
                  <div className="space-y-stack-md animate-fade-slide">
                    <div className="flex justify-between items-center">
                      <span className="text-on-surface-variant font-label-caps text-[10px] tracking-widest">{webhooks.length} OUTGOING SUBSCRIPTIONS</span>
                      <button 
                        onClick={() => {
                          setModalData({ webhookUrl: '', webhookEvents: 'insert,update,delete' });
                          setActiveModal('add_webhook');
                        }}
                        className="flex items-center gap-1 text-primary hover:bg-primary/10 px-2 py-0.5 rounded text-[9px] font-bold"
                      >
                        <span className="material-symbols-outlined text-[14px]">add_link</span> ADD WEBHOOK
                      </button>
                    </div>
    
                    <div className="space-y-3.5">
                      {loadingWebhooks ? (
                        <div className="text-center py-6 text-on-surface-variant/50 text-xs">
                          <span className="material-symbols-outlined animate-spin text-[16px] mr-1.5 align-middle">sync</span> Loading webhooks...
                        </div>
                      ) : webhooks.length === 0 ? (
                        <div className="text-center py-10 bg-surface-container/20 border border-dashed border-outline-variant rounded-xl text-xs text-on-surface-variant/40">
                          No webhooks registered.
                        </div>
                      ) : (
                        webhooks.map(wh => (
                          <div key={wh.id} className="bg-surface-container border border-outline-variant p-stack-md rounded-xl space-y-2">
                            <div className="flex justify-between items-start">
                              <div className="min-w-0 flex-1 pr-2">
                                <p className="text-xs font-bold text-on-surface truncate font-mono">{wh.url}</p>
                                <div className="flex gap-1.5 mt-1.5 flex-wrap">
                                  {(wh.events || []).map((ev: string) => (
                                    <span key={ev} className="bg-secondary/15 text-secondary text-[8px] font-bold px-1.5 py-0.5 rounded border border-secondary/20 font-mono">
                                      {ev}
                                    </span>
                                  ))}
                                </div>
                              </div>
                              <button
                                onClick={() => {
                                  if (confirm('Delete this webhook subscription?')) {
                                    api(`/admin/projects/${encodeURIComponent(projectId)}/webhooks/${encodeURIComponent(wh.id)}`, {
                                      method: 'DELETE'
                                    }).then(() => {
                                      showToast('Webhook deleted', 'success');
                                      loadWebhooks();
                                    }).catch(() => showToast('Failed to delete', 'error'));
                                  }
                                }}
                                className="text-on-surface-variant hover:text-error transition-colors p-1"
                              >
                                <span className="material-symbols-outlined text-[16px]">delete</span>
                              </button>
                            </div>
                          </div>
                        ))
                      )}
                    </div>
                  </div>
  );
}
