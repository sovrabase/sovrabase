import { create } from 'zustand';
import type {
  Project,
  DashboardStats,
  UsageStats,
  ReplicationInfo,
  PluginInfo,
} from './types';
import { api, hasToken, setToken, clearToken } from './api';

// ===== Auth =====
interface AuthState {
  isAuthenticated: boolean;
  role: string | null;
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
  checkAuth: () => boolean;
}

const getRole = () => localStorage.getItem('sovrabase_admin_role');
const setRole = (role: string) => localStorage.setItem('sovrabase_admin_role', role);

export const useAuth = create<AuthState>((set) => ({
  isAuthenticated: hasToken(),
  role: getRole(),
  login: async (email: string, password: string) => {
    const data = await api<{ token: string; role?: string }>('/admin/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    });
    setToken(data.token);
    if (data.role) setRole(data.role);
    set({ isAuthenticated: true, role: data.role || null });
  },
  logout: () => {
    clearToken();
    localStorage.removeItem('sovrabase_admin_role');
    set({ isAuthenticated: false, role: null });
  },
  checkAuth: () => {
    const authed = hasToken();
    set({ isAuthenticated: authed, role: getRole() });
    return authed;
  },
}));

// ===== Dashboard =====
interface DashboardState {
  stats: DashboardStats | null;
  usage: UsageStats | null;
  replication: ReplicationInfo | null;
  loading: boolean;
  error: string | null;
  loadDashboard: () => Promise<void>;
}

export const useDashboard = create<DashboardState>((set) => ({
  stats: null,
  usage: null,
  replication: null,
  loading: false,
  error: null,
  loadDashboard: async () => {
    set({ loading: true, error: null });
    try {
      const [stats, usage] = await Promise.all([
        api<DashboardStats>('/admin/stats'),
        api<UsageStats>('/admin/stats/usage').catch(() => null),
      ]);
      let replication: ReplicationInfo | null = null;
      try {
        const health = await api<{ replication: ReplicationInfo }>('/health');
        replication = health.replication;
      } catch {
        // health endpoint may not have replication info
      }
      set({ stats, usage, replication, loading: false });
    } catch (err) {
      set({ error: (err as Error).message, loading: false });
    }
  },
}));

// ===== Projects =====
interface ProjectsState {
  projects: Project[];
  loading: boolean;
  error: string | null;
  loadProjects: () => Promise<void>;
  createProject: (name: string) => Promise<Project>;
  deleteProject: (id: string) => Promise<void>;
}

export const useProjects = create<ProjectsState>((set, get) => ({
  projects: [],
  loading: false,
  error: null,
  loadProjects: async () => {
    set({ loading: true, error: null });
    try {
      const data = await api<{ projects: Project[] }>('/admin/projects');
      set({ projects: data.projects || [], loading: false });
    } catch (err) {
      set({ error: (err as Error).message, loading: false });
    }
  },
  createProject: async (name: string) => {
    const data = await api<{ project: Project }>('/admin/projects', {
      method: 'POST',
      body: JSON.stringify({ name }),
    });
    const project = data.project;
    set({ projects: [...get().projects, project] });
    return project;
  },
  deleteProject: async (id: string) => {
    await api(`/admin/projects/${encodeURIComponent(id)}`, { method: 'DELETE' });
    set({ projects: get().projects.filter((p) => p.id !== id) });
  },
}));

// ===== Plugins =====
interface PluginsState {
  plugins: PluginInfo | null;
  loading: boolean;
  loadPlugins: () => Promise<void>;
}

export const usePlugins = create<PluginsState>((set) => ({
  plugins: null,
  loading: false,
  loadPlugins: async () => {
    set({ loading: true });
    try {
      const data = await api<PluginInfo>('/admin/plugins');
      set({ plugins: data, loading: false });
    } catch {
      set({ loading: false });
    }
  },
}));
