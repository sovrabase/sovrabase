import { api } from '../../../api';
import { ProviderIcon } from '../ProviderIcon';
import { useMobileDashboard } from '../context';

export default function MobileAuthTab() {
  const {
    showToast, collections, authSubTab, setAuthSubTab, swipedUserId,
    setSwipedUserId, userSearchQuery, setUserSearchQuery, isInviting, setIsInviting, inviteEmail,
    setInviteEmail, editingProvider, setEditingProvider, providerConfig, setProviderConfig, authProviders,
    editingPolicyCol, setEditingPolicyCol, policyForm, setPolicyForm, loadAuthProviders, handleBanUser,
    handleResetUser, handleInviteUser, handleSaveProvider, handleSelectPolicyCol, handleSavePolicies, filteredUsers,
    projectId
  } = useMobileDashboard();
  return (
    <div className="p-container-padding space-y-stack-md animate-fade-slide">
                
                {/* Multi-Tab Filter Navigation */}
                <div className="flex border-b border-outline-variant/30 gap-6">
                  {[
                    { id: 'users', label: 'USERS' },
                    { id: 'providers', label: 'PROVIDERS' },
                    { id: 'policies', label: 'POLICIES' }
                  ].map(sub => (
                    <button
                      key={sub.id}
                      onClick={() => {
                        setAuthSubTab(sub.id as any);
                        setSwipedUserId(null);
                      }}
                      className={`relative pb-2.5 text-xs font-bold font-label-caps tracking-wider transition-colors focus:outline-none ${
                        authSubTab === sub.id ? 'text-primary' : 'text-on-surface-variant hover:text-on-surface'
                      }`}
                    >
                      {sub.label}
                      {authSubTab === sub.id && (
                        <div className="active-tab-indicator absolute bottom-0 left-0 right-0"></div>
                      )}
                    </button>
                  ))}
                </div>
    
                {/* USERS SUB-TAB */}
                {authSubTab === 'users' && (
                  <div className="space-y-stack-md animate-fade-slide">
                    <div className="flex justify-between items-center">
                      <span className="text-on-surface-variant font-label-caps text-[10px] tracking-widest">{filteredUsers.length} TOTAL USERS</span>
                      <button 
                        onClick={() => setIsInviting(!isInviting)}
                        className="flex items-center gap-1 text-primary hover:bg-primary/10 px-2 py-1 rounded transition-colors"
                      >
                        <span className="material-symbols-outlined text-[16px]">person_add</span>
                        <span className="font-label-caps text-[10px] font-bold">INVITE</span>
                      </button>
                    </div>
    
                    {/* Invite form widget */}
                    {isInviting && (
                      <div className="bg-surface-container border border-outline-variant p-stack-md rounded-xl space-y-2.5 animate-fade-slide">
                        <p className="text-xs font-bold text-on-surface">Invite User to Project</p>
                        <div className="flex gap-2">
                          <input 
                            type="email"
                            placeholder="user@example.com"
                            value={inviteEmail}
                            onChange={(e) => setInviteEmail(e.target.value)}
                            className="flex-1 bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface placeholder-on-surface-variant/40 focus:outline-none focus:border-primary/50"
                          />
                          <button 
                            onClick={handleInviteUser}
                            className="bg-primary text-on-primary px-3 py-1.5 rounded-lg text-xs font-bold active:scale-95"
                          >
                            Send
                          </button>
                        </div>
                      </div>
                    )}
    
                    {/* User search bar */}
                    <div className="relative">
                      <span className="absolute inset-y-0 left-0 pl-2.5 flex items-center pointer-events-none">
                        <span className="material-symbols-outlined text-[16px] text-on-surface-variant/50">search</span>
                      </span>
                      <input
                        type="text"
                        placeholder="Search users..."
                        value={userSearchQuery}
                        onChange={(e) => setUserSearchQuery(e.target.value)}
                        className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg pl-8 pr-3 py-1.5 text-xs text-on-surface placeholder-on-surface-variant/40 focus:outline-none focus:border-primary/50"
                      />
                    </div>
    
                    {/* Users List with Swipe micro-interactions */}
                    <div className="space-y-2">
                      {filteredUsers.map((user) => {
                        const isSwiped = swipedUserId === user.id;
                        
                        return (
                          <div 
                            key={user.id}
                            className="relative overflow-hidden rounded-xl border border-outline-variant bg-surface-container-low group"
                          >
                            {/* Swipe Actions Background (BAN and RESET) */}
                            <div className="absolute inset-0 flex justify-end z-0">
                              <button 
                                onClick={() => handleResetUser(user.id, user.email)}
                                className="bg-secondary-container text-on-secondary-container w-24 flex flex-col items-center justify-center gap-1 active:bg-secondary-container/85"
                              >
                                <span className="material-symbols-outlined text-[20px]">lock_reset</span>
                                <span className="text-[9px] font-extrabold uppercase tracking-widest font-label-caps">RESET</span>
                              </button>
                              <button 
                                onClick={() => handleBanUser(user.id, user.email)}
                                className="bg-error-container text-on-error-container w-24 flex flex-col items-center justify-center gap-1 active:bg-error-container/85"
                              >
                                <span className="material-symbols-outlined text-[20px]">block</span>
                                <span className="text-[9px] font-extrabold uppercase tracking-widest font-label-caps">BAN</span>
                              </button>
                            </div>
    
                            {/* User Foreground Card */}
                            <div 
                              onClick={() => setSwipedUserId(isSwiped ? null : user.id)}
                              className="relative bg-surface-container-low p-stack-md flex items-center justify-between cursor-pointer transition-transform duration-300 z-10"
                              style={{
                                transform: isSwiped ? 'translateX(-192px)' : 'translateX(0px)',
                                borderLeft: user.status === 'Banned' ? '3px solid #ffb4ab' : 'none'
                              }}
                            >
                              <div className="flex items-center gap-stack-md">
                                {/* User Avatar */}
                                <div className="w-10 h-10 rounded-full border border-outline-variant overflow-hidden flex items-center justify-center bg-surface-container-high shrink-0">
                                  {user.avatar_url ? (
                                    <img src={user.avatar_url} className="w-full h-full object-cover" alt="Avatar" />
                                  ) : (
                                    <span className="text-on-surface-variant font-bold text-xs uppercase">
                                      {(user.name || user.email || 'U').substring(0, 2)}
                                    </span>
                                  )}
                                </div>
                                
                                {/* User Details */}
                                <div className="flex flex-col min-w-0">
                                  <span className="text-xs font-bold text-on-surface truncate max-w-[130px] sm:max-w-[180px]">
                                    {user.email}
                                  </span>
                                  <span className="text-on-surface-variant text-[10px] font-code-xs tracking-wider uppercase flex items-center gap-1.5 mt-0.5">
                                    ID: {user.id.substring(0, 8)}
                                    {user.status === 'Banned' && (
                                      <span className="bg-error-container/20 text-error px-1 rounded text-[8px] font-bold">BANNED</span>
                                    )}
                                    {user.status === 'Pending' && (
                                      <span className="bg-secondary-container/20 text-secondary px-1 rounded text-[8px] font-bold">PENDING</span>
                                    )}
                                  </span>
                                </div>
                              </div>
    
                              <div className="flex items-center gap-2">
                                <span className="material-symbols-outlined text-on-surface-variant/60 text-[18px]">
                                  {user.provider === 'google' ? 'google' : user.provider === 'code' ? 'code' : 'mail'}
                                </span>
                                <div className={`w-2 h-2 rounded-full ${user.status === 'Banned' ? 'bg-error shadow-[0_0_8px_#ffb4ab]' : user.status === 'Pending' ? 'bg-outline-variant' : 'bg-primary shadow-[0_0_8px_#00d2ff]'}`}></div>
                              </div>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                )}
    
                {/* PROVIDERS SUB-TAB */}
                {authSubTab === 'providers' && (
                  <div className="space-y-stack-md animate-fade-slide">
                    {editingProvider ? (
                      /* Provider Configuration Form */
                      <div className="bg-surface-container border border-outline-variant p-stack-md rounded-xl space-y-4 animate-fade-slide">
                        <div className="flex justify-between items-center pb-2 border-b border-outline-variant/30">
                          <h3 className="text-xs font-bold text-on-surface uppercase tracking-widest truncate">Configure {editingProvider === 'custom' ? 'Custom Provider' : editingProvider}</h3>
                          <button onClick={() => setEditingProvider(null)} className="text-[10px] text-on-surface-variant hover:text-on-surface uppercase font-bold">Cancel</button>
                        </div>
                        
                        <div className="space-y-3">
                          <div>
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1">Status</label>
                            <label className="flex items-center gap-2 cursor-pointer">
                              <input 
                                type="checkbox" 
                                checked={providerConfig.enabled}
                                onChange={(e) => setProviderConfig(p => ({ ...p, enabled: e.target.checked }))}
                                className="w-4 h-4 rounded border-outline-variant bg-surface-container-low accent-primary"
                              />
                              <span className="text-xs text-on-surface">Enable Provider Integration</span>
                            </label>
                          </div>
    
                          {editingProvider === 'custom' && (
                            <div>
                              <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1">Provider ID / Name</label>
                              <input 
                                type="text"
                                value={providerConfig.name || ''}
                                onChange={(e) => setProviderConfig(p => ({ ...p, name: e.target.value }))}
                                className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                                placeholder="e.g. keycloak"
                              />
                            </div>
                          )}
    
                          <div>
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1">Client ID</label>
                            <input 
                              type="text"
                              value={providerConfig.client_id}
                              onChange={(e) => setProviderConfig(p => ({ ...p, client_id: e.target.value }))}
                              className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                              placeholder="Enter client identifier"
                            />
                          </div>
    
                          <div>
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1">Client Secret / API Key</label>
                            <input 
                              type="password"
                              value={providerConfig.client_secret}
                              onChange={(e) => setProviderConfig(p => ({ ...p, client_secret: e.target.value }))}
                              className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                              placeholder="••••••••••••••••"
                            />
                          </div>
    
                          {editingProvider === 'custom' && (
                            <>
                              <div>
                                <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1">Authorization URL</label>
                                <input 
                                  type="text"
                                  value={providerConfig.auth_url || ''}
                                  onChange={(e) => setProviderConfig(p => ({ ...p, auth_url: e.target.value }))}
                                  className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                                  placeholder="https://example.com/oauth/authorize"
                                />
                              </div>
                              <div>
                                <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1">Token URL</label>
                                <input 
                                  type="text"
                                  value={providerConfig.token_url || ''}
                                  onChange={(e) => setProviderConfig(p => ({ ...p, token_url: e.target.value }))}
                                  className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                                  placeholder="https://example.com/oauth/token"
                                />
                              </div>
                              <div>
                                <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1">User Info URL</label>
                                <input 
                                  type="text"
                                  value={providerConfig.userinfo_url || ''}
                                  onChange={(e) => setProviderConfig(p => ({ ...p, userinfo_url: e.target.value }))}
                                  className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                                  placeholder="https://example.com/oauth/userinfo"
                                />
                              </div>
                            </>
                          )}
    
                          <div>
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1">Scopes (Comma separated)</label>
                            <input 
                              type="text"
                              value={Array.isArray(providerConfig.scopes) ? providerConfig.scopes.join(', ') : providerConfig.scopes}
                              onChange={(e) => setProviderConfig(p => ({ ...p, scopes: e.target.value }))}
                              className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                              placeholder="e.g. email, profile"
                            />
                          </div>
    
                          <div className="pt-2">
                            <button 
                              onClick={handleSaveProvider}
                              className="w-full bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-98 transition-all"
                            >
                              Save Credentials
                            </button>
                          </div>
                        </div>
                      </div>
                    ) : (
                      /* Providers List */
                      <div className="space-y-3">
                        <p className="text-on-surface-variant font-label-caps text-[10px] tracking-widest">ACTIVE AUTHENTICATION METHODS</p>
                        <div className="space-y-3.5">
                          {/* Predefined Templates */}
                          {['google', 'github', 'discord', 'gitlab', 'microsoft', 'facebook', 'twitter'].map(type => {
                            const config = authProviders.find(p => p.name === type);
                            const isEnabled = config?.enabled || false;
                            
                            return (
                              <button 
                                key={type} 
                                onClick={() => {
                                  setEditingProvider(type);
                                  setProviderConfig({
                                    client_id: config?.client_id || '',
                                    client_secret: config?.client_secret || '',
                                    scopes: config?.scopes || [],
                                    enabled: isEnabled,
                                    auth_url: config?.auth_url || '',
                                    token_url: config?.token_url || '',
                                    userinfo_url: config?.userinfo_url || ''
                                  });
                                }}
                                className="w-full text-left bg-surface-container-low border border-outline-variant hover:bg-surface-container-high/40 p-stack-md rounded-xl flex items-center justify-between transition-colors focus:outline-none"
                              >
                                <div className="flex items-center gap-3">
                                  <span className="w-8 h-8 flex items-center justify-center rounded-lg border border-outline-variant/40 bg-surface-container-lowest text-on-surface-variant shrink-0">
                                    {<ProviderIcon name={type} className="w-4 h-4" />}
                                  </span>
                                  <div>
                                    <p className="text-xs font-bold text-on-surface capitalize">{type} OAuth</p>
                                    <p className="text-[9px] text-on-surface-variant/70 font-mono truncate max-w-[170px]">
                                      {isEnabled ? (config.client_id || 'Configured') : 'Click to configure integration'}
                                    </p>
                                  </div>
                                </div>
                                <span className={`text-[9px] font-bold px-2 py-0.5 rounded-full border ${
                                  isEnabled 
                                    ? 'bg-primary/10 border-primary/20 text-primary' 
                                    : 'bg-surface-container text-on-surface-variant/50 border-outline-variant/30'
                                }`}>
                                  {isEnabled ? 'Active' : 'Disabled'}
                                </span>
                              </button>
                            );
                          })}
    
                          {/* Custom Providers */}
                          {authProviders.filter(p => !['google', 'github', 'discord', 'gitlab', 'microsoft', 'facebook', 'twitter'].includes(p.name)).map(p => (
                            <div 
                              key={p.name}
                              onClick={() => {
                                setEditingProvider('custom');
                                setProviderConfig({
                                  name: p.name,
                                  client_id: p.client_id || '',
                                  client_secret: p.client_secret || '',
                                  scopes: p.scopes || [],
                                  enabled: p.enabled || false,
                                  auth_url: p.auth_url || '',
                                  token_url: p.token_url || '',
                                  userinfo_url: p.userinfo_url || ''
                                });
                              }}
                              className="w-full text-left bg-surface-container-low border border-outline-variant hover:bg-surface-container-high/40 p-stack-md rounded-xl flex items-center justify-between transition-colors cursor-pointer"
                            >
                              <div className="flex items-center gap-3">
                                <span className="w-8 h-8 flex items-center justify-center rounded-lg border border-outline-variant/40 bg-surface-container-lowest text-on-surface-variant shrink-0">
                                  {<ProviderIcon name={p.name} className="w-4 h-4" />}
                                </span>
                                <div>
                                  <p className="text-xs font-bold text-on-surface truncate max-w-[140px] capitalize">{p.name} (Custom)</p>
                                  <p className="text-[9px] text-on-surface-variant/70 font-mono truncate max-w-[170px]">
                                    {p.enabled ? (p.client_id || 'Configured') : 'Disabled'}
                                  </p>
                                </div>
                              </div>
                              <div className="flex items-center gap-2">
                                <span className={`text-[9px] font-bold px-2 py-0.5 rounded-full border ${
                                  p.enabled 
                                    ? 'bg-secondary-container/20 border-secondary text-secondary' 
                                    : 'bg-surface-container text-on-surface-variant/50 border-outline-variant/30'
                                }`}>
                                  {p.enabled ? 'Active' : 'Disabled'}
                                </span>
                                <button
                                  onClick={async (e) => {
                                    e.stopPropagation();
                                    if (confirm(`Delete custom identity provider "${p.name}"?`)) {
                                      try {
                                        const newList = authProviders.filter(x => x.name !== p.name);
                                        await api(`/admin/projects/${encodeURIComponent(projectId)}/auth/providers`, {
                                          method: 'PUT',
                                          body: JSON.stringify({ providers: newList })
                                        });
                                        showToast('Provider deleted', 'success');
                                        loadAuthProviders();
                                      } catch {
                                        showToast('Delete failed', 'error');
                                      }
                                    }
                                  }}
                                  className="text-on-surface-variant hover:text-error transition-colors p-1"
                                >
                                  <span className="material-symbols-outlined text-[16px]">delete</span>
                                </button>
                              </div>
                            </div>
                          ))}
                        </div>
    
                        <div className="pt-2">
                          <button 
                            onClick={() => {
                              setEditingProvider('custom');
                              setProviderConfig({
                                name: '',
                                client_id: '',
                                client_secret: '',
                                scopes: '',
                                enabled: true,
                                auth_url: '',
                                token_url: '',
                                userinfo_url: ''
                              });
                            }}
                            className="w-full bg-surface-container border border-dashed border-outline-variant/60 hover:bg-surface-container-high/40 p-3 rounded-xl flex items-center justify-center gap-1.5 text-xs text-primary font-bold transition-all focus:outline-none"
                          >
                            <span className="material-symbols-outlined text-[16px]">add_circle</span>
                            <span>Configure Custom Provider</span>
                          </button>
                        </div>
                      </div>
                    )}
                  </div>
                )}
    
                {/* POLICIES SUB-TAB */}
                {authSubTab === 'policies' && (
                  <div className="space-y-stack-md animate-fade-slide">
                    {editingPolicyCol ? (
                      /* RLS Rules Editor for selected collection */
                      <div className="bg-surface-container border border-outline-variant p-stack-md rounded-xl space-y-4 animate-fade-slide">
                        <div className="flex justify-between items-center pb-2 border-b border-outline-variant/30">
                          <h3 className="text-xs font-bold text-on-surface uppercase tracking-widest truncate max-w-[180px]">RLS: {editingPolicyCol}</h3>
                          <button onClick={() => setEditingPolicyCol(null)} className="text-[10px] text-on-surface-variant hover:text-on-surface uppercase font-bold">Cancel</button>
                        </div>
    
                        <div className="space-y-3">
                          <div>
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1">Row-Level Security</label>
                            <label className="flex items-center gap-2 cursor-pointer">
                              <input 
                                type="checkbox" 
                                checked={policyForm.enabled}
                                onChange={(e) => setPolicyForm(p => ({ ...p, enabled: e.target.checked }))}
                                className="w-4 h-4 rounded border-outline-variant bg-surface-container-low accent-primary"
                              />
                              <span className="text-xs text-on-surface">Enable Row Level Security (RLS)</span>
                            </label>
                          </div>
    
                          {/* Rule inputs for GET, LIST, CREATE, UPDATE, DELETE */}
                          {['get', 'list', 'create', 'update', 'delete'].map(action => (
                            <div key={action} className="space-y-1">
                              <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider font-mono">{action.toUpperCase()} Rule</label>
                              <input 
                                type="text"
                                value={(policyForm as any)[action]}
                                disabled={!policyForm.enabled}
                                onChange={(e) => setPolicyForm(p => ({ ...p, [action]: e.target.value }))}
                                placeholder="e.g. auth != null"
                                className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono disabled:opacity-40 disabled:cursor-not-allowed"
                              />
                            </div>
                          ))}
    
                          <div className="pt-2">
                            <button 
                              onClick={handleSavePolicies}
                              className="w-full bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-98 transition-all"
                            >
                              Save RLS Policies
                            </button>
                          </div>
                        </div>
                      </div>
                    ) : (
                      /* Collections Selector to view policies */
                      <div className="space-y-3">
                        <p className="text-on-surface-variant font-label-caps text-[10px] tracking-widest">COLLECTIONS SECURITY CONFIG</p>
                        <div className="space-y-2">
                          {collections.length === 0 ? (
                            <div className="text-center py-8 bg-surface-container-low/40 rounded-xl border border-dashed border-outline-variant/30">
                              <span className="material-symbols-outlined text-3xl text-on-surface-variant/30 block mb-1">database</span>
                              <p className="text-xs text-on-surface-variant/50">No collections available</p>
                            </div>
                          ) : (
                            collections.map(col => (
                              <button
                                key={col.name}
                                onClick={() => handleSelectPolicyCol(col.name)}
                                className="w-full text-left bg-surface-container-low border border-outline-variant hover:bg-surface-container-high/40 p-stack-md rounded-xl flex items-center justify-between transition-colors focus:outline-none"
                              >
                                <div className="flex items-center gap-2">
                                  <span className="material-symbols-outlined text-primary text-sm">lock</span>
                                  <span className="text-xs font-bold text-on-surface font-mono">{col.name}</span>
                                </div>
                                <span className="text-[9px] text-on-surface-variant font-semibold">
                                  Configure policies
                                </span>
                              </button>
                            ))
                          )}
                        </div>
                      </div>
                    )}
                  </div>
                )}
    
              </div>
  );
}
