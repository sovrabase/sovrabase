import { useCallback, useEffect, useState } from 'react';
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { ChevronLeft, Database as DbIcon, Eraser, Lock, Plus, Trash2 } from 'lucide-react';
import { api, adminProjectPath } from '../../api';
import { useProject } from '../../shell/ProjectContext';
import { projectPath } from '../../shell/nav';
import {
  Badge,
  Banner,
  Button,
  Card,
  ConfirmDialog,
  EmptyState,
  Field,
  IconButton,
  Input,
  Page,
  PageHeader,
  SearchInput,
  Sheet,
  Skeleton,
  Tabs,
  cx,
  useToast,
} from '../../ui';
import { IndexesPanel } from './database/IndexesPanel';
import { RecordsPanel, type ServerFilter } from './database/RecordsPanel';
import { RulesPanel } from './database/RulesPanel';
import { allFields, colPath, isReserved, type Doc } from './database/shared';

interface CollectionInfo {
  name: string;
  doc_count?: number;
}

type View = 'records' | 'access' | 'indexes';
const VIEWS: View[] = ['records', 'access', 'indexes'];

export default function ProjectDatabase() {
  const { projectId } = useProject();
  const { collection: selected = null } = useParams<{ collection?: string }>();
  const navigate = useNavigate();
  const { showToast } = useToast();
  const [params, setParams] = useSearchParams();
  const view: View = VIEWS.includes(params.get('view') as View) ? (params.get('view') as View) : 'records';

  const [collections, setCollections] = useState<CollectionInfo[] | null>(null);
  const [listError, setListError] = useState<string | null>(null);
  const [colQuery, setColQuery] = useState('');
  const [newOpen, setNewOpen] = useState(false);
  const [newName, setNewName] = useState('');
  const [creating, setCreating] = useState(false);
  const [confirm, setConfirm] = useState<'clear' | 'drop' | null>(null);

  const [docs, setDocs] = useState<Doc[] | null>(null);
  const [docsLoading, setDocsLoading] = useState(false);
  const [docsError, setDocsError] = useState<string | null>(null);
  // A server-side filter belongs to one collection; switching collections drops it.
  const [filterState, setFilterState] = useState<{ col: string; f: ServerFilter } | null>(null);
  const filter = filterState && filterState.col === selected ? filterState.f : null;
  const setFilter = (f: ServerFilter | null) => setFilterState(f && selected ? { col: selected, f } : null);

  const loadCollections = useCallback(async () => {
    try {
      const d = await api<{ collections: (string | CollectionInfo)[] }>(adminProjectPath(projectId, '/collections'));
      setCollections((d.collections || []).map((c) => (typeof c === 'string' ? { name: c } : c)).sort((a, b) => a.name.localeCompare(b.name)));
      setListError(null);
    } catch (e) {
      setListError((e as Error).message);
      setCollections([]);
    }
  }, [projectId]);

  useEffect(() => {
    loadCollections();
  }, [loadCollections]);

  const loadDocs = useCallback(async (): Promise<Doc[]> => {
    if (!selected) return [];
    setDocsLoading(true);
    try {
      const qs = filter ? `?filter_key=${encodeURIComponent(filter.key)}&filter_val=${encodeURIComponent(filter.value)}` : '';
      const d = await api<Doc[]>(colPath(projectId, selected, `/documents${qs}`));
      const arr = Array.isArray(d) ? d : [];
      setDocs(arr);
      setDocsError(null);
      return arr;
    } catch (e) {
      setDocsError((e as Error).message);
      setDocs([]);
      return [];
    } finally {
      setDocsLoading(false);
    }
  }, [projectId, selected, filter]);

  useEffect(() => {
    setDocs(null);
  }, [selected]);

  useEffect(() => {
    loadDocs();
  }, [loadDocs]);

  const setView = (v: View) => {
    const next = new URLSearchParams(params);
    if (v === 'records') next.delete('view');
    else next.set('view', v);
    setParams(next, { replace: true });
  };

  const createCollection = async () => {
    const name = newName.trim();
    if (!name) return;
    setCreating(true);
    try {
      await api(adminProjectPath(projectId, '/collections'), { method: 'POST', body: JSON.stringify({ name }) });
      showToast(`Collection ${name} created`, 'success');
      setNewOpen(false);
      setNewName('');
      await loadCollections();
      navigate(projectPath(projectId, `database/${encodeURIComponent(name)}`));
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setCreating(false);
    }
  };

  const clearCollection = async () => {
    if (!selected) return;
    try {
      await api(colPath(projectId, selected, '/clear'), { method: 'POST' });
      showToast(`${selected} cleared`, 'success');
      await loadDocs();
    } catch (e) {
      showToast((e as Error).message, 'error');
    }
  };

  const dropCollection = async () => {
    if (!selected) return;
    try {
      await api(colPath(projectId, selected), { method: 'DELETE' });
      showToast(`${selected} dropped`, 'success');
      await loadCollections();
      navigate(projectPath(projectId, 'database'));
    } catch (e) {
      showToast((e as Error).message, 'error');
    }
  };

  const shown = (collections ?? []).filter((c) => c.name.toLowerCase().includes(colQuery.trim().toLowerCase()));
  const userCols = shown.filter((c) => !c.name.startsWith('_'));
  const systemCols = shown.filter((c) => c.name.startsWith('_'));
  const selectedExists = !selected || !collections || collections.some((c) => c.name === selected);

  const colLink = (c: CollectionInfo) => (
    <li key={c.name}>
      <Link
        to={projectPath(projectId, `database/${encodeURIComponent(c.name)}`)}
        aria-current={selected === c.name ? 'page' : undefined}
        className={cx(
          'flex items-center justify-between gap-2 h-9 px-3 rounded-md text-sm',
          selected === c.name ? 'bg-accent-soft text-accent-2 font-medium' : 'text-ink-2 hover:bg-hover hover:text-ink',
        )}
      >
        <span className="font-mono text-[13px] truncate">{c.name}</span>
        {c.doc_count != null && <span className="text-[12px] text-ink-3 tabular-nums">{c.doc_count.toLocaleString()}</span>}
        {c.doc_count == null && isReserved(c.name) && <Lock className="w-3 h-3 text-ink-3 shrink-0" aria-label="System collection" />}
      </Link>
    </li>
  );

  return (
    <Page wide>
      <PageHeader
        title="Database"
        description="Collections of JSON records, who can read and write them, and their indexes."
        actions={
          <Button variant={collections?.length ? 'secondary' : 'primary'} icon={<Plus className="w-4 h-4" />} onClick={() => setNewOpen(true)}>
            New collection
          </Button>
        }
      />

      {listError && (
        <div className="mb-4">
          <Banner tone="danger" title="Collections could not be loaded" action={<Button size="sm" onClick={loadCollections}>Retry</Button>}>
            {listError}
          </Banner>
        </div>
      )}

      <div className="flex flex-col md:flex-row gap-5 items-start">
        {/* Collection list: on phones it is the first screen; once one is open, a back link replaces it. */}
        <nav aria-label="Collections" className={cx('w-full md:w-[240px] md:shrink-0 md:sticky md:top-4', selected && 'hidden md:block')}>
          <Card padded={false} className="p-2">
            <div className="flex items-center justify-between gap-2 px-2 pt-1 pb-2">
              <h2 className="m-0 text-[13px] font-semibold text-ink-3 uppercase tracking-[.05em]">Collections</h2>
              <IconButton size="sm" label="New collection" onClick={() => setNewOpen(true)}>
                <Plus className="w-4 h-4" />
              </IconButton>
            </div>
            {(collections?.length ?? 0) > 8 && <SearchInput value={colQuery} onChange={setColQuery} placeholder="Find a collection" className="mb-2" />}
            {!collections ? (
              <div className="flex flex-col gap-1.5 p-1">
                {[0, 1, 2, 3].map((i) => (
                  <Skeleton key={i} className="h-8 w-full" />
                ))}
              </div>
            ) : collections.length === 0 ? (
              <p className="m-0 px-2 py-3 text-[13px] text-ink-3">No collections yet.</p>
            ) : (
              <>
                <ul className="m-0 p-0 list-none flex flex-col gap-0.5">{userCols.map(colLink)}</ul>
                {systemCols.length > 0 && (
                  <>
                    <p className="m-0 mt-3 mb-1 px-3 text-[12px] text-ink-3">System</p>
                    <ul className="m-0 p-0 list-none flex flex-col gap-0.5">{systemCols.map(colLink)}</ul>
                  </>
                )}
              </>
            )}
          </Card>
        </nav>

        <div className="flex-1 min-w-0 w-full">
          {!selected ? (
            <Card className="hidden md:block">
              {collections && collections.length === 0 ? (
                <EmptyState
                  icon={<DbIcon className="w-5 h-5" />}
                  title="Create your first collection"
                  description="A collection holds records of one kind, like orders or customers. You can add fields freely; no schema needed."
                  action={
                    <Button variant="primary" onClick={() => setNewOpen(true)}>
                      New collection
                    </Button>
                  }
                />
              ) : (
                <EmptyState icon={<DbIcon className="w-5 h-5" />} title="Pick a collection" description="Choose one on the left to see its records, access rules and indexes." />
              )}
            </Card>
          ) : !selectedExists ? (
            <EmptyState
              title={`There is no collection named ${selected}`}
              description="It may have been dropped."
              action={
                <Button onClick={() => navigate(projectPath(projectId, 'database'))}>All collections</Button>
              }
            />
          ) : (
            <div className="flex flex-col gap-4">
              <Link to={projectPath(projectId, 'database')} className="md:hidden inline-flex items-center gap-1 text-[13px] text-ink-3 hover:text-ink">
                <ChevronLeft className="w-4 h-4" /> All collections
              </Link>
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="flex items-center gap-2 min-w-0">
                  <h2 className="m-0 text-[17px] font-semibold text-ink font-mono truncate">{selected}</h2>
                  {isReserved(selected) && <Badge>System</Badge>}
                </div>
                {!isReserved(selected) && (
                  <div className="flex items-center gap-1">
                    <Button size="sm" variant="ghost" icon={<Eraser className="w-3.5 h-3.5" />} onClick={() => setConfirm('clear')}>
                      Clear…
                    </Button>
                    <Button size="sm" variant="ghost" className="hover:!text-danger hover:!bg-danger-soft" icon={<Trash2 className="w-3.5 h-3.5" />} onClick={() => setConfirm('drop')}>
                      Drop…
                    </Button>
                  </div>
                )}
              </div>

              <Tabs<View>
                value={view}
                onChange={setView}
                tabs={[
                  { value: 'records', label: 'Records', count: docs && !filter ? docs.length : undefined },
                  { value: 'access', label: 'Access rules' },
                  { value: 'indexes', label: 'Indexes' },
                ]}
              />

              {view === 'records' &&
                (docsError ? (
                  <Banner tone="danger" title="Records could not be loaded" action={<Button size="sm" onClick={() => loadDocs()}>Retry</Button>}>
                    {docsError}
                  </Banner>
                ) : (
                  <RecordsPanel projectId={projectId} collection={selected} docs={docs} loading={docsLoading} filter={filter} onFilter={setFilter} reload={loadDocs} />
                ))}
              {view === 'access' && <RulesPanel key={selected} projectId={projectId} collection={selected} />}
              {view === 'indexes' && <IndexesPanel projectId={projectId} collection={selected} fields={allFields(docs ?? [])} />}
            </div>
          )}
        </div>
      </div>

      <Sheet
        open={newOpen}
        onClose={() => setNewOpen(false)}
        title="New collection"
        description="Records in a collection are free-form JSON. Add access rules and indexes after."
        footer={
          <>
            <Button variant="ghost" onClick={() => setNewOpen(false)}>
              Cancel
            </Button>
            <Button variant="primary" loading={creating} disabled={!newName.trim()} onClick={createCollection}>
              Create collection
            </Button>
          </>
        }
      >
        <Field label="Name" hint="Lowercase letters, numbers and underscores read best, e.g. orders or order_items.">
          <Input mono value={newName} onChange={(e) => setNewName(e.target.value)} placeholder="orders" autoFocus onKeyDown={(e) => e.key === 'Enter' && createCollection()} />
        </Field>
      </Sheet>

      <ConfirmDialog open={confirm === 'clear'} onClose={() => setConfirm(null)} onConfirm={clearCollection} title={`Clear ${selected}?`} confirmLabel="Clear records">
        Every record in <b className="font-mono">{selected}</b> is deleted. The collection, its access rules and indexes stay.
      </ConfirmDialog>
      <ConfirmDialog
        open={confirm === 'drop'}
        onClose={() => setConfirm(null)}
        onConfirm={dropCollection}
        title={`Drop ${selected}?`}
        confirmLabel="Drop collection"
        confirmText={selected ?? undefined}
      >
        The collection and all {docs && !filter ? docs.length.toLocaleString() : 'its'} records are deleted for good, along with its rules and indexes.
      </ConfirmDialog>
    </Page>
  );
}
