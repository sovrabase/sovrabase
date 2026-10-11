import { KeyRound, MailCheck, Wand2 } from 'lucide-react';
import { Badge, Card, CardHeader, Row, Stat } from '../../../ui';
import type { UserRow } from './types';

const FEATURES = [
  { icon: MailCheck, label: 'Email verification', text: 'New accounts get a link to confirm their address.', status: 'On' },
  { icon: Wand2, label: 'Magic links', text: 'People can sign in with a one-time link sent by email.', status: 'Available' },
  { icon: KeyRound, label: 'Two-factor (TOTP)', text: 'People can add an authenticator app to their account.', status: 'Available' },
];

export function SecurityPanel({ users }: { users: UserRow[] }) {
  const total = users.length;
  const verified = users.filter((u) => u.is_verified).length;
  const mfa = users.filter((u) => u.mfa_enabled).length;
  const pct = (n: number) => (total ? `${Math.round((n / total) * 100)}%` : '—');
  return (
    <div className="flex flex-col gap-5">
      <Card>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-5">
          <Stat label="Users" value={total.toLocaleString()} />
          <Stat label="Verified email" value={pct(verified)} sub={`${verified.toLocaleString()} users`} />
          <Stat label="Two-factor on" value={pct(mfa)} sub={`${mfa.toLocaleString()} users`} />
        </div>
      </Card>
      <Card padded={false}>
        <CardHeader title="Built-in protections" description="These are part of every project and can't be switched off here." />
        <div className="border-t border-line">
          {FEATURES.map((f) => (
            <Row key={f.label} className="py-4">
              <span className="w-9 h-9 shrink-0 inline-flex items-center justify-center rounded-[9px] border border-line bg-sunken text-ink">
                <f.icon className="w-4 h-4" aria-hidden />
              </span>
              <div className="flex-1 min-w-[180px]">
                <div className="text-sm font-semibold text-ink">{f.label}</div>
                <div className="text-[12.5px] text-ink-3">{f.text}</div>
              </div>
              <Badge tone="ok" dot>
                {f.status}
              </Badge>
            </Row>
          ))}
        </div>
      </Card>
    </div>
  );
}
