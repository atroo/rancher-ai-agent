export function init($extension, store) {
  const PRODUCT_NAME = 'ai-assistant';

  const {
    product,
    virtualType,
    basicType,
    weightType,
  } = $extension.DSL(store, PRODUCT_NAME);

  // Cluster-level product — appears in the sidebar when viewing a cluster
  product({
    icon:    'icon-search',
    inStore: 'cluster',
    weight:  50,
    to:      {
      name:   `c-cluster-${ PRODUCT_NAME }-chat`,
      params: { product: PRODUCT_NAME },
    },
  });

  // Chat page
  virtualType({
    name:     'chat',
    labelKey: 'ai-assistant.nav.chat',
    route:    {
      name:   `c-cluster-${ PRODUCT_NAME }-chat`,
      params: { product: PRODUCT_NAME },
    },
  });

  // History page
  virtualType({
    name:     'history',
    labelKey: 'ai-assistant.nav.history',
    route:    {
      name:   `c-cluster-${ PRODUCT_NAME }-history`,
      params: { product: PRODUCT_NAME },
    },
  });

  basicType(['chat', 'history']);

  weightType('chat', 2, true);
  weightType('history', 1, true);
}
