import { useState, useEffect } from 'react';
import { NavLink, Outlet, useNavigate } from 'react-router-dom';
import { LayoutDashboard, FolderKanban, Settings, Puzzle, Flag, LogOut, Users, User, Menu, X } from 'lucide-react';
import { useAuth, useDashboard } from '../store';

const navItems = [
  { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/projects', label: 'Projects', icon: FolderKanban },
  { to: '/profile', label: 'Profile', icon: User },
];

const adminNavItems = [
  { to: '/settings', label: 'Settings', icon: Settings },
  { to: '/plugins', label: 'Plugins', icon: Puzzle },
];

const superAdminNavItems = [
  { to: '/members', label: 'All Users', icon: Users },
];

export function Layout() {
  const { logout, role } = useAuth();
  const { stats, loadDashboard } = useDashboard();
  const navigate = useNavigate();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  
  const isAdmin = role === 'admin' || role === 'super_admin' || !role; // default to admin for backward compat
  const isSuperAdmin = role === 'super_admin';

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
    <div className="flex flex-col md:flex-row h-screen bg-bg-app overflow-hidden">
      
      {/* Mobile Top Bar Header */}
      <header className="flex md:hidden items-center justify-between h-14 px-4 bg-bg-card border-b border-border shrink-0 w-full">
        <div className="flex items-center gap-3">
          <div className="flex items-center justify-center w-8 h-8 rounded-lg bg-accent/10">
            <Flag className="w-4 h-4 text-primary animate-pulse" />
          </div>
          <span className="text-gradient text-base font-bold">Sovrabase</span>
        </div>
        <button 
          onClick={() => setMobileMenuOpen(true)}
          className="p-2 rounded-lg text-text-secondary hover:text-text-primary hover:bg-bg-input active:scale-95 transition-all focus:outline-none"
        >
          <Menu className="w-5 h-5" />
        </button>
      </header>

      {/* Mobile Menu Drawer Overlay */}
      {mobileMenuOpen && (
        <div className="fixed inset-0 z-[999] flex md:hidden bg-black/60 backdrop-blur-sm animate-fade-slide">
          {/* Drawer container */}
          <div className="w-[260px] bg-bg-card border-r border-border h-full flex flex-col p-4 space-y-4 animate-modal-in">
            {/* Drawer Header */}
            <div className="flex justify-between items-center pb-4 border-b border-border">
              <div className="flex items-center gap-3">
                <div className="flex items-center justify-center w-8 h-8 rounded-lg bg-accent/10">
                  <Flag className="w-4 h-4 text-primary animate-pulse" />
                </div>
                <span className="text-gradient text-base font-bold">Sovrabase</span>
              </div>
              <button 
                onClick={() => setMobileMenuOpen(false)} 
                className="p-1.5 hover:bg-bg-input rounded text-text-secondary hover:text-text-primary active:scale-95 transition-all"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {/* Drawer Nav links */}
            <nav className="flex-grow flex flex-col gap-1 overflow-y-auto">
              {navItems.map((item) => (
                <NavLink key={item.to} to={item.to} onClick={() => setMobileMenuOpen(false)} className={linkClass}>
                  <item.icon className="w-5 h-5" />
                  {item.label}
                </NavLink>
              ))}
              {isAdmin && adminNavItems.map((item) => (
                <NavLink key={item.to} to={item.to} onClick={() => setMobileMenuOpen(false)} className={linkClass}>
                  <item.icon className="w-5 h-5" />
                  {item.label}
                </NavLink>
              ))}
              {isSuperAdmin && superAdminNavItems.map((item) => (
                <NavLink key={item.to} to={item.to} onClick={() => setMobileMenuOpen(false)} className={linkClass}>
                  <item.icon className="w-5 h-5" />
                  {item.label}
                </NavLink>
              ))}
            </nav>

            {/* Drawer Footer */}
            <div className="pt-4 border-t border-border space-y-3">
              {stats && (
                <div className="text-[10px] text-text-muted space-y-0.5">
                  <p>v{stats.version}</p>
                  <p className="uppercase">{stats.region}</p>
                </div>
              )}
              <button
                onClick={() => {
                  setMobileMenuOpen(false);
                  handleLogout();
                }}
                className="flex items-center gap-2 w-full px-3 py-2 rounded-lg text-sm text-text-muted hover:text-danger hover:bg-danger/10 transition-colors"
              >
                <LogOut className="w-4 h-4" />
                Logout
              </button>
            </div>
          </div>
          {/* Click outside to close */}
          <div className="flex-grow h-full" onClick={() => setMobileMenuOpen(false)}></div>
        </div>
      )}

      {/* Desktop Sidebar (hidden on mobile) */}
      <aside className="hidden md:flex w-[220px] shrink-0 border-r border-border flex flex-col bg-bg-card">
        {/* Logo */}
        <div className="px-4 py-5 border-b border-border">
          <div className="flex items-center gap-3">
            <div className="flex items-center justify-center w-9 h-9 rounded-lg bg-accent/10 shadow-[0_0_10px_rgba(78,222,163,0.15)] border border-primary/20">
              <Flag className="w-5 h-5 text-primary animate-pulse" />
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
          {isAdmin && adminNavItems.map((item) => (
            <NavLink key={item.to} to={item.to} className={linkClass}>
              <item.icon className="w-5 h-5" />
              {item.label}
            </NavLink>
          ))}
          {isSuperAdmin && superAdminNavItems.map((item) => (
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
      <main className="flex-1 overflow-y-auto p-4 md:p-8">
        <Outlet />
      </main>
    </div>
  );
}
