import { useCallback, useEffect, useRef, useState } from 'react';
import { ChevronRight, Mail, RotateCcw } from 'lucide-react';
import { api, adminProjectPath, formatDate } from '../../../api';
import type { EmailLogEntry, EmailTemplate } from '../../../types';
import {
  Badge,
  Banner,
  Button,
  Card,
  CardHeader,
  ConfirmDialog,
  DataTable,
  EmptyState,
  Field,
  Input,
  Row,
  Sheet,
  Skeleton,
  Textarea,
  useToast,
  type Column,
} from '../../../ui';

type Template = EmailTemplate & { html_body?: string };

const TEMPLATE_INFO: Record<string, { label: string; description: string; variables: string[] }> = {
  email_verification: { label: 'Verify email', description: 'Sent after sign-up to confirm the address.', variables: ['{{.URL}}', '{{.Email}}', '{{.Token}}'] },
  password_reset: { label: 'Reset password', description: 'Sent when someone asks to reset their password.', variables: ['{{.URL}}', '{{.Email}}', '{{.Token}}'] },
  magic_link: { label: 'Magic link', description: 'One-click sign-in link.', variables: ['{{.URL}}', '{{.Email}}', '{{.Token}}'] },
  welcome: { label: 'Welcome', description: 'Sent once the account is ready.', variables: ['{{.Email}}'] },
  invitation: { label: 'Invitation', description: 'Sent when you invite someone to the project.', variables: ['{{.URL}}', '{{.Email}}', '{{.ProjectName}}'] },
};

const info = (t: string) => TEMPLATE_INFO[t] ?? { label: t, description: '', variables: [] };

function TemplatesCard({ projectId }: { projectId: string }) {
  const { showToast } = useToast();
  const [templates, setTemplates] = useState<Template[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [edit, setEdit] = useState<Template | null>(null);
  const [form, setForm] = useState({ subject: '', body: '', html_body: '' });
  const [saving, setSaving] = useState(false);
  const [resetOpen, setResetOpen] = useState(false);
  const bodyRef = useRef<HTMLTextAreaElement>(null);
  const lastFocus = useRef<'subject' | 'body' | 'html_body'>('body');

  const load = useCallback(async () => {
    try {
      const res = await api<{ data: Template[] }>(adminProjectPath(projectId, '/email-templates'));
      setTemplates(res.data || []);
      setError(null);
    } catch (e) {
      setError((e as Error).message);
      setTemplates([]);
    }
  }, [projectId]);

  useEffect(() => {
    load();
  }, [load]);

  const open = (t: Template) => {
    setEdit(t);
    setForm({ subject: t.subject, body: t.body, html_body: t.html_body ?? '' });
    lastFocus.current = 'body';
  };

  const insert = (v: string) => {
    const key = lastFocus.current;
    if (key === 'body' && bodyRef.current) {
      const el = bodyRef.current;
      const s = el.selectionStart ?? form.body.length;
      const e = el.selectionEnd ?? s;
      setForm((f) => ({ ...f, body: f.body.slice(0, s) + v + f.body.slice(e) }));
      requestAnimationFrame(() => {
        el.focus();
        el.setSelectionRange(s + v.length, s + v.length);
      });
    } else {
      setForm((f) => ({ ...f, [key]: f[key] + v }));
    }
  };

  const save = async () => {
    if (!edit || !form.subject.trim() || !form.body.trim()) return;
    setSaving(true);
    try {
      await api(adminProjectPath(projectId, '/email-templates'), {
        method: 'POST',
        body: JSON.stringify({ type: edit.type, subject: form.subject.trim(), body: form.body.trim(), html_body: form.html_body.trim() }),
      });
      showToast('Template saved', 'success');
      setEdit(null);
      load();
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setSaving(false);
    }
  };

  const reset = async () => {
    if (!edit) return;
    try {
      await api(adminProjectPath(projectId, `/email-templates/${encodeURIComponent(edit.type)}/reset`), { method: 'POST' });
      showToast('Template reset to default', 'success');
      setEdit(null);
      load();
    } catch (e) {
      showToast((e as Error).message, 'error');
      throw e;
    }
  };

  return (
    <Card padded={false}>
      <CardHeader title="Email templates" description="Every email Auth sends. They go out through the instance email settings." />
      <div className="border-t border-line">
        {templates === null ? (
          <div className="p-5 flex flex-col gap-3">
            <Skeleton className="h-10" />
            <Skeleton className="h-10" />
            <Skeleton className="h-10" />
          </div>
        ) : error ? (
          <div className="p-5">
            <Banner tone="danger" title="Templates could not be loaded">
              {error}
            </Banner>
          </div>
        ) : templates.length === 0 ? (
          <EmptyState icon={<Mail className="w-5 h-5" />} title="No templates" description="This server has no email templates to edit." />
        ) : (
          templates.map((t) => {
            const i = info(t.type);
            return (
              <Row key={t.type} onClick={() => open(t)} className="py-3.5">
                <button type="button" className="flex-1 min-w-0 text-left flex flex-col gap-0.5" onClick={(e) => (e.stopPropagation(), open(t))}>
                  <span className="flex items-center gap-2 flex-wrap">
                    <span className="text-sm font-semibold text-ink">{i.label}</span>
                    {t.updated_at ? <Badge tone="accent">Edited</Badge> : <Badge>Default</Badge>}
                  </span>
                  <span className="text-[12.5px] text-ink-3 truncate">{t.subject}</span>
                </button>
                {t.updated_at && <span className="text-[12.5px] text-ink-3 hidden sm:inline">Edited {formatDate(t.updated_at)}</span>}
                <ChevronRight className="w-4 h-4 text-ink-3" aria-hidden />
              </Row>
            );
          })
        )}
      </div>

      <Sheet
        open={!!edit}
        onClose={() => setEdit(null)}
        width={640}
        title={edit ? info(edit.type).label : ''}
        description={edit ? info(edit.type).description : undefined}
        footer={
          <>
            {edit?.updated_at && (
              <Button variant="ghost" icon={<RotateCcw className="w-4 h-4" />} onClick={() => setResetOpen(true)} className="mr-auto">
                Reset to default
              </Button>
            )}
            <Button variant="ghost" onClick={() => setEdit(null)}>
              Cancel
            </Button>
            <Button variant="primary" loading={saving} disabled={!form.subject.trim() || !form.body.trim()} onClick={save}>
              Save
            </Button>
          </>
        }
      >
        {edit && (
          <>
            <Field label="Subject">
              <Input value={form.subject} onFocus={() => (lastFocus.current = 'subject')} onChange={(e) => setForm({ ...form, subject: e.target.value })} />
            </Field>
            <Field label="Body" hint="Plain text. Sent to every mail client.">
              <Textarea
                ref={bodyRef}
                mono
                rows={12}
                value={form.body}
                onFocus={() => (lastFocus.current = 'body')}
                onChange={(e) => setForm({ ...form, body: e.target.value })}
              />
            </Field>
            {info(edit.type).variables.length > 0 && (
              <div className="flex flex-wrap items-center gap-1.5">
                <span className="text-[12.5px] text-ink-3 mr-1">Insert</span>
                {info(edit.type).variables.map((v) => (
                  <button
                    key={v}
                    type="button"
                    onMouseDown={(e) => e.preventDefault()}
                    onClick={() => insert(v)}
                    className="h-[26px] px-2 rounded-[6px] border border-line-2 bg-surface font-mono text-[12px] text-ink-2 hover:border-accent hover:text-accent"
                  >
                    {v}
                  </button>
                ))}
              </div>
            )}
            <Field label="HTML body" optional hint="Mail clients that show HTML use this instead of the plain text.">
              <Textarea
                mono
                rows={8}
                value={form.html_body}
                placeholder="<p>Your content here</p>"
                onFocus={() => (lastFocus.current = 'html_body')}
                onChange={(e) => setForm({ ...form, html_body: e.target.value })}
              />
            </Field>
          </>
        )}
      </Sheet>

      <ConfirmDialog
        open={resetOpen}
        onClose={() => setResetOpen(false)}
        onConfirm={reset}
        title="Reset this template?"
        confirmLabel="Reset"
      >
        <p className="m-0">Your subject and body are replaced by the default text.</p>
      </ConfirmDialog>
    </Card>
  );
}

/** Recent sends. The log is kept per server, so it shows mail from every project. */
function EmailLogCard() {
  const { showToast } = useToast();
  const [entries, setEntries] = useState<EmailLogEntry[] | null>(null);
  const [hidden, setHidden] = useState(false);
  const [retrying, setRetrying] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const d = await api<{ data: EmailLogEntry[]; total: number }>('/admin/email-log?limit=20&offset=0');
      setEntries(d.data || []);
    } catch {
      // Only instance admins can read the log; hide the card for everyone else.
      setHidden(true);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const retry = async (id: string) => {
    setRetrying(id);
    try {
      await api(`/admin/email-log/${encodeURIComponent(id)}/retry`, { method: 'POST' });
      showToast('Email sent again', 'success');
      load();
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setRetrying(null);
    }
  };

  if (hidden) return null;

  const columns: Column<EmailLogEntry>[] = [
    {
      key: 'subject',
      header: 'Email',
      width: 'minmax(0,2fr)',
      cell: (e) => (
        <div className="min-w-0">
          <div className="text-sm text-ink truncate">{e.subject}</div>
          <div className="text-[12.5px] text-ink-3 truncate">to {e.to}</div>
        </div>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      width: '110px',
      cell: (e) =>
        e.success ? (
          <Badge tone="ok" dot>
            Sent
          </Badge>
        ) : (
          <span title={e.error}>
            <Badge tone="danger" dot>
              Failed
            </Badge>
          </span>
        ),
    },
    { key: 'time', header: 'Time', width: '160px', cell: (e) => <span className="text-[12.5px] text-ink-3">{formatDate(e.timestamp)}</span> },
    {
      key: 'actions',
      header: <span className="sr-only">Actions</span>,
      width: '90px',
      align: 'right',
      cell: (e) =>
        !e.success ? (
          <Button size="sm" loading={retrying === e.id} onClick={() => retry(e.id)}>
            Retry
          </Button>
        ) : null,
    },
  ];

  return (
    <Card padded={false}>
      <CardHeader title="Recent emails" description="The last 20 emails this server sent, across all projects." actions={<Button size="sm" variant="ghost" onClick={load}>Refresh</Button>} />
      <div className="border-t border-line">
        {entries === null ? (
          <div className="p-5">
            <Skeleton className="h-10" />
          </div>
        ) : (
          <DataTable columns={columns} rows={entries} rowKey={(e) => e.id} empty={<EmptyState title="No emails sent yet" />} />
        )}
      </div>
    </Card>
  );
}

export function EmailsPanel({ projectId }: { projectId: string }) {
  return (
    <div className="flex flex-col gap-5">
      <TemplatesCard projectId={projectId} />
      <EmailLogCard />
    </div>
  );
}
