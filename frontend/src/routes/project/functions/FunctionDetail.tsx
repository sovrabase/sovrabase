import { useMemo, useState } from 'react';
import { ChevronDown, Play, Trash2 } from 'lucide-react';
import { api, adminProjectPath, formatBytes } from '../../../api';
import type { ServerlessFunction, TestResult } from '../../../types';
import {
  Badge,
  Banner,
  Button,
  Card,
  CardHeader,
  ChipSelect,
  CodeBlock,
  ConfirmDialog,
  CopyButton,
  Field,
  IconButton,
  Input,
  PageHeader,
  ScheduleInput,
  Select,
  Switch,
  Textarea,
  cx,
  useDraft,
  useToast,
} from '../../../ui';
import { MetricsPanel } from './MetricsPanel';
import {
  BINDING_SNIPPETS,
  LANGUAGES,
  TRIGGERS,
  bodyFromForm,
  codeHint,
  defaultCode,
  formFromFn,
  invokeUrl,
  languageLabel,
  statusTone,
  triggerSummary,
  type FnForm,
} from './shared';

export function FunctionDetail({
  projectId,
  fn,
  apiKey,
  onChanged,
  onDeleted,
}: {
  projectId: string;
  fn: ServerlessFunction;
  apiKey?: string;
  onChanged: () => Promise<void>;
  onDeleted: () => void;
}) {
  const { showToast } = useToast();
  const saved = useMemo(() => formFromFn(fn), [fn]);
  const { draft, patch, dirty, reset } = useDraft<FnForm>(saved);
  const [deploying, setDeploying] = useState(false);
  const [toggling, setToggling] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [showBindings, setShowBindings] = useState(false);
  const [testBody, setTestBody] = useState('');
  const [testError, setTestError] = useState<string | null>(null);
  const [testing, setTesting] = useState(false);
  const [result, setResult] = useState<TestResult | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);

  const path = adminProjectPath(projectId, `/functions/${encodeURIComponent(fn.id)}`);

  if (!draft) return null;
  const f = draft;

  const changeLanguage = (language: string) =>
    patch({ language, code: f.code === defaultCode(f.language, f.trigger) ? defaultCode(language, f.trigger) : f.code });
  const changeTrigger = (trigger: string) =>
    patch({ trigger, code: f.code === defaultCode(f.language, f.trigger) ? defaultCode(f.language, trigger) : f.code });

  const deploy = async () => {
    if (!f.code.trim()) {
      showToast('The function needs some code', 'error');
      return;
    }
    if (f.trigger === 'schedule' && !f.schedule.trim()) {
      showToast('Pick a schedule', 'error');
      return;
    }
    setDeploying(true);
    try {
      await api(path, { method: 'PUT', body: JSON.stringify(bodyFromForm({ ...f, enabled: fn.enabled })) });
      showToast('Changes deployed', 'success');
      await onChanged();
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setDeploying(false);
    }
  };

  const toggle = async (enabled: boolean) => {
    setToggling(true);
    try {
      await api(path, { method: 'PUT', body: JSON.stringify({ ...fn, enabled }) });
      showToast(enabled ? 'Function turned on' : 'Function turned off', 'success');
      await onChanged();
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setToggling(false);
    }
  };

  const remove = async () => {
    try {
      await api(path, { method: 'DELETE' });
      showToast('Function deleted', 'success');
      onDeleted();
    } catch (e) {
      showToast((e as Error).message, 'error');
      throw e;
    }
  };

  const runTest = async () => {
    let payload: Record<string, unknown> = {};
    if (fn.trigger === 'http' && testBody.trim()) {
      try {
        payload = { method: 'POST', body: JSON.parse(testBody) };
      } catch {
        setTestError('This is not valid JSON.');
        return;
      }
    }
    setTestError(null);
    setTesting(true);
    setResult(null);
    try {
      const res = await api<TestResult>(`${path}/test`, { method: 'POST', body: JSON.stringify(payload) });
      setResult(res);
      setRefreshKey((k) => k + 1);
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setTesting(false);
    }
  };

  const url = invokeUrl(fn.name);
  const snippets = BINDING_SNIPPETS[f.language] || BINDING_SNIPPETS.javascript;

  return (
    <div className="min-w-0">
      <PageHeader
        title={<span className="font-mono text-[20px]">{fn.name}</span>}
        meta={
          <>
            {fn.enabled ? (
              <Badge tone="ok" dot>
                On
              </Badge>
            ) : (
              <Badge dot>Off</Badge>
            )}
            {dirty && <Badge tone="warn">Unsaved changes</Badge>}
          </>
        }
        description={`${triggerSummary(fn)} · ${languageLabel(fn.language)} · timeout ${(fn.timeout_ms || 5000) / 1000} s`}
        actions={
          <>
            <Switch checked={fn.enabled} onChange={toggle} disabled={toggling} aria-label="Function on" />
            <IconButton label="Delete function" onClick={() => setConfirmDelete(true)} className="hover:!text-danger hover:!bg-danger-soft">
              <Trash2 className="w-4 h-4" />
            </IconButton>
            {dirty && (
              <Button variant="ghost" onClick={reset}>
                Discard
              </Button>
            )}
            <Button variant="primary" loading={deploying} disabled={!dirty} onClick={deploy}>
              Deploy changes
            </Button>
          </>
        }
      />

      {fn.trigger === 'http' && (
        <div className="flex flex-wrap items-center gap-2 mb-5 px-3 py-2 rounded-lg border border-line bg-sunken">
          <span className="text-[12px] font-semibold text-ink-3">POST</span>
          <code className="flex-1 min-w-0 font-mono text-[12.5px] text-ink-2 break-all">{url}</code>
          {apiKey && (
            <span className="text-[12px] text-ink-3 font-mono">
              X-Project-Key: {apiKey.slice(0, 12)}…
            </span>
          )}
          {!fn.public && <Badge>Needs a user token</Badge>}
          <CopyButton text={url} label="Copy URL" />
        </div>
      )}

      <div className="grid grid-cols-1 xl:grid-cols-[minmax(0,1fr)_340px] gap-5 mb-5">
        <Card padded={false} className="min-w-0">
          <CardHeader
            title="Code"
            description={codeHint(f.language, f.trigger)}
            actions={
              <Button size="sm" variant="ghost" aria-expanded={showBindings} onClick={() => setShowBindings((s) => !s)}>
                Bindings
                <ChevronDown className={cx('w-3.5 h-3.5 transition-transform', showBindings && 'rotate-180')} aria-hidden />
              </Button>
            }
          />
          {showBindings && (
            <div className="border-t border-line px-5 py-4 flex flex-col gap-3 bg-sunken/50">
              {f.language === 'go' && <p className="m-0 text-[12.5px] text-ink-3">Database and storage bindings are not available in Go yet, only host.Fetch.</p>}
              {snippets.map((s) => (
                <div key={s.label} className="flex flex-col gap-1.5">
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-[12.5px] font-semibold text-ink-2">{s.label}</span>
                    <CopyButton text={s.code} />
                  </div>
                  <CodeBlock className="!p-3 text-[12px]">{s.code}</CodeBlock>
                </div>
              ))}
            </div>
          )}
          <div className="border-t border-line p-3">
            <Textarea
              mono
              aria-label="Function code"
              rows={20}
              spellCheck={false}
              value={f.code}
              onChange={(e) => patch({ code: e.target.value })}
              onKeyDown={(e) => {
                if (e.key === 'Tab' && !e.shiftKey) {
                  e.preventDefault();
                  const el = e.currentTarget;
                  const s = el.selectionStart;
                  const next = f.code.slice(0, s) + '  ' + f.code.slice(el.selectionEnd);
                  patch({ code: next });
                  requestAnimationFrame(() => el.setSelectionRange(s + 2, s + 2));
                }
                if ((e.metaKey || e.ctrlKey) && e.key === 's') {
                  e.preventDefault();
                  if (dirty) deploy();
                }
              }}
              className="!bg-code !text-code-ink !border-transparent leading-[20px] min-h-[320px]"
            />
            <p className="m-0 mt-2 text-[12px] text-ink-3">Tab inserts two spaces. Ctrl+S deploys.</p>
          </div>
        </Card>

        <div className="flex flex-col gap-5 min-w-0">
          <Card className="flex flex-col gap-4">
            <h3 className="m-0 text-[15px] font-semibold text-ink">Settings</h3>
            <Field label="Runs when">
              <ChipSelect single aria-label="Trigger" options={TRIGGERS.map((t) => ({ value: t.value, label: t.label, description: t.description }))} value={[f.trigger]} onChange={(v) => v[0] && changeTrigger(v[0])} />
            </Field>
            {f.trigger.startsWith('record:') && (
              <Field label="Collection" hint="Leave empty to run for every collection." optional>
                <Input mono value={f.collection} onChange={(e) => patch({ collection: e.target.value })} placeholder="orders" />
              </Field>
            )}
            {f.trigger === 'schedule' && (
              <Field label="Schedule">
                <ScheduleInput value={f.schedule} onChange={(schedule) => patch({ schedule })} />
              </Field>
            )}
            <Field label="Language">
              <Select value={f.language} onChange={(e) => changeLanguage(e.target.value)} options={LANGUAGES} />
            </Field>
            <Field label="Timeout" hint="Between 1 and 30 seconds.">
              <Input
                type="number"
                min={1000}
                max={30000}
                step={500}
                value={f.timeout_ms}
                onChange={(e) => patch({ timeout_ms: Number(e.target.value) })}
                suffix={<span className="text-[13px] text-ink-3 pr-1.5">ms</span>}
              />
            </Field>
            {f.trigger === 'http' && (
              <Switch checked={f.public} onChange={(v) => patch({ public: v })} label="Public" description="Anyone with the URL can call it, no user token needed." />
            )}
          </Card>

          <Card className="flex flex-col gap-4">
            <div className="flex items-center justify-between gap-2">
              <h3 className="m-0 text-[15px] font-semibold text-ink">Try it</h3>
              <Button size="sm" variant="primary" icon={<Play className="w-3.5 h-3.5" />} loading={testing} onClick={runTest}>
                {result ? 'Run again' : 'Run'}
              </Button>
            </div>
            {dirty && <p className="m-0 text-[12.5px] text-warn">Runs the deployed code. Deploy first to test your changes.</p>}
            {fn.trigger === 'http' ? (
              <Field label="Request body" optional hint="JSON, sent as a POST." error={testError}>
                <Textarea mono rows={4} spellCheck={false} value={testBody} onChange={(e) => setTestBody(e.target.value)} placeholder='{ "hello": "world" }' />
              </Field>
            ) : (
              <p className="m-0 text-[12.5px] text-ink-3">Runs the function once with an empty event.</p>
            )}
            {result && (
              <div className="flex flex-col gap-3 border-t border-line pt-4">
                <div className="flex flex-wrap items-center gap-2 text-[12.5px] text-ink-3">
                  <Badge tone={statusTone(result.status)} dot>
                    {result.status}
                  </Badge>
                  {result.response && <Badge>HTTP {result.response.status}</Badge>}
                  <span className="font-mono">{result.duration_ms} ms</span>
                  <span className="font-mono">{formatBytes(result.metrics?.mem_peak_bytes || 0)}</span>
                  <span className="font-mono">CPU {(result.metrics?.cpu_user_ms || 0) + (result.metrics?.cpu_sys_ms || 0)} ms</span>
                </div>
                {result.error && (
                  <Banner tone="danger">
                    <span className="font-mono text-[12.5px] break-words">{result.error}</span>
                  </Banner>
                )}
                {result.response && (
                  <div>
                    <p className="m-0 mb-1.5 text-[12.5px] font-semibold text-ink-2">Response</p>
                    <CodeBlock className="max-h-56 !p-3 text-[12px]">
                      {typeof result.response.body === 'string' ? result.response.body : JSON.stringify(result.response.body, null, 2)}
                    </CodeBlock>
                  </div>
                )}
                {result.output && (
                  <div>
                    <p className="m-0 mb-1.5 text-[12.5px] font-semibold text-ink-2">Console</p>
                    <CodeBlock className="max-h-56 !p-3 text-[12px]">{result.output}</CodeBlock>
                  </div>
                )}
              </div>
            )}
          </Card>
        </div>
      </div>

      <MetricsPanel projectId={projectId} functionId={fn.id} refreshKey={refreshKey} />

      <ConfirmDialog
        open={confirmDelete}
        onClose={() => setConfirmDelete(false)}
        onConfirm={remove}
        title="Delete this function?"
        confirmLabel="Delete function"
        confirmText={fn.name}
      >
        <p className="m-0">The function is removed for good. Anything that calls it will get an error.</p>
      </ConfirmDialog>
    </div>
  );
}
