import {
  createContext,
  forwardRef,
  useContext,
  useId,
  useState,
  type InputHTMLAttributes,
  type ReactNode,
  type SelectHTMLAttributes,
  type TextareaHTMLAttributes,
} from 'react';
import { Lock } from 'lucide-react';
import { cx } from './cx';

interface FieldCtx {
  id: string;
  describedBy?: string;
  invalid: boolean;
  locked: boolean;
}

const FieldContext = createContext<FieldCtx | null>(null);

/** Props every control picks up from the surrounding <Field>. */
function useFieldProps(id?: string) {
  const ctx = useContext(FieldContext);
  return {
    id: id ?? ctx?.id,
    'aria-describedby': ctx?.describedBy,
    'aria-invalid': ctx?.invalid || undefined,
    disabled: ctx?.locked || undefined,
  };
}

export interface FieldProps {
  label: ReactNode;
  hint?: ReactNode;
  error?: ReactNode;
  optional?: boolean;
  /** Name of the environment variable that sets this value. Locks the control. */
  env?: string;
  /** Extra content on the right of the label row, e.g. a link. */
  aside?: ReactNode;
  className?: string;
  children: ReactNode;
}

/**
 * The one wrapper for every form control: label, hint, error and the ENV lock.
 * Controls inside pick up id / aria-describedby / aria-invalid automatically.
 */
export function Field({ label, hint, error, optional, env, aside, className, children }: FieldProps) {
  const id = useId();
  const hintId = hint || env ? `${id}-hint` : undefined;
  const errId = error ? `${id}-err` : undefined;
  const describedBy = [errId, hintId].filter(Boolean).join(' ') || undefined;
  return (
    <FieldContext.Provider value={{ id, describedBy, invalid: !!error, locked: !!env }}>
      <div className={cx('flex flex-col gap-1.5 min-w-0', className)}>
        <div className="flex items-center gap-2 min-h-5">
          <label htmlFor={id} className="text-[13px] font-semibold text-ink">
            {label}
          </label>
          {optional && <span className="text-[12.5px] text-ink-3">optional</span>}
          {env && (
            <span className="inline-flex items-center gap-1 h-5 px-1.5 rounded-[5px] bg-warn-soft text-warn text-[11px] font-semibold font-mono">
              <Lock className="w-3 h-3" aria-hidden />
              ENV
            </span>
          )}
          {aside && <span className="ml-auto">{aside}</span>}
        </div>
        {children}
        {error && (
          <p id={errId} className="text-[12.5px] leading-[18px] text-danger" role="alert">
            {error}
          </p>
        )}
        {(hint || env) && (
          <p id={hintId} className="text-[12.5px] leading-[18px] text-ink-3">
            {env ? (
              <>
                Set by <code className="font-mono">{env}</code>. Change it in the server environment.
                {hint ? <> {hint}</> : null}
              </>
            ) : (
              hint
            )}
          </p>
        )}
      </div>
    </FieldContext.Provider>
  );
}

export const controlClass =
  'w-full h-9 px-3 rounded-md border border-line-2 bg-surface text-ink text-sm placeholder:text-ink-3 ' +
  'hover:border-ink-3 focus:outline-none focus:border-accent focus:ring-3 focus:ring-accent/20 ' +
  'aria-[invalid=true]:border-danger aria-[invalid=true]:focus:ring-danger/20 ' +
  'disabled:bg-sunken disabled:text-ink-3 disabled:cursor-not-allowed transition-[border-color,box-shadow]';

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  mono?: boolean;
  /** Fixed text inside the box before the value, e.g. "https://". */
  prefix?: string;
  suffix?: ReactNode;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  { className, mono, prefix, suffix, id, disabled, ...rest },
  ref,
) {
  const f = useFieldProps(id);
  const input = (
    <input
      ref={ref}
      {...f}
      disabled={disabled || f.disabled}
      className={cx(
        controlClass,
        mono && 'font-mono text-[13px]',
        !!(prefix || suffix) && 'border-0 h-[34px] focus:ring-0 bg-transparent flex-1 min-w-0',
        !(prefix || suffix) && className,
      )}
      {...rest}
    />
  );
  if (!prefix && !suffix) return input;
  return (
    <div
      className={cx(
        'flex items-center w-full rounded-md border border-line-2 bg-surface focus-within:border-accent focus-within:ring-3 focus-within:ring-accent/20',
        className,
      )}
    >
      {prefix && <span className="pl-3 text-ink-3 text-sm whitespace-nowrap">{prefix}</span>}
      {input}
      {suffix && <span className="pr-1.5 flex items-center">{suffix}</span>}
    </div>
  );
});

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaHTMLAttributes<HTMLTextAreaElement> & { mono?: boolean }>(
  function Textarea({ className, mono, id, disabled, rows = 4, ...rest }, ref) {
    const f = useFieldProps(id);
    return (
      <textarea
        ref={ref}
        rows={rows}
        {...f}
        disabled={disabled || f.disabled}
        className={cx(controlClass, 'h-auto py-2 leading-5 resize-y', mono && 'font-mono text-[13px]', className)}
        {...rest}
      />
    );
  },
);

export interface SelectOption {
  value: string;
  label: string;
  disabled?: boolean;
}

export const Select = forwardRef<
  HTMLSelectElement,
  SelectHTMLAttributes<HTMLSelectElement> & { options?: SelectOption[]; placeholder?: string }
>(function Select({ className, options, placeholder, children, id, disabled, ...rest }, ref) {
  const f = useFieldProps(id);
  return (
    <select
      ref={ref}
      {...f}
      disabled={disabled || f.disabled}
      className={cx(
        controlClass,
        'appearance-none pr-9 bg-no-repeat bg-[length:16px] bg-[position:right_10px_center]',
        "bg-[url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='%235E6573' stroke-width='2'%3E%3Cpath d='m7 10 5 5 5-5'/%3E%3C/svg%3E\")]",
        className,
      )}
      {...rest}
    >
      {placeholder !== undefined && (
        <option value="" disabled>
          {placeholder}
        </option>
      )}
      {options?.map((o) => (
        <option key={o.value} value={o.value} disabled={o.disabled}>
          {o.label}
        </option>
      ))}
      {children}
    </select>
  );
});

export interface SwitchProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  label?: ReactNode;
  description?: ReactNode;
  disabled?: boolean;
  id?: string;
  'aria-label'?: string;
}

/** On/off switch. With `label` it renders as a full row; without it, pass aria-label. */
export function Switch({ checked, onChange, label, description, disabled, id, ...aria }: SwitchProps) {
  const f = useFieldProps(id);
  const autoId = useId();
  const sw = (
    <button
      type="button"
      role="switch"
      id={f.id ?? autoId}
      aria-checked={checked}
      aria-label={aria['aria-label']}
      aria-describedby={f['aria-describedby']}
      disabled={disabled || f.disabled}
      onClick={() => onChange(!checked)}
      className={cx(
        'relative inline-flex shrink-0 w-[34px] h-5 rounded-full transition-colors disabled:opacity-50',
        checked ? 'bg-accent' : 'bg-line-2',
      )}
    >
      <span
        className={cx(
          'absolute top-0.5 w-4 h-4 rounded-full bg-white shadow-[0_1px_2px_rgba(0,0,0,.25)] transition-[left]',
          checked ? 'left-4' : 'left-0.5',
        )}
      />
    </button>
  );
  if (!label) return sw;
  return (
    <label className="flex items-start gap-3 cursor-pointer" htmlFor={f.id ?? autoId}>
      <span className="pt-0.5">{sw}</span>
      <span className="flex flex-col">
        <span className="text-sm font-medium text-ink">{label}</span>
        {description && <span className="text-[12.5px] leading-[18px] text-ink-3">{description}</span>}
      </span>
    </label>
  );
}

export function Checkbox({
  checked,
  onChange,
  label,
  description,
  disabled,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
  label: ReactNode;
  description?: ReactNode;
  disabled?: boolean;
}) {
  return (
    <label className={cx('flex items-start gap-2.5 cursor-pointer', disabled && 'opacity-50 cursor-not-allowed')}>
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(e) => onChange(e.target.checked)}
        className="mt-0.5 w-4 h-4 rounded-[4px] accent-[var(--sv-accent)]"
      />
      <span className="flex flex-col">
        <span className="text-sm text-ink">{label}</span>
        {description && <span className="text-[12.5px] leading-[18px] text-ink-3">{description}</span>}
      </span>
    </label>
  );
}

export interface SecretInputProps {
  /** True when the server already stores a value. The value itself is never shown. */
  isSet: boolean;
  /** Last characters the server reports, if any, e.g. "4f2a". */
  hint?: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  id?: string;
}

/**
 * Write-only secret. When a value is stored it shows "Set" and a Replace button;
 * the stored value never comes back to the browser.
 */
export function SecretInput({ isSet, hint, value, onChange, placeholder, id }: SecretInputProps) {
  const [editing, setEditing] = useState(!isSet);
  const f = useFieldProps(id);
  if (isSet && !editing) {
    return (
      <div className="flex items-center gap-2 h-9 px-3 rounded-md border border-line bg-sunken">
        <Lock className="w-3.5 h-3.5 text-ink-3" aria-hidden />
        <span className="text-sm text-ink-2 flex-1 truncate">
          Set{hint ? <span className="font-mono text-[12.5px] text-ink-3"> · ends {hint}</span> : null}
        </span>
        <button
          type="button"
          className="text-[13px] font-medium text-accent hover:text-accent-2 disabled:opacity-50"
          onClick={() => setEditing(true)}
          disabled={f.disabled}
          aria-describedby={f['aria-describedby']}
        >
          Replace
        </button>
      </div>
    );
  }
  return (
    <div className="flex items-center gap-2">
      <Input
        id={id}
        type="password"
        mono
        autoComplete="new-password"
        value={value}
        placeholder={placeholder ?? (isSet ? 'New value' : '')}
        onChange={(e) => onChange(e.target.value)}
      />
      {isSet && (
        <button
          type="button"
          className="text-[13px] font-medium text-ink-3 hover:text-ink whitespace-nowrap"
          onClick={() => {
            onChange('');
            setEditing(false);
          }}
        >
          Keep current
        </button>
      )}
    </div>
  );
}
