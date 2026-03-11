<template>
  <div class="cluster-health-card">
    <div v-if="!streamState.text && !streaming" class="card-prompt">
      <p class="text-muted">Get an AI-powered health summary of your cluster.</p>
      <button class="btn btn-sm role-primary" @click="runHealthCheck">
        Run Health Check
      </button>
    </div>

    <div v-if="streaming && !streamState.text" class="card-loading">
      <span class="loading-indicator">Analyzing cluster...</span>
    </div>

    <div v-if="streamState.text" class="card-result">
      <!-- eslint-disable-next-line vue/no-v-html -->
      <div class="card-result__text" v-html="renderMarkdown(streamState.text)" />
      <button class="btn btn-sm role-secondary mt-10" :disabled="streaming" @click="runHealthCheck">
        Refresh
      </button>
    </div>

    <div v-if="streamState.error" class="banner bg-error mt-10">
      {{ streamState.error }}
    </div>
  </div>
</template>

<script lang="ts">
import { defineComponent, computed } from 'vue';
import { useRancherContext } from '../composables/useRancherContext';
import { useAssistantApi } from '../composables/useAssistantApi';

export default defineComponent({
  name: 'ClusterHealthCard',

  setup() {
    const { clusterId, chatContext } = useRancherContext();
    const { state: streamState, send } = useAssistantApi(clusterId.value);

    const streaming = computed(() => streamState.value.isStreaming);

    async function runHealthCheck() {
      await send(
        'Give me a brief cluster health summary: check for pods not running, recent warning events, node status, and resource pressure. Keep it short.',
        chatContext.value,
      );
    }

    function renderMarkdown(text: string): string {
      if (!text) {
        return '';
      }

      return text
        .replace(/```(\w*)\n([\s\S]*?)```/g, '<pre><code>$2</code></pre>')
        .replace(/`([^`]+)`/g, '<code>$1</code>')
        .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
        .replace(/\n/g, '<br>');
    }

    return {
      streaming,
      streamState,
      runHealthCheck,
      renderMarkdown,
    };
  },
});
</script>

<style lang="scss" scoped>
.cluster-health-card {
  padding: 8px;

  .card-prompt {
    text-align: center;
    padding: 16px;
  }

  .card-result__text {
    line-height: 1.5;
    font-size: 14px;

    pre {
      overflow-x: auto;
      padding: 8px;
      background: var(--input-bg);
      border-radius: 4px;
    }
  }
}

.loading-indicator {
  color: var(--muted);
  animation: pulse 1.5s ease-in-out infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 0.4; }
  50% { opacity: 1; }
}
</style>
