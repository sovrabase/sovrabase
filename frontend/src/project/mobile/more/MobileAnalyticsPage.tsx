import { useMobileDashboard } from '../context';

export default function MobileAnalyticsPage() {
  const {
    projectUsage, analyticsData
  } = useMobileDashboard();
  return (
                  <div className="space-y-stack-md animate-fade-slide">
                    <p className="text-on-surface-variant font-label-caps text-[10px] tracking-widest">METRICS OVERVIEW (LIVE TELEMETRY)</p>
                    
                    {projectUsage ? (
                      <div className="space-y-stack-md">
                        <div className="grid grid-cols-2 gap-stack-sm">
                          <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-1">
                            <span className="text-[8px] font-bold text-on-surface-variant uppercase block">Total API Requests</span>
                            <span className="text-md font-bold text-primary font-mono">{projectUsage.api_requests || 0}</span>
                            <span className="text-[8px] text-on-surface-variant/50">Hits since creation</span>
                          </div>
                          <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-1">
                            <span className="text-[8px] font-bold text-on-surface-variant uppercase block">Active WebSockets</span>
                            <span className="text-md font-bold text-secondary font-mono">{projectUsage.realtime_connections || 0}</span>
                            <span className="text-[8px] text-on-surface-variant/50">Realtime channels</span>
                          </div>
                          <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-1">
                            <span className="text-[8px] font-bold text-on-surface-variant uppercase block">DB Read Operations</span>
                            <span className="text-md font-bold text-primary font-mono">{projectUsage.db_reads_total || 0}</span>
                            <span className="text-[8px] text-on-surface-variant/50">Reads from disk</span>
                          </div>
                          <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-1">
                            <span className="text-[8px] font-bold text-on-surface-variant uppercase block">DB Write Operations</span>
                            <span className="text-md font-bold text-secondary font-mono">{projectUsage.db_writes_total || 0}</span>
                            <span className="text-[8px] text-on-surface-variant/50">Writes to disk</span>
                          </div>
                        </div>
    
                        {/* Storage info card */}
                        <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-2">
                          <div className="flex justify-between items-center border-b border-outline-variant/30 pb-1.5">
                            <h4 className="text-xs font-bold text-on-surface uppercase tracking-wider">Disk Storage Breakdown</h4>
                            <span className="text-[10px] font-bold text-primary font-mono">
                              {((projectUsage.total_storage_bytes || 0) / (1024 * 1024)).toFixed(2)} MB Total
                            </span>
                          </div>
                          <div className="space-y-1.5 text-[10px]">
                            <div className="flex justify-between">
                              <span className="text-on-surface-variant/80">Pebble Database size</span>
                              <span className="font-mono text-on-surface">
                                {((projectUsage.database_bytes || 0) / (1024 * 1024)).toFixed(2)} MB
                              </span>
                            </div>
                            <div className="flex justify-between">
                              <span className="text-on-surface-variant/80">User File storage size</span>
                              <span className="font-mono text-on-surface">
                                {((projectUsage.file_storage_bytes || 0) / (1024 * 1024)).toFixed(2)} MB
                              </span>
                            </div>
                          </div>
                        </div>
    
                        {/* --- EVENT ANALYTICS (24h) --- */}
                        {analyticsData ? (
                          <div className="space-y-stack-md">
                            {/* Total Events (24h) Summary Card */}
                            <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-1">
                              <span className="text-[8px] font-bold text-on-surface-variant uppercase block">Total Events (24h)</span>
                              <span className="text-md font-bold text-primary font-mono">{(analyticsData.total_events || 0).toLocaleString()}</span>
                              <span className="text-[8px] text-on-surface-variant/50">Custom analytics events</span>
                            </div>
    
                            {/* Hourly Activity Bar Chart */}
                            <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-3">
                              <h4 className="text-xs font-bold text-on-surface uppercase tracking-wider">Hourly Activity</h4>
                              {analyticsData.hourly && analyticsData.hourly.length > 0 ? (
                                <div className="space-y-1">
                                  {(() => {
                                    const maxCount = Math.max(...analyticsData.hourly.map((h: any) => h.count), 1);
                                    return analyticsData.hourly.map((h: any) => (
                                      <div key={h.hour} className="flex items-center gap-2">
                                        <span className="w-6 text-right text-[9px] text-on-surface-variant/60 font-mono">
                                          {String(h.hour).padStart(2, '0')}
                                        </span>
                                        <div className="flex-1 h-4 bg-surface-container-high rounded-full overflow-hidden">
                                          <div
                                            className="bg-primary/30 h-full rounded-full flex items-center justify-end pr-1.5 min-w-[2px]"
                                            style={{ width: `${Math.max((h.count / maxCount) * 100, 0.5)}%` }}
                                          >
                                            {h.count > 0 && (h.count / maxCount) > 0.15 && (
                                              <span className="text-[7px] font-bold text-on-surface font-mono leading-none">{h.count}</span>
                                            )}
                                          </div>
                                        </div>
                                        {((h.count / maxCount) <= 0.15 || h.count === 0) && (
                                          <span className="w-8 text-left text-[8px] text-on-surface-variant/40 font-mono">{h.count > 0 ? h.count : ''}</span>
                                        )}
                                      </div>
                                    ));
                                  })()}
                                  <div className="flex items-center gap-2 pt-1">
                                    <span className="w-6" />
                                    <div className="flex-1 flex justify-between text-[8px] text-on-surface-variant/40">
                                      <span>00</span><span>06</span><span>12</span><span>18</span><span>23</span>
                                    </div>
                                    <span className="w-8" />
                                  </div>
                                </div>
                              ) : (
                                <p className="text-[10px] text-on-surface-variant/60 italic text-center py-2">No hourly data yet</p>
                              )}
                            </div>
    
                            {/* Top Events */}
                            {analyticsData.top_events && analyticsData.top_events.length > 0 && (
                              <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-3">
                                <h4 className="text-xs font-bold text-on-surface uppercase tracking-wider">Top Events</h4>
                                <div className="space-y-1.5">
                                  {analyticsData.top_events.slice(0, 5).map((e: any, i: number) => (
                                    <div key={i} className="flex justify-between items-center py-1">
                                      <span className="text-[10px] text-on-surface font-mono truncate mr-2">{e.event || e.name}</span>
                                      <span className="text-[10px] font-bold text-primary bg-primary/10 px-2 py-0.5 rounded-full font-mono">
                                        {(e.count || 0).toLocaleString()}
                                      </span>
                                    </div>
                                  ))}
                                </div>
                              </div>
                            )}
                          </div>
                        ) : (
                          <div className="text-center py-4 bg-surface-container/20 border border-dashed border-outline-variant rounded-xl text-[10px] text-on-surface-variant/40">
                            <span className="material-symbols-outlined animate-spin text-[14px] mr-1 align-middle">monitoring</span> Loading event analytics...
                          </div>
                        )}
    
                        {/* Live request method distribution bar chart */}
                        <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-3">
                          <h4 className="text-xs font-bold text-on-surface uppercase tracking-wider">HTTP Method Distribution</h4>
                          {Object.keys(projectUsage.requests_by_method || {}).length === 0 ? (
                            <p className="text-[10px] text-on-surface-variant/60 italic text-center py-2">No API requests recorded yet</p>
                          ) : (
                            <div className="space-y-2">
                              {Object.entries(projectUsage.requests_by_method || {}).map(([method, count]) => {
                                const countNum = Number(count);
                                const maxVal = Math.max(...Object.values(projectUsage.requests_by_method || {}).map(Number), 1);
                                const percent = (countNum / maxVal) * 100;
                                
                                // Method specific colors
                                const barColor = 
                                  method === 'GET' ? 'bg-primary' : 
                                  method === 'POST' ? 'bg-secondary' : 
                                  method === 'DELETE' ? 'bg-error' : 'bg-on-tertiary-container';
                                
                                return (
                                  <div key={method} className="space-y-1">
                                    <div className="flex justify-between text-[10px]">
                                      <span className="font-bold text-on-surface font-mono">{method}</span>
                                      <span className="text-on-surface-variant/80 font-mono">{countNum} requests</span>
                                    </div>
                                    <div className="h-2 w-full bg-surface-container-high rounded-full overflow-hidden">
                                      <div className={`h-full ${barColor} rounded-full`} style={{ width: `${percent}%` }}></div>
                                    </div>
                                  </div>
                                );
                              })}
                            </div>
                          )}
                        </div>
                      </div>
                    ) : (
                      <div className="text-center py-10 bg-surface-container/20 border border-dashed border-outline-variant rounded-xl text-xs text-on-surface-variant/40">
                        <span className="material-symbols-outlined animate-spin text-[16px] mr-1.5 align-middle">sync</span> Loading live usage records...
                      </div>
                    )}
                  </div>
  );
}
