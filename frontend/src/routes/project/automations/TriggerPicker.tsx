import { Link } from 'react-router-dom';
import { ArrowLeft, Blocks, Clock, Database, Inbox, Layers, type LucideIcon } from 'lucide-react';
import { ButtonLink, Page, PageHeader } from '../../../ui';
import { useProject } from '../../../shell/ProjectContext';
import { projectPath } from '../../../shell/nav';

interface Choice {
  to: string;
  icon: LucideIcon;
  title: string;
  description: string;
  note?: string;
}

/** Step 1 of "New automation": pick what starts it. */
export function TriggerPicker() {
  const { projectId } = useProject();
  const p = (s: string) => projectPath(projectId, s);
  const choices: Choice[] = [
    {
      to: p('automations/new/cron'),
      icon: Clock,
      title: 'On a schedule',
      description: 'Call a function or URL every hour, every night, or on any cron schedule.',
    },
    {
      to: p('automations/new/webhook'),
      icon: Database,
      title: 'When a record changes',
      description: 'POST the record to a URL when it is created, updated or deleted.',
    },
    {
      to: p('automations/new/queue'),
      icon: Layers,
      title: 'Queue message',
      description: 'Create a queue by sending its first message. Your workers receive and delete messages.',
    },
    {
      to: p('plugins'),
      icon: Inbox,
      title: 'Incoming webhook',
      description: 'Receive events from Stripe or PayPal. Set it up on the plugin page.',
      note: 'Opens Plugins',
    },
    {
      to: p('plugins'),
      icon: Blocks,
      title: 'Plugin event',
      description: 'Let a plugin react to sign-ups and record changes. Set it up on the plugin page.',
      note: 'Opens Plugins',
    },
  ];
  return (
    <Page>
      <PageHeader
        title="New automation"
        description="Choose what starts it. You set what it does on the next step."
        actions={
          <ButtonLink variant="ghost" to={p('automations')} icon={<ArrowLeft className="w-4 h-4" />}>
            Automations
          </ButtonLink>
        }
      />
      <div className="grid gap-3 grid-cols-[repeat(auto-fill,minmax(260px,1fr))]">
        {choices.map((c) => (
          <Link
            key={c.title}
            to={c.to}
            className="group flex flex-col gap-2 p-5 rounded-lg border border-line bg-surface hover:border-accent focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-accent/30 no-underline"
          >
            <span className="w-9 h-9 rounded-[9px] bg-accent-soft text-accent-2 flex items-center justify-center">
              <c.icon className="w-[18px] h-[18px]" aria-hidden />
            </span>
            <span className="text-[15px] font-semibold text-ink">{c.title}</span>
            <span className="text-[13px] leading-[19px] text-ink-3">{c.description}</span>
            {c.note && <span className="text-[12.5px] font-medium text-accent">{c.note} →</span>}
          </Link>
        ))}
      </div>
    </Page>
  );
}
