import { useCallback, useEffect, useState } from 'react';
import { RefreshCw, ScrollText } from 'lucide-react';
import { api } from '../../../api';
import { Button, Card, ConfirmDialog, DataTable, EmptyState, IconButton, SearchInput, Skeleton, useToast, type Column } from '../../../ui';
import { shortDateTime } from './shared';

interface AuditEntry {
  timestamp: string;
  admin: string;
  action: string;
  target: string;
  details: string;
}

const LIMIT = 50;

export function AuditSection({ canClear }: { canClear: boolean }) {
  const { showToast } = useToast();
  const [rows, setRows] = useState<AuditEntry[] | null>(null);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [action, setAction] = useState('');
  const [target, setTarget] = useState('');
  const [loading, setLoading] = useState(false);
  const [clearOpen, setClearOpen] = useState(false);

  const load = useCallback(async (off: number, a: string, t: string) => {
    setLoading(true);
    try {
      const params = new URLSearchParams({ offset: String(off), limit: String(LIMIT) });
      if (a.trim()) params.set('action', a.trim());
      if (t.trim()) params.set('target', t.trim());
      const d = await api<{ entries: AuditEntry[]; total?: number }>(`/admin/audit-logs?${params}`);
      setRows(d.entries || []);
      setTotal(d.total ?? 0);
      setOffset(off);
    } catch {
      setRows([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  }, []);

  // Filters apply as you type, with a short pause.
  useEffect(() => {
    const t = setTimeout(() => load(0, action, target), 300);
    return () => clearTimeout(t);
  }, [action, target, load]);

  const clear = async () => {
    try {
      await api('/admin/audit-logs', { method: 'DELETE' });
      showToast('Audit log cleared', 'success');
      load(0, action, target);
    } catch (e) {
      showToast((e as Error).message, 'error');
    }
  };

  const cols: Column<AuditEntry>[] = [
    { key: 'action', header: 'Action', width: 'minmax(0,1.2fr)', cell: (e) => <span className="font-medium text-ink">{e.action || '—'}</span> },
    { key: 'time', header: 'When', width: '170px', cell: (e) => <span className="text-[13px] text-ink-3 whitespace-nowrap">{shortDateTime(e.timestamp)}</span> },
    { key: 'admin', header: 'Who', cell: (e) => <span className="text-ink-2 truncate block">{e.admin || '—'}</span> },
    { key: 'target', header: 'Target', cell: (e) => <span className="font-mono text-[12.5px] text-ink-2 truncate block">{e.target || '—'}</span> },
    {
      key: 'details',
      header: 'Details',
      width: 'minmax(0,1.6fr)',
      hideOnMobile: true,
      cell: (e) => (
        <span className="text-[13px] text-ink-3 truncate block" title={e.details}>
          {e.details || '—'}
        </span>
      ),
    },
  ];

  const start = total === 0 ? 0 : offset + 1;
  const end = Math.min(offset + (rows?.length ?? 0), total);

  return (
    <div className="flex flex-col gap-4 py-7">
      <div className="flex flex-wrap items-center gap-3">
        <SearchInput value={action} onChange={setAction} placeholder="Filter by action" className="w-full sm:w-[200px]" />
        <SearchInput value={target} onChange={setTarget} placeholder="Filter by target" className="w-full sm:w-[200px]" />
        <div className="flex items-center gap-2 sm:ml-auto">
          <IconButton label="Refresh" onClick={() => load(offset, action, target)}>
            <RefreshCw className={loading ? 'w-4 h-4 animate-spin' : 'w-4 h-4'} />
          </IconButton>
          {canClear && (
            <Button variant="ghost" className="!text-danger" onClick={() => setClearOpen(true)} disabled={!total}>
              Clear log
            </Button>
          )}
        </div>
      </div>

      <Card padded={false}>
        {rows === null ? (
          <div className="flex flex-col gap-3 p-5">
            <Skeleton className="h-9 w-full" />
            <Skeleton className="h-9 w-full" />
            <Skeleton className="h-9 w-full" />
          </div>
        ) : (
          <DataTable
            columns={cols}
            rows={rows}
            rowKey={(e) => `${e.timestamp}|${e.action}|${e.target}|${e.admin}`}
            empty={
              <EmptyState
                icon={<ScrollText className="w-5 h-5" />}
                title={action || target ? 'No matching entries' : 'Nothing recorded yet'}
                description="Admin actions such as saving settings or deleting a project are recorded here."
              />
            }
          />
        )}
      </Card>

      {total > 0 && (
        <div className="flex flex-wrap items-center justify-between gap-3 text-[13px] text-ink-3">
          <span>
            {start}–{end} of {total.toLocaleString()}
          </span>
          <div className="flex gap-2">
            <Button size="sm" disabled={offset === 0 || loading} onClick={() => load(Math.max(0, offset - LIMIT), action, target)}>
              Previous
            </Button>
            <Button size="sm" disabled={offset + LIMIT >= total || loading} onClick={() => load(offset + LIMIT, action, target)}>
              Next
            </Button>
          </div>
        </div>
      )}

      <ConfirmDialog open={clearOpen} onClose={() => setClearOpen(false)} onConfirm={clear} title="Clear the audit log?" confirmLabel="Clear log">
        <p className="m-0">Every recorded admin action is deleted. It can't be undone.</p>
      </ConfirmDialog>
    </div>
  );
}
