import type { ServerlessFunction } from '../../../types';

export const TRIGGERS: Array<{ value: string; label: string; description: string }> = [
  { value: 'http', label: 'HTTP', description: 'Called with a request to its URL' },
  { value: 'record:create', label: 'Record created', description: 'Runs after a record is created' },
  { value: 'record:update', label: 'Record updated', description: 'Runs after a record is updated' },
  { value: 'record:delete', label: 'Record deleted', description: 'Runs after a record is deleted' },
  { value: 'auth:signup', label: 'User signs up', description: 'Runs after a new account is created' },
  { value: 'auth:signin', label: 'User signs in', description: 'Runs after each sign-in' },
  { value: 'schedule', label: 'Schedule', description: 'Runs on a cron schedule' },
];

export const LANGUAGES = [
  { value: 'javascript', label: 'JavaScript' },
  { value: 'go', label: 'Go' },
];

export function triggerLabel(t: string) {
  return TRIGGERS.find((x) => x.value === t)?.label ?? t;
}

export function languageLabel(l?: string) {
  return l === 'go' ? 'Go' : 'JavaScript';
}

export function triggerSummary(fn: Pick<ServerlessFunction, 'trigger' | 'collection' | 'schedule'>) {
  const base = triggerLabel(fn.trigger);
  if (fn.trigger.startsWith('record:') && fn.collection) return `${base} in ${fn.collection}`;
  if (fn.trigger === 'schedule' && fn.schedule) return `${base} · ${fn.schedule}`;
  return base;
}

export function defaultCode(language: string, trigger: string) {
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
}

export function codeHint(language: string, trigger: string) {
  if (language === 'go')
    return trigger === 'http' ? 'Define func Handler(req map[string]interface{}) map[string]interface{}.' : 'Top-level code. The event variable is available.';
  return trigger === 'http' ? 'Define function handler(req) and return { status, body }.' : 'Top-level code. The global event is available.';
}

export const BINDING_SNIPPETS: Record<string, { label: string; code: string }[]> = {
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
    { label: 'Logging', code: `console.log('message', value);` },
  ],
  go: [
    {
      label: 'HTTP fetch',
      code: `import "function/host"

res, err := host.Fetch("GET", "https://api.example.com", nil)`,
    },
  ],
};

/** Public API origin (dev servers run the API on :6070). */
export function apiOrigin() {
  return window.location.origin.replace(/:\d+$/, ':6070');
}

export function invokeUrl(name: string) {
  return `${apiOrigin()}/api/v1/functions/${name}`;
}

export function statusTone(s: string): 'ok' | 'warn' | 'danger' {
  return s === 'success' ? 'ok' : s === 'timeout' ? 'warn' : 'danger';
}

export type FnForm = Pick<ServerlessFunction, 'name' | 'trigger' | 'code' | 'public' | 'timeout_ms' | 'enabled'> & {
  language: string;
  collection: string;
  schedule: string;
};

export function formFromFn(fn: ServerlessFunction): FnForm {
  return {
    name: fn.name,
    language: fn.language || 'javascript',
    trigger: fn.trigger,
    collection: fn.collection || '',
    schedule: fn.schedule || '',
    code: fn.code,
    public: fn.public,
    timeout_ms: fn.timeout_ms || 5000,
    enabled: fn.enabled,
  };
}

export function bodyFromForm(f: FnForm) {
  return {
    ...f,
    name: f.name.trim(),
    collection: f.collection.trim() || undefined,
    schedule: f.schedule.trim() || undefined,
  };
}
