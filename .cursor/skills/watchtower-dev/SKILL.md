---
name: watchtower-dev
description: >-
  Guide for developing the watchTower AI ops platform. Use when adding new Eino
  tools, creating agent workflows, modifying chat/AI-ops controllers, updating
  Prometheus/Milvus/MCP integrations, working with the model factory, skills
  system, or asking about watchTower architecture, project structure, config,
  and conventions.
---

# watchTower Development Guide

## Project Overview

watchTower is a Go Gin backend providing AI-powered ops capabilities: chat (including SSE streaming), AI ops alert analysis (Plan-Execute-Replan), knowledge base (RAG via Milvus), file upload & indexing, with a SuperBizAgent web UI frontend.

**Tech stack**: Go 1.25, Gin, CloudWeGo Eino (LLM/agent framework) + Eino ADK, Milvus (vector DB), Viper (config), GORM/MySQL, MCP (log queries via Tencent CLS).

## Directory Structure

```
watchTower/
├── main.go                              # Gin server entry (config → middleware → router)
├── etc/conf.yml                         # All config (server, LLM, embedding, Milvus, Prometheus, MCP, Google Search)
├── knowledge/告警处理手册.md                  # Example knowledge base docs for Milvus indexing
│
├── ai/
│   ├── tools/                           # Eino tool implementations (each exports a constructor)
│   │   ├── query_metric_alerts.go       # Prometheus alerts tool (GET /api/v1/alerts)
│   │   ├── query_log.go                 # Log query tool (via MCP/SSE, Tencent CLS)
│   │   ├── query_internal_docs.go       # RAG knowledge retrieval tool (Milvus vector search)
│   │   ├── get_current_time.go          # Time utility tool (Unix timestamps + human-readable)
│   │   ├── mysql_crud.go                # Database CRUD tool (GORM, with user confirmation)
│   │   ├── search_file.go               # Filesystem file search tool (by keyword/extension)
│   │   └── tools_test.go                # Tool tests
│   │
│   ├── skills/                          # Skills plugin system (domain knowledge injection)
│   │   ├── loader.go                    # Auto-scan .md files → parse YAML frontmatter → inject into system prompt
│   │   ├── alert_handling.md            # Skill: alert diagnosis & resolution playbook
│   │   ├── file_search.md               # Skill: file search usage guide
│   │   └── ReadMe                       # Skills architecture explanation
│   │
│   ├── agent/
│   │   ├── chat_workflow/               # Chat agent (Eino Graph: ReAct agent + RAG retrieval)
│   │   │   ├── orchestration.go          # BuildChatAgent() — assembles & compiles the Eino graph
│   │   │   ├── flow.go                 # ReAct agent node (tools binding + stream tool-call checker)
│   │   │   ├── prompt.go               # System prompt template (with skills injection via skills.FormatForPrompt())
│   │   │   ├── model.go                # Chat model init (via model factory → DsThinkChatModel)
│   │   │   ├── embedding.go            # Embedding model init (via model factory → DoubaoEmbedder)
│   │   │   ├── retriever.go            # Milvus retriever node
│   │   │   ├── tools_node.go           # DuckDuckGo search tool node
│   │   │   ├── lambda_func.go          # Lambda converters (UserMessage → query string / template vars)
│   │   │   ├── types.go               # UserMessage struct (ID, Query, History)
│   │   │   ├── chat_workflow_test.go
│   │   │   └── etc/                   # Eino graph visualization (auto-generated JSON + PNG)
│   │   │
│   │   ├── plan_execute_replan/        # Plan-Execute-Replan agent for complex AI ops (Eino ADK)
│   │   │   ├── workAgent_planExecuteReplan.go  # BuildPlanExecuteReplanAgent() — orchestrator
│   │   │   ├── planner.go             # Planner agent (DsThinkChatModel, generates task plan)
│   │   │   ├── executor.go            # Executor agent (DsQuickChatModel + tools: MCP/alerts/docs/time)
│   │   │   └── replan.go              # Replanner agent (DsThinkChatModel, evaluates & adjusts plan)
│   │   │
│   │   └── knowledge_index_workflow/   # Knowledge indexing pipeline (Eino Graph: load→split→embed→index)
│   │       ├── orchestration.go         # BuildKnowledgeIndexing() — assembles & compiles the graph
│   │       ├── loader.go               # FileLoader node (eino-ext file loader)
│   │       ├── transformer.go          # MarkdownSplitter node (split by # headers, UUID per chunk)
│   │       ├── embedding.go            # Embedding model init (via model factory → DoubaoEmbedder)
│   │       ├── indexer.go              # Indexer node (Milvus indexer with embedding)
│   │       └── etc/                   # Eino graph visualization (auto-generated JSON + PNG)
│   │
│   └── cmd/                             # Standalone CLI entry points for testing
│       ├── chat_cmd/main.go             # Test chat workflow
│       ├── ai_ops_cmd/main.go           # Test plan-execute-replan agent
│       ├── knowledge_cmd/main.go       # Batch-index markdown docs into Milvus (reads ./docs/*.md at project root)
│       ├── recall_cmd_lowLevel/main.go  # Test Milvus retriever directly
│       └── llm_tool_cmd_lowLevel/main.go # Test LLM + tool binding at low level
│
├── controller/chat/                      # HTTP handlers
│   ├── chat_v1_chat.go                 # POST /api/chat — sync chat (Invoke → JSON response)
│   ├── chat_v1_chatStream.go          # POST /api/chat-stream — SSE streaming chat (Stream → data: chunks)
│   ├── chat_v1_ai_ops.go              # POST /api/ai-ops — AI ops alert analysis (Plan-Execute-Replan)
│   └── chat_v1_file_upload.go         # POST /api/upload — file upload + knowledge indexing into Milvus
│
├── router/
│   ├── routerInit.go                    # InitRouter() — registers /health GET + /api group
│   └── chatRouter.go                  # ChatRouter() — registers chat/stream/ai-ops/upload POST routes
│
├── model/                              # AI model factory (singleton pattern)
│   ├── model.go                       # AIModel interface + concrete models (DsThinkChatModel, DsQuickChatModel, DoubaoEmbedder)
│   ├── model_factory.go              # AIModelFactory — global singleton via GetGlobalFactory(), creator registration
│   ├── model_test.go
│   └── vo/types.go                   # Request/Response VO structs (ChatReq, ChatRes, AIOpsRes, FileUploadRes, etc.)
│
├── mem/mem.go                         # In-memory conversation history (per-session, sliding window of 20 messages)
│
├── common/
│   ├── config/config.go                 # Viper config loading (etc/conf.yml → Config struct), global Conf singleton
│   ├── enum/enum.go                  # Enum helper type (map[int]string with Code/Value lookups)
│   ├── commonResult/commonResult.go  # Standardized API response wrapper (BizCode + CommonResult)
│   ├── log_callback/log_callback.go  # Eino callbacks handler for logging component I/O (global or per-run injection)
│   ├── fileloader/fileloader.go     # Eino file loader wrapper (eino-ext)
│   ├── milvus/
│   │   ├── milvusClient.go          # Milvus client (auto-creates DB + collection + indexes if not exist)
│   │   ├── indexer.go               # Milvus indexer (FloatVector document converter + DoubaoEmbedder)
│   │   └── retriver.go             # Milvus retriever (FloatVector converter + DoubaoEmbedder, TopK=1)
│   └── utils/http.go               # HTTP utility functions (Post, GetWithHeader, PostWithHeader with proxy support)
│
├── middleware/middleware.go           # CORS middleware (origin whitelist + dev localhost passthrough + OPTIONS preflight handling)
│
├── docker/                            # Docker Compose deployment (local dev)
│   ├── docker-compose.yml            # etcd + MinIO + Milvus standalone + Attu UI + mock-prometheus + watchtower + frontend
│   ├── conf-docker.yml              # Config file bound into watchtower container (/app/etc/conf.yml)
│   ├── Dockerfile.backend           # Multi-stage build for watchtower backend (Go 1.25-alpine, CGO_ENABLED=0)
│   ├── Dockerfile.frontend          # Nginx Alpine for static frontend (SuperBizAgentFrontend/), build-time --build-arg NGINX_API_BASE
│   ├── Dockerfile.mock-prometheus   # Go Alpine for mock Prometheus alert server
│   └── volumes/                     # Milvus/MinIO persistent data (gitignored)
│
├── k8s/                               # Kubernetes (Kind) deployment
│   ├── backend/
│   │   ├── Dockerfile              # Multi-stage build (same as docker/Dockerfile.backend)
│   │   └── go.mod                  # Minimal go.mod for k8s build context
│   └── deploy/
│       ├── 00-kind-cluster.yml      # Kind cluster: 1 control-plane (with ingress-ready label) + 3 workers, hostPort 80/443
│       ├── 01-ingress-controller.yml # Ingress-Nginx Controller v1.14.3 + RBAC + IngressClass + ValidatingWebhook
│       ├── 02-namespace-configmap.yml # watchtower Namespace + ConfigMap (conf.yml as data, includes all API keys)
│       ├── 03-statefulset-etcd.yml  # etcd v3.5.18 StatefulSet (PVC 2Gi) — Milvus metadata store
│       ├── 04-statefulset-minio.yml # MinIO RELEASE.2023-03-20 StatefulSet (PVC 10Gi) — Milvus object store
│       ├── 05-statefulset-milvus.yml # Milvus v2.5.10 Deployment (emptyDir, seccompProfile: Unconfined)
│       ├── 06-mock-prometheus.yml   # Mock Prometheus Deployment + Service (:9090)
│       ├── 07-watchtower.yml        # Backend (init container + ConfigMap mount + /health probe) + Frontend Deployment + Service
│       ├── 08-hpa.yml               # HPA autoscaling/v2: 2-5 replicas, CPU 50%
│       ├── 09-ingress.yml           # Ingress: / → frontend, /api → backend
│       ├── 10-metrics-server.yml    # Metrics Server v0.8.1 (--kubelet-insecure-tls for Kind)
│       ├── deploy.sh                # Full deploy script (build → kind load → apply infra → apply app → HPA)
│       └── stress_test.sh           # HPA load test script (curl loop + kubectl top monitoring)
│
├── scripts_mockPrometheus/            # Mock Prometheus alert server (standalone binary)
│   └── mock_prometheus.go
│
├── SuperBizAgentFrontend/            # Static HTML/JS chat UI (served by Nginx)
│   ├── index.html, app.js, styles.css
│   ├── package.json, start.sh, README.md
│   └── .gitignore
│
└── knowledge/告警处理手册.md               # Example knowledge base document (indexed into Milvus by knowledge_cmd)
```

## Key Architectural Patterns

### Model Factory (Singleton)

All AI models (LLM, Embedder) are created through `model.GetGlobalFactory()`. Each model type has a registered `ModelCreator` function. To use a model:

```go
creator := model.GetGlobalFactory().GetModelCreator(model.DsThinkChatModelType)
cm := creator(ctx, config.Conf).(*model.DsThinkChatModel).Model
```

Model types: `DsThinkChatModelType` (1), `DsQuickChatModelType` (2), `DoubaoEmbedderType` (3).

### Skills Plugin System

`ai/skills/` provides a domain-knowledge injection mechanism. Each `.md` file with YAML frontmatter (`name`, `description`) is auto-loaded at startup and injected into agent system prompts via `skills.FormatForPrompt()`. Skills are injected in both the Chat agent (`prompt.go`) and the Plan-Execute-Replan agent (`workAgent_planExecuteReplan.go`).

### Eino Graph Orchestration

Agent workflows are built as Eino directed graphs (`compose.NewGraph`) with typed nodes (Lambda, ChatTemplate, Retriever, Loader, Transformer, Indexer) and compiled into `compose.Runnable`. The chat workflow graph flows: `START → [InputToRag→MilvusRetriever, InputToChat] → ChatTemplate → ReactAgent → END`.

### Eino ADK (Plan-Execute-Replan)

The AI-ops agent uses Eino ADK's `planexecute` prebuilt pattern: Planner generates a step-by-step plan, Executor runs each step with tools, Replanner evaluates results and decides whether to adjust the plan or output the final answer.

## Adding a New Eino Tool

Follow the pattern in `ai/tools/`. Each tool is an `eino tool.InvokableTool` created via `utils.InferOptionableTool`.

```go
package tools

import (
    "context"
    "encoding/json"
    "log"

    "github.com/cloudwego/eino/components/tool"
    "github.com/cloudwego/eino/components/tool/utils"
)

type YourToolOutput struct {
    Success bool   `json:"success" jsonschema:"description=是否成功"`
    Data    string `json:"data,omitempty" jsonschema:"description=返回数据"`
    Error   string `json:"error,omitempty" jsonschema:"description=错误信息"`
}

func NewYourTool() tool.InvokableTool {
    t, err := utils.InferOptionableTool(
        "your_tool_name",
        "Tool description for the LLM to understand when to use it.",
        func(ctx context.Context, input *YourInput, opts ...tool.Option) (string, error) {
            out := YourToolOutput{Success: true, Data: "result"}
            b, _ := json.MarshalIndent(out, "", "  ")
            return string(b), nil
        },
    )
    if err != nil {
        log.Fatal(err)
    }
    return t
}
```

After creating the tool, register it in the agent's tool list:
- **Chat agent**: `ai/agent/chat_workflow/flow.go` — append to `config.ToolsConfig.Tools`
- **Plan-Execute-Replan agent**: `ai/agent/plan_execute_replan/executor.go` — append to `toolList`

## Adding a New Agent Workflow

Agent workflows live in `ai/agent/`. The three existing patterns:

1. **Chat Workflow** (`chat_workflow/`): Single/multi-turn chat with ReAct agent pattern. Built using Eino graph with parallel RAG retrieval + prompt template merging, then fed into a ReAct agent with tool-calling. Supports both `Invoke` (sync) and `Stream` (SSE). Entry: `BuildChatAgent(ctx)`.

2. **Plan-Execute-Replan** (`plan_execute_replan/`): Complex multi-step tasks via Eino ADK. Has planner (generates steps), executor (runs each step with tools), and replanner (adjusts plan or outputs final result). Uses `adk.NewRunner` for iteration. Entry: `BuildPlanExecuteReplanAgent(ctx, query)`.

3. **Knowledge Indexing** (`knowledge_index_workflow/`): Document ingestion pipeline. Graph: FileLoader → MarkdownSplitter (by `#` headers, UUID per chunk) → Milvus Indexer (embedding + store). Entry: `BuildKnowledgeIndexing(ctx)`.

To create a new agent, create a subdirectory under `ai/agent/` and expose a `Build*Agent(ctx)` function returning a `compose.Runnable` or equivalent.

## Adding a New Skill

Create a `.md` file in `ai/skills/` with YAML frontmatter:

```markdown
---
name: Your Skill Name
description: When to use this skill (1 sentence).
---

# Skill Content

Detailed domain knowledge, playbooks, procedures here...
```

The skill is auto-loaded by `ai/skills/loader.go` and injected into agent system prompts. No code changes needed.

## Config Convention

All config in `etc/conf.yml`, loaded by `common/config/config.go` into the global `config.Conf` singleton (via Viper). Key config sections:

| Section | Purpose |
|---------|---------|
| `server` | Host, port, app name |
| `logger` | Log level, stdout toggle |
| `log_callback` | Eino callback logging (detail, debug) |
| `ds_think_chat_model` | DeepSeek thinking model (api_key, base_url, model) |
| `ds_quick_chat_model` | DeepSeek quick model (api_key, base_url, model) |
| `doubao_embedding_model` | Doubao embedding (api_key, model, vector_dim) |
| `file_dir` | Knowledge base file upload directory |
| `mcp_url` | Tencent CLS MCP SSE endpoint |
| `prometheus` | Prometheus HTTP API base URL |
| `milvus` | DB name + collection name |
| `google_search` | Optional Google Custom Search API (api_key, search_engine_id) |

When adding new external service integrations, add config fields to the `Config` struct in `common/config/config.go` and corresponding entries in `etc/conf.yml`.

## API Convention

Routes registered in `router/chatRouter.go` under `/api` group (initialized in `router/routerInit.go`). Controllers in `controller/chat/`. Current endpoints:

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/health` | inline handler | Health check (K8s liveness/readiness probes) |
| POST | `/api/chat` | `chat.Chat` | Sync chat (Invoke → JSON response) |
| POST | `/api/chat-stream` | `chat.ChatStream` | SSE streaming chat (Stream → `data:` chunks + `[DONE]`) |
| POST | `/api/ai-ops` | `chat.AIOps` | AI ops alert analysis (Plan-Execute-Replan → JSON) |
| POST | `/api/upload` | `chat.FileUpload` | File upload + knowledge indexing into Milvus |

Pattern: parse request → build agent → `Invoke`/`Stream` → return JSON or SSE stream. Use `logcallback.LogCallback()` for observability. Conversation history managed via `mem.GetSimpleMemory(id)`.

**Note on health endpoint**: The `/health` GET endpoint is registered directly on the root Gin engine (before the `/api` group) in `routerInit.go`. This ensures K8s HTTP probes can reach it without needing to match a POST route.

## Infrastructure

### Docker Compose (Local Dev)

`docker/docker-compose.yml` orchestrates all services in a bridge network:

| Service | Image | External Port | Internal Port | Purpose |
|---------|-------|-------------|---------------|---------|
| etcd | quay.io/coreos/etcd:v3.5.18 | — | 2379 | Milvus metadata store |
| minio | minio/minio:RELEASE.2023-03-20T20-16-18Z | 9001 (console) | 9000 | Milvus object store |
| standalone | milvusdb/milvus:v2.5.10 | 19530, 9091 | 19530, 9091 | Vector DB |
| attu | zilliz/attu:v2.6 | 8000 | 3000 | Milvus admin UI |
| mock-prometheus | watchtower/mock-prometheus:v1.0.0 | — | 9090 | Alert data simulation |
| watchtower | watchtower/backend:v1.0.0 | 6872 | 6872 | Go backend |
| frontend | watchtower/frontend:v1.0.0 | 8080 | 80 | Nginx static UI (NGINX_API_BASE=http://localhost:6872/api) |

Collection auto-created by `common/milvus/milvusClient.go` with fields: `id` (VarChar PK), `vector` (FloatVector), `content` (VarChar), `metadata` (JSON).

### Kubernetes (Kind)

`k8s/deploy/deploy.sh` deploys the full stack to a Kind cluster. Deployment order:
1. Kind cluster (`00-kind-cluster.yml`) — control-plane with `ingress-ready=true` label + 3 workers
2. Ingress Controller (`01-ingress-controller.yml`)
3. etcd StatefulSet → MinIO StatefulSet → Milvus Deployment
4. mock-prometheus Deployment
5. watchtower Backend (with `wait-for-milvus` init container) + Frontend
6. Ingress routing + metrics-server + HPA

**K8s ConfigMap** (`02-namespace-configmap.yml`): Contains the full `conf.yml` as a data key, including API keys. Config is mounted into the backend Pod at `/app/etc/conf.yml` via `subPath: conf.yml` (prevents ConfigMap updates from overwriting the file).

**Important for Kind**: Milvus requires `seccompProfile.type: Unconfined` in the Pod securityContext (equivalent to docker-compose's `security_opt: seccomp:unconfined`). Without it, Milvus will fail to start in Kind.

### MCP

Tencent CLS log MCP via SSE client (`ai/tools/query_log.go`). URL configured in `etc/conf.yml` as `mcp_url`.

## Testing

CLI test entry points under `ai/cmd/*/main.go` allow testing agents and tools independently:

- `chat_cmd` — chat workflow end-to-end
- `ai_ops_cmd` — plan-execute-replan agent end-to-end
- `knowledge_cmd` — batch-index markdown docs from `./docs` directory (auto-deletes existing records for the same `_source` before re-indexing)
- `recall_cmd_lowLevel` — direct Milvus retriever test
- `llm_tool_cmd_lowLevel` — low-level LLM + tool binding test

Run any with: `go run ai/cmd/<name>/main.go` (requires `etc/conf.yml` and relevant services).

## Current Tool Inventory

| Tool Name | File | Description |
|-----------|------|-------------|
| `query_prometheus_alerts` | `query_metric_alerts.go` | Query active Prometheus alerts |
| `query_log` (MCP tools) | `query_log.go` | Tencent CLS log query via MCP |
| `query_internal_docs` | `query_internal_docs.go` | RAG retrieval from Milvus knowledge base |
| `get_current_time` | `get_current_time.go` | Current system time (multiple formats) |
| `mysql_crud` | `mysql_crud.go` | Execute SQL against MySQL (with confirmation) |
| `search_file` | `search_file.go` | Search files by name keyword + extension filter |
| DuckDuckGo search | `tools_node.go` | Web search via DuckDuckGo (eino-ext) |
