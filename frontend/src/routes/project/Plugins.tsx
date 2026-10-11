import { useCallback, useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { Blocks, ChevronRight, ExternalLink, PackagePlus } from 'lucide-react';
import { useProject } from '../../shell/ProjectContext';
import { projectPath } from '../../shell/nav';
import { Badge, Banner, Button, ButtonLink, Card, CardHeader, EmptyState, Page, PageHeader, Row, Skeleton } from '../../ui';
import {
  addsSummary,
  categoryLabel,
  loadCatalog,
  loadProjectIntegrations,
  missingRequired,
  PLUGIN_API_DOCS,
  type CatalogEntry,
  type ProjectIntegration,
} from '../instance/plugins/manifest';
import { PluginIcon } from '../instance/plugins/PluginBits';
import { PluginConfigPage } from './plugins/PluginConfigPage';

export default function ProjectPlugins() {
  const { projectId } = useProject();
  const { provider } = useParams<{ provider?: string }>();
  const [catalog, setCatalog] = useState<CatalogEntry[] | null>(null);
  const [installed, setInstalled] = useState<ProjectIntegration[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const [cat, ints] = await Promise.all([loadCatalog(), loadProjectIntegrations(projectId)]);
      setCatalog(cat);
      setInstalled(ints);
      setError(null);
    } catch (e) {
      setError((e as Error).message);
      setCatalog((c) => c ?? []);
      setInstalled((i) => i ?? []);
    }
  }, [projectId]);

  useEffect(() => {
    load();
  }, [load]);

  if (provider) {
    return (
      <PluginConfigPage
        provider={provider}
        catalog={catalog}
        installed={installed}
        error={error}
        reload={load}
      />
    );
  }

  return <PluginList catalog={catalog} installed={installed} error={error} />;
}

function statusOf(def: CatalogEntry, cur: ProjectIntegration) {
  const missing = missingRequired(def, cur);
  return missing.length ? (
    <Badge tone="warn" dot>
      Needs setup
    </Badge>
  ) : (
    <Badge tone="ok" dot>
      Active
    </Badge>
  );
}

function PluginList({
  catalog,
  installed,
  error,
}: {
  catalog: CatalogEntry[] | null;
  installed: ProjectIntegration[] | null;
  error: string | null;
}) {
  const { projectId } = useProject();
  const navigate = useNavigate();
  const loading = !catalog || !installed;
  const mine = (installed ?? []).map((i) => ({ cur: i, def: catalog?.find((c) => c.id === i.id) })).filter((x) => x.def);
  const available = (catalog ?? []).filter((c) => !installed?.some((i) => i.id === c.id));
  const open = (id: string) => navigate(projectPath(projectId, `plugins/${id}`));

  return (
    <Page>
      <PageHeader
        title="Plugins"
        description="Services this project uses: payments, email, messaging, search. Install, configure and remove them here."
        actions={
          <ButtonLink to="/plugins" icon={<Blocks className="w-4 h-4" />}>
            Browse library
          </ButtonLink>
        }
      />
      {error && (
        <div className="mb-5">
          <Banner tone="danger" title="Plugins could not be loaded">
            {error}
          </Banner>
        </div>
      )}

      <Card padded={false} className="mb-8">
        <CardHeader title="Installed" description="Open one to change its settings or see what it may touch." />
        {loading ? (
          <div className="px-5 pb-5 flex flex-col gap-2">
            <Skeleton className="h-10" />
            <Skeleton className="h-10" />
          </div>
        ) : mine.length === 0 ? (
          <div className="border-t border-line">
            <EmptyState
              icon={<Blocks className="w-5 h-5" />}
              title="No plugins installed"
              description="Pick one below. You'll see what it can do before anything is saved."
            />
          </div>
        ) : (
          <ul className="m-0 p-0 list-none border-t border-line">
            {mine.map(({ cur, def }) => (
              <li key={cur.id} className="border-b border-line last:border-b-0">
                <button
                  type="button"
                  onClick={() => open(cur.id)}
                  className="w-full text-left flex items-center gap-3 px-5 py-3 hover:bg-hover"
                >
                  <PluginIcon def={def!} size="sm" />
                  <span className="flex-1 min-w-0 flex flex-col">
                    <span className="text-sm font-medium text-ink truncate">{def!.name}</span>
                    <span className="text-[12.5px] text-ink-3">{categoryLabel(def!.category)}</span>
                  </span>
                  {statusOf(def!, cur)}
                  <ChevronRight className="w-4 h-4 text-ink-3" aria-hidden />
                </button>
              </li>
            ))}
          </ul>
        )}
      </Card>

      <h2 className="m-0 mb-3 text-base font-semibold text-ink">Available</h2>
      {loading ? (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-[132px] rounded-lg" />
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
          {available.map((def) => (
            <div key={def.id} className="flex flex-col gap-3 p-4 rounded-lg border border-line bg-surface">
              <div className="flex items-start gap-3">
                <PluginIcon def={def} />
                <div className="min-w-0 flex-1">
                  <span className="block text-[15px] font-semibold text-ink truncate">{def.name}</span>
                  <span className="text-[12.5px] text-ink-3">{categoryLabel(def.category)}</span>
                </div>
              </div>
              <p className="m-0 text-[13px] leading-[19px] text-ink-2 line-clamp-2">{def.description}</p>
              <div className="flex flex-wrap items-center gap-1.5 mt-auto">
                {addsSummary(def).map((s) => (
                  <Badge key={s}>{s}</Badge>
                ))}
                <Button size="sm" className="ml-auto" onClick={() => open(def.id)} aria-label={`Install ${def.name}`}>
                  Install
                </Button>
              </div>
            </div>
          ))}
          <div className="flex flex-col gap-2 p-4 rounded-lg border border-dashed border-line-2">
            <span className="flex items-center gap-2 text-[15px] font-semibold text-ink">
              <PackagePlus className="w-4 h-4 text-ink-3" aria-hidden /> Project-made
              <Badge tone="accent">Coming soon</Badge>
            </span>
            <p className="m-0 text-[13px] leading-[19px] text-ink-3">
              Packaging your own functions as plugins is not available yet. Go plugins compiled into the server work today.
            </p>
            <a
              href={PLUGIN_API_DOCS}
              target="_blank"
              rel="noreferrer"
              className="mt-auto self-start inline-flex items-center gap-1 text-[13px] font-medium text-accent hover:text-accent-2"
            >
              Go plugin API <ExternalLink className="w-3.5 h-3.5" aria-hidden />
            </a>
          </div>
        </div>
      )}
      {!loading && available.length === 0 && mine.length > 0 && (
        <Row className="px-0 border-t-0">
          <span className="text-sm text-ink-3">Every catalogue plugin is installed in this project.</span>
        </Row>
      )}
    </Page>
  );
}
