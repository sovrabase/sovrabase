import { useMemo, useState } from 'react';
import { useProjects } from '../../../store';
import { Banner, CopyButton, Field, Input, SaveBar, Section, Select, Spinner, Switch, TokenInput, useDraft, useToast } from '../../../ui';
import { api, adminProjectPath, formatDate } from '../../../api';
import type { ProjectDetail } from '../home/data';

const UNITS: Record<string, number> = { MB: 1_048_576, GB: 1_073_741_824, TB: 1_099_511_627_776 };

function splitBytes(bytes: number): { amount: string; unit: string } {
  if (!bytes || bytes <= 0) return { amount: '', unit: 'GB' };
  for (const u of ['TB', 'GB'] as const) {
    const v = bytes / UNITS[u];
    if (v >= 1 && Number.isInteger(v)) return { amount: String(v), unit: u };
  }
  return { amount: String(Math.round((bytes / UNITS.MB) * 100) / 100), unit: 'MB' };
}

interface GeneralForm {
  name: string;
  anyOrigin: boolean;
  origins: string[];
  quotaAmount: string;
  quotaUnit: string;
}

function validateOrigin(o: string): string | null {
  if (o === '*') return null;
  try {
    const u = new URL(o);
    if (u.pathname !== '/' || u.search || u.hash) return 'Use just the origin, e.g. https://app.example.com (no path).';
    return null;
  } catch {
    return 'Use a full origin with its scheme, e.g. https://app.example.com';
  }
}

export function GeneralSection({ projectId, detail, onSaved }: { projectId: string; detail: ProjectDetail | null; onSaved: () => Promise<void> }) {
  const { showToast } = useToast();
  const loadProjects = useProjects((s) => s.loadProjects);
  const [saving, setSaving] = useState(false);

  const saved = useMemo<GeneralForm | null>(() => {
    if (!detail) return null;
    const raw = (detail.allow_origins ?? detail.project.allow_origins ?? '*').trim();
    const list = raw
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean);
    const anyOrigin = !list.length || list.includes('*');
    const q = splitBytes(detail.storage_quota ?? detail.project.storage_quota ?? 0);
    return { name: detail.project.name, anyOrigin, origins: anyOrigin ? [] : list, quotaAmount: q.amount, quotaUnit: q.unit };
  }, [detail]);

  const { draft, patch, dirty, reset } = useDraft<GeneralForm>(saved);

  if (!detail || !draft) return <Spinner label="Loading settings" />;

  const nameError = !draft.name.trim() ? 'A project needs a name.' : null;
  const quotaNum = draft.quotaAmount.trim() === '' ? 0 : Number(draft.quotaAmount);
  const quotaError = Number.isNaN(quotaNum) || quotaNum < 0 ? 'Enter a positive number, or leave empty for no limit.' : null;
  const originsError = !draft.anyOrigin && draft.origins.length === 0 ? 'Add at least one origin, or allow any origin.' : null;

  const save = async () => {
    if (nameError || quotaError || originsError) return;
    setSaving(true);
    try {
      await api(adminProjectPath(projectId, ''), {
        method: 'PUT',
        body: JSON.stringify({
          name: draft.name.trim(),
          allow_origins: draft.anyOrigin ? '*' : draft.origins.join(','),
          storage_quota: Math.round(quotaNum * (UNITS[draft.quotaUnit] ?? UNITS.MB)),
        }),
      });
      showToast('Settings saved', 'success');
      await onSaved();
      loadProjects();
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div>
      <Section title="Project" description="The name shows in the console. The id is used by your apps and can't change.">
        <Field label="Name" error={nameError}>
          <Input value={draft.name} onChange={(e) => patch({ name: e.target.value })} />
        </Field>
        <Field label="Project id">
          <div className="flex items-center gap-2">
            <Input mono readOnly value={projectId} className="bg-sunken" />
            <CopyButton text={projectId} />
          </div>
        </Field>
        <p className="m-0 text-[12.5px] text-ink-3">Created {formatDate(detail.project.created_at)}</p>
      </Section>

      <Section title="Allowed origins" description="Websites allowed to call this project from a browser (CORS). Mobile apps and servers are not affected.">
        <Switch checked={draft.anyOrigin} onChange={(v) => patch({ anyOrigin: v })} label="Allow any origin" description="Fine while you build. Turn it off before launch and list your sites." />
        {!draft.anyOrigin && (
          <Field label="Origins" hint="Press Enter after each one." error={originsError}>
            <TokenInput mono value={draft.origins} onChange={(v) => patch({ origins: v })} validate={validateOrigin} placeholder="https://app.example.com" />
          </Field>
        )}
      </Section>

      <Section title="Storage limit" description="Uploads are refused once files in this project reach the limit." last>
        <Field label="Limit" optional hint="Leave empty for no limit." error={quotaError}>
          <div className="flex gap-2 max-w-[320px]">
            <Input inputMode="decimal" value={draft.quotaAmount} onChange={(e) => patch({ quotaAmount: e.target.value })} placeholder="No limit" className="flex-1" />
            <Select aria-label="Unit" value={draft.quotaUnit} onChange={(e) => patch({ quotaUnit: e.target.value })} options={Object.keys(UNITS).map((u) => ({ value: u, label: u }))} className="w-[90px]" />
          </div>
        </Field>
      </Section>

      {(nameError || quotaError || originsError) && dirty && (
        <div className="mt-4">
          <Banner tone="warn">Fix the highlighted fields to save.</Banner>
        </div>
      )}
      <SaveBar dirty={dirty} saving={saving} onSave={save} onDiscard={reset} />
    </div>
  );
}
