import type { ReactNode } from 'react';
import { formatBytes } from '../../../api';
import type { DashboardStats, ReplicationInfo, UsageStats } from '../../../types';
import { Badge, Card, Skeleton } from '../../../ui';

/** /admin/stats returns more than DashboardStats declares. */
export type InstanceStats = DashboardStats & {
  uptime?: string;
  storage_driver?: string;
  replication?: { enabled?: boolean; role?: string; node_id?: string; peers?: number };
};

function Item({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-0.5 min-w-[110px]">
      <span className="text-[12.5px] text-ink-3">{label}</span>
      <span className="text-sm font-semibold text-ink tabular-nums">{children}</span>
    </div>
  );
}

/** Compact instance health summary shown above the project list. */
export function HealthStrip({
  stats,
  usage,
  replication,
  loading,
  error,
}: {
  stats: InstanceStats | null;
  usage: UsageStats | null;
  replication: ReplicationInfo | null;
  loading: boolean;
  error: string | null;
}) {
  if (loading && !stats) {
    return (
      <Card className="mb-6">
        <div className="flex flex-wrap gap-6">
          {Array.from({ length: 5 }, (_, i) => (
            <Skeleton key={i} className="h-9 w-[110px]" />
          ))}
        </div>
      </Card>
    );
  }

  const repl = stats?.replication;
  const replRole = repl?.role ?? replication?.role;
  const peers = repl?.peers ?? replication?.peers;
  const replOn = repl?.enabled ?? replication?.active;

  return (
    <Card className="mb-6">
      <div className="flex flex-wrap items-center gap-x-8 gap-y-4">
        <Item label="Status">
          {error ? (
            <Badge tone="danger" dot>
              Unreachable
            </Badge>
          ) : (
            <Badge tone="ok" dot>
              Healthy
            </Badge>
          )}
        </Item>
        {stats?.uptime && <Item label="Uptime">{stats.uptime}</Item>}
        {stats?.memory_bytes != null && <Item label="Memory">{formatBytes(stats.memory_bytes)}</Item>}
        {stats?.storage_bytes != null && (
          <Item label="Disk">
            {formatBytes(stats.storage_bytes)}
            {stats.max_storage_bytes ? <span className="font-normal text-ink-3"> / {formatBytes(stats.max_storage_bytes)}</span> : null}
          </Item>
        )}
        {replRole && (
          <Item label="Replication">
            <span className="capitalize">{replRole}</span>
            {replOn && peers != null ? (
              <span className="font-normal text-ink-3">
                {' '}
                · {peers} {peers === 1 ? 'peer' : 'peers'}
              </span>
            ) : null}
          </Item>
        )}
        {usage?.enabled && (
          <>
            <Item label="Requests">{usage.total_requests.toLocaleString()}</Item>
            <Item label="Bandwidth">
              {formatBytes(usage.total_bandwidth_up)} <span className="font-normal text-ink-3">up</span> ·{' '}
              {formatBytes(usage.total_bandwidth_down)} <span className="font-normal text-ink-3">down</span>
            </Item>
          </>
        )}
        {(stats?.region || stats?.version) && (
          <Item label="Server">
            {stats?.region}
            {stats?.version ? <span className="font-normal text-ink-3"> · v{stats.version}</span> : null}
          </Item>
        )}
      </div>
    </Card>
  );
}
