// Theming: single local user, so preferences just live in localStorage —
// no server round-trip, no per-account sync. Also: no Google Fonts
// injection (the source this was adapted from pulls each theme's signature
// font from Google's CDN on selection) — offcode runs fully offline, so
// themes fall back to the system-font stacks already baked into
// css/themes.css instead of fetching anything.
import { THEMES } from './theme-catalog.js';
export { THEMES };

const KEY = 'offcode.theme';
const DEFAULTS = { theme: 'grafit', mode: 'system' };
const system = matchMedia('(prefers-color-scheme: dark)');
let prefs = { ...DEFAULTS };

export function normalizeThemePrefs(value) {
  return {
    theme: THEMES.some((t) => t.id === value?.theme) ? value.theme : 'grafit',
    mode: ['system', 'light', 'dark'].includes(value?.mode) ? value.mode : 'system',
  };
}

export function applyTheme(value) {
  prefs = normalizeThemePrefs(value);
  const resolved = prefs.mode === 'system' ? (system.matches ? 'dark' : 'light') : prefs.mode;
  const root = document.documentElement;
  root.dataset.theme = prefs.theme;
  root.dataset.mode = resolved;
  document.querySelector('meta[name="theme-color"]')?.setAttribute('content', getComputedStyle(root).getPropertyValue('--bg').trim());
  renderPicker();
  return { ...prefs };
}

export function saveThemePrefs(value) {
  applyTheme(value);
  try { localStorage.setItem(KEY, JSON.stringify(prefs)); } catch { /* storage disabled */ }
}

export function loadThemePrefs() {
  let stored = DEFAULTS;
  try { stored = JSON.parse(localStorage.getItem(KEY)) || DEFAULTS; } catch { /* corrupt cache */ }
  return applyTheme(stored);
}

function renderPicker() {
  const grid = document.getElementById('themeGrid');
  if (!grid) return;
  const mode = document.documentElement.dataset.mode;
  if (!grid.children.length) {
    for (const theme of THEMES) {
      const button = document.createElement('button');
      button.type = 'button';
      button.className = 'theme-card';
      button.dataset.theme = theme.id;
      button.title = theme.desc;
      button.innerHTML = `<span class="theme-preview" aria-hidden="true"><i class="theme-preview-panel"></i><i class="theme-preview-dot"></i><i class="theme-preview-line"></i><i class="theme-preview-line short"></i></span><span class="theme-card-title">${theme.name}<span class="theme-check" aria-hidden="true">✓</span></span><span class="theme-card-desc">${theme.desc}</span>${theme.id === 'grafit' ? '<span class="theme-default">Default</span>' : ''}`;
      button.addEventListener('click', () => saveThemePrefs({ ...prefs, theme: theme.id }));
      grid.append(button);
    }
  }
  for (const button of grid.children) {
    const theme = THEMES.find((t) => t.id === button.dataset.theme);
    button.setAttribute('aria-pressed', String(theme.id === prefs.theme));
    ['bg', 'panel', 'acc', 'text'].forEach((key, i) => button.style.setProperty('--preview-' + key, theme.preview[mode][i]));
  }
  document.querySelectorAll('[data-theme-mode]').forEach((button) => button.setAttribute('aria-pressed', String(button.dataset.themeMode === prefs.mode)));
}

export function initThemePicker() {
  document.querySelectorAll('[data-theme-mode]').forEach((button) => button.addEventListener('click', () => saveThemePrefs({ ...prefs, mode: button.dataset.themeMode })));
  renderPicker();
}

system.addEventListener('change', () => { if (prefs.mode === 'system') applyTheme(prefs); });

// Continue the synchronous <head> bootstrap (see index.html's inline
// script) until this module finishes loading.
loadThemePrefs();
