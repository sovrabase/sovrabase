import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { ArrowRight, Workflow } from 'lucide-react';
import {
  Badge,
  Banner,
  ButtonLink,
  Card,
  DataTable,
  EmptyState,
  Page,
  PageHeader,
  SearchInput,
  Segmented,
  Skeleton,
  Switch,
  describeCron,
  useToast,
  type Column,
} from '../../ui';
import { useProject } from '../../shell/ProjectContext';
import { projectPath } from '../../shell/nav';
import type { CronJob } from '../../types';
import {
  automationsApi,
  cronUrl,
  INCOMING_PROVIDERS,
  pluginEventLabel,
  shortUrl,
  timeAgo,
  TRIGGER_LABEL,
  webhookEvents,
  RECORD_EVENTS,
  type AutomationItem,
  type TriggerKind,
  type WebhookRow,
} from './automations/model';
import { TriggerPicker } from './automations/TriggerPicker';
import { CronBuilder } from './automations/CronBuilder';
import { WebhookBuilder } from './automations/WebhookBuilder';
import { QueueDetail } from './automations/QueueDetail';

type Filter = 'all' | TriggerKind;

export default function ProjectAutomations() {
  const { kind, itemId } = useParams<{ kind?: string; itemId?: string }>();
  if (kind === 'new' && (itemId === 'cron' || itemId === 'webhook' || itemId === 'queue')) {
    if (itemId === 'cron') return <CronBuilder jobId={null} />;
    if (itemId === 'webhook') return <WebhookBuilder webhookId={null} />;
    return <QueueDetail queueName={null} />;
  }
  if (kind === 'new') return <TriggerPicker />;
  if (kind === 'cron' && itemId) return <CronBuilder jobId={itemId} />;
  if (kind === 'webhook' && itemId) return <WebhookBuilder webhookId={itemId} />;
  if (kind === 'queue' && itemId) return <QueueDetail queueName={itemId} />;
  return <AutomationList />;
}

function AutomationList() {
  const { projectId } = useProject();
  const navigate = useNavigate();
  const { showToast } = useToast();
  const [items, setItems] = useState<AutomationItem[] | null>(null);
  const [cron, setCron] = useState<CronJob[]>([]);
  const [hooks, setHooks] = useState<WebhookRow[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState<Filter>('all');
  const [q, setQ] = useState('');
  const [lastRuns, setLastRuns] = useState<Record<string, AutomationItem['lastRun']>>({});
  const [busy, setBusy] = useState<string | null>(null);

  const p = useCallback((s: string) => projectPath(projectId, s), [projectId]);

  const load = useCallback(async () => {
    setError(null);
    try {
      const [jobs, whs, queues, integ] = await Promise.all([
        automationsApi.listCron(projectId),
        automationsApi.listWebhooks(projectId).catch(() => [] as WebhookRow[]),
        automationsApi.listQueues(projectId),
        automationsApi.integrations(projectId),
      ]);
      setCron(jobs);
      setHooks(whs);
      const out: AutomationItem[] = [];
      for (const j of jobs) {
        out.push({
          key: `cron:${j.id}`,
          kind: 'schedule',
          name: j.name,
          trigger: describeCron(j.schedule).text,
          action: `${j.method || 'POST'} ${shortUrl(cronUrl(j)) || 'no URL'}`,
          status: j.enabled ? 'on' : 'paused',
          to: p(`automations/cron/${encodeURIComponent(j.id)}`),
          lastRun: j.last_run ? { at: j.last_run } : undefined,
          editable: true,
        });
      }
      // Rows without a URL are provider events stored by incoming plugin webhooks, not automations.
      for (const w of whs.filter((w) => !!w.url)) {
        const ev = webhookEvents(w);
        out.push({
          key: `webhook:${w.id}`,
          kind: 'record',
          name: shortUrl(w.url!).split('/')[0],
          trigger: ev.length
            ? `Record ${ev.map((e) => RECORD_EVENTS.find((r) => r.value === e)?.label.toLowerCase() ?? e).join(', ')}`
            : 'Any record change',
          action: `POST ${shortUrl(w.url!)}`,
          status: w.enabled ? 'on' : 'paused',
          to: p(`automations/webhook/${encodeURIComponent(w.id)}`),
          editable: true,
        });
      }
      for (const qu of queues) {
        out.push({
          key: `queue:${qu.name}`,
          kind: 'queue',
          name: qu.name,
          trigger: `Queue · ${qu.name}`,
          action: `${qu.visible.toLocaleString()} waiting · ${qu.in_flight.toLocaleString()} in flight`,
          status: 'info',
          to: p(`automations/queue/${encodeURIComponent(qu.name)}`),
          lastRun: { text: `${qu.total.toLocaleString()} sent` },
          editable: false,
        });
      }
      for (const e of integ.enabled) {
        const def = integ.catalog.find((c) => c.id === e.id);
        const name = def?.name ?? e.id;
        if (INCOMING_PROVIDERS.has(e.id)) {
          out.push({
            key: `incoming:${e.id}`,
            kind: 'incoming',
            name: `${name} events`,
            trigger: `Incoming webhook · ${name}`,
            action: 'Store event in _webhooks',
            status: 'info',
            to: p(`plugins/${encodeURIComponent(e.id)}`),
            editable: false,
          });
        }
        if (def?.supports_triggers) {
          const evs = e.events?.length ? e.events.map(pluginEventLabel).join(', ') : 'any event';
          const cols = e.collections?.length ? ` in ${e.collections.join(', ')}` : '';
          out.push({
            key: `plugin:${e.id}`,
            kind: 'plugin',
            name,
            trigger: `${evs}${cols}`,
            action: `${name} · notify`,
            status: 'info',
            to: p(`plugins/${encodeURIComponent(e.id)}`),
            editable: false,
          });
        }
      }
      setItems(out);

      // Last run for each scheduled job, from its execution history.
      jobs.forEach((j) => {
        automationsApi
          .cronExecutions(projectId, j.id)
          .then((ex) => {
            if (!ex.length) return;
            const last = [...ex].sort((a, b) => +new Date(b.start_time) - +new Date(a.start_time))[0];
            const failed = ex.filter((x) => x.status !== 'success').length;
            setLastRuns((m) => ({
              ...m,
              [`cron:${j.id}`]: {
                at: last.start_time,
                ok: last.status === 'success',
                text: failed ? `${failed} failed of ${ex.length}` : `${ex.length} ok`,
              },
            }));
          })
          .catch(() => undefined);
      });
    } catch (e) {
      setError((e as Error).message);
      setItems([]);
    }
  }, [projectId, p]);

  useEffect(() => {
    load();
  }, [load]);

  const toggle = async (it: AutomationItem, on: boolean) => {
    setBusy(it.key);
    try {
      if (it.key.startsWith('cron:')) {
        const j = cron.find((c) => `cron:${c.id}` === it.key);
        if (!j) return;
        const body = typeof j.body === 'string' ? j.body : j.body ? JSON.stringify(j.body) : '';
        await automationsApi.updateCron(projectId, j.id, {
          ...(j as unknown as Record<string, unknown>),
          url: cronUrl(j),
          body,
          enabled: on,
        });
      } else {
        const w = hooks.find((h) => `webhook:${h.id}` === it.key);
        if (!w) return;
        await automationsApi.updateWebhook(projectId, w.id, { enabled: on });
      }
      setItems((list) => list?.map((x) => (x.key === it.key ? { ...x, status: on ? 'on' : 'paused' } : x)) ?? null);
      showToast(on ? `${it.name} is on` : `${it.name} is paused`, 'success');
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setBusy(null);
    }
  };

  const counts = useMemo(() => {
    const c: Record<Filter, number> = { all: 0, schedule: 0, record: 0, incoming: 0, queue: 0, plugin: 0 };
    items?.forEach((i) => {
      c.all++;
      c[i.kind]++;
    });
    return c;
  }, [items]);

  const rows = useMemo(() => {
    const needle = q.trim().toLowerCase();
    return (items ?? [])
      .filter((i) => filter === 'all' || i.kind === filter)
      .filter((i) => !needle || `${i.name} ${i.trigger} ${i.action}`.toLowerCase().includes(needle));
  }, [items, filter, q]);

  const columns: Column<AutomationItem>[] = [
    {
      key: 'name',
      header: 'Name',
      width: 'minmax(160px,1.2fr)',
      cell: (r) => (
        <div className="flex flex-col min-w-0">
          <span className="font-medium text-ink truncate">{r.name}</span>
          <span className="text-[12.5px] text-ink-3">{TRIGGER_LABEL[r.kind]}</span>
        </div>
      ),
    },
    {
      key: 'on',
      header: 'On',
      width: '80px',
      cell: (r) =>
        r.editable ? (
          <span className="inline-flex" onClick={(e) => e.stopPropagation()} onKeyDown={(e) => e.stopPropagation()}>
            <Switch
              checked={r.status === 'on'}
              disabled={busy === r.key}
              onChange={(v) => toggle(r, v)}
              aria-label={`${r.name} on`}
            />
          </span>
        ) : r.kind === 'queue' ? (
          <Badge tone="accent">Queue</Badge>
        ) : (
          <Badge>Plugin</Badge>
        ),
    },
    {
      key: 'flow',
      header: 'When → then',
      width: 'minmax(220px,1.8fr)',
      cell: (r) => (
        <div className="flex flex-wrap items-center gap-1.5 min-w-0">
          <span className="inline-flex items-center h-[26px] px-2 rounded-[6px] bg-sunken text-[12.5px] font-medium text-ink-2 max-w-full truncate">
            {r.trigger}
          </span>
          <ArrowRight className="w-3.5 h-3.5 text-ink-3 shrink-0" aria-label="then" />
          <span className="inline-flex items-center h-[26px] px-2 rounded-[6px] bg-sunken text-[12.5px] font-medium text-ink-2 max-w-full truncate font-mono">
            {r.action}
          </span>
        </div>
      ),
    },
    {
      key: 'last',
      header: 'Last run',
      width: '150px',
      cell: (r) => {
        const lr = lastRuns[r.key] ?? r.lastRun;
        if (r.status === 'paused') return <Badge>Paused</Badge>;
        if (!lr) return <span className="text-[12.5px] text-ink-3">—</span>;
        return (
          <span className="flex flex-col text-[12.5px]">
            {lr.text && <span className={lr.ok === false ? 'text-danger font-medium' : 'text-ink-2'}>{lr.text}</span>}
            {lr.at && <span className="text-ink-3">{timeAgo(lr.at)}</span>}
          </span>
        );
      },
    },
  ];

  const filters: Array<{ value: Filter; label: string; count?: number }> = [
    { value: 'all', label: 'All', count: counts.all },
    { value: 'schedule', label: 'Schedule', count: counts.schedule },
    { value: 'record', label: 'Record event', count: counts.record },
    { value: 'incoming', label: 'Incoming webhook', count: counts.incoming },
    { value: 'queue', label: 'Queue', count: counts.queue },
    { value: 'plugin', label: 'Plugin', count: counts.plugin },
  ];

  return (
    <Page>
      <PageHeader
        title="Automations"
        description="When something happens, do something: schedules, record events, queues and plugin triggers in one list."
        actions={
          <ButtonLink variant="primary" to={p('automations/new/pick')}>
            New automation
          </ButtonLink>
        }
      />
      {error && (
        <div className="mb-4">
          <Banner tone="danger" title="Automations could not be loaded">
            {error}
          </Banner>
        </div>
      )}
      <div className="flex flex-wrap items-center gap-3 mb-4">
        <Segmented size="sm" options={filters} value={filter} onChange={setFilter} aria-label="Trigger type" />
        <SearchInput value={q} onChange={setQ} placeholder="Search automations" className="flex-1 min-w-[200px] max-w-[320px] ml-auto" />
      </div>
      <Card padded={false}>
        {items === null ? (
          <div className="p-5 flex flex-col gap-3">
            <Skeleton className="h-10" />
            <Skeleton className="h-10" />
            <Skeleton className="h-10" />
          </div>
        ) : (
          <DataTable
            columns={columns}
            rows={rows}
            rowKey={(r) => r.key}
            onRowClick={(r) => navigate(r.to)}
            empty={
              <EmptyState
                icon={<Workflow className="w-5 h-5" />}
                title={items.length ? 'Nothing matches' : 'No automations yet'}
                description={
                  items.length
                    ? 'Try another trigger type or search.'
                    : 'Run a task on a schedule, call a URL when records change, or process a queue.'
                }
                action={
                  items.length ? undefined : (
                    <ButtonLink variant="primary" to={p('automations/new/pick')}>
                      New automation
                    </ButtonLink>
                  )
                }
              />
            }
          />
        )}
      </Card>
      <p className="mt-3 text-[12.5px] text-ink-3">
        Plugin and incoming webhook triggers are set up on each plugin's page.
      </p>
    </Page>
  );
}
