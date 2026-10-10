import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { Navigate, useNavigate, useParams } from 'react-router-dom';
import { api } from '../../api';
import { useAuth } from '../../store';
import { Banner, Button, Page, PageHeader, Spinner, Tabs } from '../../ui';
import { AuditSection } from './settings/AuditSection';
import { BackupsSection } from './settings/BackupsSection';
import { EmailLogSection } from './settings/EmailLogSection';
import { EmailSection } from './settings/EmailSection';
import { GeneralSection } from './settings/GeneralSection';
import { ReplicationSection } from './settings/ReplicationSection';
import { SecuritySection } from './settings/SecuritySection';
import { StorageSection } from './settings/StorageSection';
import { MASK, SECRET_KEYS, type Config } from './settings/shared';

const SECTIONS = [
  { value: 'general', label: 'General' },
  { value: 'security', label: 'Security' },
  { value: 'email', label: 'Email' },
  { value: 'storage', label: 'File storage' },
  { value: 'replication', label: 'Replication' },
  { value: 'backups', label: 'Backups' },
  { value: 'audit', label: 'Audit log' },
  { value: 'email-log', label: 'Email log' },
] as const;

type SectionKey = (typeof SECTIONS)[number]['value'];
/** Sections that edit /admin/config and need it loaded. */
const CONFIG_SECTIONS: SectionKey[] = ['general', 'security', 'email', 'storage', 'replication', 'backups'];

type Stats = Record<string, unknown> & { replication?: { enabled?: boolean; role?: string; node_id?: string; peers?: number } };

export default function InstanceSettings() {
  const { section } = useParams();
  const navigate = useNavigate();
  const { role } = useAuth();
  const canWrite = role !== 'support';

  const [config, setConfig] = useState<Config | null>(null);
  const [configError, setConfigError] = useState<string | null>(null);
  const [stats, setStats] = useState<Stats | null>(null);

  const loadConfig = useCallback(async () => {
    setConfigError(null);
    try {
      setConfig(await api<Config>('/admin/config'));
    } catch (e) {
      setConfigError((e as Error).message);
    }
  }, []);

  useEffect(() => {
    loadConfig();
    api<Stats>('/admin/stats')
      .then(setStats)
      .catch(() => setStats(null));
  }, [loadConfig]);

  const onSaved = useCallback((body: Record<string, unknown>) => {
    setConfig((c) => {
      const next: Config = { ...(c ?? {}) };
      for (const [k, v] of Object.entries(body)) {
        if (v === undefined) continue;
        next[k] = SECRET_KEYS.includes(k) ? (v ? MASK : next[k]) : v;
      }
      return next;
    });
  }, []);

  const current = SECTIONS.find((s) => s.value === section)?.value;
  if (section && !current) return <Navigate to="/settings" replace />;
  const active: SectionKey = current ?? 'general';

  const repl = stats?.replication;
  const meta = [stats?.version ? `v${String(stats.version)}` : null, repl?.node_id ? `node ${repl.node_id}` : null, repl?.enabled && repl.role ? repl.role : null]
    .filter(Boolean)
    .join(' · ');

  const needsConfig = CONFIG_SECTIONS.includes(active);

  let body: ReactNode = null;
  if (needsConfig && !config) {
    body = configError ? (
      <div className="py-7">
        <Banner tone="danger" title="Could not load the server settings" action={<Button size="sm" onClick={loadConfig}>Retry</Button>}>
          {configError}
        </Banner>
      </div>
    ) : (
      <Spinner />
    );
  } else if (config) {
    const props = { config, onSaved };
    switch (active) {
      case 'general':
        body = <GeneralSection {...props} stats={stats} />;
        break;
      case 'security':
        body = <SecuritySection {...props} />;
        break;
      case 'email':
        body = <EmailSection {...props} />;
        break;
      case 'storage':
        body = <StorageSection {...props} stats={stats} />;
        break;
      case 'replication':
        body = <ReplicationSection {...props} live={repl ?? null} />;
        break;
      case 'backups':
        body = <BackupsSection {...props} canWrite={canWrite} />;
        break;
    }
  }
  if (active === 'audit') body = <AuditSection canClear={canWrite} />;
  if (active === 'email-log') body = <EmailLogSection />;

  return (
    <Page>
      <PageHeader
        title="Instance settings"
        description="Server-wide settings that apply to every project on this instance."
        meta={meta ? <span className="text-[13px] text-ink-3">{meta}</span> : undefined}
      />
      <Tabs<SectionKey>
        tabs={SECTIONS.map((s) => ({ value: s.value, label: s.label }))}
        value={active}
        onChange={(v) => navigate(v === 'general' ? '/settings' : `/settings/${v}`)}
      />
      {/* key: each section starts from the saved values when you switch */}
      <div key={active}>{body}</div>
    </Page>
  );
}
