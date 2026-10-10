import { useCallback, useEffect, useMemo, useState } from 'react';
import { Bot, KeyRound, Plus, RefreshCw } from 'lucide-react';
import { useProject } from '../../shell/ProjectContext';
import {
  Badge,
  Banner,
  Button,
  Card,
  CardHeader,
  ConfirmDialog,
  CopyButton,
  DataTable,
  EmptyState,
  Field,
  IconButton,
  Input,
  Page,
  PageHeader,
  Skeleton,
  Spinner,
  Stat,
  Switch,
  useToast,
  type Column,
} from '../../ui';
import { ConnectSnippets } from './mcp/ConnectSnippets';
import { TokenSheet } from './mcp/TokenSheet';
import {
  ago,
  clock,
  getConfig,
  isNotFound,
  listCalls,
  listTokens,
  putConfig,
  revokeToken,
  SCOPE_TEXT,
  SCOPES,
  type McpCall,
  type McpConfig,
  type McpTool,
  type McpToken,
} from './mcp/model';

export default function ProjectMcp() {
  const { project, projectId } = useProject();
  const { showToast } = useToast();
  const [config, setConfig] = useState<McpConfig | null>(null);
  const [tokens, setTokens] = useState<McpToken[] | null>(null);
  const [calls, setCalls] = useState<McpCall[] | null>(null);
  const [missing, setMissing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [toggling, setToggling] = useState(false);
  const [sheet, setSheet] = useState(false);
  const [fresh, setFresh] = useState<string | null>(null);
  const [revoke, setRevoke] = useState<McpToken | null>(null);
  const [busyTool, setBusyTool] = useState<string | null>(null);

  const loadCalls = useCallback(() => {
    listCalls(projectId, 50)
      .then(setCalls)
      .catch(() => setCalls([]));
  }, [projectId]);

  const load = useCallback(async () => {
    try {
      const c = await getConfig(projectId);
      setConfig(c);
      setMissing(false);
      setError(null);
    } catch (e) {
      if (isNotFound(e)) setMissing(true);
      else setError((e as Error).message);
      return;
    }
    listTokens(projectId)
      .then(setTokens)
      .catch(() => setTokens([]));
    loadCalls();
  }, [projectId, loadCalls]);

  useEffect(() => {
    load();
  }, [load]);

  const url = `${window.location.origin}${config?.endpoint ?? '/mcp'}`;

  const setEnabled = async (enabled: boolean) => {
    setToggling(true);
    try {
      setConfig(await putConfig(projectId, { enabled }));
      showToast(enabled ? 'MCP server is on' : 'MCP server is off', 'success');
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setToggling(false);
    }
  };

  const setTool = async (tool: McpTool, on: boolean) => {
    if (!config) return;
    const disabled = on ? config.disabled_tools.filter((t) => t !== tool.name) : [...new Set([...config.disabled_tools, tool.name])];
    setBusyTool(tool.name);
    setConfig({ ...config, disabled_tools: disabled, tools: config.tools.map((t) => (t.name === tool.name ? { ...t, enabled: on } : t)) });
    try {
      setConfig(await putConfig(projectId, { disabled_tools: disabled }));
    } catch (e) {
      showToast((e as Error).message, 'error');
      load();
    } finally {
      setBusyTool(null);
    }
  };

  const doRevoke = async (t: McpToken) => {
    try {
      await revokeToken(projectId, t.id);
      setTokens((list) => (list ?? []).filter((x) => x.id !== t.id));
      showToast(`${t.name} revoked`, 'success');
    } catch (e) {
      showToast((e as Error).message, 'error');
    }
  };

  const header = (
    <PageHeader
      title="MCP server"
      meta={config ? <Badge tone={config.enabled ? 'ok' : 'neutral'} dot>{config.enabled ? 'On' : 'Off'}</Badge> : undefined}
      description="Let AI assistants such as Claude and Cursor work with this project, through tools you choose and tokens you hand out."
      actions={
        config ? (
          <div className="flex items-center gap-2.5 h-9 px-3 rounded-md border border-line bg-surface">
            <span className="text-[13px] font-medium text-ink" id="mcp-switch-label">
              {config.enabled ? 'On for this project' : 'Off for this project'}
            </span>
            <Switch checked={config.enabled} onChange={setEnabled} disabled={toggling} aria-label="MCP server for this project" />
          </div>
        ) : undefined
      }
    />
  );

  if (missing) {
    return (
      <Page>
        {header}
        <Banner tone="warn" title="This server doesn't have the MCP endpoints yet" action={<Button size="sm" onClick={load} icon={<RefreshCw className="w-3.5 h-3.5" />}>Try again</Button>}>
          The dashboard is newer than the server. Update Sovrabase to a version with MCP support, then reload this page.
        </Banner>
      </Page>
    );
  }
  if (error) {
    return (
      <Page>
        {header}
        <Banner tone="danger" title="MCP settings could not be loaded" action={<Button size="sm" onClick={load}>Try again</Button>}>
          {error}
        </Banner>
      </Page>
    );
  }
  if (!config) return <Spinner label="Loading MCP server" />;

  return (
    <Page>
      {header}

      {!config.enabled && (
        <div className="mb-6">
          <Banner
            tone="neutral"
            title="Nothing is exposed while MCP is off"
            action={
              <Button variant="primary" size="sm" loading={toggling} onClick={() => setEnabled(true)}>
                Switch on
              </Button>
            }
          >
            The endpoint refuses every token for {project.name} until you switch it on. You can prepare tokens and tools first.
          </Banner>
        </div>
      )}

      <div className="flex flex-col gap-6">
        {/* 1. Connect */}
        <Card padded={false}>
          <CardHeader title="1 · Connect a client" description="Streamable HTTP. The token picks the project, so one URL serves every project on this instance." />
          <div className="px-5 pb-5 flex flex-col gap-5 border-t border-line pt-4">
            <Field label="Endpoint">
              <Input mono readOnly value={url} suffix={<CopyButton text={url} />} />
            </Field>
            <ConnectSnippets url={url} token={fresh} />
          </div>
        </Card>

        {/* 2. Tokens */}
        <Card padded={false}>
          <CardHeader
            title="2 · Access tokens"
            description="One per person or tool. Read-only unless you tick more scopes."
            actions={
              <Button variant="primary" size="sm" icon={<Plus className="w-4 h-4" />} onClick={() => setSheet(true)}>
                New token
              </Button>
            }
          />
          <div className="border-t border-line">
            {!tokens ? (
              <div className="p-5 flex flex-col gap-2">
                <Skeleton className="h-9" />
                <Skeleton className="h-9" />
              </div>
            ) : (
              <DataTable
                rows={tokens}
                rowKey={(t) => t.id}
                columns={tokenColumns(setRevoke)}
                empty={
                  <EmptyState
                    icon={<KeyRound className="w-5 h-5" />}
                    title="No tokens yet"
                    description="A client needs a token to connect. Create one per person or tool."
                    action={
                      <Button variant="primary" icon={<Plus className="w-4 h-4" />} onClick={() => setSheet(true)}>
                        New token
                      </Button>
                    }
                  />
                }
              />
            )}
          </div>
        </Card>

        {/* 3. Tools */}
        <ToolsCard config={config} busyTool={busyTool} onToggle={setTool} />

        {/* 4. Calls */}
        <CallsCard calls={calls} onRefresh={loadCalls} />
      </div>

      <TokenSheet
        open={sheet}
        onClose={() => setSheet(false)}
        projectId={projectId}
        url={url}
        onCreated={(t) => {
          setFresh(t.token ?? null);
          const { token: _plain, ...rest } = t;
          void _plain;
          setTokens((list) => [rest, ...(list ?? [])]);
        }}
      />

      <ConfirmDialog
        open={!!revoke}
        onClose={() => setRevoke(null)}
        onConfirm={() => (revoke ? doRevoke(revoke) : undefined)}
        title={`Revoke ${revoke?.name ?? 'token'}?`}
        confirmLabel="Revoke"
      >
        <p className="m-0">
          Clients using <code className="font-mono">{revoke?.prefix}…</code> lose access right away. This can't be undone; create a new
          token if you need one again.
        </p>
      </ConfirmDialog>
    </Page>
  );
}

function tokenColumns(onRevoke: (t: McpToken) => void): Column<McpToken>[] {
  return [
    {
      key: 'name',
      header: 'Name',
      cell: (t) => (
        <span className="flex flex-col min-w-0">
          <span className="text-sm font-medium text-ink truncate">{t.name}</span>
          <code className="font-mono text-[12px] text-ink-3">{t.prefix}…</code>
        </span>
      ),
    },
    {
      key: 'scopes',
      header: 'Scopes',
      width: 'minmax(0,1.3fr)',
      cell: (t) => (
        <span className="flex flex-wrap gap-1">
          {t.scopes.map((s) => (
            <Badge key={s} tone={s === 'read' ? 'neutral' : 'accent'} className="font-mono !text-[11.5px]">
              {s}
            </Badge>
          ))}
        </span>
      ),
    },
    { key: 'used', header: 'Last used', width: '130px', cell: (t) => <span className="text-[13px] text-ink-3">{ago(t.last_used_at)}</span> },
    {
      key: 'revoke',
      header: <span className="sr-only">Actions</span>,
      width: '90px',
      align: 'right',
      cell: (t) => (
        <Button size="sm" variant="ghost" onClick={() => onRevoke(t)} aria-label={`Revoke ${t.name}`}>
          Revoke
        </Button>
      ),
    },
  ];
}

function ToolsCard({ config, busyTool, onToggle }: { config: McpConfig; busyTool: string | null; onToggle: (t: McpTool, on: boolean) => void }) {
  const groups = useMemo(() => {
    const order = SCOPES.map((s) => s.value);
    const m = new Map<string, McpTool[]>();
    for (const t of config.tools) m.set(t.scope, [...(m.get(t.scope) ?? []), t]);
    return [...m.entries()].sort((a, b) => {
      const ia = order.indexOf(a[0]);
      const ib = order.indexOf(b[0]);
      return (ia < 0 ? 99 : ia) - (ib < 0 ? 99 : ib);
    });
  }, [config.tools]);
  const on = config.tools.filter((t) => t.enabled).length;

  return (
    <Card padded={false}>
      <CardHeader
        title="3 · Tools"
        description="What a client may do. A tool also needs the matching scope on the token."
        actions={<span className="text-[12.5px] text-ink-3 tabular-nums">{on} of {config.tools.length} on</span>}
      />
      {groups.length === 0 && (
        <div className="border-t border-line">
          <EmptyState icon={<Bot className="w-5 h-5" />} title="This server exposes no tools" />
        </div>
      )}
      {groups.map(([scope, tools]) => (
        <div key={scope} className="border-t border-line">
          <h3 className="m-0 px-5 pt-3 pb-1 flex items-center gap-2 text-[12px] font-semibold uppercase tracking-[.06em] text-ink-3">
            {SCOPE_TEXT[scope] ?? scope}
            <code className="font-mono normal-case tracking-normal font-normal">{scope}</code>
          </h3>
          <ul className="m-0 p-0 list-none">
            {tools.map((t) => (
              <li key={t.name} className="flex items-center gap-3 px-5 py-2.5 border-t border-line first:border-t-0">
                <span className="flex-1 min-w-0 flex flex-col">
                  <span className="flex items-center gap-2 flex-wrap">
                    <code className="font-mono text-[13px] text-ink">{t.name}</code>
                    {t.write && <Badge tone="warn">changes data</Badge>}
                  </span>
                  <span className="text-[12.5px] leading-[18px] text-ink-3">{t.description}</span>
                </span>
                <Switch
                  checked={t.enabled}
                  disabled={busyTool === t.name}
                  onChange={(v) => onToggle(t, v)}
                  aria-label={`${t.name} ${t.enabled ? 'on' : 'off'}`}
                />
              </li>
            ))}
          </ul>
        </div>
      ))}
      <p className="m-0 px-5 py-3 border-t border-line text-[12.5px] text-ink-3">
        On every token: internal collections are hidden, collections starting with <code className="font-mono">_</code> are read-only, and
        password hashes and reset tokens are removed from results.
      </p>
    </Card>
  );
}

function CallsCard({ calls, onRefresh }: { calls: McpCall[] | null; onRefresh: () => void }) {
  const stats = useMemo(() => {
    const list = calls ?? [];
    return {
      total: list.length,
      failed: list.filter((c) => !c.ok).length,
      clients: new Set(list.map((c) => c.token_name)).size,
    };
  }, [calls]);

  const columns: Column<McpCall & { key: string }>[] = [
    { key: 'tool', header: 'Tool', cell: (c) => <code className="font-mono text-[13px] text-ink">{c.tool}</code> },
    {
      key: 'ok',
      header: 'Result',
      width: '110px',
      cell: (c) =>
        c.ok ? (
          <Badge tone="ok" dot>
            ok
          </Badge>
        ) : (
          <span title={c.error}>
            <Badge tone="danger" dot>
              failed
            </Badge>
          </span>
        ),
    },
    { key: 'at', header: 'When', width: '150px', cell: (c) => <span className="text-[13px] text-ink-3 tabular-nums">{clock(c.at)}</span> },
    { key: 'token', header: 'Token', cell: (c) => <span className="text-[13px] text-ink-2 truncate">{c.token_name || '—'}</span> },
    {
      key: 'ms',
      header: 'Time',
      width: '80px',
      align: 'right',
      hideOnMobile: true,
      cell: (c) => <span className="text-[13px] text-ink-3 tabular-nums">{c.duration_ms} ms</span>,
    },
    {
      key: 'err',
      header: 'Error',
      cell: (c) => (c.error ? <span className="text-[12.5px] text-danger truncate block">{c.error}</span> : null),
    },
  ];

  return (
    <Card padded={false}>
      <CardHeader
        title="Recent calls"
        description="The last calls kept in memory on this server. The list starts over when the server restarts."
        actions={<IconButton label="Refresh calls" size="sm" onClick={onRefresh}><RefreshCw className="w-4 h-4" /></IconButton>}
      />
      {calls && calls.length > 0 && (
        <div className="flex flex-wrap gap-x-10 gap-y-3 px-5 pb-4">
          <Stat label="Calls shown" value={stats.total} />
          <Stat label="Failed" value={stats.failed} />
          <Stat label="Tokens used" value={stats.clients} />
        </div>
      )}
      <div className="border-t border-line">
        {!calls ? (
          <div className="p-5">
            <Skeleton className="h-9" />
          </div>
        ) : (
          <DataTable
            rows={calls.map((c, i) => ({ ...c, key: String(i) }))}
            rowKey={(c) => c.key}
            columns={columns}
            empty={<EmptyState icon={<Bot className="w-5 h-5" />} title="No calls yet" description="Calls from connected clients show up here." />}
          />
        )}
      </div>
      <p className="m-0 px-5 py-3 border-t border-line text-[12.5px] text-ink-3">
        “failed” includes calls refused because the tool is off or the token lacks the scope. The client always gets a clear error.
      </p>
    </Card>
  );
}
