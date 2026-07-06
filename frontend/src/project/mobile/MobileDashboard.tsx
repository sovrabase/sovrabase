import type { MobileDashboardProps } from './types';
import { MobileDashboardProvider, useMobileDashboard } from './context';
import MobileHeader from './MobileHeader';
import MobileBottomNav from './MobileBottomNav';
import MobileOverviewTab from './tabs/MobileOverviewTab';
import MobileDatabaseTab from './tabs/MobileDatabaseTab';
import MobileAuthTab from './tabs/MobileAuthTab';
import MobileLogsTab from './tabs/MobileLogsTab';
import MobileMoreGrid from './more/MobileMoreGrid';
import MobileStoragePage from './more/MobileStoragePage';
import MobileTeamPage from './more/MobileTeamPage';
import MobileConfigPage from './more/MobileConfigPage';
import MobileWebhooksPage from './more/MobileWebhooksPage';
import MobileCronPage from './more/MobileCronPage';
import MobileQueuesPage from './more/MobileQueuesPage';
import MobileIntegrationsPage from './more/MobileIntegrationsPage';
import MobileAnalyticsPage from './more/MobileAnalyticsPage';
import MobileApiPage from './more/MobileApiPage';
import MobileModals from './modals/MobileModals';
import FunctionsTab from '../FunctionsTab';

export default function MobileDashboard(props: MobileDashboardProps) {
  return (
    <MobileDashboardProvider {...props}>
      <MobileDashboardShell />
    </MobileDashboardProvider>
  );
}

function MobileDashboardShell() {
  const { activeTab, activeSubPage, projectId, projectDetail } = useMobileDashboard();

  return (
    <div className="flex flex-col h-full bg-surface text-on-surface select-none font-body-md relative overflow-hidden">
      <MobileHeader />

      <main className="flex-1 pt-20 pb-24 overflow-y-auto no-scrollbar bg-background">
        {activeTab === 'overview' && <MobileOverviewTab />}
        {activeTab === 'database' && <MobileDatabaseTab />}
        {activeTab === 'auth' && <MobileAuthTab />}
        {activeTab === 'logs' && <MobileLogsTab />}
        {activeTab === 'more' && (
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
        )}
      </main>

      <MobileBottomNav />
      <MobileModals />
    </div>
  );
}
