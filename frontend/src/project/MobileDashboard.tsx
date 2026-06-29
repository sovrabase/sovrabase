import { useState, useEffect, useCallback, useRef } from 'react';
import { api, formatBytes } from '../api';
import { useToast } from '../components/Toast';
import type { Project, DatabaseDocument } from '../types';
import { useDashboard } from '../store';

interface MobileDashboardProps {
  projectId: string;
  projectName: string;
  projects: Project[];
  onClose?: () => void;
  onChangeProject?: (id: string) => void;
}

interface LogEntry {
  timestamp: string;
  method: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'ERROR' | 'PATCH';
  path: string;
  status: number;
  duration: string;
}

interface UserRow {
  id: string;
  email: string;
  username?: string;
  name?: string;
  avatar_url?: string;
  role?: string;
  provider?: string;
  status?: 'Active' | 'Banned' | 'Pending';
}

interface CollectionInfo {
  name: string;
  doc_count?: number;
}

interface QueueInfo { name: string; visible: number; in_flight: number; total: number; }

const renderProviderIcon = (name: string, className: string = "w-4 h-4") => {
  const n = name.toLowerCase();
  if (n === 'google') {
    return (
      <svg className={className} viewBox="0 0 24 24">
        <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" />
        <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" />
        <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.06H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.94l2.85-2.22.81-.63z" />
        <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.06l3.66 2.84c.87-2.6 3.3-4.52 6.16-4.52z" />
      </svg>
    );
  }
  if (n === 'github') {
    return (
      <svg className={className} viewBox="0 0 24 24" fill="currentColor">
        <path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12"/>
      </svg>
    );
  }
  if (n === 'discord') {
    return (
      <svg className={className} viewBox="0 0 127.14 96.36" fill="#5865f2">
        <path d="M107.7,8.07A105.15,105.15,0,0,0,77.26,0a77.19,77.19,0,0,0-3.3,6.83A96.67,96.67,0,0,0,53.22,6.83,77.19,77.19,0,0,0,49.88,0,105.15,105.15,0,0,0,19.44,8.07C3.66,31.58-1.86,54.65,1,77.53A105.73,105.73,0,0,0,32,96.36a77.7,77.7,0,0,0,6.63-10.85,68.43,68.43,0,0,1-10.43-5c.87-.64,1.71-1.32,2.51-2a75.76,75.76,0,0,0,72.77,0c.8,0,1.64.69,2.51,2a68.43,68.43,0,0,1-10.43,5,77.7,77.7,0,0,0,6.63,10.85,105.73,105.73,0,0,0,31.06-18.83C129,54.65,122.68,31.58,107.7,8.07ZM42.45,65.69C36.18,65.69,31,60,31,53S36.18,40.36,42.45,40.36,53.83,46,53.83,53,48.72,65.69,42.45,65.69Zm42.24,0C78.41,65.69,73.24,60,73.24,53S78.41,40.36,84.69,40.36,96.07,46,96.07,53,91,65.69,84.69,65.69Z"/>
      </svg>
    );
  }
  if (n === 'microsoft') {
    return (
      <svg className={className} viewBox="0 0 23 23">
        <path fill="#f25022" d="M0 0h11v11H0z" />
        <path fill="#7fba00" d="M12 0h11v11H12z" />
        <path fill="#00a4ef" d="M0 12h11v11H0z" />
        <path fill="#ffb900" d="M12 12h11v11H12z" />
      </svg>
    );
  }
  if (n === 'gitlab') {
    return (
      <svg className={className} viewBox="0 0 500 500">
        <path fill="#e24329" d="M250 420.5l-95.2-293h190.4z" />
        <path fill="#fc6d26" d="M250 420.5l-95.2-293H33.2z" />
        <path fill="#fca326" d="M33.2 127.5L8.5 203.6c-7.9 24.3 1 51 21.8 66.1l219.7 150.8z" />
        <path fill="#e24329" d="M33.2 127.5h121.6L95.2 293z" />
        <path fill="#fc6d26" d="M250 420.5l95.2-293H466.8z" />
        <path fill="#fca326" d="M466.8 127.5l24.7 76.1c7.9 24.3-1 51-21.8 66.1L250 420.5z" />
        <path fill="#e24329" d="M466.8 127.5H345.2L404.8 293z" />
      </svg>
    );
  }
  if (n === 'facebook') {
    return (
      <svg className={className} viewBox="0 0 24 24" fill="#1877f2">
        <path d="M24 12.073c0-6.627-5.373-12-12-12s-12 5.373-12 12c0 5.99 4.388 10.954 10.125 11.854v-8.385H7.078v-3.47h3.047V9.43c0-3.007 1.792-4.669 4.533-4.669 1.312 0 2.686.235 2.686.235v2.953H15.83c-1.491 0-1.956.925-1.956 1.874v2.25h3.328l-.532 3.47h-2.796v8.385C19.612 23.027 24 18.062 24 12.073z" />
      </svg>
    );
  }
  if (n === 'twitter' || n === 'x') {
    return (
      <svg className={className} viewBox="0 0 24 24" fill="currentColor">
        <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
      </svg>
    );
  }
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="2" y="2" width="20" height="8" rx="2" ry="2"/>
      <rect x="2" y="14" width="20" height="8" rx="2" ry="2"/>
      <line x1="6" y1="6" x2="6.01" y2="6"/>
      <line x1="6" y1="18" x2="6.01" y2="18"/>
    </svg>
  );
};

export default function MobileDashboard({
  projectId,
  projectName,
  projects,
  onClose,
  onChangeProject,
}: MobileDashboardProps) {
  const { showToast } = useToast();
  const { stats } = useDashboard();
  const [activeTab, setActiveTab] = useState<'overview' | 'database' | 'auth' | 'logs' | 'more'>('overview');
  const [showProjectDropdown, setShowProjectDropdown] = useState(false);

  // Database Tab States
  const [collections, setCollections] = useState<CollectionInfo[]>([]);
  const [selectedCol, setSelectedCol] = useState<string>('');
  const [docs, setDocs] = useState<DatabaseDocument[]>([]);
  const [expandedDocId, setExpandedDocId] = useState<string | null>(null);

  const selectCol = (name: string) => {
    setSelectedCol(name);
    setExpandedDocId(null);
    setDbSearchQuery('');
  };
  const [dbSearchQuery, setDbSearchQuery] = useState('');

  // Auth Tab States
  const [authSubTab, setAuthSubTab] = useState<'users' | 'providers' | 'policies'>('users');
  const [users, setUsers] = useState<UserRow[]>([]);
  const [swipedUserId, setSwipedUserId] = useState<string | null>(null);
  const [userSearchQuery, setUserSearchQuery] = useState('');
  const [isInviting, setIsInviting] = useState(false);
  const [inviteEmail, setInviteEmail] = useState('');

  // Logs Tab States
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [isStreaming, setIsStreaming] = useState(true);
  const [logFilter, setLogFilter] = useState<'ALL' | '500' | '404'>('ALL');
  const streamTimer = useRef<any>(null);

  // Cron Jobs Tab States (Real API data)
  const [cronJobs, setCronJobs] = useState<any[]>([]);
  const [loadingJobs, setLoadingJobs] = useState(false);
  const [invokingStates, setInvokingStates] = useState<Record<string, 'idle' | 'loading' | 'success' | 'error'>>({});

  // Navigation & Sub-page Launcher States
  const [activeSubPage, setActiveSubPage] = useState<'grid' | 'storage' | 'team' | 'config' | 'webhooks' | 'cron' | 'queues' | 'integrations' | 'analytics' | 'api'>('grid');

  // Storage Tab States
  const [buckets, setBuckets] = useState<any[]>([]);
  const [selectedBucket, setSelectedBucket] = useState<string | null>(null);
  const [files, setFiles] = useState<any[]>([]);
  const [loadingBuckets, setLoadingBuckets] = useState(false);
  const [loadingFiles, setLoadingFiles] = useState(false);

  // Team Tab States
  const [members, setMembers] = useState<any[]>([]);
  const [loadingMembers, setLoadingMembers] = useState(false);

  // Webhooks Tab States
  const [webhooks, setWebhooks] = useState<any[]>([]);
  const [loadingWebhooks, setLoadingWebhooks] = useState(false);

  // Config Tab States
  const [projectDetail, setProjectDetail] = useState<any>(null);
  const [savingConfig, setSavingConfig] = useState(false);
  const [configEntries, setConfigEntries] = useState<any[]>([]);
  const [loadingConfigEntries, setLoadingConfigEntries] = useState(false);

  // Analytics Tab States
  const [projectUsage, setProjectUsage] = useState<any>(null);
  const [analyticsData, setAnalyticsData] = useState<any>(null);
  const [dbAnalysis, setDbAnalysis] = useState<any>(null);
  const [analyzing, setAnalyzing] = useState(false);

  // Queues Tab States (real API data)
  const [queues, setQueues] = useState<QueueInfo[]>([]);
  const [loadingQueues, setLoadingQueues] = useState(false);

  // API Tab States
  const [apiLanguage, setApiLanguage] = useState<'js' | 'dart' | 'curl'>('js');

  // Auth Provider Editor State
  interface ProviderConfig { 
    name?: string; 
    client_id: string; 
    client_secret: string; 
    scopes: string | string[]; 
    enabled: boolean; 
    auth_url?: string; 
    token_url?: string; 
    userinfo_url?: string; 
  }
  const [editingProvider, setEditingProvider] = useState<string | null>(null);
  const [providerConfig, setProviderConfig] = useState<ProviderConfig>({ name: '', client_id: '', client_secret: '', scopes: [], enabled: false, auth_url: '', token_url: '', userinfo_url: '' });
  const [authProviders, setAuthProviders] = useState<any[]>([]);

  // Auth Policy Editor State
  const [editingPolicyCol, setEditingPolicyCol] = useState<string | null>(null);
  const [policyForm, setPolicyForm] = useState({ get: '', list: '', create: '', update: '', delete: '', enabled: false });

  // Document Editor States
  const [editingDocId, setEditingDocId] = useState<string | null>(null);
  const [editingDocJson, setEditingDocJson] = useState<string>('');
  const [savingDocJson, setSavingDocJson] = useState(false);

  // Integrations Tab States
  const [integrationsCatalog, setIntegrationsCatalog] = useState<any[]>([]);
  const [enabledIntegrations, setEnabledIntegrations] = useState<any[]>([]);
  const [loadingIntegrations, setLoadingIntegrations] = useState(false);
  const [editingIntegration, setEditingIntegration] = useState<any | null>(null);
  const [integrationConfig, setIntegrationConfig] = useState<Record<string, string>>({});
  const [integrationEvents, setIntegrationEvents] = useState<string[]>([]);
  const [integrationCollections, setIntegrationCollections] = useState('');
  const [savingIntegration, setSavingIntegration] = useState(false);

  // Dialog / Modal Form States
  const [activeModal, setActiveModal] = useState<'create_bucket' | 'upload_file' | 'invite_member' | 'add_webhook' | 'create_cron' | 'add_document' | 'import_json' | 'create_collection' | 'add_config' | null>(null);
  const [modalData, setModalData] = useState<Record<string, string>>({});

  // Load Database Collections
  const loadCollections = useCallback(async () => {
    try {
      const res = await api<{ collections: (string | CollectionInfo)[] }>(`/admin/projects/${encodeURIComponent(projectId)}/collections`);
      const mapped = (res.collections || []).map(c => typeof c === 'string' ? { name: c } : c);
      setCollections(mapped);
      if (mapped.length > 0) {
        setSelectedCol(mapped[0].name);
      } else {
        setSelectedCol('');
      }
    } catch (e) {
      setCollections([]);
      setSelectedCol('');
    }
  }, [projectId]);

  // Load Database Documents
  const loadDocuments = useCallback(async (colName: string) => {
    if (!colName) return;
    try {
      const data = await api<DatabaseDocument[]>(`/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(colName)}/documents`);
      setDocs(Array.isArray(data) ? data : []);
    } catch (e) {
      setDocs([]);
    }
  }, [projectId]);

  // Load Auth Users
  const loadUsers = useCallback(async () => {
    try {
      const data = await api<any[]>(`/admin/projects/${encodeURIComponent(projectId)}/users`);
      const mapped: UserRow[] = (data || []).map(u => ({
        id: u.id || u._id,
        email: u.email,
        username: u.username,
        name: u.name,
        avatar_url: u.avatar_url,
        role: u.role || 'user',
        provider: u._metadata?.[0]?.provider || 'email',
        status: 'Active'
      }));
      setUsers(mapped);
    } catch (e) {
      setUsers([]);
    }
  }, [projectId]);

  // Load and Stream Realtime Logs (From Server API logs)
  const loadInitialLogs = useCallback(async () => {
    try {
      const data = await api<{ entries: LogEntry[]; total: number }>(`/admin/projects/${encodeURIComponent(projectId)}/logs?limit=50`);
      setLogs(data.entries || []);
    } catch (e) {
      setLogs([]);
    }
  }, [projectId]);

  // Set up background log streaming from real API
  useEffect(() => {
    if (isStreaming) {
      streamTimer.current = setInterval(async () => {
        try {
          const data = await api<{ entries: LogEntry[]; total: number }>(`/admin/projects/${encodeURIComponent(projectId)}/logs?limit=50`);
          setLogs(data.entries || []);
        } catch (e) {
          // ignore
        }
      }, 3000);
    } else {
      if (streamTimer.current) {
        clearInterval(streamTimer.current);
      }
    }

    return () => {
      if (streamTimer.current) {
        clearInterval(streamTimer.current);
      }
    };
  }, [isStreaming, projectId]);

  // Load Scheduled Cron Jobs
  const loadCronJobs = useCallback(async () => {
    setLoadingJobs(true);
    try {
      const res = await api<{ data: any[] }>(`/admin/projects/${encodeURIComponent(projectId)}/cron`);
      setCronJobs(res.data || []);
    } catch (e) {
      setCronJobs([]);
    }
    setLoadingJobs(false);
  }, [projectId]);

  // Load Storage Buckets
  const loadBuckets = useCallback(async () => {
    setLoadingBuckets(true);
    try {
      const res = await api<any[]>(`/admin/projects/${encodeURIComponent(projectId)}/storage/buckets`);
      setBuckets(Array.isArray(res) ? res : []);
    } catch {
      setBuckets([]);
    }
    setLoadingBuckets(false);
  }, [projectId]);

  const loadFiles = useCallback(async (bucketName: string) => {
    setLoadingFiles(true);
    try {
      const res = await api<any[]>(`/admin/projects/${encodeURIComponent(projectId)}/storage/buckets/${encodeURIComponent(bucketName)}/files`);
      setFiles(Array.isArray(res) ? res : []);
    } catch {
      setFiles([]);
    }
    setLoadingFiles(false);
  }, [projectId]);

  // Load Team Members
  const loadMembers = useCallback(async () => {
    setLoadingMembers(true);
    try {
      const res = await api<{ members: any[] }>(`/admin/projects/${encodeURIComponent(projectId)}/members`);
      setMembers(res.members || []);
    } catch {
      setMembers([]);
    }
    setLoadingMembers(false);
  }, [projectId]);

  // Change member role (mobile)
  const changeMemberRole = async (userId: string, newRole: string) => {
    const currentUserId = localStorage.getItem('sovrabase_admin_user_id');
    if (userId === currentUserId) {
      showToast('You cannot change your own role', 'error');
      return;
    }
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/members/${encodeURIComponent(userId)}/role`, {
        method: 'PUT',
        body: JSON.stringify({ role: newRole }),
      });
      setMembers(prev => prev.map(m => m.user_id === userId ? { ...m, role: newRole } : m));
      showToast('Role updated', 'success');
    } catch (err: any) {
      showToast(err.message || 'Failed to update role', 'error');
    }
  };

  // Load Webhooks
  const loadWebhooks = useCallback(async () => {
    setLoadingWebhooks(true);
    try {
      const res = await api<{ data: any[] }>(`/admin/projects/${encodeURIComponent(projectId)}/webhooks`);
      setWebhooks(res.data || []);
    } catch {
      setWebhooks([]);
    }
    setLoadingWebhooks(false);
  }, [projectId]);

  // Load Message Queues
  const loadQueues = useCallback(async () => {
    setLoadingQueues(true);
    try {
      const res = await api<{ data: QueueInfo[]; count: number }>(`/admin/projects/${encodeURIComponent(projectId)}/queues`);
      setQueues(res.data || []);
    } catch {
      setQueues([]);
    }
    setLoadingQueues(false);
  }, [projectId]);

  // Load Auth Providers Config
  const loadAuthProviders = useCallback(async () => {
    try {
      const res = await api<{ providers: any[] }>(`/admin/projects/${encodeURIComponent(projectId)}/auth/providers`);
      setAuthProviders(res.providers || []);
    } catch {
      setAuthProviders([]);
    }
  }, [projectId]);

  // Load Project Detail & Analysis
  const loadProjectConfig = useCallback(async () => {
    try {
      const res = await api<any>(`/admin/projects/${encodeURIComponent(projectId)}`);
      setProjectDetail(res);
    } catch {
      setProjectDetail(null);
    }
  }, [projectId]);

  // Load Config Key-Value Entries
  const loadConfigEntries = useCallback(async () => {
    setLoadingConfigEntries(true);
    try {
      const res = await api<{ data: any[] }>(`/admin/projects/${encodeURIComponent(projectId)}/config`);
      setConfigEntries(res.data || []);
    } catch {
      setConfigEntries([]);
    }
    setLoadingConfigEntries(false);
  }, [projectId]);

  const loadProjectUsage = useCallback(async () => {
    try {
      const res = await api<any>(`/admin/projects/${encodeURIComponent(projectId)}/usage`);
      setProjectUsage(res);
    } catch {
      setProjectUsage(null);
    }
  }, [projectId]);

  const loadAnalytics = useCallback(async () => {
    try {
      const res = await api<any>(`/admin/projects/${encodeURIComponent(projectId)}/analytics`);
      setAnalyticsData(res);
    } catch {
      setAnalyticsData(null);
    }
  }, [projectId]);

  const loadIntegrations = useCallback(async () => {
    setLoadingIntegrations(true);
    try {
      const [cat, proj] = await Promise.all([
        api<{ integrations: any[] }>('/admin/integrations/catalog'),
        api<{ integrations: any[] }>(`/admin/projects/${encodeURIComponent(projectId)}/integrations`),
      ]);
      setIntegrationsCatalog(cat.integrations || []);
      setEnabledIntegrations(proj.integrations || []);
    } catch {
      setIntegrationsCatalog([]);
      setEnabledIntegrations([]);
    }
    setLoadingIntegrations(false);
  }, [projectId]);

  // Load everything on project/tab change
  useEffect(() => {
    loadCollections();
    loadUsers();
    loadInitialLogs();
    loadCronJobs();
    loadBuckets();
    loadMembers();
    loadWebhooks();
    loadQueues();
    loadAuthProviders();
    loadProjectConfig();
    loadConfigEntries();
    loadProjectUsage();
    loadIntegrations();
  }, [projectId, loadCollections, loadUsers, loadInitialLogs, loadCronJobs, loadBuckets, loadMembers, loadWebhooks, loadQueues, loadAuthProviders, loadProjectConfig, loadConfigEntries, loadProjectUsage, loadIntegrations]);

  // Load analytics when user enters analytics sub-page
  useEffect(() => {
    if (activeSubPage === 'analytics') {
      loadAnalytics();
    }
  }, [activeSubPage, loadAnalytics]);

  useEffect(() => {
    if (selectedCol) {
      loadDocuments(selectedCol);
    }
  }, [selectedCol, loadDocuments]);

  // Invoke Action (real callback fetch request)
  const handleInvoke = async (job: any) => {
    setInvokingStates(prev => ({ ...prev, [job.id]: 'loading' }));
    try {
      const controller = new AbortController();
      const id = setTimeout(() => controller.abort(), 8000); // 8s timeout
      
      const response = await fetch(job.url, {
        method: job.method || 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: job.method !== 'GET' && job.body ? JSON.stringify(job.body) : undefined,
        signal: controller.signal
      });
      clearTimeout(id);
      
      if (response.ok) {
        setInvokingStates(prev => ({ ...prev, [job.id]: 'success' }));
        showToast('Triggered successfully', 'success');
      } else {
        setInvokingStates(prev => ({ ...prev, [job.id]: 'error' }));
        showToast(`Trigger failed: HTTP ${response.status}`, 'error');
      }
    } catch (err) {
      setInvokingStates(prev => ({ ...prev, [job.id]: 'error' }));
      showToast('Connection failed', 'error');
    }
    
    setTimeout(() => {
      setInvokingStates(prev => ({ ...prev, [job.id]: 'idle' }));
    }, 3000);
  };

  // Queue actions
  const unstickQueue = async (queueName: string) => {
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/queues/${encodeURIComponent(queueName)}/make-visible`, { method: 'POST' });
      showToast(`Queue "${queueName}" unstuck`, 'success');
      loadQueues();
    } catch (err) { showToast((err as Error).message, 'error'); }
  };

  const purgeQueue = async (queueName: string) => {
    if (!confirm(`Purge all messages from "${queueName}"?`)) return;
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/queues/purge`, { method: 'POST', body: JSON.stringify({ queue: queueName }) });
      showToast(`Queue "${queueName}" purged`, 'success');
      loadQueues();
    } catch (err) { showToast((err as Error).message, 'error'); }
  };

  // Clear logs handler
  const handleClearLogs = async () => {
    if (!confirm('Clear all logs for this project?')) return;
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/logs`, { method: 'DELETE' });
      setLogs([]);
      showToast('Logs cleared successfully', 'success');
    } catch (err) {
      showToast((err as Error).message || 'Failed to clear logs', 'error');
    }
  };

  // Deep DB Analysis
  const runDbAnalysis = async () => {
    setAnalyzing(true);
    try {
      const data = await api<any>(`/admin/projects/${encodeURIComponent(projectId)}/db-analysis`);
      setDbAnalysis(data);
    } catch (err) {
      showToast((err as Error).message || 'DB analysis failed', 'error');
    } finally {
      setAnalyzing(false);
    }
  };

  // BAN action (actually Deletes user from DB)
  const handleBanUser = async (userId: string, email?: string) => {
    if (!confirm(`Are you sure you want to delete user ${email || userId}?`)) {
      setSwipedUserId(null);
      return;
    }
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/users/${encodeURIComponent(userId)}`, {
        method: 'DELETE'
      });
      showToast('User deleted', 'success');
      setUsers(prev => prev.filter(u => u.id !== userId));
    } catch (err) {
      showToast((err as Error).message || 'Delete failed', 'error');
    }
    setSwipedUserId(null);
  };

  // RESET Password action
  const handleResetUser = async (userId: string, email?: string) => {
    const newPass = Math.random().toString(36).slice(-8);
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/users/${encodeURIComponent(userId)}/password`, {
        method: 'POST',
        body: JSON.stringify({ password: newPass })
      });
      alert(`Password reset for ${email || userId}!\nNew temporary password: ${newPass}`);
      showToast('Password reset successfully', 'success');
    } catch (err) {
      showToast((err as Error).message || 'Reset failed', 'error');
    }
    setSwipedUserId(null);
  };

  // Invite User (Real API call)
  const handleInviteUser = async () => {
    if (!inviteEmail.trim()) return;
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/users`, {
        method: 'POST',
        body: JSON.stringify({ email: inviteEmail.trim(), username: inviteEmail.trim().split('@')[0], password: Math.random().toString(36).slice(-8), role: 'user' })
      });
      showToast('User invited', 'success');
      loadUsers();
      setInviteEmail('');
      setIsInviting(false);
    } catch (err) {
      showToast((err as Error).message || 'Invite failed', 'error');
    }
  };

  // Save OAuth Provider config
  const handleSaveProvider = async () => {
    if (!editingProvider) return;
    try {
      const name = editingProvider === 'custom' ? providerConfig.name?.trim() : editingProvider;
      if (!name) {
        showToast('Provider ID / Name is required', 'error');
        return;
      }
      
      const existingIdx = authProviders.findIndex(p => p.name === name);
      const updatedItem = {
        name: name,
        client_id: providerConfig.client_id.trim(),
        client_secret: providerConfig.client_secret.trim(),
        scopes: Array.isArray(providerConfig.scopes) ? providerConfig.scopes : String(providerConfig.scopes).split(',').map(s => s.trim()).filter(Boolean),
        enabled: providerConfig.enabled,
        auth_url: providerConfig.auth_url?.trim() || '',
        token_url: providerConfig.token_url?.trim() || '',
        userinfo_url: providerConfig.userinfo_url?.trim() || ''
      };
      
      let newList = [...authProviders];
      if (existingIdx >= 0) {
        newList[existingIdx] = updatedItem;
      } else {
        newList.push(updatedItem);
      }
      
      await api(`/admin/projects/${encodeURIComponent(projectId)}/auth/providers`, {
        method: 'PUT',
        body: JSON.stringify({ providers: newList })
      });
      showToast('Auth provider saved', 'success');
      setEditingProvider(null);
      loadAuthProviders();
    } catch (err) {
      showToast('Failed to save provider config', 'error');
    }
  };

  // Load rules for selected policy collection
  const handleSelectPolicyCol = async (colName: string) => {
    setEditingPolicyCol(colName);
    try {
      const res = await api<any>(`/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(colName)}/rules`);
      setPolicyForm({
        get: res.rules?.get || '',
        list: res.rules?.list || '',
        create: res.rules?.create || '',
        update: res.rules?.update || '',
        delete: res.rules?.delete || '',
        enabled: res.enabled || false
      });
    } catch {
      setPolicyForm({ get: '', list: '', create: '', update: '', delete: '', enabled: false });
    }
  };

  // Save RLS policies
  const handleSavePolicies = async () => {
    if (!editingPolicyCol) return;
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(editingPolicyCol)}/rules`, {
        method: 'POST',
        body: JSON.stringify({
          enabled: policyForm.enabled,
          rules: {
            get: policyForm.get,
            list: policyForm.list,
            create: policyForm.create,
            update: policyForm.update,
            delete: policyForm.delete
          }
        })
      });
      showToast('RLS policies updated', 'success');
      setEditingPolicyCol(null);
    } catch (err) {
      showToast('Failed to save rules', 'error');
    }
  };

  // Save Document JSON
  const handleSaveDocJson = async () => {
    if (!editingDocId || !selectedCol) return;
    setSavingDocJson(true);
    try {
      const parsed = JSON.parse(editingDocJson);
      await api(`/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(selectedCol)}/documents/${encodeURIComponent(editingDocId)}`, {
        method: 'PUT',
        body: JSON.stringify(parsed)
      });
      showToast('Document saved successfully', 'success');
      setEditingDocId(null);
      loadDocuments(selectedCol);
    } catch (err) {
      showToast((err as Error).message || 'Invalid JSON format', 'error');
    }
    setSavingDocJson(false);
  };

  // Save Project Integration
  const handleSaveIntegration = async () => {
    if (!editingIntegration) return;
    setSavingIntegration(true);
    try {
      const config: Record<string, unknown> = {};
      for (const field of editingIntegration.config_fields) {
        const val = integrationConfig[field.key] ?? '';
        if (field.type === 'boolean') {
          config[field.key] = val === 'true' || val === 'true';
        } else if (field.type === 'number') {
          config[field.key] = val === '' ? 0 : Number(val);
        } else {
          config[field.key] = val;
        }
      }

      const integration = {
        id: editingIntegration.id,
        config,
        ...(editingIntegration.supports_triggers ? {
          events: integrationEvents,
          collections: integrationCollections.split(',').map(s => s.trim()).filter(Boolean)
        } : {})
      };

      let newList = [...enabledIntegrations];
      const idx = enabledIntegrations.findIndex(e => e.id === editingIntegration.id);
      if (idx >= 0) {
        newList[idx] = integration;
      } else {
        newList.push(integration);
      }

      await api(`/admin/projects/${encodeURIComponent(projectId)}/integrations`, {
        method: 'PUT',
        body: JSON.stringify({ integrations: newList })
      });
      showToast(`${editingIntegration.name} integration saved`, 'success');
      setEditingIntegration(null);
      loadIntegrations();
    } catch (err) {
      showToast((err as Error).message || 'Failed to save integration', 'error');
    }
    setSavingIntegration(false);
  };

  // JSON Syntax highlighting helper
  const renderFormattedJson = (obj: any) => {
    const jsonStr = JSON.stringify(obj, null, 2);
    return (
      <pre className="font-code-xs text-code-xs leading-relaxed overflow-x-auto p-3 rounded-lg bg-surface-container-lowest/70 border border-outline-variant/30 text-left">
        <code>
          {jsonStr.split('\n').map((line, index) => {
            // Very simple token matching for highlighting
            let renderedLine = line;
            if (line.includes('":')) {
              const parts = line.split('":');
              const key = parts[0];
              const val = parts.slice(1).join('":');
              return (
                <div key={index}>
                  <span className="text-[#a5b4fc]">{key}"</span>:
                  <span className="text-primary">{val}</span>
                </div>
              );
            }
            return <div key={index} className="text-on-surface-variant/80">{renderedLine}</div>;
          })}
        </code>
      </pre>
    );
  };

  // --- Filtering ---
  const filteredUsers = users.filter(u => 
    (u.email || '').toLowerCase().includes(userSearchQuery.toLowerCase()) || 
    (u.name || '').toLowerCase().includes(userSearchQuery.toLowerCase()) ||
    (u.role || '').toLowerCase().includes(userSearchQuery.toLowerCase())
  );

  const filteredDocs = docs.filter(d => {
    const jsonStr = JSON.stringify(d).toLowerCase();
    return jsonStr.includes(dbSearchQuery.toLowerCase());
  });

  const filteredLogs = logs.filter(l => {
    if (logFilter === '500') return l.status >= 500 || l.method === 'ERROR';
    if (logFilter === '404') return l.status === 404;
    return true;
  });

  return (
    <div className="flex flex-col h-full bg-surface text-on-surface select-none font-body-md relative overflow-hidden">
      
      {/* HEADER */}
      <header className="absolute top-0 left-0 right-0 z-40 bg-surface/85 backdrop-blur-xl border-b border-outline-variant flex justify-between items-center h-16 px-container-padding">
        <div className="relative">
          <button 
            onClick={() => setShowProjectDropdown(!showProjectDropdown)}
            className="flex items-center gap-2 text-left focus:outline-none hover:opacity-90 active:scale-98 transition-all"
          >
            <span className="material-symbols-outlined text-primary font-bold text-[22px]" data-icon="layers">layers</span>
            <div>
              <h1 className="font-headline-md text-sm font-extrabold text-on-surface leading-tight flex items-center gap-1">
                {projectName}
                <span className="material-symbols-outlined text-xs text-on-surface-variant/70" data-icon="arrow_drop_down">arrow_drop_down</span>
              </h1>
              <p className="text-[10px] text-on-surface-variant/60 font-code-xs tracking-wider uppercase">Active Project</p>
            </div>
          </button>

          {/* PROJECT SELECTOR DROPDOWN */}
          {showProjectDropdown && (
            <>
              <div className="fixed inset-0 z-40" onClick={() => setShowProjectDropdown(false)}></div>
              <div className="absolute top-12 left-0 w-56 bg-surface-container-high border border-outline-variant rounded-xl shadow-2xl z-50 overflow-hidden py-1 animate-fade-slide">
                <p className="px-3 py-1.5 text-[10px] font-label-caps text-on-surface-variant/50 uppercase tracking-widest border-b border-outline-variant/30">Switch Project</p>
                {projects.map((p) => (
                  <button
                    key={p.id}
                    onClick={() => {
                      if (onChangeProject) onChangeProject(p.id);
                      setShowProjectDropdown(false);
                    }}
                    className={`w-full text-left px-3 py-2.5 text-xs font-semibold flex items-center justify-between hover:bg-surface-container-highest transition-colors ${p.id === projectId ? 'text-primary bg-primary/5' : 'text-on-surface-variant'}`}
                  >
                    <span>{p.name}</span>
                    {p.id === projectId && <span className="material-symbols-outlined text-sm" data-icon="check">check</span>}
                  </button>
                ))}
              </div>
            </>
          )}
        </div>

        <div className="flex items-center gap-2">
          {activeTab === 'logs' && (
            <button 
              onClick={() => setIsStreaming(!isStreaming)}
              className="p-1.5 rounded-lg hover:bg-surface-container-high/50 text-primary active:scale-90 transition-all flex items-center justify-center"
              title={isStreaming ? "Pause Log Stream" : "Resume Log Stream"}
            >
              <span className="material-symbols-outlined text-[20px]">
                {isStreaming ? 'pause' : 'play_arrow'}
              </span>
            </button>
          )}
          
          <span className="px-2.5 py-1 bg-primary-container/10 text-primary-fixed-dim font-label-caps text-[10px] border border-primary-container/20 rounded-full flex items-center gap-1">
            <span className="w-1.5 h-1.5 rounded-full bg-primary status-pulse"></span>
            Healthy
          </span>

          {onClose && (
            <button 
              onClick={onClose} 
              className="ml-1 p-1 hover:bg-surface-container-high rounded text-on-surface-variant hover:text-on-surface"
              title="Close mobile preview"
            >
              <span className="material-symbols-outlined text-[20px]">close</span>
            </button>
          )}
        </div>
      </header>

      {/* MAIN CONTENT PORT */}
      <main className="flex-1 pt-20 pb-24 overflow-y-auto no-scrollbar bg-background">
        
        {/* OVERVIEW TAB */}
        {activeTab === 'overview' && (
          <div className="p-container-padding space-y-stack-lg animate-fade-slide">
            {/* Project Quick Info */}
            <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-3 relative overflow-hidden emerald-glow">
              <div className="flex justify-between items-start">
                <div>
                  <h2 className="text-base font-extrabold text-on-surface">{projectName}</h2>
                  <p className="text-xs text-on-surface-variant/70 font-code-xs select-text">ID: {projectId}</p>
                </div>
                <span className="px-2 py-0.5 bg-primary/10 text-primary-fixed-dim font-code-xs text-[10px] border border-primary/20 rounded">
                  {projectDetail?.status || 'Active'}
                </span>
              </div>
              <div className="border-t border-outline-variant/30 pt-3 flex flex-col gap-2">
                <div className="flex justify-between text-xs">
                  <span className="text-on-surface-variant/70">Database API</span>
                  <span className="font-code-xs text-primary truncate max-w-[180px] text-right select-text">/api/v1/collections/</span>
                </div>
                <div className="flex justify-between text-xs">
                  <span className="text-on-surface-variant/70">Region</span>
                  <span className="font-code-xs uppercase text-secondary">{stats?.region || 'eu-west'}</span>
                </div>
              </div>
            </div>

            {/* Bento Statistics */}
            <div className="grid grid-cols-2 gap-stack-sm">
              <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-2">
                <p className="font-label-caps text-[10px] text-on-surface-variant uppercase tracking-wider">Collections</p>
                <p className="font-headline-lg text-xl font-bold text-primary">{collections.length}</p>
                <p className="font-code-xs text-[10px] text-on-surface-variant/60">JSON collections</p>
              </div>
              <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-2">
                <p className="font-label-caps text-[10px] text-on-surface-variant uppercase tracking-wider">Registered Users</p>
                <p className="font-headline-lg text-xl font-bold text-secondary">{users.length}</p>
                <p className="font-code-xs text-[10px] text-primary">Auth database</p>
              </div>
            </div>

            {/* Live Metrics from projectUsage */}
            <div className="space-y-3.5">
              <h3 className="font-label-caps text-[10px] text-on-surface-variant uppercase tracking-widest">Live Metrics</h3>
              <div className="grid grid-cols-2 gap-stack-sm">
                <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-1">
                  <span className="text-[8px] font-bold text-on-surface-variant uppercase block">API Requests</span>
                  <span className="text-md font-bold text-primary font-mono">{projectUsage?.api_requests || 0}</span>
                  <span className="text-[8px] text-on-surface-variant/50">Hits since creation</span>
                </div>
                <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-1">
                  <span className="text-[8px] font-bold text-on-surface-variant uppercase block">DB Reads</span>
                  <span className="text-md font-bold text-primary font-mono">{projectUsage?.db_reads_total || 0}</span>
                  <span className="text-[8px] text-on-surface-variant/50">Reads from disk</span>
                </div>
                <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-1">
                  <span className="text-[8px] font-bold text-on-surface-variant uppercase block">DB Writes</span>
                  <span className="text-md font-bold text-secondary font-mono">{projectUsage?.db_writes_total || 0}</span>
                  <span className="text-[8px] text-on-surface-variant/50">Writes to disk</span>
                </div>
                <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-1">
                  <span className="text-[8px] font-bold text-on-surface-variant uppercase block">Realtime Conn.</span>
                  <span className="text-md font-bold text-secondary font-mono">{projectUsage?.realtime_connections || 0}</span>
                  <span className="text-[8px] text-on-surface-variant/50">Active WebSockets</span>
                </div>
              </div>
              {!projectUsage && (
                <p className="text-[10px] text-on-surface-variant/50 italic text-center">Loading metrics...</p>
              )}
            </div>

            {/* Deep DB Analysis */}
            <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-3">
              <div className="flex items-center justify-between">
                <h3 className="font-label-caps text-[10px] text-on-surface-variant uppercase tracking-widest">Storage Analysis</h3>
                <button
                  onClick={runDbAnalysis}
                  disabled={analyzing}
                  className="flex items-center gap-1 px-2.5 py-1 rounded text-[10px] font-bold uppercase tracking-wider bg-primary/10 text-primary border border-primary/20 hover:bg-primary/20 active:scale-95 transition-all disabled:opacity-50"
                >
                  {analyzing ? (
                    <>
                      <span className="w-3 h-3 border-2 border-primary border-t-transparent rounded-full animate-spin" />
                      Analyzing...
                    </>
                  ) : (
                    'Run Analysis'
                  )}
                </button>
              </div>
              {dbAnalysis && (
                <div className="space-y-2">
                  <div className="flex items-center gap-2 text-on-surface-variant text-[10px] font-mono">
                    Total: <span className="text-primary font-bold">{formatBytes(dbAnalysis.total_size)}</span>
                  </div>
                  {dbAnalysis.collections && Object.keys(dbAnalysis.collections).length > 0 && (
                    <div className="space-y-1.5">
                      {Object.entries(dbAnalysis.collections).slice(0, 5).map(([name, c]: [string, any]) => (
                        <div key={name} className="flex justify-between items-center bg-surface-container-high rounded px-2 py-1">
                          <span className="text-[10px] font-mono text-on-surface truncate max-w-[120px]">{name}</span>
                          <div className="flex gap-2 text-[9px] font-mono">
                            <span className="text-on-surface-variant/70">{c.document_count?.toLocaleString()} docs</span>
                            <span className="text-primary">{formatBytes(c.document_size)}</span>
                          </div>
                        </div>
                      ))}
                      {Object.keys(dbAnalysis.collections).length > 5 && (
                        <p className="text-[9px] text-on-surface-variant/50 text-center italic">
                          +{Object.keys(dbAnalysis.collections).length - 5} more collections
                        </p>
                      )}
                    </div>
                  )}
                  <div className="flex gap-3 text-[9px] font-mono text-on-surface-variant/60">
                    <span>Metadata: {formatBytes(dbAnalysis.metadata_size)}</span>
                    <span>Index: {formatBytes(dbAnalysis.index_size)}</span>
                  </div>
                </div>
              )}
              {!dbAnalysis && !analyzing && (
                <p className="text-[9px] text-on-surface-variant/50 italic">Tap "Run Analysis" to scan database storage</p>
              )}
            </div>

            {/* Quick action button */}
            <button 
              onClick={() => { setActiveTab('more'); setActiveSubPage('api'); }}
              className="w-full bg-primary text-on-primary py-3 rounded-lg font-label-caps text-xs uppercase tracking-widest hover:brightness-110 transition-all active:scale-[0.98] font-bold flex justify-center items-center gap-2"
            >
              <span>View API Docs</span>
              <span className="material-symbols-outlined text-sm">arrow_forward</span>
            </button>
          </div>
        )}

        {/* DATABASE TAB */}
        {activeTab === 'database' && (
          <div className="p-container-padding space-y-stack-md animate-fade-slide">
            
            {/* Header info */}
            <div className="flex justify-between items-center mb-1">
              <span className="text-on-surface-variant font-label-caps text-[10px] tracking-widest uppercase">Document Collections</span>
              <div className="flex items-center gap-2">
                <span className="px-2 py-0.5 bg-surface-container-high text-on-surface-variant font-code-xs text-[10px] rounded border border-outline-variant/30">
                  {collections.length} Tables
                </span>
                <button
                  onClick={() => { setActiveModal('create_collection'); setModalData({}); }}
                  className="flex items-center gap-0.5 px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider bg-primary/10 text-primary border border-primary/20 hover:bg-primary/20 active:scale-95 transition-all"
                >
                  <span className="material-symbols-outlined text-[14px]">add</span>
                  <span>New Table</span>
                </button>
              </div>
            </div>

            {/* Collection Chips Selector */}
            <div className="flex gap-2 overflow-x-auto no-scrollbar pb-1">
              {collections.map(c => (
                <button
                  key={c.name}
                  onClick={() => selectCol(c.name)}
                  className={`px-3 py-1.5 rounded-full text-xs font-semibold uppercase whitespace-nowrap tracking-wider border transition-all active:scale-95 ${
                    selectedCol === c.name 
                      ? 'bg-primary/10 border-primary text-primary' 
                      : 'bg-surface-container-high border-outline-variant text-on-surface-variant hover:text-on-surface'
                  }`}
                >
                  {c.name}
                </button>
              ))}
            </div>

            {/* Database actions & Search */}
            <div className="flex gap-2 flex-wrap">
              <div className="relative flex-1 min-w-[120px]">
                <span className="absolute inset-y-0 left-0 pl-2.5 flex items-center pointer-events-none">
                  <span className="material-symbols-outlined text-[16px] text-on-surface-variant/50">search</span>
                </span>
                <input
                  type="text"
                  placeholder={`Search ${selectedCol}...`}
                  value={dbSearchQuery}
                  onChange={(e) => setDbSearchQuery(e.target.value)}
                  className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg pl-8 pr-3 py-1.5 text-xs text-on-surface placeholder-on-surface-variant/40 focus:outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/20"
                />
              </div>
              <button 
                onClick={() => { setActiveModal('add_document'); setModalData({ docJson: '{\n  "title": "New Document"\n}' }); }}
                disabled={!selectedCol}
                className="bg-primary hover:bg-primary-fixed-dim text-on-primary font-bold px-3.5 py-1.5 rounded-lg text-xs flex items-center gap-1 active:scale-95 transition-all disabled:opacity-50"
              >
                <span className="material-symbols-outlined text-sm">add</span>
                <span>Row</span>
              </button>
              <button 
                onClick={() => { setActiveModal('import_json'); setModalData({ importJson: '' }); }}
                disabled={!selectedCol}
                className="bg-secondary/10 text-secondary border border-secondary/20 hover:bg-secondary/20 font-bold px-3.5 py-1.5 rounded-lg text-xs flex items-center gap-1 active:scale-95 transition-all disabled:opacity-50"
              >
                <span className="material-symbols-outlined text-sm">file_upload</span>
                <span>Import</span>
              </button>
            </div>

            {/* Collection Actions: Clear & Drop */}
            {selectedCol && (
              <div className="flex gap-2">
                <button
                  onClick={async () => {
                    if (!confirm(`Clear ALL documents from "${selectedCol}"? This cannot be undone.`)) return;
                    try {
                      await api(`/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(selectedCol)}/clear`, { method: 'POST' });
                      showToast(`Collection "${selectedCol}" cleared`, 'success');
                      loadDocuments(selectedCol);
                      loadCollections();
                    } catch (err) {
                      showToast('Failed to clear collection', 'error');
                    }
                  }}
                  className="px-2.5 py-1 rounded text-[10px] font-bold uppercase tracking-wider text-on-surface-variant border border-outline-variant/60 hover:bg-surface-container-high active:scale-95 transition-all flex items-center gap-1"
                >
                  <span className="material-symbols-outlined text-[14px]">layers_clear</span>
                  Clear
                </button>
                <button
                  onClick={async () => {
                    if (!confirm(`DROP collection "${selectedCol}"? This permanently deletes the collection and all its documents.`)) return;
                    try {
                      await api(`/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(selectedCol)}`, { method: 'DELETE' });
                      showToast(`Collection "${selectedCol}" dropped`, 'success');
                      setSelectedCol('');
                      loadCollections();
                    } catch (err) {
                      showToast('Failed to drop collection', 'error');
                    }
                  }}
                  className="px-2.5 py-1 rounded text-[10px] font-bold uppercase tracking-wider text-error border border-error/30 hover:bg-error-container/10 active:scale-95 transition-all flex items-center gap-1"
                >
                  <span className="material-symbols-outlined text-[14px]">delete_forever</span>
                  Drop
                </button>
              </div>
            )}

            {/* stacked card-components with expandable height */}
            {editingDocId ? (
              /* Inline Document JSON Editor */
              <div className="bg-surface-container border border-outline-variant p-stack-md rounded-xl space-y-4 animate-fade-slide">
                <div className="flex justify-between items-center pb-2 border-b border-outline-variant/30">
                  <h3 className="text-xs font-bold text-on-surface uppercase tracking-widest truncate">Edit Row: {editingDocId}</h3>
                  <button onClick={() => setEditingDocId(null)} className="text-[10px] text-on-surface-variant hover:text-on-surface uppercase font-bold">Cancel</button>
                </div>
                
                <div className="space-y-3">
                  <div>
                    <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Document JSON Body</label>
                    <textarea 
                      value={editingDocJson}
                      onChange={(e) => setEditingDocJson(e.target.value)}
                      className="w-full h-48 bg-surface-container-low border border-outline-variant/60 rounded-lg p-2.5 text-xs text-on-surface font-mono focus:outline-none focus:border-primary/50"
                      placeholder="{}"
                    />
                  </div>

                  <div className="pt-2">
                    <button 
                      onClick={handleSaveDocJson}
                      disabled={savingDocJson}
                      className="w-full bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-[0.98] transition-all disabled:opacity-50"
                    >
                      {savingDocJson ? 'Saving changes...' : 'Save Document'}
                    </button>
                  </div>
                </div>
              </div>
            ) : (
              <div className="space-y-3.5">
                {filteredDocs.length === 0 ? (
                  <div className="text-center py-8 bg-surface-container-low/40 rounded-xl border border-dashed border-outline-variant/30">
                    <span className="material-symbols-outlined text-3xl text-on-surface-variant/30 block mb-1">database</span>
                    <p className="text-xs text-on-surface-variant/50">No documents found</p>
                  </div>
                ) : (
                  filteredDocs.map((doc) => {
                    const idVal = String(doc._id || doc.id || 'unknown');
                    const isExpanded = expandedDocId === idVal;
                    
                    return (
                      <div 
                        key={idVal}
                        className="bg-surface-container-low border border-outline-variant rounded-xl overflow-hidden transition-all duration-300"
                      >
                        {/* Summary Row */}
                        <button
                          onClick={() => setExpandedDocId(isExpanded ? null : idVal)}
                          className="w-full p-stack-md flex justify-between items-center text-left hover:bg-surface-container-high/40 transition-colors focus:outline-none"
                        >
                          <div className="flex items-center gap-2">
                            <span className="material-symbols-outlined text-xs text-primary">circle</span>
                            <span className="font-code-sm text-xs font-semibold text-on-surface">{idVal}</span>
                          </div>
                          <div className="flex items-center gap-2">
                            <span className="text-[10px] text-on-surface-variant/50 font-code-xs">
                              {Object.keys(doc).length - 1} fields
                            </span>
                            <span className={`material-symbols-outlined text-[16px] text-on-surface-variant/60 transition-transform duration-300 ${isExpanded ? 'rotate-180' : ''}`}>
                              expand_more
                            </span>
                          </div>
                        </button>
  
                        {/* Expandable JSON Detail Container */}
                        <div 
                          className={`grid transition-all duration-300 ease-in-out ${isExpanded ? 'grid-rows-[1fr]' : 'grid-rows-[0fr]'}`}
                          style={{ 
                            borderTop: isExpanded ? '1px solid rgba(60, 74, 66, 0.2)' : 'none'
                          }}
                        >
                          <div className="overflow-hidden">
                            <div className="p-stack-md bg-surface-container-lowest/20 space-y-2">
                              {renderFormattedJson(doc)}
                              <div className="flex justify-end gap-2 pt-1">
                                <button 
                                  onClick={async () => {
                                    if (confirm('Are you sure you want to delete this document?')) {
                                      try {
                                        await api(`/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(selectedCol)}/documents/${encodeURIComponent(idVal)}`, {
                                          method: 'DELETE'
                                        });
                                        showToast('Document deleted', 'success');
                                        loadDocuments(selectedCol);
                                        setExpandedDocId(null);
                                      } catch (err) {
                                        showToast('Failed to delete document', 'error');
                                      }
                                    }
                                  }}
                                  className="px-3 py-1 border border-error/30 text-error hover:bg-error-container/10 font-label-caps text-[10px] rounded transition-all active:scale-95"
                                >
                                  Delete
                                </button>
                                <button 
                                  onClick={() => {
                                    setEditingDocId(idVal);
                                    setEditingDocJson(JSON.stringify(doc, null, 2));
                                  }}
                                  className="px-3 py-1 border border-secondary text-secondary hover:bg-secondary/10 font-label-caps text-[10px] rounded transition-all active:scale-95"
                                >
                                  Edit JSON
                                </button>
                                <button 
                                  onClick={() => setExpandedDocId(null)}
                                  className="px-3 py-1 border border-outline-variant/60 text-on-surface-variant font-label-caps text-[10px] rounded transition-all active:scale-95"
                                >
                                  Collapse
                                </button>
                              </div>
                            </div>
                          </div>
                        </div>
                      </div>
                    );
                  })
                )}
              </div>
            )}
          </div>
        )}

        {/* AUTH TAB */}
        {activeTab === 'auth' && (
          <div className="p-container-padding space-y-stack-md animate-fade-slide">
            
            {/* Multi-Tab Filter Navigation */}
            <div className="flex border-b border-outline-variant/30 gap-6">
              {[
                { id: 'users', label: 'USERS' },
                { id: 'providers', label: 'PROVIDERS' },
                { id: 'policies', label: 'POLICIES' }
              ].map(sub => (
                <button
                  key={sub.id}
                  onClick={() => {
                    setAuthSubTab(sub.id as any);
                    setSwipedUserId(null);
                  }}
                  className={`relative pb-2.5 text-xs font-bold font-label-caps tracking-wider transition-colors focus:outline-none ${
                    authSubTab === sub.id ? 'text-primary' : 'text-on-surface-variant hover:text-on-surface'
                  }`}
                >
                  {sub.label}
                  {authSubTab === sub.id && (
                    <div className="active-tab-indicator absolute bottom-0 left-0 right-0"></div>
                  )}
                </button>
              ))}
            </div>

            {/* USERS SUB-TAB */}
            {authSubTab === 'users' && (
              <div className="space-y-stack-md animate-fade-slide">
                <div className="flex justify-between items-center">
                  <span className="text-on-surface-variant font-label-caps text-[10px] tracking-widest">{filteredUsers.length} TOTAL USERS</span>
                  <button 
                    onClick={() => setIsInviting(!isInviting)}
                    className="flex items-center gap-1 text-primary hover:bg-primary/10 px-2 py-1 rounded transition-colors"
                  >
                    <span className="material-symbols-outlined text-[16px]">person_add</span>
                    <span className="font-label-caps text-[10px] font-bold">INVITE</span>
                  </button>
                </div>

                {/* Invite form widget */}
                {isInviting && (
                  <div className="bg-surface-container border border-outline-variant p-stack-md rounded-xl space-y-2.5 animate-fade-slide">
                    <p className="text-xs font-bold text-on-surface">Invite User to Project</p>
                    <div className="flex gap-2">
                      <input 
                        type="email"
                        placeholder="user@example.com"
                        value={inviteEmail}
                        onChange={(e) => setInviteEmail(e.target.value)}
                        className="flex-1 bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface placeholder-on-surface-variant/40 focus:outline-none focus:border-primary/50"
                      />
                      <button 
                        onClick={handleInviteUser}
                        className="bg-primary text-on-primary px-3 py-1.5 rounded-lg text-xs font-bold active:scale-95"
                      >
                        Send
                      </button>
                    </div>
                  </div>
                )}

                {/* User search bar */}
                <div className="relative">
                  <span className="absolute inset-y-0 left-0 pl-2.5 flex items-center pointer-events-none">
                    <span className="material-symbols-outlined text-[16px] text-on-surface-variant/50">search</span>
                  </span>
                  <input
                    type="text"
                    placeholder="Search users..."
                    value={userSearchQuery}
                    onChange={(e) => setUserSearchQuery(e.target.value)}
                    className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg pl-8 pr-3 py-1.5 text-xs text-on-surface placeholder-on-surface-variant/40 focus:outline-none focus:border-primary/50"
                  />
                </div>

                {/* Users List with Swipe micro-interactions */}
                <div className="space-y-2">
                  {filteredUsers.map((user) => {
                    const isSwiped = swipedUserId === user.id;
                    
                    return (
                      <div 
                        key={user.id}
                        className="relative overflow-hidden rounded-xl border border-outline-variant bg-surface-container-low group"
                      >
                        {/* Swipe Actions Background (BAN and RESET) */}
                        <div className="absolute inset-0 flex justify-end z-0">
                          <button 
                            onClick={() => handleResetUser(user.id, user.email)}
                            className="bg-secondary-container text-on-secondary-container w-24 flex flex-col items-center justify-center gap-1 active:bg-secondary-container/85"
                          >
                            <span className="material-symbols-outlined text-[20px]">lock_reset</span>
                            <span className="text-[9px] font-extrabold uppercase tracking-widest font-label-caps">RESET</span>
                          </button>
                          <button 
                            onClick={() => handleBanUser(user.id, user.email)}
                            className="bg-error-container text-on-error-container w-24 flex flex-col items-center justify-center gap-1 active:bg-error-container/85"
                          >
                            <span className="material-symbols-outlined text-[20px]">block</span>
                            <span className="text-[9px] font-extrabold uppercase tracking-widest font-label-caps">BAN</span>
                          </button>
                        </div>

                        {/* User Foreground Card */}
                        <div 
                          onClick={() => setSwipedUserId(isSwiped ? null : user.id)}
                          className="relative bg-surface-container-low p-stack-md flex items-center justify-between cursor-pointer transition-transform duration-300 z-10"
                          style={{
                            transform: isSwiped ? 'translateX(-192px)' : 'translateX(0px)',
                            borderLeft: user.status === 'Banned' ? '3px solid #ffb4ab' : 'none'
                          }}
                        >
                          <div className="flex items-center gap-stack-md">
                            {/* User Avatar */}
                            <div className="w-10 h-10 rounded-full border border-outline-variant overflow-hidden flex items-center justify-center bg-surface-container-high shrink-0">
                              {user.avatar_url ? (
                                <img src={user.avatar_url} className="w-full h-full object-cover" alt="Avatar" />
                              ) : (
                                <span className="text-on-surface-variant font-bold text-xs uppercase">
                                  {(user.name || user.email || 'U').substring(0, 2)}
                                </span>
                              )}
                            </div>
                            
                            {/* User Details */}
                            <div className="flex flex-col min-w-0">
                              <span className="text-xs font-bold text-on-surface truncate max-w-[130px] sm:max-w-[180px]">
                                {user.email}
                              </span>
                              <span className="text-on-surface-variant text-[10px] font-code-xs tracking-wider uppercase flex items-center gap-1.5 mt-0.5">
                                ID: {user.id.substring(0, 8)}
                                {user.status === 'Banned' && (
                                  <span className="bg-error-container/20 text-error px-1 rounded text-[8px] font-bold">BANNED</span>
                                )}
                                {user.status === 'Pending' && (
                                  <span className="bg-secondary-container/20 text-secondary px-1 rounded text-[8px] font-bold">PENDING</span>
                                )}
                              </span>
                            </div>
                          </div>

                          <div className="flex items-center gap-2">
                            <span className="material-symbols-outlined text-on-surface-variant/60 text-[18px]">
                              {user.provider === 'google' ? 'google' : user.provider === 'code' ? 'code' : 'mail'}
                            </span>
                            <div className={`w-2 h-2 rounded-full ${user.status === 'Banned' ? 'bg-error shadow-[0_0_8px_#ffb4ab]' : user.status === 'Pending' ? 'bg-outline-variant' : 'bg-primary shadow-[0_0_8px_#00d2ff]'}`}></div>
                          </div>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            )}

            {/* PROVIDERS SUB-TAB */}
            {authSubTab === 'providers' && (
              <div className="space-y-stack-md animate-fade-slide">
                {editingProvider ? (
                  /* Provider Configuration Form */
                  <div className="bg-surface-container border border-outline-variant p-stack-md rounded-xl space-y-4 animate-fade-slide">
                    <div className="flex justify-between items-center pb-2 border-b border-outline-variant/30">
                      <h3 className="text-xs font-bold text-on-surface uppercase tracking-widest truncate">Configure {editingProvider === 'custom' ? 'Custom Provider' : editingProvider}</h3>
                      <button onClick={() => setEditingProvider(null)} className="text-[10px] text-on-surface-variant hover:text-on-surface uppercase font-bold">Cancel</button>
                    </div>
                    
                    <div className="space-y-3">
                      <div>
                        <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1">Status</label>
                        <label className="flex items-center gap-2 cursor-pointer">
                          <input 
                            type="checkbox" 
                            checked={providerConfig.enabled}
                            onChange={(e) => setProviderConfig(p => ({ ...p, enabled: e.target.checked }))}
                            className="w-4 h-4 rounded border-outline-variant bg-surface-container-low accent-primary"
                          />
                          <span className="text-xs text-on-surface">Enable Provider Integration</span>
                        </label>
                      </div>

                      {editingProvider === 'custom' && (
                        <div>
                          <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1">Provider ID / Name</label>
                          <input 
                            type="text"
                            value={providerConfig.name || ''}
                            onChange={(e) => setProviderConfig(p => ({ ...p, name: e.target.value }))}
                            className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                            placeholder="e.g. keycloak"
                          />
                        </div>
                      )}

                      <div>
                        <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1">Client ID</label>
                        <input 
                          type="text"
                          value={providerConfig.client_id}
                          onChange={(e) => setProviderConfig(p => ({ ...p, client_id: e.target.value }))}
                          className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                          placeholder="Enter client identifier"
                        />
                      </div>

                      <div>
                        <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1">Client Secret / API Key</label>
                        <input 
                          type="password"
                          value={providerConfig.client_secret}
                          onChange={(e) => setProviderConfig(p => ({ ...p, client_secret: e.target.value }))}
                          className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                          placeholder="••••••••••••••••"
                        />
                      </div>

                      {editingProvider === 'custom' && (
                        <>
                          <div>
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1">Authorization URL</label>
                            <input 
                              type="text"
                              value={providerConfig.auth_url || ''}
                              onChange={(e) => setProviderConfig(p => ({ ...p, auth_url: e.target.value }))}
                              className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                              placeholder="https://example.com/oauth/authorize"
                            />
                          </div>
                          <div>
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1">Token URL</label>
                            <input 
                              type="text"
                              value={providerConfig.token_url || ''}
                              onChange={(e) => setProviderConfig(p => ({ ...p, token_url: e.target.value }))}
                              className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                              placeholder="https://example.com/oauth/token"
                            />
                          </div>
                          <div>
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1">User Info URL</label>
                            <input 
                              type="text"
                              value={providerConfig.userinfo_url || ''}
                              onChange={(e) => setProviderConfig(p => ({ ...p, userinfo_url: e.target.value }))}
                              className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                              placeholder="https://example.com/oauth/userinfo"
                            />
                          </div>
                        </>
                      )}

                      <div>
                        <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1">Scopes (Comma separated)</label>
                        <input 
                          type="text"
                          value={Array.isArray(providerConfig.scopes) ? providerConfig.scopes.join(', ') : providerConfig.scopes}
                          onChange={(e) => setProviderConfig(p => ({ ...p, scopes: e.target.value }))}
                          className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                          placeholder="e.g. email, profile"
                        />
                      </div>

                      <div className="pt-2">
                        <button 
                          onClick={handleSaveProvider}
                          className="w-full bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-98 transition-all"
                        >
                          Save Credentials
                        </button>
                      </div>
                    </div>
                  </div>
                ) : (
                  /* Providers List */
                  <div className="space-y-3">
                    <p className="text-on-surface-variant font-label-caps text-[10px] tracking-widest">ACTIVE AUTHENTICATION METHODS</p>
                    <div className="space-y-3.5">
                      {/* Predefined Templates */}
                      {['google', 'github', 'discord', 'gitlab', 'microsoft', 'facebook', 'twitter'].map(type => {
                        const config = authProviders.find(p => p.name === type);
                        const isEnabled = config?.enabled || false;
                        
                        return (
                          <button 
                            key={type} 
                            onClick={() => {
                              setEditingProvider(type);
                              setProviderConfig({
                                client_id: config?.client_id || '',
                                client_secret: config?.client_secret || '',
                                scopes: config?.scopes || [],
                                enabled: isEnabled,
                                auth_url: config?.auth_url || '',
                                token_url: config?.token_url || '',
                                userinfo_url: config?.userinfo_url || ''
                              });
                            }}
                            className="w-full text-left bg-surface-container-low border border-outline-variant hover:bg-surface-container-high/40 p-stack-md rounded-xl flex items-center justify-between transition-colors focus:outline-none"
                          >
                            <div className="flex items-center gap-3">
                              <span className="w-8 h-8 flex items-center justify-center rounded-lg border border-outline-variant/40 bg-surface-container-lowest text-on-surface-variant shrink-0">
                                {renderProviderIcon(type, "w-4 h-4")}
                              </span>
                              <div>
                                <p className="text-xs font-bold text-on-surface capitalize">{type} OAuth</p>
                                <p className="text-[9px] text-on-surface-variant/70 font-mono truncate max-w-[170px]">
                                  {isEnabled ? (config.client_id || 'Configured') : 'Click to configure integration'}
                                </p>
                              </div>
                            </div>
                            <span className={`text-[9px] font-bold px-2 py-0.5 rounded-full border ${
                              isEnabled 
                                ? 'bg-primary/10 border-primary/20 text-primary' 
                                : 'bg-surface-container text-on-surface-variant/50 border-outline-variant/30'
                            }`}>
                              {isEnabled ? 'Active' : 'Disabled'}
                            </span>
                          </button>
                        );
                      })}

                      {/* Custom Providers */}
                      {authProviders.filter(p => !['google', 'github', 'discord', 'gitlab', 'microsoft', 'facebook', 'twitter'].includes(p.name)).map(p => (
                        <div 
                          key={p.name}
                          onClick={() => {
                            setEditingProvider('custom');
                            setProviderConfig({
                              name: p.name,
                              client_id: p.client_id || '',
                              client_secret: p.client_secret || '',
                              scopes: p.scopes || [],
                              enabled: p.enabled || false,
                              auth_url: p.auth_url || '',
                              token_url: p.token_url || '',
                              userinfo_url: p.userinfo_url || ''
                            });
                          }}
                          className="w-full text-left bg-surface-container-low border border-outline-variant hover:bg-surface-container-high/40 p-stack-md rounded-xl flex items-center justify-between transition-colors cursor-pointer"
                        >
                          <div className="flex items-center gap-3">
                            <span className="w-8 h-8 flex items-center justify-center rounded-lg border border-outline-variant/40 bg-surface-container-lowest text-on-surface-variant shrink-0">
                              {renderProviderIcon(p.name, "w-4 h-4")}
                            </span>
                            <div>
                              <p className="text-xs font-bold text-on-surface truncate max-w-[140px] capitalize">{p.name} (Custom)</p>
                              <p className="text-[9px] text-on-surface-variant/70 font-mono truncate max-w-[170px]">
                                {p.enabled ? (p.client_id || 'Configured') : 'Disabled'}
                              </p>
                            </div>
                          </div>
                          <div className="flex items-center gap-2">
                            <span className={`text-[9px] font-bold px-2 py-0.5 rounded-full border ${
                              p.enabled 
                                ? 'bg-secondary-container/20 border-secondary text-secondary' 
                                : 'bg-surface-container text-on-surface-variant/50 border-outline-variant/30'
                            }`}>
                              {p.enabled ? 'Active' : 'Disabled'}
                            </span>
                            <button
                              onClick={async (e) => {
                                e.stopPropagation();
                                if (confirm(`Delete custom identity provider "${p.name}"?`)) {
                                  try {
                                    const newList = authProviders.filter(x => x.name !== p.name);
                                    await api(`/admin/projects/${encodeURIComponent(projectId)}/auth/providers`, {
                                      method: 'PUT',
                                      body: JSON.stringify({ providers: newList })
                                    });
                                    showToast('Provider deleted', 'success');
                                    loadAuthProviders();
                                  } catch {
                                    showToast('Delete failed', 'error');
                                  }
                                }
                              }}
                              className="text-on-surface-variant hover:text-error transition-colors p-1"
                            >
                              <span className="material-symbols-outlined text-[16px]">delete</span>
                            </button>
                          </div>
                        </div>
                      ))}
                    </div>

                    <div className="pt-2">
                      <button 
                        onClick={() => {
                          setEditingProvider('custom');
                          setProviderConfig({
                            name: '',
                            client_id: '',
                            client_secret: '',
                            scopes: '',
                            enabled: true,
                            auth_url: '',
                            token_url: '',
                            userinfo_url: ''
                          });
                        }}
                        className="w-full bg-surface-container border border-dashed border-outline-variant/60 hover:bg-surface-container-high/40 p-3 rounded-xl flex items-center justify-center gap-1.5 text-xs text-primary font-bold transition-all focus:outline-none"
                      >
                        <span className="material-symbols-outlined text-[16px]">add_circle</span>
                        <span>Configure Custom Provider</span>
                      </button>
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* POLICIES SUB-TAB */}
            {authSubTab === 'policies' && (
              <div className="space-y-stack-md animate-fade-slide">
                {editingPolicyCol ? (
                  /* RLS Rules Editor for selected collection */
                  <div className="bg-surface-container border border-outline-variant p-stack-md rounded-xl space-y-4 animate-fade-slide">
                    <div className="flex justify-between items-center pb-2 border-b border-outline-variant/30">
                      <h3 className="text-xs font-bold text-on-surface uppercase tracking-widest truncate max-w-[180px]">RLS: {editingPolicyCol}</h3>
                      <button onClick={() => setEditingPolicyCol(null)} className="text-[10px] text-on-surface-variant hover:text-on-surface uppercase font-bold">Cancel</button>
                    </div>

                    <div className="space-y-3">
                      <div>
                        <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1">Row-Level Security</label>
                        <label className="flex items-center gap-2 cursor-pointer">
                          <input 
                            type="checkbox" 
                            checked={policyForm.enabled}
                            onChange={(e) => setPolicyForm(p => ({ ...p, enabled: e.target.checked }))}
                            className="w-4 h-4 rounded border-outline-variant bg-surface-container-low accent-primary"
                          />
                          <span className="text-xs text-on-surface">Enable Row Level Security (RLS)</span>
                        </label>
                      </div>

                      {/* Rule inputs for GET, LIST, CREATE, UPDATE, DELETE */}
                      {['get', 'list', 'create', 'update', 'delete'].map(action => (
                        <div key={action} className="space-y-1">
                          <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider font-mono">{action.toUpperCase()} Rule</label>
                          <input 
                            type="text"
                            value={(policyForm as any)[action]}
                            disabled={!policyForm.enabled}
                            onChange={(e) => setPolicyForm(p => ({ ...p, [action]: e.target.value }))}
                            placeholder="e.g. auth != null"
                            className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono disabled:opacity-40 disabled:cursor-not-allowed"
                          />
                        </div>
                      ))}

                      <div className="pt-2">
                        <button 
                          onClick={handleSavePolicies}
                          className="w-full bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-98 transition-all"
                        >
                          Save RLS Policies
                        </button>
                      </div>
                    </div>
                  </div>
                ) : (
                  /* Collections Selector to view policies */
                  <div className="space-y-3">
                    <p className="text-on-surface-variant font-label-caps text-[10px] tracking-widest">COLLECTIONS SECURITY CONFIG</p>
                    <div className="space-y-2">
                      {collections.length === 0 ? (
                        <div className="text-center py-8 bg-surface-container-low/40 rounded-xl border border-dashed border-outline-variant/30">
                          <span className="material-symbols-outlined text-3xl text-on-surface-variant/30 block mb-1">database</span>
                          <p className="text-xs text-on-surface-variant/50">No collections available</p>
                        </div>
                      ) : (
                        collections.map(col => (
                          <button
                            key={col.name}
                            onClick={() => handleSelectPolicyCol(col.name)}
                            className="w-full text-left bg-surface-container-low border border-outline-variant hover:bg-surface-container-high/40 p-stack-md rounded-xl flex items-center justify-between transition-colors focus:outline-none"
                          >
                            <div className="flex items-center gap-2">
                              <span className="material-symbols-outlined text-primary text-sm">lock</span>
                              <span className="text-xs font-bold text-on-surface font-mono">{col.name}</span>
                            </div>
                            <span className="text-[9px] text-on-surface-variant font-semibold">
                              Configure policies
                            </span>
                          </button>
                        ))
                      )}
                    </div>
                  </div>
                )}
              </div>
            )}

          </div>
        )}

        {/* LOGS TAB */}
        {activeTab === 'logs' && (
          <div className="p-container-padding space-y-stack-md animate-fade-slide relative">
            
            {/* Realtime metric widgets */}
            <section className="grid grid-cols-2 gap-gutter mb-1">
              <div className="bg-surface-container-low border border-outline-variant rounded-xl p-stack-md bento-glow">
                <div className="flex items-center gap-2 mb-1">
                  <div className="w-1.5 h-1.5 rounded-full bg-primary shadow-[0_0_8px_#00d2ff]"></div>
                  <span className="font-label-caps text-[10px] text-on-surface-variant tracking-widest uppercase">API REQUESTS</span>
                </div>
                <div className="font-headline-lg text-lg font-bold text-on-surface">{projectUsage?.api_requests || 0}</div>
              </div>
              <div className="bg-surface-container-low border border-outline-variant rounded-xl p-stack-md">
                <div className="flex items-center gap-2 mb-1">
                  <div className="w-1.5 h-1.5 rounded-full bg-secondary shadow-[0_0_8px_#d0bcff]"></div>
                  <span className="font-label-caps text-[10px] text-on-surface-variant tracking-widest uppercase">WEBSOCKETS</span>
                </div>
                <div className="font-headline-lg text-lg font-bold text-on-surface">
                  {projectUsage?.realtime_connections || 0}
                </div>
              </div>
            </section>

            {/* Filter chips + Clear Logs */}
            <div className="flex gap-2 overflow-x-auto no-scrollbar pb-1 sticky top-0 bg-background/90 z-20 backdrop-blur-md py-2 border-b border-outline-variant/20 items-center">
              <button 
                onClick={() => setLogFilter('ALL')}
                className={`px-3 py-1 rounded-full text-[10px] tracking-wider font-label-caps whitespace-nowrap active:scale-95 transition-transform border ${
                  logFilter === 'ALL' 
                    ? 'bg-primary/10 border-primary text-primary font-bold' 
                    : 'bg-surface-container-high border-outline-variant text-on-surface-variant'
                }`}
              >
                ALL LOGS
              </button>
              <button 
                onClick={() => setLogFilter('500')}
                className={`px-3 py-1 rounded-full text-[10px] tracking-wider font-label-caps whitespace-nowrap active:scale-95 transition-transform flex items-center gap-1.5 border ${
                  logFilter === '500' 
                    ? 'bg-error-container/20 border-error text-error font-bold' 
                    : 'bg-surface-container-high border-outline-variant text-on-surface-variant'
                }`}
              >
                <span className="w-1.5 h-1.5 rounded-full bg-error"></span> 500 ERRORS
              </button>
              <button 
                onClick={() => setLogFilter('404')}
                className={`px-3 py-1 rounded-full text-[10px] tracking-wider font-label-caps whitespace-nowrap active:scale-95 transition-transform flex items-center gap-1.5 border ${
                  logFilter === '404' 
                    ? 'bg-secondary-container/20 border-secondary text-secondary font-bold' 
                    : 'bg-surface-container-high border-outline-variant text-on-surface-variant'
                }`}
              >
                <span className="w-1.5 h-1.5 rounded-full bg-secondary"></span> 404 WARNINGS
              </button>
              <span className="flex-1"></span>
              <button 
                onClick={handleClearLogs}
                className="px-3 py-1 rounded-full text-[10px] tracking-wider font-label-caps whitespace-nowrap active:scale-95 transition-transform flex items-center gap-1.5 bg-error/10 border border-error/30 text-error hover:bg-error/20"
              >
                <span className="material-symbols-outlined text-[14px]">delete_sweep</span> CLEAR
              </button>
            </div>

            {/* Stream Logs timeline list */}
            <div className="space-y-2 relative" id="logs-container">
              {filteredLogs.length === 0 ? (
                <div className="text-center py-12 text-on-surface-variant/40">
                  No logs matching filter
                </div>
              ) : (
                filteredLogs.map((log, index) => {
                  // Determine Accent Color based on HTTP Method or Status
                  let logBorderColor = 'bg-primary'; // Green [GET]
                  let logTextColor = 'text-primary';
                  if (log.method === 'POST') {
                    logBorderColor = 'bg-secondary'; // Violet
                    logTextColor = 'text-secondary';
                  } else if (log.method === 'ERROR' || log.status >= 500) {
                    logBorderColor = 'bg-error'; // Red
                    logTextColor = 'text-error';
                  } else if (log.status === 404) {
                    logBorderColor = 'bg-on-tertiary-container'; // Amber/Yellow
                    logTextColor = 'text-on-tertiary-container';
                  } else if (log.method === 'PUT' || log.method === 'PATCH') {
                    logBorderColor = 'bg-secondary';
                    logTextColor = 'text-secondary';
                  }

                  return (
                    <div 
                      key={index}
                      className="log-entry bg-surface-container-low border border-outline-variant rounded p-stack-sm flex flex-col gap-1.5 relative overflow-hidden group animate-fade-slide"
                    >
                      {/* Color indicator on absolute left border */}
                      <div className={`absolute left-0 top-0 bottom-0 w-1 ${logBorderColor}`}></div>
                      
                      <div className="flex justify-between items-center px-1">
                        <div className="flex items-center gap-3">
                          <span className="font-code-xs text-code-xs text-on-surface-variant">{log.timestamp}</span>
                          <span className={`font-code-sm text-code-sm font-extrabold ${logTextColor}`}>[{log.method}]</span>
                        </div>
                        <div className="flex items-center gap-2">
                          <span className={`font-code-sm text-code-sm ${logTextColor}`}>
                            {log.status} {log.status === 200 ? 'OK' : log.status === 201 ? 'CREATED' : log.status === 500 ? 'SERVER ERR' : log.status === 404 ? 'NOT FOUND' : 'NO CONTENT'}
                          </span>
                          <span className="font-code-xs text-code-xs text-on-surface-variant">{log.duration}</span>
                        </div>
                      </div>
                      <div className="font-code-sm text-code-sm text-on-surface truncate px-1 font-semibold select-text">
                        {log.path}
                      </div>
                    </div>
                  );
                })
              )}
            </div>

            {/* Streaming atmospheric bouncing balls indicator */}
            {isStreaming && (
              <div className="flex justify-center items-center py-stack-lg gap-2">
                <span className="w-1.5 h-1.5 bg-primary/60 rounded-full animate-bounce" style={{ animationDelay: '0s' }}></span>
                <span className="w-1.5 h-1.5 bg-primary/60 rounded-full animate-bounce" style={{ animationDelay: '0.2s' }}></span>
                <span className="w-1.5 h-1.5 bg-primary/60 rounded-full animate-bounce" style={{ animationDelay: '0.4s' }}></span>
              </div>
            )}

          </div>
        )}

        {/* MORE SERVICES HUB & SUB-PAGES */}
        {activeTab === 'more' && (
          <div className="p-container-padding space-y-stack-md animate-fade-slide">
            
            {/* Bento Grid Launcher */}
            {activeSubPage === 'grid' && (
              <div className="space-y-4">
                <div className="border-b border-outline-variant/20 pb-1.5">
                  <h2 className="font-label-caps text-[10px] text-on-surface-variant uppercase tracking-widest font-bold">Project Services Launcher</h2>
                </div>
                <div className="grid grid-cols-2 gap-stack-md">
                  {[
                    { id: 'storage', name: 'Storage Buckets', desc: 'S3-ready file storage', icon: 'folder', color: 'text-primary bg-primary/10 border-primary/20 animate-pulse' },
                    { id: 'team', name: 'Collaborators', desc: 'Project team members', icon: 'group', color: 'text-secondary bg-secondary/10 border-secondary/20' },
                    { id: 'config', name: 'Project Config', desc: 'Credentials & API keys', icon: 'settings', color: 'text-primary bg-primary/10 border-primary/20' },
                    { id: 'webhooks', name: 'Webhooks', desc: 'Outgoing event alerts', icon: 'webhook', color: 'text-secondary bg-secondary/10 border-secondary/20 animate-pulse' },
                    { id: 'cron', name: 'Cron Jobs', desc: 'Scheduled callbacks', icon: 'alarm', color: 'text-primary bg-primary/10 border-primary/20' },
                    { id: 'queues', name: 'Message Queues', desc: 'Topic-based worker logs', icon: 'queue', color: 'text-secondary bg-secondary/10 border-secondary/20' },
                    { id: 'integrations', name: 'Integrations', desc: 'Connect plugin APIs', icon: 'extension', color: 'text-primary bg-primary/10 border-primary/20' },
                    { id: 'analytics', name: 'Analytics', desc: 'Traffic & storage charts', icon: 'bar_chart', color: 'text-secondary bg-secondary/10 border-secondary/20 animate-pulse' },
                    { id: 'api', name: 'API Docs', desc: 'JS & Dart client SDK', icon: 'code', color: 'text-primary bg-primary/10 border-primary/20' }
                  ].map((srv) => (
                    <button
                      key={srv.id}
                      onClick={() => {
                        setActiveSubPage(srv.id as any);
                        if (srv.id === 'cron') loadCronJobs();
                        if (srv.id === 'storage') loadBuckets();
                        if (srv.id === 'team') loadMembers();
                        if (srv.id === 'webhooks') loadWebhooks();
                        if (srv.id === 'config') { loadProjectConfig(); loadConfigEntries(); }
                        if (srv.id === 'analytics') { loadProjectUsage(); loadAnalytics(); }
                      }}
                      className="bg-surface-container-low border border-outline-variant hover:bg-surface-container-high/60 rounded-xl p-stack-md text-left flex flex-col gap-2.5 transition-all duration-200 active:scale-[0.97] focus:outline-none"
                    >
                      <div className={`w-8 h-8 rounded-lg flex items-center justify-center border ${srv.color}`}>
                        <span className="material-symbols-outlined text-[18px]">{srv.icon}</span>
                      </div>
                      <div>
                        <h4 className="text-xs font-bold text-on-surface leading-tight">{srv.name}</h4>
                        <p className="text-[9px] text-on-surface-variant/70 leading-normal mt-0.5">{srv.desc}</p>
                      </div>
                    </button>
                  ))}
                </div>
              </div>
            )}

            {/* STORAGE SUB-PAGE */}
            {activeSubPage === 'storage' && (
              <div className="space-y-stack-md animate-fade-slide">
                {selectedBucket ? (
                  /* Bucket Files View */
                  <div className="space-y-4">
                    <div className="flex justify-between items-center bg-surface-container-lowest/30 p-2 border border-outline-variant/35 rounded-lg">
                      <button 
                        onClick={() => setSelectedBucket(null)}
                        className="flex items-center gap-1 text-[10px] text-primary hover:text-accent font-bold uppercase"
                      >
                        <span className="material-symbols-outlined text-[14px]">arrow_back</span> Buckets
                      </button>
                      <span className="text-[10px] font-bold text-on-surface truncate max-w-[130px] font-mono">
                        {selectedBucket}
                      </span>
                      <button 
                        onClick={() => {
                          setModalData({ fileName: '', fileContent: 'Hello Sovrabase storage!' });
                          setActiveModal('upload_file');
                        }}
                        className="flex items-center gap-0.5 text-primary hover:bg-primary/10 px-2 py-0.5 rounded text-[9px] font-bold"
                      >
                        <span className="material-symbols-outlined text-[12px]">upload</span> UPLOAD
                      </button>
                    </div>

                    <div className="space-y-2">
                      {loadingFiles ? (
                        <div className="text-center py-6 text-on-surface-variant/50 text-xs">
                          <span className="material-symbols-outlined animate-spin text-[16px] mr-1.5 align-middle">sync</span> Loading files...
                        </div>
                      ) : files.length === 0 ? (
                        <div className="text-center py-10 bg-surface-container/20 border border-dashed border-outline-variant rounded-xl text-xs text-on-surface-variant/40">
                          No files found in this bucket.
                        </div>
                      ) : (
                        files.map((file, idx) => (
                          <div key={idx} className="bg-surface-container-low border border-outline-variant/60 rounded-xl p-stack-md flex justify-between items-center">
                            <div className="flex items-center gap-2.5 min-w-0">
                              <span className="material-symbols-outlined text-primary text-[18px]">description</span>
                              <div className="min-w-0">
                                <p className="text-xs font-bold text-on-surface truncate max-w-[150px] font-mono">{file.name || file.path}</p>
                                <p className="text-[9px] text-on-surface-variant/60 font-code-xs">
                                  {file.size ? `${(file.size / 1024).toFixed(1)} KB` : 'Unknown size'} • {file.contentType || 'binary'}
                                </p>
                              </div>
                            </div>
                            <button
                              onClick={() => {
                                if (confirm(`Delete file "${file.name || file.path}"?`)) {
                                  api(`/admin/projects/${encodeURIComponent(projectId)}/storage/buckets/${encodeURIComponent(selectedBucket)}/files/${encodeURIComponent(file.name || file.path)}`, {
                                    method: 'DELETE'
                                  }).then(() => {
                                    showToast('File deleted', 'success');
                                    loadFiles(selectedBucket);
                                  }).catch(() => showToast('Delete failed', 'error'));
                                }
                              }}
                              className="text-on-surface-variant hover:text-error transition-colors p-1"
                            >
                              <span className="material-symbols-outlined text-[16px]">delete</span>
                            </button>
                          </div>
                        ))
                      )}
                    </div>
                  </div>
                ) : (
                  /* Buckets List */
                  <div className="space-y-4">
                    <div className="flex justify-between items-center">
                      <span className="text-on-surface-variant font-label-caps text-[10px] tracking-widest">{buckets.length} BUCKETS</span>
                      <button 
                        onClick={() => {
                          setModalData({ bucketName: '' });
                          setActiveModal('create_bucket');
                        }}
                        className="flex items-center gap-1 text-primary hover:bg-primary/10 px-2.5 py-1 rounded transition-colors text-[9px] font-bold font-label-caps"
                      >
                        <span className="material-symbols-outlined text-[14px]">create_new_folder</span> CREATE BUCKET
                      </button>
                    </div>

                    <div className="space-y-2">
                      {loadingBuckets ? (
                        <div className="text-center py-6 text-on-surface-variant/50 text-xs">
                          <span className="material-symbols-outlined animate-spin text-[16px] mr-1.5 align-middle">sync</span> Loading buckets...
                        </div>
                      ) : buckets.length === 0 ? (
                        <div className="text-center py-12 bg-surface-container/20 border border-dashed border-outline-variant rounded-xl text-xs text-on-surface-variant/40 p-6">
                          No storage buckets configured.
                        </div>
                      ) : (
                        buckets.map((b) => (
                          <div 
                            key={b.name} 
                            className="bg-surface-container border border-outline-variant rounded-xl overflow-hidden hover:bg-surface-container-high transition-colors"
                          >
                            <div className="p-stack-md flex justify-between items-center">
                              <button 
                                onClick={() => {
                                  setSelectedBucket(b.name);
                                  loadFiles(b.name);
                                }}
                                className="flex items-center gap-3 text-left focus:outline-none flex-1"
                              >
                                <span className="material-symbols-outlined text-primary text-[22px]">folder</span>
                                <div>
                                  <p className="text-xs font-bold text-on-surface font-mono">{b.name}</p>
                                  <p className="text-[9px] text-on-surface-variant/60">Tap to browse S3-ready files</p>
                                </div>
                              </button>
                              <button
                                onClick={() => {
                                  if (confirm(`Delete storage bucket "${b.name}"?`)) {
                                    api(`/admin/projects/${encodeURIComponent(projectId)}/storage/buckets/${encodeURIComponent(b.name)}`, {
                                      method: 'DELETE'
                                    }).then(() => {
                                      showToast('Bucket deleted', 'success');
                                      loadBuckets();
                                    }).catch(() => showToast('Failed to delete bucket', 'error'));
                                  }
                                }}
                                className="text-on-surface-variant hover:text-error transition-colors p-1.5"
                              >
                                <span className="material-symbols-outlined text-[16px]">delete</span>
                              </button>
                            </div>
                          </div>
                        ))
                      )}
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* TEAM COLLABORATORS SUB-PAGE */}
            {activeSubPage === 'team' && (
              <div className="space-y-stack-md animate-fade-slide">
                <div className="flex justify-between items-center">
                  <span className="text-on-surface-variant font-label-caps text-[10px] tracking-widest">{members.length} COLLABORATORS</span>
                  <button 
                    onClick={() => {
                      setModalData({ memberEmail: '', memberRole: 'developer' });
                      setActiveModal('invite_member');
                    }}
                    className="flex items-center gap-1 text-primary hover:bg-primary/10 px-2 py-0.5 rounded text-[9px] font-bold"
                  >
                    <span className="material-symbols-outlined text-[14px]">group_add</span> INVITE MEMBER
                  </button>
                </div>

                <div className="space-y-3.5">
                  {loadingMembers ? (
                    <div className="text-center py-6 text-on-surface-variant/50 text-xs">
                      <span className="material-symbols-outlined animate-spin text-[16px] mr-1.5 align-middle">sync</span> Loading members...
                    </div>
                  ) : (
                    members.map(member => (
                      <div key={member.user_id} className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl flex justify-between items-center">
                        <div className="flex items-center gap-3 min-w-0">
                          <div className="w-8 h-8 rounded-full bg-secondary/15 flex items-center justify-center text-secondary border border-secondary/20">
                            <span className="text-[10px] font-bold capitalize">{(member.email || 'M').substring(0, 2)}</span>
                          </div>
                          <div className="min-w-0">
                            <p className="text-xs font-bold text-on-surface truncate max-w-[170px]">{member.email}</p>
                            <select
                              value={member.role}
                              onChange={(e) => changeMemberRole(member.user_id, e.target.value)}
                              disabled={member.user_id === (typeof window !== 'undefined' ? localStorage.getItem('sovrabase_admin_user_id') : null)}
                              className="text-[9px] bg-transparent border border-outline-variant/40 rounded px-1 py-0 text-on-surface-variant font-code-xs capitalize appearance-none cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
                              style={{ backgroundImage: `url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='8' height='8' viewBox='0 0 24 24' fill='none' stroke='%23999' stroke-width='3' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpolyline points='6 9 12 15 18 9'%3E%3C/polyline%3E%3C/svg%3E")`, backgroundRepeat: 'no-repeat', backgroundPosition: 'right 2px center', paddingRight: '14px' }}
                            >
                              <option value="owner">owner</option>
                              <option value="admin">admin</option>
                              <option value="developer">developer</option>
                              <option value="viewer">viewer</option>
                            </select>
                          </div>
                        </div>
                        {member.role !== 'owner' && (
                          <button
                            onClick={() => {
                              if (confirm(`Remove collaborator ${member.email}?`)) {
                                api(`/admin/projects/${encodeURIComponent(projectId)}/members/${encodeURIComponent(member.user_id)}`, {
                                  method: 'DELETE'
                                }).then(() => {
                                  showToast('Collaborator removed', 'success');
                                  loadMembers();
                                }).catch(err => showToast(err.message || 'Failed', 'error'));
                              }
                            }}
                            className="text-on-surface-variant hover:text-error transition-colors p-1"
                          >
                            <span className="material-symbols-outlined text-[16px]">person_remove</span>
                          </button>
                        )}
                      </div>
                    ))
                  )}
                </div>
              </div>
            )}

            {/* PROJECT CONFIG SUB-PAGE */}
            {activeSubPage === 'config' && (
              <div className="space-y-stack-md animate-fade-slide bg-surface-container border border-outline-variant p-stack-md rounded-xl">
                <div className="border-b border-outline-variant/30 pb-2">
                  <h3 className="text-xs font-bold text-on-surface uppercase tracking-widest">Project Credentials</h3>
                </div>
                
                {projectDetail ? (
                  <div className="space-y-3.5">
                    <div>
                      <span className="text-[8px] font-bold text-on-surface-variant uppercase tracking-wider block mb-1">CORS Allowed Origins</span>
                      <input 
                        type="text"
                        value={projectDetail.allow_origins || ''}
                        onChange={(e) => setProjectDetail({ ...projectDetail, allow_origins: e.target.value })}
                        className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                        placeholder="e.g. *, http://localhost:3000"
                      />
                    </div>

                    <div>
                      <span className="text-[8px] font-bold text-on-surface-variant uppercase tracking-wider block mb-1">Storage Quota (Bytes)</span>
                      <input 
                        type="number"
                        value={projectDetail.storage_quota || 0}
                        onChange={(e) => setProjectDetail({ ...projectDetail, storage_quota: Number(e.target.value) })}
                        className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                      />
                    </div>

                    <div>
                      <span className="text-[8px] font-bold text-on-surface-variant uppercase tracking-wider block mb-0.5">Project API Client Key</span>
                      <div className="flex gap-2 items-center bg-surface-container-lowest/50 border border-outline-variant/30 px-2 py-1.5 rounded-lg select-all">
                        <span className="font-code-xs text-[10px] text-primary truncate flex-1 font-mono">{projectDetail.api_key || 'No key generated'}</span>
                        <button 
                          onClick={() => {
                            navigator.clipboard.writeText(projectDetail.api_key || '');
                            showToast('API Key copied', 'success');
                          }}
                          className="text-on-surface-variant hover:text-primary active:scale-90 transition-all"
                        >
                          <span className="material-symbols-outlined text-[16px]">content_copy</span>
                        </button>
                      </div>
                    </div>

                    <div className="pt-2">
                      <button
                        onClick={() => {
                          setSavingConfig(true);
                          api(`/admin/projects/${encodeURIComponent(projectId)}`, {
                            method: 'PUT',
                            body: JSON.stringify({
                              allow_origins: projectDetail.allow_origins,
                              storage_quota: Number(projectDetail.storage_quota)
                            })
                          }).then(() => {
                            showToast('Project configuration updated', 'success');
                            loadProjectConfig();
                          }).catch(err => showToast(err.message || 'Save failed', 'error'))
                            .finally(() => setSavingConfig(false));
                        }}
                        disabled={savingConfig}
                        className="w-full bg-primary text-on-primary py-2 rounded-lg font-bold text-xs active:scale-[0.98] transition-all"
                      >
                        {savingConfig ? 'Saving...' : 'Save Configuration'}
                      </button>
                    </div>
                  </div>
                ) : (
                  <div className="text-center py-6 text-on-surface-variant/40 text-xs">
                    Failed to load project details.
                  </div>
                )}
              </div>
            )}

            {/* CONFIG KEY-VALUE ENTRIES SUB-PAGE */}
            {activeSubPage === 'config' && (
              <div className="space-y-stack-md animate-fade-slide bg-surface-container border border-outline-variant p-stack-md rounded-xl">
                <div className="flex justify-between items-center border-b border-outline-variant/30 pb-2">
                  <h3 className="text-xs font-bold text-on-surface uppercase tracking-widest">Config Entries</h3>
                  <button
                    onClick={() => {
                      setModalData({ configKey: '', configValue: '', configType: 'string', configPublic: 'false' });
                      setActiveModal('add_config');
                    }}
                    className="flex items-center gap-1 text-primary hover:bg-primary/10 px-2 py-0.5 rounded text-[9px] font-bold active:scale-95 transition-all"
                  >
                    <span className="material-symbols-outlined text-[14px]">add</span> ADD ENTRY
                  </button>
                </div>

                {loadingConfigEntries ? (
                  <div className="text-center py-6 text-on-surface-variant/50 text-xs">
                    <span className="material-symbols-outlined animate-spin text-[16px] mr-1.5 align-middle">sync</span> Loading entries...
                  </div>
                ) : configEntries.length === 0 ? (
                  <div className="text-center py-10 bg-surface-container/20 border border-dashed border-outline-variant rounded-xl text-xs text-on-surface-variant/40">
                    No config entries defined.
                  </div>
                ) : (
                  <div className="space-y-2.5">
                    {configEntries.map((entry: any, idx: number) => (
                      <div key={entry.key || idx} className="bg-surface-container-low border border-outline-variant/60 rounded-lg p-3 flex justify-between items-start gap-2">
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-1.5 mb-1 flex-wrap">
                            <span className="text-xs font-bold text-on-surface font-mono truncate">{entry.key}</span>
                            <span className={`text-[8px] font-bold px-1.5 py-0.5 rounded border font-mono ${
                              entry.type === 'string' ? 'bg-primary/10 text-primary border-primary/20' :
                              entry.type === 'number' ? 'bg-secondary/10 text-secondary border-secondary/20' :
                              entry.type === 'boolean' ? 'bg-tertiary/10 text-tertiary border-tertiary/20' :
                              'bg-surface-container-high text-on-surface-variant border-outline-variant/40'
                            }`}>
                              {entry.type}
                            </span>
                            <span className={`text-[8px] font-bold px-1.5 py-0.5 rounded border font-mono ${
                              entry.public ? 'bg-success/10 text-success border-success/20' : 'bg-error/10 text-error border-error/20'
                            }`}>
                              {entry.public ? 'PUBLIC' : 'PRIVATE'}
                            </span>
                          </div>
                          <p className="text-[10px] text-on-surface-variant/70 font-mono truncate max-w-[200px]">
                            {typeof entry.value === 'string' ? entry.value : JSON.stringify(entry.value)}
                          </p>
                        </div>
                        <button
                          onClick={() => {
                            if (confirm(`Delete config entry "${entry.key}"?`)) {
                              api(`/admin/projects/${encodeURIComponent(projectId)}/config/${encodeURIComponent(entry.key)}`, {
                                method: 'DELETE'
                              }).then(() => {
                                showToast('Entry deleted', 'success');
                                loadConfigEntries();
                              }).catch(() => showToast('Failed to delete', 'error'));
                            }
                          }}
                          className="text-on-surface-variant hover:text-error transition-colors p-1 shrink-0 active:scale-90"
                        >
                          <span className="material-symbols-outlined text-[16px]">delete</span>
                        </button>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}

            {/* WEBHOOKS SUB-PAGE */}
            {activeSubPage === 'webhooks' && (
              <div className="space-y-stack-md animate-fade-slide">
                <div className="flex justify-between items-center">
                  <span className="text-on-surface-variant font-label-caps text-[10px] tracking-widest">{webhooks.length} OUTGOING SUBSCRIPTIONS</span>
                  <button 
                    onClick={() => {
                      setModalData({ webhookUrl: '', webhookEvents: 'insert,update,delete' });
                      setActiveModal('add_webhook');
                    }}
                    className="flex items-center gap-1 text-primary hover:bg-primary/10 px-2 py-0.5 rounded text-[9px] font-bold"
                  >
                    <span className="material-symbols-outlined text-[14px]">add_link</span> ADD WEBHOOK
                  </button>
                </div>

                <div className="space-y-3.5">
                  {loadingWebhooks ? (
                    <div className="text-center py-6 text-on-surface-variant/50 text-xs">
                      <span className="material-symbols-outlined animate-spin text-[16px] mr-1.5 align-middle">sync</span> Loading webhooks...
                    </div>
                  ) : webhooks.length === 0 ? (
                    <div className="text-center py-10 bg-surface-container/20 border border-dashed border-outline-variant rounded-xl text-xs text-on-surface-variant/40">
                      No webhooks registered.
                    </div>
                  ) : (
                    webhooks.map(wh => (
                      <div key={wh.id} className="bg-surface-container border border-outline-variant p-stack-md rounded-xl space-y-2">
                        <div className="flex justify-between items-start">
                          <div className="min-w-0 flex-1 pr-2">
                            <p className="text-xs font-bold text-on-surface truncate font-mono">{wh.url}</p>
                            <div className="flex gap-1.5 mt-1.5 flex-wrap">
                              {(wh.events || []).map((ev: string) => (
                                <span key={ev} className="bg-secondary/15 text-secondary text-[8px] font-bold px-1.5 py-0.5 rounded border border-secondary/20 font-mono">
                                  {ev}
                                </span>
                              ))}
                            </div>
                          </div>
                          <button
                            onClick={() => {
                              if (confirm('Delete this webhook subscription?')) {
                                api(`/admin/projects/${encodeURIComponent(projectId)}/webhooks/${encodeURIComponent(wh.id)}`, {
                                  method: 'DELETE'
                                }).then(() => {
                                  showToast('Webhook deleted', 'success');
                                  loadWebhooks();
                                }).catch(() => showToast('Failed to delete', 'error'));
                              }
                            }}
                            className="text-on-surface-variant hover:text-error transition-colors p-1"
                          >
                            <span className="material-symbols-outlined text-[16px]">delete</span>
                          </button>
                        </div>
                      </div>
                    ))
                  )}
                </div>
              </div>
            )}

            {/* CRON JOBS SUB-PAGE */}
            {activeSubPage === 'cron' && (
              <div className="space-y-4 animate-fade-slide">
                <div className="flex justify-between items-end pb-1 border-b border-outline-variant/20">
                  <h2 className="font-label-caps text-[10px] text-on-surface-variant uppercase tracking-widest font-bold">Active Scheduled Tasks ({cronJobs.length})</h2>
                </div>

                <div className="space-y-3.5">
                  {loadingJobs ? (
                    <div className="text-center py-6 text-on-surface-variant/50 text-xs">
                      <span className="material-symbols-outlined animate-spin text-[16px]">sync</span> Loading jobs...
                    </div>
                  ) : cronJobs.length === 0 ? (
                    <div className="text-center py-10 bg-surface-container/20 border border-dashed border-outline-variant rounded-xl text-xs text-on-surface-variant/40">
                      No cron jobs created yet.
                    </div>
                  ) : (
                    cronJobs.map(job => {
                      const isLoader = invokingStates[job.id] === 'loading';
                      
                      return (
                        <div key={job.id} className="bg-surface-container border border-outline-variant rounded-xl overflow-hidden p-stack-md flex justify-between items-center gap-2">
                          <div className="min-w-0">
                            <div className="flex items-center gap-1.5">
                              <h4 className="text-xs font-bold text-on-surface font-mono">{job.name}</h4>
                              <span className="bg-primary/10 text-primary text-[8px] font-bold px-1.5 py-0.5 rounded font-mono">{job.schedule}</span>
                            </div>
                            <p className="text-[9px] text-on-surface-variant/60 truncate max-w-[170px] font-mono mt-0.5">{job.url}</p>
                          </div>
                          
                          <div className="flex items-center gap-1 bg-surface-container-low px-1.5 py-1 rounded-lg border border-outline-variant/30 shrink-0">
                            <button
                              disabled={isLoader}
                              onClick={() => handleInvoke(job)}
                              className="text-primary hover:bg-primary/10 p-1 rounded active:scale-90 transition-all"
                            >
                              <span className="material-symbols-outlined text-[16px]">{isLoader ? 'sync' : 'play_arrow'}</span>
                            </button>
                            <button
                              onClick={() => {
                                if (confirm('Delete cron job?')) {
                                  api(`/admin/projects/${encodeURIComponent(projectId)}/cron/${encodeURIComponent(job.id)}`, { method: 'DELETE' })
                                    .then(() => { showToast('Job deleted', 'success'); loadCronJobs(); })
                                    .catch(() => showToast('Failed to delete', 'error'));
                                }
                              }}
                              className="text-on-surface-variant hover:text-error p-1 rounded active:scale-90 transition-all"
                            >
                              <span className="material-symbols-outlined text-[16px]">delete</span>
                            </button>
                          </div>
                        </div>
                      );
                    })
                  )}
                </div>

                <div className="pt-2">
                  <button 
                    onClick={() => {
                      setModalData({ cronName: '', cronSchedule: '*/5 * * * *', cronUrl: '' });
                      setActiveModal('create_cron');
                    }}
                    className="w-full bg-primary text-on-primary py-2 rounded-lg font-bold text-xs active:scale-[0.98] transition-all"
                  >
                    Add scheduled task
                  </button>
                </div>
              </div>
            )}

            {/* MESSAGE QUEUES SUB-PAGE */}
            {activeSubPage === 'queues' && (
              <div className="space-y-stack-md animate-fade-slide">
                <p className="text-on-surface-variant font-label-caps text-[10px] tracking-widest">BACKGROUND TRANSACTION QUEUES ({queues.length})</p>
                {loadingQueues ? (
                  <div className="flex items-center justify-center py-8">
                    <span className="animate-spin text-xl text-on-surface-variant">↻</span>
                  </div>
                ) : queues.length === 0 ? (
                  <div className="flex flex-col items-center py-8 text-on-surface-variant/60 gap-2">
                    <span className="material-symbols-outlined text-2xl">queue</span>
                    <p className="text-xs">No message queues yet</p>
                  </div>
                ) : (
                  <div className="space-y-3.5">
                    {queues.map(q => (
                      <div key={q.name} className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl flex justify-between items-center">
                        <div>
                          <div className="flex items-center gap-1.5">
                            <span className="material-symbols-outlined text-secondary text-sm">queue</span>
                            <h4 className="text-xs font-bold text-on-surface font-mono">{q.name}</h4>
                          </div>
                          <div className="flex gap-3 mt-1">
                            <p className="text-[9px] text-on-surface-variant/60"><span className="text-success font-mono">{q.visible.toLocaleString()}</span> visible</p>
                            <p className="text-[9px] text-on-surface-variant/60"><span className="text-yellow-500 font-mono">{q.in_flight.toLocaleString()}</span> in flight</p>
                            <p className="text-[9px] text-on-surface-variant/60"><span className="text-on-surface font-mono">{q.total.toLocaleString()}</span> total</p>
                          </div>
                        </div>
                        <div className="flex items-center gap-1 shrink-0">
                          <button onClick={() => unstickQueue(q.name)} className="p-1.5 rounded text-yellow-500 hover:bg-yellow-500/10" title="Make visible">
                            <span className="material-symbols-outlined text-sm">lock_open</span>
                          </button>
                          <button onClick={() => purgeQueue(q.name)} className="p-1.5 rounded text-error hover:bg-error/10" title="Purge">
                            <span className="material-symbols-outlined text-sm">delete</span>
                          </button>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}

            {/* INTEGRATIONS SUB-PAGE */}
            {activeSubPage === 'integrations' && (
              <div className="space-y-stack-md animate-fade-slide">
                {editingIntegration ? (
                  /* Configuration Editor Form */
                  <div className="bg-surface-container border border-outline-variant p-stack-md rounded-xl space-y-4 animate-fade-slide">
                    <div className="flex justify-between items-center pb-2 border-b border-outline-variant/30">
                      <h3 className="text-xs font-bold text-on-surface uppercase tracking-widest truncate">Configure {editingIntegration.name}</h3>
                      <button onClick={() => setEditingIntegration(null)} className="text-[10px] text-on-surface-variant hover:text-on-surface uppercase font-bold">Cancel</button>
                    </div>

                    <div className="space-y-3.5">
                      {editingIntegration.config_fields.map((field: any) => (
                        <div key={field.key}>
                          <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">
                            {field.label} {field.required && <span className="text-error">*</span>}
                          </label>
                          {field.type === 'boolean' ? (
                            <label className="flex items-center gap-2 cursor-pointer mt-1 select-none">
                              <input 
                                type="checkbox" 
                                checked={integrationConfig[field.key] === 'true'}
                                onChange={(e) => setIntegrationConfig(c => ({ ...c, [field.key]: String(e.target.checked) }))}
                                className="w-4 h-4 rounded border-outline-variant bg-surface-container-low accent-primary"
                              />
                              <span className="text-xs text-on-surface-variant/80">Enable feature</span>
                            </label>
                          ) : (
                            <input 
                              type={field.type === 'number' ? 'number' : 'text'}
                              value={integrationConfig[field.key] || ''}
                              onChange={(e) => setIntegrationConfig(c => ({ ...c, [field.key]: e.target.value }))}
                              className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                              placeholder={field.placeholder || `Enter ${field.label.toLowerCase()}`}
                            />
                          )}
                          {field.help_text && (
                            <p className="text-[9px] text-on-surface-variant/60 mt-1 leading-normal">{field.help_text}</p>
                          )}
                        </div>
                      ))}

                      {editingIntegration.supports_triggers && (
                        <>
                          <div className="border-t border-outline-variant/30 pt-3">
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1.5">Trigger Database Events</label>
                            <div className="flex gap-4">
                              {['insert', 'update', 'delete'].map(ev => {
                                const isChecked = integrationEvents.includes(ev);
                                return (
                                  <label key={ev} className="flex items-center gap-1.5 cursor-pointer text-xs text-on-surface select-none">
                                    <input 
                                      type="checkbox"
                                      checked={isChecked}
                                      onChange={(e) => {
                                        if (e.target.checked) {
                                          setIntegrationEvents(prev => [...prev, ev]);
                                        } else {
                                          setIntegrationEvents(prev => prev.filter(x => x !== ev));
                                        }
                                      }}
                                      className="w-3.5 h-3.5 rounded border-outline-variant bg-surface-container-low accent-primary"
                                    />
                                    <span className="capitalize">{ev}</span>
                                  </label>
                                );
                              })}
                            </div>
                          </div>

                          <div>
                            <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1">Filter Collections (Comma separated)</label>
                            <input 
                              type="text"
                              value={integrationCollections}
                              onChange={(e) => setIntegrationCollections(e.target.value)}
                              className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                              placeholder="e.g. posts, comments (leave empty for all)"
                            />
                          </div>
                        </>
                      )}

                      <div className="pt-2 flex gap-2">
                        {enabledIntegrations.some(e => e.id === editingIntegration.id) && (
                          <button
                            onClick={async () => {
                              if (confirm('Disable and remove this integration?')) {
                                try {
                                  const newList = enabledIntegrations.filter(e => e.id !== editingIntegration.id);
                                  await api(`/admin/projects/${encodeURIComponent(projectId)}/integrations`, {
                                    method: 'PUT',
                                    body: JSON.stringify({ integrations: newList })
                                  });
                                  showToast('Integration removed', 'success');
                                  setEditingIntegration(null);
                                  loadIntegrations();
                                } catch {
                                  showToast('Failed to remove integration', 'error');
                                }
                              }
                            }}
                            className="bg-surface-container-low border border-error/30 text-error px-4 py-2 rounded-lg text-xs font-bold active:scale-[0.98] transition-all"
                          >
                            Disable
                          </button>
                        )}
                        <button 
                          onClick={handleSaveIntegration}
                          disabled={savingIntegration}
                          className="flex-1 bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-[0.98] transition-all disabled:opacity-50"
                        >
                          {savingIntegration ? 'Saving...' : 'Save Integration'}
                        </button>
                      </div>
                    </div>
                  </div>
                ) : (
                  /* Catalog List */
                  <div className="space-y-3">
                    <p className="text-on-surface-variant font-label-caps text-[10px] tracking-widest">THIRD PARTY SERVICES CATALOG</p>
                    <div className="space-y-3.5">
                      {loadingIntegrations ? (
                        <div className="text-center py-8 text-on-surface-variant/50 text-xs">
                          <span className="material-symbols-outlined animate-spin text-[16px] mr-1.5 align-middle">sync</span> Loading integrations catalog...
                        </div>
                      ) : integrationsCatalog.length === 0 ? (
                        <p className="text-xs text-on-surface-variant/50 italic text-center py-6">No integrations available.</p>
                      ) : (
                        integrationsCatalog.map(plugin => {
                          const config = enabledIntegrations.find(e => e.id === plugin.id);
                          const isEnabled = !!config;
                          
                          return (
                            <div 
                              key={plugin.id} 
                              onClick={() => {
                                setEditingIntegration(plugin);
                                const cfg: Record<string, string> = {};
                                if (config) {
                                  for (const [k, v] of Object.entries(config.config || {})) {
                                    cfg[k] = typeof v === 'boolean' ? String(v) : String(v ?? '');
                                  }
                                }
                                setIntegrationConfig(cfg);
                                setIntegrationEvents(config?.events ? [...config.events] : []);
                                setIntegrationCollections(config?.collections ? config.collections.join(', ') : '');
                              }}
                              className="w-full text-left bg-surface-container-low border border-outline-variant hover:bg-surface-container-high/40 p-stack-md rounded-xl flex items-center justify-between transition-colors cursor-pointer"
                            >
                              <div className="flex items-center gap-3">
                                <span className="w-9 h-9 flex items-center justify-center rounded-lg border border-outline-variant/50 bg-surface-container-lowest text-primary">
                                  <span className="material-symbols-outlined text-[20px]">extension</span>
                                </span>
                                <div>
                                  <h4 className="text-xs font-bold text-on-surface">{plugin.name}</h4>
                                  <p className="text-[9px] text-on-surface-variant/75 truncate max-w-[190px]">{plugin.description}</p>
                                </div>
                              </div>
                              <span className={`text-[9px] font-bold px-2 py-0.5 rounded-full border shrink-0 ml-1 ${
                                isEnabled 
                                  ? 'bg-primary/10 border-primary/20 text-primary' 
                                  : 'bg-surface-container text-on-surface-variant/50 border-outline-variant/30'
                              }`}>
                                {isEnabled ? 'Linked' : 'Link'}
                              </span>
                            </div>
                          );
                        })
                      )}
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* ANALYTICS SUB-PAGE */}
            {activeSubPage === 'analytics' && (
              <div className="space-y-stack-md animate-fade-slide">
                <p className="text-on-surface-variant font-label-caps text-[10px] tracking-widest">METRICS OVERVIEW (LIVE TELEMETRY)</p>
                
                {projectUsage ? (
                  <div className="space-y-stack-md">
                    <div className="grid grid-cols-2 gap-stack-sm">
                      <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-1">
                        <span className="text-[8px] font-bold text-on-surface-variant uppercase block">Total API Requests</span>
                        <span className="text-md font-bold text-primary font-mono">{projectUsage.api_requests || 0}</span>
                        <span className="text-[8px] text-on-surface-variant/50">Hits since creation</span>
                      </div>
                      <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-1">
                        <span className="text-[8px] font-bold text-on-surface-variant uppercase block">Active WebSockets</span>
                        <span className="text-md font-bold text-secondary font-mono">{projectUsage.realtime_connections || 0}</span>
                        <span className="text-[8px] text-on-surface-variant/50">Realtime channels</span>
                      </div>
                      <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-1">
                        <span className="text-[8px] font-bold text-on-surface-variant uppercase block">DB Read Operations</span>
                        <span className="text-md font-bold text-primary font-mono">{projectUsage.db_reads_total || 0}</span>
                        <span className="text-[8px] text-on-surface-variant/50">Reads from disk</span>
                      </div>
                      <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-1">
                        <span className="text-[8px] font-bold text-on-surface-variant uppercase block">DB Write Operations</span>
                        <span className="text-md font-bold text-secondary font-mono">{projectUsage.db_writes_total || 0}</span>
                        <span className="text-[8px] text-on-surface-variant/50">Writes to disk</span>
                      </div>
                    </div>

                    {/* Storage info card */}
                    <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-2">
                      <div className="flex justify-between items-center border-b border-outline-variant/30 pb-1.5">
                        <h4 className="text-xs font-bold text-on-surface uppercase tracking-wider">Disk Storage Breakdown</h4>
                        <span className="text-[10px] font-bold text-primary font-mono">
                          {((projectUsage.total_storage_bytes || 0) / (1024 * 1024)).toFixed(2)} MB Total
                        </span>
                      </div>
                      <div className="space-y-1.5 text-[10px]">
                        <div className="flex justify-between">
                          <span className="text-on-surface-variant/80">Pebble Database size</span>
                          <span className="font-mono text-on-surface">
                            {((projectUsage.database_bytes || 0) / (1024 * 1024)).toFixed(2)} MB
                          </span>
                        </div>
                        <div className="flex justify-between">
                          <span className="text-on-surface-variant/80">User File storage size</span>
                          <span className="font-mono text-on-surface">
                            {((projectUsage.file_storage_bytes || 0) / (1024 * 1024)).toFixed(2)} MB
                          </span>
                        </div>
                      </div>
                    </div>

                    {/* --- EVENT ANALYTICS (24h) --- */}
                    {analyticsData ? (
                      <div className="space-y-stack-md">
                        {/* Total Events (24h) Summary Card */}
                        <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-1">
                          <span className="text-[8px] font-bold text-on-surface-variant uppercase block">Total Events (24h)</span>
                          <span className="text-md font-bold text-primary font-mono">{(analyticsData.total_events || 0).toLocaleString()}</span>
                          <span className="text-[8px] text-on-surface-variant/50">Custom analytics events</span>
                        </div>

                        {/* Hourly Activity Bar Chart */}
                        <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-3">
                          <h4 className="text-xs font-bold text-on-surface uppercase tracking-wider">Hourly Activity</h4>
                          {analyticsData.hourly && analyticsData.hourly.length > 0 ? (
                            <div className="space-y-1">
                              {(() => {
                                const maxCount = Math.max(...analyticsData.hourly.map((h: any) => h.count), 1);
                                return analyticsData.hourly.map((h: any) => (
                                  <div key={h.hour} className="flex items-center gap-2">
                                    <span className="w-6 text-right text-[9px] text-on-surface-variant/60 font-mono">
                                      {String(h.hour).padStart(2, '0')}
                                    </span>
                                    <div className="flex-1 h-4 bg-surface-container-high rounded-full overflow-hidden">
                                      <div
                                        className="bg-primary/30 h-full rounded-full flex items-center justify-end pr-1.5 min-w-[2px]"
                                        style={{ width: `${Math.max((h.count / maxCount) * 100, 0.5)}%` }}
                                      >
                                        {h.count > 0 && (h.count / maxCount) > 0.15 && (
                                          <span className="text-[7px] font-bold text-on-surface font-mono leading-none">{h.count}</span>
                                        )}
                                      </div>
                                    </div>
                                    {((h.count / maxCount) <= 0.15 || h.count === 0) && (
                                      <span className="w-8 text-left text-[8px] text-on-surface-variant/40 font-mono">{h.count > 0 ? h.count : ''}</span>
                                    )}
                                  </div>
                                ));
                              })()}
                              <div className="flex items-center gap-2 pt-1">
                                <span className="w-6" />
                                <div className="flex-1 flex justify-between text-[8px] text-on-surface-variant/40">
                                  <span>00</span><span>06</span><span>12</span><span>18</span><span>23</span>
                                </div>
                                <span className="w-8" />
                              </div>
                            </div>
                          ) : (
                            <p className="text-[10px] text-on-surface-variant/60 italic text-center py-2">No hourly data yet</p>
                          )}
                        </div>

                        {/* Top Events */}
                        {analyticsData.top_events && analyticsData.top_events.length > 0 && (
                          <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-3">
                            <h4 className="text-xs font-bold text-on-surface uppercase tracking-wider">Top Events</h4>
                            <div className="space-y-1.5">
                              {analyticsData.top_events.slice(0, 5).map((e: any, i: number) => (
                                <div key={i} className="flex justify-between items-center py-1">
                                  <span className="text-[10px] text-on-surface font-mono truncate mr-2">{e.event || e.name}</span>
                                  <span className="text-[10px] font-bold text-primary bg-primary/10 px-2 py-0.5 rounded-full font-mono">
                                    {(e.count || 0).toLocaleString()}
                                  </span>
                                </div>
                              ))}
                            </div>
                          </div>
                        )}
                      </div>
                    ) : (
                      <div className="text-center py-4 bg-surface-container/20 border border-dashed border-outline-variant rounded-xl text-[10px] text-on-surface-variant/40">
                        <span className="material-symbols-outlined animate-spin text-[14px] mr-1 align-middle">monitoring</span> Loading event analytics...
                      </div>
                    )}

                    {/* Live request method distribution bar chart */}
                    <div className="bg-surface-container-low border border-outline-variant p-stack-md rounded-xl space-y-3">
                      <h4 className="text-xs font-bold text-on-surface uppercase tracking-wider">HTTP Method Distribution</h4>
                      {Object.keys(projectUsage.requests_by_method || {}).length === 0 ? (
                        <p className="text-[10px] text-on-surface-variant/60 italic text-center py-2">No API requests recorded yet</p>
                      ) : (
                        <div className="space-y-2">
                          {Object.entries(projectUsage.requests_by_method || {}).map(([method, count]) => {
                            const countNum = Number(count);
                            const maxVal = Math.max(...Object.values(projectUsage.requests_by_method || {}).map(Number), 1);
                            const percent = (countNum / maxVal) * 100;
                            
                            // Method specific colors
                            const barColor = 
                              method === 'GET' ? 'bg-primary' : 
                              method === 'POST' ? 'bg-secondary' : 
                              method === 'DELETE' ? 'bg-error' : 'bg-on-tertiary-container';
                            
                            return (
                              <div key={method} className="space-y-1">
                                <div className="flex justify-between text-[10px]">
                                  <span className="font-bold text-on-surface font-mono">{method}</span>
                                  <span className="text-on-surface-variant/80 font-mono">{countNum} requests</span>
                                </div>
                                <div className="h-2 w-full bg-surface-container-high rounded-full overflow-hidden">
                                  <div className={`h-full ${barColor} rounded-full`} style={{ width: `${percent}%` }}></div>
                                </div>
                              </div>
                            );
                          })}
                        </div>
                      )}
                    </div>
                  </div>
                ) : (
                  <div className="text-center py-10 bg-surface-container/20 border border-dashed border-outline-variant rounded-xl text-xs text-on-surface-variant/40">
                    <span className="material-symbols-outlined animate-spin text-[16px] mr-1.5 align-middle">sync</span> Loading live usage records...
                  </div>
                )}
              </div>
            )}

            {/* API CLIENT CLIENT SUB-PAGE */}
            {activeSubPage === 'api' && (
              <div className="space-y-stack-md animate-fade-slide">
                <div className="flex border-b border-outline-variant/30 gap-4 mb-2">
                  {(['js', 'dart', 'curl'] as const).map(lang => (
                    <button
                      key={lang}
                      onClick={() => setApiLanguage(lang)}
                      className={`pb-1.5 text-xs font-bold uppercase tracking-wider font-label-caps ${apiLanguage === lang ? 'text-primary border-b border-primary' : 'text-on-surface-variant'}`}
                    >
                      {lang === 'js' ? 'Javascript' : lang === 'dart' ? 'Dart / Flutter' : 'cURL'}
                    </button>
                  ))}
                </div>

                <div className="bg-surface-container-low border border-outline-variant rounded-xl p-3 select-all overflow-x-auto">
                  <pre className="font-code-xs text-[10px] leading-relaxed text-secondary font-mono">
                    {apiLanguage === 'js' && `// Initialize JavaScript SDK\nimport { createClient } from '@sovrabase/sdk';\n\nconst sovrabase = createClient({\n  url: '${window.location.origin}/api/v1',\n  project: '${projectId}',\n  key: 'CLIENT_API_KEY'\n});\n\n// Read Documents\nconst data = await sovrabase.from('posts').select('*');`}
                    {apiLanguage === 'dart' && `// Initialize Dart client\nimport 'package:sovrabase/sovrabase.dart';\n\nfinal sovrabase = SovrabaseClient(\n  url: '${window.location.origin}/api/v1',\n  project: '${projectId}',\n  key: 'CLIENT_API_KEY'\n);\n\n// Query Stream\nfinal list = await sovrabase.from('posts').select();`}
                    {apiLanguage === 'curl' && `# Fetch all rows from posts collection\ncurl -X GET \\\n  -H "Authorization: Bearer CLIENT_API_KEY" \\\n  "${window.location.origin}/api/v1/collections/posts/documents"`}
                  </pre>
                </div>
              </div>
            )}

          </div>
        )}

      </main>

      {/* BOTTOM NAV BAR */}
      <nav className="absolute bottom-0 left-0 right-0 z-40 bg-surface/75 backdrop-blur-xl border-t border-outline-variant flex justify-around items-center h-16 px-2 pb-safe">
        {[
          { id: 'overview', label: 'Overview', icon: 'dashboard' },
          { id: 'database', label: 'Database', icon: 'database' },
          { id: 'auth', label: 'Auth', icon: 'lock' },
          { id: 'logs', label: 'Logs', icon: 'terminal' },
          { id: 'more', label: 'More', icon: 'menu_open' }
        ].map(tab => (
          <button
            key={tab.id}
            onClick={() => {
              setActiveTab(tab.id as any);
              setSwipedUserId(null);
              if (tab.id === 'more') {
                setActiveSubPage('grid');
              }
            }}
            className={`flex flex-col items-center justify-center w-14 h-full gap-1 transition-all duration-200 focus:outline-none active:scale-90 ${
              activeTab === tab.id ? 'text-primary' : 'text-on-surface-variant hover:text-on-surface'
            }`}
          >
            <span 
              className="material-symbols-outlined text-[20px]"
              style={{
                fontVariationSettings: activeTab === tab.id ? "'FILL' 1" : "'FILL' 0"
              }}
            >
              {tab.icon}
            </span>
            <span className="font-label-caps text-[9px] uppercase tracking-wider font-extrabold">{tab.label}</span>
          </button>
        ))}
      </nav>

      {activeModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-fade-in">
          <div className="bg-surface-container border border-outline-variant rounded-2xl p-6 w-full max-w-sm space-y-4 shadow-2xl animate-fade-slide">
            
            {activeModal === 'create_bucket' && (
              <>
                <h3 className="text-sm font-bold text-on-surface uppercase tracking-wider">Create Storage Bucket</h3>
                <div className="space-y-3">
                  <div>
                    <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Bucket Name</label>
                    <input 
                      type="text"
                      value={modalData.bucketName || ''}
                      onChange={(e) => setModalData(d => ({ ...d, bucketName: e.target.value }))}
                      className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                      placeholder="e.g. assets"
                    />
                  </div>
                  <div className="flex gap-2 pt-2">
                    <button onClick={() => setActiveModal(null)} className="flex-1 bg-surface-container-low border border-outline-variant/60 text-on-surface-variant py-2 rounded-lg text-xs font-bold active:scale-95 transition-all">Cancel</button>
                    <button 
                      onClick={async () => {
                        const name = modalData.bucketName?.trim();
                        if (!name) return;
                        try {
                          await api(`/admin/projects/${encodeURIComponent(projectId)}/storage/buckets`, {
                            method: 'POST',
                            body: JSON.stringify({ name })
                          });
                          showToast('Bucket created successfully', 'success');
                          loadBuckets();
                          setActiveModal(null);
                        } catch (err) {
                          showToast('Failed to create bucket', 'error');
                        }
                      }} 
                      className="flex-1 bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-95 transition-all"
                    >
                      Create
                    </button>
                  </div>
                </div>
              </>
            )}

            {activeModal === 'upload_file' && (
              <>
                <h3 className="text-sm font-bold text-on-surface uppercase tracking-wider">Create Test File</h3>
                <div className="space-y-3">
                  <div>
                    <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Filename</label>
                    <input 
                      type="text"
                      value={modalData.fileName || ''}
                      onChange={(e) => setModalData(d => ({ ...d, fileName: e.target.value }))}
                      className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                      placeholder="e.g. hello.txt"
                    />
                  </div>
                  <div>
                    <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Content</label>
                    <textarea 
                      value={modalData.fileContent || ''}
                      onChange={(e) => setModalData(d => ({ ...d, fileContent: e.target.value }))}
                      className="w-full h-24 bg-surface-container-low border border-outline-variant/60 rounded-lg p-2 text-xs text-on-surface font-mono"
                      placeholder="Hello Sovrabase storage!"
                    />
                  </div>
                  <div className="flex gap-2 pt-2">
                    <button onClick={() => setActiveModal(null)} className="flex-1 bg-surface-container-low border border-outline-variant/60 text-on-surface-variant py-2 rounded-lg text-xs font-bold active:scale-95 transition-all">Cancel</button>
                    <button 
                      onClick={async () => {
                        const name = modalData.fileName?.trim();
                        const content = modalData.fileContent || '';
                        if (!name || !selectedBucket) return;
                        try {
                          const formData = new FormData();
                          formData.append('file', new File([content], name, { type: 'text/plain' }));
                          await api(`/admin/projects/${encodeURIComponent(projectId)}/storage/buckets/${encodeURIComponent(selectedBucket)}/files`, {
                            method: 'POST',
                            body: formData
                          });
                          showToast('File uploaded successfully', 'success');
                          loadFiles(selectedBucket);
                          setActiveModal(null);
                        } catch (err) {
                          showToast('Upload failed', 'error');
                        }
                      }} 
                      className="flex-1 bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-95 transition-all"
                    >
                      Upload
                    </button>
                  </div>
                </div>
              </>
            )}

            {activeModal === 'invite_member' && (
              <>
                <h3 className="text-sm font-bold text-on-surface uppercase tracking-wider">Invite Team Member</h3>
                <div className="space-y-3">
                  <div>
                    <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Email Address</label>
                    <input 
                      type="email"
                      value={modalData.memberEmail || ''}
                      onChange={(e) => setModalData(d => ({ ...d, memberEmail: e.target.value }))}
                      className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                      placeholder="e.g. developer@company.com"
                    />
                  </div>
                  <div>
                    <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Role</label>
                    <select
                      value={modalData.memberRole || 'developer'}
                      onChange={(e) => setModalData(d => ({ ...d, memberRole: e.target.value }))}
                      className="w-full bg-surface-container-low border border-outline-variant/60 text-on-surface px-2.5 py-1.5 text-xs font-mono rounded-lg outline-none focus:border-primary/50"
                    >
                      <option value="owner">Owner</option>
                      <option value="admin">Admin</option>
                      <option value="developer">Developer</option>
                      <option value="member">Member</option>
                    </select>
                  </div>
                  <div className="flex gap-2 pt-2">
                    <button onClick={() => setActiveModal(null)} className="flex-1 bg-surface-container-low border border-outline-variant/60 text-on-surface-variant py-2 rounded-lg text-xs font-bold active:scale-95 transition-all">Cancel</button>
                    <button 
                      onClick={async () => {
                        const email = modalData.memberEmail?.trim();
                        const role = modalData.memberRole || 'developer';
                        if (!email) return;
                        try {
                          await api(`/admin/projects/${encodeURIComponent(projectId)}/invite`, {
                            method: 'POST',
                            body: JSON.stringify({ email, role })
                          });
                          showToast('Invitation sent successfully', 'success');
                          loadMembers();
                          setActiveModal(null);
                        } catch (err) {
                          showToast('Failed to invite member', 'error');
                        }
                      }} 
                      className="flex-1 bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-95 transition-all"
                    >
                      Invite
                    </button>
                  </div>
                </div>
              </>
            )}

            {activeModal === 'add_webhook' && (
              <>
                <h3 className="text-sm font-bold text-on-surface uppercase tracking-wider">Add Webhook Endpoint</h3>
                <div className="space-y-3">
                  <div>
                    <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Callback URL</label>
                    <input 
                      type="url"
                      value={modalData.webhookUrl || ''}
                      onChange={(e) => setModalData(d => ({ ...d, webhookUrl: e.target.value }))}
                      className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                      placeholder="https://api.my-app.com/webhooks"
                    />
                  </div>
                  <div>
                    <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Events (Comma separated)</label>
                    <input 
                      type="text"
                      value={modalData.webhookEvents || 'insert,update,delete'}
                      onChange={(e) => setModalData(d => ({ ...d, webhookEvents: e.target.value }))}
                      className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                    />
                  </div>
                  <div className="flex gap-2 pt-2">
                    <button onClick={() => setActiveModal(null)} className="flex-1 bg-surface-container-low border border-outline-variant/60 text-on-surface-variant py-2 rounded-lg text-xs font-bold active:scale-95 transition-all">Cancel</button>
                    <button 
                      onClick={async () => {
                        const url = modalData.webhookUrl?.trim();
                        const eventsStr = modalData.webhookEvents || 'insert,update,delete';
                        if (!url) return;
                        try {
                          const events = eventsStr.split(',').map(e => e.trim().toLowerCase()).filter(Boolean);
                          await api(`/admin/projects/${encodeURIComponent(projectId)}/webhooks`, {
                            method: 'POST',
                            body: JSON.stringify({ url, events, enabled: true })
                          });
                          showToast('Webhook registered successfully', 'success');
                          loadWebhooks();
                          setActiveModal(null);
                        } catch (err) {
                          showToast('Failed to register webhook', 'error');
                        }
                      }} 
                      className="flex-1 bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-95 transition-all"
                    >
                      Add
                    </button>
                  </div>
                </div>
              </>
            )}

            {activeModal === 'create_cron' && (
              <>
                <h3 className="text-sm font-bold text-on-surface uppercase tracking-wider">Add Scheduled Task</h3>
                <div className="space-y-3">
                  <div>
                    <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Task Name</label>
                    <input 
                      type="text"
                      value={modalData.cronName || ''}
                      onChange={(e) => setModalData(d => ({ ...d, cronName: e.target.value }))}
                      className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                      placeholder="e.g. sync-orders"
                    />
                  </div>
                  <div>
                    <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Schedule (Cron expression)</label>
                    <input 
                      type="text"
                      value={modalData.cronSchedule || '*/5 * * * *'}
                      onChange={(e) => setModalData(d => ({ ...d, cronSchedule: e.target.value }))}
                      className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                      placeholder="*/5 * * * *"
                    />
                  </div>
                  <div>
                    <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Callback URL</label>
                    <input 
                      type="url"
                      value={modalData.cronUrl || ''}
                      onChange={(e) => setModalData(d => ({ ...d, cronUrl: e.target.value }))}
                      className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                      placeholder="https://api.my-app.com/tasks/sync"
                    />
                  </div>
                  <div className="flex gap-2 pt-2">
                    <button onClick={() => setActiveModal(null)} className="flex-1 bg-surface-container-low border border-outline-variant/60 text-on-surface-variant py-2 rounded-lg text-xs font-bold active:scale-95 transition-all">Cancel</button>
                    <button 
                      onClick={async () => {
                        const name = modalData.cronName?.trim();
                        const schedule = modalData.cronSchedule?.trim();
                        const url = modalData.cronUrl?.trim();
                        if (!name || !schedule || !url) return;
                        try {
                          await api(`/admin/projects/${encodeURIComponent(projectId)}/cron`, {
                            method: 'POST',
                            body: JSON.stringify({ name, schedule, method: 'POST', url, enabled: true })
                          });
                          showToast('Scheduled task added', 'success');
                          loadCronJobs();
                          setActiveModal(null);
                        } catch (err) {
                          showToast('Failed to add task', 'error');
                        }
                      }} 
                      className="flex-1 bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-95 transition-all"
                    >
                      Add Task
                    </button>
                  </div>
                </div>
              </>
            )}

            {activeModal === 'add_document' && (
              <>
                <h3 className="text-sm font-bold text-on-surface uppercase tracking-wider">Add Document to {selectedCol}</h3>
                <div className="space-y-3">
                  <div>
                    <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Document JSON</label>
                    <textarea 
                      value={modalData.docJson || ''}
                      onChange={(e) => setModalData(d => ({ ...d, docJson: e.target.value }))}
                      className="w-full h-40 bg-surface-container-low border border-outline-variant/60 rounded-lg p-2.5 text-xs text-on-surface font-mono focus:outline-none focus:border-primary/50"
                      placeholder='{"title": "New Document"}'
                    />
                  </div>
                  <div className="flex gap-2 pt-2">
                    <button onClick={() => setActiveModal(null)} className="flex-1 bg-surface-container-low border border-outline-variant/60 text-on-surface-variant py-2 rounded-lg text-xs font-bold active:scale-95 transition-all">Cancel</button>
                    <button 
                      onClick={async () => {
                        const jsonStr = modalData.docJson?.trim();
                        if (!jsonStr || !selectedCol) return;
                        try {
                          const parsed = JSON.parse(jsonStr);
                          await api(`/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(selectedCol)}/documents`, {
                            method: 'POST',
                            body: JSON.stringify(parsed)
                          });
                          showToast('Document added successfully', 'success');
                          loadDocuments(selectedCol);
                          setActiveModal(null);
                        } catch (err) {
                          showToast((err as Error).message || 'Invalid JSON or request failed', 'error');
                        }
                      }} 
                      className="flex-1 bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-95 transition-all"
                    >
                      Save
                    </button>
                  </div>
                </div>
              </>
            )}

            {activeModal === 'import_json' && (
              <>
                <h3 className="text-sm font-bold text-on-surface uppercase tracking-wider">Import JSON into {selectedCol}</h3>
                <div className="space-y-3">
                  <div>
                    <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">JSON Array</label>
                    <textarea 
                      value={modalData.importJson || ''}
                      onChange={(e) => setModalData(d => ({ ...d, importJson: e.target.value }))}
                      className="w-full h-48 bg-surface-container-low border border-outline-variant/60 rounded-lg p-2.5 text-xs text-on-surface font-mono focus:outline-none focus:border-primary/50"
                      placeholder={`[\n  { "title": "Doc One" },\n  { "title": "Doc Two" }\n]`}
                    />
                  </div>
                  <div className="flex gap-2 pt-2">
                    <button onClick={() => setActiveModal(null)} className="flex-1 bg-surface-container-low border border-outline-variant/60 text-on-surface-variant py-2 rounded-lg text-xs font-bold active:scale-95 transition-all">Cancel</button>
                    <button 
                      onClick={async () => {
                        const jsonStr = modalData.importJson?.trim();
                        if (!jsonStr || !selectedCol) return;
                        try {
                          const parsed = JSON.parse(jsonStr);
                          if (!Array.isArray(parsed)) throw new Error('Expected a JSON array');
                          const res = await api<any>(`/admin/projects/${encodeURIComponent(projectId)}/collections/${encodeURIComponent(selectedCol)}/import`, {
                            method: 'POST',
                            body: JSON.stringify(parsed)
                          });
                          const count = res?.count || parsed.length;
                          showToast(`Imported ${count} document(s) successfully`, 'success');
                          loadDocuments(selectedCol);
                          setActiveModal(null);
                        } catch (err) {
                          showToast((err as Error).message || 'Invalid JSON array or request failed', 'error');
                        }
                      }} 
                      className="flex-1 bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-95 transition-all"
                    >
                      Import
                    </button>
                  </div>
                </div>
              </>
            )}

            {activeModal === 'create_collection' && (
              <>
                <h3 className="text-sm font-bold text-on-surface uppercase tracking-wider">Create New Collection</h3>
                <div className="space-y-3">
                  <div>
                    <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Collection Name</label>
                    <input 
                      type="text"
                      value={modalData.collectionName || ''}
                      onChange={(e) => setModalData(d => ({ ...d, collectionName: e.target.value }))}
                      className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                      placeholder="e.g. products"
                      onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); (e.target as HTMLInputElement).parentElement?.parentElement?.querySelector('button.bg-primary')?.dispatchEvent(new MouseEvent('click', { bubbles: true })); } }}
                    />
                  </div>
                  <div className="flex gap-2 pt-2">
                    <button onClick={() => setActiveModal(null)} className="flex-1 bg-surface-container-low border border-outline-variant/60 text-on-surface-variant py-2 rounded-lg text-xs font-bold active:scale-95 transition-all">Cancel</button>
                    <button 
                      onClick={async () => {
                        const name = modalData.collectionName?.trim();
                        if (!name) return;
                        try {
                          await api(`/admin/projects/${encodeURIComponent(projectId)}/collections`, {
                            method: 'POST',
                            body: JSON.stringify({ name })
                          });
                          showToast(`Collection "${name}" created`, 'success');
                          loadCollections();
                          setSelectedCol(name);
                          setActiveModal(null);
                        } catch (err) {
                          showToast('Failed to create collection', 'error');
                        }
                      }} 
                      className="flex-1 bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-95 transition-all"
                    >
                      Create
                    </button>
                  </div>
                </div>
              </>
            )}

            {activeModal === 'add_config' && (
              <>
                <h3 className="text-sm font-bold text-on-surface uppercase tracking-wider">Add Config Entry</h3>
                <div className="space-y-3">
                  <div>
                    <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Key</label>
                    <input 
                      type="text"
                      value={modalData.configKey || ''}
                      onChange={(e) => setModalData(d => ({ ...d, configKey: e.target.value }))}
                      className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                      placeholder="e.g. MY_API_KEY"
                    />
                  </div>
                  <div>
                    <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Type</label>
                    <select
                      value={modalData.configType || 'string'}
                      onChange={(e) => setModalData(d => ({ ...d, configType: e.target.value }))}
                      className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                    >
                      <option value="string">string</option>
                      <option value="number">number</option>
                      <option value="boolean">boolean</option>
                      <option value="json">json</option>
                    </select>
                  </div>
                  <div>
                    <label className="block text-[9px] font-bold text-on-surface-variant uppercase tracking-wider mb-1 font-mono">Value</label>
                    <input 
                      type="text"
                      value={modalData.configValue || ''}
                      onChange={(e) => setModalData(d => ({ ...d, configValue: e.target.value }))}
                      className="w-full bg-surface-container-low border border-outline-variant/60 rounded-lg px-2.5 py-1.5 text-xs text-on-surface font-mono"
                      placeholder="Enter value..."
                    />
                  </div>
                  <div className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      id="configPublic"
                      checked={modalData.configPublic === 'true'}
                      onChange={(e) => setModalData(d => ({ ...d, configPublic: e.target.checked ? 'true' : 'false' }))}
                      className="accent-primary"
                    />
                    <label htmlFor="configPublic" className="text-[9px] font-bold text-on-surface-variant uppercase tracking-wider font-mono">Public</label>
                  </div>
                  <div className="flex gap-2 pt-2">
                    <button onClick={() => setActiveModal(null)} className="flex-1 bg-surface-container-low border border-outline-variant/60 text-on-surface-variant py-2 rounded-lg text-xs font-bold active:scale-95 transition-all">Cancel</button>
                    <button 
                      onClick={async () => {
                        const key = modalData.configKey?.trim();
                        const value = modalData.configValue || '';
                        const type = modalData.configType || 'string';
                        const pub = modalData.configPublic === 'true';
                        if (!key) return;
                        try {
                          await api(`/admin/projects/${encodeURIComponent(projectId)}/config`, {
                            method: 'POST',
                            body: JSON.stringify({ key, value, type, public: pub })
                          });
                          showToast('Config entry added', 'success');
                          loadConfigEntries();
                          setActiveModal(null);
                        } catch (err) {
                          showToast('Failed to add config entry', 'error');
                        }
                      }} 
                      className="flex-1 bg-primary text-on-primary py-2 rounded-lg text-xs font-bold active:scale-95 transition-all"
                    >
                      Add
                    </button>
                  </div>
                </div>
              </>
            )}

          </div>
        </div>
      )}

    </div>
  );
}
