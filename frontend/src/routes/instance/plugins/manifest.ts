/**
 * Plugin manifests for the dashboard.
 *
 * The backend catalogue (/admin/integrations/catalog) describes each plugin's
 * settings. What it adds (actions, triggers, routes) and what it may touch
 * (outbound hosts, secrets, webhooks) is read from the server code in
 * internal/api/integrations_*.go and kept here, so the UI states it honestly.
 */
import { useEffect, useState } from 'react';
import { api, adminProjectPath } from '../../../api';
import { useProjects } from '../../../store';
import type { SchemaField, SchemaValues } from '../../../ui';

/* ---------- Backend shapes ---------- */

export interface CatalogField {
  key: string;
  label: string;
  type: string; // text | password | boolean | number | url
  required: boolean;
  placeholder?: string;
  help_text?: string;
}

export interface CatalogEntry {
  id: string;
  name: string;
  description: string;
  category: string;
  icon: string;
  color: string;
  config_fields: CatalogField[];
  supports_triggers?: boolean;
}

export interface ProjectIntegration {
  id: string;
  config: Record<string, unknown>;
  events?: string[];
  collections?: string[];
}

export const CATALOG_PATH = '/admin/integrations/catalog';
export const projectIntegrationsPath = (projectId: string) => adminProjectPath(projectId, '/integrations');

export async function loadCatalog(): Promise<CatalogEntry[]> {
  const d = await api<{ integrations: CatalogEntry[] }>(CATALOG_PATH);
  return d.integrations || [];
}

export async function loadProjectIntegrations(projectId: string): Promise<ProjectIntegration[]> {
  const d = await api<{ integrations: ProjectIntegration[] }>(projectIntegrationsPath(projectId));
  return d.integrations || [];
}

/** The PUT replaces the whole list; masked or empty secrets keep their stored value. */
export async function saveProjectIntegrations(projectId: string, list: ProjectIntegration[]) {
  await api(projectIntegrationsPath(projectId), { method: 'PUT', body: JSON.stringify({ integrations: list }) });
}

export const CATEGORY_LABELS: Record<string, string> = {
  payments: 'Payments',
  email: 'Email',
  sms: 'SMS',
  notifications: 'Notifications',
  search: 'Search',
  analytics: 'Analytics',
};
export const categoryLabel = (c: string) => CATEGORY_LABELS[c] ?? c.charAt(0).toUpperCase() + c.slice(1);

export const PLUGIN_API_DOCS = 'https://github.com/sovrabase/sovrabase/tree/main/plugin';
export const PLUGIN_EXAMPLE = 'https://github.com/sovrabase/sovrabase/blob/main/plugins/example.go';

/* ---------- What each plugin adds (read from the server code) ---------- */

export interface PluginAction {
  name: string;
  label: string;
  /** Input fields the action reads from `data`. */
  input?: string;
}

interface Extras {
  /** Hosts the server calls on this plugin's behalf. */
  hosts: string[];
  /** True when the host is whatever URL the person enters. */
  hostFromSetting?: string;
  actions: PluginAction[];
  /** Field that verifies incoming webhook signatures, when the plugin receives webhooks. */
  webhookVerifyField?: string;
}

const EXTRAS: Record<string, Extras> = {
  paypal: {
    hosts: ['api-m.paypal.com', 'api-m.sandbox.paypal.com'],
    actions: [
      { name: 'create_order', label: 'Create order', input: 'amount, currency, description' },
      { name: 'capture_order', label: 'Capture order', input: 'order_id' },
      { name: 'get_access_token', label: 'Get access token' },
    ],
    webhookVerifyField: 'webhook_id',
  },
  stripe: {
    hosts: ['api.stripe.com'],
    actions: [
      { name: 'create_checkout_session', label: 'Create checkout session', input: 'price_id, mode, success_url, cancel_url' },
      { name: 'create_payment_intent', label: 'Create payment intent', input: 'amount, currency' },
    ],
    webhookVerifyField: 'webhook_secret',
  },
  sendgrid: {
    hosts: ['api.sendgrid.com'],
    actions: [{ name: 'send_email', label: 'Send email', input: 'to, subject, text or html, from' }],
  },
  twilio: {
    hosts: ['api.twilio.com'],
    actions: [{ name: 'send_sms', label: 'Send SMS', input: 'to, body' }],
  },
  discord_webhook: {
    hosts: [],
    hostFromSetting: 'webhook_url',
    actions: [{ name: 'send_message', label: 'Send message', input: 'content, username' }],
  },
  slack_webhook: {
    hosts: [],
    hostFromSetting: 'webhook_url',
    actions: [{ name: 'send_message', label: 'Send message', input: 'text' }],
  },
  onesignal: {
    hosts: ['onesignal.com'],
    actions: [{ name: 'send_notification', label: 'Send notification', input: 'title, message, segments, url' }],
  },
  algolia: {
    hosts: ['<app id>.algolia.net'],
    actions: [
      { name: 'save_object', label: 'Save object', input: 'index, object' },
      { name: 'save_objects', label: 'Save objects', input: 'index, objects' },
      { name: 'search', label: 'Search', input: 'index, query, filters' },
      { name: 'delete_object', label: 'Delete object', input: 'index, object_id' },
      { name: 'set_settings', label: 'Set index settings', input: 'index, settings' },
      { name: 'list_indexes', label: 'List indexes' },
    ],
  },
};

export const extrasFor = (id: string): Extras => EXTRAS[id] ?? { hosts: [], actions: [] };

/** Events a trigger-capable plugin can fire on (internal/api/integrations_triggers.go). */
export const TRIGGER_EVENTS = [
  { value: 'record:create', label: 'Record created' },
  { value: 'record:update', label: 'Record updated' },
  { value: 'record:delete', label: 'Record deleted' },
  { value: 'auth:signup', label: 'User signed up' },
  { value: 'auth:signin', label: 'User signed in' },
];

export interface Adds {
  actions: PluginAction[];
  triggers: string[];
  routes: Array<{ method: string; path: string; note: string }>;
}

export function addsOf(def: CatalogEntry): Adds {
  const x = extrasFor(def.id);
  return {
    actions: x.actions,
    triggers: def.supports_triggers ? TRIGGER_EVENTS.map((e) => e.label) : [],
    routes: x.webhookVerifyField
      ? [{ method: 'POST', path: `/api/v1/integrations/${def.id}/webhook?project_key=…`, note: 'Receives events from the provider' }]
      : [],
  };
}

export function addsSummary(def: CatalogEntry): string[] {
  const a = addsOf(def);
  const out: string[] = [];
  if (a.actions.length) out.push(`${a.actions.length} action${a.actions.length > 1 ? 's' : ''}`);
  if (a.triggers.length) out.push('Event triggers');
  if (a.routes.length) out.push(`${a.routes.length} route`);
  return out;
}

/* ---------- Permissions, derived from the catalogue + server code ---------- */

export type PermissionKind = 'network' | 'secret' | 'webhook' | 'events' | 'data';

export interface Permission {
  kind: PermissionKind;
  title: string;
  detail: string;
}

export function secretFields(def: CatalogEntry) {
  return def.config_fields.filter((f) => f.type === 'password');
}

export function permissionsOf(def: CatalogEntry): Permission[] {
  const x = extrasFor(def.id);
  const out: Permission[] = [];
  if (x.hostFromSetting) {
    const f = def.config_fields.find((c) => c.key === x.hostFromSetting);
    out.push({
      kind: 'network',
      title: `Call the ${f?.label.toLowerCase() ?? 'URL'} you enter`,
      detail: 'Messages are posted to that address only.',
    });
  } else if (x.hosts.length) {
    out.push({
      kind: 'network',
      title: `Call ${x.hosts.join(' and ')}`,
      detail: 'Only when your app calls one of its actions. No other outbound host.',
    });
  }
  const secrets = secretFields(def);
  if (secrets.length) {
    out.push({
      kind: 'secret',
      title: `Store ${secrets.length} secret${secrets.length > 1 ? 's' : ''}`,
      detail: `${secrets.map((s) => s.label).join(', ')}. Write-only here: the dashboard only sees the first 4 characters.`,
    });
  }
  if (x.webhookVerifyField) {
    const f = def.config_fields.find((c) => c.key === x.webhookVerifyField);
    out.push({
      kind: 'webhook',
      title: 'Receive webhooks',
      detail: `Adds one public URL. Each event is written to the _webhooks collection. The signature is checked once ${f?.label ?? x.webhookVerifyField} is set.`,
    });
  }
  if (def.supports_triggers) {
    out.push({
      kind: 'events',
      title: 'Send project events out',
      detail: 'On the events you choose, it sends the collection name, record id and user email to the service.',
    });
  }
  if (!x.webhookVerifyField) {
    out.push({
      kind: 'data',
      title: 'No access to your collections',
      detail: 'It only sees what your app passes to its actions.',
    });
  }
  return out;
}

/* ---------- Settings: catalogue fields -> SchemaForm ---------- */

const TYPE_MAP: Record<string, SchemaField['type']> = {
  text: 'text',
  password: 'secret',
  url: 'url',
  boolean: 'boolean',
  number: 'number',
};

export const fieldTypeLabel = (t: string) => ({ password: 'secret', text: 'text', url: 'URL', boolean: 'on/off', number: 'number' })[t] ?? t;

/** A stored secret comes back masked ("sk_l••••" or "••••"); empty means not set. */
export const isMaskedSecret = (v: unknown) => typeof v === 'string' && v.includes('••••');

export function schemaFor(def: CatalogEntry, current?: ProjectIntegration): SchemaField[] {
  return def.config_fields.map((f) => ({
    key: f.key,
    label: f.label,
    type: TYPE_MAP[f.type] ?? 'text',
    hint: f.help_text,
    placeholder: f.placeholder,
    required: f.required,
    isSet: f.type === 'password' ? isMaskedSecret(current?.config?.[f.key]) : undefined,
  }));
}

export function valuesFor(def: CatalogEntry, current?: ProjectIntegration): SchemaValues {
  const out: SchemaValues = {};
  for (const f of def.config_fields) {
    const v = current?.config?.[f.key];
    if (f.type === 'password') out[f.key] = '';
    else if (f.type === 'boolean') out[f.key] = v === true || v === 'true';
    else if (f.type === 'number') out[f.key] = typeof v === 'number' ? v : v ? Number(v) : '';
    else out[f.key] = v == null ? '' : String(v);
  }
  return out;
}

/** Build the config the API expects. Empty secrets are kept server-side. */
export function configFrom(def: CatalogEntry, values: SchemaValues): Record<string, unknown> {
  const config: Record<string, unknown> = {};
  for (const f of def.config_fields) {
    const v = values[f.key];
    if (f.type === 'boolean') config[f.key] = !!v;
    else if (f.type === 'number') config[f.key] = v === '' || v == null ? 0 : Number(v);
    else config[f.key] = v == null ? '' : String(v);
  }
  return config;
}

/** Required settings that have no value yet. */
export function missingRequired(def: CatalogEntry, current?: ProjectIntegration): string[] {
  return def.config_fields
    .filter((f) => f.required)
    .filter((f) => {
      const v = current?.config?.[f.key];
      return v === undefined || v === null || v === '';
    })
    .map((f) => f.label);
}

/* ---------- Where each plugin is installed ---------- */

/** Map of plugin id -> project ids that have it installed. */
export function useInstallMap() {
  const { projects, loadProjects } = useProjects();
  const [map, setMap] = useState<Record<string, string[]> | null>(null);

  useEffect(() => {
    if (!projects.length) loadProjects();
  }, [projects.length, loadProjects]);

  useEffect(() => {
    let alive = true;
    const list = projects.slice(0, 60);
    Promise.allSettled(list.map((p) => loadProjectIntegrations(p.id).then((ints) => ({ id: p.id, ints })))).then((res) => {
      if (!alive) return;
      const m: Record<string, string[]> = {};
      for (const r of res) {
        if (r.status !== 'fulfilled') continue;
        for (const i of r.value.ints) (m[i.id] ??= []).push(r.value.id);
      }
      setMap(m);
    });
    return () => {
      alive = false;
    };
  }, [projects]);

  return { map, projects };
}
