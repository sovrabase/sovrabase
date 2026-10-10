export interface UserRow {
  id: string;
  email: string;
  username?: string;
  name?: string;
  avatar_url?: string;
  role?: string;
  created_at?: string;
  updated_at?: string;
  is_verified?: boolean;
  mfa_enabled?: boolean;
  _metadata?: { provider: string; provider_id: string }[];
}

export const ROLES = ['user', 'admin', 'moderator'];

export function displayName(u: UserRow): string {
  return u.name || u.username || u.email || u.id;
}

export function initials(u: UserRow): string {
  const s = displayName(u).replace(/@.*/, '');
  const parts = s.split(/[\s._-]+/).filter(Boolean);
  return ((parts[0]?.[0] ?? '?') + (parts[1]?.[0] ?? '')).toUpperCase();
}

/** Sign-in methods a user has: linked OAuth providers, or "email" when none. */
export function userMethods(u: UserRow): string[] {
  const m = (u._metadata ?? []).map((x) => x.provider).filter(Boolean);
  return m.length ? m : ['email'];
}
