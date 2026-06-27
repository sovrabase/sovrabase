import { useState, useEffect, useCallback } from 'react';
import { Plus, Trash2, Search, Upload, Database, FileText, Loader2, Check, ChevronRight, ChevronDown } from 'lucide-react';
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

// ===== Recursive JSON editor =====
type JsonValue = string | number | boolean | null | JsonValue[] | { [key: string]: JsonValue };
const PROTECTED_KEYS = new Set(['_id', '_createdAt', '_updatedAt']);

interface JsonNodeProps {
  value: JsonValue;
  onChange: (v: JsonValue) => void;
  depth?: number;
  label?: string;
  readOnly?: boolean;
}

function renameKey(entries: [string, JsonValue][], oldKey: string, newKey: string): Record<string, JsonValue> {
  const out: Record<string, JsonValue> = {};
  for (const [k, v] of entries) out[k === oldKey ? newKey : k] = v;
  return out;
}

function uniqueKey(obj: object, base = 'new_key'): string {
  let k = base; let i = 1;
  while (Object.prototype.hasOwnProperty.call(obj, k)) { k = `${base}_${i++}`; }
  return k;
}

function JsonNode({ value, onChange, readOnly = false }: JsonNodeProps) {
  const [collapsed, setCollapsed] = useState(false);
  const toggleBtn = (label: string) => (
    <button type="button" onClick={() => setCollapsed((c) => !c)} className="flex items-center gap-1 text-text-secondary text-xs font-mono hover:text-text-primary">
      {collapsed ? <ChevronRight className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />} <span>{label}</span>
    </button>
  );

  if (value === null) return <span className="text-text-muted text-xs font-mono italic">null</span>;

  if (Array.isArray(value)) {
    return (
      <div>
        {toggleBtn(collapsed ? `[ ${value.length} items ]` : '[')}
        {!collapsed && (
          <div className="border-l border-border/60 ml-1 pl-2 sm:pl-3 mt-1 space-y-1">
            {value.map((item, i) => (
              <div key={i} className="flex items-start gap-1.5 sm:gap-2">
                <span className="text-text-muted text-xs font-mono shrink-0 w-4 sm:w-5 text-right select-none pt-1">{i}</span>
                <div className="flex-1 min-w-0">
                  <JsonNode value={item} onChange={(v) => { const n = [...value]; n[i] = v; onChange(n); }} readOnly={readOnly} />
                </div>
                {!readOnly && (
                  <button type="button" onClick={() => onChange(value.filter((_, idx) => idx !== i))} className="p-1 mt-0.5 text-text-muted hover:text-danger shrink-0" title="Remove item"><Trash2 className="w-3 h-3" /></button>
                )}
              </div>
            ))}
            {!readOnly && (
              <button type="button" onClick={() => onChange([...value, null])} className="flex items-center gap-1 text-accent text-xs hover:underline"><Plus className="w-3 h-3" /> Add item</button>
            )}
            <div className="text-text-muted text-xs font-mono">{']'}</div>
          </div>
        )}
      </div>
    );
  }

  if (typeof value === 'object') {
    const entries = Object.entries(value);
    return (
      <div>
        {toggleBtn(collapsed ? `{ ${entries.length} keys }` : '{')}
        {!collapsed && (
          <div className="border-l border-border/60 ml-1 pl-2 sm:pl-3 mt-1 space-y-1">
            {entries.map(([k, v], idx) => {
              const isProtected = PROTECTED_KEYS.has(k);
              const isRO = readOnly || isProtected;
              return (
                <div key={idx} className="flex items-start gap-1.5 sm:gap-2">
                  <input type="text" value={k} disabled={isRO} onChange={(e) => onChange(renameKey(entries, k, e.target.value))} className={`w-20 sm:w-28 shrink-0 bg-bg-input border border-border rounded px-2 py-1 text-xs font-mono focus:outline-none focus:border-accent disabled:cursor-default ${isProtected ? 'text-text-muted' : 'text-text-primary'}`} />
                  <div className="flex-1 min-w-0 py-1">
                    <JsonNode value={v} onChange={(nv) => onChange({ ...value, [k]: nv })} readOnly={isRO} />
                  </div>
                  {!isRO && (
                    <button type="button" onClick={() => { const n = { ...value }; delete n[k]; onChange(n); }} className="p-1 mt-0.5 text-text-muted hover:text-danger shrink-0" title="Remove field"><Trash2 className="w-3 h-3" /></button>
                  )}
                </div>
              );
            })}
            {!readOnly && (
              <button type="button" onClick={() => onChange({ ...value, [uniqueKey(value)]: '' })} className="flex items-center gap-1 text-accent text-xs hover:underline"><Plus className="w-3 h-3" /> Add field</button>
            )}
            <div className="text-text-muted text-xs font-mono">{'}'}</div>
          </div>
        )}
      </div>
    );
  }

  if (typeof value === 'boolean') {
    return (
      <label className="flex items-center gap-2 cursor-pointer">
        <input type="checkbox" checked={value} disabled={readOnly} onChange={(e) => onChange(e.target.checked)} className="w-4 h-4 rounded border-border bg-bg-input accent-accent disabled:cursor-default" />
        <span className="text-text-muted text-xs font-mono">{String(value)}</span>
      </label>
    );
  }

  if (typeof value === 'number') {
    return <input type="number" value={Number.isFinite(value) ? value : 0} disabled={readOnly} onChange={(e) => onChange(e.target.value === '' ? 0 : Number(e.target.value))} className="w-32 bg-bg-input border border-border rounded px-2 py-1 text-xs font-mono text-text-primary focus:outline-none focus:border-accent disabled:cursor-default" />;
  }

  // String (default)
  return <input type="text" value={value} disabled={readOnly} onChange={(e) => onChange(e.target.value)} className="w-64 bg-bg-input border border-border rounded px-2 py-1 text-xs font-mono text-text-primary focus:outline-none focus:border-accent disabled:cursor-default" />;
}

function stripProtected(obj: Record<string, JsonValue>): Record<string, JsonValue> {
  const out: Record<string, JsonValue> = {};
  for (const [k, v] of Object.entries(obj)) {
    if (!PROTECTED_KEYS.has(k)) out[k] = v;
  }
  return out;
}

function cloneDoc(d: DatabaseDocument): Record<string, JsonValue> {
  return JSON.parse(JSON.stringify(d)) as Record<string, JsonValue>;
}

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

  // Edited doc state (working copy in Fields panel)
  const [editedDoc, setEditedDoc] = useState<Record<string, JsonValue> | null>(null);
  const [savingDoc, setSavingDoc] = useState(false);

  // New doc state (Add Document modal)
  const [showAddDoc, setShowAddDoc] = useState(false);
  const [newDoc, setNewDoc] = useState<Record<string, JsonValue>>({});

  useEffect(() => {
    setLoadingCols(true);
    api<{ collections: (string | CollectionInfo)[] }>(`/admin/projects/${encodeURIComponent(projectId)}/collections`)
      .then((d) => setCollections((d.collections || []).map((c) =>
        typeof c === 'string' ? { name: c } : c
      )))
      .catch(() => {}).finally(() => setLoadingCols(false));
  }, [projectId]);

  const loadDocs = useCallback(async (colName: string): Promise<DatabaseDocument[]> => {
    setLoadingDocs(true);
    let arr: DatabaseDocument[] = [];
    try {
      const data = await api<DatabaseDocument[]>(
        `/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(colName)}/documents`
      );
      arr = Array.isArray(data) ? data : [];
      setDocs(arr);
    } catch { setDocs([]); }
    setLoadingDocs(false);
    return arr;
  }, [projectId]);

  const selectCol = (name: string) => { setSelectedCol(name); setSelectedDoc(null); setEditedDoc(null); setDocFilter(''); loadDocs(name); };

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

  const clearCollection = async (name: string) => {
    if (!confirm(`Clear ALL documents from "${name}"? The collection and its rules/indexes will be preserved.`)) return;
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(name)}/clear`, { method: 'POST' });
      showToast(`"${name}" cleared`, 'success');
      setDocs([]);
    } catch (e: unknown) { showToast((e as Error).message || 'Failed', 'error'); }
  };

  const dropCollection = async (name: string) => {
    if (!confirm(`Drop "${name}"? All documents will be deleted.`)) return;
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(name)}`, { method: 'DELETE' });
      showToast(`"${name}" dropped`, 'success');
      setSelectedCol(null); setDocs([]); setEditedDoc(null);
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

  const openAddDoc = () => {
    setNewDoc({});
    setShowAddDoc(true);
  };

  const insertChip = (field: keyof RlsRules, chip: string) => setRls((prev) => ({ ...prev, [field]: (prev[field] || '') + chip }));

  const addDocument = async () => {
    if (!selectedCol) return;
    setAddingDoc(true);
    try {
      const doc = stripProtected({ ...newDoc });
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

  const saveDoc = async () => {
    if (!selectedCol || !selectedDoc || !editedDoc) return;
    setSavingDoc(true);
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(selectedCol)}/documents/${encodeURIComponent(selectedDoc)}`, {
        method: 'PUT',
        body: JSON.stringify(editedDoc),
      });
      showToast('Document saved', 'success');
      const fresh = await loadDocs(selectedCol);
      const updated = fresh.find((d) => d._id === selectedDoc);
      if (updated) setEditedDoc(cloneDoc(updated));
    } catch (e: unknown) {
      showToast((e as Error).message || 'Save failed', 'error');
    }
    setSavingDoc(false);
  };

  const resetDoc = () => {
    const d = docs.find((x) => x._id === selectedDoc);
    if (d) setEditedDoc(cloneDoc(d));
  };

  const importDocuments = async () => {
    if (!selectedCol) return;
    setImporting(true);
    try {
      const docsArr = JSON.parse(importJson);
      if (!Array.isArray(docsArr)) throw new Error('Expected a JSON array');
      await api(`/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(selectedCol)}/import`, {
        method: 'POST',
        body: JSON.stringify(docsArr),
      });
      showToast(`${docsArr.length} documents imported`, 'success');
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
  const dirty = !!(activeDoc && editedDoc && JSON.stringify(editedDoc) !== JSON.stringify(activeDoc));

  // Re-clone working copy when the selection changes (intentionally not on `docs`
  // so unrelated reloads don't blow away in-flight edits).
  useEffect(() => {
    if (!selectedDoc) { setEditedDoc(null); return; }
    const d = docs.find((x) => x._id === selectedDoc);
    if (d) setEditedDoc(cloneDoc(d));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedDoc]);

  if (loadingCols) return <div className="flex items-center gap-2 py-10 text-text-muted"><Loader2 className="w-4 h-4 animate-spin" /> Loading collections...</div>;

  return (
    <div className="flex flex-col lg:flex-row gap-0 lg:h-[calc(100vh-280px)] lg:min-h-[500px] border border-border rounded-lg overflow-hidden">
      {/* Col 1: Collections */}
      <div className="w-full lg:w-[240px] shrink-0 lg:border-r border-border flex flex-col bg-bg-card max-h-[300px] lg:max-h-none">
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
        {selectedCol && !(selectedCol.startsWith('_') || selectedCol.startsWith('__')) && (
          <div className="px-3 py-2 border-t border-border space-y-1">
            <button onClick={() => clearCollection(selectedCol)} className="flex items-center gap-1 w-full px-2 py-1.5 rounded text-xs text-text-muted hover:text-yellow-500 hover:bg-yellow-500/10"><Trash2 className="w-3 h-3" /> Clear documents</button>
            <button onClick={() => dropCollection(selectedCol)} className="flex items-center gap-1 w-full px-2 py-1.5 rounded text-xs text-text-muted hover:text-danger hover:bg-danger/10"><Trash2 className="w-3 h-3" /> Drop collection</button>
          </div>
        )}
      </div>

      {/* Col 2: Documents */}
      <div className="w-full lg:w-[280px] shrink-0 lg:border-r border-b lg:border-b-0 border-border flex flex-col bg-bg-card max-h-[300px] lg:max-h-none">
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
          <button onClick={() => { if (selectedDoc && selectedCol) { if (confirm('Delete this document?')) { api(`/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(selectedCol)}/documents/${encodeURIComponent(selectedDoc)}`, { method: 'DELETE' }).then(() => { showToast('Document deleted', 'success'); setSelectedDoc(null); setEditedDoc(null); loadDocs(selectedCol); }).catch((e) => showToast(e.message, 'error')); } } }} className="ml-auto flex items-center gap-1 px-2 py-1 rounded text-xs text-text-muted hover:text-danger hover:bg-danger/10 disabled:opacity-30" disabled={!selectedDoc} title="Delete"><Trash2 className="w-3.5 h-3.5" /> Delete</button>
        </div>

        <div className="flex-1 overflow-y-auto p-4">
          {subTab === 'fields' ? (
            !activeDoc || !editedDoc ? (
              <div className="flex flex-col items-center py-12 text-text-muted gap-2"><FileText className="w-6 h-6" /><p className="text-xs">Select a document</p></div>
            ) : (
              <div>
                <div className="flex items-center justify-between mb-3 gap-2">
                  <h4 className="text-text-primary font-semibold text-sm font-mono truncate">{activeDoc._id}</h4>
                  <div className="flex items-center gap-2 shrink-0">
                    {dirty && <span className="text-xs text-[#f59e0b] font-medium whitespace-nowrap">&bull; Unsaved changes</span>}
                    <button onClick={resetDoc} disabled={!dirty} className="px-3 py-1.5 border border-border rounded-lg text-text-secondary text-xs hover:text-text-primary disabled:opacity-40 disabled:cursor-not-allowed">Reset</button>
                    <button onClick={saveDoc} disabled={!dirty || savingDoc} className="flex items-center gap-1 px-3 py-1.5 bg-accent text-white rounded-lg text-xs font-medium hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed">
                      {savingDoc ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Check className="w-3.5 h-3.5" />} Save
                    </button>
                  </div>
                </div>
                <div className="bg-bg-input/30 border border-border rounded-lg p-3 overflow-auto max-h-[calc(100vh-420px)]">
                  <JsonNode value={editedDoc} onChange={(v) => setEditedDoc(v as Record<string, JsonValue>)} />
                </div>
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

      <Modal isOpen={showAddDoc} onClose={() => setShowAddDoc(false)} title={`Add Document to ${selectedCol}`} size="lg">
        <div className="space-y-4">
          <div className="bg-bg-input/30 border border-border rounded-lg p-3 max-h-[60vh] overflow-auto">
            <JsonNode value={newDoc} onChange={(v) => setNewDoc(v as Record<string, JsonValue>)} />
          </div>
          <p className="text-text-muted text-xs">Use Add field to build nested objects and arrays. Protected keys (<code className="font-mono text-text-secondary">_id</code>, <code className="font-mono text-text-secondary">_createdAt</code>, <code className="font-mono text-text-secondary">_updatedAt</code>) are auto-managed and stripped on insert.</p>
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
