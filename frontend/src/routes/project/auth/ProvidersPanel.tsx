import { useState } from 'react';
import { AtSign, ChevronDown, Plus, Wand2 } from 'lucide-react';
import { api, adminProjectPath } from '../../../api';
import type { OAuthProvider } from '../../../types';
import {
  Badge,
  Button,
  Card,
  CardHeader,
  ConfirmDialog,
  CopyButton,
  Field,
  Input,
  Row,
  SecretInput,
  Sheet,
  Switch,
  TokenInput,
  cx,
  useToast,
} from '../../../ui';
import { PROVIDER_TEMPLATES, ProviderBadge, TEMPLATE_KEYS, callbackUrl, detectTemplate, providerLabel } from './providers';

interface FormState {
  name: string;
  template: string;
  client_id: string;
  client_secret: string;
  redirect_url: string;
  auth_url: string;
  token_url: string;
  userinfo_url: string;
  scopes: string[];
  email_field: string;
  name_field: string;
  avatar_field: string;
  id_field: string;
}

function formFromTemplate(key: string): FormState {
  const f = PROVIDER_TEMPLATES[key]?.fields ?? {};
  const name = key === 'custom' ? '' : key;
  return {
    name,
    template: key,
    client_id: '',
    client_secret: '',
    redirect_url: name ? callbackUrl(name) : '',
    auth_url: f.auth_url ?? '',
    token_url: f.token_url ?? '',
    userinfo_url: f.userinfo_url ?? '',
    scopes: f.scopes ?? [],
    email_field: f.email_field ?? '',
    name_field: f.name_field ?? '',
    avatar_field: f.avatar_field ?? '',
    id_field: f.id_field ?? '',
  };
}

const isUrl = (s: string) => /^https?:\/\/\S+$/.test(s.trim());

export function ProvidersPanel({
  projectId,
  providers,
  usersByMethod,
  reload,
}: {
  projectId: string;
  providers: OAuthProvider[];
  usersByMethod: Record<string, number>;
  reload: () => Promise<void>;
}) {
  const { showToast } = useToast();
  const [editing, setEditing] = useState<OAuthProvider | null>(null);
  const [sheetOpen, setSheetOpen] = useState(false);
  const [form, setForm] = useState<FormState>(formFromTemplate('custom'));
  const [advanced, setAdvanced] = useState(false);
  const [saving, setSaving] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [turnOff, setTurnOff] = useState<OAuthProvider | null>(null);


  const byTemplate = new Map<string, OAuthProvider>();
  const custom: OAuthProvider[] = [];
  for (const p of providers) {
    const t = detectTemplate(p);
    if (t !== 'custom' && !byTemplate.has(t)) byTemplate.set(t, p);
    else custom.push(p);
  }

  const openNew = (key: string) => {
    setEditing(null);
    setForm(formFromTemplate(key));
    setAdvanced(key === 'custom');
    setErrors({});
    setSheetOpen(true);
  };

  const openEdit = (p: OAuthProvider) => {
    setEditing(p);
    setForm({
      name: p.name,
      template: detectTemplate(p),
      client_id: p.client_id,
      client_secret: '',
      redirect_url: p.redirect_url || callbackUrl(p.name),
      auth_url: p.auth_url,
      token_url: p.token_url,
      userinfo_url: p.userinfo_url,
      scopes: p.scopes ?? [],
      email_field: p.email_field,
      name_field: p.name_field,
      avatar_field: p.avatar_field,
      id_field: p.id_field,
    });
    setAdvanced(false);
    setErrors({});
    setSheetOpen(true);
  };

  const putProviders = (list: OAuthProvider[]) =>
    api(adminProjectPath(projectId, '/auth/providers'), { method: 'PUT', body: JSON.stringify({ providers: list }) });

  const validate = (): Record<string, string> => {
    const e: Record<string, string> = {};
    const name = form.name.trim();
    if (!name) e.name = 'Give the provider a name.';
    else if (!/^[a-z0-9_-]+$/i.test(name)) e.name = 'Use letters, numbers, - and _ only.';
    else if (!editing && providers.some((p) => p.name === name)) e.name = `A provider named "${name}" already exists.`;
    if (!form.client_id.trim()) e.client_id = 'Client ID is required.';
    if (!editing && !form.client_secret.trim()) e.client_secret = 'Client secret is required.';
    if (form.redirect_url && !isUrl(form.redirect_url)) e.redirect_url = 'Enter a full URL starting with https://';
    for (const k of ['auth_url', 'token_url', 'userinfo_url'] as const) {
      if (!form[k].trim()) e[k] = 'Required.';
      else if (!isUrl(form[k])) e[k] = 'Enter a full URL.';
    }
    return e;
  };

  const save = async () => {
    const e = validate();
    setErrors(e);
    if (Object.keys(e).length) {
      if (e.auth_url || e.token_url || e.userinfo_url) setAdvanced(true);
      return;
    }
    setSaving(true);
    try {
      const provider: OAuthProvider = {
        name: form.name.trim(),
        client_id: form.client_id.trim(),
        // Empty means "keep current": the server only knows its own stored value,
        // so we hand back the masked value it gave us.
        client_secret: form.client_secret.trim() || editing?.client_secret || '',
        redirect_url: form.redirect_url.trim(),
        auth_url: form.auth_url.trim(),
        token_url: form.token_url.trim(),
        userinfo_url: form.userinfo_url.trim(),
        scopes: form.scopes,
        email_field: form.email_field.trim(),
        name_field: form.name_field.trim(),
        avatar_field: form.avatar_field.trim(),
        id_field: form.id_field.trim(),
      };
      const list = editing ? providers.map((p) => (p.name === editing.name ? provider : p)) : [...providers, provider];
      await putProviders(list);
      showToast(editing ? `${providerLabel(provider.name)} updated` : `${providerLabel(provider.name)} is on`, 'success');
      setSheetOpen(false);
      await reload();
    } catch (err) {
      showToast((err as Error).message || 'Could not save', 'error');
    } finally {
      setSaving(false);
    }
  };

  const remove = async () => {
    if (!turnOff) return;
    try {
      await putProviders(providers.filter((p) => p.name !== turnOff.name));
      showToast(`${providerLabel(turnOff.name)} turned off`, 'success');
      await reload();
    } catch (err) {
      showToast((err as Error).message || 'Could not turn it off', 'error');
      throw err;
    }
  };

  const set = (patch: Partial<FormState>) => setForm((f) => ({ ...f, ...patch }));
  const tplLabel = form.template !== 'custom' ? PROVIDER_TEMPLATES[form.template]?.label : undefined;

  const providerRow = (key: string, p: OAuthProvider | undefined, label: string) => {
    const count = p ? usersByMethod[p.name] ?? 0 : 0;
    return (
      <Row key={p?.name ?? key} className="py-4">
        <ProviderBadge name={key} />
        <div className="flex-1 min-w-[180px] flex flex-col gap-1">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-sm font-semibold text-ink">{label}</span>
            {key === 'custom' && <Badge>Custom OAuth</Badge>}
          </div>
          <span className="text-[12.5px] text-ink-3">
            {p ? (
              <>
                Configured · {count} {count === 1 ? 'user' : 'users'} · <span className="font-mono">{p.client_id.length > 28 ? p.client_id.slice(0, 28) + '…' : p.client_id}</span>
              </>
            ) : (
              'Off'
            )}
          </span>
          {p && (
            <div className="flex items-center gap-2 mt-1.5 min-w-0">
              <code className="flex-1 min-w-0 truncate font-mono text-[12px] text-ink-2 bg-sunken rounded-md px-2 py-1">{p.redirect_url || callbackUrl(p.name)}</code>
              <CopyButton text={p.redirect_url || callbackUrl(p.name)} />
            </div>
          )}
        </div>
        <div className="flex items-center gap-3 ml-auto">
          {p ? (
            <Button size="sm" onClick={() => openEdit(p)}>
              Options
            </Button>
          ) : (
            <Button size="sm" onClick={() => openNew(key)}>
              Set up
            </Button>
          )}
          <Switch
            checked={!!p}
            onChange={(on) => (on ? openNew(key) : p && setTurnOff(p))}
            aria-label={`${label} sign-in`}
          />
        </div>
      </Row>
    );
  };

  return (
    <div className="flex flex-col gap-5">
      <Card padded={false}>
        <CardHeader
          title="How people sign in"
          description="Turn a method on, then fill only what it needs. The callback URL is shown for you to copy."
          actions={
            <Button size="sm" icon={<Plus className="w-4 h-4" />} onClick={() => openNew('custom')}>
              Custom provider
            </Button>
          }
        />
        <div className="border-t border-line">
          <Row className="py-4">
            <span className="w-9 h-9 shrink-0 inline-flex items-center justify-center rounded-[9px] border border-line bg-sunken text-ink">
              <AtSign className="w-4 h-4" aria-hidden />
            </span>
            <div className="flex-1 min-w-[180px]">
              <div className="text-sm font-semibold text-ink">Email and password</div>
              <div className="text-[12.5px] text-ink-3">Built in · verification email sent on sign-up · {usersByMethod.email ?? 0} users</div>
            </div>
            <Badge tone="ok" dot>
              Always on
            </Badge>
          </Row>
          <Row className="py-4">
            <span className="w-9 h-9 shrink-0 inline-flex items-center justify-center rounded-[9px] border border-line bg-sunken text-ink">
              <Wand2 className="w-4 h-4" aria-hidden />
            </span>
            <div className="flex-1 min-w-[180px]">
              <div className="text-sm font-semibold text-ink">Magic link</div>
              <div className="text-[12.5px] text-ink-3">Built in · the email uses the Magic link template</div>
            </div>
            <Badge tone="ok" dot>
              Available
            </Badge>
          </Row>
          {TEMPLATE_KEYS.map((k) => providerRow(k, byTemplate.get(k), PROVIDER_TEMPLATES[k].label))}
          {custom.map((p) => providerRow('custom', p, p.name))}
        </div>
      </Card>

      <Sheet
        open={sheetOpen}
        onClose={() => setSheetOpen(false)}
        width={560}
        title={editing ? `${providerLabel(editing.name)} options` : tplLabel ? `Set up ${tplLabel}` : 'Custom OAuth provider'}
        description={
          tplLabel
            ? `Create an OAuth app in the ${PROVIDER_TEMPLATES[form.template]?.console ?? tplLabel + ' console'}, then paste its keys here.`
            : 'Any OAuth 2 provider that returns the user from a userinfo endpoint.'
        }
        footer={
          <>
            <Button variant="ghost" onClick={() => setSheetOpen(false)}>
              Cancel
            </Button>
            <Button variant="primary" loading={saving} onClick={save}>
              {editing ? 'Save' : 'Turn on'}
            </Button>
          </>
        }
      >
        {form.template === 'custom' && (
          <Field label="Name" hint="Used in the sign-in and callback URLs." error={errors.name}>
            <Input
              mono
              value={form.name}
              disabled={!!editing}
              placeholder="my-sso"
              onChange={(e) => {
                const name = e.target.value;
                set({ name, redirect_url: !form.redirect_url || form.redirect_url === callbackUrl(form.name) ? callbackUrl(name) : form.redirect_url });
              }}
            />
          </Field>
        )}
        {form.template !== 'custom' && errors.name && <p className="m-0 text-[12.5px] text-danger">{errors.name}</p>}
        <Field label="Client ID" error={errors.client_id}>
          <Input mono value={form.client_id} onChange={(e) => set({ client_id: e.target.value })} autoComplete="off" />
        </Field>
        <Field label="Client secret" error={errors.client_secret} hint={editing ? 'Stored on the server. It is never shown again.' : undefined}>
          <SecretInput isSet={!!editing && !!editing.client_secret} value={form.client_secret} onChange={(v) => set({ client_secret: v })} />
        </Field>
        <Field
          label="Callback URL"
          hint={`Paste this exact URL in the provider's allowed redirect URIs${tplLabel ? ` in ${tplLabel}` : ''}.`}
          error={errors.redirect_url}
          aside={<CopyButton text={form.redirect_url} />}
        >
          <Input mono value={form.redirect_url} onChange={(e) => set({ redirect_url: e.target.value })} placeholder={callbackUrl(form.name)} />
        </Field>
        <Field label="Scopes" hint="Type a scope and press Enter.">
          <TokenInput mono value={form.scopes} onChange={(scopes) => set({ scopes })} placeholder="openid" />
        </Field>

        <div className="border border-line rounded-lg">
          <button
            type="button"
            aria-expanded={advanced}
            onClick={() => setAdvanced((a) => !a)}
            className="w-full flex items-center justify-between gap-3 px-4 h-11 text-[13.5px] font-medium text-ink-2 hover:text-ink"
          >
            Endpoints and field mapping
            <ChevronDown className={cx('w-4 h-4 transition-transform', advanced && 'rotate-180')} aria-hidden />
          </button>
          {advanced && (
            <div className="flex flex-col gap-4 px-4 pb-4 pt-1 border-t border-line">
              <Field label="Authorization URL" error={errors.auth_url} className="mt-3">
                <Input mono value={form.auth_url} onChange={(e) => set({ auth_url: e.target.value })} placeholder="https://" />
              </Field>
              <Field label="Token URL" error={errors.token_url}>
                <Input mono value={form.token_url} onChange={(e) => set({ token_url: e.target.value })} placeholder="https://" />
              </Field>
              <Field label="Userinfo URL" error={errors.userinfo_url}>
                <Input mono value={form.userinfo_url} onChange={(e) => set({ userinfo_url: e.target.value })} placeholder="https://" />
              </Field>
              <p className="m-0 text-[12.5px] text-ink-3">
                Which keys of the userinfo response hold each value. Dot paths work. Leave empty for the defaults (email, name, avatar_url, id).
              </p>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <Field label="Email field" optional>
                  <Input mono value={form.email_field} onChange={(e) => set({ email_field: e.target.value })} placeholder="email" />
                </Field>
                <Field label="Name field" optional>
                  <Input mono value={form.name_field} onChange={(e) => set({ name_field: e.target.value })} placeholder="name" />
                </Field>
                <Field label="Avatar field" optional>
                  <Input mono value={form.avatar_field} onChange={(e) => set({ avatar_field: e.target.value })} placeholder="avatar_url" />
                </Field>
                <Field label="ID field" optional>
                  <Input mono value={form.id_field} onChange={(e) => set({ id_field: e.target.value })} placeholder="id" />
                </Field>
              </div>
            </div>
          )}
        </div>
      </Sheet>

      <ConfirmDialog
        open={!!turnOff}
        onClose={() => setTurnOff(null)}
        onConfirm={remove}
        title={turnOff ? `Turn off ${providerLabel(turnOff.name)}?` : ''}
        confirmLabel="Turn off"
      >
        <p className="m-0">
          Its client ID and secret are removed. People who signed up with it keep their accounts but can't sign in with it until you set it up again.
        </p>
      </ConfirmDialog>
    </div>
  );
}
