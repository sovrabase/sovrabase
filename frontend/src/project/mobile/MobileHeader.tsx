import { useMobileDashboard } from './context';

export default function MobileHeader() {
  const {
    projectName,
    projects,
    projectId,
    onClose,
    onChangeProject,
    showProjectDropdown,
    setShowProjectDropdown,
    activeTab,
    isStreaming,
    setIsStreaming,
  } = useMobileDashboard();

  return (
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
            title={isStreaming ? 'Pause Log Stream' : 'Resume Log Stream'}
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
  );
}
