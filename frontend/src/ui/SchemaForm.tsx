import type { ReactNode } from 'react';
import { Field, Input, Select, SecretInput, Switch, Textarea } from './Field';
import { ChipSelect, KeyValueEditor, ScheduleInput, TokenInput, kvFromRecord, kvToRecord } from './Controls';

/**
 * Field types a settings schema can declare. Plugins, integrations and project
 * settings all describe their forms with these, so every form looks and behaves the same.
 */
export type SchemaFieldType =
  | 'text'
  | 'secret'
  | 'url'
  | 'email'
  | 'number'
  | 'boolean'
  | 'select'
  | 'multiselect'
  | 'textarea'
  | 'json'
  | 'keyvalue'
  | 'list'
  | 'cron';

export interface SchemaField {
  key: string;
  label: string;
  type: SchemaFieldType;
  hint?: string;
  placeholder?: string;
  required?: boolean;
  options?: Array<{ value: string; label: string }>;
  /** For secrets: whether the server already stores a value. */
  isSet?: boolean;
  /** Environment variable that overrides this field; renders locked. */
  env?: string;
  /** Group heading this field sits under. */
  group?: string;
}

export type SchemaValues = Record<string, unknown>;

export function validateSchema(fields: SchemaField[], values: SchemaValues): Record<string, string> {
  const errs: Record<string, string> = {};
  for (const f of fields) {
    const v = values[f.key];
    const empty = v === undefined || v === null || v === '' || (Array.isArray(v) && v.length === 0);
    if (f.required && empty && !(f.type === 'secret' && f.isSet)) {
      errs[f.key] = `${f.label} is required.`;
      continue;
    }
    if (empty) continue;
    if (f.type === 'url' && typeof v === 'string' && !/^https?:\/\/\S+$/.test(v)) errs[f.key] = 'Enter a full URL starting with https://';
    if (f.type === 'email' && typeof v === 'string' && !/^\S+@\S+\.\S+$/.test(v)) errs[f.key] = 'Enter an email address.';
    if (f.type === 'json' && typeof v === 'string') {
      try {
        JSON.parse(v);
      } catch {
        errs[f.key] = 'This is not valid JSON.';
      }
    }
  }
  return errs;
}

function Control({ f, value, set }: { f: SchemaField; value: unknown; set: (v: unknown) => void }): ReactNode {
  switch (f.type) {
    case 'secret':
      return <SecretInput isSet={!!f.isSet} value={(value as string) ?? ''} onChange={set} placeholder={f.placeholder} />;
    case 'number':
      return <Input type="number" value={(value as number | string) ?? ''} onChange={(e) => set(e.target.value === '' ? '' : Number(e.target.value))} placeholder={f.placeholder} />;
    case 'boolean':
      return <Switch checked={!!value} onChange={set} aria-label={f.label} />;
    case 'select':
      return <Select value={(value as string) ?? ''} onChange={(e) => set(e.target.value)} options={f.options} placeholder={f.placeholder ?? 'Choose…'} />;
    case 'multiselect':
      return <ChipSelect options={(f.options ?? []).map((o) => ({ value: o.value, label: o.label }))} value={(value as string[]) ?? []} onChange={set} aria-label={f.label} />;
    case 'textarea':
      return <Textarea value={(value as string) ?? ''} onChange={(e) => set(e.target.value)} placeholder={f.placeholder} />;
    case 'json':
      return <Textarea mono rows={6} value={(value as string) ?? ''} onChange={(e) => set(e.target.value)} placeholder={f.placeholder ?? '{ }'} />;
    case 'keyvalue':
      return <KeyValueEditor value={kvFromRecord(value as Record<string, string>)} onChange={(rows) => set(kvToRecord(rows))} />;
    case 'list':
      return <TokenInput value={(value as string[]) ?? []} onChange={set} placeholder={f.placeholder ?? 'Type and press Enter'} />;
    case 'cron':
      return <ScheduleInput value={(value as string) ?? ''} onChange={set} />;
    case 'url':
      return <Input type="url" mono value={(value as string) ?? ''} onChange={(e) => set(e.target.value)} placeholder={f.placeholder ?? 'https://'} />;
    case 'email':
      return <Input type="email" value={(value as string) ?? ''} onChange={(e) => set(e.target.value)} placeholder={f.placeholder} />;
    default:
      return <Input value={(value as string) ?? ''} onChange={(e) => set(e.target.value)} placeholder={f.placeholder} />;
  }
}

/** Renders any settings schema with the shared input system. */
export function SchemaForm({
  fields,
  values,
  onChange,
  errors = {},
}: {
  fields: SchemaField[];
  values: SchemaValues;
  onChange: (next: SchemaValues) => void;
  errors?: Record<string, string>;
}) {
  const groups: Array<{ name?: string; fields: SchemaField[] }> = [];
  for (const f of fields) {
    const g = groups.find((x) => x.name === f.group);
    if (g) g.fields.push(f);
    else groups.push({ name: f.group, fields: [f] });
  }
  return (
    <div className="flex flex-col gap-6">
      {groups.map((g, i) => (
        <div key={g.name ?? i} className="flex flex-col gap-4">
          {g.name && <h4 className="m-0 text-[12px] font-semibold uppercase tracking-[.06em] text-ink-3">{g.name}</h4>}
          {g.fields.map((f) =>
            f.type === 'boolean' ? (
              <Switch key={f.key} checked={!!values[f.key]} onChange={(v) => onChange({ ...values, [f.key]: v })} label={f.label} description={f.hint} disabled={!!f.env} />
            ) : (
              <Field key={f.key} label={f.label} hint={f.hint} error={errors[f.key]} optional={!f.required} env={f.env}>
                <Control f={f} value={values[f.key]} set={(v) => onChange({ ...values, [f.key]: v })} />
              </Field>
            ),
          )}
        </div>
      ))}
    </div>
  );
}
