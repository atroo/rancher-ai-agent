<template>
  <div class="memory-page">
    <header class="memory-page__header">
      <h1>Long-Term Memory</h1>
      <div class="memory-page__filters">
        <select v-model="categoryFilter" class="select-sm">
          <option value="">All Categories</option>
          <option value="error_pattern">Error Patterns</option>
          <option value="performance">Performance</option>
          <option value="scaling">Scaling</option>
          <option value="security">Security</option>
          <option value="config_drift">Config Drift</option>
        </select>
        <label class="checkbox-label">
          <input v-model="showResolved" type="checkbox">
          Show Resolved
        </label>
      </div>
    </header>

    <div v-if="loading" class="loading">
      <span class="loading-indicator">Loading memories...</span>
    </div>

    <div v-if="error" class="banner bg-error mt-10">
      {{ error }}
    </div>

    <div v-if="!loading && !entries.length" class="empty-state">
      <p>No memory entries found. The AI assistant stores patterns it discovers during investigations.</p>
    </div>

    <table v-if="entries.length" class="sortable-table">
      <thead>
        <tr>
          <th>Category</th>
          <th>Severity</th>
          <th>Summary</th>
          <th>Resource</th>
          <th>Seen</th>
          <th>Last Seen</th>
          <th>Status</th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="entry in entries" :key="entry.id" :class="{ 'resolved': entry.resolved }">
          <td>
            <span class="badge" :class="'badge--' + entry.category">
              {{ formatCategory(entry.category) }}
            </span>
          </td>
          <td>
            <span class="severity" :class="'severity--' + entry.severity">
              {{ entry.severity }}
            </span>
          </td>
          <td class="summary-cell">
            <div class="summary-text">{{ entry.summary }}</div>
            <div v-if="expandedId === entry.id" class="details-text">{{ entry.details }}</div>
            <button
              v-if="entry.details"
              class="btn btn-xs role-link"
              @click="toggleExpand(entry.id)"
            >
              {{ expandedId === entry.id ? 'Less' : 'More' }}
            </button>
          </td>
          <td>
            <span v-if="entry.namespace" class="text-muted">{{ entry.namespace }}/</span>{{ entry.resource || '-' }}
          </td>
          <td>{{ entry.occurrenceCount }}x</td>
          <td>{{ formatDate(entry.lastSeenAt) }}</td>
          <td>
            <span v-if="entry.resolved" class="text-muted">Resolved</span>
            <span v-else class="text-warning">Active</span>
          </td>
          <td class="actions-cell">
            <button
              class="btn btn-xs role-secondary"
              @click="toggleResolved(entry)"
            >
              {{ entry.resolved ? 'Reopen' : 'Resolve' }}
            </button>
            <button
              class="btn btn-xs role-secondary text-error"
              @click="deleteEntry(entry)"
            >
              Delete
            </button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script lang="ts">
import { defineComponent, ref, watch, onMounted, getCurrentInstance } from 'vue';

export default defineComponent({
  name: 'MemoryPage',

  setup() {
    const instance = getCurrentInstance();
    const proxy = instance?.proxy as any;

    const entries = ref<any[]>([]);
    const loading = ref(false);
    const error = ref<string | null>(null);
    const categoryFilter = ref('');
    const showResolved = ref(false);
    const expandedId = ref<number | null>(null);

    function backendUrl(path: string): string {
      const clusterId = proxy?.$route?.params?.cluster || '';

      return `/k8s/clusters/${ clusterId }/api/v1/namespaces/cattle-ai-assistant/services/http:ai-assistant-backend:8080/proxy${ path }`;
    }

    async function fetchEntries() {
      loading.value = true;
      error.value = null;

      try {
        const params = new URLSearchParams();

        if (categoryFilter.value) {
          params.set('category', categoryFilter.value);
        }
        if (showResolved.value) {
          params.set('resolved', 'true');
        }

        const url = backendUrl(`/api/v1/memories?${ params.toString() }`);
        const resp = await fetch(url);

        if (!resp.ok) {
          throw new Error(`Backend returned ${ resp.status }`);
        }

        entries.value = await resp.json();
      } catch (err: any) {
        error.value = err.message || 'Failed to load memories';
      } finally {
        loading.value = false;
      }
    }

    async function toggleResolved(entry: any) {
      try {
        const url = backendUrl(`/api/v1/memories/${ entry.id }`);

        await fetch(url, {
          method:  'PATCH',
          headers: { 'Content-Type': 'application/json' },
          body:    JSON.stringify({ resolved: !entry.resolved }),
        });

        entry.resolved = !entry.resolved;
      } catch (err: any) {
        error.value = err.message;
      }
    }

    async function deleteEntry(entry: any) {
      try {
        const url = backendUrl(`/api/v1/memories/${ entry.id }`);

        await fetch(url, { method: 'DELETE' });

        entries.value = entries.value.filter((e: any) => e.id !== entry.id);
      } catch (err: any) {
        error.value = err.message;
      }
    }

    function toggleExpand(id: number) {
      expandedId.value = expandedId.value === id ? null : id;
    }

    function formatCategory(cat: string): string {
      return cat.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
    }

    function formatDate(dateStr: string): string {
      if (!dateStr) {
        return '-';
      }

      const d = new Date(dateStr);
      const now = new Date();
      const diff = now.getTime() - d.getTime();
      const hours = Math.floor(diff / 3600000);

      if (hours < 1) {
        return 'Just now';
      }
      if (hours < 24) {
        return `${ hours }h ago`;
      }

      const days = Math.floor(hours / 24);

      if (days < 7) {
        return `${ days }d ago`;
      }

      return d.toLocaleDateString();
    }

    watch([categoryFilter, showResolved], () => fetchEntries());
    onMounted(() => fetchEntries());

    return {
      entries,
      loading,
      error,
      categoryFilter,
      showResolved,
      expandedId,
      toggleResolved,
      deleteEntry,
      toggleExpand,
      formatCategory,
      formatDate,
    };
  },
});
</script>

<style lang="scss" scoped>
.memory-page {
  padding: 20px;

  &__header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 20px;

    h1 {
      margin: 0;
      font-size: 20px;
    }
  }

  &__filters {
    display: flex;
    gap: 12px;
    align-items: center;
  }
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  font-size: 14px;
}

.sortable-table {
  width: 100%;
  border-collapse: collapse;

  th, td {
    padding: 8px 12px;
    text-align: left;
    border-bottom: 1px solid var(--border);
  }

  th {
    font-weight: 600;
    font-size: 13px;
    color: var(--muted);
  }

  tr.resolved {
    opacity: 0.6;
  }
}

.summary-cell {
  max-width: 400px;
}

.summary-text {
  font-weight: 500;
}

.details-text {
  margin-top: 6px;
  font-size: 13px;
  color: var(--muted);
  white-space: pre-wrap;
}

.actions-cell {
  white-space: nowrap;

  .btn + .btn {
    margin-left: 4px;
  }
}

.badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;

  &--error_pattern { background: var(--error-bg, #fde8e8); color: var(--error, #c00); }
  &--performance { background: var(--warning-bg, #fff3cd); color: var(--warning, #856404); }
  &--scaling { background: var(--info-bg, #d1ecf1); color: var(--info, #0c5460); }
  &--security { background: var(--error-bg, #fde8e8); color: var(--error, #c00); }
  &--config_drift { background: var(--muted-bg, #e9ecef); color: var(--muted, #6c757d); }
}

.severity {
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;

  &--critical { color: var(--error, #c00); }
  &--warning { color: var(--warning, #856404); }
  &--info { color: var(--muted, #6c757d); }
}

.empty-state {
  text-align: center;
  padding: 60px 20px;
  color: var(--muted);
}

.loading-indicator {
  color: var(--muted);
  animation: pulse 1.5s ease-in-out infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 0.4; }
  50% { opacity: 1; }
}

.select-sm {
  padding: 4px 8px;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--input-bg);
  color: var(--input-text);
  font-size: 14px;
}
</style>
