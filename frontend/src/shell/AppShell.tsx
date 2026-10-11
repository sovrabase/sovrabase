import { useEffect, useRef, useState } from 'react';
import { NavLink, Outlet, useLocation, useMatch, useNavigate } from 'react-router-dom';
import { Activity, ChevronDown, Database, FolderKanban, Home, LogOut, Menu, Search, X } from 'lucide-react';
import { useAuth, useDashboard, useProjects } from '../store';
import { cx, Kbd } from '../ui';
import { Mark } from './Mark';
import { instanceNav, projectNav, projectPath, type NavGroup } from './nav';
import { CommandPalette } from './CommandPalette';

function initials(name: string) {
  return (
    name
      .split(/[\s_-]+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((w) => w[0]!.toUpperCase())
      .join('') || '?'
  );
}

export function ProjectAvatar({ name, size = 22 }: { name: string; size?: number }) {
  return (
    <span
      className="inline-flex items-center justify-center shrink-0 rounded-[6px] bg-accent-soft text-accent-2 font-bold"
      style={{ width: size, height: size, fontSize: Math.round(size * 0.48) }}
      aria-hidden
    >
      {initials(name)}
    </span>
  );
}

function NavGroups({ groups, onNavigate }: { groups: NavGroup[]; onNavigate?: () => void }) {
  return (
    <nav className="flex flex-col gap-px" aria-label="Main">
      {groups.map((g, i) => (
        <div key={g.label ?? i} className="flex flex-col gap-px">
          {g.label && <div className="text-[11.5px] font-semibold tracking-[.06em] uppercase text-ink-3 px-2.5 pt-4 pb-1.5">{g.label}</div>}
          {g.items.map((it) => (
            <NavLink
              key={it.to}
              to={it.to}
              end={it.end}
              onClick={onNavigate}
              className={({ isActive }) =>
                cx(
                  'flex items-center gap-2.5 h-8 px-2.5 rounded-[7px] text-sm font-medium transition-colors',
                  isActive
                    ? 'bg-surface text-ink shadow-[0_0_0_1px_var(--sv-line),0_1px_2px_rgba(13,16,23,.05)]'
                    : 'text-ink-2 hover:bg-sunken hover:text-ink',
                )
              }
            >
              <it.icon className="w-4 h-4 shrink-0 opacity-80" aria-hidden />
              {it.label}
            </NavLink>
          ))}
        </div>
      ))}
    </nav>
  );
}

function ProjectSwitcher({ projectId }: { projectId: string }) {
  const { projects, loadProjects } = useProjects();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const navigate = useNavigate();
  useEffect(() => {
    if (!projects.length) loadProjects();
  }, [projects.length, loadProjects]);
  useEffect(() => {
    if (!open) return;
    const h = (e: MouseEvent) => !ref.current?.contains(e.target as Node) && setOpen(false);
    document.addEventListener('mousedown', h);
    return () => document.removeEventListener('mousedown', h);
  }, [open]);
  const current = projects.find((p) => p.id === projectId);
  const name = current?.name ?? projectId;
  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        aria-haspopup="listbox"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
        className="w-full h-10 flex items-center gap-2 px-2.5 rounded-md border border-line-2 bg-surface hover:bg-sunken text-sm font-medium text-ink"
      >
        <ProjectAvatar name={name} />
        <span className="truncate flex-1 text-left">{name}</span>
        <ChevronDown className="w-4 h-4 text-ink-3" aria-hidden />
      </button>
      {open && (
        <div className="absolute z-40 left-0 right-0 mt-1 p-1 rounded-[10px] border border-line bg-surface shadow-pop animate-pop max-h-[320px] overflow-y-auto" role="listbox">
          {projects.map((p) => (
            <button
              key={p.id}
              role="option"
              aria-selected={p.id === projectId}
              type="button"
              onClick={() => {
                setOpen(false);
                navigate(projectPath(p.id));
              }}
              className={cx('w-full flex items-center gap-2 h-9 px-2 rounded-[7px] text-sm text-left hover:bg-sunken', p.id === projectId && 'bg-sunken')}
            >
              <ProjectAvatar name={p.name} />
              <span className="truncate">{p.name}</span>
            </button>
          ))}
          <div className="border-t border-line my-1" />
          <button
            type="button"
            onClick={() => {
              setOpen(false);
              navigate('/projects');
            }}
            className="w-full flex items-center gap-2 h-9 px-2 rounded-[7px] text-sm text-ink-2 hover:bg-sunken"
          >
            <FolderKanban className="w-4 h-4" aria-hidden /> All projects
          </button>
        </div>
      )}
    </div>
  );
}

function Sidebar({ projectId, onOpenPalette, onNavigate }: { projectId: string | null; onOpenPalette: () => void; onNavigate?: () => void }) {
  const { role, logout } = useAuth();
  const { stats } = useDashboard();
  const navigate = useNavigate();
  const groups = projectId ? projectNav(projectId) : instanceNav(role);
  const email = localStorage.getItem('sovrabase_admin_email') || 'Your account';
  return (
    <div className="flex flex-col gap-2.5 h-full px-3 py-3.5">
      <NavLink to="/projects" onClick={onNavigate} className="flex items-center gap-2.5 px-1.5 py-1 text-ink no-underline">
        <Mark />
        <b className="text-[15px]">Sovrabase</b>
        {stats?.region && <span className="ml-auto inline-flex h-[22px] items-center px-2 rounded-full bg-sunken text-[12px] text-ink-2">{stats.region}</span>}
      </NavLink>
      {projectId && <ProjectSwitcher projectId={projectId} />}
      <button
        type="button"
        onClick={onOpenPalette}
        className="w-full h-9 flex items-center justify-between px-3 rounded-md border border-line bg-surface text-sm text-ink-3 hover:border-line-2"
      >
        <span className="flex items-center gap-2">
          <Search className="w-4 h-4" aria-hidden /> Search or jump to
        </span>
        <Kbd>Ctrl K</Kbd>
      </button>
      <div className="flex-1 overflow-y-auto -mx-1 px-1">
        <NavGroups groups={groups} onNavigate={onNavigate} />
        {projectId && (
          <>
            <div className="text-[11.5px] font-semibold tracking-[.06em] uppercase text-ink-3 px-2.5 pt-4 pb-1.5">Instance</div>
            <NavLink to="/projects" end onClick={onNavigate} className="flex items-center gap-2.5 h-8 px-2.5 rounded-[7px] text-sm font-medium text-ink-2 hover:bg-sunken hover:text-ink">
              <FolderKanban className="w-4 h-4 opacity-80" aria-hidden /> All projects
            </NavLink>
          </>
        )}
      </div>
      <div className="flex items-center gap-2">
        <NavLink
          to="/account"
          onClick={onNavigate}
          className={({ isActive }) =>
            cx('flex-1 min-w-0 flex items-center gap-2.5 p-2 rounded-lg text-ink hover:bg-sunken', isActive && 'bg-surface shadow-[0_0_0_1px_var(--sv-line)]')
          }
        >
          <span className="w-[30px] h-[30px] rounded-full bg-[#0D1017] text-white text-[12px] font-bold flex items-center justify-center shrink-0" aria-hidden>
            {email[0]?.toUpperCase()}
          </span>
          <span className="flex flex-col min-w-0">
            <b className="text-[13px] truncate">{email}</b>
            <span className="text-[12px] text-ink-3 capitalize">{(role || 'admin').replace('_', ' ')}</span>
          </span>
        </NavLink>
        <button
          type="button"
          aria-label="Sign out"
          title="Sign out"
          onClick={() => {
            logout();
            navigate('/');
          }}
          className="w-9 h-9 inline-flex items-center justify-center rounded-md text-ink-3 hover:text-ink hover:bg-sunken"
        >
          <LogOut className="w-4 h-4" />
        </button>
      </div>
      {stats?.version && <p className="m-0 px-2 text-[11.5px] text-ink-3">v{stats.version}</p>}
    </div>
  );
}

function MobileTabs({ projectId, onSearch, onMore }: { projectId: string | null; onSearch: () => void; onMore: () => void }) {
  const tab = 'flex-1 min-h-[52px] flex flex-col items-center justify-center gap-0.5 text-[11.5px] font-medium';
  const link = ({ isActive }: { isActive: boolean }) => cx(tab, isActive ? 'text-accent' : 'text-ink-3');
  return (
    <nav className="md:hidden fixed bottom-0 inset-x-0 z-30 flex bg-surface border-t border-line px-1 pb-[max(env(safe-area-inset-bottom),6px)]" aria-label="Sections">
      {projectId ? (
        <>
          <NavLink to={projectPath(projectId)} end className={link}>
            <Home className="w-[22px] h-[22px]" aria-hidden /> Home
          </NavLink>
          <NavLink to={projectPath(projectId, 'database')} className={link}>
            <Database className="w-[22px] h-[22px]" aria-hidden /> Data
          </NavLink>
          <NavLink to={projectPath(projectId, 'logs')} className={link}>
            <Activity className="w-[22px] h-[22px]" aria-hidden /> Logs
          </NavLink>
        </>
      ) : (
        <NavLink to="/projects" className={link}>
          <FolderKanban className="w-[22px] h-[22px]" aria-hidden /> Projects
        </NavLink>
      )}
      <button type="button" onClick={onSearch} className={cx(tab, 'text-ink-3')}>
        <Search className="w-[22px] h-[22px]" aria-hidden /> Search
      </button>
      <button type="button" onClick={onMore} className={cx(tab, 'text-ink-3')}>
        <Menu className="w-[22px] h-[22px]" aria-hidden /> More
      </button>
    </nav>
  );
}

export function AppShell() {
  const match = useMatch('/projects/:id/*');
  const projectId = match?.params.id && match.params.id !== 'new' ? decodeURIComponent(match.params.id) : null;
  const [palette, setPalette] = useState(false);
  const [drawer, setDrawer] = useState(false);
  const location = useLocation();
  const { loadDashboard } = useDashboard();

  useEffect(() => {
    loadDashboard();
  }, [loadDashboard]);

  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        setPalette((p) => !p);
      }
    };
    window.addEventListener('keydown', h);
    return () => window.removeEventListener('keydown', h);
  }, []);

  useEffect(() => {
    setDrawer(false);
  }, [location.pathname]);

  return (
    <div className="min-h-screen bg-bg md:flex">
      <a href="#main" className="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-50 focus:px-3 focus:py-2 focus:bg-surface focus:rounded-md">
        Skip to content
      </a>
      <aside className="hidden md:block w-[248px] shrink-0 border-r border-line h-screen sticky top-0">
        <Sidebar projectId={projectId} onOpenPalette={() => setPalette(true)} />
      </aside>

      {drawer && (
        <div className="md:hidden fixed inset-0 z-40 flex" role="dialog" aria-modal="true" aria-label="Navigation">
          <div className="w-[300px] max-w-[85vw] h-full bg-bg border-r border-line animate-sheet relative">
            <button
              type="button"
              aria-label="Close menu"
              onClick={() => setDrawer(false)}
              className="absolute top-3.5 right-3 w-9 h-9 inline-flex items-center justify-center rounded-md text-ink-3 hover:bg-sunken z-10"
            >
              <X className="w-5 h-5" />
            </button>
            <Sidebar projectId={projectId} onOpenPalette={() => setPalette(true)} onNavigate={() => setDrawer(false)} />
          </div>
          <div className="flex-1 bg-[rgba(13,16,23,.32)]" onClick={() => setDrawer(false)} />
        </div>
      )}

      <main id="main" className="flex-1 min-w-0 pb-[72px] md:pb-0">
        <Outlet />
      </main>

      <MobileTabs projectId={projectId} onSearch={() => setPalette(true)} onMore={() => setDrawer(true)} />
      <CommandPalette open={palette} onClose={() => setPalette(false)} projectId={projectId} />
    </div>
  );
}
