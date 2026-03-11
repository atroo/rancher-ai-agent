const PRODUCT_NAME = 'ai-assistant';

const routes = [
  {
    name:      `c-cluster-${ PRODUCT_NAME }-chat`,
    path:      `/c/:cluster/${ PRODUCT_NAME }/chat`,
    component: () => import('../pages/index.vue'),
    meta:      { product: PRODUCT_NAME },
  },
  {
    name:      `c-cluster-${ PRODUCT_NAME }-history`,
    path:      `/c/:cluster/${ PRODUCT_NAME }/history`,
    component: () => import('../pages/history.vue'),
    meta:      { product: PRODUCT_NAME },
  },
];

export default routes;
