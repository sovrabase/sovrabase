import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { ArrowLeft, Trash2 } from 'lucide-react';
import {
  Badge,
  Button,
  ButtonLink,
  Card,
  CardHeader,
  ChipSelect,
  CodeBlock,
  ConfirmDialog,
  EmptyState,
  Field,
  Input,
  Page,
  PageHeader,
  Spinner,
  Switch,
  useToast,
} from '../../../ui';
import { formatDate } from '../../../api';
import { useProject } from '../../../shell/ProjectContext';
import { projectPath } from '../../../shell/nav';
import { automationsApi, RECORD_EVENTS, shortUrl, webhookEvents, type WebhookRow } from './model';
import { BuilderLayout, Join, Step } from './parts';

const SAMPLE = `{
  "event": "insert",
  "collection": "orders",
  "doc_id": "7f3a91c2",
  "data": { "...": "the record" },
  "timestamp": "2026-10-10T14:02:11Z"
}`;

export function WebhookBuilder({ webhookId }: { webhookId: string | null }) {
  const { projectId } = useProject();
  const navigate = useNavigate();
  const { showToast } = useToast();
  const p = (s: string) => projectPath(projectId, s);

  const [hook, setHook] = useState<WebhookRow | null>(null);
  const [url, setUrl] = useState('');
  const [events, setEvents] = useState<string[]>([]);
  const [enabled, setEnabled] = useState(true);
  const [loading, setLoading] = useState(!!webhookId);
  const [notFound, setNotFound] = useState(false);
  const [saving, setSaving] = useState(false);
  const [tried, setTried] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);

  useEffect(() => {
    if (!webhookId) return;
    setLoading(true);
    automationsApi
      .listWebhooks(projectId)
      .then((list) => {
        const w = list.find((x) => x.id === webhookId);
        if (!w) {
          setNotFound(true);
          return;
        }
        setHook(w);
        setUrl(w.url ?? '');
        setEvents(webhookEvents(w));
        setEnabled(!!w.enabled);
      })
      .catch((e) => showToast((e as Error).message, 'error'))
      .finally(() => setLoading(false));
  }, [projectId, webhookId, showToast]);

  const urlError = !url.trim() ? 'Enter the URL to call.' : !/^https?:\/\//.test(url.trim()) ? 'Start with http:// or https://' : null;

  const save = async () => {
    setTried(true);
    if (urlError) return;
    setSaving(true);
    const body = { url: url.trim(), events: events.join(','), enabled };
    try {
      if (hook) {
        const updated = await automationsApi.updateWebhook(projectId, hook.id, body);
        setHook({ ...hook, ...updated });
        showToast('Webhook saved', 'success');
      } else {
        const created = await automationsApi.createWebhook(projectId, body);
        showToast('Webhook created', 'success');
        navigate(p(`automations/webhook/${encodeURIComponent(created.id)}`), { replace: true });
      }
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setSaving(false);
    }
  };

  const togglePause = async (on: boolean) => {
    setEnabled(on);
    if (!hook) return;
    try {
      await automationsApi.updateWebhook(projectId, hook.id, { enabled: on });
      setHook({ ...hook, enabled: on });
      showToast(on ? 'Webhook is on' : 'Webhook is paused', 'success');
    } catch (e) {
      setEnabled(!on);
      showToast((e as Error).message, 'error');
    }
  };

  const remove = async () => {
    if (!hook) return;
    try {
      await automationsApi.deleteWebhook(projectId, hook.id);
      showToast('Webhook deleted', 'success');
      navigate(p('automations'));
    } catch (e) {
      showToast((e as Error).message, 'error');
    }
  };

  if (loading) return <Spinner label="Loading webhook" />;
  if (notFound) {
    return (
      <Page>
        <EmptyState title="This webhook no longer exists" action={<ButtonLink to={p('automations')}>All automations</ButtonLink>} />
      </Page>
    );
  }

  const title = hook?.url ? shortUrl(hook.url) : 'New record webhook';

  return (
    <Page>
      <div className="mb-2">
        <ButtonLink variant="ghost" size="sm" to={p('automations')} icon={<ArrowLeft className="w-4 h-4" />}>
          Automations
        </ButtonLink>
      </div>
      <PageHeader
        title={<span className="break-all">{title}</span>}
        meta={hook ? enabled ? <Badge tone="ok" dot>On</Badge> : <Badge>Paused</Badge> : undefined}
        description="POSTs the record to a URL when records are created, updated or deleted."
        actions={
          <>
            {hook && <Switch checked={enabled} onChange={togglePause} aria-label={enabled ? 'Pause webhook' : 'Turn webhook on'} />}
            <Button variant="primary" loading={saving} onClick={save}>
              {hook ? 'Save' : 'Create webhook'}
            </Button>
          </>
        }
      />
      <BuilderLayout
        main={
          <>
            <Step n={1} eyebrow="Trigger" title="When a record changes">
              <Field label="Events" hint={events.length ? undefined : 'None picked: every create, update and delete fires.'}>
                <ChipSelect options={RECORD_EVENTS} value={events} onChange={setEvents} aria-label="Record events" />
              </Field>
              {!hook && <Switch checked={enabled} onChange={setEnabled} label="On" description="Paused webhooks keep their settings and send nothing." />}
            </Step>
            <Join />
            <Step n={2} eyebrow="Action" title="POST to a URL">
              <Field label="URL" error={tried ? urlError : null} hint="Called with a JSON body. Failed calls are not retried.">
                <Input mono value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://your-app.com/api/webhook" />
              </Field>
            </Step>
            {hook && (
              <div className="mt-6 flex flex-wrap items-center justify-between gap-3 px-1">
                <span className="text-[12.5px] text-ink-3">{hook.created_at ? `Created ${formatDate(hook.created_at)}` : ''}</span>
                <Button variant="ghost" className="text-danger" icon={<Trash2 className="w-4 h-4" />} onClick={() => setConfirmDelete(true)}>
                  Delete webhook
                </Button>
              </div>
            )}
          </>
        }
        side={
          <Card padded={false}>
            <CardHeader title="What your URL receives" description="One POST per change, with this body." />
            <div className="px-5 pb-5">
              <CodeBlock>{SAMPLE}</CodeBlock>
              <p className="m-0 mt-3 text-[12.5px] text-ink-3">The server does not keep a delivery history for webhooks. Check your endpoint's logs.</p>
            </div>
          </Card>
        }
      />
      <ConfirmDialog open={confirmDelete} onClose={() => setConfirmDelete(false)} onConfirm={remove} title="Delete this webhook?">
        <span className="break-all">
          Changes stop being sent to <code className="font-mono">{hook?.url}</code>.
        </span>
      </ConfirmDialog>
    </Page>
  );
}
