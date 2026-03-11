import { computed } from 'vue';
import { useRoute } from 'vue-router';
import { useStore } from 'vuex';

/**
 * Extracts the current Rancher navigation context (cluster, namespace, resource)
 * from the Vue Router route params. Used to auto-scope AI assistant queries.
 */
export function useRancherContext() {
  const route = useRoute();
  const store = useStore();

  const clusterId = computed(() => route.params.cluster as string);
  const namespace = computed(() => (route.params.namespace as string) || undefined);
  const resourceType = computed(() => (route.params.resource as string) || undefined);
  const resourceName = computed(() => (route.params.id as string) || undefined);

  const clusterName = computed(() => {
    try {
      const clusters = store.getters['management/all']('provisioning.cattle.io.cluster');

      return clusters.find((c: any) => c.id === clusterId.value)?.nameDisplay || clusterId.value;
    } catch {
      return clusterId.value;
    }
  });

  const contextSummary = computed(() => {
    const parts: string[] = [];

    if (clusterName.value) {
      parts.push(`Cluster: ${ clusterName.value }`);
    }
    if (namespace.value) {
      parts.push(`Namespace: ${ namespace.value }`);
    }
    if (resourceType.value && resourceName.value) {
      parts.push(`${ resourceType.value }: ${ resourceName.value }`);
    }

    return parts.join(' / ');
  });

  const chatContext = computed(() => ({
    clusterId:    clusterId.value,
    namespace:    namespace.value,
    resourceType: resourceType.value,
    resourceName: resourceName.value,
  }));

  return {
    clusterId,
    clusterName,
    namespace,
    resourceType,
    resourceName,
    contextSummary,
    chatContext,
  };
}
