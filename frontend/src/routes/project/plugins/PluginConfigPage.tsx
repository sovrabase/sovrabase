import { useEffect, useMemo, useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { ArrowRight, Blocks, ChevronRight, Info, Trash2 } from 'lucide-react';
import { api, adminProjectPath } from '../../../api';
import { useProject } from '../../../shell/ProjectContext';
import { projectPath } from '../../../shell/nav';
import {
  Badge,
  Banner,
  Button,
  ButtonLink,
  Card,
  CardHeader,
  ChipSelect,
  CodeBlock,
  ConfirmDialog,
  CopyButton,
  EmptyState,
  Field,
  Input,
  Page,
  PageHeader,
  SaveBar,
  SchemaForm,
  Spinner,
  Tabs,
  TokenInput,
  cx,
  useDraft,
  useToast,
  validateSchema,
  type SchemaValues,
} from '../../../ui';
import {
  addsOf,
  categoryLabel,
  configFrom,
  extrasFor,
  missingRequired,
  permissionsOf,
  saveProjectIntegrations,
  schemaFor,
  TRIGGER_EVENTS,
  valuesFor,
  type CatalogEntry,
  type ProjectIntegration,
} from '../../instance/plugins/manifest';
import { AddsList, PermissionList, PluginIcon } from '../../instance/plugins/PluginBits';
import { snippetFor } from './snippets';

interface FormState {
  values: SchemaValues;
  events: string[];
  collections: string[];
}

type TabKey = 'settings' | 'permissions' | 'usage';

export function PluginConfigPage({
  provider,
  catalog,
  installed,
  error,
  reload,
}: {
  provider: string;
  catalog: CatalogEntry[] | null;
  installed: ProjectIntegration[] | null;
  error: string | null;
  reload: () => Promise<void>;
}) {
  const { projectId } = useProject();
  if (!catalog || !installed) return <Spinner label="Loading plugin" />;
  const def = catalog.find((c) => c.id === provider);
  if (!def) {
    return (
      <Page>
        <Crumbs projectId={projectId} name={provider} />
        {error && <Banner tone="danger" title="Plugins could not be loaded">{error}</Banner>}
        <EmptyState
          icon={<Blocks className="w-5 h-5" />}
          title="This plugin is not in the catalogue"
          action={<ButtonLink to={projectPath(projectId, 'plugins')}>All plugins</ButtonLink>}
        />
      </Page>
    );
  }
  return <Config key={def.id} def={def} installed={installed} reload={reload} />;
}

function Crumbs({ projectId, name }: { projectId: string; name: string }) {
  return (
    <nav aria-label="Breadcrumb" className="flex items-center gap-1.5 text-[13px] text-ink-3 mb-3">
      <Link to={projectPath(projectId, 'plugins')} className="hover:text-ink">
        Plugins
      </Link>
      <ChevronRight className="w-3.5 h-3.5" aria-hidden />
      <span className="text-ink-2 truncate">{name}</span>
    </nav>
  );
}

function Config({ def, installed, reload }: { def: CatalogEntry; installed: ProjectIntegration[]; reload: () => Promise<void> }) {
  const { project, projectId } = useProject();
  const { showToast } = useToast();
  const navigate = useNavigate();
  const location = useLocation();
  const current = installed.find((i) => i.id === def.id);
  const isInstalled = !!current;
  const [approved, setApproved] = useState<boolean>(!!(location.state as { approved?: boolean } | null)?.approved);
  const [tab, setTab] = useState<TabKey>('settings');
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [confirmRemove, setConfirmRemove] = useState(false);
  const [collections, setCollections] = useState<string[]>([]);

  const fields = useMemo(() => schemaFor(def, current), [def, current]);
  const saved = useMemo<FormState>(
    () => ({ values: valuesFor(def, current), events: current?.events ?? [], collections: current?.collections ?? [] }),
    [def, current],
  );
  const { draft, setDraft, dirty, reset } = useDraft<FormState>(saved);
  const adds = useMemo(() => addsOf(def), [def]);
  const perms = useMemo(() => permissionsOf(def), [def]);
  const extras = extrasFor(def.id);

  useEffect(() => {
    if (!def.supports_triggers) return;
    api<{ collections: Array<string | { name: string }> }>(adminProjectPath(projectId, '/collections'))
      .then((d) => setCollections((d.collections || []).map((c) => (typeof c === 'string' ? c : c.name))))
      .catch(() => setCollections([]));
  }, [def.supports_triggers, projectId]);

  const save = async () => {
    if (!draft) return;
    const errs = validateSchema(fields, draft.values);
    setErrors(errs);
    if (Object.keys(errs).length) {
      setTab('settings');
      showToast('Some settings need attention', 'error');
      return;
    }
    const entry: ProjectIntegration = { id: def.id, config: configFrom(def, draft.values) };
    if (def.supports_triggers) {
      entry.events = draft.events;
      entry.collections = draft.collections;
    }
    const list = isInstalled ? installed.map((i) => (i.id === def.id ? entry : i)) : [...installed, entry];
    setSaving(true);
    try {
      await saveProjectIntegrations(projectId, list);
      showToast(isInstalled ? `${def.name} saved` : `${def.name} installed`, 'success');
      await reload();
    } catch (e) {
      showToast((e as Error).message || 'Could not save', 'error');
    } finally {
      setSaving(false);
    }
  };

  const uninstall = async () => {
    try {
      await saveProjectIntegrations(
        projectId,
        installed.filter((i) => i.id !== def.id),
      );
      showToast(`${def.name} uninstalled`, 'success');
      await reload();
      navigate(projectPath(projectId, 'plugins'));
    } catch (e) {
      showToast((e as Error).message || 'Could not uninstall', 'error');
    }
  };

  const missing = current ? missingRequired(def, current) : [];
  const status = !isInstalled ? (
    <Badge>Not installed</Badge>
  ) : missing.length ? (
    <Badge tone="warn" dot>
      Needs setup
    </Badge>
  ) : (
    <Badge tone="ok" dot>
      Active
    </Badge>
  );

  const header = (
    <>
      <Crumbs projectId={projectId} name={def.name} />
      <div className="flex items-start gap-4">
        <PluginIcon def={def} size="lg" />
        <div className="flex-1 min-w-0">
          <PageHeader
            title={def.name}
            meta={status}
            description={`${categoryLabel(def.category)} · ${def.description}`}
            actions={
              isInstalled ? (
                <Button variant="ghost" icon={<Trash2 className="w-4 h-4" />} onClick={() => setConfirmRemove(true)}>
                  Uninstall
                </Button>
              ) : undefined
            }
          />
        </div>
      </div>
    </>
  );

  /* Step 1 of an install: review what it may touch. */
  if (!isInstalled && !approved) {
    return (
      <Page>
        {header}
        <div className="grid grid-cols-1 lg:grid-cols-[minmax(0,1fr)_340px] gap-5 items-start">
          <Card className="flex flex-col gap-5">
            <div className="flex flex-col gap-1">
              <h2 className="m-0 text-[15px] font-semibold text-ink">
                Install {def.name} in {project.name}?
              </h2>
              <p className="m-0 text-[13px] text-ink-3">Review what it will be able to do. Nothing is saved until you fill in its settings.</p>
            </div>
            <PermissionList items={perms} />
            <div className="flex flex-wrap gap-2">
              <Button variant="primary" icon={<ArrowRight className="w-4 h-4" />} onClick={() => setApproved(true)}>
                Approve and configure
              </Button>
              <ButtonLink variant="ghost" to={projectPath(projectId, 'plugins')}>
                Cancel
              </ButtonLink>
            </div>
          </Card>
          <Card>
            <h2 className="m-0 mb-4 text-[15px] font-semibold text-ink">What it adds</h2>
            <AddsList adds={adds} />
          </Card>
        </div>
      </Page>
    );
  }

  if (!draft) return <Spinner />;

  const webhookUrl = extras.webhookVerifyField
    ? `${window.location.origin}/api/v1/integrations/${def.id}/webhook?project_key=${project.api_key ?? 'PROJECT_KEY'}`
    : null;

  const settingsForm = (
    <div className="flex flex-col gap-5">
      {isInstalled && missing.length > 0 && (
        <Banner tone="warn" title="Required settings are missing">
          {missing.join(', ')}. Its actions will fail until they are filled in.
        </Banner>
      )}
      <Card>
        <h2 className="m-0 mb-4 text-[15px] font-semibold text-ink">Settings</h2>
        <SchemaForm key={fields.map((f) => (f.isSet ? 1 : 0)).join('')} fields={fields} values={draft.values} onChange={(values) => setDraft({ ...draft, values })} errors={errors} />
      </Card>

      {def.supports_triggers && (
        <Card className="flex flex-col gap-4">
          <div className="flex flex-col gap-1">
            <h2 className="m-0 text-[15px] font-semibold text-ink">When to notify</h2>
            <p className="m-0 text-[13px] text-ink-3">
              {def.name} posts a message on these events. For conditions or other steps,{' '}
              <Link to={projectPath(projectId, 'automations')} className="text-accent hover:text-accent-2 font-medium">
                build an automation
              </Link>
              .
            </p>
          </div>
          <Field label="Events" hint="Pick none to fire on every event.">
            <ChipSelect
              aria-label="Events"
              options={TRIGGER_EVENTS}
              value={draft.events}
              onChange={(events) => setDraft({ ...draft, events })}
            />
          </Field>
          <Field label="Collections" optional hint="Only record events from these collections. Leave empty for all.">
            <TokenInput
              mono
              value={draft.collections}
              onChange={(c) => setDraft({ ...draft, collections: c })}
              suggestions={collections}
              placeholder="orders"
            />
          </Field>
        </Card>
      )}

      {webhookUrl && (
        <Card className="flex flex-col gap-3">
          <h2 className="m-0 text-[15px] font-semibold text-ink">Webhook URL</h2>
          <Field label={`Paste this in your ${def.name} dashboard`} hint="Events are stored in the _webhooks collection.">
            <Input mono readOnly value={webhookUrl} suffix={<CopyButton text={webhookUrl} />} />
          </Field>
        </Card>
      )}

      {isInstalled ? (
        <div className="flex items-start gap-2 text-[12.5px] text-ink-3">
          <Info className="w-4 h-4 shrink-0 mt-px" aria-hidden />
          <span>
            There is no test button yet: the server has no admin test endpoint for plugins. Call an action from your app (see the Usage tab)
            to check the settings.
          </span>
        </div>
      ) : (
        <div className="flex flex-wrap gap-2">
          <Button variant="primary" loading={saving} onClick={save}>
            Install {def.name}
          </Button>
          <ButtonLink variant="ghost" to={projectPath(projectId, 'plugins')}>
            Cancel
          </ButtonLink>
        </div>
      )}
    </div>
  );

  const side = (
    <Card>
      <h2 className="m-0 mb-4 text-[15px] font-semibold text-ink">What it adds</h2>
      <AddsList
        adds={adds}
        triggersAside={
          <Link to={projectPath(projectId, 'automations')} className="text-[12.5px] text-accent hover:text-accent-2 font-medium">
            Open Automations
          </Link>
        }
      />
    </Card>
  );

  return (
    <Page>
      {header}
      {isInstalled && (
        <Tabs<TabKey>
          className="mb-5"
          value={tab}
          onChange={setTab}
          tabs={[
            { value: 'settings', label: 'Settings' },
            { value: 'permissions', label: 'Permissions' },
            { value: 'usage', label: 'Usage' },
          ]}
        />
      )}
      <div className="grid grid-cols-1 lg:grid-cols-[minmax(0,1fr)_340px] gap-5 items-start">
        <div className="min-w-0">
          {tab === 'settings' || !isInstalled ? (
            settingsForm
          ) : tab === 'permissions' ? (
            <Card padded={false}>
              <CardHeader title="Approved permissions" description="Granted when it was installed. Uninstall to revoke them all." />
              <div className="px-5 pb-5">
                <PermissionList items={perms} granted />
              </div>
            </Card>
          ) : (
            <Card className="flex flex-col gap-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <h2 className="m-0 text-[15px] font-semibold text-ink">Use it from your app</h2>
                <CopyButton text={snippetFor(def)} />
              </div>
              <CodeBlock className="max-h-[420px]">{snippetFor(def)}</CodeBlock>
            </Card>
          )}
        </div>
        <div className={cx('min-w-0', !isInstalled && 'order-first lg:order-none')}>{side}</div>
      </div>

      {isInstalled && (
        <SaveBar
          dirty={dirty}
          saving={saving}
          onSave={save}
          onDiscard={() => {
            reset();
            setErrors({});
          }}
        />
      )}

      <ConfirmDialog
        open={confirmRemove}
        onClose={() => setConfirmRemove(false)}
        onConfirm={uninstall}
        title={`Uninstall ${def.name}?`}
        confirmLabel="Uninstall"
      >
        <p className="m-0">
          Its settings and stored secrets are deleted from {project.name}. Calls to its actions will fail
          {extras.webhookVerifyField ? ' and its webhook URL stops accepting events' : ''}. Records already in your collections stay.
        </p>
      </ConfirmDialog>
    </Page>
  );
}
