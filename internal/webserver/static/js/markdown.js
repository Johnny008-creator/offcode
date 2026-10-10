/**
 * Markdown -> sanitized HTML. marked/DOMPurify/highlight.js are loaded as
 * global scripts in index.html from the vendored copies in vendor/ (no
 * CDN — offcode runs fully offline). Math (KaTeX) and diagram (Mermaid)
 * rendering from the design this was adapted from are intentionally not
 * ported: both need real network access (CDN-hosted fonts/runtime) that
 * offcode's "runs entirely on your own hardware" stance rules out by
 * default. Unrecognized fenced-code languages (including ```mermaid```)
 * just render as a plain, unhighlighted code block — an honest
 * degradation, not a broken one.
 */

/* global marked, DOMPurify, hljs */

const esc = (s) => s.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));

const PREVIEWABLE = new Set(['html', 'svg']);
const LANG_NAMES = {
  js: 'JavaScript', javascript: 'JavaScript', ts: 'TypeScript', typescript: 'TypeScript',
  py: 'Python', python: 'Python', html: 'HTML', css: 'CSS', json: 'JSON', bash: 'Bash',
  sh: 'Shell', shell: 'Shell', sql: 'SQL', java: 'Java', c: 'C', cpp: 'C++', 'c++': 'C++',
  csharp: 'C#', cs: 'C#', go: 'Go', rust: 'Rust', php: 'PHP', ruby: 'Ruby', svg: 'SVG',
  jsx: 'JSX', tsx: 'TSX', yaml: 'YAML', yml: 'YAML', md: 'Markdown', markdown: 'Markdown',
  kotlin: 'Kotlin', swift: 'Swift', lua: 'Lua', powershell: 'PowerShell', ps1: 'PowerShell',
};

const renderer = new marked.Renderer();

renderer.code = (code, info) => {
  const lang = (info || '').trim().split(/\s+/)[0].toLowerCase();
  let body;
  if (lang && hljs.getLanguage(lang)) {
    try { body = hljs.highlight(code, { language: lang, ignoreIllegals: true }).value; } catch { body = esc(code); }
  } else {
    body = esc(code);
  }
  const lines = code.split('\n').length;
  const name = LANG_NAMES[lang] || lang || 'text';
  const actions =
    (PREVIEWABLE.has(lang) ? `<button type="button" class="cb-btn cb-run" data-act="preview" data-lang="${esc(lang)}">▶ Preview</button>` : '') +
    `<button type="button" class="cb-btn" data-act="download" data-lang="${esc(lang)}">Download</button>` +
    `<button type="button" class="cb-btn" data-act="copy">Copy</button>`;
  return `<div class="codeblock" data-lang="${esc(lang)}"><div class="cb-head"><span class="cb-lang">${esc(name)}</span>` +
    `<span class="cb-lines">${lines} ln</span><span class="cb-actions">${actions}</span></div>` +
    `<pre><code class="hljs">${body}</code></pre></div>`;
};

renderer.table = (header, body) =>
  `<div class="table-wrap"><table><thead>${header}</thead><tbody>${body}</tbody></table></div>`;

marked.setOptions({ renderer, gfm: true, breaks: false });

DOMPurify.addHook('afterSanitizeAttributes', (node) => {
  if (node.tagName === 'A' && node.getAttribute('href')) {
    node.setAttribute('target', '_blank');
    node.setAttribute('rel', 'noopener noreferrer');
  }
  if (node.tagName === 'IMG') {
    // Only real addresses — a model can reference a sandbox-local file
    // path ("outputs/chart.png") that would just be a broken image here.
    if (!/^(https?:|data:image\/)/i.test(node.getAttribute('src') || '')) { node.remove(); return; }
    node.setAttribute('loading', 'lazy');
    node.setAttribute('referrerpolicy', 'no-referrer');
  }
});

export function renderMarkdown(src) {
  return DOMPurify.sanitize(marked.parse(src || ''), { ADD_ATTR: ['target'] });
}

export { esc };
