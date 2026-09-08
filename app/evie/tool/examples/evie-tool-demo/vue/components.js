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
  emits: ['update:baseUrl', 'update:token', 'update:enableEnhance', 'check-health', 'load-preset'],
  data() {
    return { tokenVisible: false };
  },
  methods: {
    toggleTokenVis() {
      this.tokenVisible = !this.tokenVisible;
    },
    loadPreset() {
      this.$emit('update:token', '182ed78960304d90a5e286b08a57145d');
      this.tokenVisible = false;
      this.$nextTick(() => this.$refs.tokenInput?.focus());
    },
  },
  template: `
    <section class="config-card">
      <h2>连接配置</h2>
      <div class="form-row">
        <label class="form-item">
          <span class="form-label">Base URL</span>
          <input type="text" :value="baseUrl"
            @input="$emit('update:baseUrl', $event.target.value)"
            placeholder="http://localhost:8110">
        </label>
        <label class="form-item form-item-grow">
          <span class="form-label">
            Bearer Token
            <span class="form-hint">
              <span class="hint-icon">ⓘ</span>
              <span class="hint-text">存于 Redis db=14 <code>oauth2_access_token:*</code>；不是 qua sync_token</span>
            </span>
          </span>
          <div class="input-with-action">
            <input ref="tokenInput" :type="tokenVisible ? 'text' : 'password'"
              :value="token"
              @input="$emit('update:token', $event.target.value)"
              placeholder="粘贴 OAuth access_token"
              autocomplete="off">
            <button class="btn-icon" @click="toggleTokenVis" :title="tokenVisible ? '隐藏' : '显示'">
              {{ tokenVisible ? '🙈' : '👁' }}
            </button>
          </div>
        </label>
      </div>
      <div class="form-row form-row-options">
        <label class="checkbox-label">
          <input type="checkbox"
            :checked="enableEnhance"
            @change="$emit('update:enableEnhance', $event.target.checked)">
          <span>启用文本增强（8 层 processor，推荐）</span>
        </label>
        <div class="preset-actions">
          <button class="btn-link" @click="loadPreset">⚡ 加载演示凭据</button>
          <button class="btn btn-secondary" @click="$emit('check-health')">检查健康</button>
        </div>
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
      return this.result.rawText || this.result.raw_text
          || this.result.originalText || this.result.original_text || '—';
    },
    enhancedText() {
      return this.result.enhancedText || this.result.enhanced_text || '—';
    },
    changes() {
      return this.result.changes || [];
    },
    groupedChanges() {
      // 按 source 分组（fuzzy / alias / deterministic / clean 等）
      const groups = new Map();
      // labelMap 基于 evie/tool processor 真实 source 名
      const labelMap = {
        normalize: '① 清洗（全/半角、空白、标点）',
        disfluency: '② 填充词删除（啊/呃/那个）',
        alias: '③ 别名（产品功能名 / 业务术语）',
        deterministic: '④ 确定性（量词 / 专用词）',
        fuzzy_vocab: '⑤ 模糊匹配（Hamming / Pinyin / lock_alias）',
        pinyin: '拼音归一（兜底）',
        ctxproc: '上下文处理',
        cleaning: '文本清洗',
      };
      for (const ch of this.changes) {
        const from = ch.from ?? ch.original ?? '';
        const to = ch.to ?? ch.replacement ?? '';
        if (from === to && !to) continue;
        const key = ch.source || ch.type || ch.kind || 'other';
        if (!groups.has(key)) groups.set(key, { key, label: labelMap[key] || key, items: [] });
        groups.get(key).items.push({
          from, to,
          type: ch.type ?? ch.kind ?? '—',
          conf: ch.confidence,
        });
      }
      return Array.from(groups.values());
    },
    timingFields() {
      return [
        { key: 'cleaningTimeMs', label: 'cleaning' },
        { key: 'fillerTimeMs', label: 'filler' },
        { key: 'vocabMatchTimeMs', label: 'vocab match' },
        { key: 'aliasTimeMs', label: 'alias' },
        { key: 'deterministicTimeMs', label: 'deterministic' },
        { key: 'pinyinTimeMs', label: 'pinyin' },
        { key: 'fuzzyTimeMs', label: 'fuzzy' },
        { key: 'contextTimeMs', label: 'context' },
      ];
    },
    timingValues() {
      const values = this.timingFields.map(f => {
        // 响应里 *TimeMs 是字符串数字
        const v = this.result[f.key];
        return { ...f, value: parseInt(v, 10) || 0 };
      });
      const max = Math.max(...values.map(v => v.value), 1);
      return values.map(v => ({ ...v, pct: Math.max(2, (v.value / max) * 100) }));
    },
    totalTime() {
      const raw = this.result.processingTimeMs ?? this.result.processing_time_ms;
      let total = parseInt(raw, 10) || 0;
      if (!total) total = this.timingValues.reduce((s, v) => s + v.value, 0);
      return total + 'ms';
    },
    enhancedHtml() {
      if (!this.enhancedText || this.enhancedText === '—') return '—';
      if (!this.changes || this.changes.length === 0) return this.escapeHtml(this.enhancedText);
      // 只高亮 from != to 的真实改动
      const real = this.changes
        .map(ch => ({
          from: ch.from ?? ch.original ?? '',
          to: ch.to ?? ch.replacement ?? '',
        }))
        .filter(c => c.from !== c.to && c.to);
      if (real.length === 0) return this.escapeHtml(this.enhancedText);
      let html = this.escapeHtml(this.enhancedText);
      const sorted = [...real].sort((a, b) => b.to.length - a.to.length);
      for (const c of sorted) {
        const to = this.escapeHtml(c.to);
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
          <span class="meta-item" v-if="result.providerName || result.provider_name">provider: {{ result.providerName || result.provider_name }}</span>
          <span class="meta-item" v-if="result.confidence != null">conf: {{ (+result.confidence).toFixed(2) }}</span>
          <span class="meta-item" v-if="result.durationMs || result.duration_ms">dur: {{ ((result.durationMs || result.duration_ms) / 1000).toFixed(1) }}s</span>
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
          <div v-if="groupedChanges.length === 0" class="no-changes">✨ 无改动</div>
          <div v-else class="changes-groups">
            <div v-for="g in groupedChanges" :key="g.key" class="change-group">
              <div class="change-group-title">{{ g.label }} <span class="badge">{{ g.items.length }}</span></div>
              <div class="changes-list">
                <div v-for="(ch, i) in g.items" :key="i" class="change-item">
                  <span class="change-kind">{{ ch.type }}</span>
                  <span class="change-from">{{ ch.from }}</span>
                  <span class="change-to">{{ ch.to }}</span>
                  <span class="change-confidence">{{ ch.conf != null ? (+ch.conf).toFixed(2) : '—' }}</span>
                </div>
              </div>
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
