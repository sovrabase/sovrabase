import { Badge, Field, Input, SaveBar, Section, Select, TokenInput } from '../../../ui';
import type { ReactNode } from 'react';
import { DefList, str, useConfigSection, type Config, type SectionProps } from './shared';

type Form = { role: string; node_id: string; repl_addr: string; peers: string[] };

function normRole(r: string) {
  const v = r.toLowerCase();
  return v === 'standalone' ? '' : v;
}

const toForm = (c: Config): Form => ({
  role: normRole(str(c, 'role')),
  node_id: str(c, 'node_id'),
  repl_addr: str(c, 'repl_addr'),
  peers: Array.isArray(c.peers) ? (c.peers as unknown[]).map(String) : [],
});

const toBody = (d: Form) => ({ role: d.role, node_id: d.node_id.trim(), repl_addr: d.repl_addr.trim(), peers: d.peers });

const ROLES = [
  { value: '', label: 'Standalone', help: 'A single server. No replication.' },
  { value: 'master', label: 'Master', help: 'Accepts writes and streams them to the other nodes.' },
  { value: 'heir', label: 'Heir', help: 'Keeps a full copy and takes over if the master goes away.' },
  { value: 'reader', label: 'Reader', help: 'Read-only copy that serves reads close to users.' },
];

type Live = { enabled?: boolean; role?: string; node_id?: string; peers?: number };

function liveRows(live: Live): Array<[string, ReactNode]> {
  const rows: Array<[string, ReactNode]> = [];
  rows.push(['State', live.enabled ? stateBadge(true) : stateBadge(false)]);
  rows.push(['Role', capitalize(live.role || 'standalone')]);
  if (live.node_id) rows.push(['Node', live.node_id]);
  if (live.enabled) rows.push(['Peers connected', String(live.peers ?? 0)]);
  return rows;
}

function stateBadge(on: boolean) {
  return on ? (
    <Badge tone="ok" dot>
      Replicating
    </Badge>
  ) : (
    <Badge>Off</Badge>
  );
}

const capitalize = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);

export function ReplicationSection({ config, onSaved, live }: SectionProps & { live: Live | null }) {
  const { draft, patch, dirty, reset, saving, save } = useConfigSection(config, toForm, toBody, onSaved);
  if (!draft) return null;
  const on = draft.role !== '';

  return (
    <>
      {live && (
        <Section title="Status" description="What the running server reports.">
          <DefList rows={liveRows(live)} />
        </Section>
      )}

      <Section title="This node" description="Changes apply after a restart." last={!on}>
        <Field label="Role" hint={ROLES.find((r) => r.value === draft.role)?.help}>
          <Select value={draft.role} onChange={(e) => patch({ role: e.target.value })} options={ROLES} />
        </Field>
      </Section>

      {on && (
        <Section title="Cluster" description="How this node finds the others." last>
          <Field label="Node ID" hint="Unique name of this node in the cluster.">
            <Input mono value={draft.node_id} onChange={(e) => patch({ node_id: e.target.value })} placeholder="eu-west-1" />
          </Field>
          <Field label="Listen address" hint="Where this node accepts replication traffic.">
            <Input mono value={draft.repl_addr} onChange={(e) => patch({ repl_addr: e.target.value })} placeholder="0.0.0.0:9000" />
          </Field>
          <Field label="Peers" hint="Address of each other node. Press Enter after each one.">
            <TokenInput mono value={draft.peers} onChange={(v) => patch({ peers: v })} placeholder="10.0.0.2:9000" />
          </Field>
        </Section>
      )}

      <SaveBar dirty={dirty} saving={saving} onSave={save} onDiscard={reset} message="Unsaved changes · restart to apply" />
    </>
  );
}
