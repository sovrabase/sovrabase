import { useCallback, useEffect, useState } from 'react';
import { Globe, Lock, Plus, Trash2 } from 'lucide-react';
import { api, adminProjectPath } from '../../../api';
import type { ConfigEntry } from '../../../types';
import { Badge, Banner, Button, Card, CardHeader, ConfirmDialog, DataTable, EmptyState, Field, IconButton, Input, Segmented, Sheet, Spinner, Switch, Textarea, useToast } from '../../../ui';

type VarType = 'string' | 'number' | 'boolean' | 'json';

interface Form {
  key: string;
  type: VarType;
  value: string;
  bool: boolean;
  pub: boolean;
}

const EMPTY: Form = { key: '', type: 'string', value: '', bool: true, pub: true };

function show(v: unknown) {
  return typeof v === 'object' ? JSON.stringify(v) : String(v);
}

/** Remote config: values functions read, and the app too when public. */
export function VariablesSection({ projectId }: { projectId: string }) {
  const { showToast } = useToast();
  const [entries, setEntries] = useState<ConfigEntry[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState<Form | null>(null);
  const [editing, setEditing] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleteKey, setDeleteKey] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const d = await api<{ data: ConfigEntry[] }>(adminProjectPath(projectId, '/config'));
      setEntries(d.data || []);
      setError(null);
    } catch (e) {
      setError((e as Error).message);
    }
  }, [projectId]);

  useEffect(() => {
    load();
  }, [load]);

  const openNew = () => {
    setForm(EMPTY);
    setEditing(false);
    setFormError(null);
  };

  const openEdit = (e: ConfigEntry) => {
    const type = (['string', 'number', 'boolean', 'json'].includes(e.type ?? '') ? e.type : 'string') as VarType;
    setForm({
      key: e.key,
      type,
      value: type === 'json' ? JSON.stringify(e.value, null, 2) : type === 'boolean' ? '' : show(e.value),
      bool: type === 'boolean' ? String(e.value) === 'true' : true,
      pub: !!e.public,
    });
    setEditing(true);
    setFormError(null);
  };

  const save = async () => {
    if (!form) return;
    const key = form.key.trim();
    if (!key) {
      setFormError('Give the variable a key.');
      return;
    }
    let value: unknown = form.value;
    if (form.type === 'number') {
      value = Number(form.value);
      if (form.value.trim() === '' || Number.isNaN(value)) {
        setFormError('The value must be a number.');
        return;
      }
    } else if (form.type === 'boolean') value = form.bool;
    else if (form.type === 'json') {
      try {
        value = JSON.parse(form.value);
      } catch {
        setFormError('The value is not valid JSON.');
        return;
      }
    }
    setSaving(true);
    setFormError(null);
    try {
      await api(adminProjectPath(projectId, '/config'), { method: 'POST', body: JSON.stringify({ key, value, type: form.type, public: form.pub }) });
      showToast(editing ? 'Variable updated' : 'Variable added', 'success');
      setForm(null);
      load();
    } catch (e) {
      setFormError((e as Error).message);
    } finally {
      setSaving(false);
    }
  };

  const remove = async () => {
    if (!deleteKey) return;
    try {
      await api(adminProjectPath(projectId, `/config/${encodeURIComponent(deleteKey)}`), { method: 'DELETE' });
      setEntries((es) => (es ?? []).filter((e) => e.key !== deleteKey));
      showToast('Variable deleted', 'success');
    } catch (e) {
      showToast((e as Error).message, 'error');
    }
  };

  if (error) return <Banner tone="danger" title="Variables could not be loaded">{error}</Banner>;
  if (!entries) return <Spinner label="Loading variables" />;

  return (
    <Card padded={false}>
      <CardHeader
        title="Variables"
        description="Settings your functions can read. Public ones are also readable by your app, so never make secrets public."
        actions={
          <Button size="sm" icon={<Plus className="w-3.5 h-3.5" />} onClick={openNew}>
            Add variable
          </Button>
        }
      />
      <div className="border-t border-line">
        <DataTable
          rows={entries}
          rowKey={(e) => e.key}
          onRowClick={openEdit}
          empty={
            <EmptyState
              title="No variables yet"
              description="Use them for feature flags, prices, limits: values you want to change without a deploy."
              action={
                <Button variant="primary" size="sm" onClick={openNew}>
                  Add variable
                </Button>
              }
            />
          }
          columns={[
            { key: 'key', header: 'Key', cell: (e) => <span className="font-mono text-[13px] text-ink truncate block">{e.key}</span> },
            {
              key: 'vis',
              header: 'Visibility',
              width: '110px',
              cell: (e) =>
                e.public ? (
                  <Badge tone="ok">
                    <Globe className="w-3 h-3" aria-hidden /> Public
                  </Badge>
                ) : (
                  <Badge>
                    <Lock className="w-3 h-3" aria-hidden /> Private
                  </Badge>
                ),
            },
            { key: 'value', header: 'Value', cell: (e) => <span className="font-mono text-[12.5px] text-ink-2 truncate block" title={show(e.value)}>{show(e.value)}</span> },
            { key: 'type', header: 'Type', width: '90px', hideOnMobile: true, cell: (e) => <span className="text-ink-3 text-[12.5px]">{e.type || 'string'}</span> },
            {
              key: 'x',
              header: <span className="sr-only">Actions</span>,
              width: '50px',
              align: 'right',
              cell: (e) => (
                <IconButton
                  size="sm"
                  label={`Delete ${e.key}`}
                  onClick={(ev) => {
                    ev.stopPropagation();
                    setDeleteKey(e.key);
                  }}
                >
                  <Trash2 className="w-4 h-4" />
                </IconButton>
              ),
            },
          ]}
        />
      </div>

      <Sheet
        open={!!form}
        onClose={() => setForm(null)}
        title={editing ? `Edit ${form?.key}` : 'Add variable'}
        footer={
          <>
            <Button variant="ghost" onClick={() => setForm(null)}>
              Cancel
            </Button>
            <Button variant="primary" loading={saving} onClick={save}>
              {editing ? 'Save' : 'Add variable'}
            </Button>
          </>
        }
      >
        {form && (
          <>
            {formError && <Banner tone="danger">{formError}</Banner>}
            <Field label="Key" hint={editing ? "Keys can't be renamed. Delete and add a new one instead." : 'Dots group related keys, e.g. shop.currency.'}>
              <Input mono value={form.key} readOnly={editing} className={editing ? 'bg-sunken' : undefined} onChange={(e) => setForm({ ...form, key: e.target.value })} placeholder="shop.currency" />
            </Field>
            <Field label="Type">
              <Segmented
                aria-label="Type"
                value={form.type}
                onChange={(t) => setForm({ ...form, type: t })}
                options={[
                  { value: 'string', label: 'Text' },
                  { value: 'number', label: 'Number' },
                  { value: 'boolean', label: 'Yes / no' },
                  { value: 'json', label: 'JSON' },
                ]}
              />
            </Field>
            {form.type === 'boolean' ? (
              <Switch checked={form.bool} onChange={(v) => setForm({ ...form, bool: v })} label={form.bool ? 'Yes (true)' : 'No (false)'} />
            ) : form.type === 'json' ? (
              <Field label="Value">
                <Textarea mono rows={8} value={form.value} onChange={(e) => setForm({ ...form, value: e.target.value })} placeholder='{"free_shipping_over": 60}' spellCheck={false} />
              </Field>
            ) : (
              <Field label="Value">
                <Input mono={form.type === 'number'} inputMode={form.type === 'number' ? 'decimal' : undefined} value={form.value} onChange={(e) => setForm({ ...form, value: e.target.value })} />
              </Field>
            )}
            <Switch checked={form.pub} onChange={(v) => setForm({ ...form, pub: v })} label="Public" description="Your app can read it with the project key. Leave off for anything secret." />
          </>
        )}
      </Sheet>

      <ConfirmDialog open={!!deleteKey} onClose={() => setDeleteKey(null)} onConfirm={remove} title={`Delete ${deleteKey}?`} confirmLabel="Delete variable">
        Functions and apps that read it get nothing back from now on.
      </ConfirmDialog>
    </Card>
  );
}
