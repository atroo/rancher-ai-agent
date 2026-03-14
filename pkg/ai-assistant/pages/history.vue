<template>
  <div class="ai-assistant-history">
    <header>
      <h1>Chat History</h1>
    </header>

    <div v-if="messages.length === 0" class="empty-state">
      <p>No conversation history yet. Start a chat from the AI Assistant page.</p>
    </div>

    <div v-else class="history-list">
      <div
        v-for="msg in messages"
        :key="msg.id"
        class="history-item"
        :class="`history-item--${msg.role}`"
      >
        <span class="history-item__role">{{ msg.role }}</span>
        <span class="history-item__time">{{ formatTime(msg.timestamp) }}</span>
        <p class="history-item__content">{{ msg.content }}</p>
      </div>

      <div class="history-actions">
        <button class="btn role-secondary" @click="clearHistory">
          Clear History
        </button>
      </div>
    </div>
  </div>
</template>

<script lang="ts">
import { defineComponent, computed, getCurrentInstance } from 'vue';

export default defineComponent({
  name: 'AiAssistantHistory',

  setup() {
    const instance = getCurrentInstance();
    const store = (instance?.proxy as any)?.$store;

    const messages = computed(() => store.getters['ai-assistant-chat/allMessages']);

    function formatTime(ts: number): string {
      return new Date(ts).toLocaleString();
    }

    function clearHistory() {
      store.dispatch('ai-assistant-chat/clearHistory');
    }

    return { messages, formatTime, clearHistory };
  },
});
</script>

<style lang="scss" scoped>
.ai-assistant-history {
  padding: 20px;
}

.empty-state {
  text-align: center;
  padding: 40px;
  color: var(--muted);
}

.history-list {
  max-width: 800px;
}

.history-item {
  padding: 12px;
  margin-bottom: 8px;
  border-radius: 4px;
  border: 1px solid var(--border);

  &--user {
    background: var(--input-bg);
  }

  &__role {
    font-weight: bold;
    text-transform: capitalize;
    margin-right: 8px;
  }

  &__time {
    font-size: 12px;
    color: var(--muted);
  }

  &__content {
    margin-top: 4px;
    white-space: pre-wrap;
  }
}

.history-actions {
  margin-top: 16px;
}
</style>
