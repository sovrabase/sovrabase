import { useState, useEffect, useCallback } from 'react';
import { Plus, Trash2, Search, Upload, Database, FileText, Loader2, Check } from 'lucide-react';
import { api } from '../api';
import Modal from '../components/Modal';
import { useToast } from '../components/Toast';
import type { DatabaseDocument } from '../types';

interface CollectionInfo { name: string; doc_count?: number; schema_columns?: string[]; indexes?: string[]; }
interface RlsRules { get: string; list: string; create: string; update: string; delete: string; enabled: boolean; }
interface Props { projectId: string; }

const QUICK_CHIPS = ['auth', 'auth.uid', 'auth.role', 'data', 'true', 'false', 'null'];
const RLSPLACEHOLDERS: Record<string, string> = { get: 'e.g. auth != null', list: 'e.g. auth != null', create: 'e.g. auth != null', update: 'e.g. auth.uid == data.author_id', delete: 'e.g. auth.uid == data.author_id' };
const RLACTIONS = ['get', 'list', 'create', 'update', 'delete'] as const;

export default function DatabaseTab({ projectId }: Props) {
  const { showToast } = useToast();
  const [collections, setCollections] = useState<CollectionInfo[]>([]);
  const [loadingCols, setLoadingCols] = useState(true);
  const [selectedCol, setSelectedCol] = useState<string | null>(null);
  const [showNewCol, setShowNewCol] = useState(false);
  const [newColName, setNewColName] = useState('');
  const [creatingCol, setCreatingCol] = useState(false);
  const [docs, setDocs] = useState<DatabaseDocument[]>([]);
  const [loadingDocs, setLoadingDocs] = useState(false);
  const [docFilter, setDocFilter] = useState('');
  const [selectedDoc, setSelectedDoc] = useState<string | null>(null);
  const [subTab, setSubTab] = useState<'fields' | 'rules'>('fields');
  const [rls, setRls] = useState<RlsRules>({ get: '', list: '', create: '', update: '', delete: '', enabled: false });
  const [savingRules, setSavingRules] = useState(false);

  const [addingDoc, setAddingDoc] = useState(false);

  // Import modal
  const [showImport, setShowImport] = useState(false);
  const [importJson, setImportJson] = useState('');
  const [importing, setImporting] = useState(false);

  useEffect(() => {
    setLoadingCols(true);
    api<{ collections: (string | CollectionInfo)[] }>(`/admin/projects/${encodeURIComponent(projectId)}/collections`)
      .then((d) => setCollections((d.collections || []).map((c) =>
        typeof c === 'string' ? { name: c } : c
      )))
      .catch(() => {}).finally(() => setLoadingCols(false));
  }, [projectId]);

  const loadDocs = useCallback(async (colName: string) => {
    setLoadingDocs(true);
    try {
      const data = await api<{ documents: DatabaseDocument[] }>(
        `/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(colName)}/documents`
      );
      setDocs(data.documents || []);
    } catch { setDocs([]); }
    setLoadingDocs(false);
  }, [projectId]);

  const selectCol = (name: string) => { setSelectedCol(name); setSelectedDoc(null); setDocFilter(''); loadDocs(name); };

  const createCollection = async () => {
    if (!newColName.trim()) return;
    setCreatingCol(true);
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/collections`, { method: 'POST', body: JSON.stringify({ name: newColName.trim() }) });
      showToast(`Collection "${newColName.trim()}" created`, 'success');
      setShowNewCol(false); setNewColName('');
      const d = await api<{ collections: (string | CollectionInfo)[] }>(`/admin/projects/${encodeURIComponent(projectId)}/collections`);
      setCollections((d.collections || []).map((c) => typeof c === 'string' ? { name: c } : c));
    } catch (e: unknown) { showToast((e as Error).message || 'Failed', 'error'); }
    setCreatingCol(false);
  };

  const dropCollection = async (name: string) => {
    if (!confirm(`Drop "${name}"? All documents will be deleted.`)) return;
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(name)}`, { method: 'DELETE' });
      showToast(`"${name}" dropped`, 'success');
      setSelectedCol(null); setDocs([]);
      setCollections((prev) => prev.filter((c) => c.name !== name));
    } catch (e: unknown) { showToast((e as Error).message || 'Failed', 'error'); }
  };

  const saveRules = async () => {
    if (!selectedCol) return;
    setSavingRules(true);
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(selectedCol)}/rules`, { method: 'POST', body: JSON.stringify(rls) });
      showToast('RLS rules saved', 'success');
    } catch (e: unknown) { showToast((e as Error).message || 'Failed', 'error'); }
    setSavingRules(false);
  };
  const [showAddDoc, setShowAddDoc] = useState(false);
  const [docFields, setDocFields] = useState<{ key: string; value: string }[]>([{ key: '', value: '' }]);
  const [docJsonMode, setDocJsonMode] = useState(false);
  const [docJsonRaw, setDocJsonRaw] = useState('{\n  \n}');

  const openAddDoc = () => {
    setDocFields([{ key: '', value: '' }]);
    setDocJsonMode(false);
    setShowAddDoc(true);
  };

  const addDocField = () => setDocFields((f) => [...f, { key: '', value: '' }]);
  const removeDocField = (i: number) => setDocFields((f) => f.filter((_, idx) => idx !== i));
  const updateDocField = (i: number, field: 'key' | 'value', val: string) => setDocFields((f) => f.map((item, idx) => idx === i ? { ...item, [field]: val } : item));

  const insertChip = (field: keyof RlsRules, chip: string) => setRls((prev) => ({ ...prev, [field]: (prev[field] || '') + chip }));

  const addDocument = async () => {
    if (!selectedCol) return;
    setAddingDoc(true);
    try {
      const doc = docJsonMode
        ? JSON.parse(docJsonRaw)
        : Object.fromEntries(docFields.filter((f) => f.key.trim()).map((f) => [f.key.trim(), f.value]));
      await api(`/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(selectedCol)}/documents`, {
        method: 'POST',
        body: JSON.stringify(doc),
      });
      showToast('Document created', 'success');
      setShowAddDoc(false);
      loadDocs(selectedCol);
    } catch (e: unknown) {
      showToast((e as Error).message || 'Invalid data', 'error');
    }
    setAddingDoc(false);
  };

  const importDocuments = async () => {
    if (!selectedCol) return;
    setImporting(true);
    try {
      const docs = JSON.parse(importJson);
      if (!Array.isArray(docs)) throw new Error('Expected a JSON array');
      await api(`/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(selectedCol)}/import`, {
        method: 'POST',
        body: JSON.stringify(docs),
      });
      showToast(`${docs.length} documents imported`, 'success');
      setShowImport(false);
      setImportJson('');
      loadDocs(selectedCol);
    } catch (e: unknown) {
      showToast((e as Error).message || 'Invalid JSON array', 'error');
    }
    setImporting(false);
  };

  const activeDoc = docs.find((d) => d._id === selectedDoc);
  const filteredDocs = docs.filter((d) => !docFilter.trim() ? true : Object.values(d).some((v) => String(v).toLowerCase().includes(docFilter.toLowerCase())));

  if (loadingCols) return <div className="flex items-center gap-2 py-10 text-text-muted"><Loader2 className="w-4 h-4 animate-spin" /> Loading collections...</div>;

  return (
    <div className="flex gap-0 h-[calc(100vh-280px)] min-h-[500px] border border-border rounded-lg overflow-hidden">
      {/* Col 1: Collections */}
      <div className="w-[240px] shrink-0 border-r border-border flex flex-col bg-bg-card">
        <div className="px-4 py-3 border-b border-border flex items-center justify-between">
          <h3 className="text-text-primary font-semibold text-sm">Collections</h3>
          <button onClick={() => setShowNewCol(true)} className="p-1 rounded hover:bg-bg-input text-text-muted hover:text-text-primary" title="New"><Plus className="w-4 h-4" /></button>
        </div>
        <div className="flex-1 overflow-y-auto">
          {collections.length === 0 ? (
            <div className="flex flex-col items-center py-12 text-text-muted gap-2"><Database className="w-6 h-6" /><p className="text-xs">No collections</p></div>
          ) : collections.map((c) => (
            <button key={c.name} onClick={() => selectCol(c.name)} className={`w-full text-left px-4 py-2.5 flex items-center justify-between transition-colors ${selectedCol === c.name ? 'bg-accent/10 border-r-2 border-accent' : 'hover:bg-bg-input/50'}`}>
              <span className="text-text-primary text-sm truncate">{c.name}</span>
              <span className="text-text-muted text-xs ml-2 shrink-0">{c.doc_count ?? 0}</span>
            </button>
          ))}
        </div>
        {selectedCol && (
          <div className="px-3 py-2 border-t border-border">
            <button onClick={() => dropCollection(selectedCol)} className="flex items-center gap-1 w-full px-2 py-1.5 rounded text-xs text-text-muted hover:text-danger hover:bg-danger/10"><Trash2 className="w-3 h-3" /> Drop collection</button>
          </div>
        )}
      </div>

      {/* Col 2: Documents */}
      <div className="w-[280px] shrink-0 border-r border-border flex flex-col bg-bg-card">
        <div className="px-4 py-3 border-b border-border flex items-center justify-between gap-2">
          <h3 className="text-text-primary font-semibold text-sm">Documents</h3>
          <div className="flex items-center gap-1">
            <button onClick={() => selectedCol && openAddDoc()} className="p-1 rounded hover:bg-bg-input text-text-muted hover:text-text-primary" title="Add"><Plus className="w-3.5 h-3.5" /></button>
            <button onClick={() => selectedCol && setShowImport(true)} className="p-1 rounded hover:bg-bg-input text-text-muted hover:text-text-primary" title="Import"><Upload className="w-3.5 h-3.5" /></button>
          </div>
        </div>
        <div className="px-3 py-2 border-b border-border/50">
          <div className="relative">
            <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-text-muted" />
            <input type="text" placeholder="Filter documents..." value={docFilter} onChange={(e) => setDocFilter(e.target.value)} className="w-full bg-bg-input border border-border rounded-md pl-8 pr-3 py-1.5 text-text-primary text-xs placeholder:text-text-muted focus:outline-none focus:border-accent" />
          </div>
        </div>
        <div className="flex-1 overflow-y-auto">
          {!selectedCol ? (
            <div className="flex flex-col items-center py-16 text-text-muted gap-2"><Database className="w-6 h-6" /><p className="text-xs">Select a collection</p></div>
          ) : loadingDocs ? (
            <div className="flex items-center gap-2 py-8 justify-center text-text-muted"><Loader2 className="w-4 h-4 animate-spin" /> Loading...</div>
          ) : filteredDocs.length === 0 ? (
            <div className="flex flex-col items-center py-16 text-text-muted gap-2"><FileText className="w-6 h-6" /><p className="text-xs">{docFilter ? 'No matching docs' : 'No documents'}</p></div>
          ) : filteredDocs.map((d) => {
            const preview = Object.entries(d).filter(([k]) => k !== '_id' && !k.startsWith('_')).slice(0, 1).map(([, v]) => String(v).slice(0, 40)).join('') || '(empty)';
            return (
              <button key={d._id} onClick={() => setSelectedDoc(d._id)} className={`w-full text-left px-4 py-2.5 border-b border-border/50 transition-colors ${selectedDoc === d._id ? 'bg-accent/10 border-l-2 border-l-accent' : 'hover:bg-bg-input/30'}`}>
                <p className="text-text-primary text-xs font-mono truncate">{d._id}</p>
                <p className="text-text-muted text-[11px] truncate">{preview}</p>
              </button>
            );
          })}
        </div>
      </div>

      {/* Col 3: Fields & Rules */}
      <div className="flex-1 flex flex-col bg-bg-card">
        <div className="px-4 py-3 border-b border-border flex items-center gap-1">
          {(['fields', 'rules'] as const).map((t) => (
            <button key={t} onClick={() => setSubTab(t)} className={`px-3 py-1.5 rounded text-xs font-medium transition-colors ${subTab === t ? 'bg-accent/10 text-accent' : 'text-text-muted hover:text-text-primary'}`}>
              {t === 'fields' ? 'Fields' : 'Rules (RLS)'}
            </button>
          ))}
          <button onClick={() => { if (selectedDoc && selectedCol) { if (confirm('Delete this document?')) { api(`/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(selectedCol)}/documents/${encodeURIComponent(selectedDoc)}`, { method: 'DELETE' }).then(() => { showToast('Document deleted', 'success'); setSelectedDoc(null); loadDocs(selectedCol); }).catch((e) => showToast(e.message, 'error')); } } }} className="ml-auto px-2 py-1 rounded text-xs text-text-muted hover:text-danger hover:bg-danger/10 disabled:opacity-30" disabled={!selectedDoc} title="Delete"><Trash2 className="w-3.5 h-3.5" /> Delete</button>
        </div>

        <div className="flex-1 overflow-y-auto p-4">
          {subTab === 'fields' ? (
            !activeDoc ? (
              <div className="flex flex-col items-center py-12 text-text-muted gap-2"><FileText className="w-6 h-6" /><p className="text-xs">Select a document</p></div>
            ) : (
              <div className="space-y-2">
                <h4 className="text-text-primary font-semibold text-sm mb-3 font-mono">{activeDoc._id}</h4>
                {Object.entries(activeDoc).map(([key, value]) => (
                  <div key={key} className="flex gap-2">
                    <span className="w-32 shrink-0 text-text-muted text-xs font-mono truncate">{key}</span>
                    <span className="flex-1 text-text-primary text-xs font-mono bg-bg-input border border-border rounded px-2 py-1 break-all">{typeof value === 'object' ? JSON.stringify(value) : String(value)}</span>
                  </div>
                ))}
              </div>
            )
          ) : (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <h4 className="text-text-primary font-semibold text-sm">Row-Level Security Rules</h4>
                <label className="flex items-center gap-2 cursor-pointer">
                  <span className="text-text-muted text-xs">Enable RLS</span>
                  <input type="checkbox" checked={rls.enabled} onChange={(e) => setRls((p) => ({ ...p, enabled: e.target.checked }))} className="w-4 h-4 rounded border-border bg-bg-input accent-accent" />
                </label>
              </div>
              <div className="flex items-center gap-1 flex-wrap">
                <span className="text-text-muted text-[10px] mr-1">Insert into get:</span>
                {QUICK_CHIPS.map((chip) => (
                  <button key={chip} onClick={() => insertChip('get', chip)} className="px-2 py-0.5 bg-bg-input border border-border rounded text-text-secondary text-[10px] font-mono hover:border-accent hover:text-accent">{chip}</button>
                ))}
              </div>
              {RLACTIONS.map((action) => (
                <div key={action}>
                  <label className="block text-text-secondary text-xs font-medium mb-1 capitalize">{action}</label>
                  <div className="flex gap-2">
                    <input type="text" value={rls[action]} onChange={(e) => setRls((p) => ({ ...p, [action]: e.target.value }))} placeholder={RLSPLACEHOLDERS[action]} disabled={!rls.enabled} className="flex-1 bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-xs font-mono placeholder:text-text-muted focus:outline-none focus:border-accent disabled:opacity-40 disabled:cursor-not-allowed" />
                    <div className="flex gap-0.5 shrink-0">
                      {['auth', 'true', 'false'].map((chip) => (
                        <button key={chip} onClick={() => insertChip(action, chip)} disabled={!rls.enabled} className="px-1.5 py-1 bg-bg-input border border-border rounded text-text-muted text-[10px] font-mono hover:border-accent hover:text-accent disabled:opacity-30 disabled:cursor-not-allowed">{chip}</button>
                      ))}
                    </div>
                  </div>
                </div>
              ))}
              <button onClick={saveRules} disabled={savingRules || !selectedCol} className="flex items-center gap-2 px-4 py-2 bg-accent text-white rounded-lg text-sm font-medium hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed">
                {savingRules ? <Loader2 className="w-4 h-4 animate-spin" /> : <Check className="w-4 h-4" />} Save Rules
              </button>
            </div>
          )}
        </div>
      </div>

      {/* Modals */}
      <Modal isOpen={showNewCol} onClose={() => setShowNewCol(false)} title="Create Collection" size="sm">
        <div className="space-y-4">
          <input type="text" value={newColName} onChange={(e) => setNewColName(e.target.value)} placeholder="e.g. posts, users" className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm placeholder:text-text-muted focus:outline-none focus:border-accent" autoFocus onKeyDown={(e) => e.key === 'Enter' && createCollection()} />
          <div className="flex justify-end gap-2">
            <button onClick={() => setShowNewCol(false)} className="px-4 py-2 border border-border rounded-lg text-text-secondary text-sm hover:text-text-primary">Cancel</button>
            <button onClick={createCollection} disabled={creatingCol || !newColName.trim()} className="flex items-center gap-1 px-4 py-2 bg-accent text-white rounded-lg text-sm font-medium hover:opacity-90 disabled:opacity-50">
              {creatingCol ? <Loader2 className="w-4 h-4 animate-spin" /> : <Plus className="w-4 h-4" />} Create
            </button>
          </div>
        </div>
      </Modal>

      <Modal isOpen={showAddDoc} onClose={() => setShowAddDoc(false)} title={`Add Document to ${selectedCol}`} size="md">
        <div className="space-y-4">
          {!docJsonMode ? (
            <>
              {docFields.map((f, i) => (
                <div key={i} className="flex items-center gap-2">
                  <input value={f.key} onChange={(e) => updateDocField(i, 'key', e.target.value)} placeholder="field_name" className="flex-1 bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm font-mono placeholder:text-text-muted focus:outline-none focus:border-accent" />
                  <input value={f.value} onChange={(e) => updateDocField(i, 'value', e.target.value)} placeholder="value" className="flex-1 bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm placeholder:text-text-muted focus:outline-none focus:border-accent" />
                  {docFields.length > 1 && <button onClick={() => removeDocField(i)} className="shrink-0 p-1 text-text-muted hover:text-danger"><span className="sr-only">Remove</span>×</button>}
                </div>
              ))}
              <button onClick={addDocField} className="flex items-center gap-1 text-accent text-xs hover:underline"><Plus className="w-3 h-3" /> Add field</button>
            </>
          ) : (
            <textarea value={docJsonRaw} onChange={(e) => setDocJsonRaw(e.target.value)} rows={8} className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm font-mono placeholder:text-text-muted focus:outline-none focus:border-accent resize-y" placeholder='{"field": "value"}' />
          )}
          <div className="flex items-center gap-2">
            <label className="flex items-center gap-1.5 text-text-muted text-xs cursor-pointer">
              <input type="checkbox" checked={docJsonMode} onChange={(e) => setDocJsonMode(e.target.checked)} className="rounded border-border bg-bg-input accent-accent w-3.5 h-3.5" />
              JSON mode
            </label>
          </div>
          <div className="flex justify-end gap-2">
            <button onClick={() => setShowAddDoc(false)} className="px-4 py-2 border border-border rounded-lg text-text-secondary text-sm hover:text-text-primary">Cancel</button>
            <button onClick={addDocument} disabled={addingDoc} className="px-4 py-2 bg-accent text-white rounded-lg text-sm font-medium hover:opacity-90 disabled:opacity-50">
              {addingDoc ? 'Creating...' : 'Create'}
            </button>
          </div>
        </div>
      </Modal>

      <Modal isOpen={showImport} onClose={() => setShowImport(false)} title="Import Documents (JSON Array)" size="md">
        <div className="space-y-4">
          <textarea value={importJson} onChange={(e) => setImportJson(e.target.value)} rows={10} className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm font-mono placeholder:text-text-muted focus:outline-none focus:border-accent resize-y" placeholder='[{"name": "Alice"}, {"name": "Bob"}]' />
          <div className="flex justify-end gap-2">
            <button onClick={() => setShowImport(false)} className="px-4 py-2 border border-border rounded-lg text-text-secondary text-sm hover:text-text-primary">Cancel</button>
            <button onClick={importDocuments} disabled={importing} className="px-4 py-2 bg-accent text-white rounded-lg text-sm font-medium hover:opacity-90 disabled:opacity-50">
              {importing ? 'Importing...' : 'Import'}
            </button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
