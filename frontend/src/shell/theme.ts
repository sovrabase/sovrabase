export type ThemePref = 'system' | 'light' | 'dark';

const KEY = 'sovrabase_theme';

export function getTheme(): ThemePref {
  try {
    const t = localStorage.getItem(KEY);
    return t === 'light' || t === 'dark' ? t : 'system';
  } catch {
    return 'system';
  }
}

export function setTheme(t: ThemePref) {
  try {
    if (t === 'system') localStorage.removeItem(KEY);
    else localStorage.setItem(KEY, t);
  } catch {
    /* storage blocked: theme still applies for this page view */
  }
  if (t === 'system') delete document.documentElement.dataset.theme;
  else document.documentElement.dataset.theme = t;
}

export type Density = 'comfortable' | 'compact';
export function getDensity(): Density {
  try {
    return localStorage.getItem('sovrabase_density') === 'compact' ? 'compact' : 'comfortable';
  } catch {
    return 'comfortable';
  }
}
export function setDensity(d: Density) {
  try {
    localStorage.setItem('sovrabase_density', d);
  } catch {
    /* ignore */
  }
  document.documentElement.dataset.density = d;
}
