import { useState, useEffect } from 'react';
import { User, Mail, Loader2, Save, Shield } from 'lucide-react';
import { api } from '../api';
import { useToast } from '../components/Toast';

interface MyProfile {
  user_id: string;
  email: string;
  name?: string;
  role: string;
  admin_role?: string;
  project_count?: number;
}

export default function Profile() {
  const { showToast } = useToast();
  const [profile, setProfile] = useState<MyProfile | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [name, setName] = useState('');
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');

  const loadProfile = async () => {
    setLoading(true);
    try {
      const data = await api<MyProfile>('/admin/members/me');
      setProfile(data);
      setName(data.name || '');
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { loadProfile(); }, []);

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    if (newPassword && newPassword !== confirmPassword) {
      showToast('Passwords do not match', 'error');
      return;
    }
    if (newPassword && newPassword.length < 8) {
      showToast('Password must be at least 8 characters', 'error');
      return;
    }
    if (newPassword && !currentPassword) {
      showToast('Current password is required to set a new password', 'error');
      return;
    }

    setSaving(true);
    try {
      await api('/admin/members/me', {
        method: 'PUT',
        body: JSON.stringify({
          name: name || undefined,
          current_password: currentPassword || undefined,
          new_password: newPassword || undefined,
        }),
      });
      showToast('Profile updated', 'success');
      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
      loadProfile();
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-24">
        <Loader2 className="w-8 h-8 text-accent animate-spin" />
      </div>
    );
  }

  if (!profile) {
    return (
      <div className="flex flex-col items-center justify-center py-24 text-text-muted">
        <p>Failed to load profile</p>
      </div>
    );
  }

  const inputCls = 'w-full px-3 py-2 rounded-lg bg-bg-input border border-border text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent transition-colors text-sm';
  const labelCls = 'text-text-secondary text-sm font-medium';

  return (
    <div className="max-w-lg mx-auto space-y-6 py-8">
      <h1 className="text-2xl font-bold text-text-primary">My Profile</h1>

      {/* Account Info */}
      <div className="bg-bg-card border border-border rounded-xl p-6 space-y-4">
        <h2 className="flex items-center gap-2 text-lg font-semibold text-text-primary">
          <User className="w-5 h-5 text-accent" />
          Account
        </h2>

        <div className="flex items-center gap-2 text-sm">
          <Mail className="w-4 h-4 text-text-muted" />
          <span className="text-text-secondary">Email:</span>
          <span className="text-text-primary font-medium">{profile.email}</span>
        </div>

        <div className="flex items-center gap-2 text-sm">
          <Shield className="w-4 h-4 text-text-muted" />
          <span className="text-text-secondary">Role:</span>
          <span className="inline-flex items-center gap-1 px-2 py-0.5 bg-accent/10 text-accent text-xs font-medium rounded-full">
            {profile.admin_role === 'super_admin' ? 'Super Admin' : profile.role === 'admin' ? 'Admin' : 'Member'}
          </span>
        </div>

        {profile.project_count !== undefined && (
          <div className="flex items-center gap-2 text-sm">
            <span className="text-text-secondary">Projects:</span>
            <span className="text-text-primary font-medium">{profile.project_count}</span>
          </div>
        )}

        <div className="flex items-center gap-2 text-sm">
          <span className="text-text-secondary">User ID:</span>
          <code className="text-text-muted text-xs font-mono">{profile.user_id}</code>
        </div>
      </div>

      {/* Edit Form */}
      <form onSubmit={handleSave} className="bg-bg-card border border-border rounded-xl p-6 space-y-4">
        <h2 className="flex items-center gap-2 text-lg font-semibold text-text-primary">
          <Save className="w-5 h-5 text-accent" />
          Edit Profile
        </h2>

        <div>
          <label className={`${labelCls} block mb-1`}>Display Name</label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Your name"
            className={inputCls}
          />
        </div>

        <hr className="border-border" />

        <div>
          <label className={`${labelCls} block mb-1`}>Current Password</label>
          <input
            type="password"
            value={currentPassword}
            onChange={(e) => setCurrentPassword(e.target.value)}
            placeholder="Required to change password"
            className={inputCls}
            autoComplete="current-password"
          />
        </div>

        <div>
          <label className={`${labelCls} block mb-1`}>New Password</label>
          <input
            type="password"
            value={newPassword}
            onChange={(e) => setNewPassword(e.target.value)}
            placeholder="Leave blank to keep current"
            className={inputCls}
            autoComplete="new-password"
          />
        </div>

        <div>
          <label className={`${labelCls} block mb-1`}>Confirm New Password</label>
          <input
            type="password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            placeholder="Repeat new password"
            className={inputCls}
            autoComplete="new-password"
          />
        </div>

        <div className="flex justify-end pt-2">
          <button
            type="submit"
            disabled={saving}
            className="flex items-center gap-2 px-5 py-2.5 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover transition-colors disabled:opacity-50"
          >
            {saving ? <Loader2 className="w-4 h-4 animate-spin" /> : <Save className="w-4 h-4" />}
            {saving ? 'Saving...' : 'Save Changes'}
          </button>
        </div>
      </form>
    </div>
  );
}
