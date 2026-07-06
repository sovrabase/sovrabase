import { useState, useEffect, useCallback, useRef } from 'react';
import { api } from '../../../api';
import { useToast } from '../../../components/Toast';
import type { DatabaseDocument } from '../../../types';
import { useDashboard } from '../../../store';
import type {
  CollectionInfo,
  LogEntry,
  MobileDashboardProps,
  ProviderConfig,
  QueueInfo,
  UserRow,
} from '../types';

export function useMobileDashboardData({
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
  const [activeSubPage, setActiveSubPage] = useState<'grid' | 'storage' | 'team' | 'config' | 'webhooks' | 'cron' | 'functions' | 'queues' | 'integrations' | 'analytics' | 'api'>('grid');

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
    } catch {
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
    } catch {
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
    } catch {
      setUsers([]);
    }
  }, [projectId]);

  // Load and Stream Realtime Logs (From Server API logs)
  const loadInitialLogs = useCallback(async () => {
    try {
      const data = await api<{ entries: LogEntry[]; total: number }>(`/admin/projects/${encodeURIComponent(projectId)}/logs?limit=50`);
      setLogs(data.entries || []);
    } catch {
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
        } catch {
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
    } catch {
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
    } catch {
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
    } catch {
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
    } catch {
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
          config[field.key] = val === 'true';
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
  return {
    showToast, stats,
    activeTab, setActiveTab, showProjectDropdown, setShowProjectDropdown,
    collections, setCollections, selectedCol, setSelectedCol, docs, setDocs,
    expandedDocId, setExpandedDocId, selectCol, dbSearchQuery, setDbSearchQuery,
    authSubTab, setAuthSubTab, users, setUsers, swipedUserId, setSwipedUserId,
    userSearchQuery, setUserSearchQuery, isInviting, setIsInviting, inviteEmail, setInviteEmail,
    logs, setLogs, isStreaming, setIsStreaming, logFilter, setLogFilter,
    cronJobs, loadingJobs, invokingStates, activeSubPage, setActiveSubPage,
    buckets, setBuckets, selectedBucket, setSelectedBucket, files, loadingBuckets, loadingFiles,
    members, loadingMembers, webhooks, loadingWebhooks, projectDetail, setProjectDetail,
    savingConfig, setSavingConfig, configEntries, loadingConfigEntries, projectUsage, analyticsData,
    dbAnalysis, analyzing, queues, loadingQueues, apiLanguage, setApiLanguage,
    editingProvider, setEditingProvider, providerConfig, setProviderConfig, authProviders,
    editingPolicyCol, setEditingPolicyCol, policyForm, setPolicyForm,
    editingDocId, setEditingDocId, editingDocJson, setEditingDocJson, savingDocJson,
    integrationsCatalog, enabledIntegrations, loadingIntegrations, editingIntegration, setEditingIntegration,
    integrationConfig, setIntegrationConfig, integrationEvents, setIntegrationEvents,
    integrationCollections, setIntegrationCollections, savingIntegration,
    activeModal, setActiveModal, modalData, setModalData,
    loadCollections, loadDocuments, loadUsers, loadInitialLogs, loadCronJobs, loadBuckets, loadFiles,
    loadMembers, changeMemberRole, loadWebhooks, loadQueues, loadAuthProviders, loadProjectConfig,
    loadConfigEntries, loadProjectUsage, loadAnalytics, loadIntegrations,
    handleInvoke, unstickQueue, purgeQueue, handleClearLogs, runDbAnalysis,
    handleBanUser, handleResetUser, handleInviteUser, handleSaveProvider, handleSelectPolicyCol,
    handleSavePolicies, handleSaveDocJson, handleSaveIntegration,
    filteredUsers, filteredDocs, filteredLogs,
    projectId, projectName, projects, onClose, onChangeProject,
  };
}

export type MobileDashboardContextValue = ReturnType<typeof useMobileDashboardData>;
