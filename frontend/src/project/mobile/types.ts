import type { Project } from '../../types';

export interface MobileDashboardProps {
  projectId: string;
  projectName: string;
  projects: Project[];
  onClose?: () => void;
  onChangeProject?: (id: string) => void;
}

export interface LogEntry {
  timestamp: string;
  method: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'ERROR' | 'PATCH';
  path: string;
  status: number;
  duration: string;
}

export interface UserRow {
  id: string;
  email: string;
  username?: string;
  name?: string;
  avatar_url?: string;
  role?: string;
  provider?: string;
  status?: 'Active' | 'Banned' | 'Pending';
}

export interface CollectionInfo {
  name: string;
  doc_count?: number;
}

export interface QueueInfo {
  name: string;
  visible: number;
  in_flight: number;
  total: number;
}

export interface ProviderConfig {
  name?: string;
  client_id: string;
  client_secret: string;
  scopes: string | string[];
  enabled: boolean;
  auth_url?: string;
  token_url?: string;
  userinfo_url?: string;
}

export type ActiveTab = 'overview' | 'database' | 'auth' | 'logs' | 'more';
export type ActiveSubPage =
  | 'grid' | 'storage' | 'team' | 'config' | 'webhooks' | 'cron'
  | 'functions' | 'queues' | 'integrations' | 'analytics' | 'api';
export type ActiveModal =
  | 'create_bucket' | 'upload_file' | 'invite_member' | 'add_webhook'
  | 'create_cron' | 'add_document' | 'import_json' | 'create_collection' | 'add_config'
  | null;
