<template>
  <div class="ai-assistant-page">
    <header class="ai-assistant-page__header">
      <h1>AI Assistant</h1>
      <p class="text-muted">
        Investigate issues in your cluster using natural language.
        <span v-if="contextSummary" class="context-badge">{{ contextSummary }}</span>
      </p>
    </header>

    <div class="ai-assistant-page__chat">
      <div ref="messagesContainer" class="chat-messages">
        <div v-if="messages.length === 0" class="chat-empty">
          <h2>What would you like to investigate?</h2>
          <div class="suggested-queries">
            <button
              v-for="suggestion in suggestions"
              :key="suggestion"
              class="btn btn-sm role-secondary"
              @click="sendMessage(suggestion)"
            >
              {{ suggestion }}
            </button>
          </div>
        </div>

        <div
          v-for="msg in messages"
          :key="msg.id"
          class="chat-message"
          :class="`chat-message--${msg.role}`"
        >
          <div class="chat-message__content">
            <div v-if="msg.role === 'assistant' && msg.toolCalls?.length" class="tool-calls">
              <details v-for="tc in msg.toolCalls" :key="tc.toolCallId" class="tool-call">
                <summary>
                  <span class="tool-call__icon">&#9881;</span>
                  {{ tc.toolName }}
                  <span v-if="tc.status === 'running'" class="tool-call__status"> (running...)</span>
                  <span v-if="tc.status === 'error'" class="text-error"> (error)</span>
                </summary>
                <div class="tool-call__detail">
                  <div v-if="tc.input" class="tool-call__section">
                    <strong>Input:</strong>
                    <pre>{{ formatJSON(tc.input) }}</pre>
                  </div>
                  <div v-if="tc.output" class="tool-call__section">
                    <strong>Output:</strong>
                    <pre>{{ formatOutput(tc.output) }}</pre>
                  </div>
                  <div v-if="tc.error" class="tool-call__section text-error">
                    <strong>Error:</strong> {{ tc.error }}
                  </div>
                </div>
              </details>
            </div>
            <!-- eslint-disable-next-line vue/no-v-html -->
            <div v-html="renderMarkdown(msg.content)" />
            <span v-if="msg.role === 'assistant' && !msg.content && streaming" class="loading-indicator">
              Thinking...
            </span>
          </div>
        </div>
      </div>

      <div class="chat-input">
        <textarea
          v-model="inputText"
          placeholder="Ask about pods, metrics, traces, logs..."
          rows="2"
          :disabled="streaming"
          @keydown.enter.exact.prevent="sendMessage(inputText)"
        />
        <button
          v-if="!streaming"
          class="btn role-primary"
          :disabled="!inputText.trim()"
          @click="sendMessage(inputText)"
        >
          Send
        </button>
        <button
          v-else
          class="btn role-secondary"
          @click="cancelStream"
        >
          Stop
        </button>
      </div>

      <div v-if="streamError" class="chat-error banner bg-error">
        {{ streamError }}
      </div>
    </div>
  </div>
</template>

<script lang="ts">
import { defineComponent, ref, computed, nextTick, watch } from 'vue';
import { useStore } from 'vuex';
import { useRancherContext } from '../composables/useRancherContext';
import { useAssistantApi } from '../composables/useAssistantApi';

export default defineComponent({
  name: 'AiAssistantPage',

  setup() {
    const store = useStore();
    const { clusterId, contextSummary, chatContext } = useRancherContext();
    const { state: streamState, send, abort } = useAssistantApi(clusterId.value);

    const inputText = ref('');
    const messagesContainer = ref<HTMLElement | null>(null);

    const messages = computed(() => store.getters['ai-assistant-chat/allMessages']);
    const streaming = computed(() => streamState.value.isStreaming);
    const streamError = computed(() => streamState.value.error);

    const suggestions = [
      'Are there any pods in a crash loop?',
      'Show me the top 5 CPU-consuming workloads',
      'Are there any recent warning events?',
      'What is the overall cluster health?',
    ];

    // Watch stream state and update the assistant message in the store
    watch(
      () => ({ text: streamState.value.text, toolCalls: [...streamState.value.toolCalls] }),
      (val) => {
        store.dispatch('ai-assistant-chat/updateAssistant', {
          content:   val.text,
          toolCalls: val.toolCalls,
        });
      },
      { deep: true },
    );

    // Auto-scroll on message changes
    watch(messages, async() => {
      await nextTick();
      if (messagesContainer.value) {
        messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight;
      }
    }, { deep: true });

    async function sendMessage(text: string) {
      if (!text.trim() || streaming.value) {
        return;
      }

      const msg = text.trim();

      inputText.value = '';

      // Add user message and assistant placeholder to the store
      store.dispatch('ai-assistant-chat/addUserMessage', msg);
      store.dispatch('ai-assistant-chat/addAssistantPlaceholder');

      // Start SSE stream
      await send(msg, chatContext.value);
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

    function formatJSON(obj: any): string {
      try {
        return JSON.stringify(obj, null, 2);
      } catch {
        return String(obj);
      }
    }

    function formatOutput(output: any): string {
      if (typeof output === 'string') {
        // Truncate long outputs
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
      inputText,
      messagesContainer,
      messages,
      streaming,
      streamError,
      suggestions,
      contextSummary,
      sendMessage,
      cancelStream,
      renderMarkdown,
      formatJSON,
      formatOutput,
    };
  },
});
</script>

<style lang="scss" scoped>
.ai-assistant-page {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 120px);
  padding: 20px;

  &__header {
    margin-bottom: 16px;

    .context-badge {
      display: inline-block;
      padding: 2px 8px;
      margin-left: 8px;
      font-size: 12px;
      background: var(--primary);
      color: var(--primary-text);
      border-radius: 4px;
    }
  }

  &__chat {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
  }
}

.chat-messages {
  flex: 1;
  overflow-y: auto;
  padding: 16px 0;
}

.chat-empty {
  text-align: center;
  padding: 40px;

  h2 {
    margin-bottom: 20px;
    color: var(--body-text);
  }

  .suggested-queries {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    justify-content: center;
  }
}

.chat-message {
  margin-bottom: 16px;
  display: flex;

  &--user {
    justify-content: flex-end;

    .chat-message__content {
      background: var(--primary);
      color: var(--primary-text);
      border-radius: 12px 12px 0 12px;
    }
  }

  &--assistant {
    justify-content: flex-start;

    .chat-message__content {
      background: var(--body-bg);
      border: 1px solid var(--border);
      border-radius: 12px 12px 12px 0;
    }
  }

  &__content {
    max-width: 80%;
    padding: 12px 16px;
    line-height: 1.5;
    word-break: break-word;

    pre {
      overflow-x: auto;
      padding: 8px;
      background: var(--input-bg);
      border-radius: 4px;
      font-size: 13px;
    }

    code {
      padding: 1px 4px;
      background: var(--input-bg);
      border-radius: 3px;
      font-size: 13px;
    }
  }
}

.tool-calls {
  margin-bottom: 8px;
}

.tool-call {
  margin-bottom: 4px;
  font-size: 13px;

  summary {
    cursor: pointer;
    color: var(--muted);

    &:hover {
      color: var(--body-text);
    }
  }

  &__status {
    font-style: italic;
  }

  &__detail {
    margin-top: 4px;
  }

  &__section {
    margin-bottom: 4px;

    pre {
      max-height: 200px;
      overflow-y: auto;
      margin-top: 2px;
      font-size: 12px;
    }
  }
}

.chat-input {
  display: flex;
  gap: 8px;
  padding-top: 12px;
  border-top: 1px solid var(--border);

  textarea {
    flex: 1;
    resize: none;
    padding: 8px 12px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--input-bg);
    color: var(--input-text);
    font-size: 14px;

    &:focus {
      outline: none;
      border-color: var(--primary);
    }
  }

  button {
    align-self: flex-end;
  }
}

.chat-error {
  margin-top: 8px;
  padding: 8px 12px;
  border-radius: 4px;
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
