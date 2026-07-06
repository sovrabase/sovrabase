import { useState, useEffect, useCallback } from 'react';
import {
  Zap,
  Plus,
  Loader2,
  Trash2,
  Play,
  Pencil,
  Copy,
  ToggleLeft,
  ToggleRight,
} from 'lucide-react';
import { api, adminProjectPath, formatBytes } from '../api';
import type { ServerlessFunction, TestResult } from '../types';
import Modal from '../components/Modal';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { useToast } from '../components/Toast';
import FunctionMetricsChart from './FunctionMetricsChart';

interface Props {
  projectId: string;
  apiKey?: string;
}

const TRIGGERS = [
  'http',
  'record:create',
  'record:update',
  'record:delete',
  'auth:signup',
  'auth:signin',
  'schedule',
] as const;

const defaultCode = (language: string, trigger: string) => {
  if (language === 'go') {
    if (trigger === 'http') {
      return `package function

func Handler(req map[string]interface{}) map[string]interface{} {
    return map[string]interface{}{
        "status": 200,
        "body":   map[string]interface{}{"ok": true},
    }
}`;
    }
    return `package function

func init() {
    // event.Type, event.Collection, event.Data
    _ = event
}`;
  }
  if (trigger === 'http') {
    return `function handler(req) {
  return {
    status: 200,
    body: { ok: true },
  };
}`;
  }
  return `// event.type, event.collection, event.data
console.log(event);`;
};

const BINDING_SNIPPETS: Record<string, { label: string; code: string }[]> = {
  javascript: [
    {
      label: 'Database',
      code: `const doc = sovrabase.db.get('users', id);
const created = sovrabase.db.create('users', { name: 'Ada' });
sovrabase.db.update('users', id, { name: 'Ada L.' });
sovrabase.db.delete('users', id);`,
    },
    {
      label: 'Storage (10 MB limit)',
      code: `sovrabase.storage.upload('avatars', 'a.png', data, { contentType: 'image/png' });
const file = sovrabase.storage.download('avatars', 'a.png');
const files = sovrabase.storage.list('avatars', 'prefix/');
sovrabase.storage.delete('avatars', 'a.png');`,
    },
    {
      label: 'HTTP fetch',
      code: `const res = fetch('https://api.example.com', {
  method: 'POST',
  headers: { Authorization: 'Bearer ...' },
  body: { hello: 'world' },
});
console.log(res.status, res.body);`,
    },
    {
      label: 'Logging',
      code: `console.log('message', value);`,
    },
  ],
  go: [
    {
      label: 'HTTP fetch',
      code: `import "function/host"

res, err := host.Fetch("GET", "https://api.example.com", nil)`,
    },
  ],
};

const emptyForm = () => ({
  name: '',
  language: 'javascript',
  trigger: 'http',
  collection: '',
  schedule: '',
  code: defaultCode('javascript', 'http'),
  public: false,
  timeout_ms: 5000,
  enabled: true,
});

export default function FunctionsTab({ projectId, apiKey }: Props) {
  const [fns, setFns] = useState<ServerlessFunction[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [showModal, setShowModal] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testModalOpen, setTestModalOpen] = useState(false);
  const [testBody, setTestBody] = useState('');
  const [testResult, setTestResult] = useState<TestResult | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);
  const [deleteTargetId, setDeleteTargetId] = useState<string | null>(null);
  const [form, setForm] = useState(emptyForm());
  const { showToast } = useToast();

  const load = useCallback(() => {
    setLoading(true);
    api<{ data: ServerlessFunction[] }>(adminProjectPath(projectId, '/functions'))
      .then((res) => {
        const list = res.data || [];
        setFns(list);
        if (list.length && !selectedId) setSelectedId(list[0].id);
      })
      .finally(() => setLoading(false));
  }, [projectId]);

  useEffect(() => {
    load();
  }, [projectId]);

  const openCreate = () => {
    setEditingId(null);
    setForm(emptyForm());
    setShowModal(true);
  };

  const openEdit = (fn: ServerlessFunction) => {
    setEditingId(fn.id);
    setForm({
      name: fn.name,
      language: fn.language || 'javascript',
      trigger: fn.trigger,
      collection: fn.collection || '',
      schedule: fn.schedule || '',
      code: fn.code,
      public: fn.public,
      timeout_ms: fn.timeout_ms || 5000,
      enabled: fn.enabled,
    });
    setShowModal(true);
  };

  const save = async () => {
    if (!form.name.trim() || !form.code.trim()) {
      showToast('Name and code are required', 'error');
      return;
    }
    setSaving(true);
    try {
      const body = {
        ...form,
        name: form.name.trim(),
        collection: form.collection.trim() || undefined,
        schedule: form.schedule.trim() || undefined,
      };
      if (editingId) {
        await api(`/admin/projects/${encodeURIComponent(projectId)}/functions/${encodeURIComponent(editingId)}`, {
          method: 'PUT',
          body: JSON.stringify(body),
        });
        showToast('Function updated', 'success');
      } else {
        await api(`/admin/projects/${encodeURIComponent(projectId)}/functions`, {
          method: 'POST',
          body: JSON.stringify(body),
        });
        showToast('Function deployed', 'success');
      }
      setShowModal(false);
      load();
    } catch (err) {
      showToast((err as Error).message, 'error');
    } finally {
      setSaving(false);
    }
  };

  const confirmDelete = async () => {
    if (!deleteTargetId) return;
    try {
      await api(adminProjectPath(projectId, `/functions/${encodeURIComponent(deleteTargetId)}`), {
        method: 'DELETE',
      });
      if (selectedId === deleteTargetId) setSelectedId(null);
      setDeleteTargetId(null);
      showToast('Function deleted', 'success');
      load();
    } catch (err) {
      showToast((err as Error).message, 'error');
    }
  };

  const runTest = async (fn: ServerlessFunction, bodyText: string) => {
    let payload: Record<string, unknown> = {};
    if (fn.trigger === 'http' && bodyText.trim()) {
      try {
        payload = { method: 'POST', body: JSON.parse(bodyText) };
      } catch {
        showToast('Invalid JSON test body', 'error');
        return;
      }
    }
    setTesting(true);
    setTestResult(null);
    try {
      const res = await api<TestResult>(
        `/admin/projects/${encodeURIComponent(projectId)}/functions/${encodeURIComponent(fn.id)}/test`,
        {
          method: 'POST',
          body: JSON.stringify(payload),
        },
      );
      setTestResult(res);
      setRefreshKey((k) => k + 1);
    } catch (err) {
      showToast((err as Error).message, 'error');
    } finally {
      setTesting(false);
    }
  };

  const openTest = (fn: ServerlessFunction) => {
    setTestBody('');
    setTestResult(null);
    setTestModalOpen(true);
    if (fn.trigger !== 'http') {
      runTest(fn, '');
    }
  };

  const toggleEnabled = async (fn: ServerlessFunction) => {
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/functions/${encodeURIComponent(fn.id)}`, {
        method: 'PUT',
        body: JSON.stringify({ ...fn, enabled: !fn.enabled }),
      });
      load();
    } catch (err) {
      showToast((err as Error).message, 'error');
    }
  };

  const copyInvokeUrl = (name: string) => {
    const origin = window.location.origin.replace(/:\d+$/, ':6070');
    const url = `${origin}/api/v1/functions/${name}`;
    navigator.clipboard.writeText(url);
    showToast('Invoke URL copied', 'success');
  };

  const inputCls =
    'w-full px-3 py-2 rounded-lg bg-bg-input border border-border text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent transition-colors text-sm';

  const selected = fns.find((f) => f.id === selectedId);

  if (loading) {
    return (
      <div className="flex items-center gap-2 py-10 text-text-muted">
        <Loader2 className="w-4 h-4 animate-spin" /> Loading functions...
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-text-primary">Functions ({fns.length})</h2>
        <button
          onClick={openCreate}
          className="flex items-center gap-2 px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover transition-colors"
        >
          <Plus className="w-4 h-4" /> Deploy
        </button>
      </div>

      {fns.length === 0 ? (
        <div className="flex flex-col items-center py-16 text-text-muted gap-3">
          <Zap className="w-10 h-10" />
          <p>No serverless functions yet</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
          <div className="lg:col-span-1 border border-border rounded-lg overflow-hidden">
            <table className="w-full text-sm">
              <thead className="bg-bg-input text-text-muted text-xs uppercase">
                <tr>
                  <th className="text-left px-3 py-2">Name</th>
                  <th className="text-left px-3 py-2">Trigger</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {fns.map((fn) => (
                  <tr
                    key={fn.id}
                    onClick={() => setSelectedId(fn.id)}
                    className={`cursor-pointer hover:bg-bg-input/50 ${
                      selectedId === fn.id ? 'bg-accent/10' : ''
                    }`}
                  >
                    <td className="px-3 py-2">
                      <div className="font-medium text-text-primary">{fn.name}</div>
                      <div className="text-xs text-text-muted">{fn.language || 'javascript'}</div>
                    </td>
                    <td className="px-3 py-2 text-xs font-mono text-accent">{fn.trigger}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="lg:col-span-2 space-y-4">
            {selected && (
              <>
                <div className="flex flex-wrap items-center gap-2 justify-between border border-border rounded-lg p-4 bg-bg-card">
                  <div>
                    <h3 className="font-semibold text-text-primary">{selected.name}</h3>
                    <p className="text-xs text-text-muted mt-1">
                      {selected.trigger}
                      {selected.collection ? ` · ${selected.collection}` : ''}
                      {selected.schedule ? ` · ${selected.schedule}` : ''}
                      {' · '}
                      {selected.enabled ? 'enabled' : 'disabled'}
                    </p>
                  </div>
                  <div className="flex items-center gap-1 flex-wrap">
                    {selected.trigger === 'http' && (
                      <button
                        onClick={() => copyInvokeUrl(selected.name)}
                        className="p-2 rounded-md text-text-muted hover:text-accent hover:bg-accent/10"
                        title="Copy invoke URL"
                      >
                        <Copy className="w-4 h-4" />
                      </button>
                    )}
                    <button
                      onClick={() => openTest(selected)}
                      disabled={testing}
                      className="p-2 rounded-md text-text-muted hover:text-accent hover:bg-accent/10"
                      title="Test"
                    >
                      {testing ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4" />}
                    </button>
                    <button
                      onClick={() => openEdit(selected)}
                      className="p-2 rounded-md text-text-muted hover:text-accent hover:bg-accent/10"
                      title="Edit"
                    >
                      <Pencil className="w-4 h-4" />
                    </button>
                    <button
                      onClick={() => toggleEnabled(selected)}
                      className="p-2 rounded-md text-text-muted hover:text-accent hover:bg-accent/10"
                      title="Toggle enabled"
                    >
                      {selected.enabled ? (
                        <ToggleRight className="w-4 h-4 text-success" />
                      ) : (
                        <ToggleLeft className="w-4 h-4" />
                      )}
                    </button>
                    <button
                      onClick={() => setDeleteTargetId(selected.id)}
                      className="p-2 rounded-md text-text-muted hover:text-danger hover:bg-danger/10"
                      title="Delete"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>
                </div>

                {selected.trigger === 'http' && apiKey && (
                  <div className="text-xs bg-bg-input border border-border rounded-lg p-3 font-mono text-text-secondary">
                    <span className="text-text-muted">POST </span>
                    /api/v1/functions/{selected.name}
                    <br />
                    <span className="text-text-muted">X-Project-Key: </span>
                    {apiKey.slice(0, 12)}...
                  </div>
                )}

                <FunctionMetricsChart projectId={projectId} functionId={selected.id} refreshKey={refreshKey} />
              </>
            )}
          </div>
        </div>
      )}

      <Modal
        isOpen={showModal}
        onClose={() => setShowModal(false)}
        title={editingId ? 'Edit Function' : 'Deploy Function'}
        size="lg"
      >
        <div className="space-y-4 max-h-[70vh] overflow-y-auto pr-1">
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-text-secondary text-sm font-medium">Name</label>
              <input
                type="text"
                value={form.name}
                onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                className={inputCls}
                placeholder="hello-world"
                disabled={!!editingId}
              />
            </div>
            <div>
              <label className="text-text-secondary text-sm font-medium">Language</label>
              <select
                value={form.language}
                onChange={(e) => {
                  const lang = e.target.value;
                  setForm((f) => ({
                    ...f,
                    language: lang,
                    code: defaultCode(lang, f.trigger),
                  }));
                }}
                className={inputCls}
              >
                <option value="javascript">JavaScript (goja)</option>
                <option value="go">Go (yaegi)</option>
              </select>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-text-secondary text-sm font-medium">Trigger</label>
              <select
                value={form.trigger}
                onChange={(e) => {
                  const trigger = e.target.value;
                  setForm((f) => ({
                    ...f,
                    trigger,
                    code: defaultCode(f.language, trigger),
                  }));
                }}
                className={inputCls}
              >
                {TRIGGERS.map((t) => (
                  <option key={t} value={t}>
                    {t}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="text-text-secondary text-sm font-medium">Timeout (ms)</label>
              <input
                type="number"
                min={1000}
                max={30000}
                value={form.timeout_ms}
                onChange={(e) => setForm((f) => ({ ...f, timeout_ms: Number(e.target.value) }))}
                className={inputCls}
              />
            </div>
          </div>

          {form.trigger.startsWith('record:') && (
            <div>
              <label className="text-text-secondary text-sm font-medium">Collection</label>
              <input
                type="text"
                value={form.collection}
                onChange={(e) => setForm((f) => ({ ...f, collection: e.target.value }))}
                className={inputCls}
                placeholder="users"
              />
            </div>
          )}

          {form.trigger === 'schedule' && (
            <div>
              <label className="text-text-secondary text-sm font-medium">Cron schedule</label>
              <input
                type="text"
                value={form.schedule}
                onChange={(e) => setForm((f) => ({ ...f, schedule: e.target.value }))}
                className={inputCls}
                placeholder="0 * * * *"
              />
              <p className="text-text-muted text-xs mt-1">
                Examples: <code>*/5 * * * *</code> · <code>0 3 * * *</code>
              </p>
            </div>
          )}

          {form.trigger === 'http' && (
            <label className="flex items-center gap-2 text-sm text-text-secondary">
              <input
                type="checkbox"
                checked={form.public}
                onChange={(e) => setForm((f) => ({ ...f, public: e.target.checked }))}
              />
              Public (no JWT required)
            </label>
          )}

          <label className="flex items-center gap-2 text-sm text-text-secondary">
            <input
              type="checkbox"
              checked={form.enabled}
              onChange={(e) => setForm((f) => ({ ...f, enabled: e.target.checked }))}
            />
            Enabled
          </label>

          <details className="bg-bg-input border border-border rounded-lg">
            <summary className="px-3 py-2 text-sm text-text-secondary cursor-pointer select-none hover:text-text-primary">
              Available bindings
            </summary>
            <div className="px-3 pb-3 space-y-3">
              {form.language === 'go' && (
                <p className="text-text-muted text-xs">
                  db/storage bindings are not available in Go yet — only host.Fetch.
                </p>
              )}
              {(BINDING_SNIPPETS[form.language] || BINDING_SNIPPETS.javascript).map((snippet) => (
                <div key={snippet.label}>
                  <div className="flex items-center justify-between mb-1">
                    <span className="text-xs text-text-muted">{snippet.label}</span>
                    <button
                      onClick={() => {
                        navigator.clipboard.writeText(snippet.code);
                        showToast('Snippet copied', 'success');
                      }}
                      className="p-1 rounded text-text-muted hover:text-accent hover:bg-accent/10"
                      title="Copy snippet"
                    >
                      <Copy className="w-3.5 h-3.5" />
                    </button>
                  </div>
                  <pre className="text-xs font-mono bg-bg-card border border-border rounded p-2 overflow-x-auto text-text-secondary">
                    {snippet.code}
                  </pre>
                </div>
              ))}
            </div>
          </details>

          <div>
            <label className="text-text-secondary text-sm font-medium">Code</label>
            <p className="text-text-muted text-xs mb-1">
              {form.language === 'go'
                ? form.trigger === 'http'
                  ? 'Define func Handler(req map[string]interface{}) map[string]interface{}'
                  : 'Top-level code with event variable'
                : form.trigger === 'http'
                  ? 'Define function handler(req) { ... }'
                  : 'Top-level code; global event is available'}
            </p>
            <textarea
              value={form.code}
              onChange={(e) => setForm((f) => ({ ...f, code: e.target.value }))}
              rows={16}
              className={inputCls + ' font-mono text-xs resize-y'}
              spellCheck={false}
            />
          </div>

          <div className="flex justify-end gap-3 pt-2">
            <button
              onClick={() => setShowModal(false)}
              className="px-4 py-2 rounded-lg text-text-secondary text-sm hover:bg-bg-input"
            >
              Cancel
            </button>
            <button
              onClick={save}
              disabled={saving}
              className="px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover disabled:opacity-50"
            >
              {saving ? 'Saving...' : editingId ? 'Update' : 'Deploy'}
            </button>
          </div>
        </div>
      </Modal>

      <Modal
        isOpen={testModalOpen && !!selected}
        onClose={() => setTestModalOpen(false)}
        title={selected ? `Test — ${selected.name}` : 'Test'}
        size="lg"
      >
        {selected && (
          <div className="space-y-4">
            {selected.trigger === 'http' && (
              <div>
                <label className="text-text-secondary text-sm font-medium">
                  Request body (JSON, optional)
                </label>
                <textarea
                  value={testBody}
                  onChange={(e) => setTestBody(e.target.value)}
                  rows={4}
                  className={inputCls + ' font-mono text-xs resize-y'}
                  placeholder='{ "hello": "world" }'
                  spellCheck={false}
                />
              </div>
            )}

            <div className="flex justify-end">
              <button
                onClick={() => runTest(selected, testBody)}
                disabled={testing}
                className="flex items-center gap-2 px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover disabled:opacity-50"
              >
                {testing ? (
                  <Loader2 className="w-4 h-4 animate-spin" />
                ) : (
                  <Play className="w-4 h-4" />
                )}
                {testResult ? 'Run again' : 'Run test'}
              </button>
            </div>

            {testing && !testResult && (
              <div className="flex items-center gap-2 text-text-muted text-sm py-4 justify-center">
                <Loader2 className="w-4 h-4 animate-spin" /> Running...
              </div>
            )}

            {testResult && (
              <div className="space-y-3 border-t border-border pt-4">
                <div className="flex items-center gap-3 flex-wrap text-sm">
                  <span
                    className={`px-2 py-0.5 rounded text-xs font-medium ${
                      testResult.status === 'success'
                        ? 'bg-success/10 text-success'
                        : testResult.status === 'timeout'
                          ? 'bg-yellow-500/10 text-yellow-500'
                          : 'bg-danger/10 text-danger'
                    }`}
                  >
                    {testResult.status}
                  </span>
                  <span className="text-text-muted text-xs font-mono">
                    {testResult.duration_ms} ms
                  </span>
                  <span className="text-text-muted text-xs font-mono">
                    RAM peak {formatBytes(testResult.metrics?.mem_peak_bytes || 0)}
                  </span>
                  <span className="text-text-muted text-xs font-mono">
                    CPU{' '}
                    {(testResult.metrics?.cpu_user_ms || 0) +
                      (testResult.metrics?.cpu_sys_ms || 0)}{' '}
                    ms
                  </span>
                </div>

                {testResult.response && (
                  <div>
                    <p className="text-xs text-text-muted mb-1">
                      HTTP response · status {testResult.response.status}
                    </p>
                    <pre className="text-xs bg-bg-input border border-border rounded p-3 overflow-auto max-h-48 font-mono text-text-secondary">
                      {typeof testResult.response.body === 'string'
                        ? testResult.response.body
                        : JSON.stringify(testResult.response.body, null, 2)}
                    </pre>
                  </div>
                )}

                {testResult.output && (
                  <div>
                    <p className="text-xs text-text-muted mb-1">Console output</p>
                    <pre className="text-xs bg-bg-input border border-border rounded p-3 overflow-auto max-h-48 font-mono text-text-secondary">
                      {testResult.output}
                    </pre>
                  </div>
                )}

                {testResult.error && (
                  <p className="text-danger text-sm">{testResult.error}</p>
                )}
              </div>
            )}
          </div>
        )}
      </Modal>

      <ConfirmDialog
        isOpen={deleteTargetId !== null}
        title="Delete function"
        message="Delete this function? This cannot be undone."
        confirmLabel="Delete"
        variant="danger"
        onCancel={() => setDeleteTargetId(null)}
        onConfirm={confirmDelete}
      />
    </div>
  );
}
