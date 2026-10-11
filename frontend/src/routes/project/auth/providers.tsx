import type { OAuthProvider } from '../../../types';
import { cx } from '../../../ui';

/** Pre-filled endpoints, scopes and field mappings for well-known OAuth providers. */
export interface ProviderTemplate {
  label: string;
  /** Where to create the OAuth app, shown in the setup sheet. */
  console?: string;
  fields: Partial<OAuthProvider>;
}

export const PROVIDER_TEMPLATES: Record<string, ProviderTemplate> = {
  google: {
    label: 'Google',
    console: 'Google Cloud console',
    fields: {
      auth_url: 'https://accounts.google.com/o/oauth2/v2/auth',
      token_url: 'https://oauth2.googleapis.com/token',
      userinfo_url: 'https://www.googleapis.com/oauth2/v3/userinfo',
      scopes: ['openid', 'email', 'profile'],
      email_field: 'email',
      name_field: 'name',
      avatar_field: 'picture',
      id_field: 'sub',
    },
  },
  github: {
    label: 'GitHub',
    console: 'GitHub developer settings',
    fields: {
      auth_url: 'https://github.com/login/oauth/authorize',
      token_url: 'https://github.com/login/oauth/access_token',
      userinfo_url: 'https://api.github.com/user',
      scopes: ['user:email'],
      email_field: 'email',
      name_field: 'name',
      avatar_field: 'avatar_url',
      id_field: 'id',
    },
  },
  discord: {
    label: 'Discord',
    console: 'Discord developer portal',
    fields: {
      auth_url: 'https://discord.com/api/oauth2/authorize',
      token_url: 'https://discord.com/api/oauth2/token',
      userinfo_url: 'https://discord.com/api/users/@me',
      scopes: ['identify', 'email'],
      email_field: 'email',
      name_field: 'username',
      // Discord returns only an avatar hash here, not a full URL.
      avatar_field: 'avatar',
      id_field: 'id',
    },
  },
  gitlab: {
    label: 'GitLab',
    console: 'GitLab applications settings',
    fields: {
      auth_url: 'https://gitlab.com/oauth/authorize',
      token_url: 'https://gitlab.com/oauth/token',
      userinfo_url: 'https://gitlab.com/api/v4/user',
      scopes: ['read_user'],
      email_field: 'email',
      name_field: 'name',
      avatar_field: 'avatar_url',
      id_field: 'id',
    },
  },
  microsoft: {
    label: 'Microsoft',
    console: 'Azure app registrations',
    fields: {
      auth_url: 'https://login.microsoftonline.com/common/oauth2/v2.0/authorize',
      token_url: 'https://login.microsoftonline.com/common/oauth2/v2.0/token',
      userinfo_url: 'https://graph.microsoft.com/oidc/userinfo',
      scopes: ['openid', 'email', 'profile'],
      email_field: 'email',
      name_field: 'name',
      avatar_field: 'picture',
      id_field: 'sub',
    },
  },
  facebook: {
    label: 'Facebook',
    console: 'Meta for Developers',
    fields: {
      auth_url: 'https://www.facebook.com/v18.0/dialog/oauth',
      token_url: 'https://graph.facebook.com/v18.0/oauth/access_token',
      userinfo_url: 'https://graph.facebook.com/v19.0/me?fields=id,name,email,picture',
      scopes: ['email'],
      email_field: 'email',
      name_field: 'name',
      avatar_field: 'picture',
      id_field: 'id',
    },
  },
  x: {
    label: 'X',
    console: 'X developer portal',
    fields: {
      auth_url: 'https://twitter.com/i/oauth2/2',
      token_url: 'https://api.twitter.com/2/oauth2/token',
      userinfo_url: 'https://api.twitter.com/2/users/me',
      scopes: ['users.read', 'tweet.read'],
      email_field: 'email',
      name_field: 'name',
      avatar_field: 'profile_image_url',
      id_field: 'id',
    },
  },
};

export const TEMPLATE_KEYS = Object.keys(PROVIDER_TEMPLATES);

/** Which template a stored provider was made from, or 'custom'. */
export function detectTemplate(p: OAuthProvider): string {
  for (const key of TEMPLATE_KEYS) {
    const f = PROVIDER_TEMPLATES[key].fields;
    if (f.auth_url && p.auth_url && p.auth_url === f.auth_url) return key;
  }
  if (p.name && PROVIDER_TEMPLATES[p.name]) return p.name;
  return 'custom';
}

export function providerLabel(name: string): string {
  return PROVIDER_TEMPLATES[name]?.label ?? name;
}

/** Public API origin (the admin console runs next to it; dev servers proxy to :6070). */
export function apiOrigin(): string {
  if (typeof window === 'undefined') return '';
  return window.location.origin.replace(/:\d+$/, ':6070');
}

export function callbackUrl(name: string): string {
  return `${apiOrigin()}/auth/v1/oauth/${encodeURIComponent(name || 'provider')}/callback`;
}

/** Monochrome provider mark, drawn in the current text colour. */
export function ProviderIcon({ name, className = 'w-4 h-4' }: { name: string; className?: string }) {
  const n = name.toLowerCase();
  if (n === 'google')
    return (
      <svg className={className} viewBox="0 0 24 24" fill="currentColor" aria-hidden>
        <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" />
        <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84A11 11 0 0 0 12 23z" opacity=".75" />
        <path d="M5.84 14.09A6.6 6.6 0 0 1 5.49 12c0-.73.13-1.43.35-2.09V7.07H2.18A11 11 0 0 0 1 12c0 1.78.43 3.45 1.18 4.93l3.66-2.84z" opacity=".55" />
        <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15A10.6 10.6 0 0 0 12 1 11 11 0 0 0 2.18 7.07l3.66 2.84C6.71 7.31 9.14 5.38 12 5.38z" opacity=".85" />
      </svg>
    );
  if (n === 'github')
    return (
      <svg className={className} viewBox="0 0 24 24" fill="currentColor" aria-hidden>
        <path d="M12 .3a12 12 0 0 0-3.8 23.38c.6.12.82-.26.82-.57v-2.04c-3.34.72-4.04-1.61-4.04-1.61-.55-1.39-1.34-1.76-1.34-1.76-1.08-.74.09-.73.09-.73 1.2.09 1.84 1.24 1.84 1.24 1.07 1.83 2.8 1.3 3.49 1 .1-.78.42-1.31.76-1.6-2.67-.31-5.47-1.34-5.47-5.93 0-1.31.47-2.38 1.24-3.22-.14-.3-.54-1.52.1-3.18 0 0 1-.32 3.3 1.23a11.5 11.5 0 0 1 6 0c2.28-1.55 3.29-1.23 3.29-1.23.64 1.66.24 2.88.12 3.18a4.65 4.65 0 0 1 1.23 3.22c0 4.61-2.8 5.62-5.48 5.92.42.36.81 1.1.81 2.22v3.29c0 .32.21.69.82.57A12 12 0 0 0 12 .3" />
      </svg>
    );
  if (n === 'discord')
    return (
      <svg className={className} viewBox="0 0 127.14 96.36" fill="currentColor" aria-hidden>
        <path d="M107.7 8.07A105.15 105.15 0 0 0 77.26 0a77.19 77.19 0 0 0-3.3 6.83 96.67 96.67 0 0 0-20.74 0A77.19 77.19 0 0 0 49.88 0a105.15 105.15 0 0 0-30.44 8.07C3.66 31.58-1.86 54.65 1 77.53a105.73 105.73 0 0 0 31 18.83 77.7 77.7 0 0 0 6.63-10.85 68.43 68.43 0 0 1-10.43-5c.87-.64 1.71-1.32 2.51-2a75.76 75.76 0 0 0 72.77 0c.8.69 1.64 1.37 2.51 2a68.43 68.43 0 0 1-10.43 5 77.7 77.7 0 0 0 6.63 10.85 105.73 105.73 0 0 0 31.06-18.83c2.64-26.6-4.51-49.47-18.55-69.46ZM42.45 65.69C36.18 65.69 31 60 31 53s5.18-12.64 11.45-12.64S53.83 46 53.83 53s-5.11 12.69-11.38 12.69Zm42.24 0C78.41 65.69 73.24 60 73.24 53s5.17-12.64 11.45-12.64S96.07 46 96.07 53s-5.07 12.69-11.38 12.69Z" />
      </svg>
    );
  if (n === 'microsoft')
    return (
      <svg className={className} viewBox="0 0 23 23" fill="currentColor" aria-hidden>
        <path d="M0 0h11v11H0z" />
        <path d="M12 0h11v11H12z" opacity=".8" />
        <path d="M0 12h11v11H0z" opacity=".65" />
        <path d="M12 12h11v11H12z" opacity=".5" />
      </svg>
    );
  if (n === 'gitlab')
    return (
      <svg className={className} viewBox="0 0 500 500" fill="currentColor" aria-hidden>
        <path d="M250 420.5 154.8 127.5h190.4z" />
        <path d="M250 420.5 154.8 127.5H33.2z" opacity=".75" />
        <path d="M33.2 127.5 8.5 203.6c-7.9 24.3 1 51 21.8 66.1L250 420.5z" opacity=".55" />
        <path d="M250 420.5l95.2-293h121.6z" opacity=".75" />
        <path d="m466.8 127.5 24.7 76.1c7.9 24.3-1 51-21.8 66.1L250 420.5z" opacity=".55" />
      </svg>
    );
  if (n === 'facebook')
    return (
      <svg className={className} viewBox="0 0 24 24" fill="currentColor" aria-hidden>
        <path d="M24 12.07C24 5.45 18.63.07 12 .07S0 5.45 0 12.07c0 5.99 4.39 10.95 10.13 11.85v-8.38H7.08v-3.47h3.05V9.43c0-3 1.79-4.67 4.53-4.67 1.31 0 2.69.24 2.69.24v2.95h-1.52c-1.49 0-1.95.93-1.95 1.87v2.25h3.33l-.53 3.47h-2.8v8.38C19.61 23.03 24 18.06 24 12.07z" />
      </svg>
    );
  if (n === 'x' || n === 'twitter')
    return (
      <svg className={className} viewBox="0 0 24 24" fill="currentColor" aria-hidden>
        <path d="M18.24 2.25h3.31l-7.23 8.26 8.5 11.24H16.17l-5.21-6.82-5.97 6.82H1.68l7.73-8.84L1.25 2.25h6.83l4.71 6.23zm-1.16 17.52h1.83L7.08 4.13H5.12z" />
      </svg>
    );
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 7v5l3 2" />
    </svg>
  );
}

export function ProviderBadge({ name, className }: { name: string; className?: string }) {
  return (
    <span className={cx('w-9 h-9 shrink-0 inline-flex items-center justify-center rounded-[9px] border border-line bg-sunken text-ink', className)}>
      <ProviderIcon name={name} />
    </span>
  );
}
