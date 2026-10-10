import { useMemo, useState, type ReactNode } from 'react';
import { api } from '../../../api';
import { useDraft, useToast } from '../../../ui';

/** Raw /admin/config response. Secrets come back as a mask when set, '' when not. */
export type Config = Record<string, unknown>;

export const SECRET_KEYS = ['admin_password', 'jwt_secret', 's3_secret_key', 'smtp_password', 'email_api_key', 'email_api_secret', 'captcha_secret'];
export const MASK = '••••••••';

export const str = (c: Config, k: string) => (typeof c[k] === 'string' ? (c[k] as string) : c[k] == null ? '' : String(c[k]));
export const bool = (c: Config, k: string) => c[k] === true;
export const isSet = (c: Config, k: string) => typeof c[k] === 'string' && c[k] !== '';

export interface SectionProps {
  config: Config;
  /** Merge what was just saved into the loaded config. */
  onSaved: (body: Record<string, unknown>) => void;
}

/**
 * Draft + save for one settings section.
 * `toForm` and `toBody` must be stable (module-level functions).
 */
export function useConfigSection<T>(
  config: Config,
  toForm: (c: Config) => T,
  toBody: (d: T) => Record<string, unknown>,
  onSaved: (body: Record<string, unknown>) => void,
  validate?: (d: T) => string | null,
) {
  const saved = useMemo(() => toForm(config), [config, toForm]);
  const { draft, setDraft, patch, dirty, reset } = useDraft<T>(saved);
  const [saving, setSaving] = useState(false);
  const { showToast } = useToast();

  const save = async () => {
    if (!draft) return;
    const err = validate?.(draft);
    if (err) {
      showToast(err, 'error');
      return;
    }
    const body = toBody(draft);
    // Write-only secrets: leave out the ones nobody typed.
    for (const k of SECRET_KEYS) if (body[k] === '' || body[k] === undefined) delete body[k];
    setSaving(true);
    try {
      await api('/admin/config', { method: 'POST', body: JSON.stringify(body) });
      showToast('Settings saved', 'success');
      onSaved(body);
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setSaving(false);
    }
  };

  return { draft, setDraft, patch, dirty, reset, saving, save, saved };
}

/** Number field value to a number for the API (empty stays out). */
export function toNum(v: string): number | undefined {
  const n = Number(v);
  return v.trim() === '' || Number.isNaN(n) ? undefined : n;
}

export function DefList({ rows }: { rows: Array<[string, ReactNode]> }) {
  return (
    <dl className="m-0 grid grid-cols-1 sm:grid-cols-2 gap-x-8">
      {rows.map(([k, v]) => (
        <div key={k} className="flex justify-between gap-4 py-2 border-b border-line text-sm min-w-0">
          <dt className="text-ink-3 shrink-0">{k}</dt>
          <dd className="m-0 text-ink font-medium text-right truncate">{v}</dd>
        </div>
      ))}
    </dl>
  );
}

export function shortDateTime(s?: string) {
  if (!s) return '—';
  const d = new Date(s);
  return Number.isNaN(d.getTime())
    ? s
    : d.toLocaleString(undefined, { month: 'short', day: 'numeric', year: 'numeric', hour: '2-digit', minute: '2-digit' });
}
