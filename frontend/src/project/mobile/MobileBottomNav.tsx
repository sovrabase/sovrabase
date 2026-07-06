import { useMobileDashboard } from './context';

export default function MobileBottomNav() {
  const { activeTab, setActiveTab, setSwipedUserId, setActiveSubPage } = useMobileDashboard();
  return (
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
  );
}
