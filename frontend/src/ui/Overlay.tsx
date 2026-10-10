import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { X } from 'lucide-react';
import { cx } from './cx';
import { Button } from './Button';
import { Field, Input } from './Field';

function useEscape(open: boolean, onClose: () => void) {
  useEffect(() => {
    if (!open) return;
    const h = (e: KeyboardEvent) => e.key === 'Escape' && onClose();
    document.addEventListener('keydown', h);
    return () => document.removeEventListener('keydown', h);
  }, [open, onClose]);
}

function useFocusFirst(open: boolean, ref: React.RefObject<HTMLElement | null>) {
  useEffect(() => {
    if (!open) return;
    const prev = document.activeElement as HTMLElement | null;
    const t = setTimeout(() => {
      const el = ref.current?.querySelector<HTMLElement>('[autofocus],input:not([type=hidden]),select,textarea,button:not([data-close])');
      (el ?? ref.current)?.focus();
    }, 10);
    return () => {
      clearTimeout(t);
      prev?.focus?.();
    };
  }, [open, ref]);
}

/**
 * Side sheet: the default place for create/edit forms. Keeps the list visible behind it.
 * Full screen on phones.
 */
export function Sheet({
  open,
  onClose,
  title,
  description,
  children,
  footer,
  width = 520,
}: {
  open: boolean;
  onClose: () => void;
  title: ReactNode;
  description?: ReactNode;
  children: ReactNode;
  footer?: ReactNode;
  width?: number;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const titleId = useId();
  useEscape(open, onClose);
  useFocusFirst(open, ref);
  if (!open) return null;
  return createPortal(
    <div className="fixed inset-0 z-50 flex justify-end">
      <div className="absolute inset-0 bg-[rgba(13,16,23,.32)]" onClick={onClose} aria-hidden />
      <div
        ref={ref}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        style={{ maxWidth: width }}
        className="relative w-full h-full bg-surface border-l border-line shadow-pop flex flex-col animate-sheet outline-none"
      >
        <div className="flex items-start gap-3 px-6 py-4 border-b border-line">
          <div className="flex-1 min-w-0">
            <h2 id={titleId} className="m-0 text-[17px] leading-6 font-semibold text-ink">
              {title}
            </h2>
            {description && <p className="m-0 mt-0.5 text-[13px] text-ink-3">{description}</p>}
          </div>
          <button
            type="button"
            data-close
            onClick={onClose}
            aria-label="Close"
            className="w-8 h-8 -mr-1 inline-flex items-center justify-center rounded-md text-ink-3 hover:text-ink hover:bg-sunken"
          >
            <X className="w-[18px] h-[18px]" />
          </button>
        </div>
        <div className="flex-1 overflow-y-auto px-6 py-5 flex flex-col gap-5">{children}</div>
        {footer && <div className="flex justify-end gap-2 px-6 py-3.5 border-t border-line bg-surface">{footer}</div>}
      </div>
    </div>,
    document.body,
  );
}

/** Centered dialog for short questions. */
export function Dialog({
  open,
  onClose,
  title,
  children,
  footer,
  size = 'sm',
}: {
  open: boolean;
  onClose: () => void;
  title: ReactNode;
  children?: ReactNode;
  footer?: ReactNode;
  size?: 'sm' | 'md' | 'lg';
}) {
  const ref = useRef<HTMLDivElement>(null);
  const titleId = useId();
  useEscape(open, onClose);
  useFocusFirst(open, ref);
  if (!open) return null;
  const w = { sm: 'max-w-[440px]', md: 'max-w-[560px]', lg: 'max-w-[760px]' }[size];
  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 bg-[rgba(13,16,23,.32)]" onClick={onClose} aria-hidden />
      <div
        ref={ref}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        className={cx('relative w-full bg-surface border border-line rounded-[14px] shadow-pop animate-pop outline-none max-h-[90vh] flex flex-col', w)}
      >
        <div className="px-6 pt-5 pb-1">
          <h2 id={titleId} className="m-0 text-[17px] leading-6 font-semibold text-ink">
            {title}
          </h2>
        </div>
        {children && <div className="px-6 py-3 text-sm text-ink-2 overflow-y-auto flex flex-col gap-4">{children}</div>}
        {footer && <div className="flex justify-end gap-2 px-6 pb-5 pt-3">{footer}</div>}
      </div>
    </div>,
    document.body,
  );
}

/**
 * Confirmation for destructive actions. With `confirmText`, the person must type it
 * (e.g. the project name) before the button unlocks. Replaces window.confirm().
 */
export function ConfirmDialog({
  open,
  onClose,
  onConfirm,
  title,
  children,
  confirmLabel = 'Delete',
  confirmText,
  tone = 'danger',
}: {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void | Promise<void>;
  title: ReactNode;
  children?: ReactNode;
  confirmLabel?: string;
  confirmText?: string;
  tone?: 'danger' | 'primary';
}) {
  const [typed, setTyped] = useState('');
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    if (open) setTyped('');
  }, [open]);
  const locked = !!confirmText && typed !== confirmText;
  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={title}
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button
            variant={tone === 'danger' ? 'danger' : 'primary'}
            disabled={locked}
            loading={busy}
            onClick={async () => {
              setBusy(true);
              try {
                await onConfirm();
                onClose();
              } finally {
                setBusy(false);
              }
            }}
          >
            {confirmLabel}
          </Button>
        </>
      }
    >
      {children}
      {confirmText && (
        <Field label={<>Type <code className="font-mono font-medium">{confirmText}</code> to confirm</>}>
          <Input mono value={typed} onChange={(e) => setTyped(e.target.value)} autoComplete="off" autoFocus />
        </Field>
      )}
    </Dialog>
  );
}
