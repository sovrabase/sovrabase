import { useEffect, useState, useCallback } from 'react';
import {
  Plus, Trash2, Copy, Loader2, AlertTriangle,
  Users, FolderKanban, Eye, EyeOff,
} from 'lucide-react';
import { useProjects } from '../store';
import { formatDate } from '../api';
import { useToast } from '../components/Toast';
import Modal from '../components/Modal';
import { useNavigate } from 'react-router-dom';
import type { Project } from '../types';

export default function Projects() {
  const { projects, loading, error, loadProjects, createProject, deleteProject } = useProjects();
  const { showToast } = useToast();
  const navigate = useNavigate();

  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<Project | null>(null);
  const [revealedKeys, setRevealedKeys] = useState<Set<string>>(new Set());
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    loadProjects();
  }, [loadProjects]);

  const handleCreate = useCallback(async () => {
    if (!newName.trim()) return;
    setCreating(true);
    try {
      await createProject(newName.trim());
      setNewName('');
      setShowCreate(false);
      showToast('Project created', 'success');
    } catch (err) {
      showToast((err as Error).message, 'error');
    } finally {
      setCreating(false);
    }
  }, [newName, createProject, showToast]);

  const handleDelete = useCallback(async () => {
    if (!deleteTarget) return;
    try {
      await deleteProject(deleteTarget.id);
      showToast('Project deleted', 'success');
    } catch (err) {
      showToast((err as Error).message, 'error');
    } finally {
      setDeleteTarget(null);
    }
  }, [deleteTarget, deleteProject, showToast]);

  const toggleKey = (e: React.MouseEvent, id: string) => {
    e.stopPropagation();
    setRevealedKeys((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const copyKey = (e: React.MouseEvent, key: string) => {
    e.stopPropagation();
    navigator.clipboard.writeText(key).then(() => showToast('API key copied', 'success'));
  };

  const maskKey = (key: string) => key.slice(0, 4) + '\u25CF'.repeat(12) + key.slice(-4);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-24">
        <Loader2 className="w-8 h-8 text-accent animate-spin" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center py-24 gap-3 text-text-muted">
        <AlertTriangle className="w-10 h-10 text-danger" />
        <p className="text-text-secondary">{error}</p>
        <button onClick={loadProjects} className="mt-2 px-4 py-2 rounded-lg bg-accent text-white text-sm hover:bg-accent-hover transition-colors">
          Retry
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-text-primary">Projects</h1>
        <button
          onClick={() => setShowCreate(true)}
          className="flex items-center gap-2 px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover transition-colors"
        >
          <Plus className="w-4 h-4" />
          New Project
        </button>
      </div>

      {projects.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-text-muted gap-4">
          <FolderKanban className="w-12 h-12 text-text-muted/40" />
          <p className="text-text-secondary text-lg">No projects yet. Create your first project.</p>
          <button
            onClick={() => setShowCreate(true)}
            className="flex items-center gap-2 px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover transition-colors"
          >
            <Plus className="w-4 h-4" />
            Create Project
          </button>
        </div>
      ) : (
        <div className="bg-bg-card border border-border rounded-xl overflow-hidden">
          <table className="w-full">
            <thead>
              <tr className="border-b border-border">
                <th className="text-left text-text-muted text-xs font-medium uppercase tracking-wider px-6 py-3">Name</th>
                <th className="text-left text-text-muted text-xs font-medium uppercase tracking-wider px-6 py-3">API Key</th>
                <th className="text-left text-text-muted text-xs font-medium uppercase tracking-wider px-6 py-3">Collections</th>
                <th className="text-left text-text-muted text-xs font-medium uppercase tracking-wider px-6 py-3">Members</th>
                <th className="text-left text-text-muted text-xs font-medium uppercase tracking-wider px-6 py-3">Created</th>
                <th className="text-right text-text-muted text-xs font-medium uppercase tracking-wider px-6 py-3">Actions</th>
              </tr>
            </thead>
            <tbody>
              {projects.map((p) => {
                const isRevealed = revealedKeys.has(p.id);
                const apiKey = p.api_key || '';
                return (
                  <tr
                    key={p.id}
                    onClick={() => navigate(`/projects/${p.id}`)}
                    className="border-b border-border/50 hover:bg-bg-input/30 transition-colors cursor-pointer"
                  >
                    <td className="px-6 py-4 font-medium text-text-primary">{p.name}</td>
                    <td className="px-6 py-4">
                      {apiKey ? (
                        <div className="flex items-center gap-2">
                          <code className="text-text-secondary text-sm font-mono">
                            {isRevealed ? apiKey : maskKey(apiKey)}
                          </code>
                          <button
                            onClick={(e) => toggleKey(e, p.id)}
                            className="p-1 rounded text-text-muted hover:text-text-primary transition-colors"
                            title={isRevealed ? 'Hide' : 'Reveal'}
                          >
                            {isRevealed ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
                          </button>
                          {isRevealed && (
                            <button
                              onClick={(e) => copyKey(e, apiKey)}
                              className="p-1 rounded text-text-muted hover:text-accent transition-colors"
                              title="Copy"
                            >
                              <Copy className="w-3.5 h-3.5" />
                            </button>
                          )}
                        </div>
                      ) : (
                        <span className="text-text-muted text-sm">&mdash;</span>
                      )}
                    </td>
                    <td className="px-6 py-4">
                      <span className="inline-flex items-center gap-1 text-text-secondary text-sm">
                        <FolderKanban className="w-3.5 h-3.5" />
                        {p.collection_count ?? 0}
                      </span>
                    </td>
                    <td className="px-6 py-4">
                      <span className="inline-flex items-center gap-1 text-text-secondary text-sm">
                        <Users className="w-3.5 h-3.5" />
                        {p.member_count ?? 0}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-text-secondary text-sm">{formatDate(p.created_at)}</td>
                    <td className="px-6 py-4 text-right">
                      <button
                        onClick={(e) => { e.stopPropagation(); setDeleteTarget(p); }}
                        className="p-2 rounded-lg text-text-muted hover:text-danger hover:bg-danger/10 transition-colors"
                        title="Delete project"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <Modal isOpen={showCreate} onClose={() => setShowCreate(false)} title="New Project">
        <div className="space-y-4">
          <div>
            <label className="block text-text-secondary text-sm font-medium mb-1.5">Project Name</label>
            <input
              type="text"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
              className="w-full px-3 py-2 rounded-lg bg-bg-input border border-border text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent transition-colors"
              placeholder="my-project"
              autoFocus
            />
          </div>
          <div className="flex justify-end gap-3">
            <button
              onClick={() => setShowCreate(false)}
              className="px-4 py-2 rounded-lg text-text-secondary text-sm hover:bg-bg-input transition-colors"
            >
              Cancel
            </button>
            <button
              onClick={handleCreate}
              disabled={!newName.trim() || creating}
              className="px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover disabled:opacity-50 transition-colors"
            >
              {creating ? 'Creating...' : 'Create'}
            </button>
          </div>
        </div>
      </Modal>

      <Modal isOpen={!!deleteTarget} onClose={() => setDeleteTarget(null)} title="Delete Project" size="sm">
        <div className="space-y-4">
          <p className="text-text-secondary text-sm">
            Are you sure you want to delete <strong className="text-text-primary">{deleteTarget?.name}</strong>?
            This action cannot be undone.
          </p>
          <div className="flex justify-end gap-3">
            <button
              onClick={() => setDeleteTarget(null)}
              className="px-4 py-2 rounded-lg text-text-secondary text-sm hover:bg-bg-input transition-colors"
            >
              Cancel
            </button>
            <button
              onClick={handleDelete}
              className="px-4 py-2 rounded-lg bg-danger text-white text-sm font-medium hover:bg-danger-hover transition-colors"
            >
              Delete
            </button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
