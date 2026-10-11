import { useEffect, useState } from 'react';
import { Button, Field, Input, Select, Sheet } from '../../../ui';

export const ADMIN_ROLES = [
  { value: 'super_admin', label: 'Super admin', description: 'Everything, including managing other admins and members.' },
  { value: 'admin', label: 'Admin', description: 'Projects and instance settings. Cannot manage admins.' },
  { value: 'support', label: 'Support', description: 'Read-only access to projects, stats and the audit log.' },
];

export function roleLabel(role?: string) {
  return ADMIN_ROLES.find((r) => r.value === role)?.label ?? (role ? role.replace(/_/g, ' ') : 'Admin');
}

export interface NewAdmin {
  email: string;
  password: string;
  role: string;
  name: string;
}

/** Add a console admin. */
export function AddAdminSheet({ open, onClose, onSubmit }: { open: boolean; onClose: () => void; onSubmit: (a: NewAdmin) => Promise<void> }) {
  const [form, setForm] = useState<NewAdmin>({ email: '', password: '', role: 'admin', name: '' });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    if (open) {
      setForm({ email: '', password: '', role: 'admin', name: '' });
      setError(null);
    }
  }, [open]);

  const tooShort = form.password.length > 0 && form.password.length < 8;
  const valid = /\S+@\S+\.\S+/.test(form.email) && form.password.length >= 8;

  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      await onSubmit(form);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Sheet
      open={open}
      onClose={onClose}
      title="Add admin"
      description="Admins sign in to this console. Project roles are set inside each project."
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" loading={busy} disabled={!valid} onClick={submit}>
            Add admin
          </Button>
        </>
      }
    >
      <Field label="Email">
        <Input type="email" autoComplete="off" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} placeholder="name@example.com" />
      </Field>
      <Field label="Name" optional>
        <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
      </Field>
      <Field label="Password" hint="At least 8 characters. Share it privately; they can change it in Account." error={tooShort ? 'Use at least 8 characters.' : undefined}>
        <Input type="password" autoComplete="new-password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} />
      </Field>
      <Field label="Instance role" hint={ADMIN_ROLES.find((r) => r.value === form.role)?.description}>
        <Select value={form.role} onChange={(e) => setForm({ ...form, role: e.target.value })} options={ADMIN_ROLES} />
      </Field>
      {error && <p className="m-0 text-[13px] text-danger" role="alert">{error}</p>}
    </Sheet>
  );
}
