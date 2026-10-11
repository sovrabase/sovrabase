import { useCallback, useEffect, useState } from 'react';
import { api } from '../../../api';
import { Banner, Card, Field, Input, SaveBar, Segmented, Spinner, Switch, useDraft, useToast } from '../../../ui';
import { colPath } from './shared';

interface RulesConfig {
  enabled: boolean;
  rules: Record<string, string>;
}

const ACTIONS: Array<{ key: string; label: string; hint: string }> = [
  { key: 'get', label: 'Read one', hint: 'Fetch a single record by id' },
  { key: 'list', label: 'List', hint: 'List and query records' },
  { key: 'create', label: 'Create', hint: 'Insert new records' },
  { key: 'update', label: 'Update', hint: 'Change existing records' },
  { key: 'delete', label: 'Delete', hint: 'Remove records' },
];

type Preset = 'anyone' | 'signed' | 'owner' | 'none' | 'custom';

const OWNER_RE = /^\s*auth\.uid\s*==\s*data\.([A-Za-z_][\w.]*)\s*$/;

function presetOf(expr: string): { preset: Preset; ownerField: string } {
  const e = expr.trim();
  if (e === 'true') return { preset: 'anyone', ownerField: 'user_id' };
  if (e === 'auth != null') return { preset: 'signed', ownerField: 'user_id' };
  if (e === '' || e === 'false') return { preset: 'none', ownerField: 'user_id' };
  const m = OWNER_RE.exec(e);
  if (m) return { preset: 'owner', ownerField: m[1] };
  return { preset: 'custom', ownerField: 'user_id' };
}

function exprFor(preset: Preset, ownerField: string, custom: string): string {
  switch (preset) {
    case 'anyone':
      return 'true';
    case 'signed':
      return 'auth != null';
    case 'owner':
      return `auth.uid == data.${ownerField || 'user_id'}`;
    case 'none':
      return 'false';
    case 'custom':
      return custom;
  }
}

const PRESETS: Array<{ value: Preset; label: string }> = [
  { value: 'anyone', label: 'Anyone' },
  { value: 'signed', label: 'Signed-in' },
  { value: 'owner', label: 'Owner only' },
  { value: 'none', label: 'Nobody' },
  { value: 'custom', label: 'Custom' },
];

const READING: Record<Preset, string> = {
  anyone: 'Any request with the project key is allowed.',
  signed: 'Only requests from a signed-in user are allowed.',
  owner: 'A user can only do this on records whose owner field is their own user id.',
  none: 'App requests are always refused. Use it for data only the console writes.',
  custom: 'Your own expression. Use auth.uid, auth.email, auth.role and data.<field> with ==, !=, &&, || and !.',
};

/** Who can read and write a collection: a preset per action, or a custom expression. */
export function RulesPanel({ projectId, collection }: { projectId: string; collection: string }) {
  const { showToast } = useToast();
  const [saved, setSaved] = useState<RulesConfig | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const { draft, setDraft, dirty, reset } = useDraft<RulesConfig>(saved);

  const load = useCallback(async () => {
    setError(null);
    try {
      const d = await api<RulesConfig>(colPath(projectId, collection, '/rules'));
      setSaved({ enabled: !!d.enabled, rules: d.rules || {} });
    } catch (e) {
      setError((e as Error).message);
    }
  }, [projectId, collection]);

  useEffect(() => {
    setSaved(null);
    load();
  }, [load]);

  if (error) return <Banner tone="danger" title="Rules could not be loaded">{error}</Banner>;
  if (!draft) return <Spinner label="Loading rules" />;

  const setRule = (action: string, expr: string) => setDraft({ ...draft, rules: { ...draft.rules, [action]: expr } });

  const save = async () => {
    setSaving(true);
    try {
      await api(colPath(projectId, collection, '/rules'), { method: 'POST', body: JSON.stringify(draft) });
      setSaved(draft);
      showToast('Access rules saved', 'success');
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <Switch
          checked={draft.enabled}
          onChange={(v) => setDraft({ ...draft, enabled: v })}
          label="Enforce access rules"
          description={
            draft.enabled
              ? 'Each app request is checked against the rules below. An action with no rule is refused.'
              : 'Off: any request with the project key can read and write this collection. The console is never limited.'
          }
        />
      </Card>

      <Card padded={false} className={draft.enabled ? '' : 'opacity-60'}>
        <div className="px-5 pt-4 pb-2">
          <h3 className="m-0 text-[15px] font-semibold text-ink">Who can do what</h3>
          <p className="m-0 text-[12.5px] text-ink-3">Pick a preset per action, or write a rule.</p>
        </div>
        {ACTIONS.map((a) => (
          <RuleRow key={a.key} action={a} expr={draft.rules[a.key] ?? ''} disabled={!draft.enabled} onChange={(e) => setRule(a.key, e)} />
        ))}
      </Card>

      <SaveBar dirty={dirty} saving={saving} onSave={save} onDiscard={reset} message="Unsaved rule changes" />
    </div>
  );
}

function RuleRow({
  action,
  expr,
  disabled,
  onChange,
}: {
  action: { key: string; label: string; hint: string };
  expr: string;
  disabled: boolean;
  onChange: (expr: string) => void;
}) {
  const parsed = presetOf(expr);
  // Keep "Custom" selected while typing even if the text happens to match a preset.
  const [forceCustom, setForceCustom] = useState(parsed.preset === 'custom');
  const preset: Preset = forceCustom ? 'custom' : parsed.preset;

  return (
    <fieldset disabled={disabled} className="m-0 px-5 py-4 border-0 border-t border-line flex flex-col gap-2.5 min-w-0">
      <legend className="sr-only">{action.label}</legend>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span aria-hidden>
          <span className="text-sm font-semibold text-ink">{action.label}</span>
          <span className="text-[12.5px] text-ink-3 ml-2">{action.hint}</span>
        </span>
        <Segmented
          aria-label={`${action.label} access`}
          size="sm"
          value={preset}
          options={PRESETS}
          onChange={(p) => {
            setForceCustom(p === 'custom');
            onChange(exprFor(p, parsed.ownerField, expr));
          }}
        />
      </div>
      <p className="m-0 text-[12.5px] text-ink-3">{READING[preset]}</p>
      {preset === 'owner' && (
        <Field label="Owner field" hint="The record field that holds the user id.">
          <Input mono prefix="data." value={parsed.ownerField} onChange={(e) => onChange(exprFor('owner', e.target.value.replace(/[^\w.]/g, ''), ''))} />
        </Field>
      )}
      {preset === 'custom' && (
        <Field label="Rule">
          <Input mono value={expr} placeholder="auth.uid == data.author_id || auth.role == 'admin'" onChange={(e) => onChange(e.target.value)} />
        </Field>
      )}
    </fieldset>
  );
}
