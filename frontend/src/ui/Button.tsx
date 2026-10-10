import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from 'react';
import { Link, type LinkProps } from 'react-router-dom';
import { Loader2 } from 'lucide-react';
import { cx } from './cx';

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger';
export type ButtonSize = 'sm' | 'md';

const base =
  'inline-flex items-center justify-center gap-2 whitespace-nowrap font-medium border transition-colors disabled:opacity-50 disabled:pointer-events-none select-none';
const sizes: Record<ButtonSize, string> = {
  sm: 'h-[30px] px-2.5 text-[13px] rounded-[7px]',
  md: 'h-9 px-3.5 text-sm rounded-md',
};
const variants: Record<ButtonVariant, string> = {
  primary: 'bg-accent border-accent text-on-accent hover:bg-accent-2 hover:border-accent-2',
  secondary: 'bg-surface border-line-2 text-ink hover:bg-sunken',
  ghost: 'bg-transparent border-transparent text-ink-2 hover:bg-sunken hover:text-ink',
  danger: 'bg-danger border-danger text-white hover:bg-danger-2 hover:border-danger-2',
};

export function buttonClass(variant: ButtonVariant = 'secondary', size: ButtonSize = 'md', extra?: string) {
  return cx(base, sizes[size], variants[variant], extra);
}

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  loading?: boolean;
  icon?: ReactNode;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant = 'secondary', size = 'md', loading, icon, className, children, disabled, type = 'button', ...rest },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      className={buttonClass(variant, size, className)}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      {...rest}
    >
      {loading ? <Loader2 className="w-4 h-4 animate-spin" aria-hidden /> : icon}
      {children}
    </button>
  );
});

export function ButtonLink({
  variant = 'secondary',
  size = 'md',
  icon,
  className,
  children,
  ...rest
}: LinkProps & { variant?: ButtonVariant; size?: ButtonSize; icon?: ReactNode }) {
  return (
    <Link className={buttonClass(variant, size, className)} {...rest}>
      {icon}
      {children}
    </Link>
  );
}

/** Square icon-only button. Always pass a `label` for screen readers. */
export function IconButton({
  label,
  className,
  children,
  size = 'md',
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & { label: string; size?: ButtonSize }) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      className={cx(
        'inline-flex items-center justify-center rounded-md text-ink-3 hover:text-ink hover:bg-sunken transition-colors disabled:opacity-50',
        size === 'sm' ? 'w-[30px] h-[30px]' : 'w-9 h-9',
        className,
      )}
      {...rest}
    >
      {children}
    </button>
  );
}
