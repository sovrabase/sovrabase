import { useEffect, useState, type FormEvent } from 'react';
import { Button, Field, Input, Sheet } from '../../../ui';

/** Create-project form in a side sheet. The API only needs a name. */
export function NewProjectSheet({
  open,
  onClose,
  onCreate,
}: {
  open: boolean;
  onClose: () => void;
  onCreate: (name: string) => Promise<void>;
}) {
  const [name, setName] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setName('');
      setError(null);
    }
  }, [open]);

  const submit = async (e?: FormEvent) => {
    e?.preventDefault();
    if (!name.trim()) return;
    setBusy(true);
    setError(null);
    try {
      await onCreate(name.trim());
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Sheet
      open={open}
      onClose={onClose}
      title="New project"
      description="A project gets its own database, users, files and API keys."
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" loading={busy} disabled={!name.trim()} onClick={() => submit()}>
            Create project
          </Button>
        </>
      }
    >
      <form onSubmit={submit} className="flex flex-col gap-5">
        <Field label="Name" hint="Lowercase letters, numbers and dashes read best in URLs." error={error}>
          <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="acme-shop" autoFocus autoComplete="off" />
        </Field>
        <p className="m-0 text-[13px] text-ink-3">
          After creating it you land on the project home, with its keys and a connect snippet.
        </p>
      </form>
    </Sheet>
  );
}
