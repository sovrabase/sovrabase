import { useMobileDashboard } from '../context';

export default function MobileMoreGrid() {
  const {
    setActiveSubPage,
    loadCronJobs, loadBuckets, loadMembers, loadWebhooks, loadProjectConfig, loadConfigEntries,
    loadProjectUsage, loadAnalytics
  } = useMobileDashboard();
  return (
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
                        { id: 'functions', name: 'Functions', desc: 'Serverless JS & Go', icon: 'bolt', color: 'text-secondary bg-secondary/10 border-secondary/20' },
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
  );
}
