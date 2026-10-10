import { useEffect, useMemo, useState } from 'react';
import { Plus, Trash2, X } from 'lucide-react';
import { formatDate } from '../../../api';
import { Banner, Button, CopyButton, Field, IconButton, Input, Segmented, Select, Sheet, Switch, Textarea } from '../../../ui';
import { PROTECTED_KEYS, type Doc } from './shared';

type Kind = 'text' | 'number' | 'boolean' | 'null' | 'json';

interface FieldRow {
  id: number;
  key: string;
  kind: Kind;
  /** text / number / json are edited as strings; boolean as 'true' | 'false'. */
  raw: string;
}

let seq = 0;

function kindOf(v: unknown): Kind {
  if (v === null || v === undefined) return 'null';
  if (typeof v === 'string') return 'text';
  if (typeof v === 'number') return 'number';
  if (typeof v === 'boolean') return 'boolean';
  return 'json';
}

function toRows(obj: Record<string, unknown>): FieldRow[] {
  return Object.entries(obj)
    .filter(([k]) => !PROTECTED_KEYS.has(k))
    .map(([key, v]) => {
      const kind = kindOf(v);
      const raw = kind === 'json' ? JSON.stringify(v, null, 2) : kind === 'null' ? '' : String(v);
      return { id: ++seq, key, kind, raw };
    });
}

function parseRow(r: FieldRow): unknown {
  switch (r.kind) {
    case 'text':
      return r.raw;
    case 'number': {
      const n = Number(r.raw);
      if (r.raw.trim() === '' || Number.isNaN(n)) throw new Error(`"${r.key}" must be a number`);
      return n;
    }
    case 'boolean':
      return r.raw === 'true';
    case 'null':
      return null;
    case 'json':
      try {
        return JSON.parse(r.raw);
      } catch {
        throw new Error(`"${r.key}" is not valid JSON`);
      }
  }
}

function fromRows(rows: FieldRow[]): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const r of rows) {
    const k = r.key.trim();
    if (!k) throw new Error('Every field needs a name');
    if (PROTECTED_KEYS.has(k)) throw new Error(`"${k}" is managed by the server`);
    if (k in out) throw new Error(`"${k}" appears twice`);
    out[k] = parseRow(r);
  }
  return out;
}

const KIND_OPTIONS = [
  { value: 'text', label: 'Text' },
  { value: 'number', label: 'Number' },
  { value: 'boolean', label: 'Yes / no' },
  { value: 'json', label: 'List or object' },
  { value: 'null', label: 'Empty (null)' },
];

/**
 * Create or edit one record. Two views of the same data: a field list for quick edits
 * and the raw JSON for anything nested.
 */
export function RecordSheet({
  open,
  onClose,
  collection,
  doc,
  onSave,
  onDelete,
}: {
  open: boolean;
  onClose: () => void;
  collection: string;
  /** null = new record */
  doc: Doc | null;
  onSave: (body: Record<string, unknown>) => Promise<void>;
  onDelete?: () => void;
}) {
  const [view, setView] = useState<'fields' | 'json'>('fields');
  const [rows, setRows] = useState<FieldRow[]>([]);
  const [json, setJson] = useState('{}');
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const initial = useMemo(() => {
    const base = doc ? { ...doc } : {};
    const editable: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(base)) if (!PROTECTED_KEYS.has(k)) editable[k] = v;
    return editable;
  }, [doc]);

  useEffect(() => {
    if (!open) return;
    setRows(doc ? toRows(initial) : [{ id: ++seq, key: '', kind: 'text', raw: '' }]);
    setJson(JSON.stringify(initial, null, 2));
    setView('fields');
    setError(null);
  }, [open, doc, initial]);

  const current = (): Record<string, unknown> => {
    if (view === 'json') {
      let v: unknown;
      try {
        v = JSON.parse(json);
      } catch {
        throw new Error('The JSON is not valid');
      }
      if (!v || typeof v !== 'object' || Array.isArray(v)) throw new Error('A record must be a JSON object');
      return v as Record<string, unknown>;
    }
    return fromRows(rows);
  };

  const switchView = (next: 'fields' | 'json') => {
    if (next === view) return;
    try {
      const obj = current();
      if (next === 'json') setJson(JSON.stringify(obj, null, 2));
      else setRows(toRows(obj));
      setView(next);
      setError(null);
    } catch (e) {
      setError((e as Error).message);
    }
  };

  const save = async () => {
    let body: Record<string, unknown>;
    try {
      body = current();
    } catch (e) {
      setError((e as Error).message);
      return;
    }
    setError(null);
    setSaving(true);
    try {
      // Updates keep the server-managed keys the old console sent back; new records omit them.
      const keep: Record<string, unknown> = {};
      if (doc) for (const k of PROTECTED_KEYS) if (doc[k] !== undefined) keep[k] = doc[k];
      await onSave(doc ? { ...body, ...keep } : body);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSaving(false);
    }
  };

  const set = (id: number, patch: Partial<FieldRow>) => setRows((rs) => rs.map((r) => (r.id === id ? { ...r, ...patch } : r)));

  const changeKind = (r: FieldRow, kind: Kind) => {
    let raw = r.raw;
    if (kind === 'boolean') raw = r.raw === 'true' ? 'true' : 'false';
    else if (kind === 'json' && r.kind !== 'json') raw = r.kind === 'text' ? JSON.stringify(r.raw) : r.raw || '{}';
    else if (kind === 'null') raw = '';
    set(r.id, { kind, raw });
  };

  return (
    <Sheet
      width={620}
      open={open}
      onClose={onClose}
      title={doc ? <span className="font-mono text-[15px] break-all">{collection}/{doc._id}</span> : `New record in ${collection}`}
      footer={
        <>
          {doc && onDelete && (
            <Button variant="ghost" className="mr-auto text-danger hover:!text-danger hover:!bg-danger-soft" icon={<Trash2 className="w-4 h-4" />} onClick={onDelete}>
              Delete…
            </Button>
          )}
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" loading={saving} onClick={save}>
            {doc ? 'Save' : 'Create record'}
          </Button>
        </>
      }
    >
      <div className="flex items-center justify-between gap-2 flex-wrap">
        <Segmented
          aria-label="Editor"
          size="sm"
          value={view}
          onChange={switchView}
          options={[
            { value: 'fields', label: 'Fields' },
            { value: 'json', label: 'JSON' },
          ]}
        />
        {doc && <CopyButton text={doc._id} label="Copy id" />}
      </div>

      {error && <Banner tone="danger">{error}</Banner>}

      {view === 'json' ? (
        <Field label="Record as JSON" hint="_id, _createdAt and _updatedAt are set by the server and can't be changed.">
          <Textarea mono rows={18} value={json} onChange={(e) => setJson(e.target.value)} spellCheck={false} />
        </Field>
      ) : (
        <div className="flex flex-col gap-3">
          {rows.length === 0 && <p className="m-0 text-sm text-ink-3">No fields yet.</p>}
          {rows.map((r) => (
            <div key={r.id} className="flex flex-col gap-2 p-3 rounded-md border border-line bg-sunken/40">
              <div className="flex gap-2 items-start">
                <Input mono aria-label="Field name" placeholder="field_name" value={r.key} onChange={(e) => set(r.id, { key: e.target.value })} className="flex-1 min-w-0" />
                <Select aria-label={`Type of ${r.key || 'field'}`} options={KIND_OPTIONS} value={r.kind} onChange={(e) => changeKind(r, e.target.value as Kind)} className="w-[150px] shrink-0" />
                <IconButton label={`Remove ${r.key || 'field'}`} onClick={() => setRows((rs) => rs.filter((x) => x.id !== r.id))}>
                  <X className="w-4 h-4" />
                </IconButton>
              </div>
              {r.kind === 'text' && <Input aria-label={`Value of ${r.key || 'field'}`} value={r.raw} onChange={(e) => set(r.id, { raw: e.target.value })} />}
              {r.kind === 'number' && <Input aria-label={`Value of ${r.key || 'field'}`} inputMode="decimal" mono value={r.raw} onChange={(e) => set(r.id, { raw: e.target.value })} />}
              {r.kind === 'boolean' && (
                <Switch aria-label={`Value of ${r.key || 'field'}`} label={r.raw === 'true' ? 'Yes' : 'No'} checked={r.raw === 'true'} onChange={(v) => set(r.id, { raw: v ? 'true' : 'false' })} />
              )}
              {r.kind === 'json' && (
                <Textarea aria-label={`Value of ${r.key || 'field'}`} mono rows={Math.min(10, Math.max(3, r.raw.split('\n').length))} value={r.raw} onChange={(e) => set(r.id, { raw: e.target.value })} spellCheck={false} />
              )}
            </div>
          ))}
          <Button size="sm" variant="ghost" className="self-start" icon={<Plus className="w-3.5 h-3.5" />} onClick={() => setRows((rs) => [...rs, { id: ++seq, key: '', kind: 'text', raw: '' }])}>
            Add field
          </Button>
        </div>
      )}

      {doc && (
        <dl className="m-0 grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-[12.5px] pt-3 border-t border-line">
          <dt className="text-ink-3">Created</dt>
          <dd className="m-0 text-ink-2">{formatDate(doc._createdAt as string | undefined)}</dd>
          <dt className="text-ink-3">Updated</dt>
          <dd className="m-0 text-ink-2">{formatDate(doc._updatedAt as string | undefined)}</dd>
        </dl>
      )}
    </Sheet>
  );
}

