import { api } from '../../../api';
import { useMobileDashboard } from '../context';

export default function MobileIntegrationsPage() {
  const {
    showToast, integrationsCatalog, enabledIntegrations, loadingIntegrations,
    editingIntegration, setEditingIntegration, integrationConfig, setIntegrationConfig, integrationEvents, setIntegrationEvents,
    integrationCollections, setIntegrationCollections, savingIntegration, loadIntegrations, handleSaveIntegration, projectId
  } = useMobileDashboard();
  return (
                  <div className="space-y-stack-md animate-fade-slide">
                    {editingIntegration ? (
                      /* Configuration Editor Form */
                      <div className="bg-surface-container border border-outline-variant p-stack-md rounded-xl space-y-4 animate-fade-slide">
                        <div className="flex justify-between items-center pb-2 border-b border-outline-variant/30">
                          <h3 className="text-xs font-bold text-on-surface uppercase tracking-widest truncate">Configure {editingIntegration.name}</h3>
                          <button onClick={() => setEditingIntegration(null)} className="text-[10px] text-on-surface-variant hover:text-on-surface uppercase font-bold">Cancel</button>
                        </div>
    
                        <div className="space-y-3.5">
                          {editingIntegration.config_fields.map((field: any) => (
                            <div key={field.key}>
                              <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">
                                {field.label} {field.required && <span className="text-error">*</span>}
                              </label>
                              {field.type === 'boolean' ? (
                                <label className="flex items-center gap-2 cursor-pointer mt-1 select-none">
                                  <input 
                                    type="checkbox" 
                                    checked={integrationConfig[field.key] === 'true'}
                                    onChange={(e) => setIntegrationConfig(c => ({ ...c, [field.key]: String(e.target.checked) }))}
                                    className="w-4 h-4 rounded border-outline-variant bg-surface-container-low accent-primary"
                                  />
                                  <span className="text-xs text-on-surface-variant/80">Enable feature</span>
                                </label>
                              ) : (
                                <input 
                                  type={field.type === 'number' ? 'number' : 'text'}
                                  value={integrationConfig[field.key] || ''}
                                  onChange={(e) => setIntegrationConfig(c => ({ ...c, [field.key]: e.target.value }))}
                                  className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                                  placeholder={field.placeholder || `Enter ${field.label.toLowerCase()}`}
                                />
                              )}
                              {field.help_text && (
                                <p className="text-[9px] text-on-surface-variant/60 mt-1 leading-normal">{field.help_text}</p>
                              )}
                            </div>
                          ))}
    
                          {editingIntegration.supports_triggers && (
                            <>
                              <div className="border-t border-outline-variant/30 pt-3">
                                <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1.5">Trigger Database Events</label>
                                <div className="flex gap-4">
                                  {['insert', 'update', 'delete'].map(ev => {
                                    const isChecked = integrationEvents.includes(ev);
                                    return (
                                      <label key={ev} className="flex items-center gap-1.5 cursor-pointer text-xs text-on-surface select-none">
                                        <input 
                                          type="checkbox"
                                          checked={isChecked}
                                          onChange={(e) => {
                                            if (e.target.checked) {
                                              setIntegrationEvents(prev => [...prev, ev]);
                                            } else {
                                              setIntegrationEvents(prev => prev.filter(x => x !== ev));
                                            }
                                          }}
                                          className="w-3.5 h-3.5 rounded border-outline-variant bg-surface-container-low accent-primary"
                                        />
                                        <span className="capitalize">{ev}</span>
                                      </label>
                                    );
                                  })}
                                </div>
                              </div>
    
                              <div>
                                <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1">Filter Collections (Comma separated)</label>
                                <input 
                                  type="text"
                                  value={integrationCollections}
                                  onChange={(e) => setIntegrationCollections(e.target.value)}
                                  className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                                  placeholder="e.g. posts, comments (leave empty for all)"
                                />
                              </div>
                            </>
                          )}
    
                          <div className="pt-2 flex gap-2">
                            {enabledIntegrations.some(e => e.id === editingIntegration.id) && (
                              <button
                                onClick={async () => {
                                  if (confirm('Disable and remove this integration?')) {
                                    try {
                                      const newList = enabledIntegrations.filter(e => e.id !== editingIntegration.id);
                                      await api(`/admin/projects/${encodeURIComponent(projectId)}/integrations`, {
                                        method: 'PUT',
                                        body: JSON.stringify({ integrations: newList })
                                      });
                                      showToast('Integration removed', 'success');
                                      setEditingIntegration(null);
                                      loadIntegrations();
                                    } catch {
                                      showToast('Failed to remove integration', 'error');
                                    }
                                  }
                                }}
                                className="bg-surface-container-low border border-error/30 text-error px-4 py-2 rounded-lg text-xs font-bold active:scale-[0.98] transition-all"
                              >
                                Disable
                              </button>
                            )}
                            <button 
                              onClick={handleSaveIntegration}
                              disabled={savingIntegration}
                              className="flex-1 bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-[0.98] transition-all disabled:opacity-50"
                            >
                              {savingIntegration ? 'Saving...' : 'Save Integration'}
                            </button>
                          </div>
                        </div>
                      </div>
                    ) : (
                      /* Catalog List */
                      <div className="space-y-3">
                        <p className="text-on-surface-variant font-label-caps text-[10px] tracking-widest">THIRD PARTY SERVICES CATALOG</p>
                        <div className="space-y-3.5">
                          {loadingIntegrations ? (
                            <div className="text-center py-8 text-on-surface-variant/50 text-xs">
                              <span className="material-symbols-outlined animate-spin text-[16px] mr-1.5 align-middle">sync</span> Loading integrations catalog...
                            </div>
                          ) : integrationsCatalog.length === 0 ? (
                            <p className="text-xs text-on-surface-variant/50 italic text-center py-6">No integrations available.</p>
                          ) : (
                            integrationsCatalog.map(plugin => {
                              const config = enabledIntegrations.find(e => e.id === plugin.id);
                              const isEnabled = !!config;
                              
                              return (
                                <div 
                                  key={plugin.id} 
                                  onClick={() => {
                                    setEditingIntegration(plugin);
                                    const cfg: Record<string, string> = {};
                                    if (config?.config) {
                                      for (const [k, v] of Object.entries(config.config)) {
                                        cfg[k] = String(v ?? '');
                                      }
                                    }
                                    setIntegrationConfig(cfg);
                                    setIntegrationEvents(config?.events ? [...config.events] : []);
                                    setIntegrationCollections(config?.collections ? config.collections.join(', ') : '');
                                  }}
                                  className="w-full text-left bg-surface-container-low border border-outline-variant hover:bg-surface-container-high/40 p-stack-md rounded-xl flex items-center justify-between transition-colors cursor-pointer"
                                >
                                  <div className="flex items-center gap-3">
                                    <span className="w-9 h-9 flex items-center justify-center rounded-lg border border-outline-variant/50 bg-surface-container-lowest text-primary">
                                      <span className="material-symbols-outlined text-[20px]">extension</span>
                                    </span>
                                    <div>
                                      <h4 className="text-xs font-bold text-on-surface">{plugin.name}</h4>
                                      <p className="text-[9px] text-on-surface-variant/75 truncate max-w-[190px]">{plugin.description}</p>
                                    </div>
                                  </div>
                                  <span className={`text-[9px] font-bold px-2 py-0.5 rounded-full border shrink-0 ml-1 ${
                                    isEnabled 
                                      ? 'bg-primary/10 border-primary/20 text-primary' 
                                      : 'bg-surface-container text-on-surface-variant/50 border-outline-variant/30'
                                  }`}>
                                    {isEnabled ? 'Linked' : 'Link'}
                                  </span>
                                </div>
                              );
                            })
                          )}
                        </div>
                      </div>
                    )}
                  </div>
  );
}
