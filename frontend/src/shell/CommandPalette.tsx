import { useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { useNavigate } from 'react-router-dom';
import { CornerDownLeft, FolderKanban, LogOut, Moon, Plus, Search, Sun, type LucideIcon } from 'lucide-react';
import { useAuth, useProjects } from '../store';
import { cx, Kbd } from '../ui';
import { instanceNav, projectNav, projectPath } from './nav';
import { getTheme, setTheme } from './theme';

interface Cmd {
  id: string;
  label: string;
  group: string;
  icon: LucideIcon;
  keywords?: string;
  run: () => void;
}

export function CommandPalette({ open, onClose, projectId }: { open: boolean; onClose: () => void; projectId: string | null }) {
  const navigate = useNavigate();
  const { role, logout } = useAuth();
  const { projects, loadProjects } = useProjects();
  const [q, setQ] = useState('');
  const [active, setActive] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    setQ('');
    setActive(0);
    if (!projects.length) loadProjects();
    setTimeout(() => inputRef.current?.focus(), 10);
  }, [open, projects.length, loadProjects]);

  const cmds = useMemo<Cmd[]>(() => {
    const go = (to: string) => () => {
      navigate(to);
      onClose();
    };
    const out: Cmd[] = [];
    if (projectId) {
      const name = projects.find((p) => p.id === projectId)?.name ?? 'This project';
      for (const g of projectNav(projectId))
        for (const it of g.items) out.push({ id: `p:${it.to}`, label: it.label, group: name, icon: it.icon, keywords: it.keywords, run: go(it.to) });
    }
    for (const p of projects)
      out.push({ id: `proj:${p.id}`, label: p.name, group: 'Projects', icon: FolderKanban, keywords: p.id, run: go(projectPath(p.id)) });
    for (const g of instanceNav(role))
      for (const it of g.items) out.push({ id: `i:${it.to}`, label: it.label, group: 'Instance', icon: it.icon, keywords: it.keywords, run: go(it.to) });
    const dark = getTheme() === 'dark' || (getTheme() === 'system' && window.matchMedia?.('(prefers-color-scheme: dark)').matches);
    out.push(
      { id: 'a:new', label: 'New project', group: 'Actions', icon: Plus, keywords: 'create', run: go('/projects?new=1') },
      {
        id: 'a:theme',
        label: dark ? 'Switch to light theme' : 'Switch to dark theme',
        group: 'Actions',
        icon: dark ? Sun : Moon,
        keywords: 'theme dark light mode',
        run: () => {
          setTheme(dark ? 'light' : 'dark');
          onClose();
        },
      },
      {
        id: 'a:out',
        label: 'Sign out',
        group: 'Actions',
        icon: LogOut,
        keywords: 'logout',
        run: () => {
          logout();
          navigate('/');
          onClose();
        },
      },
    );
    return out;
  }, [projectId, projects, role, navigate, onClose, logout]);

  const results = useMemo(() => {
    const t = q.trim().toLowerCase();
    if (!t) return cmds;
    return cmds.filter((c) => `${c.label} ${c.group} ${c.keywords ?? ''}`.toLowerCase().includes(t));
  }, [cmds, q]);

  useEffect(() => setActive(0), [q]);
  useEffect(() => {
    listRef.current?.querySelector<HTMLElement>(`[data-i="${active}"]`)?.scrollIntoView({ block: 'nearest' });
  }, [active]);

  if (!open) return null;

  let lastGroup = '';
  return createPortal(
    <div className="fixed inset-0 z-[60] flex items-start justify-center p-4 pt-[12vh]">
      <div className="absolute inset-0 bg-[rgba(13,16,23,.32)]" onClick={onClose} aria-hidden />
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Search or jump to"
        className="relative w-full max-w-[600px] bg-surface border border-line rounded-[14px] shadow-pop animate-pop overflow-hidden"
        onKeyDown={(e) => {
          if (e.key === 'Escape') onClose();
          else if (e.key === 'ArrowDown') {
            e.preventDefault();
            setActive((a) => Math.min(a + 1, results.length - 1));
          } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            setActive((a) => Math.max(a - 1, 0));
          } else if (e.key === 'Enter') {
            e.preventDefault();
            results[active]?.run();
          }
        }}
      >
        <div className="flex items-center gap-2.5 px-4 h-[52px] border-b border-line">
          <Search className="w-[18px] h-[18px] text-ink-3" aria-hidden />
          <input
            ref={inputRef}
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Search pages, projects and actions"
            aria-label="Search pages, projects and actions"
            role="combobox"
            aria-expanded="true"
            aria-controls="palette-list"
            aria-activedescendant={results[active] ? `cmd-${active}` : undefined}
            className="flex-1 bg-transparent outline-none text-[15px] text-ink placeholder:text-ink-3"
          />
          <Kbd>Esc</Kbd>
        </div>
        <div ref={listRef} id="palette-list" role="listbox" className="max-h-[360px] overflow-y-auto p-1.5">
          {results.length === 0 && <p className="m-0 px-3 py-8 text-center text-sm text-ink-3">Nothing matches “{q}”.</p>}
          {results.map((c, i) => {
            const head = c.group !== lastGroup ? c.group : null;
            lastGroup = c.group;
            return (
              <div key={c.id}>
                {head && <div className="px-2.5 pt-2.5 pb-1 text-[11.5px] font-semibold uppercase tracking-[.06em] text-ink-3">{head}</div>}
                <button
                  id={`cmd-${i}`}
                  data-i={i}
                  role="option"
                  aria-selected={i === active}
                  type="button"
                  onMouseMove={() => setActive(i)}
                  onClick={c.run}
                  className={cx('w-full flex items-center gap-2.5 h-9 px-2.5 rounded-[8px] text-sm text-left', i === active ? 'bg-accent-soft text-accent-2' : 'text-ink')}
                >
                  <c.icon className="w-4 h-4 shrink-0 opacity-80" aria-hidden />
                  <span className="flex-1 truncate">{c.label}</span>
                  {i === active && <CornerDownLeft className="w-3.5 h-3.5" aria-hidden />}
                </button>
              </div>
            );
          })}
        </div>
      </div>
    </div>,
    document.body,
  );
}
