import { useCallback, useEffect, useMemo, useRef, useState, type DragEvent } from 'react';
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { ChevronLeft, ChevronRight, Folder, FolderPlus, HardDrive, LayoutGrid, List, Plus, Trash2, Upload } from 'lucide-react';
import { api, adminProjectPath, formatBytes } from '../../api';
import { useProject } from '../../shell/ProjectContext';
import { projectPath } from '../../shell/nav';
import {
  Banner,
  Button,
  Card,
  ConfirmDialog,
  DataTable,
  EmptyState,
  Field,
  IconButton,
  Input,
  Page,
  PageHeader,
  SearchInput,
  Segmented,
  Sheet,
  Skeleton,
  cx,
  useToast,
} from '../../ui';
import { timeAgo, useProjectDetail, useProjectUsage } from './home/data';
import { FileSheet } from './storage/FileSheet';
import { bucketPath, extLabel, filePath, fileName, uploadFile, type StorageFile } from './storage/files';

type Entry = { type: 'folder'; name: string; prefix: string; count: number } | { type: 'file'; file: StorageFile };

const LAYOUT_KEY = 'sovrabase_storage_layout';

function readLayout(): 'list' | 'grid' {
  try {
    return localStorage.getItem(LAYOUT_KEY) === 'grid' ? 'grid' : 'list';
  } catch {
    return 'list';
  }
}

export default function ProjectStorage() {
  const { projectId } = useProject();
  const { bucket: selected = null } = useParams<{ bucket?: string }>();
  const [params, setParams] = useSearchParams();
  const prefix = (params.get('path') ?? '').replace(/^\/+|\/+$/g, '');
  const navigate = useNavigate();
  const { showToast } = useToast();
  const { detail } = useProjectDetail(projectId);
  const { usage, refresh: refreshUsage } = useProjectUsage(projectId, 0);

  const [buckets, setBuckets] = useState<string[] | null>(null);
  const [bucketError, setBucketError] = useState<string | null>(null);
  const [files, setFiles] = useState<StorageFile[] | null>(null);
  const [filesError, setFilesError] = useState<string | null>(null);
  const [q, setQ] = useState('');
  const [layout, setLayoutState] = useState<'list' | 'grid'>(readLayout);
  const [newBucket, setNewBucket] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [newFolder, setNewFolder] = useState<string | null>(null);
  const [extraFolders, setExtraFolders] = useState<string[]>([]);
  const [openPath, setOpenPath] = useState<string | null>(null);
  const [deleteFile, setDeleteFile] = useState<StorageFile | null>(null);
  const [deleteBucket, setDeleteBucket] = useState(false);
  const [uploading, setUploading] = useState<{ done: number; total: number } | null>(null);
  const [dragging, setDragging] = useState(false);
  const fileInput = useRef<HTMLInputElement>(null);

  const setLayout = (l: 'list' | 'grid') => {
    setLayoutState(l);
    try {
      localStorage.setItem(LAYOUT_KEY, l);
    } catch {
      /* private mode */
    }
  };

  const loadBuckets = useCallback(async () => {
    try {
      const d = await api<{ buckets: string[] }>(adminProjectPath(projectId, '/storage/buckets'));
      setBuckets([...(d.buckets || [])].sort());
      setBucketError(null);
    } catch (e) {
      setBucketError((e as Error).message);
      setBuckets([]);
    }
  }, [projectId]);

  const loadFiles = useCallback(async () => {
    if (!selected) return;
    try {
      const d = await api<StorageFile[]>(bucketPath(projectId, selected, '/files'));
      setFiles(d || []);
      setFilesError(null);
    } catch (e) {
      setFilesError((e as Error).message);
      setFiles([]);
    }
  }, [projectId, selected]);

  useEffect(() => {
    loadBuckets();
  }, [loadBuckets]);

  useEffect(() => {
    setFiles(null);
    setExtraFolders([]);
    setQ('');
    loadFiles();
  }, [loadFiles]);

  const goTo = (p: string) => {
    const next = new URLSearchParams(params);
    if (p) next.set('path', p);
    else next.delete('path');
    setParams(next);
    setQ('');
  };

  /* Entries of the current folder: sub-folders first, then files. A search looks through the whole bucket. */
  const entries: Entry[] = useMemo(() => {
    const all = files ?? [];
    const needle = q.trim().toLowerCase();
    if (needle) return all.filter((f) => f.path.toLowerCase().includes(needle)).map((file) => ({ type: 'file', file }));
    const base = prefix ? `${prefix}/` : '';
    const folders = new Map<string, number>();
    const here: Entry[] = [];
    for (const f of all) {
      if (!f.path.startsWith(base)) continue;
      const rest = f.path.slice(base.length);
      const i = rest.indexOf('/');
      if (i >= 0) folders.set(rest.slice(0, i), (folders.get(rest.slice(0, i)) ?? 0) + 1);
      else here.push({ type: 'file', file: f });
    }
    for (const ef of extraFolders) {
      if (ef.startsWith(base)) {
        const rest = ef.slice(base.length);
        const name = rest.split('/')[0];
        if (name && !folders.has(name)) folders.set(name, 0);
      }
    }
    const fEntries: Entry[] = [...folders.entries()].sort(([a], [b]) => a.localeCompare(b)).map(([name, count]) => ({ type: 'folder', name, prefix: base + name, count }));
    return [...fEntries, ...here.sort((a, b) => (a.type === 'file' && b.type === 'file' ? fileName(a.file.path).localeCompare(fileName(b.file.path)) : 0))];
  }, [files, prefix, q, extraFolders]);

  const bucketBytes = (files ?? []).reduce((s, f) => s + (f.size ?? 0), 0);
  const openFile = (files ?? []).find((f) => f.path === openPath) ?? null;

  const upload = async (list: FileList | File[]) => {
    if (!selected) return;
    const arr = Array.from(list);
    if (!arr.length) return;
    setUploading({ done: 0, total: arr.length });
    let failed = 0;
    for (let i = 0; i < arr.length; i++) {
      try {
        await uploadFile(projectId, selected, arr[i], prefix ? `${prefix}/${arr[i].name}` : arr[i].name);
      } catch (e) {
        failed++;
        showToast(`${arr[i].name}: ${(e as Error).message}`, 'error');
      }
      setUploading({ done: i + 1, total: arr.length });
    }
    setUploading(null);
    if (arr.length - failed > 0) showToast(arr.length - failed === 1 ? `${arr[0].name} uploaded` : `${arr.length - failed} files uploaded`, 'success');
    if (fileInput.current) fileInput.current.value = '';
    await loadFiles();
    refreshUsage();
  };

  const createBucket = async () => {
    const name = (newBucket ?? '').trim();
    if (!name) return;
    setCreating(true);
    try {
      await api(adminProjectPath(projectId, '/storage/buckets'), { method: 'POST', body: JSON.stringify({ name }) });
      showToast(`Bucket ${name} created`, 'success');
      setNewBucket(null);
      await loadBuckets();
      navigate(projectPath(projectId, `storage/${encodeURIComponent(name)}`));
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setCreating(false);
    }
  };

  const removeBucket = async () => {
    if (!selected) return;
    try {
      await api(bucketPath(projectId, selected), { method: 'DELETE' });
      showToast(`Bucket ${selected} deleted`, 'success');
      await loadBuckets();
      refreshUsage();
      navigate(projectPath(projectId, 'storage'));
    } catch (e) {
      showToast((e as Error).message, 'error');
    }
  };

  const removeFile = async () => {
    if (!deleteFile || !selected) return;
    try {
      await api(filePath(projectId, selected, deleteFile.path), { method: 'DELETE' });
      showToast(`${fileName(deleteFile.path)} deleted`, 'success');
      if (openPath === deleteFile.path) setOpenPath(null);
      setFiles((fs) => (fs ?? []).filter((f) => f.path !== deleteFile.path));
      refreshUsage();
    } catch (e) {
      showToast((e as Error).message, 'error');
    }
  };

  const addFolder = () => {
    const name = (newFolder ?? '').trim().replace(/^\/+|\/+$/g, '');
    if (!name) return;
    const full = prefix ? `${prefix}/${name}` : name;
    setExtraFolders((x) => [...x, full]);
    setNewFolder(null);
    goTo(full);
  };

  const onDrop = (e: DragEvent) => {
    e.preventDefault();
    setDragging(false);
    if (selected && e.dataTransfer.files.length) upload(e.dataTransfer.files);
  };

  const quota = detail?.storage_quota ?? 0;
  const used = usage?.file_storage_bytes ?? usage?.total_storage_bytes;
  const crumbs = prefix ? prefix.split('/') : [];

  return (
    <Page wide>
      <PageHeader
        title="Storage"
        description="Buckets of files your app uploads and serves: images, documents, exports."
        actions={
          selected ? (
            <>
              <Button icon={<FolderPlus className="w-4 h-4" />} onClick={() => setNewFolder('')}>
                New folder
              </Button>
              <Button variant="primary" icon={<Upload className="w-4 h-4" />} loading={!!uploading} onClick={() => fileInput.current?.click()}>
                {uploading ? `Uploading ${uploading.done}/${uploading.total}` : 'Upload'}
              </Button>
            </>
          ) : (
            <Button variant="primary" icon={<Plus className="w-4 h-4" />} onClick={() => setNewBucket('')}>
              New bucket
            </Button>
          )
        }
      />
      <input ref={fileInput} type="file" multiple hidden onChange={(e) => e.target.files && upload(e.target.files)} />

      {bucketError && (
        <div className="mb-4">
          <Banner tone="danger" title="Buckets could not be loaded" action={<Button size="sm" onClick={loadBuckets}>Retry</Button>}>
            {bucketError}
          </Banner>
        </div>
      )}

      <div className="flex flex-col md:flex-row gap-5 items-start">
        <nav aria-label="Buckets" className={cx('w-full md:w-[240px] md:shrink-0 md:sticky md:top-4', selected && 'hidden md:block')}>
          <Card padded={false} className="p-2">
            <div className="flex items-center justify-between gap-2 px-2 pt-1 pb-2">
              <h2 className="m-0 text-[13px] font-semibold text-ink-3 uppercase tracking-[.05em]">Buckets</h2>
              <IconButton size="sm" label="New bucket" onClick={() => setNewBucket('')}>
                <Plus className="w-4 h-4" />
              </IconButton>
            </div>
            {!buckets ? (
              <div className="flex flex-col gap-1.5 p-1">
                {[0, 1, 2].map((i) => (
                  <Skeleton key={i} className="h-8 w-full" />
                ))}
              </div>
            ) : buckets.length === 0 ? (
              <p className="m-0 px-2 py-3 text-[13px] text-ink-3">No buckets yet.</p>
            ) : (
              <ul className="m-0 p-0 list-none flex flex-col gap-0.5">
                {buckets.map((b) => (
                  <li key={b}>
                    <Link
                      to={projectPath(projectId, `storage/${encodeURIComponent(b)}`)}
                      aria-current={selected === b ? 'page' : undefined}
                      className={cx(
                        'flex items-center justify-between gap-2 h-9 px-3 rounded-md text-sm',
                        selected === b ? 'bg-accent-soft text-accent-2 font-medium' : 'text-ink-2 hover:bg-hover hover:text-ink',
                      )}
                    >
                      <span className="truncate">{b}</span>
                      {selected === b && files && <span className="text-[12px] text-ink-3 tabular-nums">{formatBytes(bucketBytes)}</span>}
                    </Link>
                  </li>
                ))}
              </ul>
            )}
            {used != null && (
              <div className="mt-2 mx-1 px-2 pt-3 pb-1 border-t border-line flex flex-col gap-1.5">
                <span className="text-[12.5px] text-ink-3">
                  Project total <b className="font-semibold text-ink">{formatBytes(used)}</b>
                  {quota > 0 && <> of {formatBytes(quota)}</>}
                </span>
                {quota > 0 && (
                  <div className="h-1.5 rounded-full bg-sunken overflow-hidden" role="meter" aria-label="Storage quota used" aria-valuemin={0} aria-valuemax={quota} aria-valuenow={used}>
                    <div className={cx('h-full rounded-full', used / quota > 0.9 ? 'bg-danger' : used / quota > 0.75 ? 'bg-warn' : 'bg-accent')} style={{ width: `${Math.min(100, (used / quota) * 100)}%` }} />
                  </div>
                )}
              </div>
            )}
          </Card>
        </nav>

        <div className="flex-1 min-w-0 w-full">
          {!selected ? (
            <Card className="hidden md:block">
              {buckets && buckets.length === 0 ? (
                <EmptyState
                  icon={<HardDrive className="w-5 h-5" />}
                  title="Create your first bucket"
                  description="A bucket groups files, for example avatars or invoices."
                  action={
                    <Button variant="primary" onClick={() => setNewBucket('')}>
                      New bucket
                    </Button>
                  }
                />
              ) : (
                <EmptyState icon={<HardDrive className="w-5 h-5" />} title="Pick a bucket" description="Choose one on the left to browse and upload files." />
              )}
            </Card>
          ) : buckets && !buckets.includes(selected) ? (
            <EmptyState title={`There is no bucket named ${selected}`} description="It may have been deleted." action={<Button onClick={() => navigate(projectPath(projectId, 'storage'))}>All buckets</Button>} />
          ) : (
            <div
              className="flex flex-col gap-3"
              onDragOver={(e) => {
                e.preventDefault();
                setDragging(true);
              }}
              onDragLeave={(e) => {
                if (!e.currentTarget.contains(e.relatedTarget as Node)) setDragging(false);
              }}
              onDrop={onDrop}
            >
              <Link to={projectPath(projectId, 'storage')} className="md:hidden inline-flex items-center gap-1 text-[13px] text-ink-3 hover:text-ink">
                <ChevronLeft className="w-4 h-4" /> All buckets
              </Link>

              <div className="flex flex-wrap items-center justify-between gap-3">
                <nav aria-label="Folder" className="flex items-center flex-wrap gap-1 text-[15px] min-w-0">
                  <button type="button" onClick={() => goTo('')} className={cx('font-semibold hover:text-accent', crumbs.length ? 'text-ink-2' : 'text-ink')}>
                    {selected}
                  </button>
                  {crumbs.map((c, i) => (
                    <span key={i} className="inline-flex items-center gap-1 min-w-0">
                      <ChevronRight className="w-4 h-4 text-ink-3" aria-hidden />
                      <button
                        type="button"
                        onClick={() => goTo(crumbs.slice(0, i + 1).join('/'))}
                        aria-current={i === crumbs.length - 1 ? 'location' : undefined}
                        className={cx('truncate hover:text-accent', i === crumbs.length - 1 ? 'font-semibold text-ink' : 'text-ink-2')}
                      >
                        {c}
                      </button>
                    </span>
                  ))}
                </nav>
                <Button size="sm" variant="ghost" className="hover:!text-danger hover:!bg-danger-soft" icon={<Trash2 className="w-3.5 h-3.5" />} onClick={() => setDeleteBucket(true)}>
                  Delete bucket…
                </Button>
              </div>

              <div className="flex flex-wrap items-center gap-2">
                <SearchInput value={q} onChange={setQ} placeholder="Find files in this bucket" className="flex-[1_1_220px] min-w-0" />
                <Segmented
                  aria-label="Layout"
                  size="sm"
                  value={layout}
                  onChange={setLayout}
                  options={[
                    { value: 'list', label: <><List className="w-3.5 h-3.5" aria-hidden /> List</> },
                    { value: 'grid', label: <><LayoutGrid className="w-3.5 h-3.5" aria-hidden /> Grid</> },
                  ]}
                />
              </div>

              {filesError && <Banner tone="danger" title="Files could not be loaded">{filesError}</Banner>}

              <Card padded={false} className={cx('relative', dragging && 'ring-2 ring-accent')}>
                {dragging && (
                  <div className="absolute inset-0 z-10 rounded-lg bg-accent-soft/90 flex items-center justify-center text-accent-2 text-sm font-medium pointer-events-none">
                    Drop to upload to {prefix ? `${selected}/${prefix}` : selected}
                  </div>
                )}
                {!files ? (
                  <div className="p-5 flex flex-col gap-2">
                    {[0, 1, 2, 3].map((i) => (
                      <Skeleton key={i} className="h-9 w-full" />
                    ))}
                  </div>
                ) : entries.length === 0 ? (
                  q ? (
                    <EmptyState title="No matching files" />
                  ) : (
                    <EmptyState
                      icon={<Upload className="w-5 h-5" />}
                      title={prefix ? 'This folder is empty' : `${selected} is empty`}
                      description="Drop files here, or choose them from your computer."
                      action={
                        <Button variant="primary" onClick={() => fileInput.current?.click()}>
                          Upload files
                        </Button>
                      }
                    />
                  )
                ) : layout === 'grid' ? (
                  <ul className="m-0 p-3 list-none grid grid-cols-[repeat(auto-fill,minmax(140px,1fr))] gap-3">
                    {entries.map((e) => (
                      <li key={e.type === 'folder' ? `d:${e.prefix}` : e.file.path}>
                        <button
                          type="button"
                          onClick={() => (e.type === 'folder' ? goTo(e.prefix) : setOpenPath(e.file.path))}
                          className={cx(
                            'w-full flex flex-col gap-2 p-2 rounded-lg border text-left hover:border-ink-3',
                            e.type === 'file' && openPath === e.file.path ? 'border-accent bg-accent-soft/50' : 'border-line',
                          )}
                        >
                          <span className="aspect-[4/3] rounded-md bg-sunken flex items-center justify-center text-ink-3">
                            {e.type === 'folder' ? <Folder className="w-8 h-8" /> : <span className="font-mono text-[12px] font-semibold">{extLabel(e.file.path, e.file.content_type)}</span>}
                          </span>
                          <span className="text-[13px] text-ink truncate">{e.type === 'folder' ? e.name : fileName(e.file.path)}</span>
                          <span className="text-[12px] text-ink-3 -mt-1.5">
                            {e.type === 'folder' ? `${e.count} file${e.count === 1 ? '' : 's'}` : e.file.size != null ? formatBytes(e.file.size) : ''}
                          </span>
                        </button>
                      </li>
                    ))}
                  </ul>
                ) : (
                  <DataTable<Entry>
                    rows={entries}
                    rowKey={(e) => (e.type === 'folder' ? `d:${e.prefix}` : e.file.path)}
                    selectedKey={openPath}
                    onRowClick={(e) => (e.type === 'folder' ? goTo(e.prefix) : setOpenPath(e.file.path))}
                    columns={[
                      {
                        key: 'name',
                        header: 'Name',
                        cell: (e) =>
                          e.type === 'folder' ? (
                            <span className="flex items-center gap-2 min-w-0">
                              <Folder className="w-4 h-4 text-ink-3 shrink-0" aria-hidden />
                              <span className="truncate text-ink">{e.name}</span>
                            </span>
                          ) : (
                            <span className="flex items-center gap-2 min-w-0">
                              <span className="w-9 shrink-0 font-mono text-[10.5px] font-semibold text-ink-3">{extLabel(e.file.path, e.file.content_type)}</span>
                              <span className="truncate text-ink">{q ? e.file.path : fileName(e.file.path)}</span>
                            </span>
                          ),
                      },
                      {
                        key: 'size',
                        header: 'Size',
                        width: '100px',
                        align: 'right',
                        cell: (e) => <span className="text-ink-2 tabular-nums">{e.type === 'folder' ? `${e.count} file${e.count === 1 ? '' : 's'}` : e.file.size != null ? formatBytes(e.file.size) : '—'}</span>,
                      },
                      { key: 'type', header: 'Type', width: '160px', hideOnMobile: true, cell: (e) => <span className="text-ink-3 truncate block">{e.type === 'file' ? e.file.content_type || '—' : 'Folder'}</span> },
                      { key: 'at', header: 'Updated', width: '120px', align: 'right', cell: (e) => <span className="text-ink-3">{e.type === 'file' ? timeAgo(e.file.updated_at || e.file.created_at) : ''}</span> },
                    ]}
                  />
                )}
              </Card>
              {files && files.length > 0 && <p className="m-0 text-[12.5px] text-ink-3">Drop files anywhere here to upload them to {prefix ? `${prefix}/` : 'the top of the bucket'}.</p>}
            </div>
          )}
        </div>
      </div>

      {selected && <FileSheet projectId={projectId} bucket={selected} file={openFile} onClose={() => setOpenPath(null)} onDelete={setDeleteFile} />}

      <Sheet
        open={newBucket !== null}
        onClose={() => setNewBucket(null)}
        title="New bucket"
        description="A bucket groups files. You can make folders inside it."
        footer={
          <>
            <Button variant="ghost" onClick={() => setNewBucket(null)}>
              Cancel
            </Button>
            <Button variant="primary" loading={creating} disabled={!newBucket?.trim()} onClick={createBucket}>
              Create bucket
            </Button>
          </>
        }
      >
        <Field label="Name" hint="Lowercase letters, numbers and dashes, e.g. avatars or product-images.">
          <Input mono value={newBucket ?? ''} onChange={(e) => setNewBucket(e.target.value)} placeholder="avatars" onKeyDown={(e) => e.key === 'Enter' && createBucket()} />
        </Field>
      </Sheet>

      <Sheet
        open={newFolder !== null}
        onClose={() => setNewFolder(null)}
        title="New folder"
        description={`Inside ${selected}${prefix ? `/${prefix}` : ''}. It is saved when you upload the first file into it.`}
        footer={
          <>
            <Button variant="ghost" onClick={() => setNewFolder(null)}>
              Cancel
            </Button>
            <Button variant="primary" disabled={!newFolder?.trim()} onClick={addFolder}>
              Open folder
            </Button>
          </>
        }
      >
        <Field label="Folder name" hint="Use / to create nested folders, e.g. 2026/autumn.">
          <Input mono value={newFolder ?? ''} onChange={(e) => setNewFolder(e.target.value)} placeholder="2026/autumn" onKeyDown={(e) => e.key === 'Enter' && addFolder()} />
        </Field>
      </Sheet>

      <ConfirmDialog open={!!deleteFile} onClose={() => setDeleteFile(null)} onConfirm={removeFile} title={`Delete ${deleteFile ? fileName(deleteFile.path) : ''}?`} confirmLabel="Delete file">
        Links to this file stop working. This can't be undone.
      </ConfirmDialog>
      <ConfirmDialog
        open={deleteBucket}
        onClose={() => setDeleteBucket(false)}
        onConfirm={removeBucket}
        title={`Delete bucket ${selected}?`}
        confirmLabel="Delete bucket"
        confirmText={selected ?? undefined}
      >
        The bucket and its {files ? `${files.length} file${files.length === 1 ? '' : 's'} (${formatBytes(bucketBytes)})` : 'files'} are deleted for good.
      </ConfirmDialog>
    </Page>
  );
}
