import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { ArrowLeft, RefreshCw, Trash2, Unlock } from 'lucide-react';
import {
  Banner,
  Button,
  ButtonLink,
  Card,
  CardHeader,
  CodeBlock,
  ConfirmDialog,
  Field,
  Input,
  Page,
  PageHeader,
  Spinner,
  Stat,
  Tabs,
  Textarea,
  useToast,
} from '../../../ui';
import { formatDate } from '../../../api';
import { useProject } from '../../../shell/ProjectContext';
import { projectPath } from '../../../shell/nav';
import { automationsApi, queuePublic, type QueueInfo, type QueueMessage } from './model';

function parseJson(text: string): { ok: true; value: Record<string, unknown> } | { ok: false; error: string } {
  if (!text.trim()) return { ok: true, value: {} };
  try {
    const v = JSON.parse(text);
    if (!v || typeof v !== 'object' || Array.isArray(v)) return { ok: false, error: 'The body must be a JSON object.' };
    return { ok: true, value: v as Record<string, unknown> };
  } catch {
    return { ok: false, error: 'That is not valid JSON.' };
  }
}

export function QueueDetail({ queueName }: { queueName: string | null }) {
  const { project, projectId } = useProject();
  const navigate = useNavigate();
  const { showToast } = useToast();
  const p = (s: string) => projectPath(projectId, s);
  const apiKey = project.api_key ?? '';

  const [info, setInfo] = useState<QueueInfo | null>(null);
  const [loading, setLoading] = useState(!!queueName);
  const [confirmPurge, setConfirmPurge] = useState(false);
  const [unsticking, setUnsticking] = useState(false);

  // Test panel
  const [tab, setTab] = useState<'send' | 'receive'>('send');
  const [newName, setNewName] = useState('');
  const [body, setBody] = useState('{\n  "hello": "world"\n}');
  const [sending, setSending] = useState(false);
  const [sent, setSent] = useState<string | null>(null);
  const [limit, setLimit] = useState('5');
  const [receiving, setReceiving] = useState(false);
  const [received, setReceived] = useState<QueueMessage[] | null>(null);
  const [deleting, setDeleting] = useState<string | null>(null);
  const [tried, setTried] = useState(false);

  const load = useCallback(async () => {
    if (!queueName) return;
    const list = await automationsApi.listQueues(projectId);
    setInfo(list.find((q) => q.name === queueName) ?? { name: queueName, visible: 0, in_flight: 0, total: 0 });
    setLoading(false);
  }, [projectId, queueName]);

  useEffect(() => {
    load();
  }, [load]);

  const name = queueName ?? newName.trim();
  const parsed = parseJson(body);
  const nameError = !queueName && !newName.trim() ? 'Name the queue.' : null;

  const send = async () => {
    setTried(true);
    if (nameError || !parsed.ok) return;
    setSending(true);
    try {
      const r = await queuePublic<{ id: string; queue: string }>(apiKey, 'send', { queue: name, body: parsed.value });
      setSent(r.id);
      showToast('Message sent', 'success');
      if (!queueName) navigate(p(`automations/queue/${encodeURIComponent(name)}`), { replace: true });
      else load();
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setSending(false);
    }
  };

  const receive = async () => {
    setReceiving(true);
    try {
      const n = Math.max(1, Math.min(100, parseInt(limit, 10) || 1));
      const r = await queuePublic<{ data: QueueMessage[] }>(apiKey, 'receive', { queue: name, limit: n });
      setReceived(r.data || []);
      load();
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setReceiving(false);
    }
  };

  const ack = async (id: string) => {
    setDeleting(id);
    try {
      await queuePublic(apiKey, 'delete', { queue: name, id });
      setReceived((list) => list?.filter((m) => m.id !== id) ?? null);
      showToast('Message deleted', 'success');
      load();
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setDeleting(null);
    }
  };

  const unstick = async () => {
    if (!queueName) return;
    setUnsticking(true);
    try {
      await automationsApi.makeVisible(projectId, queueName);
      showToast('In-flight messages are visible again', 'success');
      load();
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setUnsticking(false);
    }
  };

  const purge = async () => {
    if (!queueName) return;
    try {
      await automationsApi.purgeQueue(projectId, queueName);
      showToast(`Queue "${queueName}" purged`, 'success');
      setReceived(null);
      load();
    } catch (e) {
      showToast((e as Error).message, 'error');
    }
  };

  if (loading) return <Spinner label="Loading queue" />;

  const sendForm = (
    <div className="flex flex-col gap-4">
      {!queueName && (
        <Field label="Queue name" error={tried ? nameError : null} hint="The queue is created by its first message.">
          <Input mono value={newName} onChange={(e) => setNewName(e.target.value)} placeholder="receipts" />
        </Field>
      )}
      <Field label="Message body" error={!parsed.ok ? parsed.error : null}>
        <Textarea mono rows={6} value={body} onChange={(e) => setBody(e.target.value)} />
      </Field>
      <div className="flex flex-wrap items-center gap-3">
        <Button variant="primary" loading={sending} onClick={send} disabled={!apiKey}>
          {queueName ? 'Send message' : 'Create queue'}
        </Button>
        {sent && (
          <span className="text-[12.5px] text-ink-3">
            Sent <code className="font-mono">{sent}</code>
          </span>
        )}
      </div>
    </div>
  );

  return (
    <Page>
      <div className="mb-2">
        <ButtonLink variant="ghost" size="sm" to={p('automations')} icon={<ArrowLeft className="w-4 h-4" />}>
          Automations
        </ButtonLink>
      </div>
      <PageHeader
        title={queueName ? <span className="font-mono">{queueName}</span> : 'New queue'}
        description={
          queueName
            ? 'Durable message queue. Your workers receive messages, then delete them when done.'
            : 'Queues are created when the first message is sent.'
        }
        actions={
          queueName ? (
            <>
              <Button icon={<RefreshCw className="w-4 h-4" />} onClick={load}>
                Refresh
              </Button>
              <Button icon={<Unlock className="w-4 h-4" />} loading={unsticking} onClick={unstick}>
                Make visible
              </Button>
              <Button variant="danger" icon={<Trash2 className="w-4 h-4" />} onClick={() => setConfirmPurge(true)}>
                Purge
              </Button>
            </>
          ) : undefined
        }
      />
      {!apiKey && (
        <div className="mb-4">
          <Banner tone="warn" title="No project key">
            Sending and receiving test messages uses the project's API key, and this project has none.
          </Banner>
        </div>
      )}
      <div className="flex flex-col gap-5">
        {queueName && info && (
          <Card>
            <div className="grid grid-cols-3 gap-4">
              <Stat label="Waiting" value={info.visible.toLocaleString()} sub="ready to receive" />
              <Stat label="In flight" value={info.in_flight.toLocaleString()} sub="received, not deleted" />
              <Stat label="Total sent" value={info.total.toLocaleString()} />
            </div>
          </Card>
        )}
        <Card padded={false}>
          <CardHeader
            title={queueName ? 'Test the queue' : 'First message'}
            description={queueName ? 'Calls the public queue API with the project key, like your app would.' : undefined}
          />
          {queueName ? (
            <>
              <Tabs
                className="px-5"
                value={tab}
                onChange={setTab}
                tabs={[
                  { value: 'send', label: 'Send' },
                  { value: 'receive', label: 'Receive and delete' },
                ]}
              />
              <div className="p-5">
                {tab === 'send' ? (
                  sendForm
                ) : (
                  <div className="flex flex-col gap-4">
                    <div className="flex flex-wrap items-end gap-3">
                      <Field label="How many" className="w-[120px]">
                        <Input type="number" min={1} max={100} value={limit} onChange={(e) => setLimit(e.target.value)} />
                      </Field>
                      <Button loading={receiving} onClick={receive} disabled={!apiKey}>
                        Receive
                      </Button>
                    </div>
                    <p className="m-0 text-[12.5px] text-ink-3">
                      Received messages are hidden from other workers until you delete them or they time out.
                    </p>
                    {received !== null &&
                      (received.length === 0 ? (
                        <p className="m-0 text-[13px] text-ink-3">No messages waiting.</p>
                      ) : (
                        <ul className="m-0 p-0 list-none flex flex-col gap-3">
                          {received.map((m) => (
                            <li key={m.id} className="border border-line rounded-lg p-3 flex flex-col gap-2">
                              <div className="flex flex-wrap items-center gap-2">
                                <code className="font-mono text-[12px] text-ink-2 truncate flex-1 min-w-0">{m.id}</code>
                                {m.created_at && <span className="text-[12px] text-ink-3">{formatDate(m.created_at)}</span>}
                                {m.receive_count ? <span className="text-[12px] text-ink-3">received {m.receive_count}×</span> : null}
                                <Button size="sm" variant="ghost" className="text-danger" loading={deleting === m.id} onClick={() => ack(m.id)}>
                                  Delete
                                </Button>
                              </div>
                              <CodeBlock>{JSON.stringify(m.body ?? {}, null, 2)}</CodeBlock>
                            </li>
                          ))}
                        </ul>
                      ))}
                  </div>
                )}
              </div>
            </>
          ) : (
            <div className="px-5 pb-5">{sendForm}</div>
          )}
        </Card>
      </div>
      <ConfirmDialog
        open={confirmPurge}
        onClose={() => setConfirmPurge(false)}
        onConfirm={purge}
        title={`Purge ${queueName}?`}
        confirmLabel="Purge"
        confirmText={queueName ?? undefined}
      >
        Every message in the queue is deleted. This cannot be undone.
      </ConfirmDialog>
    </Page>
  );
}
