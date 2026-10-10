import { initThemePicker } from './theme.js';
import { renderMarkdown, esc } from './markdown.js';
import { initArtifact, open as openArtifact, download as downloadArtifact, copyText } from './artifact.js';

const el = (id) => document.getElementById(id);
const chat = el('chat');
const messagesEl = el('messages');
const welcome = el('welcome');
const sidebar = el('sidebar');
const sbScrim = el('sbScrim');
const convList = el('convList');
const search = el('search');
const planPanel = el('planPanel');
const planList = el('planList');

const modelBtn = el('modelBtn');
const modelName = el('modelName');
const modelMenu = el('modelMenu');
const modelList = el('modelList');

const modeBtn = el('modeBtn');
const modeLabel = el('modeLabel');
const modeMenu = el('modeMenu');

const activityChip = el('activityChip');
const stopBtn = el('stopBtn');

const composer = el('composer');
const input = el('input');
const sendBtn = el('sendBtn');
const micBtn = el('micBtn');

const confirmDlg = el('confirm');
const confirmToolName = el('confirmToolName');
const confirmArgs = el('confirmArgs');
const askDlg = el('ask');
const askQuestion = el('askQuestion');
const askOptions = el('askOptions');
const researchDlg = el('research');
const usageDlg = el('usage');
const settingsDlg = el('settings');
const toast = el('toast');

let currentAudio = null;
let lastStatus = null;
let lastRenderKey = '';
let lastSessionId = null;
let modelPickerShown = false;
let sendOnEnter = localStorage.getItem('offcode.sendOnEnter') !== 'false';

if (window.marked) { /* renderer configured in markdown.js */ }

async function api(path, body) {
  const opts = body !== undefined
    ? { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }
    : {};
  const res = await fetch(path, opts);
  if (!res.ok) {
    const data = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(data.error || res.statusText);
  }
  return res.json();
}

function showToast(msg) {
  toast.textContent = msg;
  toast.hidden = false;
  clearTimeout(showToast._t);
  showToast._t = setTimeout(() => { toast.hidden = true; }, 2600);
}

// ---- transcript rendering --------------------------------------------------

const ICONS = {
  tool: '<svg viewBox="0 0 24 24"><path d="M14.7 6.3a4 4 0 0 0-5.6 5.6L4 17l3 3 5.1-5.1a4 4 0 0 0 5.6-5.6l-2.5 2.5-2-2z"/></svg>',
  error: '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M12 8v5M12 16h.01"/></svg>',
  think: '<svg viewBox="0 0 24 24"><path d="M9 18h6M10 21h4M12 3a6 6 0 0 0-4 10.5c.7.7 1 1.5 1 2.5h6c0-1 .3-1.8 1-2.5A6 6 0 0 0 12 3z"/></svg>',
};

function fileExt(name) { return (name || '').split('.').pop().toLowerCase(); }

function artifactCard(url, name, blockIndex) {
  const ext = fileExt(name);
  const imgExt = ['png', 'jpg', 'jpeg', 'webp', 'gif'].includes(ext);
  const audioExt = ['mp3', 'wav', 'flac'].includes(ext);
  const videoExt = ['mp4', 'webm'].includes(ext);
  const wrap = document.createElement('div');
  if (imgExt) {
    const img = document.createElement('img');
    img.className = 'art-preview'; img.alt = name; img.src = url;
    wrap.appendChild(img);
  } else if (audioExt) {
    const a = document.createElement('audio'); a.className = 'art-preview'; a.src = url; a.controls = true; a.preload = 'metadata';
    wrap.appendChild(a);
  } else if (videoExt) {
    const v = document.createElement('video'); v.className = 'art-preview'; v.src = url; v.controls = true; v.preload = 'metadata';
    wrap.appendChild(v);
  }
  const card = document.createElement('button');
  card.type = 'button';
  card.className = 'art-card';
  const canLivePreview = ext === 'html' || ext === 'svg';
  card.innerHTML = `<span class="ac-ico">${ICONS.tool}</span><span style="min-width:0;flex:1"><b></b><span></span></span>` +
    `<span class="ac-go">${canLivePreview ? '<svg viewBox="0 0 24 24"><path d="M9 18l6-6-6-6"/></svg>' : '<svg viewBox="0 0 24 24"><path d="M12 3v12M7 10l5 5 5-5M5 21h14"/></svg>'}</span>`;
  card.querySelector('b').textContent = name;
  card.querySelector('.ac-go').previousElementSibling.querySelector('span').textContent = canLivePreview ? 'Open preview' : 'Download file';
  card.addEventListener('click', async () => {
    if (canLivePreview) {
      try {
        const res = await fetch(url);
        const code = await res.text();
        openArtifact({ code, lang: ext === 'svg' ? 'svg' : 'html' });
      } catch (e) { showToast('Could not load: ' + e.message); }
    } else {
      const a = document.createElement('a'); a.href = url; a.download = name; a.click();
    }
  });
  wrap.appendChild(card);
  return wrap;
}

function readAloudButton(text) {
  const btn = document.createElement('button');
  btn.type = 'button'; btn.className = 'icon-btn'; btn.title = 'Read aloud'; btn.dataset.tip = 'Read aloud';
  btn.innerHTML = '<svg viewBox="0 0 24 24"><path d="M11 5L6 9H2v6h4l5 4V5z"/><path d="M19 8a7 7 0 0 1 0 8M15.5 10.5a3 3 0 0 1 0 3"/></svg>';
  btn.addEventListener('click', async () => {
    btn.disabled = true;
    try {
      if (currentAudio) { currentAudio.pause(); URL.revokeObjectURL(currentAudio.src); }
      const res = await fetch('/api/speak', { method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text, language: (navigator.language || 'en').split('-')[0] }) });
      if (!res.ok) throw new Error((await res.json()).error);
      const url = URL.createObjectURL(await res.blob());
      currentAudio = new Audio(url); currentAudio.onended = () => URL.revokeObjectURL(url);
      await currentAudio.play();
    } catch (e) { showToast(e.message); } finally { btn.disabled = false; }
  });
  return btn;
}

function renderStep(block) {
  const step = document.createElement('div');
  step.className = 'step' + (block.result?.is_error ? ' error' : '');
  const head = document.createElement('div');
  head.className = 'step-head';
  head.innerHTML = `<span class="s-ico">${ICONS.tool}</span><span class="s-label">${esc(block.tool_name)}</span>` +
    `<span class="s-arg">${esc(summarizeArgs(block.tool_args))}</span>` +
    `<span class="s-state">${block.result ? '' : '<span class="spinner"></span>'}</span>`;
  step.appendChild(head);
  if (block.result) {
    if (block.result.is_error) {
      const err = document.createElement('div');
      err.className = 'step-err';
      err.textContent = block.result.text || '';
      step.appendChild(err);
    } else if (block.result.text) {
      const details = document.createElement('details');
      const summary = document.createElement('summary');
      summary.textContent = 'Result';
      const body = document.createElement('div');
      body.className = 'step-body';
      const pre = document.createElement('pre');
      pre.textContent = block.result.text.length > 2000 ? block.result.text.slice(0, 2000) + '…' : block.result.text;
      body.appendChild(pre);
      details.appendChild(summary);
      details.appendChild(body);
      step.appendChild(details);
    }
    for (const data of block.result.images || []) {
      const img = document.createElement('img');
      img.className = 'art-preview'; img.alt = 'Tool image'; img.style.margin = '0 12px 10px 46px'; img.style.maxWidth = 'calc(100% - 58px)';
      img.src = 'data:image/' + (data.startsWith('/9j/') ? 'jpeg' : 'png') + ';base64,' + data;
      step.appendChild(img);
    }
    if (block.result.artifact_url) {
      const card = artifactCard(block.result.artifact_url, block.result.artifact_name, 0);
      card.style.margin = '0 12px 10px 46px';
      step.appendChild(card);
    }
  }
  return step;
}

function summarizeArgs(argsJSON) {
  if (!argsJSON) return '';
  try {
    const obj = JSON.parse(argsJSON);
    const first = Object.values(obj)[0];
    return typeof first === 'string' ? first.slice(0, 80) : JSON.stringify(obj).slice(0, 80);
  } catch {
    return argsJSON.slice(0, 80);
  }
}

// Group offcode's flat block list into visual turns: a user block starts a
// new turn, everything after (reasoning/tool_call+tool_result pairs/
// assistant text/system/error) accumulates into one assistant message
// until the next user block.
function groupBlocks(blocks) {
  const turns = [];
  let current = null;
  blocks = blocks || [];
  for (let i = 0; i < blocks.length; i++) {
    const b = blocks[i];
    if (b.kind === 'user') {
      turns.push({ kind: 'user', text: b.text });
      current = { kind: 'assistant', items: [] };
      turns.push(current);
      continue;
    }
    if (!current) { current = { kind: 'assistant', items: [] }; turns.push(current); }
    if (b.kind === 'tool_call') {
      const next = blocks[i + 1];
      const paired = next && next.kind === 'tool_result' ? next : null;
      if (paired) i++;
      current.items.push({ kind: 'step', tool_name: b.tool_name, tool_args: b.tool_args, result: paired });
    } else {
      current.items.push(b);
    }
  }
  return turns;
}

function renderTurn(turn) {
  if (turn.kind === 'user') {
    const msg = document.createElement('div');
    msg.className = 'msg user';
    const bubble = document.createElement('div');
    bubble.className = 'bubble';
    bubble.textContent = turn.text;
    msg.appendChild(bubble);
    return msg;
  }

  const msg = document.createElement('div');
  msg.className = 'msg assistant';
  const avatar = document.createElement('div');
  avatar.className = 'avatar';
  avatar.innerHTML = '<svg viewBox="0 0 24 24"><path d="M6 20a6 6 0 0 1 12 0" fill="none"/><circle cx="12" cy="20" r="2"/></svg>';
  const body = document.createElement('div');
  body.className = 'a-body';

  for (const item of turn.items) {
    if (item.kind === 'step') {
      body.appendChild(renderStep(item));
      continue;
    }
    switch (item.kind) {
      case 'reasoning': {
        const det = document.createElement('details');
        det.className = 'think';
        const summary = document.createElement('summary');
        summary.innerHTML = `${ICONS.think}<span class="t-label">Thinking</span><svg class="chev" viewBox="0 0 24 24"><path d="M6 9l6 6 6-6"/></svg>`;
        const tb = document.createElement('div');
        tb.className = 'think-body';
        tb.textContent = item.text || '';
        det.appendChild(summary); det.appendChild(tb);
        body.appendChild(det);
        break;
      }
      case 'assistant': {
        const prose = document.createElement('div');
        prose.className = 'prose';
        prose.innerHTML = renderMarkdown(item.text || '');
        prose.querySelectorAll('pre code').forEach((code) => {
          try { hljs.highlightElement(code); } catch { /* best-effort */ }
        });
        body.appendChild(prose);
        if ((item.text || '').trim()) body.appendChild(readAloudButton(item.text));
        for (const data of item.images || []) {
          const img = document.createElement('img');
          img.className = 'art-preview'; img.alt = 'Image'; img.src = 'data:image/' + (data.startsWith('/9j/') ? 'jpeg' : 'png') + ';base64,' + data;
          body.appendChild(img);
        }
        if (item.artifact_url) body.appendChild(artifactCard(item.artifact_url, item.artifact_name));
        break;
      }
      case 'error': {
        const box = document.createElement('div');
        box.className = 'err-box';
        box.innerHTML = `${ICONS.error}<span>${esc(item.text || '')}</span>`;
        body.appendChild(box);
        break;
      }
      case 'system': {
        const note = document.createElement('p');
        note.className = 'hint';
        note.textContent = item.text || '';
        body.appendChild(note);
        break;
      }
      default: {
        if (item.text) {
          const pre = document.createElement('div');
          pre.className = 'hint';
          pre.textContent = item.text;
          body.appendChild(pre);
        }
      }
    }
  }

  if (!body.children.length) return null;
  msg.appendChild(avatar);
  msg.appendChild(body);
  return msg;
}

// Only touch the turns that actually changed since the last render — the
// backend pushes a full state snapshot on every single streaming token,
// so a naive innerHTML rebuild on every tick replaces every message's DOM
// node dozens of times a second, replaying each one's entrance animation
// and causing visible flicker across the whole transcript (found live).
// Unchanged turns are left completely untouched; only a turn whose
// content actually changed gets its node swapped, and swapped nodes skip
// the entrance animation (reserved for turns appearing for the first
// time) so an in-progress reply doesn't keep flashing as it grows.
function renderTranscript(blocks) {
  blocks = blocks || [];
  const atBottom = chat.scrollHeight - chat.scrollTop - chat.clientHeight < 80;
  const turns = groupBlocks(blocks);

  while (messagesEl.children.length > turns.length) messagesEl.lastElementChild.remove();

  turns.forEach((turn, i) => {
    const key = JSON.stringify(turn);
    const existing = messagesEl.children[i];
    if (existing && existing.dataset.turnKey === key) return;
    const node = renderTurn(turn);
    if (!node) { if (existing) existing.remove(); return; }
    node.dataset.turnKey = key;
    if (existing) { node.classList.add('no-anim'); existing.replaceWith(node); }
    else messagesEl.appendChild(node);
  });

  document.body.classList.toggle('has-chat', blocks.length > 0);
  if (atBottom) chat.scrollTop = chat.scrollHeight;
}

// Code-block action buttons (Preview/Download/Copy) are generated inside
// rendered markdown by markdown.js — delegate from the transcript root
// rather than attaching a listener per block.
messagesEl.addEventListener('click', async (e) => {
  const btn = e.target.closest('.cb-btn');
  if (!btn) return;
  const block = btn.closest('.codeblock');
  const code = block.querySelector('code').textContent;
  const lang = block.dataset.lang;
  if (btn.dataset.act === 'preview') openArtifact({ code, lang: lang === 'svg' ? 'svg' : 'html' });
  if (btn.dataset.act === 'download') downloadArtifact(code, 'snippet.' + (lang || 'txt'), 'text/plain');
  if (btn.dataset.act === 'copy') { await copyText(code); showToast('Copied'); }
});

// ---- plan -------------------------------------------------------------

function renderPlan(plan) {
  if (!plan || !plan.length) { planPanel.hidden = true; return; }
  planPanel.hidden = false;
  planList.innerHTML = '';
  const marks = { pending: '○', in_progress: '●', done: '✓', skipped: '–' };
  for (const step of plan) {
    const row = document.createElement('div');
    row.className = 'plan-step status-' + step.status;
    row.innerHTML = `<span class="mark">${marks[step.status] || '○'}</span><span></span>`;
    row.querySelector('span:last-child').textContent = step.text;
    planList.appendChild(row);
  }
}

// ---- sessions -----------------------------------------------------------

function fmtWhen(iso) {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return '';
  const now = new Date();
  const sameDay = d.toDateString() === now.toDateString();
  return sameDay ? d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : d.toLocaleDateString([], { month: 'short', day: 'numeric' });
}

async function refreshSessions() {
  try {
    const data = await api('/api/sessions');
    renderSessionList(data.sessions || [], data.current || '');
  } catch (e) {
    convList.innerHTML = '<div class="conv-empty">' + esc(e.message) + '</div>';
  }
}

function renderSessionList(sessions, currentId) {
  const q = search.value.trim().toLowerCase();
  const filtered = q ? sessions.filter((s) => (s.summary || '').toLowerCase().includes(q)) : sessions;
  if (!filtered.length) {
    convList.innerHTML = '<div class="conv-empty">' + (q ? 'No matches' : 'No saved sessions yet') + '</div>';
    return;
  }
  convList.innerHTML = '';
  for (const s of filtered) {
    const row = document.createElement('button');
    row.type = 'button';
    row.className = 'conv' + (s.id === currentId ? ' active' : '');
    row.innerHTML = `<span class="c-title"></span><span class="c-meta"></span>`;
    row.querySelector('.c-title').textContent = s.summary || '(empty session)';
    row.querySelector('.c-meta').textContent = fmtWhen(s.saved_at) + ' · ' + s.messages + ' msgs';
    row.addEventListener('click', async () => {
      if (s.id === currentId) return;
      try { await api('/api/sessions/switch', { id: s.id }); closeMobileSidebar(); } catch (e) { showToast(e.message); }
    });
    convList.appendChild(row);
  }
}
search.addEventListener('input', refreshSessions);

async function newSession() {
  try { await api('/api/sessions/new', {}); closeMobileSidebar(); } catch (e) { showToast(e.message); }
}
el('newChat').addEventListener('click', newSession);
el('newChatBrand').addEventListener('click', newSession);

// ---- sidebar open/close --------------------------------------------------

function closeMobileSidebar() { document.body.classList.remove('sb-mobile-open'); }
el('sbOpen').addEventListener('click', () => {
  if (window.innerWidth <= 820) document.body.classList.add('sb-mobile-open');
  else document.body.classList.remove('sb-collapsed');
});
el('sbClose').addEventListener('click', () => {
  if (window.innerWidth <= 820) closeMobileSidebar();
  else document.body.classList.add('sb-collapsed');
});
sbScrim.addEventListener('click', closeMobileSidebar);

// ---- model picker ---------------------------------------------------------

async function openModelPicker() {
  modelMenu.hidden = false;
  modelBtn.setAttribute('aria-expanded', 'true');
  modelList.innerHTML = '<div class="mm-empty">Loading…</div>';
  try {
    const models = await api('/api/models');
    modelList.innerHTML = '';
    for (const m of models) {
      const row = document.createElement('button');
      row.type = 'button';
      row.className = 'mm-item' + (m.known && !m.installed ? ' not-installed' : '');
      row.setAttribute('role', 'option');
      row.innerHTML = `<span class="mm-name"></span><span class="mm-check">✓</span><span class="mm-blurb"></span>`;
      row.querySelector('.mm-name').textContent = m.display_name;
      if (!m.installed && m.known) {
        const tag = document.createElement('span'); tag.className = 'tag warn'; tag.textContent = 'not installed';
        row.querySelector('.mm-name').appendChild(tag);
      }
      row.querySelector('.mm-blurb').textContent = m.tagline || m.tag;
      row.addEventListener('click', async () => {
        try { await api('/api/model', { model: m.tag }); closeModelMenu(); } catch (e) { showToast(e.message); }
      });
      modelList.appendChild(row);
    }
  } catch (e) {
    modelList.innerHTML = '<div class="mm-empty">' + esc(e.message) + '</div>';
  }
}
function closeModelMenu() { modelMenu.hidden = true; modelBtn.setAttribute('aria-expanded', 'false'); }
modelBtn.addEventListener('click', () => { modelMenu.hidden ? openModelPicker() : closeModelMenu(); });

// ---- mode menu --------------------------------------------------------

const MODE_LABELS = { plan: 'Plan', default: 'Default', auto: 'Auto' };
function closeModeMenu() { modeMenu.hidden = true; modeBtn.setAttribute('aria-expanded', 'false'); }
modeBtn.addEventListener('click', () => {
  if (modeMenu.hidden) { modeMenu.hidden = false; modeBtn.setAttribute('aria-expanded', 'true'); } else closeModeMenu();
});
modeMenu.querySelectorAll('button[data-mode]').forEach((btn) => {
  btn.addEventListener('click', async () => {
    closeModeMenu();
    try { await api('/api/mode', { mode: btn.dataset.mode }); } catch (e) { showToast(e.message); }
  });
});

document.addEventListener('click', (e) => {
  if (!modelMenu.hidden && !e.target.closest('.model-picker')) closeModelMenu();
  if (!modeMenu.hidden && !e.target.closest('.mode')) closeModeMenu();
});

// ---- confirm / ask --------------------------------------------------------

function showConfirm(c) {
  confirmToolName.textContent = c.tool_name;
  confirmArgs.textContent = c.preview || c.tool_args;
  if (!confirmDlg.open) confirmDlg.showModal();
}
el('confirmAllow').addEventListener('click', async () => {
  confirmDlg.close();
  try { await api('/api/confirm', { allow: true }); } catch (e) { showToast(e.message); }
});
el('confirmDeny').addEventListener('click', async () => {
  confirmDlg.close();
  try { await api('/api/confirm', { allow: false }); } catch (e) { showToast(e.message); }
});

function showAsk(a) {
  askQuestion.textContent = a.question;
  askOptions.innerHTML = '';
  a.options.forEach((opt, i) => {
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'ask-option' + (i === a.recommended ? ' recommended' : '');
    btn.innerHTML = `<span></span>${i === a.recommended ? '<span class="rec-tag">Recommended</span>' : ''}`;
    btn.querySelector('span').textContent = opt;
    btn.addEventListener('click', async () => {
      askDlg.close();
      try { await api('/api/answer', { answer: opt }); } catch (e) { showToast(e.message); }
    });
    askOptions.appendChild(btn);
  });
  if (!askDlg.open) askDlg.showModal();
}

document.addEventListener('keydown', (e) => {
  if (confirmDlg.open) {
    if (e.key === 'y') el('confirmAllow').click();
    if (e.key === 'n' || e.key === 'Escape') el('confirmDeny').click();
  }
  if (e.key === 'Escape' && !confirmDlg.open && !askDlg.open) {
    if (!stopBtn.hidden) stopBtn.click();
  }
  if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key.toLowerCase() === 'o') { e.preventDefault(); newSession(); }
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'b') { e.preventDefault(); el('sbOpen').click(); }
});

// ---- research ---------------------------------------------------------

el('researchBtn').addEventListener('click', () => {
  el('researchQuestion').value = '';
  researchDlg.showModal();
});
el('researchCancel').addEventListener('click', () => researchDlg.close());
el('researchStart').addEventListener('click', async () => {
  const question = el('researchQuestion').value.trim();
  if (!question) return;
  const depth = document.querySelector('input[name=depth]:checked').value;
  researchDlg.close();
  try { await api('/api/research', { question, depth }); } catch (e) { showToast(e.message); }
});

// ---- usage --------------------------------------------------------------

el('usageBtn').addEventListener('click', async () => {
  usageDlg.showModal();
  const body = el('usageBody');
  body.innerHTML = '<p class="hint">Loading…</p>';
  try {
    const data = await api('/api/usage');
    const rows = data.summaries || [];
    body.innerHTML = '';
    if (rows.length) {
      const chart = document.createElement('div');
      chart.className = 'usage-chart';
      const max = Math.max(1, ...rows.map((r) => r.PromptTokens + r.CompletionTokens));
      for (const row of rows) {
        const label = document.createElement('label');
        const count = row.PromptTokens + row.CompletionTokens;
        label.textContent = row.Tag + ' · ' + count.toLocaleString() + ' tokens';
        const meter = document.createElement('meter'); meter.min = 0; meter.max = max; meter.value = count;
        label.appendChild(document.createElement('br'));
        label.appendChild(meter);
        chart.appendChild(label);
      }
      body.appendChild(chart);
    }
    const pre = document.createElement('pre');
    pre.className = 'confirm-args';
    pre.textContent = data.report || 'No usage recorded yet.';
    body.appendChild(pre);
  } catch (e) {
    body.innerHTML = '<p class="hint">' + esc(e.message) + '</p>';
  }
});
el('usageClose').addEventListener('click', () => usageDlg.close());

// ---- export -------------------------------------------------------------

el('exportBtn').addEventListener('click', () => {
  const lines = [];
  document.querySelectorAll('.msg').forEach((msg) => {
    if (msg.classList.contains('user')) lines.push('**You:** ' + msg.querySelector('.bubble').textContent, '');
    else {
      const prose = msg.querySelectorAll('.prose');
      prose.forEach((p) => lines.push(p.textContent, ''));
    }
  });
  downloadArtifact(lines.join('\n'), 'offcode-session.md', 'text/markdown');
});

// ---- settings -------------------------------------------------------------

el('openSettings').addEventListener('click', () => settingsDlg.showModal());
const sendOnEnterToggle = el('sendOnEnter');
sendOnEnterToggle.checked = sendOnEnter;
sendOnEnterToggle.addEventListener('change', () => {
  sendOnEnter = sendOnEnterToggle.checked;
  localStorage.setItem('offcode.sendOnEnter', String(sendOnEnter));
});

// ---- mic / dictation --------------------------------------------------

let recorder = null;
let recordingTimer = null;
micBtn.addEventListener('click', async () => {
  if (recorder && recorder.state === 'recording') { recorder.stop(); return; }
  let stream;
  try {
    if (!navigator.mediaDevices || !window.MediaRecorder) throw new Error('Audio recording is not supported in this browser.');
    stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    const recording = new MediaRecorder(stream); recorder = recording;
    const chunks = [];
    recording.ondataavailable = (e) => { if (e.data.size) chunks.push(e.data); };
    recording.onstop = async () => {
      clearTimeout(recordingTimer); stream.getTracks().forEach((t) => t.stop());
      micBtn.classList.remove('rec'); micBtn.disabled = true;
      try {
        const res = await fetch('/api/transcribe', { method: 'POST', body: new Blob(chunks, { type: recording.mimeType }) });
        const data = await res.json(); if (!res.ok) throw new Error(data.error);
        input.value = [input.value, data.text].filter(Boolean).join(' '); input.dispatchEvent(new Event('input'));
      } catch (e) { showToast(e.message); } finally { recorder = null; micBtn.disabled = false; }
    };
    recording.start(); micBtn.classList.add('rec');
    recordingTimer = setTimeout(() => { if (recording.state === 'recording') recording.stop(); }, 120000);
  } catch (e) {
    if (stream) stream.getTracks().forEach((t) => t.stop());
    showToast(e.message); micBtn.classList.remove('rec'); recorder = null;
  }
});

// ---- composer -----------------------------------------------------------

composer.addEventListener('submit', async (e) => {
  e.preventDefault();
  const text = input.value.trim();
  if (!text) return;
  input.value = '';
  input.style.height = 'auto';
  try { await api('/api/message', { text }); } catch (e) { showToast(e.message); }
});
input.addEventListener('input', () => {
  input.style.height = 'auto';
  input.style.height = Math.min(input.scrollHeight, 320) + 'px';
});
input.addEventListener('keydown', (e) => {
  if (e.key === 'Enter' && !e.shiftKey && sendOnEnter) { e.preventDefault(); composer.requestSubmit(); }
});
stopBtn.addEventListener('click', async () => { try { await api('/api/stop', {}); } catch { /* ignore */ } });

// ---- suggestions ------------------------------------------------------

const SUGGESTIONS = [
  { icon: 'tool', title: 'Explore this project', sub: 'What does this codebase do, and how is it laid out?' },
  { icon: 'tool', title: 'Fix a bug', sub: 'Help me track down and fix an issue' },
  { icon: 'tool', title: 'Write a test', sub: 'Add test coverage for a function or module' },
  { icon: 'tool', title: 'Explain some code', sub: 'Walk me through how something in this repo works' },
];
const suggestionsEl = el('suggestions');
for (const s of SUGGESTIONS) {
  const btn = document.createElement('button');
  btn.type = 'button'; btn.className = 'sugg';
  btn.innerHTML = `<span class="sugg-ico">${ICONS.tool}</span><span><b></b><span></span></span>`;
  btn.querySelector('b').textContent = s.title;
  btn.querySelector('span > span').textContent = s.sub;
  btn.addEventListener('click', () => { input.value = s.sub; input.dispatchEvent(new Event('input')); input.focus(); });
  suggestionsEl.appendChild(btn);
}

// ---- state / SSE ------------------------------------------------------

function renderActivity(state) {
  if (state.status === 'generating' || state.status === 'confirming' || state.status === 'asking') {
    activityChip.hidden = false;
    activityChip.textContent = state.activity || 'working…';
    stopBtn.hidden = state.status !== 'generating';
  } else {
    stopBtn.hidden = true;
    if (state.tokens_per_sec > 0) { activityChip.hidden = false; activityChip.textContent = state.tokens_per_sec.toFixed(1) + ' tok/s'; }
    else activityChip.hidden = true;
  }
}

function render(state) {
  modelName.textContent = state.model ? (state.model_display || state.model) : 'No model';
  if (document.activeElement !== modeBtn) {
    modeLabel.textContent = MODE_LABELS[state.mode] || 'Default';
    modeMenu.querySelectorAll('button[data-mode]').forEach((b) => b.setAttribute('aria-checked', String(b.dataset.mode === state.mode)));
  }
  renderActivity(state);
  renderPlan(state.plan);

  const key = JSON.stringify([state.session_id, state.blocks]);
  if (key !== lastRenderKey) { renderTranscript(state.blocks); lastRenderKey = key; }

  if (state.session_id !== lastSessionId || (lastStatus !== 'idle' && state.status === 'idle')) refreshSessions();
  lastSessionId = state.session_id;

  input.disabled = state.status !== 'idle';
  sendBtn.disabled = state.status !== 'idle' || !input.value.trim();

  if (state.confirm) showConfirm(state.confirm);
  else if (confirmDlg.open) confirmDlg.close();

  if (state.ask && lastStatus !== 'asking') showAsk(state.ask);
  else if (!state.ask && askDlg.open) askDlg.close();

  if (!state.model && !modelPickerShown) { modelPickerShown = true; openModelPicker(); }
  else if (state.model && modelPickerShown) { modelPickerShown = false; closeModelMenu(); }

  lastStatus = state.status;
}
input.addEventListener('input', () => { sendBtn.disabled = input.disabled || !input.value.trim(); });

function connect() {
  const es = new EventSource('/api/events');
  es.onmessage = (ev) => {
    try { render(JSON.parse(ev.data)); } catch (e) { console.error('bad state payload', e); }
  };
}

// ---- boot -----------------------------------------------------------------

initThemePicker();
initArtifact({ toast: showToast });
connect();
refreshSessions();
document.body.classList.remove('booting');
