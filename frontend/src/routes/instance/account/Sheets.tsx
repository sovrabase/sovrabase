import { useEffect, useState } from 'react';
import { Button, Field, Input, Sheet, cx } from '../../../ui';

function strength(pw: string): { score: number; label: string } {
  if (!pw) return { score: 0, label: '' };
  let s = 0;
  if (pw.length >= 8) s++;
  if (pw.length >= 12) s++;
  if (/[a-z]/.test(pw) && /[A-Z]/.test(pw)) s++;
  if (/\d/.test(pw)) s++;
  if (/[^A-Za-z0-9]/.test(pw)) s++;
  const score = Math.min(4, Math.max(1, s - (pw.length < 8 ? 1 : 0)));
  return { score, label: ['', 'Weak', 'Fair', 'Good', 'Strong'][score] };
}

function StrengthMeter({ value }: { value: string }) {
  const { score, label } = strength(value);
  if (!value) return null;
  const tone = score <= 1 ? 'bg-danger' : score === 2 ? 'bg-warn' : 'bg-ok';
  return (
    <div className="flex items-center gap-2" aria-live="polite">
      <div className="flex gap-1 flex-1" aria-hidden>
        {[1, 2, 3, 4].map((i) => (
          <span key={i} className={cx('h-1 flex-1 rounded-full', i <= score ? tone : 'bg-line')} />
        ))}
      </div>
      <span className="text-[12.5px] text-ink-3 w-12 text-right">{label}</span>
    </div>
  );
}

export function PasswordSheet({
  open,
  onClose,
  onSubmit,
}: {
  open: boolean;
  onClose: () => void;
  onSubmit: (current: string, next: string) => Promise<void>;
}) {
  const [current, setCurrent] = useState('');
  const [next, setNext] = useState('');
  const [confirm, setConfirm] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setCurrent('');
      setNext('');
      setConfirm('');
      setError(null);
    }
  }, [open]);

  const tooShort = next.length > 0 && next.length < 8;
  const mismatch = confirm.length > 0 && confirm !== next;
  const valid = !!current && next.length >= 8 && next === confirm;

  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      await onSubmit(current, next);
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
      title="Change password"
      description="You stay signed in on this device."
      width={460}
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" loading={busy} disabled={!valid} onClick={submit}>
            Change password
          </Button>
        </>
      }
    >
      <form
        className="flex flex-col gap-5"
        onSubmit={(e) => {
          e.preventDefault();
          if (valid) submit();
        }}
      >
        <Field label="Current password" error={error}>
          <Input type="password" autoComplete="current-password" value={current} onChange={(e) => setCurrent(e.target.value)} />
        </Field>
        <Field label="New password" hint="At least 8 characters." error={tooShort ? 'Use at least 8 characters.' : undefined}>
          <Input type="password" autoComplete="new-password" value={next} onChange={(e) => setNext(e.target.value)} />
        </Field>
        <StrengthMeter value={next} />
        <Field label="Repeat new password" error={mismatch ? 'The passwords do not match.' : undefined}>
          <Input type="password" autoComplete="new-password" value={confirm} onChange={(e) => setConfirm(e.target.value)} />
        </Field>
        <button type="submit" hidden aria-hidden tabIndex={-1} />
      </form>
    </Sheet>
  );
}

export function EmailSheet({
  open,
  onClose,
  currentEmail,
  onSubmit,
}: {
  open: boolean;
  onClose: () => void;
  currentEmail: string;
  onSubmit: (email: string, currentPassword: string) => Promise<void>;
}) {
  const [email, setEmail] = useState('');
  const [pw, setPw] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setEmail(currentEmail);
      setPw('');
      setError(null);
    }
  }, [open, currentEmail]);

  const valid = /\S+@\S+\.\S+/.test(email) && email.trim() !== currentEmail && !!pw;

  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      await onSubmit(email.trim(), pw);
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
      title="Change sign-in email"
      description="You will sign in with the new address from now on."
      width={460}
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" loading={busy} disabled={!valid} onClick={submit}>
            Change email
          </Button>
        </>
      }
    >
      <Field label="New email">
        <Input type="email" autoComplete="email" value={email} onChange={(e) => setEmail(e.target.value)} />
      </Field>
      <Field label="Current password" hint="Needed to confirm it's you." error={error}>
        <Input type="password" autoComplete="current-password" value={pw} onChange={(e) => setPw(e.target.value)} />
      </Field>
    </Sheet>
  );
}
