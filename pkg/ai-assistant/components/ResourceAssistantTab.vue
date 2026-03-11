<template>
  <div class="resource-assistant-tab">
    <div class="suggested-queries">
      <p class="text-muted">Ask the AI assistant about this {{ resourceKind }}:</p>
      <button
        v-for="suggestion in suggestions"
        :key="suggestion"
        class="btn btn-sm role-secondary"
        :disabled="streaming"
        @click="askQuestion(suggestion)"
      >
        {{ suggestion }}
      </button>
    </div>

    <div class="custom-query">
      <textarea
        v-model="customQuestion"
        :placeholder="`Ask anything about this ${resourceKind}...`"
        rows="2"
        :disabled="streaming"
        @keydown.enter.exact.prevent="askQuestion(customQuestion)"
      />
      <button
        v-if="!streaming"
        class="btn role-primary btn-sm"
        :disabled="!customQuestion.trim()"
        @click="askQuestion(customQuestion)"
      >
        Ask
      </button>
      <button
        v-else
        class="btn role-secondary btn-sm"
        @click="cancelStream"
      >
        Stop
      </button>
    </div>

    <div v-if="streaming && !streamState.text" class="loading">
      <span class="loading-indicator">Investigating...</span>
    </div>

    <div v-if="streamState.toolCalls.length || streamState.text" class="answer">
      <div v-if="streamState.toolCalls.length" class="tool-calls">
        <details v-for="tc in streamState.toolCalls" :key="tc.toolCallId">
          <summary>
            &#9881; {{ tc.toolName }}
            <span v-if="tc.status === 'running'"> (running...)</span>
            <span v-if="tc.status === 'error'" class="text-error"> (error)</span>
          </summary>
          <pre v-if="tc.output">{{ formatOutput(tc.output) }}</pre>
          <p v-if="tc.error" class="text-error">{{ tc.error }}</p>
        </details>
      </div>
      <!-- eslint-disable-next-line vue/no-v-html -->
      <div class="answer__text" v-html="renderMarkdown(streamState.text)" />
    </div>

    <div v-if="streamState.error" class="banner bg-error mt-10">
      {{ streamState.error }}
    </div>
  </div>
</template>

<script lang="ts">
import { defineComponent, ref, computed, reactive } from 'vue';
import { useRancherContext } from '../composables/useRancherContext';
import { useAssistantApi } from '../composables/useAssistantApi';

export default defineComponent({
  name: 'ResourceAssistantTab',

  props: {
    resource: {
      type:    Object,
      default: null,
    },
  },

  setup(props) {
    const { clusterId, chatContext } = useRancherContext();
    const { state: streamState, send, abort } = useAssistantApi(clusterId.value);

    const customQuestion = ref('');

    const streaming = computed(() => streamState.value.isStreaming);

    const resourceKind = computed(() => {
      if (props.resource?.kind) {
        return props.resource.kind.toLowerCase();
      }

      return 'resource';
    });

    const resourceName = computed(() => props.resource?.metadata?.name || 'this resource');
    const resourceNamespace = computed(() => props.resource?.metadata?.namespace || '');

    const suggestions = computed(() => {
      const kind = resourceKind.value;
      const name = resourceName.value;

      if (kind === 'pod') {
        return [
          `Why is ${ name } in its current state?`,
          `Show me recent events for ${ name }`,
          `Get the last 50 log lines from ${ name }`,
          `What resources is ${ name } consuming?`,
        ];
      }

      return [
        `What is the status of ${ name }?`,
        `Are there any issues with ${ name }?`,
        `Show me recent events for ${ name }`,
        `What metrics are available for ${ name }?`,
      ];
    });

    async function askQuestion(question: string) {
      if (!question.trim() || streaming.value) {
        return;
      }

      const ctx = {
        ...chatContext.value,
        namespace:    resourceNamespace.value || chatContext.value.namespace,
        resourceType: props.resource?.type || chatContext.value.resourceType,
        resourceName: resourceName.value,
      };

      customQuestion.value = '';
      await send(question.trim(), ctx);
    }

    function cancelStream() {
      abort();
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

    function formatOutput(output: any): string {
      if (typeof output === 'string') {
        return output.length > 2000 ? `${ output.slice(0, 2000) }...(truncated)` : output;
      }

      try {
        const s = JSON.stringify(output, null, 2);

        return s.length > 2000 ? `${ s.slice(0, 2000) }...(truncated)` : s;
      } catch {
        return String(output);
      }
    }

    return {
      customQuestion,
      streaming,
      streamState,
      resourceKind,
      suggestions,
      askQuestion,
      cancelStream,
      renderMarkdown,
      formatOutput,
    };
  },
});
</script>

<style lang="scss" scoped>
.resource-assistant-tab {
  padding: 16px 0;
}

.suggested-queries {
  margin-bottom: 16px;

  button {
    margin: 4px 4px 4px 0;
  }
}

.custom-query {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;

  textarea {
    flex: 1;
    resize: none;
    padding: 8px 12px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--input-bg);
    color: var(--input-text);
    font-size: 14px;
  }

  button {
    align-self: flex-end;
  }
}

.answer {
  padding: 16px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--body-bg);

  &__text {
    line-height: 1.6;

    pre {
      overflow-x: auto;
      padding: 8px;
      background: var(--input-bg);
      border-radius: 4px;
    }
  }
}

.tool-calls {
  margin-bottom: 12px;

  details {
    margin-bottom: 4px;
    font-size: 13px;
    color: var(--muted);

    pre {
      max-height: 150px;
      overflow-y: auto;
      font-size: 12px;
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
