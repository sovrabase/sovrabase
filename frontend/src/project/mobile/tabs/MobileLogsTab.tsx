import { useMobileDashboard } from '../context';

export default function MobileLogsTab() {
  const {
    isStreaming, logFilter, setLogFilter, projectUsage, handleClearLogs,
    filteredLogs
  } = useMobileDashboard();
  return (
    <div className="p-container-padding space-y-stack-md animate-fade-slide relative">
                
                {/* Realtime metric widgets */}
                <section className="grid grid-cols-2 gap-gutter mb-1">
                  <div className="bg-surface-container-low border border-outline-variant rounded-xl p-stack-md bento-glow">
                    <div className="flex items-center gap-2 mb-1">
                      <div className="w-1.5 h-1.5 rounded-full bg-primary shadow-[0_0_8px_#00d2ff]"></div>
                      <span className="font-label-caps text-[10px] text-on-surface-variant tracking-widest uppercase">API REQUESTS</span>
                    </div>
                    <div className="font-headline-lg text-lg font-bold text-on-surface">{projectUsage?.api_requests || 0}</div>
                  </div>
                  <div className="bg-surface-container-low border border-outline-variant rounded-xl p-stack-md">
                    <div className="flex items-center gap-2 mb-1">
                      <div className="w-1.5 h-1.5 rounded-full bg-secondary shadow-[0_0_8px_#d0bcff]"></div>
                      <span className="font-label-caps text-[10px] text-on-surface-variant tracking-widest uppercase">WEBSOCKETS</span>
                    </div>
                    <div className="font-headline-lg text-lg font-bold text-on-surface">
                      {projectUsage?.realtime_connections || 0}
                    </div>
                  </div>
                </section>
    
                {/* Filter chips + Clear Logs */}
                <div className="flex gap-2 overflow-x-auto no-scrollbar pb-1 sticky top-0 bg-background/90 z-20 backdrop-blur-md py-2 border-b border-outline-variant/20 items-center">
                  <button 
                    onClick={() => setLogFilter('ALL')}
                    className={`px-3 py-1 rounded-full text-[10px] tracking-wider font-label-caps whitespace-nowrap active:scale-95 transition-transform border ${
                      logFilter === 'ALL' 
                        ? 'bg-primary/10 border-primary text-primary font-bold' 
                        : 'bg-surface-container-high border-outline-variant text-on-surface-variant'
                    }`}
                  >
                    ALL LOGS
                  </button>
                  <button 
                    onClick={() => setLogFilter('500')}
                    className={`px-3 py-1 rounded-full text-[10px] tracking-wider font-label-caps whitespace-nowrap active:scale-95 transition-transform flex items-center gap-1.5 border ${
                      logFilter === '500' 
                        ? 'bg-error-container/20 border-error text-error font-bold' 
                        : 'bg-surface-container-high border-outline-variant text-on-surface-variant'
                    }`}
                  >
                    <span className="w-1.5 h-1.5 rounded-full bg-error"></span> 500 ERRORS
                  </button>
                  <button 
                    onClick={() => setLogFilter('404')}
                    className={`px-3 py-1 rounded-full text-[10px] tracking-wider font-label-caps whitespace-nowrap active:scale-95 transition-transform flex items-center gap-1.5 border ${
                      logFilter === '404' 
                        ? 'bg-secondary-container/20 border-secondary text-secondary font-bold' 
                        : 'bg-surface-container-high border-outline-variant text-on-surface-variant'
                    }`}
                  >
                    <span className="w-1.5 h-1.5 rounded-full bg-secondary"></span> 404 WARNINGS
                  </button>
                  <span className="flex-1"></span>
                  <button 
                    onClick={handleClearLogs}
                    className="px-3 py-1 rounded-full text-[10px] tracking-wider font-label-caps whitespace-nowrap active:scale-95 transition-transform flex items-center gap-1.5 bg-error/10 border border-error/30 text-error hover:bg-error/20"
                  >
                    <span className="material-symbols-outlined text-[14px]">delete_sweep</span> CLEAR
                  </button>
                </div>
    
                {/* Stream Logs timeline list */}
                <div className="space-y-2 relative" id="logs-container">
                  {filteredLogs.length === 0 ? (
                    <div className="text-center py-12 text-on-surface-variant/40">
                      No logs matching filter
                    </div>
                  ) : (
                    filteredLogs.map((log, index) => {
                      // Determine Accent Color based on HTTP Method or Status
                      let logBorderColor = 'bg-primary'; // Green [GET]
                      let logTextColor = 'text-primary';
                      if (log.method === 'POST') {
                        logBorderColor = 'bg-secondary'; // Violet
                        logTextColor = 'text-secondary';
                      } else if (log.method === 'ERROR' || log.status >= 500) {
                        logBorderColor = 'bg-error'; // Red
                        logTextColor = 'text-error';
                      } else if (log.status === 404) {
                        logBorderColor = 'bg-on-tertiary-container'; // Amber/Yellow
                        logTextColor = 'text-on-tertiary-container';
                      } else if (log.method === 'PUT' || log.method === 'PATCH') {
                        logBorderColor = 'bg-secondary';
                        logTextColor = 'text-secondary';
                      }
    
                      return (
                        <div 
                          key={index}
                          className="log-entry bg-surface-container-low border border-outline-variant rounded p-stack-sm flex flex-col gap-1.5 relative overflow-hidden group animate-fade-slide"
                        >
                          {/* Color indicator on absolute left border */}
                          <div className={`absolute left-0 top-0 bottom-0 w-1 ${logBorderColor}`}></div>
                          
                          <div className="flex justify-between items-center px-1">
                            <div className="flex items-center gap-3">
                              <span className="font-code-xs text-code-xs text-on-surface-variant">{log.timestamp}</span>
                              <span className={`font-code-sm text-code-sm font-extrabold ${logTextColor}`}>[{log.method}]</span>
                            </div>
                            <div className="flex items-center gap-2">
                              <span className={`font-code-sm text-code-sm ${logTextColor}`}>
                                {log.status} {log.status === 200 ? 'OK' : log.status === 201 ? 'CREATED' : log.status === 500 ? 'SERVER ERR' : log.status === 404 ? 'NOT FOUND' : 'NO CONTENT'}
                              </span>
                              <span className="font-code-xs text-code-xs text-on-surface-variant">{log.duration}</span>
                            </div>
                          </div>
                          <div className="font-code-sm text-code-sm text-on-surface truncate px-1 font-semibold select-text">
                            {log.path}
                          </div>
                        </div>
                      );
                    })
                  )}
                </div>
    
                {/* Streaming atmospheric bouncing balls indicator */}
                {isStreaming && (
                  <div className="flex justify-center items-center py-stack-lg gap-2">
                    <span className="w-1.5 h-1.5 bg-primary/60 rounded-full animate-bounce" style={{ animationDelay: '0s' }}></span>
                    <span className="w-1.5 h-1.5 bg-primary/60 rounded-full animate-bounce" style={{ animationDelay: '0.2s' }}></span>
                    <span className="w-1.5 h-1.5 bg-primary/60 rounded-full animate-bounce" style={{ animationDelay: '0.4s' }}></span>
                  </div>
                )}
    
              </div>
  );
}
