import { computed, getCurrentInstance } from 'vue';

/**
 * Extracts the current Rancher navigation context (cluster, namespace, resource)
 * from the Vue Router route params. Used to auto-scope AI assistant queries.
 *
 * NOTE: Rancher extensions run outside the normal vue-router provide/inject scope,
 * so we access $route and $store from the component instance proxy instead of
 * using useRoute()/useStore() composables.
 */
export function useRancherContext() {
  const instance = getCurrentInstance();
  const proxy = instance?.proxy as any;

  const clusterId = computed(() => proxy?.$route?.params?.cluster || '');
  const namespace = computed(() => proxy?.$route?.params?.namespace || undefined);
  const resourceType = computed(() => proxy?.$route?.params?.resource || undefined);
  const resourceName = computed(() => proxy?.$route?.params?.id || undefined);

  const clusterName = computed(() => {
    try {
      const clusters = proxy?.$store?.getters['management/all']?.('provisioning.cattle.io.cluster');

      return clusters?.find((c: any) => c.id === clusterId.value)?.nameDisplay || clusterId.value;
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
