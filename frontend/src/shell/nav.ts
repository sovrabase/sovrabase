import type { LucideIcon } from 'lucide-react';
import {
  Activity,
  BarChart3,
  Blocks,
  Bot,
  Database,
  FolderKanban,
  HardDrive,
  Home,
  KeyRound,
  Settings,
  SquareFunction,
  Users,
  Workflow,
} from 'lucide-react';

export interface NavItem {
  label: string;
  to: string;
  icon: LucideIcon;
  /** Words the command palette also matches. */
  keywords?: string;
  end?: boolean;
}

export interface NavGroup {
  label?: string;
  items: NavItem[];
}

export function projectPath(projectId: string, section = '') {
  return `/projects/${encodeURIComponent(projectId)}${section ? `/${section}` : ''}`;
}

export function projectNav(projectId: string): NavGroup[] {
  const p = (s: string) => projectPath(projectId, s);
  return [
    { items: [{ label: 'Home', to: p(''), icon: Home, end: true, keywords: 'overview dashboard' }] },
    {
      label: 'Build',
      items: [
        { label: 'Database', to: p('database'), icon: Database, keywords: 'collections records rules indexes' },
        { label: 'Storage', to: p('storage'), icon: HardDrive, keywords: 'files buckets uploads' },
        { label: 'Auth', to: p('auth'), icon: KeyRound, keywords: 'users sign-in providers oauth email templates mfa' },
        { label: 'Functions', to: p('functions'), icon: SquareFunction, keywords: 'serverless code deploy' },
      ],
    },
    {
      label: 'Connect',
      items: [
        { label: 'Automations', to: p('automations'), icon: Workflow, keywords: 'cron schedule webhooks queues triggers' },
        { label: 'Plugins', to: p('plugins'), icon: Blocks, keywords: 'integrations stripe slack paypal' },
        { label: 'MCP server', to: p('mcp'), icon: Bot, keywords: 'ai claude cursor model context protocol tokens' },
      ],
    },
    {
      label: 'Observe',
      items: [
        { label: 'Logs', to: p('logs'), icon: Activity, keywords: 'requests errors' },
        { label: 'Analytics', to: p('analytics'), icon: BarChart3, keywords: 'events metrics usage' },
      ],
    },
    {
      label: 'Project',
      items: [{ label: 'Settings', to: p('settings'), icon: Settings, keywords: 'api keys team members variables config danger delete' }],
    },
  ];
}

export function instanceNav(role: string | null): NavGroup[] {
  const isAdmin = role === 'admin' || role === 'super_admin' || !role;
  const items: NavItem[] = [
    { label: 'Projects', to: '/projects', icon: FolderKanban, keywords: 'home' },
    { label: 'Plugins', to: '/plugins', icon: Blocks, keywords: 'library catalogue integrations' },
  ];
  if (role === 'super_admin') items.push({ label: 'Members', to: '/members', icon: Users, keywords: 'people admins users' });
  if (isAdmin) items.push({ label: 'Settings', to: '/settings', icon: Settings, keywords: 'smtp s3 security backups replication' });
  return [{ label: 'Instance', items }];
}
