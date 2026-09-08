/* evie/tool demo · app.js
 * 纯原生 JS + fetch。覆盖 4 个 HTTP 接口：
 *   1) POST /evie/tool/v1/asr:recognize  (音频 → 文本 + 增强)
 *   2) POST /evie/tool/v1/enhance         (纯文本增强)
 *   3) GET  /evie/tool/v1/admin/vocab/status
 *   4) POST /evie/tool/v1/admin/vocab:sync
 *   5) GET  /health/ready                 (健康检查)
 *
 * UI 状态：所有响应字段都是 Readonly DOM 渲染，无状态管理库依赖。
 */

// ===== 全局状态 =====
const state = {
  audioBase64: null,        // 当前选中音频的 base64
  audioEncoding: null,      // wav / mp3 / pcm / opus
  audioMeta: null,          // {name, size}
  audioUrl: null,           // url 模式下用
  recordBlob: null,         // 录音模式用
  recording: false,
  mediaRecorder: null,
  recordStart: 0,
  recordTimer: null,
};

// ===== DOM 工具 =====
const $ = (id) => document.getElementById(id);
const $$ = (sel) => document.querySelectorAll(sel);

// ===== 初始化事件 =====
document.addEventListener('DOMContentLoaded', () => {
  bindConfigEvents();
  bindAudioSourceTabs();
  bindAudioEvents();
  bindTextEvents();
  bindAdminEvents();
  // 自动 ping 一次
  pingHealth();
});

function bindConfigEvents() {
  $('checkHealth').addEventListener('click', pingHealth);
}

function bindAudioSourceTabs() {
  $$('.source-tab').forEach(tab => {
    tab.addEventListener('click', () => {
      $$('.source-tab').forEach(t => t.classList.remove('active'));
      $$('.source-panel').forEach(p => p.classList.remove('active'));
      tab.classList.add('active');
      $(`panel-${tab.dataset.tab}`).classList.add('active');
    });
  });
}

function bindAudioEvents() {
  // 文件拖拽
  const dz = $('dropZone');
  const fi = $('fileInput');

  $('selectFileBtn').addEventListener('click', (e) => {
    e.stopPropagation();
    fi.click();
  });
  dz.addEventListener('click', () => fi.click());

  ['dragenter', 'dragover'].forEach(evt =>
    dz.addEventListener(evt, (e) => {
      e.preventDefault();
      dz.classList.add('dragover');
    })
  );
  ['dragleave', 'drop'].forEach(evt =>
    dz.addEventListener(evt, (e) => {
      e.preventDefault();
      dz.classList.remove('dragover');
    })
  );

  dz.addEventListener('drop', (e) => {
    const f = e.dataTransfer.files[0];
    if (f) handleAudioFile(f);
  });
  fi.addEventListener('change', (e) => {
    const f = e.target.files[0];
    if (f) handleAudioFile(f);
  });

  // URL
  $('loadUrlBtn').addEventListener('click', loadAudioFromUrl);

  // 录音
  $('recordBtn').addEventListener('click', toggleRecording);

  // 识别
  $('recognizeBtn').addEventListener('click', recognizeAudio);
}

function bindTextEvents() {
  $('enhanceBtn').addEventListener('click', enhanceText);
}

function bindAdminEvents() {
  $('syncStatusBtn').addEventListener('click', getVocabStatus);
}

// ===== Health Check =====
async function pingHealth() {
  const baseUrl = $('baseUrl').value.replace(/\/$/, '');
  setStatus('checking', '检查中...');
  try {
    const r = await fetch(`${baseUrl}/health/ready`, { method: 'GET' });
    if (!r.ok) {
      setStatus('unhealthy', `HTTP ${r.status}`);
      return;
    }
    const data = await r.json();
    if (data.status === 'ok') {
      setStatus('healthy', `就绪 · ${JSON.stringify(data.details || {})}`);
    } else {
      setStatus('unhealthy', `不可用 · ${JSON.stringify(data.details || {})}`);
    }
  } catch (err) {
    setStatus('unhealthy', `连接失败: ${err.message}`);
  }
}

function setStatus(level, text) {
  const badge = $('statusBadge');
  badge.className = `status-badge ${level}`;
  $('statusText').textContent = text;
}

// ===== Audio File =====
function handleAudioFile(file) {
  if (!file.type.startsWith('audio/') && !file.name.match(/\.(mp3|wav|pcm|opus)$/i)) {
    alert('请选择音频文件（mp3 / wav / pcm / opus）');
    return;
  }
  const enc = file.name.toLowerCase().endsWith('.mp3') ? 'mp3'
    : file.name.toLowerCase().endsWith('.wav') ? 'wav'
    : file.name.toLowerCase().endsWith('.pcm') ? 'pcm'
    : file.name.toLowerCase().endsWith('.opus') ? 'opus'
    : 'wav';

  const reader = new FileReader();
  reader.onload = () => {
    const dataUrl = reader.result;
    const base64 = dataUrl.split(',')[1];
    state.audioBase64 = base64;
    state.audioEncoding = enc;
    state.audioMeta = {
      name: file.name,
      size: file.size,
      type: file.type || `audio/${enc}`,
    };
    showFileInfo();
    enableRecognize(true);
  };
  reader.onerror = () => alert('文件读取失败');
  reader.readAsDataURL(file);
}

function showFileInfo() {
  const m = state.audioMeta;
  $('fileName').textContent = m.name;
  $('fileMeta').textContent = `${formatSize(m.size)} · ${m.type} · ${state.audioEncoding}`;
  $('fileInfo').classList.remove('hidden');
}

// ===== Audio URL =====
async function loadAudioFromUrl() {
  const url = $('audioUrl').value.trim();
  if (!url) {
    alert('请输入音频 URL');
    return;
  }
  setStatus('checking', '下载音频...');
  try {
    const r = await fetch(url);
    if (!r.ok) throw new Error(`HTTP ${r.status}`);
    const blob = await r.blob();
    const enc = url.toLowerCase().endsWith('.mp3') ? 'mp3'
      : url.toLowerCase().endsWith('.wav') ? 'wav'
      : url.toLowerCase().endsWith('.pcm') ? 'pcm'
      : url.toLowerCase().endsWith('.opus') ? 'opus'
      : 'wav';

    const reader = new FileReader();
    reader.onload = () => {
      const base64 = reader.result.split(',')[1];
      state.audioBase64 = base64;
      state.audioEncoding = enc;
      state.audioUrl = url;
      state.audioMeta = {
        name: url.split('/').pop(),
        size: blob.size,
        type: blob.type || `audio/${enc}`,
      };
      showFileInfo();
      enableRecognize(true);
      setStatus('healthy', '音频已加载');
    };
    reader.readAsDataURL(blob);
  } catch (err) {
    setStatus('unhealthy', `下载失败: ${err.message}`);
  }
}

// ===== Recording =====
async function toggleRecording() {
  if (state.recording) {
    stopRecording();
  } else {
    startRecording();
  }
}

async function startRecording() {
  try {
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    const mr = new MediaRecorder(stream);
    state.mediaRecorder = mr;
    state.recording = true;
    state.recordStart = Date.now();

    const chunks = [];
    mr.ondataavailable = (e) => chunks.push(e.data);
    mr.onstop = () => {
      const blob = new Blob(chunks, { type: 'audio/webm' });
      state.recordBlob = blob;
      const url = URL.createObjectURL(blob);
      const preview = $('recordPreview');
      preview.src = url;
      preview.hidden = false;

      const reader = new FileReader();
      reader.onload = () => {
        const base64 = reader.result.split(',')[1];
        state.audioBase64 = base64;
        state.audioEncoding = 'wav'; // MediaRecorder 输出的 webm，服务端需支持
        state.audioMeta = {
          name: `recording-${Date.now()}.webm`,
          size: blob.size,
          type: 'audio/webm',
        };
        enableRecognize(true);
      };
      reader.readAsDataURL(blob);
      stream.getTracks().forEach(t => t.stop());
    };

    mr.start();
    $('recordBtn').classList.add('recording');
    $('recLabel').textContent = '停止录制';

    // 计时
    state.recordTimer = setInterval(() => {
      const sec = Math.floor((Date.now() - state.recordStart) / 1000);
      const mm = String(Math.floor(sec / 60)).padStart(2, '0');
      const ss = String(sec % 60).padStart(2, '0');
      $('recordTime').textContent = `${mm}:${ss}`;
    }, 200);
  } catch (err) {
    alert(`无法访问麦克风: ${err.message}`);
  }
}

function stopRecording() {
  if (state.mediaRecorder && state.mediaRecorder.state !== 'inactive') {
    state.mediaRecorder.stop();
  }
  state.recording = false;
  clearInterval(state.recordTimer);
  $('recordBtn').classList.remove('recording');
  $('recLabel').textContent = '开始录制';
}

function enableRecognize(enabled) {
  $('recognizeBtn').disabled = !enabled;
}

// ===== Recognize =====
async function recognizeAudio() {
  if (!state.audioBase64) {
    alert('请先选择音频');
    return;
  }
  const baseUrl = $('baseUrl').value.replace(/\/$/, '');
  const token = $('token').value.trim();
  const enhance = $('enableEnhance').checked;

  const body = {
    format: {
      encoding: state.audioEncoding,
      sampleRate: 16000,
      bitDepth: 16,
      channels: 1,
    },
    audioData: state.audioBase64,
    sessionId: `demo-${Date.now()}`,
    enableEnhancement: enhance,
  };

  const btn = $('recognizeBtn');
  btn.disabled = true;
  btn.textContent = '识别中...';
  setStatus('checking', 'ASR 识别中...');

  try {
    const r = await fetch(`${baseUrl}/evie/tool/v1/asr:recognize`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
      },
      body: JSON.stringify(body),
    });

    const text = await r.text();
    let data;
    try { data = JSON.parse(text); } catch { data = { error_message: text }; }

    if (!r.ok) {
      showError(r.status, data);
      setStatus('unhealthy', `HTTP ${r.status}`);
    } else {
      renderResult(data, 'asr');
      setStatus('healthy', '识别完成');
    }
  } catch (err) {
    showError(0, { error_message: err.message });
    setStatus('unhealthy', '请求失败');
  } finally {
    btn.disabled = false;
    btn.textContent = '▶ 识别并增强';
  }
}

// ===== Enhance Text =====
async function enhanceText() {
  const text = $('textInput').value.trim();
  if (!text) {
    alert('请输入要增强的文本');
    return;
  }
  const baseUrl = $('baseUrl').value.replace(/\/$/, '');
  const token = $('token').value.trim();

  const btn = $('enhanceBtn');
  btn.disabled = true;
  btn.textContent = '增强中...';

  try {
    const r = await fetch(`${baseUrl}/evie/tool/v1/enhance`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
      },
      body: JSON.stringify({ text }),
    });

    const data = await r.json();
    if (!r.ok) {
      showError(r.status, data);
    } else {
      renderResult(data, 'enhance');
    }
  } catch (err) {
    showError(0, { error_message: err.message });
  } finally {
    btn.disabled = false;
    btn.textContent = '▶ 增强';
  }
}

// ===== Render Result =====
function renderResult(data, kind) {
  $('resultCard').classList.remove('hidden');
  $('resultCard').scrollIntoView({ behavior: 'smooth', block: 'start' });

  // 状态 pill
  const statusMap = { 1: 'success', 2: 'degraded' };
  const statusTextMap = { 1: 'SUCCESS', 2: 'DEGRADED' };
  const status = statusMap[data.status] || 'unknown';
  $('resultStatus').className = `status-pill ${status}`;
  $('resultStatus').textContent = statusTextMap[data.status] || 'UNKNOWN';

  // 元数据
  $('resultProvider').textContent = `provider: ${data.provider_name || '—'}`;
  $('resultConfidence').textContent = `conf: ${data.confidence != null ? data.confidence.toFixed(2) : '—'}`;
  $('resultDuration').textContent = `dur: ${data.duration_ms ? (data.duration_ms / 1000).toFixed(1) + 's' : '—'}`;

  const rawText = data.raw_text || data.original_text || '';
  const enhancedText = data.enhanced_text || '';

  // 文本渲染
  $('rawText').textContent = rawText || '—';
  $('enhancedText').innerHTML = renderDiff(rawText, enhancedText, data.changes || []);

  // 改动详情
  const changes = data.changes || [];
  $('changeCount').textContent = changes.length;
  renderChanges(changes);

  // 耗时
  renderTimings(data, kind);

  // 原始 JSON
  $('rawJson').textContent = JSON.stringify(data, null, 2);
}

// ===== Diff 渲染（高亮 from → to）=====
function renderDiff(raw, enhanced, changes) {
  if (!enhanced) return '—';
  if (!changes || changes.length === 0) return escapeHtml(enhanced);

  // 在 enhanced 文本里高亮 replacements
  let html = escapeHtml(enhanced);
  // 按 replacement 长度倒序排，避免短串先替换影响长串
  const sorted = [...changes].sort((a, b) =>
    (b.replacement || '').length - (a.replacement || '').length
  );
  for (const ch of sorted) {
    const from = ch.original || '';
    const to = ch.replacement || '';
    if (!to) continue;
    // 转义后做替换
    const fromEsc = escapeHtml(from);
    const toEsc = escapeHtml(to);
    // 替换最后一次出现（避免误伤）
    const idx = html.lastIndexOf(toEsc);
    if (idx >= 0) {
      html = html.slice(0, idx) +
        `<mark class="to">${toEsc}</mark>` +
        html.slice(idx + toEsc.length);
    }
    // 高亮原始文本中的 from（如果有）
    if (from && from !== to) {
      const fromIdx = html.indexOf(`<mark class="to">${toEsc}</mark>`);
      if (fromIdx === -1) {
        const rawIdx = html.indexOf(fromEsc);
        if (rawIdx >= 0) {
          html = html.slice(0, rawIdx) +
            `<mark class="from">${fromEsc}</mark>` +
            html.slice(rawIdx + fromEsc.length);
        }
      }
    }
  }
  return html;
}

function renderChanges(changes) {
  const container = $('changesList');
  container.innerHTML = '';
  if (changes.length === 0) {
    container.innerHTML = '<div class="no-changes">✨ 无改动</div>';
    return;
  }
  for (const ch of changes) {
    const item = document.createElement('div');
    item.className = 'change-item';
    item.innerHTML = `
      <span class="change-kind">${escapeHtml(ch.kind || '—')}</span>
      <span class="change-from">${escapeHtml(ch.original || '')}</span>
      <span class="change-to">${escapeHtml(ch.replacement || '')}</span>
      <span class="change-confidence">${ch.confidence != null ? ch.confidence.toFixed(2) : '—'}</span>
    `;
    container.appendChild(item);
  }
}

function renderTimings(data, kind) {
  const fields = [
    { key: 'cleaning_time_ms', label: 'cleaning' },
    { key: 'filler_time_ms', label: 'filler' },
    { key: 'vocab_match_time_ms', label: 'vocab match' },
    { key: 'alias_time_ms', label: 'alias' },
    { key: 'deterministic_time_ms', label: 'deterministic' },
    { key: 'pinyin_time_ms', label: 'pinyin' },
    { key: 'fuzzy_time_ms', label: 'fuzzy' },
    { key: 'context_time_ms', label: 'context' },
  ];

  const values = fields.map(f => ({ ...f, value: data[f.key] || 0 }));
  const max = Math.max(...values.map(v => v.value), 1);

  const container = $('timingBars');
  container.innerHTML = '';
  for (const v of values) {
    const pct = Math.max(2, (v.value / max) * 100); // 最小 2% 避免 0 不可见
    const row = document.createElement('div');
    row.className = 'timing-bar';
    row.innerHTML = `
      <span class="timing-bar-label">${v.label}</span>
      <div class="timing-bar-track"><div class="timing-bar-fill" style="width:${pct}%"></div></div>
      <span class="timing-bar-value">${v.value}ms</span>
    `;
    container.appendChild(row);
  }

  // processingTimeMs 在响应里是字符串数字（"8"），要 parseInt
  const totalRaw = data.processingTimeMs ?? data.processing_time_ms;
  let total = parseInt(totalRaw, 10) || 0;
  if (!total) total = values.reduce((sum, v) => sum + v.value, 0);
  $('totalTime').textContent = total + 'ms';
}

// ===== Health Details =====
async function getVocabStatus() {
  await adminRequest('GET', '/health/ready');
}

async function adminRequest(method, path, body) {
  const baseUrl = $('baseUrl').value.replace(/\/$/, '');
  const token = $('token').value.trim();
  const out = $('adminOutput');

  out.textContent = `${method} ${path}\n请求中...`;
  try {
    const r = await fetch(`${baseUrl}${path}`, {
      method,
      headers: {
        'Content-Type': 'application/json',
        ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
      },
      body: body ? JSON.stringify(body) : undefined,
    });
    const text = await r.text();
    let pretty = text;
    try { pretty = JSON.stringify(JSON.parse(text), null, 2); } catch {}
    out.textContent = `HTTP ${r.status} ${r.statusText}\n\n${pretty}`;
  } catch (err) {
    out.textContent = `请求失败: ${err.message}`;
  }
}

// ===== Error =====
function showError(httpCode, data) {
  $('resultCard').classList.remove('hidden');
  $('resultCard').scrollIntoView({ behavior: 'smooth', block: 'start' });

  const statusText = httpCode === 0 ? 'NETWORK_ERROR'
    : httpCode === 401 ? 'UNAUTHORIZED'
    : httpCode === 403 ? 'FORBIDDEN'
    : httpCode === 422 ? 'VALIDATION_ERROR'
    : httpCode >= 500 ? 'SERVER_ERROR'
    : 'ERROR';

  $('resultStatus').className = 'status-pill error';
  $('resultStatus').textContent = statusText;
  $('resultProvider').textContent = '';
  $('resultConfidence').textContent = '';
  $('resultDuration').textContent = '';

  $('rawText').textContent = '—';
  $('enhancedText').innerHTML = `<div style="color: var(--color-error)">${escapeHtml(data.error_message || data.message || JSON.stringify(data))}</div>`;

  $('changeCount').textContent = '0';
  $('changesList').innerHTML = '';
  $('timingBars').innerHTML = '';
  $('totalTime').textContent = '—';

  $('rawJson').textContent = JSON.stringify({ http_code: httpCode, body: data }, null, 2);
}

// ===== 工具 =====
function escapeHtml(s) {
  if (s == null) return '';
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function formatSize(bytes) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`;
}
