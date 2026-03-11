// String values from @shell/core/types enums — avoids needing TS compilation
const ActionLocation = { HEADER: 'header-action' };
const TabLocation = { RESOURCE_DETAIL_PAGE: 'resource-detail-page' };
const CardLocation = { CLUSTER_DASHBOARD_CARD: 'cluster-dashboard-card' };

export default function(plugin) {
  plugin.metadata = require('./package.json');

  // Register as a cluster-level product (sidebar entry per cluster)
  plugin.addProduct(require('./product'));

  // Global header button — opens the AI chat drawer from any page
  plugin.addAction(ActionLocation.HEADER, {}, {
    tooltip:    'AI Assistant',
    shortcut:   'i',
    icon:       'icon-search',
    invoke(opts, resources) {
      // Toggle the chat drawer via the Vuex store
      opts.dispatch('ai-assistant-chat/toggleDrawer');
    },
  });

  // Tab on pod detail pages
  plugin.addTab(
    TabLocation.RESOURCE_DETAIL_PAGE,
    { resource: ['pod'] },
    {
      name:      'ai-assistant',
      label:     'AI Assistant',
      weight:    -10,
      component: () => import('./components/ResourceAssistantTab.vue'),
    }
  );

  // Tab on workload detail pages (deployments, statefulsets, daemonsets)
  plugin.addTab(
    TabLocation.RESOURCE_DETAIL_PAGE,
    { resource: ['apps.deployment', 'apps.statefulset', 'apps.daemonset'] },
    {
      name:      'ai-assistant',
      label:     'AI Assistant',
      weight:    -10,
      component: () => import('./components/ResourceAssistantTab.vue'),
    }
  );

  // Card on the cluster dashboard — quick health check
  plugin.addCard(
    CardLocation.CLUSTER_DASHBOARD_CARD,
    {},
    {
      label:     'AI Assistant',
      component: () => import('./components/ClusterHealthCard.vue'),
    }
  );

  // Register the chat store module
  plugin.addStore('ai-assistant-chat', require('./stores/chat').default);
}
