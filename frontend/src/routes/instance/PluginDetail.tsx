import { useEffect, useMemo, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { ArrowRight, Blocks, ChevronRight } from 'lucide-react';
import { usePlugins } from '../../store';
import { Badge, Banner, Button, ButtonLink, Card, CardHeader, EmptyState, Field, Page, PageHeader, Row, Select, Spinner } from '../../ui';
import { projectPath } from '../../shell/nav';
import {
  addsOf,
  categoryLabel,
  fieldTypeLabel,
  loadCatalog,
  permissionsOf,
  useInstallMap,
  type CatalogEntry,
} from './plugins/manifest';
import { AddsList, PermissionList, PluginIcon } from './plugins/PluginBits';
import { ServerPlugins } from './plugins/ServerPlugins';

function Crumbs({ name }: { name: string }) {
  return (
    <nav aria-label="Breadcrumb" className="flex items-center gap-1.5 text-[13px] text-ink-3 mb-3">
      <Link to="/plugins" className="hover:text-ink">
        Plugins
      </Link>
      <ChevronRight className="w-3.5 h-3.5" aria-hidden />
      <span className="text-ink-2 truncate">{name}</span>
    </nav>
  );
}

export default function PluginDetail() {
  const { pluginId = '' } = useParams<{ pluginId: string }>();
  const [catalog, setCatalog] = useState<CatalogEntry[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadCatalog()
      .then(setCatalog)
      .catch((e) => {
        setError((e as Error).message);
        setCatalog([]);
      });
  }, []);

  if (pluginId.startsWith('server:')) return <ServerPluginDetail name={pluginId.slice(7)} />;
  if (!catalog) return <Spinner label="Loading plugin" />;
  const def = catalog.find((c) => c.id === pluginId);
  if (!def) {
    return (
      <Page>
        <Crumbs name={pluginId} />
        {error && <Banner tone="danger" title="The catalogue could not be loaded">{error}</Banner>}
        <EmptyState
          icon={<Blocks className="w-5 h-5" />}
          title="This plugin is not in the catalogue"
          description="It may have been removed from this build."
          action={<ButtonLink to="/plugins">All plugins</ButtonLink>}
        />
      </Page>
    );
  }
  return <CatalogDetail def={def} />;
}

function CatalogDetail({ def }: { def: CatalogEntry }) {
  const navigate = useNavigate();
  const { map, projects } = useInstallMap();
  const installedIn = map?.[def.id] ?? [];
  const [target, setTarget] = useState('');
  const adds = useMemo(() => addsOf(def), [def]);
  const perms = useMemo(() => permissionsOf(def), [def]);
  const projectName = (id: string) => projects.find((p) => p.id === id)?.name ?? id;

  const options = projects.map((p) => ({
    value: p.id,
    label: installedIn.includes(p.id) ? `${p.name} (installed)` : p.name,
  }));
  const already = !!target && installedIn.includes(target);

  return (
    <Page>
      <Crumbs name={def.name} />
      <div className="flex items-start gap-4">
        <PluginIcon def={def} size="lg" />
        <div className="flex-1 min-w-0">
          <PageHeader
            title={def.name}
            meta={<Badge>Catalogue · {categoryLabel(def.category)}</Badge>}
            description={def.description}
          />
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-[minmax(0,1fr)_380px] gap-5 items-start">
        <div className="flex flex-col gap-5 min-w-0">
          <Card>
            <h2 className="m-0 mb-4 text-[15px] font-semibold text-ink">What it adds to a project</h2>
            <AddsList adds={adds} />
          </Card>

          <Card padded={false}>
            <CardHeader title="Settings" description="What you fill in when you install it." />
            {def.config_fields.map((f) => (
              <Row key={f.key}>
                <span className="flex-1 min-w-0 flex flex-col">
                  <span className="text-sm text-ink">{f.label}</span>
                  {f.help_text && <span className="text-[12.5px] text-ink-3">{f.help_text}</span>}
                </span>
                <span className="text-[12.5px] text-ink-3">{fieldTypeLabel(f.type)}</span>
                {f.required ? <Badge>Required</Badge> : <Badge className="!bg-transparent !text-ink-3">Optional</Badge>}
              </Row>
            ))}
          </Card>

          <Card padded={false}>
            <CardHeader title="Installed in" />
            {!map ? (
              <Row>
                <span className="text-sm text-ink-3">Checking projects…</span>
              </Row>
            ) : installedIn.length === 0 ? (
              <Row>
                <span className="text-sm text-ink-3">Not installed in any project yet.</span>
              </Row>
            ) : (
              installedIn.map((pid) => (
                <Row key={pid}>
                  <span className="flex-1 min-w-0 text-sm font-medium text-ink truncate">{projectName(pid)}</span>
                  <ButtonLink size="sm" variant="ghost" to={projectPath(pid, `plugins/${def.id}`)}>
                    Open
                  </ButtonLink>
                </Row>
              ))
            )}
          </Card>
        </div>

        <Card className="lg:sticky lg:top-4 flex flex-col gap-5">
          <h2 className="m-0 text-[15px] font-semibold text-ink">Install in a project</h2>
          <Field label="Project">
            <Select value={target} onChange={(e) => setTarget(e.target.value)} options={options} placeholder="Choose a project" />
          </Field>
          <div className="flex flex-col gap-3">
            <h3 className="m-0 text-[13px] font-semibold text-ink">{def.name} will be able to</h3>
            <PermissionList items={perms} />
          </div>
          <Button
            variant="primary"
            disabled={!target}
            icon={<ArrowRight className="w-4 h-4" />}
            onClick={() => navigate(projectPath(target, `plugins/${def.id}`), { state: { approved: !already } })}
          >
            {already ? 'Open its settings' : 'Approve and configure'}
          </Button>
          <p className="m-0 -mt-2 text-[12.5px] text-ink-3">Nothing is saved until you fill in the settings and install it.</p>
        </Card>
      </div>
    </Page>
  );
}

function ServerPluginDetail({ name }: { name: string }) {
  const { plugins, loading, loadPlugins } = usePlugins();
  useEffect(() => {
    loadPlugins();
  }, [loadPlugins]);
  if (loading && !plugins) return <Spinner label="Loading plugin" />;
  const known = plugins?.plugins.includes(name);
  return (
    <Page>
      <Crumbs name={name} />
      <PageHeader
        title={<span className="font-mono">{name}</span>}
        meta={<Badge>Server · Go</Badge>}
        description="Compiled into this binary. It applies to every project; change it in code and redeploy."
      />
      {!known ? (
        <EmptyState
          icon={<Blocks className="w-5 h-5" />}
          title="This server plugin is not registered"
          action={<ButtonLink to="/plugins">All plugins</ButtonLink>}
        />
      ) : (
        <Card padded={false}>
          <CardHeader
            title="Hooks and routes on this server"
            description="The server reports hooks and routes for all Go plugins together, not per plugin."
          />
          <div className="border-t border-line">
            <ServerPlugins info={plugins} showNames={false} />
          </div>
        </Card>
      )}
    </Page>
  );
}
