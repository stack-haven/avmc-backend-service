/* evie/tool demo · app.js (Vue 3 root)
 *
 * 顶层 Composition API：管理 baseUrl / token / 音频数据 / 结果。
 * 通过 props/emits 与各组件通信。
 */

const { createApp, ref, computed, onMounted } = Vue;

const App = {
  components: {
    AppHeader,
    ConfigPanel,
    AsrPanel,
    TextEnhancePanel,
    ResultViewer,
    AdminPanel,
    AppFooter,
  },
  setup() {
    // ===== 配置 =====
    const baseUrl = ref(localStorage.getItem('evie.baseUrl') || 'http://localhost:8110');
    const token = ref(localStorage.getItem('evie.token') || '');
    const enableEnhance = ref(true);

    // ===== 状态 =====
    const status = ref({ level: 'unknown', text: '未连接' });
    const audioBase64 = ref(null);
    const audioEncoding = ref(null);
    const audioMeta = ref(null);
    const result = ref(null);
    const resultKind = ref('enhance'); // 'asr' | 'enhance'

    // ===== 持久化 =====
    function persist() {
      localStorage.setItem('evie.baseUrl', baseUrl.value);
      localStorage.setItem('evie.token', token.value);
    }
    watch([baseUrl, token], persist);

    // ===== 健康检查 =====
    async function pingHealth() {
      status.value = { level: 'checking', text: '检查中...' };
      try {
        const r = await fetch(baseUrl.value + '/health/ready');
        if (!r.ok) {
          status.value = { level: 'unhealthy', text: `HTTP ${r.status}` };
          return;
        }
        const data = await r.json();
        if (data.status === 'ok') {
          status.value = {
            level: 'healthy',
            text: `就绪 · ${JSON.stringify(data.details || {})}`,
          };
        } else {
          status.value = {
            level: 'unhealthy',
            text: `不可用 · ${JSON.stringify(data.details || {})}`,
          };
        }
      } catch (err) {
        status.value = { level: 'unhealthy', text: `连接失败: ${err.message}` };
      }
    }

    // ===== 音频处理 =====
    function setAudio({ audioBase64: b64, audioEncoding: enc, audioMeta: meta }) {
      audioBase64.value = b64;
      audioEncoding.value = enc;
      audioMeta.value = meta;
    }

    // ===== 识别请求 =====
    async function onRecognize({ audioBase64: b64, audioEncoding: enc, sessionId, enableEnhancement }) {
      status.value = { level: 'checking', text: 'ASR 识别中...' };
      const body = {
        format: { encoding: enc, sampleRate: 16000, bitDepth: 16, channels: 1 },
        audioData: b64,
        sessionId,
        enableEnhancement,
      };
      const headers = { 'Content-Type': 'application/json' };
      if (token.value) headers['Authorization'] = `Bearer ${token.value}`;

      try {
        const r = await fetch(baseUrl.value + '/evie/tool/v1/asr:recognize', {
          method: 'POST', headers, body: JSON.stringify(body),
        });
        const data = await r.json();
        result.value = data;
        resultKind.value = 'asr';
        status.value = r.ok
          ? { level: 'healthy', text: '识别完成' }
          : { level: 'unhealthy', text: `HTTP ${r.status}` };
      } catch (err) {
        result.value = { error_message: err.message };
        resultKind.value = 'asr';
        status.value = { level: 'unhealthy', text: '请求失败: ' + err.message };
      }
    }

    // ===== 文本增强 =====
    async function onEnhance({ text }) {
      const headers = { 'Content-Type': 'application/json' };
      if (token.value) headers['Authorization'] = `Bearer ${token.value}`;

      try {
        const r = await fetch(baseUrl.value + '/evie/tool/v1/enhance', {
          method: 'POST', headers, body: JSON.stringify({ text }),
        });
        const data = await r.json();
        result.value = data;
        resultKind.value = 'enhance';
      } catch (err) {
        result.value = { error_message: err.message };
        resultKind.value = 'enhance';
      }
    }

    onMounted(() => {
      pingHealth();
    });

    return {
      baseUrl, token, enableEnhance,
      status,
      audioBase64, audioEncoding, audioMeta,
      result, resultKind,
      pingHealth,
      setAudio,
      onRecognize,
      onEnhance,
    };
  },
  template: `
    <app-header :status="status" @check-health="pingHealth"></app-header>
    <config-panel
      v-model:base-url="baseUrl"
      v-model:token="token"
      v-model:enable-enhance="enableEnhance"
      @check-health="pingHealth"></config-panel>

    <div class="workspace">
      <asr-panel
        :audio-base64="audioBase64"
        :audio-encoding="audioEncoding"
        :audio-meta="audioMeta"
        :enable-enhance="enableEnhance"
        :base-url="baseUrl"
        :token="token"
        @recognize-input="setAudio"
        @recognize="onRecognize"></asr-panel>

      <text-enhance-panel
        :base-url="baseUrl"
        :token="token"
        @enhance="onEnhance"></text-enhance-panel>
    </div>

    <result-viewer v-if="result" :result="result" :kind="resultKind"></result-viewer>

    <admin-panel :base-url="baseUrl" :token="token"></admin-panel>

    <app-footer></app-footer>
  `,
};

// Vue 3 + 全局组件（components.js 里直接 var 定义）
const app = createApp(App);
app.mount('#app');
