import { NavLink, Outlet, useNavigate } from 'react-router-dom';
import { LayoutDashboard, FolderKanban, Settings, Puzzle, Flag, LogOut } from 'lucide-react';
import { useAuth, useDashboard } from '../store';
import { useEffect } from 'react';

const navItems = [
  { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/projects', label: 'Projects', icon: FolderKanban },
  { to: '/settings', label: 'Settings', icon: Settings },
  { to: '/plugins', label: 'Plugins', icon: Puzzle },
];

export function Layout() {
  const { logout } = useAuth();
  const { stats, loadDashboard } = useDashboard();
  const navigate = useNavigate();

  useEffect(() => {
    loadDashboard();
  }, [loadDashboard]);

  const handleLogout = () => {
    logout();
    navigate('/');
  };

  const linkClass = ({ isActive }: { isActive: boolean }) =>
    `flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-colors ${
      isActive
        ? 'bg-accent/15 text-accent'
        : 'text-text-secondary hover:text-text-primary hover:bg-bg-input'
    }`;

  return (
    <div className="flex h-screen bg-bg-app">
      {/* Sidebar */}
      <aside className="w-[220px] shrink-0 border-r border-border flex flex-col bg-bg-card">
        {/* Logo */}
        <div className="px-4 py-5 border-b border-border">
          <div className="flex items-center gap-3">
            <div className="flex items-center justify-center w-9 h-9 rounded-lg bg-accent/10">
              <Flag className="w-5 h-5" style={{ color: '#003399' }} />
            </div>
            <div>
              <span className="text-gradient text-lg font-bold">Sovrabase</span>
            </div>
          </div>
        </div>

        {/* Nav */}
        <nav className="flex-1 px-3 py-4 flex flex-col gap-1 overflow-y-auto">
          {navItems.map((item) => (
            <NavLink key={item.to} to={item.to} className={linkClass}>
              <item.icon className="w-5 h-5" />
              {item.label}
            </NavLink>
          ))}
        </nav>

        {/* Footer */}
        <div className="px-4 py-4 border-t border-border space-y-3">
          {stats && (
            <div className="text-xs text-text-muted space-y-0.5">
              <p>v{stats.version}</p>
              <p className="uppercase">{stats.region}</p>
            </div>
          )}
          <button
            onClick={handleLogout}
            className="flex items-center gap-2 w-full px-3 py-2 rounded-lg text-sm text-text-muted hover:text-danger hover:bg-danger/10 transition-colors"
          >
            <LogOut className="w-4 h-4" />
            Logout
          </button>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-y-auto">
        <Outlet />
      </main>
    </div>
  );
}
