import { useEffect, useState } from 'react';
import { api, adminProjectPath } from '../../../api';
import type { ServerlessFunction } from '../../../types';
import { Button, ChipSelect, Field, Input, ScheduleInput, Segmented, Sheet, Switch, useToast } from '../../../ui';
import { LANGUAGES, TRIGGERS, bodyFromForm, defaultCode, type FnForm } from './shared';

const empty = (): FnForm => ({
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

export function CreateFunctionSheet({
  open,
  onClose,
  projectId,
  existing,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  projectId: string;
  existing: ServerlessFunction[];
  onCreated: (name: string) => void;
}) {
  const { showToast } = useToast();
  const [f, setF] = useState<FnForm>(empty);
  const [saving, setSaving] = useState(false);
  const [nameError, setNameError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setF(empty());
      setNameError(null);
    }
  }, [open]);

  const set = (p: Partial<FnForm>) => setF((x) => ({ ...x, ...p }));

  const create = async () => {
    const name = f.name.trim();
    if (!name) return setNameError('Give the function a name.');
    if (!/^[a-zA-Z0-9_-]+$/.test(name)) return setNameError('Use letters, numbers, - and _ only. It becomes part of the URL.');
    if (existing.some((x) => x.name === name)) return setNameError('A function with this name already exists.');
    if (f.trigger === 'schedule' && !f.schedule.trim()) return showToast('Pick a schedule', 'error');
    setNameError(null);
    setSaving(true);
    try {
      await api(adminProjectPath(projectId, '/functions'), { method: 'POST', body: JSON.stringify(bodyFromForm(f)) });
      showToast('Function deployed', 'success');
      onCreated(name);
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setSaving(false);
    }
  };

  return (
    <Sheet
      open={open}
      onClose={onClose}
      title="New function"
      description="It is deployed with starter code that you can edit next."
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" loading={saving} onClick={create}>
            Create and deploy
          </Button>
        </>
      }
    >
      <Field label="Name" error={nameError} hint="Lowercase with dashes works best, e.g. create-checkout.">
        <Input mono value={f.name} onChange={(e) => set({ name: e.target.value })} placeholder="hello-world" autoFocus />
      </Field>
      <Field label="Language">
        <Segmented
          aria-label="Language"
          value={f.language}
          onChange={(language) => set({ language, code: defaultCode(language, f.trigger) })}
          options={LANGUAGES.map((l) => ({ value: l.value, label: l.label }))}
        />
      </Field>
      <Field label="Runs when">
        <ChipSelect
          single
          aria-label="Trigger"
          options={TRIGGERS.map((t) => ({ value: t.value, label: t.label, description: t.description }))}
          value={[f.trigger]}
          onChange={(v) => v[0] && set({ trigger: v[0], code: defaultCode(f.language, v[0]) })}
        />
      </Field>
      {f.trigger.startsWith('record:') && (
        <Field label="Collection" optional hint="Leave empty to run for every collection.">
          <Input mono value={f.collection} onChange={(e) => set({ collection: e.target.value })} placeholder="orders" />
        </Field>
      )}
      {f.trigger === 'schedule' && (
        <Field label="Schedule">
          <ScheduleInput value={f.schedule} onChange={(schedule) => set({ schedule })} />
        </Field>
      )}
      <Field label="Timeout" hint="Between 1 and 30 seconds.">
        <Input
          type="number"
          min={1000}
          max={30000}
          step={500}
          value={f.timeout_ms}
          onChange={(e) => set({ timeout_ms: Number(e.target.value) })}
          suffix={<span className="text-[13px] text-ink-3 pr-1.5">ms</span>}
        />
      </Field>
      {f.trigger === 'http' && (
        <Switch checked={f.public} onChange={(v) => set({ public: v })} label="Public" description="Anyone with the URL can call it, no user token needed." />
      )}
      <Switch checked={f.enabled} onChange={(v) => set({ enabled: v })} label="On" description="Turn off to deploy without running it yet." />
    </Sheet>
  );
}
