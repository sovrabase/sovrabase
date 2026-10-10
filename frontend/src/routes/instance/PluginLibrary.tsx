import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { Blocks, Boxes, ExternalLink, PackagePlus, Server } from 'lucide-react';
import { usePlugins } from '../../store';
import { Badge, Banner, Card, CardHeader, EmptyState, Page, PageHeader, Row, SearchInput, Segmented, Skeleton, cx } from '../../ui';
import { addsSummary, categoryLabel, loadCatalog, useInstallMap, PLUGIN_API_DOCS, type CatalogEntry } from './plugins/manifest';
import { PluginIcon } from './plugins/PluginBits';
import { ServerPlugins } from './plugins/ServerPlugins';

export default function PluginLibrary() {
  const [catalog, setCatalog] = useState<CatalogEntry[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [q, setQ] = useState('');
  const [cat, setCat] = useState('all');
  const { plugins, loading: pluginsLoading, loadPlugins } = usePlugins();
  const { map } = useInstallMap();

  useEffect(() => {
    loadPlugins();
    loadCatalog()
      .then(setCatalog)
      .catch((e) => {
        setError((e as Error).message);
        setCatalog([]);
      });
  }, [loadPlugins]);

  const categories = useMemo(() => [...new Set((catalog ?? []).map((c) => c.category))].sort(), [catalog]);
  const filtered = useMemo(() => {
    const term = q.trim().toLowerCase();
    return (catalog ?? []).filter((c) => {
      if (cat === 'installed' && !map?.[c.id]?.length) return false;
      if (cat !== 'all' && cat !== 'installed' && c.category !== cat) return false;
      if (!term) return true;
      return `${c.name} ${c.description} ${c.category} ${c.id}`.toLowerCase().includes(term);
    });
  }, [catalog, q, cat, map]);

  const serverCount = plugins?.plugins.length ?? 0;

  return (
    <Page>
      <PageHeader
        title="Plugins"
        description="Packages that bring settings, actions, triggers and routes. Browse them here, then install one in a project."
      />

      {/* Three sources */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 mb-8">
        <SourceCard href="#catalogue" icon={<Blocks className="w-4 h-4" />} label="Catalogue" title="Ready-made services" count={catalog?.length}>
          Payments, email, messaging, search. Configured from a form, no code.
        </SourceCard>
        <SourceCard href="#server" icon={<Server className="w-4 h-4" />} label="Server" title="Compiled into this binary" count={plugins ? serverCount : undefined}>
          Go plugins using the hooks API. Read-only here.
        </SourceCard>
        <SourceCard href="#project-made" icon={<Boxes className="w-4 h-4" />} label="Project-made" title="Your functions, packaged" soon>
          Publish a function with a settings schema so other projects can install it.
        </SourceCard>
      </div>

      {/* Catalogue */}
      <section id="catalogue" className="scroll-mt-20 mb-10">
        <div className="flex flex-wrap items-center justify-between gap-3 mb-3">
          <h2 className="m-0 text-base font-semibold text-ink">Catalogue</h2>
        </div>
        <div className="flex flex-wrap items-center gap-3 mb-4">
          <SearchInput value={q} onChange={setQ} placeholder="Search plugins" className="flex-[1_1_220px] max-w-[360px]" />
          <Segmented
            aria-label="Category"
            size="sm"
            value={cat}
            onChange={setCat}
            options={[
              { value: 'all', label: 'All' },
              ...categories.map((c) => ({ value: c, label: categoryLabel(c) })),
              { value: 'installed', label: 'Installed' },
            ]}
          />
        </div>
        {error && (
          <div className="mb-4">
            <Banner tone="danger" title="The catalogue could not be loaded">
              {error}
            </Banner>
          </div>
        )}
        {!catalog ? (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {[0, 1, 2, 3, 4, 5].map((i) => (
              <Skeleton key={i} className="h-[148px] rounded-lg" />
            ))}
          </div>
        ) : filtered.length === 0 ? (
          <Card>
            <EmptyState
              icon={<Blocks className="w-5 h-5" />}
              title={catalog.length ? 'No plugin matches' : 'The catalogue is empty'}
              description={catalog.length ? 'Try another word or category.' : undefined}
            />
          </Card>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {filtered.map((c) => (
              <CatalogCard key={c.id} def={c} installs={map?.[c.id]?.length ?? (map ? 0 : undefined)} />
            ))}
          </div>
        )}
      </section>

      {/* Server plugins */}
      <section id="server" className="scroll-mt-20 mb-10">
        <Card padded={false}>
          <CardHeader title="Server plugins" description="Compiled in. Change them in code and redeploy." />
          <div className="border-t border-line">
            {pluginsLoading && !plugins ? (
              <div className="p-5 flex flex-col gap-2">
                <Skeleton className="h-5 w-1/3" />
                <Skeleton className="h-5 w-1/2" />
              </div>
            ) : (
              <ServerPlugins info={plugins} />
            )}
          </div>
        </Card>
      </section>

      {/* Project-made */}
      <section id="project-made" className="scroll-mt-20">
        <Card padded={false}>
          <CardHeader title="Project-made plugins" actions={<Badge tone="accent">Coming soon</Badge>} />
          <Row className="items-start">
            <span className="w-10 h-10 shrink-0 rounded-[10px] bg-sunken text-ink-3 inline-flex items-center justify-center" aria-hidden>
              <PackagePlus className="w-5 h-5" />
            </span>
            <div className="flex-1 min-w-[220px] flex flex-col gap-1">
              <p className="m-0 text-sm text-ink-2">
                Packaging a project's function as an installable plugin is not available yet. Today you can extend Sovrabase with a Go
                plugin: register hooks and routes, compile it into the binary, and it shows up under Server plugins.
              </p>
              <a
                href={PLUGIN_API_DOCS}
                target="_blank"
                rel="noreferrer"
                className="self-start inline-flex items-center gap-1 text-[13px] font-medium text-accent hover:text-accent-2"
              >
                Build one with the Go plugin API <ExternalLink className="w-3.5 h-3.5" aria-hidden />
              </a>
            </div>
          </Row>
        </Card>
      </section>
    </Page>
  );
}

function SourceCard({
  href,
  icon,
  label,
  title,
  count,
  soon,
  children,
}: {
  href: string;
  icon: ReactNode;
  label: string;
  title: string;
  count?: number;
  soon?: boolean;
  children: ReactNode;
}) {
  return (
    <a
      href={href}
      className="group flex flex-col gap-1.5 p-4 rounded-lg border border-line bg-surface hover:border-line-2 hover:bg-hover transition-colors"
    >
      <span className="flex items-center gap-2 text-[12px] font-semibold uppercase tracking-[.06em] text-ink-3">
        <span aria-hidden>{icon}</span>
        {label}
        <span className="ml-auto normal-case tracking-normal">
          {soon ? <Badge tone="accent">Soon</Badge> : count !== undefined ? <Badge>{count}</Badge> : null}
        </span>
      </span>
      <span className="text-[15px] font-semibold text-ink">{title}</span>
      <span className="text-[13px] leading-[19px] text-ink-3">{children}</span>
    </a>
  );
}

function CatalogCard({ def, installs }: { def: CatalogEntry; installs?: number }) {
  const summary = addsSummary(def);
  return (
    <Link
      to={`/plugins/${encodeURIComponent(def.id)}`}
      className={cx(
        'flex flex-col gap-3 p-4 rounded-lg border bg-surface hover:bg-hover transition-colors',
        installs ? 'border-accent/40' : 'border-line hover:border-line-2',
      )}
    >
      <div className="flex items-start gap-3">
        <PluginIcon def={def} />
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-[15px] font-semibold text-ink truncate">{def.name}</span>
          </div>
          <span className="text-[12.5px] text-ink-3">{categoryLabel(def.category)}</span>
        </div>
        {installs ? <Badge tone="ok" dot>{`In ${installs} project${installs > 1 ? 's' : ''}`}</Badge> : null}
      </div>
      <p className="m-0 text-[13px] leading-[19px] text-ink-2 line-clamp-2">{def.description}</p>
      {summary.length > 0 && (
        <div className="flex flex-wrap gap-1.5 mt-auto">
          {summary.map((s) => (
            <Badge key={s}>{s}</Badge>
          ))}
        </div>
      )}
    </Link>
  );
}
