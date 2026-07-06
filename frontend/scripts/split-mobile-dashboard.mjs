import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.join(__dirname, '..', 'src', 'project');
const srcPath = path.join(root, 'MobileDashboard.tsx');
const mobileDir = path.join(root, 'mobile');

// Read original from backup if already re-exported
let srcContent = fs.readFileSync(srcPath, 'utf8');
if (srcContent.trim().startsWith('export { default }')) {
  const backup = path.join(root, 'MobileDashboard.tsx.bak');
  if (!fs.existsSync(backup)) {
    console.error('Original MobileDashboard.tsx already replaced; restore from git');
    process.exit(1);
  }
  srcContent = fs.readFileSync(backup, 'utf8');
} else {
  fs.writeFileSync(path.join(root, 'MobileDashboard.tsx.bak'), srcContent, 'utf8');
}

const lines = srcContent.split(/\r?\n/);

function slice(start, end) {
  return lines.slice(start - 1, end).join('\n');
}

function write(rel, content) {
  const p = path.join(mobileDir, rel);
  fs.mkdirSync(path.dirname(p), { recursive: true });
  fs.writeFileSync(p, content, 'utf8');
}

const CONTEXT_KEYS = [
  'showToast', 'stats', 'activeTab', 'setActiveTab', 'showProjectDropdown', 'setShowProjectDropdown',
  'collections', 'setCollections', 'selectedCol', 'setSelectedCol', 'docs', 'setDocs',
  'expandedDocId', 'setExpandedDocId', 'selectCol', 'dbSearchQuery', 'setDbSearchQuery',
  'authSubTab', 'setAuthSubTab', 'users', 'setUsers', 'swipedUserId', 'setSwipedUserId',
  'userSearchQuery', 'setUserSearchQuery', 'isInviting', 'setIsInviting', 'inviteEmail', 'setInviteEmail',
  'logs', 'setLogs', 'isStreaming', 'setIsStreaming', 'logFilter', 'setLogFilter',
  'cronJobs', 'loadingJobs', 'invokingStates', 'activeSubPage', 'setActiveSubPage',
  'buckets', 'setBuckets', 'selectedBucket', 'setSelectedBucket', 'files', 'loadingBuckets', 'loadingFiles',
  'members', 'loadingMembers', 'webhooks', 'loadingWebhooks', 'projectDetail', 'setProjectDetail',
  'savingConfig', 'setSavingConfig', 'configEntries', 'loadingConfigEntries', 'projectUsage', 'analyticsData',
  'dbAnalysis', 'analyzing', 'queues', 'loadingQueues', 'apiLanguage', 'setApiLanguage',
  'editingProvider', 'setEditingProvider', 'providerConfig', 'setProviderConfig', 'authProviders',
  'editingPolicyCol', 'setEditingPolicyCol', 'policyForm', 'setPolicyForm',
  'editingDocId', 'setEditingDocId', 'editingDocJson', 'setEditingDocJson', 'savingDocJson',
  'integrationsCatalog', 'enabledIntegrations', 'loadingIntegrations', 'editingIntegration', 'setEditingIntegration',
  'integrationConfig', 'setIntegrationConfig', 'integrationEvents', 'setIntegrationEvents',
  'integrationCollections', 'setIntegrationCollections', 'savingIntegration',
  'activeModal', 'setActiveModal', 'modalData', 'setModalData',
  'loadCollections', 'loadDocuments', 'loadUsers', 'loadInitialLogs', 'loadCronJobs', 'loadBuckets', 'loadFiles',
  'loadMembers', 'changeMemberRole', 'loadWebhooks', 'loadQueues', 'loadAuthProviders', 'loadProjectConfig',
  'loadConfigEntries', 'loadProjectUsage', 'loadAnalytics', 'loadIntegrations',
  'handleInvoke', 'unstickQueue', 'purgeQueue', 'handleClearLogs', 'runDbAnalysis',
  'handleBanUser', 'handleResetUser', 'handleInviteUser', 'handleSaveProvider', 'handleSelectPolicyCol',
  'handleSavePolicies', 'handleSaveDocJson', 'handleSaveIntegration',
  'renderFormattedJson', 'filteredUsers', 'filteredDocs', 'filteredLogs', 'projectId',
];

function destructureBlock(body) {
  const used = CONTEXT_KEYS.filter((k) => new RegExp(`\\b${k}\\b`).test(body));
  if (used.length === 0) return '  const {} = useMobileDashboard();';
  const perLine = 6;
  const lines = [];
  for (let i = 0; i < used.length; i += perLine) {
    lines.push('    ' + used.slice(i, i + perLine).join(', ') + (i + perLine < used.length ? ',' : ''));
  }
  return `  const {\n${lines.join('\n')}\n  } = useMobileDashboard();`;
}

function wrapComponent(name, body, extraImports = '') {
  const transformed = body
    .replace(/renderProviderIcon\(([^,)]+),\s*"([^"]+)"\)/g, '<ProviderIcon name={$1} className="$2" />')
    .replace(/renderProviderIcon\(([^,)]+),\s*'([^']+)'\)/g, "<ProviderIcon name={$1} className='$2' />")
    .replace(/renderProviderIcon\(([^)]+)\)/g, '<ProviderIcon name={$1} />');
  const destructure = destructureBlock(transformed);
  return `${extraImports}import { useMobileDashboard } from '../context';

export default function ${name}() {
${destructure}
  return (
${transformed.split('\n').map((l) => '    ' + l).join('\n')}
  );
}
`;
}

function wrapComponentMore(name, body) {
  const transformed = body
    .replace(/renderProviderIcon\(/g, 'ProviderIcon(');
  const destructure = destructureBlock(transformed);
  return `import { api } from '../../api';
import { useMobileDashboard } from '../context';

export default function ${name}() {
${destructure}
  return (
${transformed.split('\n').map((l) => '    ' + l).join('\n')}
  );
}
`;
}

// types.ts
write('types.ts', `import type { Project } from '../../types';

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
`);

// ProviderIcon
const providerBody = slice(43, 112)
  .replace(/^const renderProviderIcon = \(name: string, className: string = "w-4 h-4"\) => \{/, '')
  .replace(/\n\};?\s*$/, '');
write('ProviderIcon.tsx', `export function ProviderIcon({ name, className = 'w-4 h-4' }: { name: string; className?: string }) {
${providerBody}
}
`);

// utils
const utilsBody = slice(781, 803)
  .replace(/^  const renderFormattedJson = \(obj: any\) => \{/, '')
  .replace(/\n  \};?\s*$/, '');
write('utils.ts', `export function renderFormattedJson(obj: unknown) {
${utilsBody}
}
`);

// Hook - remove inner interface ProviderConfig
let hookBody = slice(122, 822);
hookBody = hookBody.replace(/\n  interface ProviderConfig \{[\s\S]*?\n  \}\n/, '\n');

write('hooks/useMobileDashboardData.ts', `import { useState, useEffect, useCallback, useRef } from 'react';
import { api } from '../../api';
import { useToast } from '../../components/Toast';
import type { DatabaseDocument } from '../../types';
import { useDashboard } from '../../store';
import { renderFormattedJson } from '../utils';
import type {
  CollectionInfo,
  LogEntry,
  ProviderConfig,
  QueueInfo,
  UserRow,
} from '../types';

export function useMobileDashboardData(projectId: string) {
${hookBody}
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
    renderFormattedJson, filteredUsers, filteredDocs, filteredLogs, projectId,
  };
}

export type MobileDashboardContextValue = ReturnType<typeof useMobileDashboardData>;
`);

write('context.tsx', `import { createContext, useContext, type ReactNode } from 'react';
import { useMobileDashboardData, type MobileDashboardContextValue } from './hooks/useMobileDashboardData';

const MobileDashboardContext = createContext<MobileDashboardContextValue | null>(null);

export function MobileDashboardProvider({ projectId, children }: { projectId: string; children: ReactNode }) {
  const value = useMobileDashboardData(projectId);
  return <MobileDashboardContext.Provider value={value}>{children}</MobileDashboardContext.Provider>;
}

export function useMobileDashboard(): MobileDashboardContextValue {
  const ctx = useContext(MobileDashboardContext);
  if (!ctx) throw new Error('useMobileDashboard must be used within MobileDashboardProvider');
  return ctx;
}
`);

// Tabs - strip outer conditional wrapper lines
function stripTabWrapper(body, tabCondition) {
  let b = body.trim();
  if (b.startsWith(`{activeTab === '${tabCondition}' && (`)) {
    b = b.replace(new RegExp(`^\\{activeTab === '${tabCondition}' && \\(\\s*`), '');
    b = b.replace(/\)\s*\}\s*$/, '');
  }
  return b.trim();
}

write('tabs/MobileOverviewTab.tsx', wrapComponent(
  'MobileOverviewTab',
  stripTabWrapper(slice(902, 1033), 'overview'),
  "import { formatBytes } from '../../api';\n"
));

write('tabs/MobileDatabaseTab.tsx', wrapComponent(
  'MobileDatabaseTab',
  stripTabWrapper(slice(1036, 1268), 'database'),
  "import { api } from '../../api';\n"
));

write('tabs/MobileAuthTab.tsx', wrapComponent(
  'MobileAuthTab',
  stripTabWrapper(slice(1271, 1768), 'auth'),
  "import { api } from '../../api';\nimport { ProviderIcon } from '../ProviderIcon';\n"
));

write('tabs/MobileLogsTab.tsx', wrapComponent(
  'MobileLogsTab',
  stripTabWrapper(slice(1771, 1899), 'logs'),
  ''
));

write('more/MobileMoreGrid.tsx', wrapComponentMore(
  'MobileMoreGrid',
  slice(1906, 1948).trim()
));

const morePages = [
  ['MobileStoragePage', 1951, 2086],
  ['MobileTeamPage', 2089, 2154],
  ['MobileConfigPage', 2157, 2302],
  ['MobileWebhooksPage', 2305, 2364],
  ['MobileCronPage', 2366, 2434],
  ['MobileQueuesPage', 2443, 2483],
  ['MobileIntegrationsPage', 2486, 2659],
  ['MobileAnalyticsPage', 2662, 2829],
  ['MobileApiPage', 2832, 2854],
];

for (const [name, start, end] of morePages) {
  write(`more/${name}.tsx`, wrapComponentMore(name, slice(start, end).trim()));
}

write('more/MobileMoreSection.tsx', `import { useMobileDashboard } from '../context';
import MobileMoreGrid from './MobileMoreGrid';
import MobileStoragePage from './MobileStoragePage';
import MobileTeamPage from './MobileTeamPage';
import MobileConfigPage from './MobileConfigPage';
import MobileWebhooksPage from './MobileWebhooksPage';
import MobileCronPage from './MobileCronPage';
import MobileQueuesPage from './MobileQueuesPage';
import MobileIntegrationsPage from './MobileIntegrationsPage';
import MobileAnalyticsPage from './MobileAnalyticsPage';
import MobileApiPage from './MobileApiPage';
import FunctionsTab from '../FunctionsTab';

export default function MobileMoreSection() {
  const { activeSubPage, projectId, projectDetail } = useMobileDashboard();
  return (
    <div className="p-container-padding space-y-stack-md animate-fade-slide">
      {activeSubPage === 'grid' && <MobileMoreGrid />}
      {activeSubPage === 'storage' && <MobileStoragePage />}
      {activeSubPage === 'team' && <MobileTeamPage />}
      {activeSubPage === 'config' && <MobileConfigPage />}
      {activeSubPage === 'webhooks' && <MobileWebhooksPage />}
      {activeSubPage === 'cron' && <MobileCronPage />}
      {activeSubPage === 'functions' && (
        <div className="animate-fade-slide px-1">
          <FunctionsTab projectId={projectId} apiKey={projectDetail?.api_key} />
        </div>
      )}
      {activeSubPage === 'queues' && <MobileQueuesPage />}
      {activeSubPage === 'integrations' && <MobileIntegrationsPage />}
      {activeSubPage === 'analytics' && <MobileAnalyticsPage />}
      {activeSubPage === 'api' && <MobileApiPage />}
    </div>
  );
}
`);

// Modals
const modalBody = slice(2900, 3366).trim();
write('modals/MobileModals.tsx', `import { api } from '../../api';
import { useMobileDashboard } from '../context';

export default function MobileModals() {
${destructureBlock(modalBody)}
  if (!activeModal) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-fade-in">
      <div className="bg-surface-container border border-outline-variant rounded-2xl p-6 w-full max-w-sm space-y-4 shadow-2xl animate-fade-slide">
${modalBody.split('\n').map((l) => '        ' + l).join('\n')}
      </div>
    </div>
  );
}
`);

// Header - needs props for projectName, projects, onChangeProject, onClose
const headerBody = slice(828, 896).trim();
write('MobileHeader.tsx', `import type { MobileDashboardProps } from './types';
import { useMobileDashboard } from './context';

type HeaderProps = Pick<MobileDashboardProps, 'projectName' | 'projects' | 'projectId' | 'onClose' | 'onChangeProject'>;

export default function MobileHeader({ projectName, projects, projectId, onClose, onChangeProject }: HeaderProps) {
  const {
    showProjectDropdown, setShowProjectDropdown, activeTab, isStreaming, setIsStreaming,
  } = useMobileDashboard();
  return (
${headerBody.split('\n').map((l) => '    ' + l).join('\n')}
  );
}
`);

write('MobileBottomNav.tsx', `import { useMobileDashboard } from './context';

export default function MobileBottomNav() {
  const { activeTab, setActiveTab, setSwipedUserId, setActiveSubPage } = useMobileDashboard();
  return (
${slice(2862, 2894).split('\n').map((l) => '    ' + l).join('\n')}
  );
}
`);

write('MobileDashboard.tsx', `import type { MobileDashboardProps } from './types';
import { MobileDashboardProvider, useMobileDashboard } from './context';
import MobileHeader from './MobileHeader';
import MobileBottomNav from './MobileBottomNav';
import MobileOverviewTab from './tabs/MobileOverviewTab';
import MobileDatabaseTab from './tabs/MobileDatabaseTab';
import MobileAuthTab from './tabs/MobileAuthTab';
import MobileLogsTab from './tabs/MobileLogsTab';
import MobileMoreSection from './more/MobileMoreSection';
import MobileModals from './modals/MobileModals';

export default function MobileDashboard(props: MobileDashboardProps) {
  return (
    <MobileDashboardProvider projectId={props.projectId}>
      <MobileDashboardShell {...props} />
    </MobileDashboardProvider>
  );
}

function MobileDashboardShell({
  projectId,
  projectName,
  projects,
  onClose,
  onChangeProject,
}: MobileDashboardProps) {
  const { activeTab } = useMobileDashboard();
  return (
    <div className="flex flex-col h-full bg-surface text-on-surface select-none font-body-md relative overflow-hidden">
      <MobileHeader
        projectId={projectId}
        projectName={projectName}
        projects={projects}
        onClose={onClose}
        onChangeProject={onChangeProject}
      />
      <main className="flex-1 pt-20 pb-24 overflow-y-auto no-scrollbar bg-background">
        {activeTab === 'overview' && <MobileOverviewTab />}
        {activeTab === 'database' && <MobileDatabaseTab />}
        {activeTab === 'auth' && <MobileAuthTab />}
        {activeTab === 'logs' && <MobileLogsTab />}
        {activeTab === 'more' && <MobileMoreSection />}
      </main>
      <MobileBottomNav />
      <MobileModals />
    </div>
  );
}
`);

fs.writeFileSync(srcPath, "export { default } from './mobile/MobileDashboard';\n", 'utf8');
console.log('Split complete');
