import { api } from '../../../api';
import { useMobileDashboard } from '../context';

export default function MobileTeamPage() {
  const {
    showToast, members, loadingMembers, setActiveModal, setModalData,
    loadMembers, changeMemberRole, projectId
  } = useMobileDashboard();
  return (
                  <div className="space-y-stack-md animate-fade-slide">
                    <div className="flex justify-between items-center">
                      <span className="text-on-surface-variant font-label-caps text-[10px] tracking-widest">{members.length} COLLABORATORS</span>
                      <button 
                        onClick={() => {
                          setModalData({ memberEmail: '', memberRole: 'developer' });
                          setActiveModal('invite_member');
                        }}
                        className="flex items-center gap-1 text-primary hover:bg-primary/10 px-2 py-0.5 rounded text-[9px] font-bold"
                      >
                        <span className="material-symbols-outlined text-[14px]">group_add</span> INVITE MEMBER
                      </button>
                    </div>
    
                    <div className="space-y-3.5">
                      {loadingMembers ? (
                        <div className="text-center py-6 text-on-surface-variant/50 text-xs">
                          <span className="material-symbols-outlined animate-spin text-[16px] mr-1.5 align-middle">sync</span> Loading members...
                        </div>
                      ) : (
                        members.map(member => (
                          <div key={member.user_id} className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl flex justify-between items-center">
                            <div className="flex items-center gap-3 min-w-0">
                              <div className="w-8 h-8 rounded-full bg-secondary/15 flex items-center justify-center text-secondary border border-secondary/20">
                                <span className="text-[10px] font-bold capitalize">{(member.email || 'M').substring(0, 2)}</span>
                              </div>
                              <div className="min-w-0">
                                <p className="text-xs font-bold text-on-surface truncate max-w-[170px]">{member.email}</p>
                                <select
                                  value={member.role}
                                  onChange={(e) => changeMemberRole(member.user_id, e.target.value)}
                                  disabled={member.user_id === (typeof window !== 'undefined' ? localStorage.getItem('sovrabase_admin_user_id') : null)}
                                  className="text-[9px] bg-transparent border border-outline-variant/40 rounded px-1 py-0 text-on-surface-variant font-code-xs capitalize appearance-none cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
                                  style={{ backgroundImage: `url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='8' height='8' viewBox='0 0 24 24' fill='none' stroke='%23999' stroke-width='3' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpolyline points='6 9 12 15 18 9'%3E%3C/polyline%3E%3C/svg%3E")`, backgroundRepeat: 'no-repeat', backgroundPosition: 'right 2px center', paddingRight: '14px' }}
                                >
                                  <option value="owner">owner</option>
                                  <option value="admin">admin</option>
                                  <option value="developer">developer</option>
                                  <option value="viewer">viewer</option>
                                </select>
                              </div>
                            </div>
                            {member.role !== 'owner' && (
                              <button
                                onClick={() => {
                                  if (confirm(`Remove collaborator ${member.email}?`)) {
                                    api(`/admin/projects/${encodeURIComponent(projectId)}/members/${encodeURIComponent(member.user_id)}`, {
                                      method: 'DELETE'
                                    }).then(() => {
                                      showToast('Collaborator removed', 'success');
                                      loadMembers();
                                    }).catch(err => showToast(err.message || 'Failed', 'error'));
                                  }
                                }}
                                className="text-on-surface-variant hover:text-error transition-colors p-1"
                              >
                                <span className="material-symbols-outlined text-[16px]">person_remove</span>
                              </button>
                            )}
                          </div>
                        ))
                      )}
                    </div>
                  </div>
  );
}
