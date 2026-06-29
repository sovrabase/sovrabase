// ===== API Response Types =====

export interface Project {
  id: string;
  name: string;
  api_key?: string;
  env?: string;
  created_at?: string;
  updated_at?: string;
  member_count?: number;
  collection_count?: number;
  bucket_count?: number;
  is_online?: boolean;
  status?: string;
}

export interface TeamMember {
  user_id: string;
  email?: string;
  role: 'owner' | 'admin' | 'developer' | 'viewer';
  joined_at?: string;
  is_owner?: boolean;
}

export interface Collection {
  name: string;
  schema?: Record<string, unknown>;
  rls_rules?: RlsRule[];
  doc_count?: number;
  created_at?: string;
}

export interface RlsRule {
  action: string;
  expression: string;
}

export interface DatabaseDocument {
  _id: string;
  _createdAt?: string;
  _updatedAt?: string;
  [key: string]: unknown;
}

export interface User {
  id: string;
  email: string;
  name?: string;
  avatar_url?: string;
  role?: string;
  created_at?: string;
  _metadata?: ProviderMeta[];
}

export interface ProviderMeta {
  provider: string;
  provider_id: string;
}

export interface OAuthProvider {
  name: string;
  client_id: string;
  client_secret: string;
  redirect_url: string;
  auth_url: string;
  token_url: string;
  userinfo_url: string;
  scopes: string[];
  email_field: string;
  name_field: string;
  avatar_field: string;
  id_field: string;
}

export interface Bucket {
  name: string;
  file_count?: number;
  total_size?: number;
  created_at?: string;
}

export interface StorageFile {
  name: string;
  path: string;
  size?: number;
  content_type?: string;
  updated_at?: string;
  url?: string;
}

export interface ConfigEntry {
  key: string;
  value: string | object;
  type?: string;
  description?: string;
  public?: boolean;
}

export interface CronJob {
  id: string;
  name: string;
  schedule: string;
  endpoint?: string;
  method?: string;
  body?: unknown;
  enabled: boolean;
  last_run?: string;
  next_run?: string;
}

export interface Webhook {
  id: string;
  name: string;
  url: string;
  events: string[];
  enabled: boolean;
  created_at?: string;
}

export interface QueueMessage {
  id: string;
  queue: string;
  payload: unknown;
  status: string;
  created_at?: string;
}

export interface AnalyticsSummary {
  total_requests?: number;
  requests_by_endpoint?: Record<string, number>;
  bandwidth_up?: number;
  bandwidth_down?: number;
  period?: string;
}

export interface LogEntry {
  timestamp: string;
  level: string;
  message: string;
  source?: string;
}

export interface PluginInfo {
  plugins: string[];
  hooks: HookInfo[];
  routes: RouteInfo[];
}

export interface HookInfo {
  type: string;
  action: string;
  collection: string;
  count: number;
}

export interface RouteInfo {
  method: string;
  path: string;
}

// ===== Dashboard Stats =====

export interface DashboardStats {
  projects: number;
  memory_bytes?: number;
  storage_bytes: number;
  max_storage_bytes?: number;
  region: string;
  version: string;
}

export interface UsageStats {
  enabled: boolean;
  total_requests: number;
  total_bandwidth_up: number;
  total_bandwidth_down: number;
}

export interface ReplicationInfo {
  role: string;
  active: boolean;
  peers: number;
}

// ===== Settings =====

export interface AdminSettings {
  admin_email: string;
  admin_password?: string;
}

export interface AdminUser {
  id: string;
  email: string;
  role: string;
  created_at?: string;
}

export interface SecuritySettings {
  jwt_secret?: string;
  session_duration?: string;
  allow_origins?: string;
  cert_file?: string;
  key_file?: string;
}

export interface S3Settings {
  s3_enabled: boolean;
  s3_endpoint: string;
  s3_access_key: string;
  s3_secret_key?: string;
  s3_bucket_prefix: string;
  s3_use_ssl: boolean;
}

export interface SmtpSettings {
  email_provider: string;
  email_api_key?: string;
  email_api_secret?: string;
  smtp_host: string;
  smtp_port: number;
  smtp_user: string;
  smtp_password?: string;
  smtp_sender: string;
}

export interface ReplicationSettings {
  role: string;
  node_id: string;
  repl_addr: string;
  peers: string[];
  lease_ttl: string;
}

export interface EmailTemplate {
  type: string;
  subject: string;
  body: string;
  updated_at?: string;
}

export interface EmailLogEntry {
  id: string;
  timestamp: string;
  provider: string;
  from: string;
  to: string;
  subject: string;
  success: boolean;
  error?: string;
}

export interface Backup {
  name: string;
  size?: number;
  modified?: string;
  is_dir?: boolean;
}
