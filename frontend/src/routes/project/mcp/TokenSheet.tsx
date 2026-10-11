import { useEffect, useState } from 'react';
import { Banner, Button, ChipSelect, CopyButton, Field, Input, Sheet, useToast } from '../../../ui';
import { ConnectSnippets } from './ConnectSnippets';
import { createToken, SCOPES, type McpToken } from './model';

export function TokenSheet({
  open,
  onClose,
  projectId,
  url,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  projectId: string;
  url: string;
  onCreated: (t: McpToken) => void;
}) {
  const { showToast } = useToast();
  const [name, setName] = useState('');
  const [scopes, setScopes] = useState<string[]>(['read']);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [created, setCreated] = useState<McpToken | null>(null);

  useEffect(() => {
    if (open) {
      setName('');
      setScopes(['read']);
      setError(null);
      setCreated(null);
    }
  }, [open]);

  const submit = async () => {
    if (!name.trim()) {
      setError('Give the token a name, such as the person or tool using it.');
      return;
    }
    if (!scopes.length) {
      setError('Pick at least one scope.');
      return;
    }
    setBusy(true);
    try {
      const t = await createToken(projectId, name.trim(), scopes);
      setCreated(t);
      onCreated(t);
      showToast('Token created', 'success');
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
      title={created ? 'Copy your token' : 'New access token'}
      description={created ? 'Shown once. Sovrabase keeps only a hash.' : 'One per person or tool, so you can revoke it alone.'}
      width={600}
      footer={
        created ? (
          <Button variant="primary" onClick={onClose}>
            Done
          </Button>
        ) : (
          <>
            <Button variant="ghost" onClick={onClose}>
              Cancel
            </Button>
            <Button variant="primary" loading={busy} onClick={submit}>
              Create token
            </Button>
          </>
        )
      }
    >
      {created?.token ? (
        <>
          <Banner tone="warn" title="Copy it now">
            You will not see this token again. If you lose it, revoke it and create a new one.
          </Banner>
          <Field label={created.name}>
            <Input mono readOnly value={created.token} onFocus={(e) => e.currentTarget.select()} suffix={<CopyButton text={created.token} />} />
          </Field>
          <div className="flex flex-col gap-2">
            <h3 className="m-0 text-[13px] font-semibold text-ink">Connect a client</h3>
            <ConnectSnippets url={url} token={created.token} />
          </div>
        </>
      ) : (
        <>
          <Field label="Name" hint="Who or what uses it, e.g. “Lea's Cursor”." error={error && !name.trim() ? error : undefined}>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Jeremy's Claude Code" autoFocus />
          </Field>
          <Field label="Scopes" hint="Scopes don't include each other: a token that reads and writes needs both. A tool also has to be switched on below.">
            <ChipSelect aria-label="Scopes" options={SCOPES} value={scopes} onChange={setScopes} />
          </Field>
          <ul className="m-0 p-0 list-none flex flex-col gap-1 text-[12.5px] text-ink-3">
            {SCOPES.map((s) => (
              <li key={s.value}>
                <code className="font-mono text-ink-2">{s.value}</code> · {s.description}
              </li>
            ))}
          </ul>
          {error && name.trim() && <Banner tone="danger">{error}</Banner>}
        </>
      )}
    </Sheet>
  );
}
