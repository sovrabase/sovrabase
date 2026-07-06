import { useMobileDashboard } from '../context';

export default function MobileQueuesPage() {
  const {
    queues, loadingQueues, unstickQueue, purgeQueue
  } = useMobileDashboard();
  return (
                  <div className="space-y-stack-md animate-fade-slide">
                    <p className="text-on-surface-variant font-label-caps text-[10px] tracking-widest">BACKGROUND TRANSACTION QUEUES ({queues.length})</p>
                    {loadingQueues ? (
                      <div className="flex items-center justify-center py-8">
                        <span className="animate-spin text-xl text-on-surface-variant">↻</span>
                      </div>
                    ) : queues.length === 0 ? (
                      <div className="flex flex-col items-center py-8 text-on-surface-variant/60 gap-2">
                        <span className="material-symbols-outlined text-2xl">queue</span>
                        <p className="text-xs">No message queues yet</p>
                      </div>
                    ) : (
                      <div className="space-y-3.5">
                        {queues.map(q => (
                          <div key={q.name} className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl flex justify-between items-center">
                            <div>
                              <div className="flex items-center gap-1.5">
                                <span className="material-symbols-outlined text-secondary text-sm">queue</span>
                                <h4 className="text-xs font-bold text-on-surface font-mono">{q.name}</h4>
                              </div>
                              <div className="flex gap-3 mt-1">
                                <p className="text-[9px] text-on-surface-variant/60"><span className="text-success font-mono">{q.visible.toLocaleString()}</span> visible</p>
                                <p className="text-[9px] text-on-surface-variant/60"><span className="text-yellow-500 font-mono">{q.in_flight.toLocaleString()}</span> in flight</p>
                                <p className="text-[9px] text-on-surface-variant/60"><span className="text-on-surface font-mono">{q.total.toLocaleString()}</span> total</p>
                              </div>
                            </div>
                            <div className="flex items-center gap-1 shrink-0">
                              <button onClick={() => unstickQueue(q.name)} className="p-1.5 rounded text-yellow-500 hover:bg-yellow-500/10" title="Make visible">
                                <span className="material-symbols-outlined text-sm">lock_open</span>
                              </button>
                              <button onClick={() => purgeQueue(q.name)} className="p-1.5 rounded text-error hover:bg-error/10" title="Purge">
                                <span className="material-symbols-outlined text-sm">delete</span>
                              </button>
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
  );
}
