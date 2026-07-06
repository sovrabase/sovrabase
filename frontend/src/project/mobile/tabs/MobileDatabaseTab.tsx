import { api } from '../../../api';
import { useMobileDashboard } from '../context';
import { renderFormattedJson } from '../utils';

export default function MobileDatabaseTab() {
  const {
    showToast, collections, selectedCol, setSelectedCol, expandedDocId, setExpandedDocId,
    selectCol, dbSearchQuery, setDbSearchQuery, editingDocId, setEditingDocId, editingDocJson,
    setEditingDocJson, savingDocJson, setActiveModal, setModalData, loadCollections, loadDocuments,
    handleSaveDocJson, filteredDocs, projectId
  } = useMobileDashboard();
  return (
    <div className="p-container-padding space-y-stack-md animate-fade-slide">
                
                {/* Header info */}
                <div className="flex justify-between items-center mb-1">
                  <span className="text-on-surface-variant font-label-caps text-[10px] tracking-widest uppercase">Document Collections</span>
                  <div className="flex items-center gap-2">
                    <span className="px-2 py-0.5 bg-surface-container-high text-on-surface-variant font-code-xs text-[10px] rounded border border-outline-variant/30">
                      {collections.length} Tables
                    </span>
                    <button
                      onClick={() => { setActiveModal('create_collection'); setModalData({}); }}
                      className="flex items-center gap-0.5 px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider bg-primary/10 text-primary border border-primary/20 hover:bg-primary/20 active:scale-95 transition-all"
                    >
                      <span className="material-symbols-outlined text-[14px]">add</span>
                      <span>New Table</span>
                    </button>
                  </div>
                </div>
    
                {/* Collection Chips Selector */}
                <div className="flex gap-2 overflow-x-auto no-scrollbar pb-1">
                  {collections.map(c => (
                    <button
                      key={c.name}
                      onClick={() => selectCol(c.name)}
                      className={`px-3 py-1.5 rounded-full text-xs font-semibold uppercase whitespace-nowrap tracking-wider border transition-all active:scale-95 ${
                        selectedCol === c.name 
                          ? 'bg-primary/10 border-primary text-primary' 
                          : 'bg-surface-container-high border-outline-variant text-on-surface-variant hover:text-on-surface'
                      }`}
                    >
                      {c.name}
                    </button>
                  ))}
                </div>
    
                {/* Database actions & Search */}
                <div className="flex gap-2 flex-wrap">
                  <div className="relative flex-1 min-w-[120px]">
                    <span className="absolute inset-y-0 left-0 pl-2.5 flex items-center pointer-events-none">
                      <span className="material-symbols-outlined text-[16px] text-on-surface-variant/50">search</span>
                    </span>
                    <input
                      type="text"
                      placeholder={`Search ${selectedCol}...`}
                      value={dbSearchQuery}
                      onChange={(e) => setDbSearchQuery(e.target.value)}
                      className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg pl-8 pr-3 py-1.5 text-xs text-on-surface placeholder-on-surface-variant/40 focus:outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/20"
                    />
                  </div>
                  <button 
                    onClick={() => { setActiveModal('add_document'); setModalData({ docJson: '{\n  "title": "New Document"\n}' }); }}
                    disabled={!selectedCol}
                    className="bg-primary hover:bg-primary-fixed-dim text-on-primary font-bold px-3.5 py-1.5 rounded-lg text-xs flex items-center gap-1 active:scale-95 transition-all disabled:opacity-50"
                  >
                    <span className="material-symbols-outlined text-sm">add</span>
                    <span>Row</span>
                  </button>
                  <button 
                    onClick={() => { setActiveModal('import_json'); setModalData({ importJson: '' }); }}
                    disabled={!selectedCol}
                    className="bg-secondary/10 text-secondary border border-secondary/20 hover:bg-secondary/20 font-bold px-3.5 py-1.5 rounded-lg text-xs flex items-center gap-1 active:scale-95 transition-all disabled:opacity-50"
                  >
                    <span className="material-symbols-outlined text-sm">file_upload</span>
                    <span>Import</span>
                  </button>
                </div>
    
                {/* Collection Actions: Clear & Drop */}
                {selectedCol && (
                  <div className="flex gap-2">
                    <button
                      onClick={async () => {
                        if (!confirm(`Clear ALL documents from "${selectedCol}"? This cannot be undone.`)) return;
                        try {
                          await api(`/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(selectedCol)}/clear`, { method: 'POST' });
                          showToast(`Collection "${selectedCol}" cleared`, 'success');
                          loadDocuments(selectedCol);
                          loadCollections();
                        } catch {
                          showToast('Failed to clear collection', 'error');
                        }
                      }}
                      className="px-2.5 py-1 rounded text-[10px] font-bold uppercase tracking-wider text-on-surface-variant border border-outline-variant/60 hover:bg-surface-container-high active:scale-95 transition-all flex items-center gap-1"
                    >
                      <span className="material-symbols-outlined text-[14px]">layers_clear</span>
                      Clear
                    </button>
                    <button
                      onClick={async () => {
                        if (!confirm(`DROP collection "${selectedCol}"? This permanently deletes the collection and all its documents.`)) return;
                        try {
                          await api(`/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(selectedCol)}`, { method: 'DELETE' });
                          showToast(`Collection "${selectedCol}" dropped`, 'success');
                          setSelectedCol('');
                          loadCollections();
                        } catch {
                          showToast('Failed to drop collection', 'error');
                        }
                      }}
                      className="px-2.5 py-1 rounded text-[10px] font-bold uppercase tracking-wider text-error border border-error/30 hover:bg-error-container/10 active:scale-95 transition-all flex items-center gap-1"
                    >
                      <span className="material-symbols-outlined text-[14px]">delete_forever</span>
                      Drop
                    </button>
                  </div>
                )}
    
                {/* stacked card-components with expandable height */}
                {editingDocId ? (
                  /* Inline Document JSON Editor */
                  <div className="bg-surface-container border border-outline-variant p-stack-md rounded-xl space-y-4 animate-fade-slide">
                    <div className="flex justify-between items-center pb-2 border-b border-outline-variant/30">
                      <h3 className="text-xs font-bold text-on-surface uppercase tracking-widest truncate">Edit Row: {editingDocId}</h3>
                      <button onClick={() => setEditingDocId(null)} className="text-[10px] text-on-surface-variant hover:text-on-surface uppercase font-bold">Cancel</button>
                    </div>
                    
                    <div className="space-y-3">
                      <div>
                        <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Document JSON Body</label>
                        <textarea 
                          value={editingDocJson}
                          onChange={(e) => setEditingDocJson(e.target.value)}
                          className="w-full h-48 bg-surface-container-low border border-outline-variant/60 rounded-lg p-2.5 text-xs text-on-surface font-mono focus:outline-none focus:border-primary/50"
                          placeholder="{}"
                        />
                      </div>
    
                      <div className="pt-2">
                        <button 
                          onClick={handleSaveDocJson}
                          disabled={savingDocJson}
                          className="w-full bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-[0.98] transition-all disabled:opacity-50"
                        >
                          {savingDocJson ? 'Saving changes...' : 'Save Document'}
                        </button>
                      </div>
                    </div>
                  </div>
                ) : (
                  <div className="space-y-3.5">
                    {filteredDocs.length === 0 ? (
                      <div className="text-center py-8 bg-surface-container-low/40 rounded-xl border border-dashed border-outline-variant/30">
                        <span className="material-symbols-outlined text-3xl text-on-surface-variant/30 block mb-1">database</span>
                        <p className="text-xs text-on-surface-variant/50">No documents found</p>
                      </div>
                    ) : (
                      filteredDocs.map((doc) => {
                        const idVal = String(doc._id || doc.id || 'unknown');
                        const isExpanded = expandedDocId === idVal;
                        
                        return (
                          <div 
                            key={idVal}
                            className="bg-surface-container-low border border-outline-variant rounded-xl overflow-hidden transition-all duration-300"
                          >
                            {/* Summary Row */}
                            <button
                              onClick={() => setExpandedDocId(isExpanded ? null : idVal)}
                              className="w-full p-stack-md flex justify-between items-center text-left hover:bg-surface-container-high/40 transition-colors focus:outline-none"
                            >
                              <div className="flex items-center gap-2">
                                <span className="material-symbols-outlined text-xs text-primary">circle</span>
                                <span className="font-code-sm text-xs font-semibold text-on-surface">{idVal}</span>
                              </div>
                              <div className="flex items-center gap-2">
                                <span className="text-[10px] text-on-surface-variant/50 font-code-xs">
                                  {Object.keys(doc).length - 1} fields
                                </span>
                                <span className={`material-symbols-outlined text-[16px] text-on-surface-variant/60 transition-transform duration-300 ${isExpanded ? 'rotate-180' : ''}`}>
                                  expand_more
                                </span>
                              </div>
                            </button>
      
                            {/* Expandable JSON Detail Container */}
                            <div 
                              className={`grid transition-all duration-300 ease-in-out ${isExpanded ? 'grid-rows-[1fr]' : 'grid-rows-[0fr]'}`}
                              style={{ 
                                borderTop: isExpanded ? '1px solid rgba(60, 74, 66, 0.2)' : 'none'
                              }}
                            >
                              <div className="overflow-hidden">
                                <div className="p-stack-md bg-surface-container-lowest/20 space-y-2">
                                  {renderFormattedJson(doc)}
                                  <div className="flex justify-end gap-2 pt-1">
                                    <button 
                                      onClick={async () => {
                                        if (confirm('Are you sure you want to delete this document?')) {
                                          try {
                                            await api(`/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(selectedCol)}/documents/${encodeURIComponent(idVal)}`, {
                                              method: 'DELETE'
                                            });
                                            showToast('Document deleted', 'success');
                                            loadDocuments(selectedCol);
                                            setExpandedDocId(null);
                                          } catch {
                                            showToast('Failed to delete document', 'error');
                                          }
                                        }
                                      }}
                                      className="px-3 py-1 border border-error/30 text-error hover:bg-error-container/10 font-label-caps text-[10px] rounded transition-all active:scale-95"
                                    >
                                      Delete
                                    </button>
                                    <button 
                                      onClick={() => {
                                        setEditingDocId(idVal);
                                        setEditingDocJson(JSON.stringify(doc, null, 2));
                                      }}
                                      className="px-3 py-1 border border-secondary text-secondary hover:bg-secondary/10 font-label-caps text-[10px] rounded transition-all active:scale-95"
                                    >
                                      Edit JSON
                                    </button>
                                    <button 
                                      onClick={() => setExpandedDocId(null)}
                                      className="px-3 py-1 border border-outline-variant/60 text-on-surface-variant font-label-caps text-[10px] rounded transition-all active:scale-95"
                                    >
                                      Collapse
                                    </button>
                                  </div>
                                </div>
                              </div>
                            </div>
                          </div>
                        );
                      })
                    )}
                  </div>
                )}
              </div>
  );
}
