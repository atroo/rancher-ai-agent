# Rancher AI Assistant

An AI-powered investigation assistant for Kubernetes clusters managed by Rancher. It queries Prometheus metrics, Grafana Tempo traces, and the Kubernetes API to help operators diagnose issues — directly from the Rancher UI.

## Architecture

```
┌─────────────────────────────────────────────────┐
│  Rancher UI                                     │
│  ┌───────────────────────────────────────────┐  │
│  │  AI Assistant Extension (Vue 3)           │  │
│  │  - Chat page, resource tabs, dashboard    │  │
│  │    card                                   │  │
│  │  - SSE streaming (Vercel AI SDK protocol) │  │
│  └──────────────────┬────────────────────────┘  │
│                     │ /k8s/clusters/<id>/proxy   │
│  Rancher Server ────┘                           │
└─────────────────┬───────────────────────────────┘
                  │ K8s service proxy
┌─────────────────▼───────────────────────────────┐
│  Downstream Cluster                             │
│  ┌───────────────────────────────────────────┐  │
│  │  ai-assistant-backend (Go)                │  │
│  │  - Agent loop with tool-use               │  │
│  │  - Parallel tool execution                │  │
│  │  - Session persistence (SQLite)           │  │
│  │  - Long-term memory with embeddings       │  │
│  │  - Sub-agent spawning                     │  │
│  └──┬──────────┬──────────┬──────────────────┘  │
│     │          │          │                     │
│  Prometheus  Tempo    K8s API                   │
└─────────────────────────────────────────────────┘
```

**UI Extension** — Rancher UI Extensions v3 (Vue 3). Adds a chat page, tabs on pod/workload detail pages, and a cluster dashboard health card. Communicates with the backend via SSE through Rancher's K8s service proxy.

**Backend** — Go service deployed in the downstream cluster. Runs an LLM agent loop (Anthropic Claude) with tools for querying Prometheus, Tempo, K8s events/logs/status. Large results are stored in a virtual filesystem (VFS) to keep LLM context lean. Sessions and long-term memory are persisted to SQLite.

**Security** — Read-only RBAC. The backend's ServiceAccount can only `get`, `list`, and `watch` resources. No write operations, no secrets access. Auth is handled by Rancher's proxy layer.

## Features

- **Prometheus queries** — PromQL instant and range queries with automatic summarization
- **Tempo traces** — TraceQL search and trace-by-ID retrieval
- **K8s introspection** — Pod logs, events, resource status, workload listing
- **Virtual filesystem** — Large tool results stored out-of-context with search, pagination, and JSON query tools
- **Long-term memory** — Stores recurring patterns (errors, performance issues, scaling events) across conversations with optional semantic search via embeddings
- **Sub-agents** — Spawn focused child agents for deep-dive investigations without cluttering the main conversation
- **Session persistence** — Conversations survive pod restarts via SQLite

## Prerequisites

- Rancher 2.10+ with UI Extensions v3 enabled
- RKE2 cluster with kube-prometheus-stack (Rancher Monitoring)
- Grafana Tempo (optional, for distributed tracing)
- Anthropic API key

## Deployment

### 1. Backend

Create the namespace and secrets, then deploy with Helm:

```bash
kubectl create namespace cattle-ai-assistant

# Required: LLM API key
kubectl -n cattle-ai-assistant create secret generic ai-assistant-llm-key \
  --from-literal=api-key=<your-anthropic-api-key>

# Optional: Embedding API key (enables semantic search in long-term memory)
kubectl -n cattle-ai-assistant create secret generic ai-assistant-embedding-key \
  --from-literal=api-key=<your-voyage-ai-key>

# Deploy
helm install ai-assistant-backend ./charts/ai-assistant-backend/0.1.0 \
  --namespace cattle-ai-assistant
```

With embedding support:

```bash
helm install ai-assistant-backend ./charts/ai-assistant-backend/0.1.0 \
  --namespace cattle-ai-assistant \
  --set embedding.apiKeySecretName=ai-assistant-embedding-key
```

### 2. UI Extension

The UI extension is distributed as an OCI catalog image. After creating a GitHub Release, the CI workflow pushes the image to `ghcr.io`.

In Rancher:
1. Navigate to **Extensions > Manage Repositories > Create**
2. Select **Container Image** as the type
3. Enter the catalog image: `ghcr.io/<org>/rancher_ai_assistant/ui-extension-catalog:<version>`
4. The **AI Assistant** extension appears in the Extensions page — install it

## Helm Values Reference

### Image

| Value | Description | Default |
|---|---|---|
| `image.repository` | Backend container image | `ghcr.io/atroo/ai-assistant-backend` |
| `image.tag` | Image tag | `latest` |
| `image.pullPolicy` | Pull policy | `IfNotPresent` |
| `replicaCount` | Number of replicas (only 1 supported due to SQLite) | `1` |

### LLM

| Value | Description | Default |
|---|---|---|
| `llm.provider` | LLM provider | `anthropic` |
| `llm.model` | Model name | `claude-sonnet-4-6` |
| `llm.apiKeySecretName` | Name of the Secret containing the API key | `ai-assistant-llm-key` |
| `llm.apiKeySecretKey` | Key within the Secret | `api-key` |

### Embeddings (optional)

Enables semantic search in long-term memory. Works with any OpenAI-compatible embedding API (Voyage AI, OpenAI, Ollama, etc.). If not configured, memory search falls back to text matching.

| Value | Description | Default |
|---|---|---|
| `embedding.baseUrl` | Embedding API base URL | `https://api.voyageai.com/v1` |
| `embedding.model` | Embedding model name | `voyage-3-lite` |
| `embedding.dimensions` | Vector dimensions | `512` |
| `embedding.apiKeySecretName` | Secret name (empty = disabled) | `""` |
| `embedding.apiKeySecretKey` | Key within the Secret | `api-key` |

### Datasources

| Value | Description | Default |
|---|---|---|
| `datasources.prometheus.url` | Prometheus URL | `http://rancher-monitoring-prometheus.cattle-monitoring-system:9090` |
| `datasources.tempo.url` | Tempo query frontend URL | `http://tempo-query-frontend.cattle-monitoring-system:3200` |

### Persistence

| Value | Description | Default |
|---|---|---|
| `persistence.enabled` | Enable PVC for SQLite database | `true` |
| `persistence.size` | PVC size | `1Gi` |
| `persistence.storageClass` | Storage class (empty = default) | `""` |

### Other

| Value | Description | Default |
|---|---|---|
| `service.port` | Service port | `8080` |
| `resources.requests.memory` | Memory request | `128Mi` |
| `resources.requests.cpu` | CPU request | `100m` |
| `resources.limits.memory` | Memory limit | `512Mi` |
| `rbac.create` | Create ClusterRole and binding | `true` |
| `namespace` | Target namespace | `cattle-ai-assistant` |

## Development

### Backend

```bash
cd backend
go build ./...
go run ./cmd/server
```

Requires environment variables: `LLM_API_KEY`, and optionally `PROMETHEUS_URL`, `TEMPO_URL`, `EMBEDDING_API_KEY`.

### UI Extension

Requires Node 20 (see `.nvmrc`).

```bash
nvm use 20
yarn install
yarn build-pkg ai-assistant    # production build → dist-pkg/
yarn dev                       # watch mode + dev server on http://localhost:4500
```

To test in Rancher, enable Extension Developer Features in Preferences, then load from `http://localhost:4500/ai-assistant-<version>/ai-assistant-<version>.umd.js`.

## CI/CD

| Workflow | Trigger | Output |
|---|---|---|
| `build-extension-charts.yml` | GitHub Release, manual | UI extension + backend Helm charts → gh-pages |
| `build-backend.yml` | GitHub Release, manual, `backend-*` tags | Backend Docker image → `ghcr.io` |

## Releasing

### UI Extension

The version must be consistent across two files before creating a release:

| File | Field |
|---|---|
| `package.json` (root) | `version` |
| `pkg/ai-assistant/package.json` | `version` |

Steps:

1. Bump the version in **both** `package.json` files (e.g., `0.4.0`)
2. Commit and push to `main`
3. Create a GitHub Release with the tag **`ai-assistant-<version>`** (e.g., `ai-assistant-0.4.0`)
   - The tag format is `<pkg-folder-name>-<version>` — it must match `pkg/ai-assistant/package.json`
4. The `build-extension-charts.yml` workflow runs automatically:
   - Builds the UI extension
   - Packages it as a Helm chart
   - Publishes to the `gh-pages` branch
   - The backend chart is also added to the same Helm index
5. The chart becomes available at `https://<org>.github.io/<repo>/`

### Backend

1. Optionally bump `charts/ai-assistant-backend/<version>/Chart.yaml` if the chart changed
2. Create a GitHub Release with the tag **`backend-<version>`** (e.g., `backend-0.2.0`)
3. The `build-backend.yml` workflow builds and pushes the Docker image to `ghcr.io`

### Adding the Helm Repository in Rancher

After the first successful release, enable GitHub Pages (Settings > Pages > Deploy from `gh-pages` branch). Then in Rancher:

1. Navigate to **Apps > Repositories > Create**
2. Select **HTTP** as the type
3. Enter the URL: `https://<org>.github.io/<repo>/`
4. Both the UI extension and backend charts appear in the catalog

## License

TBD
