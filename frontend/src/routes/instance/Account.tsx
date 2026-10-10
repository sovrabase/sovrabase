import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { LogOut } from 'lucide-react';
import { api } from '../../api';
import { useAuth } from '../../store';
import { getDensity, getTheme, setDensity, setTheme, type Density, type ThemePref } from '../../shell/theme';
import {
  Badge,
  Banner,
  Button,
  Card,
  CopyButton,
  Field,
  Input,
  Page,
  PageHeader,
  SaveBar,
  Section,
  Segmented,
  Spinner,
  useDraft,
  useToast,
} from '../../ui';
import { EmailSheet, PasswordSheet } from './account/Sheets';
import { roleLabel } from './members/AdminSheet';

interface MyProfile {
  user_id: string;
  email: string;
  name?: string;
  role: string;
  admin_role?: string;
  project_count?: number;
}

interface AccountConfig {
  admin_email_locked?: boolean;
  admin_password_locked?: boolean;
}

function initials(s: string) {
  return (
    s
      .split(/[\s@._-]+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((w) => w[0]!.toUpperCase())
      .join('') || '?'
  );
}

export default function Account() {
  const { showToast } = useToast();
  const { logout } = useAuth();
  const navigate = useNavigate();

  const [profile, setProfile] = useState<MyProfile | null>(null);
  const [config, setConfig] = useState<AccountConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [pwOpen, setPwOpen] = useState(false);
  const [emailOpen, setEmailOpen] = useState(false);

  const [theme, setThemeState] = useState<ThemePref>(getTheme);
  const [density, setDensityState] = useState<Density>(getDensity);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [p, c] = await Promise.all([
        api<MyProfile>('/admin/members/me'),
        // Only admins can read the server config; members get null here.
        api<AccountConfig>('/admin/config').catch(() => null),
      ]);
      setProfile(p);
      setConfig(c);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const saved = useMemo(() => (profile ? { name: profile.name ?? '' } : null), [profile]);
  const { draft, patch, dirty, reset } = useDraft(saved);

  const saveProfile = async () => {
    if (!draft) return;
    setSaving(true);
    try {
      await api('/admin/members/me', { method: 'PUT', body: JSON.stringify({ name: draft.name.trim() || undefined }) });
      setProfile((p) => (p ? { ...p, name: draft.name.trim() } : p));
      showToast('Profile saved', 'success');
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setSaving(false);
    }
  };

  const changePassword = async (current: string, next: string) => {
    await api('/admin/members/me', {
      method: 'PUT',
      body: JSON.stringify({ current_password: current, new_password: next }),
    });
    showToast('Password changed', 'success');
    setPwOpen(false);
  };

  const changeEmail = async (email: string, currentPassword: string) => {
    await api('/admin/config', {
      method: 'POST',
      body: JSON.stringify({ admin_email: email, admin_current_password: currentPassword }),
    });
    localStorage.setItem('sovrabase_admin_email', email);
    showToast('Email changed', 'success');
    setEmailOpen(false);
    load();
  };

  const signOut = () => {
    logout();
    navigate('/');
  };

  if (loading && !profile) {
    return (
      <Page>
        <PageHeader title="Your account" description="Your profile, password and how the console looks for you." />
        <Spinner />
      </Page>
    );
  }

  if (!profile) {
    return (
      <Page>
        <PageHeader title="Your account" description="Your profile, password and how the console looks for you." />
        <Banner tone="danger" title="Could not load your account" action={<Button size="sm" onClick={load}>Retry</Button>}>
          {error}
        </Banner>
      </Page>
    );
  }

  const isAdmin = profile.role !== 'member';
  const emailLocked = !!config?.admin_email_locked;
  const pwLocked = !!config?.admin_password_locked && isAdmin;
  const canChangeEmail = !!config && isAdmin && !emailLocked;
  const displayRole = profile.admin_role ? roleLabel(profile.admin_role) : isAdmin ? 'Admin' : 'Member';

  return (
    <Page>
      <PageHeader
        title="Your account"
        description="Your profile, password and how the console looks for you."
        actions={
          <Button icon={<LogOut className="w-4 h-4" />} onClick={signOut}>
            Sign out
          </Button>
        }
      />

      <Card className="!py-0">
        <Section title="Profile" description="Shown to your team in member lists and the audit log.">
          <div className="flex items-center gap-3">
            <span
              className="inline-flex items-center justify-center w-12 h-12 rounded-full bg-accent-soft text-accent-2 text-base font-semibold"
              aria-hidden
            >
              {initials(draft?.name || profile.email)}
            </span>
            <div className="flex flex-col min-w-0">
              <span className="text-sm font-medium text-ink truncate">{profile.name || profile.email}</span>
              <span className="flex flex-wrap items-center gap-2 text-[12.5px] text-ink-3">
                <Badge tone={profile.admin_role === 'super_admin' ? 'accent' : 'neutral'}>{displayRole}</Badge>
                {profile.project_count !== undefined && (
                  <span>
                    {profile.project_count} {profile.project_count === 1 ? 'project' : 'projects'}
                  </span>
                )}
              </span>
            </div>
          </div>
          <Field label="Display name" optional>
            <Input value={draft?.name ?? ''} onChange={(e) => patch({ name: e.target.value })} placeholder="Your name" autoComplete="name" />
          </Field>
          <Field
            label="Email"
            env={emailLocked && isAdmin ? 'SOVRABASE_ADMIN_EMAIL' : undefined}
            aside={
              canChangeEmail ? (
                <button type="button" className="text-[13px] font-medium text-accent hover:text-accent-2" onClick={() => setEmailOpen(true)}>
                  Change
                </button>
              ) : undefined
            }
          >
            <Input value={profile.email} readOnly />
          </Field>
          <Field label="User ID" hint="Use it when asking for support or reading the audit log.">
            <Input mono value={profile.user_id} readOnly suffix={<CopyButton text={profile.user_id} />} />
          </Field>
        </Section>

        <Section title="Sign-in and security" description="Your console password.">
          {pwLocked ? (
            <Field label="Password" env="SOVRABASE_ADMIN_PASSWORD">
              <Input value="••••••••" readOnly />
            </Field>
          ) : (
            <div className="flex flex-wrap items-center gap-3">
              <div className="flex flex-col flex-1 min-w-[200px]">
                <span className="text-sm font-medium text-ink">Password</span>
                <span className="text-[12.5px] text-ink-3">Use at least 8 characters. You need your current password to change it.</span>
              </div>
              <Button onClick={() => setPwOpen(true)}>Change password</Button>
            </div>
          )}
        </Section>

        <Section title="Preferences" description="Saved in this browser only." last>
          <Field label="Theme">
            <Segmented<ThemePref>
              aria-label="Theme"
              value={theme}
              onChange={(t) => {
                setTheme(t);
                setThemeState(t);
              }}
              options={[
                { value: 'system', label: 'System' },
                { value: 'light', label: 'Light' },
                { value: 'dark', label: 'Dark' },
              ]}
            />
          </Field>
          <Field label="Density" hint="Compact tightens tables and logs.">
            <Segmented<Density>
              aria-label="Density"
              value={density}
              onChange={(d) => {
                setDensity(d);
                setDensityState(d);
              }}
              options={[
                { value: 'comfortable', label: 'Comfortable' },
                { value: 'compact', label: 'Compact' },
              ]}
            />
          </Field>
        </Section>
      </Card>

      <SaveBar dirty={dirty} saving={saving} onSave={saveProfile} onDiscard={reset} />

      <PasswordSheet open={pwOpen} onClose={() => setPwOpen(false)} onSubmit={changePassword} />
      <EmailSheet open={emailOpen} onClose={() => setEmailOpen(false)} currentEmail={profile.email} onSubmit={changeEmail} />
    </Page>
  );
}
