import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, Loader2, AlertTriangle } from 'lucide-react';
import { api } from '../api';
import type { Project } from '../types';
import { TabBar } from '../components/TabBar';
import OverviewTab from './OverviewTab';
import TeamTab from './TeamTab';
import DatabaseTab from './DatabaseTab';
import AuthTab from './AuthTab';
import StorageTab from './StorageTab';
import ConfigTab from './ConfigTab';
import CronTab from './CronTab';
import WebhooksTab from './WebhooksTab';
import QueuesTab from './QueuesTab';
import AnalyticsTab from './AnalyticsTab';
import ApiTab from './ApiTab';
import LogsTab from './LogsTab';

const TABS = [
  { id: 'Overview', label: 'Overview' },
  { id: 'Team', label: 'Team' },
  { id: 'Database', label: 'Database' },
  { id: 'Auth', label: 'Auth' },
  { id: 'Storage', label: 'Storage' },
  { id: 'Config', label: 'Config' },
  { id: 'Cron', label: 'Cron' },
  { id: 'Webhooks', label: 'Webhooks' },
  { id: 'Queues', label: 'Queues' },
  { id: 'Analytics', label: 'Analytics' },
  { id: 'API', label: 'API' },
  { id: 'Logs', label: 'Logs' },
];

type Tab = typeof TABS[number]['id'];

export default function ProjectDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [project, setProject] = useState<Project | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<Tab>('Overview');

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    setError(null);
    api<Project>(`/admin/projects/${encodeURIComponent(id)}`)
      .then(setProject)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Loader2 className="w-6 h-6 text-accent animate-spin" />
        <span className="ml-2 text-text-secondary">Loading project...</span>
      </div>
    );
  }

  if (error || !project) {
    return (
      <div className="flex flex-col items-center justify-center py-20 gap-4">
        <AlertTriangle className="w-10 h-10 text-danger" />
        <p className="text-danger">{error || 'Project not found'}</p>
        <button onClick={() => navigate(-1)} className="px-4 py-2 bg-bg-input border border-border rounded-md text-text-primary hover:bg-border transition-colors">
          Go Back
        </button>
      </div>
    );
  }

  const renderTab = () => {
    const props = { projectId: id!, apiKey: project.api_key };
    switch (activeTab) {
      case 'Overview': return <OverviewTab {...props} />;
      case 'Team': return <TeamTab {...props} />;
      case 'Database': return <DatabaseTab {...props} />;
      case 'Auth': return <AuthTab {...props} />;
      case 'Storage': return <StorageTab {...props} />;
      case 'Config': return <ConfigTab {...props} />;
      case 'Cron': return <CronTab {...props} />;
      case 'Webhooks': return <WebhooksTab {...props} />;
      case 'Queues': return <QueuesTab {...props} />;
      case 'Analytics': return <AnalyticsTab {...props} />;
      case 'API': return <ApiTab {...props} />;
      case 'Logs': return <LogsTab {...props} />;
    }
  };

  return (
    <div className="max-w-7xl mx-auto px-4 py-6">
      {/* Header */}
      <div className="flex items-center gap-4 mb-6">
        <button onClick={() => navigate(-1)} className="p-2 rounded-md hover:bg-bg-input border border-transparent hover:border-border transition-colors text-text-secondary hover:text-text-primary" title="Back">
          <ArrowLeft className="w-5 h-5" />
        </button>
        <div>
          <h1 className="text-2xl font-bold text-text-primary">{project.name}</h1>
          <p className="text-text-muted text-sm mt-0.5">Project ID: {project.id}</p>
        </div>
      </div>

      {/* Tab Bar */}
      <div className="mb-6">
        <TabBar tabs={TABS} activeTab={activeTab} onClick={(t: string) => setActiveTab(t as Tab)} />
      </div>

      {/* Tab Content */}
      <div className="animate-fade-slide">{renderTab()}</div>
    </div>
  );
}
