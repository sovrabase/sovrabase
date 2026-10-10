import { useEffect, useMemo, useRef, useState } from 'react';
import { FileJson, Plus, Upload, X } from 'lucide-react';
import { api } from '../../../api';
import { Banner, Button, Card, ConfirmDialog, DataTable, EmptyState, Field, Input, SearchInput, Select, Sheet, Skeleton, Textarea, useToast, type Column } from '../../../ui';
import { timeAgo } from '../home/data';
import { RecordSheet } from './RecordSheet';
import { allFields, colPath, pickColumns, preview, stripProtected, type Doc } from './shared';

const PAGE = 50;

export interface ServerFilter {
  key: string;
  value: string;
}

export function RecordsPanel({
  projectId,
  collection,
  docs,
  loading,
  filter,
  onFilter,
  reload,
}: {
  projectId: string;
  collection: string;
  docs: Doc[] | null;
  loading: boolean;
  filter: ServerFilter | null;
  onFilter: (f: ServerFilter | null) => void;
  reload: () => Promise<Doc[]>;
}) {
  const { showToast } = useToast();
  const [q, setQ] = useState('');
  const [page, setPage] = useState(0);
  const [openId, setOpenId] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [importing, setImporting] = useState(false);
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [fKey, setFKey] = useState('');
  const [fVal, setFVal] = useState('');

  useEffect(() => {
    setQ('');
    setPage(0);
    setOpenId(null);
  }, [collection]);
  useEffect(() => setPage(0), [q, filter]);

  const rows = useMemo(() => {
    const list = docs ?? [];
    const needle = q.trim().toLowerCase();
    if (!needle) return list;
    return list.filter((d) => Object.values(d).some((v) => (typeof v === 'object' ? JSON.stringify(v) : String(v)).toLowerCase().includes(needle)));
  }, [docs, q]);

  const cols = useMemo(() => pickColumns(docs ?? []), [docs]);
  const fields = useMemo(() => allFields(docs ?? []), [docs]);
  const pageRows = rows.slice(page * PAGE, page * PAGE + PAGE);
  const pages = Math.max(1, Math.ceil(rows.length / PAGE));
  const openDoc = (docs ?? []).find((d) => d._id === openId) ?? null;

  const columns: Column<Doc>[] = [
    { key: '_id', header: 'id', width: 'minmax(110px,180px)', cell: (d) => <span className="font-mono text-[12.5px] text-ink truncate block">{d._id}</span> },
    ...cols.map<Column<Doc>>((k) => ({
      key: k,
      header: k,
      cell: (d) => <span className="truncate block text-ink-2">{preview(d[k])}</span>,
    })),
    {
      key: '_createdAt',
      header: 'Created',
      width: '120px',
      align: 'right',
      hideOnMobile: cols.length > 1,
      cell: (d) => <span className="text-ink-3 whitespace-nowrap">{timeAgo(d._createdAt as string | undefined)}</span>,
    },
  ];

  const docPath = (id: string) => colPath(projectId, collection, `/documents/${encodeURIComponent(id)}`);

  const saveDoc = async (body: Record<string, unknown>) => {
    if (openDoc) {
      await api(docPath(openDoc._id), { method: 'PUT', body: JSON.stringify(body) });
      showToast('Record saved', 'success');
    } else {
      await api(colPath(projectId, collection, '/documents'), { method: 'POST', body: JSON.stringify(stripProtected(body)) });
      showToast('Record created', 'success');
      setCreating(false);
    }
    await reload();
  };

  const deleteDoc = async () => {
    if (!deleteId) return;
    try {
      await api(docPath(deleteId), { method: 'DELETE' });
      showToast('Record deleted', 'success');
      if (openId === deleteId) setOpenId(null);
      await reload();
    } catch (e) {
      showToast((e as Error).message, 'error');
    }
  };

  const applyFilter = () => {
    if (!fKey) return;
    onFilter({ key: fKey, value: fVal });
  };

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <SearchInput value={q} onChange={setQ} placeholder="Search records" className="flex-[1_1_200px] min-w-0" />
        <div className="flex items-center gap-2 flex-wrap">
          <Button size="sm" icon={<Upload className="w-3.5 h-3.5" />} onClick={() => setImporting(true)}>
            Import
          </Button>
          <Button size="sm" variant="primary" icon={<Plus className="w-3.5 h-3.5" />} onClick={() => setCreating(true)}>
            New record
          </Button>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        {filter ? (
          <span className="inline-flex items-center gap-1 h-[30px] pl-2.5 pr-1 rounded-full bg-accent-soft text-accent-2 text-[13px]">
            <span className="font-mono">{filter.key}</span> contains “{filter.value}”
            <button type="button" aria-label="Remove filter" className="w-6 h-6 inline-flex items-center justify-center rounded-full hover:bg-surface/60" onClick={() => onFilter(null)}>
              <X className="w-3.5 h-3.5" />
            </button>
          </span>
        ) : (
          <form
            className="flex flex-wrap items-center gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              applyFilter();
            }}
          >
            <Select aria-label="Filter field" value={fKey} onChange={(e) => setFKey(e.target.value)} placeholder="Filter by field" className="h-[30px] text-[13px] w-[160px]" options={fields.map((f) => ({ value: f, label: f }))} />
            <Input aria-label="Filter value" placeholder="contains…" value={fVal} onChange={(e) => setFVal(e.target.value)} className="h-[30px] text-[13px] w-[160px]" />
            <Button size="sm" type="submit" disabled={!fKey}>
              Apply
            </Button>
          </form>
        )}
        <span className="ml-auto text-[12.5px] text-ink-3 tabular-nums">
          {docs ? `${rows.length.toLocaleString()} ${filter || q ? 'matching' : rows.length === 1 ? 'record' : 'records'}` : ''}
        </span>
      </div>

      <Card padded={false}>
        {loading && !docs ? (
          <div className="p-5 flex flex-col gap-2">
            {[0, 1, 2, 3, 4].map((i) => (
              <Skeleton key={i} className="h-9 w-full" />
            ))}
          </div>
        ) : (
          <DataTable
            columns={columns}
            rows={pageRows}
            rowKey={(d) => d._id}
            selectedKey={openId}
            onRowClick={(d) => setOpenId(d._id)}
            empty={
              q || filter ? (
                <EmptyState title="No matching records" description="Try another search or remove the filter." />
              ) : (
                <EmptyState
                  icon={<FileJson className="w-5 h-5" />}
                  title={`${collection} is empty`}
                  description="Add a record by hand, import a JSON array, or write from your app."
                  action={
                    <Button variant="primary" onClick={() => setCreating(true)}>
                      New record
                    </Button>
                  }
                />
              )
            }
          />
        )}
        {pages > 1 && (
          <div className="flex items-center justify-between gap-2 px-5 py-3 border-t border-line text-[12.5px] text-ink-3">
            <span className="tabular-nums">
              {page * PAGE + 1}–{Math.min(rows.length, (page + 1) * PAGE)} of {rows.length.toLocaleString()}
            </span>
            <div className="flex gap-2">
              <Button size="sm" disabled={page === 0} onClick={() => setPage((p) => p - 1)}>
                Previous
              </Button>
              <Button size="sm" disabled={page >= pages - 1} onClick={() => setPage((p) => p + 1)}>
                Next
              </Button>
            </div>
          </div>
        )}
      </Card>

      <RecordSheet
        open={!!openDoc || creating}
        onClose={() => {
          setOpenId(null);
          setCreating(false);
        }}
        collection={collection}
        doc={creating ? null : openDoc}
        onSave={saveDoc}
        onDelete={openDoc ? () => setDeleteId(openDoc._id) : undefined}
      />

      <ImportSheet open={importing} onClose={() => setImporting(false)} projectId={projectId} collection={collection} onDone={reload} />

      <ConfirmDialog open={!!deleteId} onClose={() => setDeleteId(null)} onConfirm={deleteDoc} title="Delete this record?" confirmLabel="Delete record">
        <code className="font-mono text-[13px] break-all">{deleteId}</code> is removed from {collection}. This can't be undone.
      </ConfirmDialog>
    </div>
  );
}

function ImportSheet({
  open,
  onClose,
  projectId,
  collection,
  onDone,
}: {
  open: boolean;
  onClose: () => void;
  projectId: string;
  collection: string;
  onDone: () => Promise<unknown>;
}) {
  const { showToast } = useToast();
  const [text, setText] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (open) {
      setText('');
      setError(null);
    }
  }, [open]);

  let count: number | null = null;
  try {
    const v = JSON.parse(text);
    if (Array.isArray(v)) count = v.length;
  } catch {
    /* shown on submit */
  }

  const readFile = async (f: File) => setText(await f.text());

  const submit = async () => {
    let arr: unknown;
    try {
      arr = JSON.parse(text);
    } catch {
      setError('That is not valid JSON.');
      return;
    }
    if (!Array.isArray(arr)) {
      setError('Paste a JSON array of records, e.g. [{"name": "Alice"}].');
      return;
    }
    setBusy(true);
    try {
      await api(colPath(projectId, collection, '/import'), { method: 'POST', body: JSON.stringify(arr) });
      showToast(`${arr.length} records imported`, 'success');
      onClose();
      await onDone();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Sheet
      open={open}
      onClose={onClose}
      title={`Import into ${collection}`}
      description="Each object becomes a record. Include _id to choose its id."
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" loading={busy} disabled={!text.trim()} onClick={submit}>
            {count != null ? `Import ${count} record${count === 1 ? '' : 's'}` : 'Import'}
          </Button>
        </>
      }
    >
      {error && <Banner tone="danger">{error}</Banner>}
      <div className="flex items-center gap-3 flex-wrap">
        <input ref={fileRef} type="file" accept="application/json,.json" hidden onChange={(e) => e.target.files?.[0] && readFile(e.target.files[0])} />
        <Button size="sm" icon={<Upload className="w-3.5 h-3.5" />} onClick={() => fileRef.current?.click()}>
          Choose a JSON file
        </Button>
        <span className="text-[12.5px] text-ink-3">or paste the content below.</span>
      </div>
      <Field label="Records" hint="A JSON array of objects.">
        <Textarea mono rows={14} value={text} onChange={(e) => setText(e.target.value)} placeholder='[{"name": "Alice"}, {"name": "Bob"}]' spellCheck={false} />
      </Field>
    </Sheet>
  );
}
