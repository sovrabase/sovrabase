import { useCallback, useEffect, useMemo, useState } from 'react';
import { Inbox, RefreshCw, RotateCw } from 'lucide-react';
import { api } from '../../../api';
import type { EmailLogEntry } from '../../../types';
import { Badge, Button, Card, ConfirmDialog, DataTable, EmptyState, IconButton, Segmented, Skeleton, useToast, type Column } from '../../../ui';
import { shortDateTime } from './shared';

const LIMIT = 50;
type Filter = 'all' | 'failed';

export function EmailLogSection() {
  const { showToast } = useToast();
  const [rows, setRows] = useState<EmailLogEntry[] | null>(null);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(false);
  const [retrying, setRetrying] = useState<string | null>(null);
  const [clearOpen, setClearOpen] = useState(false);
  const [filter, setFilter] = useState<Filter>('all');

  const load = useCallback(async (off: number) => {
    setLoading(true);
    try {
      const d = await api<{ data: EmailLogEntry[]; total: number }>(`/admin/email-log?limit=${LIMIT}&offset=${off}`);
      setRows((prev) => (off === 0 ? d.data || [] : [...(prev ?? []), ...(d.data || [])]));
      setTotal(d.total ?? 0);
      setOffset(off);
    } catch {
      setRows((prev) => prev ?? []);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load(0);
  }, [load]);

  const retry = async (id: string) => {
    setRetrying(id);
    try {
      await api(`/admin/email-log/${encodeURIComponent(id)}/retry`, { method: 'POST' });
      showToast('Email sent again', 'success');
      load(0);
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setRetrying(null);
    }
  };

  const clear = async () => {
    try {
      await api('/admin/email-log', { method: 'DELETE' });
      showToast('Email log cleared', 'success');
      setRows([]);
      setTotal(0);
      setOffset(0);
    } catch (e) {
      showToast((e as Error).message, 'error');
    }
  };

  const shown = useMemo(() => (filter === 'failed' ? (rows ?? []).filter((r) => !r.success) : rows ?? []), [rows, filter]);
  const failedCount = (rows ?? []).filter((r) => !r.success).length;

  const cols: Column<EmailLogEntry>[] = [
    {
      key: 'subject',
      header: 'Email',
      width: 'minmax(0,2fr)',
      cell: (e) => (
        <div className="flex flex-col min-w-0">
          <span className="text-ink font-medium truncate" title={e.subject}>
            {e.subject || '(no subject)'}
          </span>
          <span className="text-[12.5px] text-ink-3 truncate">to {e.to}</span>
        </div>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      width: '110px',
      cell: (e) =>
        e.success ? (
          <Badge tone="ok" dot>
            Sent
          </Badge>
        ) : (
          <Badge tone="danger" dot>
            Failed
          </Badge>
        ),
    },
    { key: 'time', header: 'When', width: '170px', cell: (e) => <span className="text-[13px] text-ink-3 whitespace-nowrap">{shortDateTime(e.timestamp)}</span> },
    { key: 'provider', header: 'Provider', width: '100px', hideOnMobile: true, cell: (e) => <span className="text-[13px] text-ink-2">{e.provider}</span> },
    {
      key: 'error',
      header: 'Error',
      width: 'minmax(0,1.6fr)',
      cell: (e) =>
        e.error ? (
          <span className="flex items-center gap-2 min-w-0">
            <span className="text-[13px] text-danger truncate" title={e.error}>
              {e.error}
            </span>
            <IconButton label="Send again" size="sm" disabled={retrying === e.id} onClick={() => retry(e.id)} className="shrink-0">
              <RotateCw className={retrying === e.id ? 'w-4 h-4 animate-spin' : 'w-4 h-4'} />
            </IconButton>
          </span>
        ) : (
          <span className="text-ink-3 hidden md:inline">—</span>
        ),
    },
  ];

  return (
    <div className="flex flex-col gap-4 py-7">
      <div className="flex flex-wrap items-center gap-3">
        <Segmented<Filter>
          aria-label="Show"
          value={filter}
          onChange={setFilter}
          options={[
            { value: 'all', label: 'All', count: total },
            { value: 'failed', label: 'Failed', count: failedCount },
          ]}
        />
        <div className="flex items-center gap-2 ml-auto">
          <IconButton label="Refresh" onClick={() => load(0)}>
            <RefreshCw className={loading ? 'w-4 h-4 animate-spin' : 'w-4 h-4'} />
          </IconButton>
          <Button variant="ghost" className="!text-danger" disabled={!rows?.length} onClick={() => setClearOpen(true)}>
            Clear log
          </Button>
        </div>
      </div>

      <Card padded={false}>
        {rows === null ? (
          <div className="flex flex-col gap-3 p-5">
            <Skeleton className="h-9 w-full" />
            <Skeleton className="h-9 w-full" />
          </div>
        ) : (
          <DataTable
            columns={cols}
            rows={shown}
            rowKey={(e) => e.id}
            empty={
              <EmptyState
                icon={<Inbox className="w-5 h-5" />}
                title={filter === 'failed' ? 'No failed emails' : 'No emails sent yet'}
                description="Every email the server sends, or tries to send, shows up here."
              />
            }
          />
        )}
      </Card>

      {rows && rows.length < total && (
        <div className="flex justify-center">
          <Button loading={loading} onClick={() => load(offset + LIMIT)}>
            Load more ({rows.length} of {total})
          </Button>
        </div>
      )}

      <ConfirmDialog open={clearOpen} onClose={() => setClearOpen(false)} onConfirm={clear} title="Clear the email log?" confirmLabel="Clear log">
        <p className="m-0">All entries are deleted, including failed ones you might want to send again. It can't be undone.</p>
      </ConfirmDialog>
    </div>
  );
}
