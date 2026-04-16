---
name: watchtower-dev
description: >-
  Guide for developing the watchTower AI ops platform. Use when adding new Eino
  tools, creating agent workflows, modifying chat/AI-ops controllers, updating
  Prometheus/Milvus/MCP integrations, or asking about watchTower architecture,
  project structure, config, and conventions.
---

# watchTower Development Guide

## Project Overview

watchTower is a Go Gin backend providing AI-powered ops capabilities: chat (including streaming), AI ops alert analysis, knowledge base (RAG via Milvus), with a SuperBizAgent web UI frontend.

**Tech stack**: Go 1.25, Gin, CloudWeGo Eino (LLM/agent framework), Milvus (vector DB), Viper (config), GORM/MySQL.

## Directory Structure

```
watchTower/
├── main.go                          # Gin server entry
├── etc/conf.yml                     # All config (app, LLM, embedding, Milvus, Prometheus, MCP)
├── ai/
│   ├── tools/                       # Eino tool implementations
│   │   ├── query_metric_alerts.go   # Prometheus alerts tool
│   │   ├── query_log.go             # Log query tool (via MCP)
│   │   ├── query_internal_docs.go   # RAG knowledge retrieval tool
│   │   ├── get_current_time.go      # Time utility tool
│   │   └── mysql_crud.go            # Database CRUD tool
│   ├── agent/
│   │   ├── chat_workflow/           # Chat agent (orchestration, prompt, flow, retriever, tools_node)
│   │   ├── plan_execute_replan/     # Plan-Execute-Replan agent for complex AI ops
│   │   └── knowledge_index_workflow/# Knowledge indexing pipeline (load→transform→embed→index)
│   └── cmd/                         # Standalone CLI entry points for testing
├── controller/chat/                 # HTTP handlers (Chat, ChatStream, AIOps, FileUpload)
├── router/                          # Gin route registration
├── common/                          # Shared utilities (config, enum, fileloader, milvus, utils)
├── model/                           # LLM model factory
├── mem/                             # In-memory conversation history
├── middleware/                      # CORS middleware
├── docker/                          # Docker Compose (Milvus, etcd, MinIO)
└── SuperBizAgentFrontend/           # Static HTML/JS chat UI
```

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
            // Implementation
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
- **Chat agent**: `ai/agent/chat_workflow/tools_node.go`
- **Plan-Execute-Replan agent**: `ai/agent/plan_execute_replan/executor.go`

## Adding a New Agent Workflow

Agent workflows live in `ai/agent/`. The two main patterns:

1. **Chat Workflow** (`chat_workflow/`): Single-turn or multi-turn chat with optional tool calls and RAG retrieval. Built using Eino's graph orchestration.

2. **Plan-Execute-Replan** (`plan_execute_replan/`): Complex multi-step tasks. Has a planner (generates steps), executor (runs each step with tools), and replanner (adjusts plan based on results).

To create a new agent, create a subdirectory under `ai/agent/` and expose a `Build*Agent(ctx)` function.

## Config Convention

All config in `etc/conf.yml`, loaded by `common/config`. Access via `config.Conf.*`. When adding new external service integrations, add config fields to the config struct and `conf.yml`.

## API Convention

Routes registered in `router/chatRouter.go` under `/api` group. Controllers in `controller/chat/`. Follow existing patterns: parse request, call agent, return JSON or SSE stream.

## Testing

CLI test entry points under `ai/cmd/*/main.go` allow testing agents and tools independently without starting the full HTTP server.
