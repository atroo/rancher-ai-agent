export function init($extension, store) {
  const PRODUCT_NAME = 'aiassistant';

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
    labelKey: 'aiassistant.nav.chat',
    route:    {
      name:   `c-cluster-${ PRODUCT_NAME }-chat`,
      params: { product: PRODUCT_NAME },
    },
  });

  // History page
  virtualType({
    name:     'history',
    labelKey: 'aiassistant.nav.history',
    route:    {
      name:   `c-cluster-${ PRODUCT_NAME }-history`,
      params: { product: PRODUCT_NAME },
    },
  });

  // Memory management page
  virtualType({
    name:     'memory',
    label:    'Memory',
    route:    {
      name:   `c-cluster-${ PRODUCT_NAME }-memory`,
      params: { product: PRODUCT_NAME },
    },
  });

  basicType(['chat', 'history', 'memory']);

  weightType('chat', 3, true);
  weightType('history', 2, true);
  weightType('memory', 1, true);
}
