import { api } from '../../../api';
import { useMobileDashboard } from '../context';

export default function MobileModals() {
  const {
    activeModal, modalData, setModalData, setActiveModal, selectedCol, setSelectedCol, selectedBucket,
    showToast, loadBuckets, loadFiles, loadMembers, loadWebhooks, loadCronJobs,
    loadDocuments, loadCollections, loadConfigEntries, projectId,
  } = useMobileDashboard();
  if (!activeModal) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-fade-in">
      <div className="bg-surface-container border border-outline-variant rounded-2xl p-6 w-full max-w-sm space-y-4 shadow-2xl animate-fade-slide">
        {activeModal === 'create_bucket' && (
                      <>
                        <h3 className="text-sm font-bold text-on-surface uppercase tracking-wider">Create Storage Bucket</h3>
                        <div className="space-y-3">
                          <div>
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Bucket Name</label>
                            <input 
                              type="text"
                              value={modalData.bucketName || ''}
                              onChange={(e) => setModalData(d => ({ ...d, bucketName: e.target.value }))}
                              className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                              placeholder="e.g. assets"
                            />
                          </div>
                          <div className="flex gap-2 pt-2">
                            <button onClick={() => setActiveModal(null)} className="flex-1 bg-surface-container-low border border-outline-variant/60 text-on-surface-variant py-2 rounded-lg text-xs font-bold active:scale-95 transition-all">Cancel</button>
                            <button 
                              onClick={async () => {
                                const name = modalData.bucketName?.trim();
                                if (!name) return;
                                try {
                                  await api(`/admin/projects/${encodeURIComponent(projectId)}/storage/buckets`, {
                                    method: 'POST',
                                    body: JSON.stringify({ name })
                                  });
                                  showToast('Bucket created successfully', 'success');
                                  loadBuckets();
                                  setActiveModal(null);
                                } catch {
                                  showToast('Failed to create bucket', 'error');
                                }
                              }} 
                              className="flex-1 bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-95 transition-all"
                            >
                              Create
                            </button>
                          </div>
                        </div>
                      </>
                    )}
        
                    {activeModal === 'upload_file' && (
                      <>
                        <h3 className="text-sm font-bold text-on-surface uppercase tracking-wider">Create Test File</h3>
                        <div className="space-y-3">
                          <div>
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Filename</label>
                            <input 
                              type="text"
                              value={modalData.fileName || ''}
                              onChange={(e) => setModalData(d => ({ ...d, fileName: e.target.value }))}
                              className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                              placeholder="e.g. hello.txt"
                            />
                          </div>
                          <div>
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Content</label>
                            <textarea 
                              value={modalData.fileContent || ''}
                              onChange={(e) => setModalData(d => ({ ...d, fileContent: e.target.value }))}
                              className="w-full h-24 bg-surface-container-low border border-outline-variant/60 rounded-lg p-2 text-xs text-on-surface font-mono"
                              placeholder="Hello Sovrabase storage!"
                            />
                          </div>
                          <div className="flex gap-2 pt-2">
                            <button onClick={() => setActiveModal(null)} className="flex-1 bg-surface-container-low border border-outline-variant/60 text-on-surface-variant py-2 rounded-lg text-xs font-bold active:scale-95 transition-all">Cancel</button>
                            <button 
                              onClick={async () => {
                                const name = modalData.fileName?.trim();
                                const content = modalData.fileContent || '';
                                if (!name || !selectedBucket) return;
                                try {
                                  const formData = new FormData();
                                  formData.append('file', new File([content], name, { type: 'text/plain' }));
                                  await api(`/admin/projects/${encodeURIComponent(projectId)}/storage/buckets/${encodeURIComponent(selectedBucket)}/files`, {
                                    method: 'POST',
                                    body: formData
                                  });
                                  showToast('File uploaded successfully', 'success');
                                  loadFiles(selectedBucket);
                                  setActiveModal(null);
                                } catch {
                                  showToast('Upload failed', 'error');
                                }
                              }} 
                              className="flex-1 bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-95 transition-all"
                            >
                              Upload
                            </button>
                          </div>
                        </div>
                      </>
                    )}
        
                    {activeModal === 'invite_member' && (
                      <>
                        <h3 className="text-sm font-bold text-on-surface uppercase tracking-wider">Invite Team Member</h3>
                        <div className="space-y-3">
                          <div>
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Email Address</label>
                            <input 
                              type="email"
                              value={modalData.memberEmail || ''}
                              onChange={(e) => setModalData(d => ({ ...d, memberEmail: e.target.value }))}
                              className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                              placeholder="e.g. developer@company.com"
                            />
                          </div>
                          <div>
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Role</label>
                            <select
                              value={modalData.memberRole || 'developer'}
                              onChange={(e) => setModalData(d => ({ ...d, memberRole: e.target.value }))}
                              className="w-full bg-surface-container-low border border-outline-variant/60 text-on-surface px-2.5 py-1.5 text-xs font-mono rounded-lg outline-none focus:border-primary/50"
                            >
                              <option value="owner">Owner</option>
                              <option value="admin">Admin</option>
                              <option value="developer">Developer</option>
                              <option value="member">Member</option>
                            </select>
                          </div>
                          <div className="flex gap-2 pt-2">
                            <button onClick={() => setActiveModal(null)} className="flex-1 bg-surface-container-low border border-outline-variant/60 text-on-surface-variant py-2 rounded-lg text-xs font-bold active:scale-95 transition-all">Cancel</button>
                            <button 
                              onClick={async () => {
                                const email = modalData.memberEmail?.trim();
                                const role = modalData.memberRole || 'developer';
                                if (!email) return;
                                try {
                                  await api(`/admin/projects/${encodeURIComponent(projectId)}/invite`, {
                                    method: 'POST',
                                    body: JSON.stringify({ email, role })
                                  });
                                  showToast('Invitation sent successfully', 'success');
                                  loadMembers();
                                  setActiveModal(null);
                                } catch {
                                  showToast('Failed to invite member', 'error');
                                }
                              }} 
                              className="flex-1 bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-95 transition-all"
                            >
                              Invite
                            </button>
                          </div>
                        </div>
                      </>
                    )}
        
                    {activeModal === 'add_webhook' && (
                      <>
                        <h3 className="text-sm font-bold text-on-surface uppercase tracking-wider">Add Webhook Endpoint</h3>
                        <div className="space-y-3">
                          <div>
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Callback URL</label>
                            <input 
                              type="url"
                              value={modalData.webhookUrl || ''}
                              onChange={(e) => setModalData(d => ({ ...d, webhookUrl: e.target.value }))}
                              className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                              placeholder="https://api.my-app.com/webhooks"
                            />
                          </div>
                          <div>
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Events (Comma separated)</label>
                            <input 
                              type="text"
                              value={modalData.webhookEvents || 'insert,update,delete'}
                              onChange={(e) => setModalData(d => ({ ...d, webhookEvents: e.target.value }))}
                              className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                            />
                          </div>
                          <div className="flex gap-2 pt-2">
                            <button onClick={() => setActiveModal(null)} className="flex-1 bg-surface-container-low border border-outline-variant/60 text-on-surface-variant py-2 rounded-lg text-xs font-bold active:scale-95 transition-all">Cancel</button>
                            <button 
                              onClick={async () => {
                                const url = modalData.webhookUrl?.trim();
                                const eventsStr = modalData.webhookEvents || 'insert,update,delete';
                                if (!url) return;
                                try {
                                  const events = eventsStr.split(',').map(e => e.trim().toLowerCase()).filter(Boolean);
                                  await api(`/admin/projects/${encodeURIComponent(projectId)}/webhooks`, {
                                    method: 'POST',
                                    body: JSON.stringify({ url, events, enabled: true })
                                  });
                                  showToast('Webhook registered successfully', 'success');
                                  loadWebhooks();
                                  setActiveModal(null);
                                } catch {
                                  showToast('Failed to register webhook', 'error');
                                }
                              }} 
                              className="flex-1 bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-95 transition-all"
                            >
                              Add
                            </button>
                          </div>
                        </div>
                      </>
                    )}
        
                    {activeModal === 'create_cron' && (
                      <>
                        <h3 className="text-sm font-bold text-on-surface uppercase tracking-wider">Add Scheduled Task</h3>
                        <div className="space-y-3">
                          <div>
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Task Name</label>
                            <input 
                              type="text"
                              value={modalData.cronName || ''}
                              onChange={(e) => setModalData(d => ({ ...d, cronName: e.target.value }))}
                              className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                              placeholder="e.g. sync-orders"
                            />
                          </div>
                          <div>
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Schedule (Cron expression)</label>
                            <input 
                              type="text"
                              value={modalData.cronSchedule || '*/5 * * * *'}
                              onChange={(e) => setModalData(d => ({ ...d, cronSchedule: e.target.value }))}
                              className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                              placeholder="*/5 * * * *"
                            />
                          </div>
                          <div>
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Callback URL</label>
                            <input 
                              type="url"
                              value={modalData.cronUrl || ''}
                              onChange={(e) => setModalData(d => ({ ...d, cronUrl: e.target.value }))}
                              className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                              placeholder="https://api.my-app.com/tasks/sync"
                            />
                          </div>
                          <div className="flex gap-2 pt-2">
                            <button onClick={() => setActiveModal(null)} className="flex-1 bg-surface-container-low border border-outline-variant/60 text-on-surface-variant py-2 rounded-lg text-xs font-bold active:scale-95 transition-all">Cancel</button>
                            <button 
                              onClick={async () => {
                                const name = modalData.cronName?.trim();
                                const schedule = modalData.cronSchedule?.trim();
                                const url = modalData.cronUrl?.trim();
                                if (!name || !schedule || !url) return;
                                try {
                                  await api(`/admin/projects/${encodeURIComponent(projectId)}/cron`, {
                                    method: 'POST',
                                    body: JSON.stringify({ name, schedule, method: 'POST', url, enabled: true })
                                  });
                                  showToast('Scheduled task added', 'success');
                                  loadCronJobs();
                                  setActiveModal(null);
                                } catch {
                                  showToast('Failed to add task', 'error');
                                }
                              }} 
                              className="flex-1 bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-95 transition-all"
                            >
                              Add Task
                            </button>
                          </div>
                        </div>
                      </>
                    )}
        
                    {activeModal === 'add_document' && (
                      <>
                        <h3 className="text-sm font-bold text-on-surface uppercase tracking-wider">Add Document to {selectedCol}</h3>
                        <div className="space-y-3">
                          <div>
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Document JSON</label>
                            <textarea 
                              value={modalData.docJson || ''}
                              onChange={(e) => setModalData(d => ({ ...d, docJson: e.target.value }))}
                              className="w-full h-40 bg-surface-container-low border border-outline-variant/60 rounded-lg p-2.5 text-xs text-on-surface font-mono focus:outline-none focus:border-primary/50"
                              placeholder='{"title": "New Document"}'
                            />
                          </div>
                          <div className="flex gap-2 pt-2">
                            <button onClick={() => setActiveModal(null)} className="flex-1 bg-surface-container-low border border-outline-variant/60 text-on-surface-variant py-2 rounded-lg text-xs font-bold active:scale-95 transition-all">Cancel</button>
                            <button 
                              onClick={async () => {
                                const jsonStr = modalData.docJson?.trim();
                                if (!jsonStr || !selectedCol) return;
                                try {
                                  const parsed = JSON.parse(jsonStr);
                                  await api(`/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(selectedCol)}/documents`, {
                                    method: 'POST',
                                    body: JSON.stringify(parsed)
                                  });
                                  showToast('Document added successfully', 'success');
                                  loadDocuments(selectedCol);
                                  setActiveModal(null);
                                } catch (err) {
                                  showToast((err as Error).message || 'Invalid JSON or request failed', 'error');
                                }
                              }} 
                              className="flex-1 bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-95 transition-all"
                            >
                              Save
                            </button>
                          </div>
                        </div>
                      </>
                    )}
        
                    {activeModal === 'import_json' && (
                      <>
                        <h3 className="text-sm font-bold text-on-surface uppercase tracking-wider">Import JSON into {selectedCol}</h3>
                        <div className="space-y-3">
                          <div>
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">JSON Array</label>
                            <textarea 
                              value={modalData.importJson || ''}
                              onChange={(e) => setModalData(d => ({ ...d, importJson: e.target.value }))}
                              className="w-full h-48 bg-surface-container-low border border-outline-variant/60 rounded-lg p-2.5 text-xs text-on-surface font-mono focus:outline-none focus:border-primary/50"
                              placeholder={`[\n  { "title": "Doc One" },\n  { "title": "Doc Two" }\n]`}
                            />
                          </div>
                          <div className="flex gap-2 pt-2">
                            <button onClick={() => setActiveModal(null)} className="flex-1 bg-surface-container-low border border-outline-variant/60 text-on-surface-variant py-2 rounded-lg text-xs font-bold active:scale-95 transition-all">Cancel</button>
                            <button 
                              onClick={async () => {
                                const jsonStr = modalData.importJson?.trim();
                                if (!jsonStr || !selectedCol) return;
                                try {
                                  const parsed = JSON.parse(jsonStr);
                                  if (!Array.isArray(parsed)) throw new Error('Expected a JSON array');
                                  const res = await api<any>(`/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(selectedCol)}/import`, {
                                    method: 'POST',
                                    body: JSON.stringify(parsed)
                                  });
                                  const count = res?.count || parsed.length;
                                  showToast(`Imported ${count} document(s) successfully`, 'success');
                                  loadDocuments(selectedCol);
                                  setActiveModal(null);
                                } catch (err) {
                                  showToast((err as Error).message || 'Invalid JSON array or request failed', 'error');
                                }
                              }} 
                              className="flex-1 bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-95 transition-all"
                            >
                              Import
                            </button>
                          </div>
                        </div>
                      </>
                    )}
        
                    {activeModal === 'create_collection' && (
                      <>
                        <h3 className="text-sm font-bold text-on-surface uppercase tracking-wider">Create New Collection</h3>
                        <div className="space-y-3">
                          <div>
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Collection Name</label>
                            <input 
                              type="text"
                              value={modalData.collectionName || ''}
                              onChange={(e) => setModalData(d => ({ ...d, collectionName: e.target.value }))}
                              className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                              placeholder="e.g. products"
                              onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); (e.target as HTMLInputElement).parentElement?.parentElement?.querySelector('button.bg-primary')?.dispatchEvent(new MouseEvent('click', { bubbles: true })); } }}
                            />
                          </div>
                          <div className="flex gap-2 pt-2">
                            <button onClick={() => setActiveModal(null)} className="flex-1 bg-surface-container-low border border-outline-variant/60 text-on-surface-variant py-2 rounded-lg text-xs font-bold active:scale-95 transition-all">Cancel</button>
                            <button 
                              onClick={async () => {
                                const name = modalData.collectionName?.trim();
                                if (!name) return;
                                try {
                                  await api(`/admin/projects/${encodeURIComponent(projectId)}/collections`, {
                                    method: 'POST',
                                    body: JSON.stringify({ name })
                                  });
                                  showToast(`Collection "${name}" created`, 'success');
                                  loadCollections();
                                  setSelectedCol(name);
                                  setActiveModal(null);
                                } catch {
                                  showToast('Failed to create collection', 'error');
                                }
                              }} 
                              className="flex-1 bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-95 transition-all"
                            >
                              Create
                            </button>
                          </div>
                        </div>
                      </>
                    )}
        
                    {activeModal === 'add_config' && (
                      <>
                        <h3 className="text-sm font-bold text-on-surface uppercase tracking-wider">Add Config Entry</h3>
                        <div className="space-y-3">
                          <div>
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Key</label>
                            <input 
                              type="text"
                              value={modalData.configKey || ''}
                              onChange={(e) => setModalData(d => ({ ...d, configKey: e.target.value }))}
                              className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                              placeholder="e.g. MY_API_KEY"
                            />
                          </div>
                          <div>
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Type</label>
                            <select
                              value={modalData.configType || 'string'}
                              onChange={(e) => setModalData(d => ({ ...d, configType: e.target.value }))}
                              className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                            >
                              <option value="string">string</option>
                              <option value="number">number</option>
                              <option value="boolean">boolean</option>
                              <option value="json">json</option>
                            </select>
                          </div>
                          <div>
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Value</label>
                            <input 
                              type="text"
                              value={modalData.configValue || ''}
                              onChange={(e) => setModalData(d => ({ ...d, configValue: e.target.value }))}
                              className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                              placeholder="Enter value..."
                            />
                          </div>
                          <div className="flex items-center gap-2">
                            <input
                              type="checkbox"
                              id="configPublic"
                              checked={modalData.configPublic === 'true'}
                              onChange={(e) => setModalData(d => ({ ...d, configPublic: e.target.checked ? 'true' : 'false' }))}
                              className="accent-primary"
                            />
                            <label htmlFor="configPublic" className="text-[9px] font-bold text-on-surface-variant uppercase tracking-wider font-mono">Public</label>
                          </div>
                          <div className="flex gap-2 pt-2">
                            <button onClick={() => setActiveModal(null)} className="flex-1 bg-surface-container-low border border-outline-variant/60 text-on-surface-variant py-2 rounded-lg text-xs font-bold active:scale-95 transition-all">Cancel</button>
                            <button 
                              onClick={async () => {
                                const key = modalData.configKey?.trim();
                                const value = modalData.configValue || '';
                                const type = modalData.configType || 'string';
                                const pub = modalData.configPublic === 'true';
                                if (!key) return;
                                try {
                                  await api(`/admin/projects/${encodeURIComponent(projectId)}/config`, {
                                    method: 'POST',
                                    body: JSON.stringify({ key, value, type, public: pub })
                                  });
                                  showToast('Config entry added', 'success');
                                  loadConfigEntries();
                                  setActiveModal(null);
                                } catch {
                                  showToast('Failed to add config entry', 'error');
                                }
                              }} 
                              className="flex-1 bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-95 transition-all"
                            >
                              Add
                            </button>
                          </div>
                        </div>
                      </>
                    )}
      </div>
    </div>
  );
}
