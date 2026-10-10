/**
 * Live-preview panel (HTML apps, SVG) — shown either from a code block's
 * "Preview" button (the code the model streamed, previewed client-side)
 * or from a real file offcode's write/generate_image tools produced
 * (fetched from /api/artifact, then shown the same way).
 *
 * Security: the iframe is sandboxed WITHOUT allow-same-origin, so code
 * from the model runs in an opaque origin — it can't reach this page's
 * state or the offcode backend's session cookie (there isn't one, but the
 * principle holds). "Open in new tab" doesn't open the code directly
 * either (that blob would share our origin) — it wraps it in another
 * sandboxed iframe first.
 */

/* global hljs */

import { esc } from './markdown.js';

const $ = (s) => document.querySelector(s);
const panel = $('#artifact');
const frame = $('#artFrame');
const codeEl = $('#artCode');
const titleEl = $('#artTitle');

let current = null; // { code, lang, title }
let onToast = () => {};

export function initArtifact({ toast }) {
  onToast = toast;

  panel.querySelectorAll('.art-tab').forEach((t) =>
    t.addEventListener('click', () => showTab(t.dataset.tab)));

  panel.querySelector('.art-actions').addEventListener('click', (e) => {
    const act = e.target.closest('[data-art]')?.dataset.art;
    if (!act || !current) return;
    if (act === 'close') close();
    if (act === 'reload') load();
    if (act === 'copy') copyText(current.code).then(() => onToast('Code copied'));
    if (act === 'download') download(current.code, filename(current), current.lang === 'svg' ? 'image/svg+xml' : 'text/html');
    if (act === 'open') openInTab();
    if (act === 'full') panel.classList.toggle('full');
  });

  const handle = $('#artResize');
  handle.addEventListener('pointerdown', (e) => {
    e.preventDefault();
    document.body.classList.add('resizing');
    const move = (ev) => {
      const w = Math.min(Math.max(window.innerWidth - ev.clientX, 340), window.innerWidth * 0.75);
      document.documentElement.style.setProperty('--art-w', w + 'px');
    };
    const up = () => {
      document.body.classList.remove('resizing');
      removeEventListener('pointermove', move);
      removeEventListener('pointerup', up);
    };
    addEventListener('pointermove', move);
    addEventListener('pointerup', up);
  });

  addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && panel.classList.contains('full')) panel.classList.remove('full');
  });
}

export function titleOf(code, lang) {
  const t = code.match(/<title[^>]*>([^<]{1,80})<\/title>/i)?.[1]?.trim();
  if (t) return t;
  const h = code.match(/<h1[^>]*>([^<]{1,60})<\/h1>/i)?.[1]?.trim();
  if (h) return h;
  return lang === 'svg' ? 'SVG image' : 'App preview';
}

export function open({ code, lang = 'html' }) {
  current = { code, lang, title: titleOf(code, lang) };
  titleEl.textContent = current.title;
  panel.hidden = false;
  document.body.classList.add('art-open');
  showTab('preview');
  load();
}

export function close() {
  panel.hidden = true;
  panel.classList.remove('full');
  document.body.classList.remove('art-open');
  frame.srcdoc = '';
  current = null;
}

export const isOpen = () => !panel.hidden;

// In a sandbox without allow-same-origin, even touching localStorage
// throws and takes down the whole script (typically a game's "high
// score"). Shim it with an in-memory store instead — the app runs, it
// just doesn't remember anything after the preview closes.
const STORAGE_SHIM = `<script>(function(){function mem(){var d={};return{getItem:function(k){return Object.prototype.hasOwnProperty.call(d,k)?d[k]:null},setItem:function(k,v){d[k]=String(v)},removeItem:function(k){delete d[k]},clear:function(){d={}},key:function(i){var k=Object.keys(d)[i];return k===undefined?null:k},get length(){return Object.keys(d).length}}}
['localStorage','sessionStorage'].forEach(function(n){try{window[n].length}catch(e){try{Object.defineProperty(window,n,{value:mem(),configurable:true})}catch(_){}}});
try{document.cookie}catch(e){try{Object.defineProperty(document,'cookie',{get:function(){return''},set:function(){},configurable:true})}catch(_){}}})();<\/script>`;

function withShim(html) {
  if (/<head[^>]*>/i.test(html)) return html.replace(/<head[^>]*>/i, (m) => m + STORAGE_SHIM);
  if (/<html[^>]*>/i.test(html)) return html.replace(/<html[^>]*>/i, (m) => m + STORAGE_SHIM);
  return STORAGE_SHIM + html;
}

function docFor(a) {
  return withShim(rawDoc(a));
}

function rawDoc({ code, lang }) {
  if (lang === 'svg') {
    return `<!DOCTYPE html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<style>html,body{margin:0;height:100%;background:#fff}body{display:grid;place-items:center;padding:24px;box-sizing:border-box}
svg{max-width:100%;max-height:100%;height:auto}</style></head><body>${code}</body></html>`;
  }
  // A fragment without <html>/<body> gets wrapped so it at least has an
  // encoding and a viewport.
  if (!/<html[\s>]/i.test(code) && !/<!doctype/i.test(code)) {
    return `<!DOCTYPE html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head><body>${code}</body></html>`;
  }
  return code;
}

function load() {
  frame.srcdoc = '';
  // Next frame — otherwise an iframe with the same srcdoc won't reload.
  requestAnimationFrame(() => { frame.srcdoc = docFor(current); });
  const lang = current.lang === 'svg' ? 'xml' : 'html';
  codeEl.firstElementChild.innerHTML = hljs.highlight(current.code, { language: lang, ignoreIllegals: true }).value;
}

function showTab(tab) {
  panel.querySelectorAll('.art-tab').forEach((t) => t.setAttribute('aria-selected', String(t.dataset.tab === tab)));
  frame.hidden = tab !== 'preview';
  codeEl.hidden = tab !== 'code';
}

function filename({ title, lang }) {
  const base = title.normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/[^a-z0-9]+/gi, '-').replace(/^-|-$/g, '').toLowerCase() || 'artifact';
  return `${base}.${lang === 'svg' ? 'svg' : 'html'}`;
}

function openInTab() {
  const wrapper = `<!DOCTYPE html><html><head><meta charset="utf-8"><title>${esc(current.title)}</title>
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>html,body,iframe{margin:0;width:100%;height:100%;border:0;display:block;background:#fff}</style></head>
<body><iframe sandbox="allow-scripts allow-modals allow-forms allow-popups allow-pointer-lock allow-downloads" allow="fullscreen; autoplay" srcdoc="${esc(docFor(current))}"></iframe></body></html>`;
  const url = URL.createObjectURL(new Blob([wrapper], { type: 'text/html' }));
  window.open(url, '_blank', 'noopener');
  setTimeout(() => URL.revokeObjectURL(url), 60_000);
}

export function download(text, name, type = 'text/plain') {
  const url = URL.createObjectURL(new Blob([text], { type: type + ';charset=utf-8' }));
  const a = Object.assign(document.createElement('a'), { href: url, download: name });
  document.body.append(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 5000);
}

export async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    const ta = Object.assign(document.createElement('textarea'), { value: text });
    document.body.append(ta);
    ta.select();
    document.execCommand('copy');
    ta.remove();
  }
}
