import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react';
import { Outlet, useParams } from 'react-router-dom';
import { AlertTriangle } from 'lucide-react';
import { api, adminProjectPath } from '../api';
import type { Project } from '../types';
import { ButtonLink, EmptyState, Spinner } from '../ui';

interface ProjectCtx {
  project: Project;
  projectId: string;
  reload: () => Promise<void>;
}

const Ctx = createContext<ProjectCtx | null>(null);

/** The current project. Only valid inside /projects/:id routes. */
export function useProject(): ProjectCtx {
  const c = useContext(Ctx);
  if (!c) throw new Error('useProject must be used inside a project route');
  return c;
}

/** Same as useProject but returns null outside a project (used by the shell). */
export function useMaybeProject(): ProjectCtx | null {
  return useContext(Ctx);
}

export function ProjectProvider({ children }: { children?: ReactNode }) {
  const { id = '' } = useParams<{ id: string }>();
  const [project, setProject] = useState<Project | null>(null);
  const [error, setError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    try {
      // The endpoint wraps the project: { project, api_key, api_url, ... }.
      const d = await api<{ project?: Project; api_key?: string } & Partial<Project>>(adminProjectPath(id, ''));
      const p = (d.project ?? d) as Project;
      setProject({ ...p, api_key: d.api_key ?? p.api_key });
      setError(null);
    } catch (e) {
      setError((e as Error).message);
    }
  }, [id]);

  useEffect(() => {
    setProject(null);
    reload();
  }, [reload]);

  if (error) {
    return (
      <EmptyState
        icon={<AlertTriangle className="w-5 h-5" />}
        title="This project could not be opened"
        description={error}
        action={<ButtonLink to="/projects">All projects</ButtonLink>}
        className="py-24"
      />
    );
  }
  if (!project) return <Spinner label="Opening project" />;
  return <Ctx.Provider value={{ project, projectId: id, reload }}>{children ?? <Outlet />}</Ctx.Provider>;
}
