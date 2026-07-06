import { formatBytes } from '../../../api';
import { useMobileDashboard } from '../context';

export default function MobileOverviewTab() {
  const {
    stats, setActiveTab, collections, users, setActiveSubPage,
    projectDetail, projectUsage, dbAnalysis, analyzing, runDbAnalysis, projectId, projectName
  } = useMobileDashboard();
  return (
    <div className="p-container-padding space-y-stack-lg animate-fade-slide">
                {/* Project Quick Info */}
                <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-3 relative overflow-hidden emerald-glow">
                  <div className="flex justify-between items-start">
                    <div>
                      <h2 className="text-base font-extrabold text-on-surface">{projectName}</h2>
                      <p className="text-xs text-on-surface-variant/70 font-code-xs select-text">ID: {projectId}</p>
                    </div>
                    <span className="px-2 py-0.5 bg-primary/10 text-primary-fixed-dim font-code-xs text-[10px] border border-primary/20 rounded">
                      {projectDetail?.status || 'Active'}
                    </span>
                  </div>
                  <div className="border-t border-outline-variant/30 pt-3 flex flex-col gap-2">
                    <div className="flex justify-between text-xs">
                      <span className="text-on-surface-variant/70">Database API</span>
                      <span className="font-code-xs text-primary truncate max-w-[180px] text-right select-text">/api/v1/collections/</span>
                    </div>
                    <div className="flex justify-between text-xs">
                      <span className="text-on-surface-variant/70">Region</span>
                      <span className="font-code-xs uppercase text-secondary">{stats?.region || 'eu-west'}</span>
                    </div>
                  </div>
                </div>
    
                {/* Bento Statistics */}
                <div className="grid grid-cols-2 gap-stack-sm">
                  <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-2">
                    <p className="font-label-caps text-[10px] text-on-surface-variant uppercase tracking-wider">Collections</p>
                    <p className="font-headline-lg text-xl font-bold text-primary">{collections.length}</p>
                    <p className="font-code-xs text-[10px] text-on-surface-variant/60">JSON collections</p>
                  </div>
                  <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-2">
                    <p className="font-label-caps text-[10px] text-on-surface-variant uppercase tracking-wider">Registered Users</p>
                    <p className="font-headline-lg text-xl font-bold text-secondary">{users.length}</p>
                    <p className="font-code-xs text-[10px] text-primary">Auth database</p>
                  </div>
                </div>
    
                {/* Live Metrics from projectUsage */}
                <div className="space-y-3.5">
                  <h3 className="font-label-caps text-[10px] text-on-surface-variant uppercase tracking-widest">Live Metrics</h3>
                  <div className="grid grid-cols-2 gap-stack-sm">
                    <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-1">
                      <span className="text-[8px] font-bold text-on-surface-variant uppercase block">API Requests</span>
                      <span className="text-md font-bold text-primary font-mono">{projectUsage?.api_requests || 0}</span>
                      <span className="text-[8px] text-on-surface-variant/50">Hits since creation</span>
                    </div>
                    <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-1">
                      <span className="text-[8px] font-bold text-on-surface-variant uppercase block">DB Reads</span>
                      <span className="text-md font-bold text-primary font-mono">{projectUsage?.db_reads_total || 0}</span>
                      <span className="text-[8px] text-on-surface-variant/50">Reads from disk</span>
                    </div>
                    <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-1">
                      <span className="text-[8px] font-bold text-on-surface-variant uppercase block">DB Writes</span>
                      <span className="text-md font-bold text-secondary font-mono">{projectUsage?.db_writes_total || 0}</span>
                      <span className="text-[8px] text-on-surface-variant/50">Writes to disk</span>
                    </div>
                    <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-1">
                      <span className="text-[8px] font-bold text-on-surface-variant uppercase block">Realtime Conn.</span>
                      <span className="text-md font-bold text-secondary font-mono">{projectUsage?.realtime_connections || 0}</span>
                      <span className="text-[8px] text-on-surface-variant/50">Active WebSockets</span>
                    </div>
                  </div>
                  {!projectUsage && (
                    <p className="text-[10px] text-on-surface-variant/50 italic text-center">Loading metrics...</p>
                  )}
                </div>
    
                {/* Deep DB Analysis */}
                <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-3">
                  <div className="flex items-center justify-between">
                    <h3 className="font-label-caps text-[10px] text-on-surface-variant uppercase tracking-widest">Storage Analysis</h3>
                    <button
                      onClick={runDbAnalysis}
                      disabled={analyzing}
                      className="flex items-center gap-1 px-2.5 py-1 rounded text-[10px] font-bold uppercase tracking-wider bg-primary/10 text-primary border border-primary/20 hover:bg-primary/20 active:scale-95 transition-all disabled:opacity-50"
                    >
                      {analyzing ? (
                        <>
                          <span className="w-3 h-3 border-2 border-primary border-t-transparent rounded-full animate-spin" />
                          Analyzing...
                        </>
                      ) : (
                        'Run Analysis'
                      )}
                    </button>
                  </div>
                  {dbAnalysis && (
                    <div className="space-y-2">
                      <div className="flex items-center gap-2 text-on-surface-variant text-[10px] font-mono">
                        Total: <span className="text-primary font-bold">{formatBytes(dbAnalysis.total_size)}</span>
                      </div>
                      {dbAnalysis.collections && Object.keys(dbAnalysis.collections).length > 0 && (
                        <div className="space-y-1.5">
                          {Object.entries(dbAnalysis.collections).slice(0, 5).map(([name, c]: [string, any]) => (
                            <div key={name} className="flex justify-between items-center bg-surface-container-high rounded px-2 py-1">
                              <span className="text-[10px] font-mono text-on-surface truncate max-w-[120px]">{name}</span>
                              <div className="flex gap-2 text-[9px] font-mono">
                                <span className="text-on-surface-variant/70">{c.document_count?.toLocaleString()} docs</span>
                                <span className="text-primary">{formatBytes(c.document_size)}</span>
                              </div>
                            </div>
                          ))}
                          {Object.keys(dbAnalysis.collections).length > 5 && (
                            <p className="text-[9px] text-on-surface-variant/50 text-center italic">
                              +{Object.keys(dbAnalysis.collections).length - 5} more collections
                            </p>
                          )}
                        </div>
                      )}
                      <div className="flex gap-3 text-[9px] font-mono text-on-surface-variant/60">
                        <span>Metadata: {formatBytes(dbAnalysis.metadata_size)}</span>
                        <span>Index: {formatBytes(dbAnalysis.index_size)}</span>
                      </div>
                    </div>
                  )}
                  {!dbAnalysis && !analyzing && (
                    <p className="text-[9px] text-on-surface-variant/50 italic">Tap "Run Analysis" to scan database storage</p>
                  )}
                </div>
    
                {/* Quick action button */}
                <button 
                  onClick={() => { setActiveTab('more'); setActiveSubPage('api'); }}
                  className="w-full bg-primary text-on-primary py-3 rounded-lg font-label-caps text-xs uppercase tracking-widest hover:brightness-110 transition-all active:scale-[0.98] font-bold flex justify-center items-center gap-2"
                >
                  <span>View API Docs</span>
                  <span className="material-symbols-outlined text-sm">arrow_forward</span>
                </button>
              </div>
  );
}
