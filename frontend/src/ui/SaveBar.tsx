import { useEffect, useMemo, useState } from 'react';
import { Button } from './Button';

/**
 * Track a form's saved value against its edited value.
 * `dirty` is a deep comparison, so the save bar only shows when something really changed.
 */
export function useDraft<T>(saved: T | null | undefined) {
  const [draft, setDraft] = useState<T | null>(saved ?? null);
  useEffect(() => {
    setDraft(saved ?? null);
  }, [saved]);
  const dirty = useMemo(() => JSON.stringify(draft) !== JSON.stringify(saved ?? null), [draft, saved]);
  const patch = (p: Partial<T>) => setDraft((d) => (d ? { ...d, ...p } : d));
  const reset = () => setDraft(saved ?? null);
  return { draft, setDraft, patch, dirty, reset };
}

/** Sticky bar at the bottom of a settings page. Appears only when there are unsaved changes. */
export function SaveBar({
  dirty,
  saving,
  onSave,
  onDiscard,
  message = 'You have unsaved changes',
}: {
  dirty: boolean;
  saving?: boolean;
  onSave: () => void;
  onDiscard: () => void;
  message?: string;
}) {
  useEffect(() => {
    if (!dirty) return;
    const h = (e: BeforeUnloadEvent) => e.preventDefault();
    window.addEventListener('beforeunload', h);
    return () => window.removeEventListener('beforeunload', h);
  }, [dirty]);
  if (!dirty) return null;
  return (
    <div className="sticky bottom-[72px] md:bottom-4 z-20 mt-6 animate-fade">
      <div className="flex flex-wrap items-center gap-3 px-4 py-2.5 rounded-[12px] bg-ink text-bg shadow-pop">
        <span className="flex-1 min-w-[160px] text-sm font-medium">{message}</span>
        <Button variant="ghost" size="sm" onClick={onDiscard} className="!text-bg hover:!bg-white/10">
          Discard
        </Button>
        <Button variant="primary" size="sm" loading={saving} onClick={onSave}>
          Save changes
        </Button>
      </div>
    </div>
  );
}
