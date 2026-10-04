(() => {
  'use strict';

  const el = (id) => document.getElementById(id);
  const transcript = el('transcript');
  const modelBadge = el('model-badge');
  const modeSelect = el('mode-select');
  const activityEl = el('activity');
  const stopBtn = el('stop-btn');
  const input = el('input');
  const composer = el('composer');
  const sendBtn = el('send-btn');
  const planList = el('plan-list');

  const modelPickerOverlay = el('model-picker-overlay');
  const modelListEl = el('model-list');
  const confirmOverlay = el('confirm-overlay');
  const confirmToolName = el('confirm-tool-name');
  const confirmArgs = el('confirm-args');
  const askOverlay = el('ask-overlay');
  const askQuestion = el('ask-question');
  const askOptions = el('ask-options');
  const researchOverlay = el('research-overlay');
  const usageOverlay = el('usage-overlay');

  let lastStatus = null;
  let modelPickerShown = false;
  let lastRenderKey = '';

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

  function escapeHtml(s) {
    return s.replace(/[&<>"']/g, (c) => ({
      '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
    }[c]));
  }

  function renderBlock(b) {
    const div = document.createElement('div');
    div.className = 'block block-' + b.kind;
    switch (b.kind) {
      case 'tool_call': {
        const head = document.createElement('div');
        head.className = 'tool-head';
        head.textContent = '→ ' + b.tool_name;
        div.appendChild(head);
        if (b.tool_args) {
          const pre = document.createElement('pre');
          pre.textContent = b.tool_args;
          div.appendChild(pre);
        }
        break;
      }
      case 'tool_result': {
        if (b.is_error) div.classList.add('is-error');
        div.textContent = (b.tool_name ? b.tool_name + ': ' : '') + b.text;
        break;
      }
      default:
        div.textContent = b.text || '';
    }
    return div;
  }

  function renderTranscript(blocks) {
    const atBottom = transcript.scrollHeight - transcript.scrollTop - transcript.clientHeight < 60;
    transcript.innerHTML = '';
    for (const b of blocks) transcript.appendChild(renderBlock(b));
    if (atBottom) transcript.scrollTop = transcript.scrollHeight;
  }

  function renderPlan(plan) {
    if (!plan || plan.length === 0) {
      planList.className = 'plan-list empty';
      planList.textContent = 'no plan yet';
      return;
    }
    planList.className = 'plan-list';
    planList.innerHTML = '';
    const marks = { pending: '○', in_progress: '●', done: '✓', skipped: '–' };
    for (const step of plan) {
      const row = document.createElement('div');
      row.className = 'plan-step status-' + step.status;
      const mark = document.createElement('span');
      mark.className = 'mark';
      mark.textContent = marks[step.status] || '○';
      const text = document.createElement('span');
      text.textContent = step.text;
      row.appendChild(mark);
      row.appendChild(text);
      planList.appendChild(row);
    }
  }

  function renderModelBadge(state) {
    if (state.model) {
      modelBadge.textContent = state.model_display || state.model;
      modelBadge.title = state.backend + ' · ' + state.model;
    } else {
      modelBadge.textContent = 'no model';
    }
  }

  function renderActivity(state) {
    if (state.status === 'generating' || state.status === 'confirming' || state.status === 'asking') {
      activityEl.textContent = state.activity || 'working…';
      stopBtn.hidden = state.status !== 'generating';
    } else {
      activityEl.textContent = state.tokens_per_sec > 0
        ? state.tokens_per_sec.toFixed(1) + ' tok/s'
        : '';
      stopBtn.hidden = true;
    }
  }

  function setComposerDisabled(disabled) {
    input.disabled = disabled;
    sendBtn.disabled = disabled;
  }

  function showConfirm(c) {
    confirmToolName.textContent = c.tool_name;
    confirmArgs.textContent = c.tool_args;
    confirmOverlay.hidden = false;
  }

  function showAsk(a) {
    askQuestion.textContent = a.question;
    askOptions.innerHTML = '';
    a.options.forEach((opt, i) => {
      const btn = document.createElement('button');
      btn.className = 'ask-option' + (i === a.recommended ? ' recommended' : '');
      btn.textContent = opt + (i === a.recommended ? '' : '');
      if (i === a.recommended) {
        const tag = document.createElement('span');
        tag.className = 'rec-tag';
        tag.textContent = '(recommended)';
        btn.appendChild(tag);
      }
      btn.addEventListener('click', async () => {
        askOverlay.hidden = true;
        try { await api('/api/answer', { answer: opt }); } catch (e) { alert(e.message); }
      });
      askOptions.appendChild(btn);
    });
    askOverlay.hidden = false;
  }

  function render(state) {
    renderModelBadge(state);
    renderActivity(state);
    renderPlan(state.plan);

    const key = JSON.stringify(state.blocks.map((b) => b.text && b.text.length).concat(state.blocks.length));
    if (key !== lastRenderKey) {
      renderTranscript(state.blocks);
      lastRenderKey = key;
    }

    if (document.activeElement !== modeSelect) modeSelect.value = state.mode || 'default';

    setComposerDisabled(state.status !== 'idle');

    confirmOverlay.hidden = !state.confirm;
    if (state.confirm) showConfirm(state.confirm);

    askOverlay.hidden = !state.ask;
    if (state.ask && lastStatus !== 'asking') showAsk(state.ask);

    if (!state.model && !modelPickerShown) {
      modelPickerShown = true;
      openModelPicker();
    } else if (state.model && modelPickerShown) {
      modelPickerShown = false;
      modelPickerOverlay.hidden = true;
    }

    lastStatus = state.status;
  }

  async function openModelPicker() {
    modelPickerOverlay.hidden = false;
    modelListEl.innerHTML = '<div style="color:var(--text-dim)">loading…</div>';
    try {
      const models = await api('/api/models');
      modelListEl.innerHTML = '';
      for (const m of models) {
        const row = document.createElement('div');
        row.className = 'model-item' + (m.known && !m.installed ? ' not-installed' : '');
        const left = document.createElement('div');
        left.innerHTML = `<div>${escapeHtml(m.display_name)}</div><div class="tagline">${escapeHtml(m.tagline || m.tag)}</div>`;
        const right = document.createElement('div');
        right.className = 'tag';
        right.textContent = m.installed ? m.tag : m.tag + ' (not installed)';
        row.appendChild(left);
        row.appendChild(right);
        row.addEventListener('click', async () => {
          try { await api('/api/model', { model: m.tag }); } catch (e) { alert(e.message); }
        });
        modelListEl.appendChild(row);
      }
    } catch (e) {
      modelListEl.innerHTML = '<div style="color:var(--red)">' + escapeHtml(e.message) + '</div>';
    }
  }

  composer.addEventListener('submit', async (e) => {
    e.preventDefault();
    const text = input.value.trim();
    if (!text) return;
    input.value = '';
    input.style.height = 'auto';
    try {
      await api('/api/message', { text });
    } catch (e) {
      alert(e.message);
    }
  });

  input.addEventListener('input', () => {
    input.style.height = 'auto';
    input.style.height = Math.min(input.scrollHeight, 160) + 'px';
  });

  input.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      composer.requestSubmit();
    }
  });

  modeSelect.addEventListener('change', async () => {
    try { await api('/api/mode', { mode: modeSelect.value }); } catch (e) { alert(e.message); }
  });

  stopBtn.addEventListener('click', async () => {
    try { await api('/api/stop'); } catch (e) { /* ignore */ }
  });

  el('confirm-allow').addEventListener('click', async () => {
    confirmOverlay.hidden = true;
    try { await api('/api/confirm', { allow: true }); } catch (e) { alert(e.message); }
  });
  el('confirm-deny').addEventListener('click', async () => {
    confirmOverlay.hidden = true;
    try { await api('/api/confirm', { allow: false }); } catch (e) { alert(e.message); }
  });

  el('research-btn').addEventListener('click', () => {
    el('research-question').value = '';
    researchOverlay.hidden = false;
  });
  el('research-cancel').addEventListener('click', () => { researchOverlay.hidden = true; });
  el('research-start').addEventListener('click', async () => {
    const question = el('research-question').value.trim();
    if (!question) return;
    const depth = document.querySelector('input[name=depth]:checked').value;
    researchOverlay.hidden = true;
    try {
      await api('/api/research', { question, depth });
    } catch (e) {
      alert(e.message);
    }
  });

  el('usage-btn').addEventListener('click', async () => {
    usageOverlay.hidden = false;
    el('usage-report').textContent = 'loading…';
    try {
      const data = await api('/api/usage');
      el('usage-report').textContent = data.report;
    } catch (e) {
      el('usage-report').textContent = e.message;
    }
  });
  el('usage-close').addEventListener('click', () => { usageOverlay.hidden = true; });

  document.addEventListener('keydown', (e) => {
    if (!confirmOverlay.hidden) {
      if (e.key === 'y') el('confirm-allow').click();
      if (e.key === 'n' || e.key === 'Escape') el('confirm-deny').click();
    }
  });

  function connect() {
    const es = new EventSource('/api/events');
    es.onmessage = (ev) => {
      try {
        render(JSON.parse(ev.data));
      } catch (e) {
        console.error('bad state payload', e);
      }
    };
    es.onerror = () => {
      // EventSource auto-reconnects; nothing to do here besides let it retry.
    };
  }

  connect();
})();
