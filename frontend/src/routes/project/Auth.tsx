import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { Plus } from 'lucide-react';
import { api, adminProjectPath } from '../../api';
import type { OAuthProvider } from '../../types';
import { useProject } from '../../shell/ProjectContext';
import { projectPath } from '../../shell/nav';
import { Banner, Button, Page, PageHeader, Spinner, Tabs } from '../../ui';
import { UsersPanel } from './auth/UsersPanel';
import { ProvidersPanel } from './auth/ProvidersPanel';
import { EmailsPanel } from './auth/EmailsPanel';
import { SecurityPanel } from './auth/SecurityPanel';
import { userMethods, type UserRow } from './auth/types';

type Section = 'users' | 'sign-in' | 'emails' | 'security';
const SECTIONS: Section[] = ['users', 'sign-in', 'emails', 'security'];

const DESCRIPTIONS: Record<Section, string> = {
  users: 'The people who have an account in your app.',
  'sign-in': 'Choose how people sign in to your app.',
  emails: 'Edit the emails Auth sends: verification, reset and magic links.',
  security: 'Account protections that apply to every user.',
};

export default function ProjectAuth() {
  const { projectId } = useProject();
  const { section: raw } = useParams<{ section?: string }>();
  const navigate = useNavigate();
  const section: Section = SECTIONS.includes(raw as Section) ? (raw as Section) : 'users';

  const [users, setUsers] = useState<UserRow[]>([]);
  const [providers, setProviders] = useState<OAuthProvider[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [createUser, setCreateUser] = useState(false);

  const loadUsers = useCallback(async () => {
    const d = await api<UserRow[]>(adminProjectPath(projectId, '/users'));
    setUsers(d || []);
  }, [projectId]);
  const loadProviders = useCallback(async () => {
    const d = await api<{ providers: OAuthProvider[] }>(adminProjectPath(projectId, '/auth/providers'));
    setProviders(d.providers || []);
  }, [projectId]);

  useEffect(() => {
    setLoading(true);
    Promise.allSettled([loadUsers(), loadProviders()])
      .then((r) => {
        const failed = r.find((x) => x.status === 'rejected') as PromiseRejectedResult | undefined;
        setError(failed ? (failed.reason as Error).message : null);
      })
      .finally(() => setLoading(false));
  }, [loadUsers, loadProviders]);

  const usersByMethod = useMemo(() => {
    const m: Record<string, number> = {};
    for (const u of users) for (const k of userMethods(u)) m[k] = (m[k] ?? 0) + 1;
    return m;
  }, [users]);

  const go = (s: Section) => navigate(projectPath(projectId, s === 'users' ? 'auth' : `auth/${s}`));

  return (
    <Page>
      <PageHeader
        title="Auth"
        description={DESCRIPTIONS[section]}
        actions={
          section === 'users' ? (
            <Button variant="primary" icon={<Plus className="w-4 h-4" />} onClick={() => setCreateUser(true)}>
              Add user
            </Button>
          ) : undefined
        }
      />
      <Tabs
        className="mb-5"
        value={section}
        onChange={go}
        tabs={[
          { value: 'users', label: 'Users', count: loading ? undefined : users.length },
          { value: 'sign-in', label: 'Sign-in methods' },
          { value: 'emails', label: 'Emails' },
          { value: 'security', label: 'Security' },
        ]}
      />
      {error && (
        <div className="mb-4">
          <Banner tone="danger" title="Some auth data could not be loaded">
            {error}
          </Banner>
        </div>
      )}
      {section === 'emails' ? (
        <EmailsPanel projectId={projectId} />
      ) : loading ? (
        <Spinner label="Loading auth" />
      ) : section === 'users' ? (
        <UsersPanel projectId={projectId} users={users} setUsers={setUsers} reload={loadUsers} createOpen={createUser} setCreateOpen={setCreateUser} />
      ) : section === 'sign-in' ? (
        <ProvidersPanel projectId={projectId} providers={providers} usersByMethod={usersByMethod} reload={loadProviders} />
      ) : (
        <SecurityPanel users={users} />
      )}
    </Page>
  );
}
