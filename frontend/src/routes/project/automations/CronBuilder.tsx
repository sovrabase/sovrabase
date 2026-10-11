import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { ArrowLeft, Play, RefreshCw, Trash2 } from 'lucide-react';
import {
  Badge,
  Banner,
  Button,
  ButtonLink,
  Card,
  CardHeader,
  ConfirmDialog,
  EmptyState,
  Field,
  Input,
  KeyValueEditor,
  Page,
  PageHeader,
  ScheduleInput,
  Segmented,
  Select,
  Spinner,
  Switch,
  Textarea,
  describeCron,
  kvFromRecord,
  kvToRecord,
  useToast,
  type KV,
} from '../../../ui';
import { formatDate } from '../../../api';
import { useProject } from '../../../shell/ProjectContext';
import { projectPath } from '../../../shell/nav';
import type { CronJob, CronJobExecution, ServerlessFunction } from '../../../types';
import { automationsApi, cronUrl, publicOrigin } from './model';
import { BuilderLayout, Join, Step } from './parts';

type Job = CronJob & { headers?: Record<string, string> };

interface Form {
  name: string;
  schedule: string;
  method: string;
  url: string;
  functionName: string;
  headers: KV[];
  body: string;
  enabled: boolean;
}

const emptyForm = (): Form => ({
  name: '',
  schedule: '0 3 * * *',
  method: 'POST',
  url: '',
  functionName: '',
  headers: [],
  body: '',
  enabled: true,
});

const METHODS = ['POST', 'GET', 'PUT', 'DELETE'].map((m) => ({ value: m, label: m }));

function bodyText(b: unknown) {
  if (b === undefined || b === null || b === '') return '';
  return typeof b === 'string' ? b : JSON.stringify(b, null, 2);
}

export function CronBuilder({ jobId }: { jobId: string | null }) {
  const { project, projectId } = useProject();
  const navigate = useNavigate();
  const { showToast } = useToast();
  const p = (s: string) => projectPath(projectId, s);

  const [job, setJob] = useState<Job | null>(null);
  const [form, setForm] = useState<Form>(emptyForm);
  const [functions, setFunctions] = useState<ServerlessFunction[]>([]);
  const [loading, setLoading] = useState(!!jobId);
  const [notFound, setNotFound] = useState(false);
  const [saving, setSaving] = useState(false);
  const [tried, setTried] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [running, setRunning] = useState(false);
  const [runResult, setRunResult] = useState<{ ok: boolean; text: string } | null>(null);

  const [executions, setExecutions] = useState<CronJobExecution[] | null>(null);
  const [runFilter, setRunFilter] = useState<'all' | 'failed'>('all');
  const [openRun, setOpenRun] = useState<number | null>(null);

  const set = (patch: Partial<Form>) => setForm((f) => ({ ...f, ...patch }));

  const loadRuns = useCallback(() => {
    if (!jobId) return;
    setExecutions(null);
    automationsApi
      .cronExecutions(projectId, jobId)
      .then((ex) => setExecutions([...ex].sort((a, b) => +new Date(b.start_time) - +new Date(a.start_time))))
      .catch(() => setExecutions([]));
  }, [projectId, jobId]);

  useEffect(() => {
    automationsApi.listFunctions(projectId).then(setFunctions);
    if (!jobId) return;
    setLoading(true);
    automationsApi
      .listCron(projectId)
      .then((jobs) => {
        const j = jobs.find((x) => x.id === jobId) as Job | undefined;
        if (!j) {
          setNotFound(true);
          return;
        }
        setJob(j);
        setForm({
          name: j.name,
          schedule: j.schedule,
          method: j.method || 'POST',
          url: cronUrl(j),
          functionName: '',
          headers: kvFromRecord(j.headers),
          body: bodyText(j.body),
          enabled: j.enabled,
        });
      })
      .catch((e) => showToast((e as Error).message, 'error'))
      .finally(() => setLoading(false));
    loadRuns();
  }, [projectId, jobId, loadRuns, showToast]);

  const applyFunction = (name: string) => {
    if (!name) {
      set({ functionName: '' });
      return;
    }
    const key = project.api_key ? `?project_key=${encodeURIComponent(project.api_key)}` : '';
    set({ functionName: name, url: `${publicOrigin()}/api/v1/functions/${name}${key}` });
  };

  const errors = {
    name: !form.name.trim() ? 'Give it a name.' : null,
    schedule: !describeCron(form.schedule).ok ? 'Enter a valid schedule.' : null,
    url: !form.url.trim() ? 'Enter the URL to call.' : !/^https?:\/\//.test(form.url.trim()) ? 'Start with http:// or https://' : null,
  };
  const invalid = Object.values(errors).some(Boolean);

  const payload = (enabled = form.enabled) => {
    const out: Record<string, unknown> = {
      name: form.name.trim(),
      schedule: form.schedule.trim(),
      method: form.method,
      url: form.url.trim(),
      enabled,
      body: form.body.trim() ? form.body : '',
    };
    const h = kvToRecord(form.headers);
    if (Object.keys(h).length) out.headers = h;
    return out;
  };

  const save = async () => {
    setTried(true);
    if (invalid) return;
    setSaving(true);
    try {
      if (job) {
        const updated = await automationsApi.updateCron(projectId, job.id, payload());
        setJob({ ...job, ...(updated as Job) });
        showToast('Scheduled job saved', 'success');
      } else {
        const created = await automationsApi.createCron(projectId, payload());
        showToast('Scheduled job created', 'success');
        navigate(p(`automations/cron/${encodeURIComponent(created.id)}`), { replace: true });
      }
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setSaving(false);
    }
  };

  const togglePause = async (on: boolean) => {
    set({ enabled: on });
    if (!job) return;
    try {
      await automationsApi.updateCron(projectId, job.id, { ...payload(on) });
      setJob({ ...job, enabled: on });
      showToast(on ? 'Job is on' : 'Job is paused', 'success');
    } catch (e) {
      set({ enabled: !on });
      showToast((e as Error).message, 'error');
    }
  };

  // Same behaviour as the old "Trigger now": the browser calls the URL directly.
  const runNow = async () => {
    const endpoint = form.url.trim();
    if (!endpoint) {
      showToast('No URL configured', 'error');
      return;
    }
    setRunning(true);
    setRunResult(null);
    const controller = new AbortController();
    const t = setTimeout(() => controller.abort(), 8000);
    try {
      const init: RequestInit = {
        method: form.method,
        headers: { 'Content-Type': 'application/json', ...kvToRecord(form.headers) },
        signal: controller.signal,
      };
      if (form.body.trim() && form.method !== 'GET') init.body = form.body;
      const started = performance.now();
      const res = await fetch(endpoint, init);
      const ms = Math.round(performance.now() - started);
      if (!res.ok) throw new Error(`HTTP ${res.status} ${res.statusText}`);
      setRunResult({ ok: true, text: `HTTP ${res.status} in ${ms} ms` });
      showToast(`Ran "${form.name || 'job'}"`, 'success');
    } catch (err) {
      const msg = (err as Error).name === 'AbortError' ? 'Timed out after 8 s' : (err as Error).message;
      setRunResult({ ok: false, text: msg });
      showToast(`Run failed: ${msg}`, 'error');
    } finally {
      clearTimeout(t);
      setRunning(false);
    }
  };

  const remove = async () => {
    if (!job) return;
    try {
      await automationsApi.deleteCron(projectId, job.id);
      showToast('Scheduled job deleted', 'success');
      navigate(p('automations'));
    } catch (e) {
      showToast((e as Error).message, 'error');
    }
  };

  const shownRuns = useMemo(
    () => (executions ?? []).filter((x) => runFilter === 'all' || x.status !== 'success'),
    [executions, runFilter],
  );
  const failedCount = (executions ?? []).filter((x) => x.status !== 'success').length;

  if (loading) return <Spinner label="Loading job" />;
  if (notFound) {
    return (
      <Page>
        <EmptyState
          title="This scheduled job no longer exists"
          action={<ButtonLink to={p('automations')}>All automations</ButtonLink>}
        />
      </Page>
    );
  }

  const fnOptions = [{ value: '', label: 'Custom URL' }, ...functions.map((f) => ({ value: f.name, label: f.name }))];

  return (
    <Page>
      <div className="mb-2">
        <ButtonLink variant="ghost" size="sm" to={p('automations')} icon={<ArrowLeft className="w-4 h-4" />}>
          Automations
        </ButtonLink>
      </div>
      <PageHeader
        title={job ? job.name : 'New scheduled job'}
        meta={
          job ? (
            form.enabled ? (
              <Badge tone="ok" dot>
                On
              </Badge>
            ) : (
              <Badge>Paused</Badge>
            )
          ) : undefined
        }
        description="Calls a URL or HTTP function on a schedule."
        actions={
          <>
            {job && (
              <Switch checked={form.enabled} onChange={togglePause} aria-label={form.enabled ? 'Pause job' : 'Turn job on'} />
            )}
            {job && (
              <Button icon={<Play className="w-4 h-4" />} loading={running} onClick={runNow}>
                Run now
              </Button>
            )}
            <Button variant="primary" loading={saving} onClick={save}>
              {job ? 'Save' : 'Create job'}
            </Button>
          </>
        }
      />
      {runResult && (
        <div className="mb-4">
          <Banner tone={runResult.ok ? 'ok' : 'danger'} title={runResult.ok ? 'Run succeeded' : 'Run failed'}>
            {runResult.text}
          </Banner>
        </div>
      )}
      <BuilderLayout
        main={
          <>
            <Step n={1} eyebrow="Trigger" title="On a schedule">
              <Field label="Name" error={tried ? errors.name : null}>
                <Input mono value={form.name} onChange={(e) => set({ name: e.target.value })} placeholder="nightly-cleanup" />
              </Field>
              <Field label="Schedule" error={tried ? errors.schedule : null} hint="Server time. Five fields: minute hour day month weekday.">
                <ScheduleInput value={form.schedule} onChange={(v) => set({ schedule: v })} />
              </Field>
              {!job && <Switch checked={form.enabled} onChange={(v) => set({ enabled: v })} label="On" description="Paused jobs keep their settings and do not run." />}
            </Step>
            <Join />
            <Step n={2} eyebrow="Action" title="Send an HTTP request">
              {functions.length > 0 && (
                <Field label="Call a function" optional hint="Picks the function's public URL with this project's key.">
                  <Select options={fnOptions} value={form.functionName} onChange={(e) => applyFunction(e.target.value)} />
                </Field>
              )}
              <div className="grid gap-4 sm:grid-cols-[120px_minmax(0,1fr)]">
                <Field label="Method">
                  <Select options={METHODS} value={form.method} onChange={(e) => set({ method: e.target.value })} />
                </Field>
                <Field label="URL" error={tried ? errors.url : null}>
                  <Input
                    mono
                    value={form.url}
                    onChange={(e) => set({ url: e.target.value, functionName: '' })}
                    placeholder="https://example.com/hooks/nightly"
                  />
                </Field>
              </div>
              <Field label="Headers" optional>
                <KeyValueEditor value={form.headers} onChange={(v) => set({ headers: v })} keyPlaceholder="Header" addLabel="Add header" />
              </Field>
              {form.method !== 'GET' && (
                <Field label="Body" optional hint="Sent as is, usually JSON.">
                  <Textarea mono rows={5} value={form.body} onChange={(e) => set({ body: e.target.value })} placeholder='{"task": "cleanup"}' />
                </Field>
              )}
            </Step>
            {job && (
              <div className="mt-6 flex flex-wrap items-center justify-between gap-3 px-1">
                <span className="text-[12.5px] text-ink-3">
                  {job.next_run ? `Next run ${formatDate(job.next_run)}` : describeCron(form.schedule).text}
                </span>
                <Button variant="ghost" className="text-danger" icon={<Trash2 className="w-4 h-4" />} onClick={() => setConfirmDelete(true)}>
                  Delete job
                </Button>
              </div>
            )}
          </>
        }
        side={
          job ? (
            <Card padded={false}>
              <CardHeader
                title="Runs"
                description={executions ? `${executions.length} recorded${failedCount ? `, ${failedCount} failed` : ''}` : undefined}
                actions={
                  <button
                    type="button"
                    aria-label="Refresh runs"
                    onClick={loadRuns}
                    className="w-8 h-8 inline-flex items-center justify-center rounded-md text-ink-3 hover:text-ink hover:bg-sunken"
                  >
                    <RefreshCw className="w-4 h-4" />
                  </button>
                }
              />
              <div className="px-5 pb-3">
                <Segmented
                  size="sm"
                  aria-label="Runs to show"
                  value={runFilter}
                  onChange={setRunFilter}
                  options={[
                    { value: 'all', label: 'All' },
                    { value: 'failed', label: 'Failed', count: failedCount },
                  ]}
                />
              </div>
              {executions === null ? (
                <Spinner label="Loading runs" />
              ) : shownRuns.length === 0 ? (
                <p className="px-5 pb-5 m-0 text-[13px] text-ink-3">{executions.length ? 'No failed runs.' : 'No runs recorded yet.'}</p>
              ) : (
                <ul className="m-0 p-0 list-none max-h-[520px] overflow-y-auto">
                  {shownRuns.map((ex, i) => {
                    const ok = ex.status === 'success';
                    const dur =
                      ex.start_time && ex.end_time ? new Date(ex.end_time).getTime() - new Date(ex.start_time).getTime() : 0;
                    const open = openRun === i;
                    return (
                      <li key={`${ex.start_time}-${i}`} className="border-t border-line">
                        <button
                          type="button"
                          aria-expanded={open}
                          onClick={() => setOpenRun(open ? null : i)}
                          className={`w-full grid grid-cols-[10px_minmax(0,1fr)_auto_auto] gap-3 items-center px-5 py-2.5 text-left text-[13px] hover:bg-hover ${
                            !ok ? 'bg-danger-soft/40' : ''
                          }`}
                        >
                          <span className={`w-2 h-2 rounded-full ${ok ? 'bg-ok' : 'bg-danger'}`} aria-hidden />
                          <span className="truncate text-ink-2">{formatDate(ex.start_time)}</span>
                          <span className={`font-mono text-[12px] ${ok ? 'text-ink-3' : 'text-danger'}`}>
                            {ex.status_code ?? ex.status}
                          </span>
                          <span className="font-mono text-[12px] text-ink-3 w-14 text-right">{dur} ms</span>
                        </button>
                        {open && (
                          <div className="px-5 pb-3 flex flex-col gap-1 text-[12.5px]">
                            <span className="text-ink-2">
                              Status <b className={ok ? 'text-ok' : 'text-danger'}>{ex.status}</b>
                              {ex.status_code ? <> · HTTP {ex.status_code}</> : null}
                            </span>
                            {ex.error && (
                              <pre className="m-0 p-2.5 rounded-md bg-sunken font-mono text-[12px] text-ink-2 whitespace-pre-wrap break-words">
                                {ex.error}
                              </pre>
                            )}
                          </div>
                        )}
                      </li>
                    );
                  })}
                </ul>
              )}
            </Card>
          ) : (
            <Card>
              <p className="m-0 text-[13px] text-ink-3">Runs show here once the job is created and has run at least once.</p>
            </Card>
          )
        }
      />
      <ConfirmDialog
        open={confirmDelete}
        onClose={() => setConfirmDelete(false)}
        onConfirm={remove}
        title={`Delete ${job?.name ?? 'this job'}?`}
        confirmText={job?.name}
      >
        The job stops running and its run history is removed.
      </ConfirmDialog>
    </Page>
  );
}
