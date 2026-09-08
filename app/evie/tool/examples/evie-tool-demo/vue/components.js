/* evie/tool demo · components.js
 * Vue 3 单文件组件风格的 template + setup 组件（CDN 引入版）。
 * 设计：响应式 state + props/emits，与 Vben Admin 风格一致。
 */

// ===== AppHeader =====
const AppHeader = {
  props: ['status'],
  emits: ['check-health'],
  setup(props, { emit }) {
    return { emit };
  },
  template: `
    <header class="app-header">
      <div class="logo">
        <span class="logo-icon">🎙️</span>
        <div>
          <h1>evie/tool <span class="vue-badge">Vue 3</span></h1>
          <p class="subtitle">ASR + Text Enhancement HTTP Demo</p>
        </div>
      </div>
      <div class="status-badge" :class="status.level">
        <span class="status-dot"></span>
        <span>{{ status.text }}</span>
      </div>
    </header>
  `,
};

// ===== ConfigPanel =====
const ConfigPanel = {
  props: ['baseUrl', 'token', 'enableEnhance'],
  emits: ['update:baseUrl', 'update:token', 'update:enableEnhance', 'check-health'],
  template: `
    <section class="config-card">
      <h2>配置</h2>
      <div class="form-grid">
        <label>
          <span>Base URL</span>
          <input type="text" :value="baseUrl"
            @input="$emit('update:baseUrl', $event.target.value)"
            placeholder="http://localhost:8110">
        </label>
        <label>
          <span>Bearer Token</span>
          <input type="text" :value="token"
            @input="$emit('update:token', $event.target.value)"
            placeholder="从 Redis 共享 auth 获取的 token">
        </label>
        <label class="checkbox-label">
          <input type="checkbox"
            :checked="enableEnhance"
            @change="$emit('update:enableEnhance', $event.target.checked)">
          <span>启用文本增强</span>
        </label>
      </div>
      <div class="config-actions">
        <button class="btn btn-secondary" @click="$emit('check-health')">检查健康</button>
      </div>
    </section>
  `,
};

// ===== AsrPanel =====
const AsrPanel = {
  props: ['audioBase64', 'audioEncoding', 'audioMeta', 'enableEnhance', 'baseUrl', 'token'],
  emits: ['recognize', 'reset'],
  data() {
    return {
      sourceTab: 'file',
      audioUrl: '',
      loadingUrl: false,
      recognizeBtn: { loading: false, label: '▶ 识别并增强' },
      // 录音
      recording: false,
      recordTime: '00:00',
      recordPreview: null,
      recordStart: 0,
      recordTimer: null,
      mediaRecorder: null,
    };
  },
  computed: {
    canRecognize() {
      return this.audioBase64 && !this.recognizeBtn.loading;
    },
  },
  methods: {
    async handleFileSelect(e) {
      const file = e.target.files[0];
      if (!file) return;
      await this.loadFile(file);
    },
    async handleDrop(e) {
      e.preventDefault();
      const file = e.dataTransfer.files[0];
      if (file) await this.loadFile(file);
    },
    async loadFile(file) {
      const enc = this.detectEncoding(file.name);
      const reader = new FileReader();
      reader.onload = () => {
        const base64 = reader.result.split(',')[1];
        this.$emit('recognize-input', {
          audioBase64: base64,
          audioEncoding: enc,
          audioMeta: { name: file.name, size: file.size, type: file.type },
        });
      };
      reader.readAsDataURL(file);
    },
    async loadUrl() {
      if (!this.audioUrl.trim()) return;
      this.loadingUrl = true;
      try {
        const r = await fetch(this.audioUrl);
        const blob = await r.blob();
        const enc = this.detectEncoding(this.audioUrl);
        const reader = new FileReader();
        reader.onload = () => {
          const base64 = reader.result.split(',')[1];
          this.$emit('recognize-input', {
            audioBase64: base64,
            audioEncoding: enc,
            audioMeta: {
              name: this.audioUrl.split('/').pop(),
              size: blob.size,
              type: blob.type,
            },
          });
        };
        reader.readAsDataURL(blob);
      } catch (err) {
        alert('加载失败: ' + err.message);
      } finally {
        this.loadingUrl = false;
      }
    },
    detectEncoding(name) {
      const n = name.toLowerCase();
      if (n.endsWith('.mp3')) return 'mp3';
      if (n.endsWith('.wav')) return 'wav';
      if (n.endsWith('.pcm')) return 'pcm';
      if (n.endsWith('.opus')) return 'opus';
      return 'wav';
    },
    async toggleRecord() {
      if (this.recording) {
        this.stopRecord();
      } else {
        await this.startRecord();
      }
    },
    async startRecord() {
      try {
        const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
        const mr = new MediaRecorder(stream);
        this.mediaRecorder = mr;
        this.recording = true;
        this.recordStart = Date.now();

        const chunks = [];
        mr.ondataavailable = (e) => chunks.push(e.data);
        mr.onstop = () => {
          const blob = new Blob(chunks, { type: 'audio/webm' });
          this.recordPreview = URL.createObjectURL(blob);
          const reader = new FileReader();
          reader.onload = () => {
            const base64 = reader.result.split(',')[1];
            this.$emit('recognize-input', {
              audioBase64: base64,
              audioEncoding: 'wav',
              audioMeta: {
                name: `recording-${Date.now()}.webm`,
                size: blob.size,
                type: 'audio/webm',
              },
            });
          };
          reader.readAsDataURL(blob);
          stream.getTracks().forEach(t => t.stop());
        };

        mr.start();
        this.recordTimer = setInterval(() => {
          const sec = Math.floor((Date.now() - this.recordStart) / 1000);
          this.recordTime = `${String(Math.floor(sec / 60)).padStart(2, '0')}:${String(sec % 60).padStart(2, '0')}`;
        }, 200);
      } catch (err) {
        alert('无法访问麦克风: ' + err.message);
      }
    },
    stopRecord() {
      if (this.mediaRecorder && this.mediaRecorder.state !== 'inactive') {
        this.mediaRecorder.stop();
      }
      this.recording = false;
      clearInterval(this.recordTimer);
    },
    formatSize(bytes) {
      if (bytes < 1024) return bytes + ' B';
      if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
      return (bytes / 1024 / 1024).toFixed(2) + ' MB';
    },
    async doRecognize() {
      if (!this.canRecognize) return;
      this.recognizeBtn.loading = true;
      this.recognizeBtn.label = '识别中...';
      this.$emit('recognize', {
        audioBase64: this.audioBase64,
        audioEncoding: this.audioEncoding,
        sessionId: `vue-demo-${Date.now()}`,
        enableEnhancement: this.enableEnhance,
      });
      // 父组件会异步完成后调用 finishRecognize
    },
    finishRecognize() {
      this.recognizeBtn.loading = false;
      this.recognizeBtn.label = '▶ 识别并增强';
    },
  },
  beforeUnmount() {
    clearInterval(this.recordTimer);
  },
  template: `
    <div class="card">
      <header class="card-header">
        <h3>① ASR 识别 + 增强</h3>
        <span class="hint">POST /evie/tool/v1/asr:recognize</span>
      </header>
      <div class="card-body">
        <div class="audio-source">
          <label class="source-tab" :class="{active: sourceTab === 'file'}" @click="sourceTab = 'file'">📁 文件</label>
          <label class="source-tab" :class="{active: sourceTab === 'url'}" @click="sourceTab = 'url'">🔗 URL</label>
          <label class="source-tab" :class="{active: sourceTab === 'record'}" @click="sourceTab = 'record'">🎤 录制</label>
        </div>

        <!-- 文件模式 -->
        <div v-show="sourceTab === 'file'" class="source-panel">
          <div class="drop-zone"
            @dragover.prevent @drop="handleDrop"
            @click="$refs.fileInput.click()">
            <p class="drop-zone-icon">⬇</p>
            <p>拖拽音频文件到此处</p>
            <p class="hint">或点击选择文件</p>
          </div>
          <input ref="fileInput" type="file" accept="audio/*" hidden @change="handleFileSelect">
        </div>

        <!-- URL 模式 -->
        <div v-show="sourceTab === 'url'" class="source-panel">
          <input type="text" v-model="audioUrl" placeholder="https://example.com/audio.mp3">
          <button class="btn btn-secondary" @click="loadUrl" :disabled="loadingUrl">
            {{ loadingUrl ? '加载中...' : '加载' }}
          </button>
        </div>

        <!-- 录音模式 -->
        <div v-show="sourceTab === 'record'" class="source-panel">
          <div class="record-controls">
            <button class="btn btn-record" :class="{recording}" @click="toggleRecord">
              <span class="rec-dot"></span>
              <span>{{ recording ? '停止录制' : '开始录制' }}</span>
            </button>
            <span class="record-time">{{ recordTime }}</span>
          </div>
          <audio v-if="recordPreview" :src="recordPreview" controls></audio>
        </div>

        <!-- 文件信息 -->
        <div v-if="audioMeta" class="file-info">
          <span class="file-name">{{ audioMeta.name }}</span>
          <span class="file-meta">{{ formatSize(audioMeta.size) }} · {{ audioMeta.type }} · {{ audioEncoding }}</span>
        </div>

        <button class="btn btn-primary btn-block" :disabled="!canRecognize" @click="doRecognize">
          {{ recognizeBtn.label }}
        </button>
      </div>
    </div>
  `,
};

// ===== TextEnhancePanel =====
const TextEnhancePanel = {
  props: ['baseUrl', 'token'],
  emits: ['enhance'],
  data() {
    return {
      text: '金种子 是 我们的 主要 产品 之一。',
      loading: false,
    };
  },
  template: `
    <div class="card">
      <header class="card-header">
        <h3>② 纯文本增强</h3>
        <span class="hint">POST /evie/tool/v1/enhance</span>
      </header>
      <div class="card-body">
        <label class="textarea-label">
          <span>输入文本</span>
          <textarea v-model="text" rows="6"></textarea>
        </label>
        <button class="btn btn-primary btn-block"
          :disabled="loading || !text.trim()"
          @click="$emit('enhance', { text })">
          {{ loading ? '增强中...' : '▶ 增强' }}
        </button>
      </div>
    </div>
  `,
};

// ===== ResultViewer =====
const ResultViewer = {
  props: ['result', 'kind'],
  computed: {
    statusClass() {
      const map = { 1: 'success', 2: 'degraded' };
      return map[this.result.status] || 'unknown';
    },
    statusText() {
      const map = { 1: 'SUCCESS', 2: 'DEGRADED' };
      return map[this.result.status] || 'UNKNOWN';
    },
    rawText() {
      return this.result.raw_text || this.result.original_text || '—';
    },
    enhancedText() {
      return this.result.enhanced_text || '—';
    },
    changes() {
      return this.result.changes || [];
    },
    timingFields() {
      return [
        { key: 'cleaning_time_ms', label: 'cleaning' },
        { key: 'filler_time_ms', label: 'filler' },
        { key: 'vocab_match_time_ms', label: 'vocab match' },
        { key: 'alias_time_ms', label: 'alias' },
        { key: 'deterministic_time_ms', label: 'deterministic' },
        { key: 'pinyin_time_ms', label: 'pinyin' },
        { key: 'fuzzy_time_ms', label: 'fuzzy' },
        { key: 'context_time_ms', label: 'context' },
      ];
    },
    timingValues() {
      const values = this.timingFields.map(f => ({ ...f, value: this.result[f.key] || 0 }));
      const max = Math.max(...values.map(v => v.value), 1);
      return values.map(v => ({ ...v, pct: Math.max(2, (v.value / max) * 100) }));
    },
    totalTime() {
      return this.result.processing_time_ms
        || this.timingValues.reduce((s, v) => s + v.value, 0);
    },
    enhancedHtml() {
      if (!this.result.enhanced_text) return '—';
      if (this.changes.length === 0) return this.escapeHtml(this.result.enhanced_text);
      let html = this.escapeHtml(this.result.enhanced_text);
      const sorted = [...this.changes].sort((a, b) =>
        (b.replacement || '').length - (a.replacement || '').length
      );
      for (const ch of sorted) {
        const to = this.escapeHtml(ch.replacement || '');
        if (!to) continue;
        const idx = html.lastIndexOf(to);
        if (idx >= 0) {
          html = html.slice(0, idx) +
            `<mark class="to">${to}</mark>` +
            html.slice(idx + to.length);
        }
      }
      return html;
    },
  },
  methods: {
    escapeHtml(s) {
      if (s == null) return '';
      return String(s)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
    },
  },
  template: `
    <section class="result-card">
      <header class="card-header">
        <h3>③ 识别 / 增强结果</h3>
        <div class="result-meta">
          <span class="status-pill" :class="statusClass">{{ statusText }}</span>
          <span class="meta-item" v-if="result.provider_name">provider: {{ result.provider_name }}</span>
          <span class="meta-item" v-if="result.confidence != null">conf: {{ result.confidence.toFixed(2) }}</span>
          <span class="meta-item" v-if="result.duration_ms">dur: {{ (result.duration_ms / 1000).toFixed(1) }}s</span>
        </div>
      </header>
      <div class="card-body">
        <div class="diff-section">
          <div class="diff-row">
            <span class="diff-label">原始</span>
            <div class="diff-text">{{ rawText }}</div>
          </div>
          <div class="diff-row">
            <span class="diff-label">增强</span>
            <div class="diff-text enhanced" v-html="enhancedHtml"></div>
          </div>
        </div>

        <div class="changes-section">
          <h4>改动详情 <span class="badge">{{ changes.length }}</span></h4>
          <div v-if="changes.length === 0" class="no-changes">✨ 无改动</div>
          <div v-else class="changes-list">
            <div v-for="(ch, i) in changes" :key="i" class="change-item">
              <span class="change-kind">{{ ch.kind || '—' }}</span>
              <span class="change-from">{{ ch.original || '' }}</span>
              <span class="change-to">{{ ch.replacement || '' }}</span>
              <span class="change-confidence">{{ ch.confidence != null ? ch.confidence.toFixed(2) : '—' }}</span>
            </div>
          </div>
        </div>

        <div class="timing-section">
          <h4>各阶段耗时（ms）</h4>
          <div class="timing-bars">
            <div v-for="t in timingValues" :key="t.key" class="timing-bar">
              <span class="timing-bar-label">{{ t.label }}</span>
              <div class="timing-bar-track">
                <div class="timing-bar-fill" :style="{width: t.pct + '%'}"></div>
              </div>
              <span class="timing-bar-value">{{ t.value }}ms</span>
            </div>
          </div>
          <div class="timing-total">总耗时: <strong>{{ totalTime }}</strong> ms</div>
        </div>

        <details class="raw-response">
          <summary>查看完整响应 JSON</summary>
          <pre>{{ JSON.stringify(result, null, 2) }}</pre>
        </details>
      </div>
    </section>
  `,
};

// ===== AdminPanel =====
const AdminPanel = {
  props: ['baseUrl', 'token'],
  data() {
    return {
      output: '（点击上方按钮查询 /health/ready）',
    };
  },
  methods: {
    async getStatus() {
      const headers = { 'Content-Type': 'application/json' };
      this.output = `GET /health/ready\n请求中...`;
      try {
        const r = await fetch(this.baseUrl + '/health/ready', { headers });
        const text = await r.text();
        let pretty = text;
        try { pretty = JSON.stringify(JSON.parse(text), null, 2); } catch {}
        this.output = `HTTP ${r.status} ${r.statusText}\n\n${pretty}`;
      } catch (err) {
        this.output = '请求失败: ' + err.message;
      }
    },
  },
  template: `
    <section class="admin-card">
      <header class="card-header">
        <h3>④ 服务健康详情</h3>
        <span class="hint">GET /health/ready</span>
      </header>
      <div class="card-body">
        <div class="admin-actions">
          <button class="btn btn-secondary" @click="getStatus">查看 /health/ready 详情</button>
        </div>
        <pre class="admin-output">{{ output }}</pre>
      </div>
    </section>
  `,
};

// ===== AppFooter =====
const AppFooter = {
  template: `
    <footer class="app-footer">
      <p>evie/tool · Go 1.25 · Kratos v2 · Vue 3 Composition API demo</p>
      <p>接口规范详见 <code>README.md</code></p>
    </footer>
  `,
};
