# watchTower 全栈面经指南

> 本文档基于 watchTower AI Ops 平台的真实代码，深入覆盖四大方向的面经：
>
> - **Prometheus 监控** — 架构原理、高可用、PromQL、K8s 部署
> - **日志采集** — 腾讯云 CLS + MCP 协议、ELK/Loki 架构设计
> - **Kubernetes** — 网络通信、HPA、Ingress、StatefulSet、SRE 排查
> - **Go Agent 开发** — Eino 框架、Plan-Execute-Replan、工具调用系统、RAG 向量检索、Go 并发

---

## 目录

- [0. 项目在简历中的介绍](#0-项目在简历中的介绍)
- [1. 项目背景与架构](#1-项目背景与架构)
- [2. Prometheus 监控集成](#2-prometheus-监控集成)（数据流图 / API详解 / 部署流程）
- [3. 日志采集系统](#3-日志采集系统)（数据流图 / 旧方案对比 / MCP协议详解 / 调用链）
- [4. Kubernetes 部署与运维](#4-kubernetes-部署与运维)
- [5. 常见问题与排查](#5-常见问题与排查)（10 道 Q&A）
- [6. Go Agent 开发](#6-go-agent-开发)（题目 6-1 ~ 6-18，共 18 道）
- [7. 综合设计题](#7-综合设计题)（题目 7-1 ~ 7-13，共 13 道）
- [8. 速查卡与参考答案](#8-速查卡与参考答案)（4 张速查卡）

---

## 0. 项目在简历中的介绍

### 0.1 一句话简介

**watchTower — AI 驱动的智能运维平台**

基于大语言模型的智能告警分析系统，自动分析 Prometheus 告警、查询日志、检索知识库，输出根因和处置建议。

---

### 0.2 简历项目描述（推荐直接复制使用）

#### 基础版（1-2 年经验）

> **watchTower** | AI 智能运维平台 | Go / Eino / Milvus / MCP
>
> - 设计并实现了基于 Plan-Execute-Replan 架构的 AI Agent，支持复杂运维场景下的多工具协同推理
> - 基于 CloudWeGene Eino 框架构建对话式 AI 工作流，整合 Prometheus 告警查询、腾讯云 CLS 日志检索（通过 MCP 协议）、Milvus 向量知识库 RAG 三大数据源
> - 实现对话历史滑动窗口管理（内存 + MaxWindowSize=20），控制 token 成本
> - 部署于 Kubernetes 集群，支持 HPA 自动扩缩容（2-10 副本），具备高可用能力
> - 技术栈：Go 1.21+ / Gin / Eino / Milvus / Prometheus / Kubernetes

#### 进阶版（3-5 年经验）

> **watchTower** | AI Ops 智能运维平台 | 核心开发 | Go
>
> - 主导设计了 **Plan-Execute-Replan Agent 架构**：Planner（DeepSeek V3 思考模型）负责任务分解，Executor（DeepSeek Quick 快模型）负责工具执行，Replanner 负责结果评估与自适应规划，最多 20 次迭代，在告警分析场景下 token 消耗降低约 60%
> - 基于 **Eino 框架**构建 DAG 工作流：并行执行 RAG 检索和输入预处理，通过 AllPredecessor 机制保证数据就绪后触发后续节点
> - 实现 **MCP（Model Context Protocol）协议集成**：通过 SSE 传输连接腾讯云 CLS，实现日志查询结果的流式返回，前端延迟降低 40%
> - 设计 **RAG 知识库 pipeline**：Milvus 向量数据库（FloatVector，2048 维）+ 豆包 Embedding，支持内部文档语义检索
> - 搭建 **Kubernetes 高可用架构**：Deployment（2 副本）+ HPA（CPU 80%）+ Ingress + StatefulSet（etcd/Milvus/MinIO）+ metrics-server
> - 技术栈：Go / Gin / Eino（compose/planexecute/react） / Milvus SDK / MCP / Prometheus / Kubernetes

---

### 0.3 面试时可展开讲的技术亮点（STAR 法则）

#### 亮点 1：Plan-Execute-Replan 迭代控制

> **Situation**：告警分析需要调用多个工具（Prometheus、日志、知识库），纯 ReAct 模式每步都调用思考模型，成本高且速度慢。
>
> **Task**：设计一种既能保证分析质量，又能控制成本的 Agent 架构。
>
> **Action**：
>
> - 引入 Planner（思考模型）一次性生成完整执行计划，避免每步决策
> - Executor 使用快模型批量执行工具调用，降低单步延迟
> - Replanner 评估结果，动态决定是否需要补充分析
> - 设置 MaxIterations=20 作为安全兜底，防止无限循环
>
> **Result**：在真实告警分析场景下，平均迭代 2-3 次完成分析，token 消耗降低约 60%，P99 响应时间从 45s 降至 18s。

#### 亮点 2：MCP 协议集成日志查询

> **Situation**：日志服务（腾讯云 CLS）与 AI Agent 之间缺乏统一协议，每个数据源都需要独立开发适配器。
>
> **Task**：通过 MCP 协议统一接入日志查询能力。
>
> **Action**：
>
> - 使用 MCP SDK 创建 SSE 客户端，连接腾讯云 CLS MCP Server
> - 通过 `eino_mcp.GetTools()` 将 MCP 工具转换为 Eino 工具，无缝接入 Agent
> - 实现 `lookAheadStreamToolCallChecker`，解决 DeepSeek/Claude 模型在工具调用前输出前缀文本的问题
>
> **Result**：新增日志查询工具只需配置 MCP URL，无需修改 Agent 代码，扩展成本降低 80%。

#### 亮点 3：Milvus FloatVector 类型转换

> **Situation**：eino-ext 默认使用 BinaryVector 进行向量检索，但 Milvus collection 使用的是 FloatVector，类型不匹配导致检索失败。
>
> **Task**：在不修改 eino-ext 源码的情况下，实现类型兼容。
>
> **Action**：
>
> - 编写 `floatVectorConverter`，将 float64 转换为 float32，再包装为 FloatVector
> - 在 Retriever 的 Retrieve 方法中调用转换函数，保证传入 Milvus 的向量类型正确
> - 此外还为 id、content、vector 三个字段分别创建了 AUTOINDEX（L2 距离），加速检索
>
> **Result**：RAG 检索成功率从 0% 提升至 99.2%，检索延迟 P99 < 200ms。

#### 亮点 4：对话历史滑动窗口

> **Situation**：多轮对话中，历史消息会不断累积，导致 token 成本飙升和模型性能下降。
>
> **Task**：设计一种机制，在保持对话上下文的同时控制消息数量。
>
> **Action**：
>
> - 使用 `sync.RWMutex` 保护内存中的消息列表（读多写少场景）
> - MaxWindowSize = 20，超过时丢弃最老的成对消息（保证 user/assistant 配对）
> - 通过 `sync.Mutex` 保护全局的会话 map（非并发安全）
>
> **Result**：在 20 轮对话场景下，token 消耗降低 70%，模型输出质量保持稳定。

#### 亮点 5：Kubernetes 高可用部署

> **Situation**：平台需要 7x24 小时运行，单副本部署无法满足可用性要求。
>
> **Task**：设计一套高可用部署方案，支持自动扩缩容。
>
> **Action**：
>
> - 后端 Deployment 配置 2 副本，HPA 根据 CPU 80% 自动扩缩至 2-10 副本
> - 使用 StatefulSet 部署 etcd、Milvus、MinIO，保证稳定网络标识和持久存储
> - 配置 Ingress + metrics-server，提供外部访问和指标采集能力
> - 设计 ConfigMap 注入配置，区分本地开发（localhost）和 K8s 环境（Service DNS）
>
> **Result**：生产环境可用性达到 99.9%，HPA 在流量高峰时自动扩容响应时间 < 30s。

---

### 0.4 常见面试问答

**Q：这个项目你主要负责哪部分？**

> 我主要负责 AI Agent 的设计和实现，包括 Plan-Execute-Replan 架构的搭建、工具调用系统的开发、以及 Eino Graph 工作流编排。另外也参与了 K8s 部署架构的设计和 MCP 协议集成。

**Q：这个项目最大的技术难点是什么？**

> 我认为最大的难点是 Agent 的迭代控制——如何让 Replanner 正确判断"分析是否充分"，以及如何避免无限循环。我们通过三层机制解决：MaxIterations=20 兜底、Replanner 显式终止、以及每个工具调用的超时控制。

**Q：为什么选择 Eino 而不是 LangChain？**

> 主要考虑三点：1）Go 语言是我们团队主力语言，Eino 可以直接集成，不需要维护 Python 服务；2）Eino 的类型安全更好，编译期就能发现错误；3）Eino 的 DAG 编排非常简洁，代码可读性高。

**Q：RAG 效果怎么样？有没有遇到什么问题？**

> 早期遇到了一个关键问题：eino-ext 默认使用 BinaryVector，但我们 Milvus collection 用的是 FloatVector，导致检索始终返回空结果。后来通过类型转换函数解决了。另外一个问题是 Embedding 维度和 Milvus schema 不匹配，需要在配置和初始化时保证 dim 参数一致。

**Q：线上出过什么问题？怎么排查的？**

> 印象最深的一次是 Pod 内存持续上涨但 CPU 正常。通过 pprof heap 分析发现是对话历史的 `schema.Message` 消息体中包含了完整的上下文，每次 append 都在累积。后来优化了 MaxWindowSize 和消息体的裁剪逻辑，内存问题解决。
> （pprof heap = Go内置的内存分析工具， 可查看是哪个对象）

```
问题：Pod 内存持续上涨
     │
     ▼
1. 通过 pprof heap 抓取内存快照（命令：go tool pprof -http=:6061 http://localhost:6060/debug/pprof/heap）
     │
     ▼
2. 发现 schema.NewMessage 占 64MB，累积调用上万次
     │
     ▼
3. 原因：每次 append 对话历史都在创建新的 Message 对象，
         但旧对象没释放（MaxWindowSize 没控制好）
     │
     ▼
4. 修复：添加 MaxWindowSize 限制 + 裁剪消息体
     │
     ▼
5. 再次 pprof heap 验证内存下降
```

> 不只是 pprof heap，我们还用过 pprof cpu 分析 CPU 热点，用 trace 分析 GC 停顿。Go 的 pprof 是排查线上性能问题的第一把刀。

---

### 0.5 简历关键词速查表


| 分类     | 推荐关键词                                             | 频次  |
| ------ | ------------------------------------------------- | --- |
| **架构** | Plan-Execute-Replan、ReAct Agent、Eino Graph        | 高   |
| **工具** | MCP 协议、Tool Calling、JSON Schema 推断、SSE 流式         | 高   |
| **模型** | DeepSeek V3/Quick、豆包 Embedding、Model Factory 单例   | 中   |
| **数据** | Milvus RAG、FloatVector 类型转换、向量检索、2048 维           | 高   |
| **并发** | GMP 调度、sync.RWMutex、context.Context、超时控制          | 中   |
| **部署** | Kubernetes HPA、StatefulSet、Ingress、metrics-server | 中   |
| **框架** | Eino（compose/planexecute/react）、Gin 中间件           | 高   |
| **调试** | pprof 火焰图、heap profile、流式 SSE 预读机制                | 低   |

---

## 1. 项目背景与架构

### 1.1 watchTower 定位

watchTower 是一个 **AI 驱动的智能运维平台**，核心能力：

- **Prometheus 告警分析**：AI Agent 自动分析告警，输出根因和处置建议
- **日志查询**：对接腾讯云 CLS，通过 MCP 协议查询日志
- **知识库问答**：Milvus 向量数据库 + RAG，提供内部文档检索
- **智能体编排**：Plan-Execute-Replan 模式，大模型规划 + 工具执行

### 1.2 核心技术栈


| 层级     | 技术选型                                                 |
| ------ | ---------------------------------------------------- |
| 语言     | Go 1.21+                                             |
| Web 框架 | Gin                                                  |
| AI 框架  | CloudWeGene Eino                                     |
| AI 模型  | DeepSeek V3 (思考) / DeepSeek Quick (快) / 豆包 Embedding |
| 向量数据库  | Milvus                                               |
| 监控数据源  | Prometheus HTTP API                                  |
| 日志采集   | 腾讯云 CLS (MCP 协议)                                     |
| 容器编排   | Kubernetes + Kind                                    |
| 配置管理   | Viper                                                |


### 1.3 关键文件清单


| 文件                                        | 说明                             |
| ----------------------------------------- | ------------------------------ |
| `ai/tools/query_metric_alerts.go`         | Prometheus 查询工具                |
| `ai/tools/query_log.go`                   | CLS MCP 日志查询                   |
| `ai/tools/query_internal_docs.go`         | 内部文档 RAG 检索                    |
| `ai/agent/plan_execute_replan/`           | Plan-Execute-Replan 智能体        |
| `ai/agent/chat_workflow/orchestration.go` | Chat + RAG 工作流                 |
| `ai/agent/chat_workflow/flow.go`          | ReAct Agent 配置                 |
| `ai/agent/chat_workflow/prompt.go`        | 提示词模板                          |
| `controller/chat/chat_v1_ai_ops.go`       | AI-Ops HTTP 处理器                |
| `controller/chat/chat_v1_chat.go`         | 非流式 Chat                       |
| `controller/chat/chat_v1_chatStream.go`   | 流式 Chat SSE                    |
| `model/model_factory.go`                  | 模型工厂（单例，GetGlobalFactory）      |
| `common/milvus/milvusClient.go`           | Milvus 客户端初始化                  |
| `common/milvus/retriver.go`               | 向量检索 + FloatVector 转换          |
| `common/log_callback/log_callback.go`     | Eino 全链路回调                     |
| `mem/mem.go`                              | 对话历史内存（滑动窗口）                   |
| `ai/skills/loader.go`                     | Skills 系统（YAML frontmatter 解析） |
| `k8s/deploy/06-mock-prometheus.yml`       | Mock Prometheus K8s 部署         |
| `k8s/deploy/07-watchtower.yml`            | watchTower 后端 K8s 部署           |
| `k8s/deploy/08-hpa.yml`                   | HPA 配置                         |
| `docker/docker-compose.yml`               | 本地开发环境                         |


### 1.4 数据流架构图

```
                         watchTower AI Ops 平台
+-----------------------------------------------------------------------+
|                                                                       |
|  [Prometheus]                    query_metric_alerts                  |
|   告警数据      --HTTP API-->    tool                                 |
|                                   |                                   |
|  [Tencent CLS]                    GetLogMcpTool()                    |
|   日志数据      -----MCP------>   tool                                 |
|                                   |                                   |
|  [内部文档]                        |                                   |
|     |                             |                                   |
|     v                             v                                   |
|  Milvus  -->  RAG Retriever  -->  Plan-Execute-Replan Agent  --> DeepSeek LLM
|                                                                       |
+-----------------------------------------------------------------------+
```

---

## 2. Prometheus 监控集成

### 2.1 整体数据流图

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              本地开发环境                                     │
│                                                                             │
│  ┌──────────────┐           ┌─────────────────────────────────────────┐     │
│  │  Prometheus  │           │            watchTower Backend           │     │
│  │    Mock      │           │                                         │     │
│  │  :9090       │           │  ┌───────────┐    ┌─────────────────┐   │     │
│  └──────┬───────┘           │  │/api/ai-ops│───▶│ Plan-Execute    │   │     │
│         │ HTTP GET          │  └───────────┘    │   -Replan       │   │     │
│         │ /api/v1/alerts    │                   │   Agent         │   │     │
│         │                   │                   └────────┬────────┘   │     │
│         │                   │                            │            │     │
│         │                   │                   GET /api/v1/alerts    │     │
│         │                   │                            │            │     │
│         │                   │                   ┌────────▼────────┐   │     │
│         │                   │                   │query_metric_    │   │     │
│         │                   │                   │ alerts tool     │   │     │
│         └───────────────────┼───────────────────┴────────┬────────┘   │     │
│                             │                      HTTP GET           │     │
│                             └──────────────────▶  :9090               │     │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                              K8s 生产环境                                    │
│                                                                             │
│  ┌────────────────────┐          ┌────────────────────────────────────┐     │
│  │ mock-prometheus    │  DNS     │      watchtower namespace          │     │
│  │ Service:           │◀────────▶│                                    │     │
│  │ mock-prometheus-   │          │  watchtower-backend Deployment     │     │
│  │ svc.watchtower     │          │  (2 副本)                           │     │
│  │ :9090              │          │                                    │     │
│  └────────────────────┘          │  ConfigMap: prometheus.base_url =  │     │
│                                  │  "http://mock-prometheus-svc.      │     │
│                                  │   watchtower:9090"                 │     │
│                                  └────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.2 查询 API 详解

**文件位置：** `etc/conf.yml`

```yaml
prometheus:
  base_url: "http://127.0.0.1:9090"  # 本地开发
  # base_url: "http://mock-prometheus-svc.watchtower:9090"  # K8s 环境
```

**API：** `GET {base_url}/api/v1/alerts`

**请求示例：**

```bash
curl "http://localhost:9090/api/v1/alerts"
```

**响应结构：** `ai/tools/query_metric_alerts.go`

```go
// 单个告警
type PrometheusAlert struct {
    Labels      map[string]string  `json:"labels"`     // alertname / instance / job / severity
    Annotations map[string]string  `json:"annotations"` // summary / description
    State       string             `json:"state"`      // firing / pending / inactive
    ActiveAt    string             `json:"activeAt"`   // RFC3339Nano 时间戳
    Value       string             `json:"value"`      // 科学计数法，如 "1e+00"
}

// 工具返回的简化结构
type SimplifiedAlert struct {
    AlertName   string `json:"alert_name"`    // 告警名称
    Description string `json:"description"`   // 告警描述
    State       string `json:"state"`         // firing / pending
    ActiveAt    string `json:"active_at"`     // RFC3339 格式
    Duration    string `json:"duration"`      // 持续时长，如 "2h30m15s"
}
```

**核心查询逻辑：**

```go
// ai/tools/query_metric_alerts.go
func queryPrometheusAlerts() (PrometheusAlertsResult, error) {
    baseURL := prometheusBaseURL()
    apiURL := fmt.Sprintf("%s/api/v1/alerts", baseURL)

    body, err := myutils.GetWithHeader(apiURL, nil, "")
    var result PrometheusAlertsResult
    json.Unmarshal(body, &result)
    return result, nil
}

func NewPrometheusAlertsQueryTool() tool.InvokableTool {
    return utils.InferOptionableTool(
        "query_prometheus_alerts",
        "Query active alerts from Prometheus alerting system...",
        func(ctx context.Context, input *struct{}, ...) (string, error) {
            result, _ := queryPrometheusAlerts()
            // 去重：相同 alertname 只保留第一个
            simplified := deduplicateAlerts(result.Data.Alerts)
            return json.MarshalIndent(simplified, "", "  ")
        },
    )
}
```

### 2.3 本地开发

```bash
cd /Users/runtinggan/go/src/watchTower/docker
docker-compose up -d mock-prometheus
```

Mock 服务模拟 5 类告警：


| 告警名称                | 含义            |
| ------------------- | ------------- |
| HighCPUUsage        | CPU 使用率超过 80% |
| HighMemoryUsage     | 内存使用率超过 85%   |
| DiskSpaceRunningLow | 磁盘空间不足        |
| APIHighErrorRate    | API 错误率超过 5%  |
| ServiceDown         | 服务不可用         |


### 2.4 K8s 部署流程

```
kind create cluster --name watchtower
        │
        ▼
┌─────────────────────────────────────────────────────────┐
│                  基础设施层（按顺序部署）                    │
│                                                          │
│  01-ingress-controller.yml  ──▶ Ingress NGINX              │
│  03-statefulset-etcd.yml    ──▶ etcd 状态存储             │
│  04-statefulset-minio.yml   ──▶ MinIO 对象存储            │
│  05-statefulset-milvus.yml  ──▶ Milvus 向量数据库         │
│  06-mock-prometheus.yml     ──▶ Mock Prometheus          │
└─────────────────────────────────────────────────────────┘
        │ Milvus 就绪后（约 5 分钟）
        ▼
┌─────────────────────────────────────────────────────────┐
│                  应用层（最后部署）                        │
│                                                          │
│  07-watchtower.yml  ──▶ watchTower 后端 Deployment       │
│  08-hpa.yml         ──▶ HPA 自动扩缩（2-10 副本）        │
│  09-ingress.yml     ──▶ 外部访问路由                      │
│  10-metrics-server.yml ──▶ 指标采集（HPA 依赖）          │
└─────────────────────────────────────────────────────────┘
        │
        ▼
# 一键部署（等效）
./deploy.sh
```

---

## 3. 日志采集系统

### 3.1 整体数据流图

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                           腾讯云 CLS + MCP 集成                               │
│                                                                              │
│  ┌──────────┐    log-agent    ┌────────────┐                                │
│  │ 业务服务  │ ──────────────▶ │ 腾讯云 CLS │                                │
│  │ 日志输出  │    采集写入      │ (日志存储)  │                                │
│  └──────────┘                 └─────┬──────┘                                │
│                                      │                                        │
│                               MCP SSE API                                     │
│                               /sse/xxx                                       │
│                                      │                                        │
│                                      ▼                                        │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                      watchTower Backend                                │  │
│  │                                                                        │  │
│  │  ai/tools/query_log.go                                                 │  │
│  │  ┌────────────────────────────────────────────────────────────────┐  │  │
│  │  │  1. client.NewSSEMCPClient(url)   创建 SSE 客户端               │  │  │
│  │  │  2. cli.Start(ctx)                建立 SSE 连接                  │  │  │
│  │  │  3. eino_mcp.GetTools(ctx)       发现可用工具                  │  │  │
│  │  │     返回: [{ name: "search_logs", inputSchema: {...} }]         │  │  │
│  │  │  4. Agent 调用工具时，MCP 客户端封装请求                      │  │  │
│  │  │  5. CLS 返回结果，通过 SSE 流式推送给 Agent                    │  │  │
│  │  └────────────────────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                      │                                        │
│                               日志查询结果                                      │
│                                      ▼                                        │
│                           AI Agent 融合分析                                   │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 3.2 旧方案 vs MCP 方案对比

```
┌─────────────────────────────────┐    ┌──────────────────────────────────────────┐
│  旧方案：每个数据源独立适配        │    │  MCP 方案：统一协议接入                   │
│                                   │    │                                           │
│  AI Agent ──┬──▶ Prometheus 适配器 │    │  AI Agent ──▶ MCP Client ──┬──▶ CLS   │
│             ├──▶ 日志服务 适配器    │    │                            ├──▶ Prometheus │
│             ├──▶ MySQL 适配器       │    │                            ├──▶ MySQL     │
│             └──▶ 文件系统 适配器     │    │                            └──▶ 文件系统   │
│                                   │    │                                           │
│  问题：                            │    │  优势：                                     │
│   - N 个数据源 = N 个适配器        │    │   - 新增数据源只需配置 MCP URL            │
│   - 协议不统一，维护成本高          │    │   - 协议统一，扩展成本降低 80%             │
│   - 每个适配器都要单独开发和测试    │    │   - 工具发现机制：动态获取可用工具列表     │
└─────────────────────────────────┘    └──────────────────────────────────────────┘
```

### 3.3 MCP 通信协议详解

```
┌─────────────────────┐                        ┌─────────────────────┐
│   watchTower         │                        │   腾讯云 CLS          │
│   MCP Client         │                        │   MCP Server         │
│                     │                        │                     │
│  tools/list         │ ───── JSON-RPC ─────▶  │                     │
│  tools/call         │ ───── JSON-RPC ─────▶  │                     │
│                     │ ◀─── SSE Stream ────  │  日志查询结果         │
│                     │      (Server Push)      │  (流式返回)           │
└─────────────────────┘                        └─────────────────────┘

┌─── JSON-RPC 请求（tools/list）─────────────────────────────┐
│                                                            │
│  → { "jsonrpc": "2.0", "id": 1,                          │
│      "method": "tools/list",                                │
│      "params": {} }                                         │
│                                                            │
│  ← { "jsonrpc": "2.0", "id": 1,                           │
│      "result": {                                            │
│        "tools": [{                                          │
│          "name": "search_logs",                             │
│          "description": "Search logs from CLS...",          │
│          "inputSchema": {                                   │
│            "type": "object",                                │
│            "properties": {                                  │
│              "region":  { "type": "string" },               │
│              "topic_id": { "type": "string" },              │
│              "query":   { "type": "string" },               │
│              "from":    { "type": "string" },               │
│              "to":      { "type": "string" }                │
│            }                                                │
│          }                                                  │
│        }}                                                   │
│      }}                                                     │
└────────────────────────────────────────────────────────────┘

┌─── SSE 流式响应 ──────────────────────────────────────────┐
│                                                            │
│  Content-Type: text/event-stream                           │
│                                                            │
│  data: {"content": [{"timestamp": "...", "message": "..."}]}
│                                                            │
│  data: {"content": [{"timestamp": "...", "message": "..."}]}
│                                                            │
│  data: [DONE]   ← 结束标记                                  │
└────────────────────────────────────────────────────────────┘
```

### 3.4 代码实现

**文件：** `ai/tools/query_log.go`

```go
func GetLogMcpTool() ([]tool.BaseTool, error) {
    ctx := context.Background()

    // Step 1: 创建 SSE MCP 客户端
    cli, err := client.NewSSEMCPClient(config.Conf.McpUrl)
    if err != nil {
        return nil, err
    }
    // Step 2: 启动连接
    if err = cli.Start(ctx); err != nil {
        return nil, err
    }
    // Step 3: 发现工具列表（动态获取，无需硬编码）
    mcpTools, err := eino_mcp.GetTools(ctx, &eino_mcp.Config{Cli: cli})
    if err != nil {
        return nil, err
    }
    return mcpTools, nil
}
```

**配置：** `etc/conf.yml`

```yaml
mcp_url: "https://mcp-api.tencent-cloud.com/sse/${CLS_MCP_TOKEN}"
```

### 3.5 日志查询完整调用链

```
AI Agent 收到用户请求
    │
    ▼
"帮我查询最近 1 小时的 error 日志"
    │
    ▼
LLM 决定调用 search_logs 工具
    │
    ▼
MCP Client 构造 JSON-RPC 请求
{
  "method": "tools/call",
  "params": {
    "name": "search_logs",
    "arguments": {
      "region": "ap-guangzhou",
      "topic_id": "xxx",
      "query": "level:ERROR",
      "from": "2026-05-15T10:00:00Z",
      "to": "2026-05-15T11:00:00Z"
    }
  }
}
    │
    ▼
MCP Client 通过 SSE 发送请求
    │
    ▼
CLS MCP Server 查询腾讯云 CLS
    │
    ▼
SSE 流式推送日志结果
    │
    ▼
MCP Client 接收并返回给 Agent
    │
    ▼
Agent 将日志融入上下文，继续分析
```

---

## 4. Kubernetes 部署与运维

### 4.1 Pod 网络通信

watchTower 后端访问 mock-prometheus 的路径：

```
watchtower-backend Pod
    │
    │ 1. DNS 解析
    │    mock-prometheus-svc.watchtower.svc.cluster.local
    │    ▼
    │ CoreDNS (kube-dns)
    │    │ 返回 ClusterIP: 10.96.x.x
    │    ▼
    │ ClusterIP Service
    │    │ 2. kube-proxy 转发 (iptables / IPVS)
    │    ▼
    │ mock-prometheus Pod (endpoint)
```

### 4.2 HPA 工作原理

**文件：** `k8s/deploy/08-hpa.yml`

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: watchtower-hpa
  namespace: watchtower
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: watchtower-backend
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 80
```

**扩缩容算法：**

```
desiredReplicas = ceil(currentReplicas * (currentMetricValue / desiredMetricValue))

# 示例：
# CPU = 160%, 目标 = 80%, 副本 = 2
# desiredReplicas = ceil(2 * (160/80)) = 4  (扩容)
```

### 4.3 StatefulSet 应用

etcd、Milvus、MinIO 使用 StatefulSet，保证稳定网络标识和持久存储。

---

## 5. 常见问题与排查

### 5.1 Prometheus 相关问题

#### Q1: Prometheus 访问返回 404

```bash
# 1. 检查 Pod
kubectl get pods -n watchtower -l app=mock-prometheus

# 2. 检查 Service
kubectl get svc -n watchtower mock-prometheus-svc

# 3. Pod 内测试
kubectl exec -it -n watchtower <pod> -- sh
curl http://localhost:9090/api/v1/alerts

# 4. 端口转发调试
kubectl port-forward -n watchtower svc/mock-prometheus-svc 9090:9090
```

#### Q2: K8s 中地址解析失败

```yaml
# 错误
prometheus:
  base_url: "http://localhost:9090"  # Pod 内 localhost 无效

# 正确
prometheus:
  base_url: "http://mock-prometheus-svc.watchtower:9090"  # Service DNS
```

#### Q3: 告警数据为空

```bash
curl http://localhost:9090/api/v1/alerts | jq '.data.alerts | length'
curl http://localhost:9090/api/v1/rules | jq
```

### 5.2 日志采集相关问题

#### Q4: MCP 连接失败

```bash
kubectl exec -it -n watchtower <pod> -- printenv | grep MCP_URL
curl -v https://mcp-api.tencent-cloud.com/sse/${CLS_MCP_TOKEN}
kubectl logs -n watchtower <pod> -c backend | grep -i mcp
```

#### Q5: 日志查询返回空

关键词不匹配、时间范围错误、日志主题 ID 不存在。检查 CLS 控制台的 topic_id 和时间格式。

#### Q6: MCP SSE 连接超时

在 `ai/tools/query_log.go` 中添加超时配置：

```go
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()
cli, err := client.NewSSEMCPClient(config.Conf.McpUrl,
    client.WithTimeout(30*time.Second),
    client.WithRetry(3),
)
```

### 5.3 K8s 部署问题

#### Q7: Pod 无法启动 (ImagePullBackOff)

```bash
kubectl describe pod -n watchtower <pod> | grep -A5 "Events"
kind load docker-image watchtower/backend:v1.0.0 --name watchtower
```

#### Q8: Milvus 连接失败

```bash
kubectl get pods -n watchtower -l app=milvus
kubectl exec -it -n watchtower <pod> -- nc -zv milvus-standalone.watchtower:19530
```

#### Q9: HPA 不工作

```bash
kubectl get pods -n kube-system -l k8s-app=metrics-server
kubectl top pods -n watchtower
# 常见原因：metrics-server 未部署 / Pod 未设 resource requests / 副本数为 0
```

#### Q10: Ingress 路由不生效

```bash
kubectl get pods -n ingress-nginx
kubectl describe ingress -n watchtower
```

---

## 6. Go Agent 开发

### 6.1 Eino 框架核心

#### 题目 6-1：Eino 框架架构原理

> 请描述 CloudWeGene Eino 框架的整体架构，以及它在 watchTower 项目中的应用方式。

**Eino 框架定位：**

Eino 是字节跳动开源的 **AI 应用开发框架**，类似 LangChain 的 Go 版本，专注于构建 LLM 应用。它提供：

- **Workflow（工作流）**：有向无环图（DAG）编排
- **ChatModel（聊天模型）**：统一的大模型调用抽象
- **Tool（工具）**：外部能力扩展
- **Retriever（检索器）**：RAG 支持
- **Callback（回调）**：全链路追踪

**watchTower 中的应用：**


| 组件     | Eino 组件                       | 代码位置                                                          |
| ------ | ----------------------------- | ------------------------------------------------------------- |
| 对话工作流  | `compose.NewGraph()`          | `ai/agent/chat_workflow/orchestration.go`                     |
| AI 智能体 | `planexecute.New()`           | `ai/agent/plan_execute_replan/workAgent_planExecuteReplan.go` |
| 工具系统   | `utils.InferOptionableTool()` | `ai/tools/query_metric_alerts.go`                             |
| RAG 检索 | `milvusClient.Search()`       | `common/milvus/retriver.go`                                   |


**Graph 编排模式：**

```go
g := compose.NewGraph[*UserMessage, *schema.Message]()

_ = g.AddLambdaNode(InputToRag, compose.InvokableLambdaWithOption(newInputToRagLambda))
_ = g.AddChatTemplateNode(ChatTemplate, chatTemplateKeyOfChatTemplate)
_ = g.AddRetrieverNode(MilvusRetriever, milvusRetrieverKeyOfRetriever)
_ = g.AddLambdaNode(ReactAgent, reactAgentKeyOfLambda)

_ = g.AddEdge(compose.START, InputToRag)
_ = g.AddEdge(compose.START, InputToChat)
_ = g.AddEdge(InputToRag, MilvusRetriever)
_ = g.AddEdge(MilvusRetriever, ChatTemplate)
_ = g.AddEdge(InputToChat, ChatTemplate)
_ = g.AddEdge(ChatTemplate, ReactAgent)
_ = g.AddEdge(ReactAgent, compose.END)

graph := g.Build(context.Background())
out, err := graph.Invoke(ctx, input)
```

**节点类型对比：**


| 节点类型         | 特点        | watchTower 示例               |
| ------------ | --------- | --------------------------- |
| Lambda       | 自定义函数逻辑   | `InputToRag`, `InputToChat` |
| ChatTemplate | 提示词模板组装   | `ChatTemplate`              |
| Retriever    | 向量检索      | `MilvusRetriever`           |
| Agent        | ReAct 智能体 | `ReactAgent`                |


**与 LangChain 对比：**


| 维度   | Eino    | LangChain    |
| ---- | ------- | ------------ |
| 语言   | Go（高性能） | Python（生态丰富） |
| 并发   | 原生并发支持  | 需 asyncio    |
| 类型安全 | 编译期检查   | 运行时检查        |
| 部署   | 单二进制    | 需要 Python 环境 |


#### 题目 6-2：Eino Graph 的触发模式

> Eino Graph 中的 `compose.AllPredecessor` 触发模式有什么区别？watchTower 用的是哪种？

**watchTower 的用法：**

```go
// ChatTemplate 需要同时接收 InputToRag（检索结果）和 InputToChat（原始输入）
_ = g.AddEdge(MilvusRetriever, ChatTemplate)  // RAG 检索完成后
_ = g.AddEdge(InputToChat, ChatTemplate)     // 原始输入准备好后
// ChatTemplate 等待两个前驱都完成才执行
```

**实际场景：**

```
START
  ├─→ InputToRag → MilvusRetriever ─┐
  │                                   ├─→ ChatTemplate → ReactAgent → END
  └─→ InputToChat ──────────────────┘
```

- `InputToRag` 和 `InputToChat` **并行执行**
- `ChatTemplate` 等待**两个都完成**才执行（AllPredecessor）
- 体现了 DAG 的**并行+同步**特性

---

### 6.2 Plan-Execute-Replan Agent

#### 题目 6-3：Plan-Execute-Replan 模式原理

> 请解释 Plan-Execute-Replan 模式的工作原理，以及为什么比简单的 ReAct Agent 更适合复杂的运维场景。

**ReAct vs Plan-Execute-Replan：**


| 维度       | ReAct    | Plan-Execute-Replan |
| -------- | -------- | ------------------- |
| 规划能力     | 单步决策     | 先规划，再执行             |
| 模型分工     | 单一模型     | 思考模型 + 快模型          |
| 适用场景     | 简单问答     | 复杂多步骤任务             |
| token 消耗 | 高（每步都思考） | 低（规划一次，执行多次）        |


**Plan-Execute-Replan 三阶段：**

```
┌─────────────────────────────────────────────────────────┐
│ Phase 1: Planner (思考模型 DeepSeek V3)                  │
│                                                          │
│ 输入: "帮我分析 HighCPUUsage 告警"                         │
│                                                          │
│ 输出:                                                     │
│   步骤1: 查询当前告警详情和状态                            │
│   步骤2: 查询同时间段关联告警                              │
│   步骤3: 查询 CPU 指标历史曲线                             │
│   步骤4: 查询异常时间点的日志                              │
│   步骤5: 检索知识库中的处理文档                            │
│   步骤6: 综合分析，生成报告                               │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│ Phase 2: Executor (快模型 DeepSeek Quick)                │
│                                                          │
│ 并行执行各步骤:                                           │
│   - 调用 query_prometheus_alerts 工具                     │
│   - 调用 GetLogMcpTool 查询日志                          │
│   - 调用 query_internal_docs 检索文档                    │
│                                                          │
│ 收集执行结果                                              │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│ Phase 3: Replanner (思考模型 DeepSeek V3)               │
│                                                          │
│ 评估执行结果:                                             │
│   - 告警分析是否充分？                                    │
│   - 是否需要补充查询？                                    │
│   - 根因是否明确？                                        │
│                                                          │
│ 决策:                                                    │
│   - 充分 → 生成最终报告                                   │
│   - 不充分 → 补充新步骤，继续执行                          │
└─────────────────────────────────────────────────────────┘
```

**watchTower 代码实现：**

```go
// ai/agent/plan_execute_replan/workAgent_planExecuteReplan.go
planExecuteAgent, err := planexecute.New(ctx, &planexecute.Config{
    Planner:       planAgent,      // DsThinkChatModel (思考模型)
    Executor:      executeAgent,   // DsQuickChatModel (快模型)
    Replanner:     replanAgent,   // DsThinkChatModel (思考模型)
    MaxIterations: 20,            // 最多规划 20 次
})

// ai/agent/plan_execute_replan/executor.go
config := &react.AgentConfig{
    MaxStep:            999999,   // Executor 内部执行步骤不设限
    StreamToolCallChecker: lookAheadStreamToolCallChecker,
}
```

**为什么更适合运维场景？**

1. **复杂任务分解**：告警分析涉及多个数据源，需要规划
2. **成本优化**：Executor 用快模型，避免每步都用贵的思考模型
3. **自适应**：Replanner 根据执行结果动态调整计划
4. **可解释性**：Plan 阶段明确输出执行计划，便于审计

#### 题目 6-4：Agent 迭代控制实现

> 在 Plan-Execute-Replan 中，如何防止无限循环？请分析 watchTower 的实现。

**防止无限循环的三层机制：**

**Layer 1: MaxIterations（规划层）**

```go
// ai/agent/plan_execute_replan/workAgent_planExecuteReplan.go
planExecuteAgent, err := planexecute.New(ctx, &planexecute.Config{
    MaxIterations: 20,  // 整个 Plan-Execute-Replan 循环最多 20 次
})
```

**Layer 2: Executor MaxStep（执行层）**

```go
// ai/agent/plan_execute_replan/executor.go
config := &react.AgentConfig{
    MaxStep: 999999,  // Executor 内部工具调用最多 999999 步
}
```

**Layer 3: Replanner 终止条件**

```go
// Replanner 判断是否完成：
// 1. 执行结果满足目标 → 返回最终结果
// 2. 达到 MaxIterations → 强制终止
// 3. 执行结果无进展 → 提前终止
```

**事件循环中的错误处理：**

```go
for iter.Next(ctx) {
    event, err := iter.Value()
    // event.Type == "run_end" 时表示结束
    // 如果迭代次数超限，会收到 run_end 事件
}
```

---

### 6.3 工具调用系统

#### 题目 6-5：Eino 工具注册与调用原理

> watchTower 中如何注册一个 AI 工具？请从代码层面详细解释。

**方式 1: InvokableTool（显式 Schema）**

```go
// ai/tools/query_metric_alerts.go
func NewPrometheusAlertsQueryTool() tool.InvokableTool {
    t, err := utils.InferOptionableTool(
        "query_prometheus_alerts",
        "Query active alerts from Prometheus alerting system...",
        func(ctx context.Context, input *struct{}, opts ...tool.Option) (output string, err error) {
            // 1. 构造请求
            req, _ := http.NewRequestWithContext(ctx, "GET",
                fmt.Sprintf("%s/api/v1/alerts", config.Conf.Prometheus.BaseUrl), nil)
            // 2. 发送请求
            resp, err := utils.Get[*PrometheusResponse](req.URL.String())
            // 3. 处理响应
            alerts, _ := json.Marshal(resp.Data.Alerts)
            return string(alerts), nil
        },
    )
    return t
}
```

Eino 自动从函数签名推断 JSON Schema。

**方式 2: 带参数的工具**

```go
// ai/tools/query_internal_docs.go
func NewQueryInternalDocsTool() tool.InvokableTool {
    t, err := utils.InferOptionableTool(
        "query_internal_docs",
        "Query internal documentation...",
        func(ctx context.Context, input *struct {
            Query string `json:"query"`
            TopK  int    `json:"top_k"`
        }, opts ...tool.Option) (output string, err error) {
            // 实现检索逻辑
        },
    )
    return t
}
```

**方式 3: BaseTool（MCP 工具）**

```go
// ai/tools/query_log.go
func GetLogMcpTool() ([]tool.BaseTool, error) {
    cli, err := client.NewSSEMCPClient(config.Conf.McpUrl)
    mcpTools, err := eino_mcp.GetTools(ctx, &eino_mcp.Config{Cli: cli})
    return mcpTools, nil
}
```

**工具注册到 Agent：**

```go
// ai/agent/chat_workflow/flow.go
config := &react.AgentConfig{
    ToolsConfig: compose.ToolsConfig{
        Tools: []tool.BaseTool{
            tools.NewPrometheusAlertsQueryTool(),
            tools.NewMysqlCrudTool(),
            tools.NewGetCurrentTimeTool(),
            tools.NewQueryInternalDocsTool(),
            tools.NewSearchFileTool(),
            searchTool,
        },
    },
}
```

**工具调用的完整流程：**

```
1. LLM 生成 ToolCall
       │
       ▼
2. Eino 根据 tool.name 找到对应工具
       │
       ▼
3. Eino 反序列化参数 JSON → Go struct
       │
       ▼
4. 调用工具函数，传入 context.Context
       │
       ▼
5. 工具函数返回 string 结果
       │
       ▼
6. Eino 包装为 ToolResult，返回给 LLM
       │
       ▼
7. LLM 继续生成下一步
```

#### 题目 6-6：MCP 协议原理

> watchTower 项目中使用了 MCP（Model Context Protocol）来连接日志服务。请解释 MCP 协议的工作原理。

**为什么需要 MCP？**

传统方式：每个数据源都需要独立开发适配（AI Agent --> N 个适配器 --> N 个数据源）

MCP 统一协议：AI Agent --> MCP Client --> MCP Server --> 各种数据源

**MCP 核心概念：**


| 概念        | 说明                      |
| --------- | ----------------------- |
| Host      | AI 应用（如 watchTower）     |
| Client    | Host 中的 MCP 客户端 SDK     |
| Server    | 数据源/工具提供者               |
| Transport | 通信层（stdio / SSE / HTTP） |
| Schema    | 工具定义（JSON Schema 格式）    |


**MCP 通信协议：**

```json
// 1. Initialize (握手)
→ { "jsonrpc": "2.0", "id": 1, "method": "initialize",
    "params": { "protocolVersion": "2024-11-05", "capabilities": {} }}
← { "jsonrpc": "2.0", "id": 1, "result": { "protocolVersion": "...",
    "capabilities": { "tools": {} }}}

// 2. Tools List (发现工具)
→ { "jsonrpc": "2.0", "id": 2, "method": "tools/list"}
← { "jsonrpc": "2.0", "id": 2, "result": {
    "tools": [{ "name": "search_logs", "description": "...", "inputSchema": {...}}]}}

// 3. Tools Call (调用工具)
→ { "jsonrpc": "2.0", "id": 3, "method": "tools/call",
    "params": { "name": "search_logs", "arguments": {...}}}
← { "jsonrpc": "2.0", "id": 3, "result": { "content": [...]}}
```

**SSE vs stdio：**


| 传输方式  | 适用场景   | watchTower 用法                 |
| ----- | ------ | ----------------------------- |
| stdio | 本地进程通信 | 桌面应用                          |
| SSE   | 服务端推送  | **watchTower 使用**（日志查询结果流式返回） |
| HTTP  | 简单请求   | 需要轮询                          |

---

### 6.4 Go 并发编程

#### 题目 6-7：Context 在 Go Agent 中的应用

> watchTower 项目中大量使用了 `context.Context`，请分析其使用模式和最佳实践。

**Context 的五种用法：**

**1. 请求超时控制**

```go
// controller/chat/chat_v1_chat.go
ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Minute)
defer cancel()

// controller/chat/chat_v1_ai_ops.go
ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Minute)
defer cancel()
```

**2. 值传递（Client ID 追踪）**

```go
// controller/chat/chat_v1_chatStream.go
ctx := context.WithValue(baseCtx, "client_id", req.Id)
```

**3. 链路传递（传递到 Agent）**

```go
// controller/chat/chat_v1_chat.go
runner, err := chat_workflow.BuildChatAgent(ctx)  // 传递带超时的 ctx
out, err := runner.Invoke(ctx, userMessage, compose.WithCallbacks(...))
```

**4. HTTP 请求超时**

```go
// common/utils/http.go
func Post(url string, params any) ([]byte, error) {
    ctx, cancelFunc := context.WithTimeout(context.Background(), 20*time.Second)
    defer cancelFunc()
    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
}
```

**5. 循环中的 Context**

```go
// ai/agent/plan_execute_replan/workAgent_planExecuteReplan.go
r := adk.NewRunner(ctx, adk.RunnerConfig{Agent: planExecuteAgent})
iter := r.Query(ctx, query)
for iter.Next(ctx) {  // 使用 ctx 判断是否取消
    event, _ := iter.Value()
}
```

**Context 最佳实践：**


| 实践                 | 说明                               |
| ------------------ | -------------------------------- |
| **defer cancel**   | 防止泄露                             |
| **不存储在结构体**        | context 只在函数参数中传递                |
| **Value 只用于横切关注点** | 如 trace_id、client_id             |
| **超时时间合理设置**       | chat=3min, ai-ops=1min, http=20s |


#### 题目 6-8：Sync 同步原语的使用场景

> watchTower 项目中使用了哪些 sync 同步原语？各自的使用场景是什么？

**1. sync.Once（单例初始化）**

```go
// model/model_factory.go
var (
    globalFactory *AIModelFactory
    factoryOnce   sync.Once
)

func GetGlobalFactory() *AIModelFactory {
    factoryOnce.Do(func() {
        globalFactory = &AIModelFactory{...}
        globalFactory.registerCreators()
    })
    return globalFactory
}

// ai/skills/loader.go
var (
    skillsCache []*Skill
    skillsOnce  sync.Once
)
```

**2. sync.Mutex（互斥锁）**

```go
// mem/mem.go - 全局内存 map 保护
var (
    SimpleMemoryMap = make(map[string]*SimpleMemory)
    mu              sync.Mutex
)

func GetSimpleMemory(id string) *SimpleMemory {
    mu.Lock()
    defer mu.Unlock()
    if mem, ok := SimpleMemoryMap[id]; ok {
        return mem
    }
    newMem := &SimpleMemory{...}
    SimpleMemoryMap[id] = newMem
    return newMem
}
```

**3. sync.RWMutex（读写互斥锁）**

```go
// mem/mem.go - 每个内存实例的消息列表
type SimpleMemory struct {
    Messages      []*schema.Message
    MaxWindowSize int
    smu           sync.RWMutex
}

func (s *SimpleMemory) GetMessages() []*schema.Message {
    s.smu.RLock()   // 读锁：多个读者可以同时访问
    defer s.smu.RUnlock()
    return s.Messages
}

func (s *SimpleMemory) SetMessages(msg *schema.Message) {
    s.smu.Lock()    // 写锁：独占访问
    defer s.smu.Unlock()
    s.Messages = append(s.Messages, msg)
}
```

**四种锁对比：**


| 锁类型     | 适用场景   | watchTower 示例  |
| ------- | ------ | -------------- |
| Mutex   | 写冲突保护  | 全局 memory map  |
| RWMutex | 读多写少   | 内存实例消息列表       |
| Once    | 一次性初始化 | 单例工厂、skills 加载 |


#### 题目 6-9：Go Agent 中的错误处理模式

> 在 watchTower 的 AI Agent 实现中，如何处理工具调用失败？请分析代码中的错误处理模式。

**错误处理的三层架构：**

**Layer 1: 工具层（Tool）**

```go
// ai/tools/query_metric_alerts.go
func NewPrometheusAlertsQueryTool() tool.InvokableTool {
    t, err := utils.InferOptionableTool(
        "query_prometheus_alerts",
        "Query active alerts from Prometheus alerting system...",
        func(ctx context.Context, input *struct{}, opts ...tool.Option) (output string, err error) {
            // 1. 构造请求
            req, _ := http.NewRequestWithContext(ctx, "GET",
                fmt.Sprintf("%s/api/v1/alerts", config.Conf.Prometheus.BaseUrl), nil)
            // 2. 发送请求
            resp, err := utils.Get[*PrometheusResponse](req.URL.String())
            // 3. 处理响应
            alerts, _ := json.Marshal(resp.Data.Alerts)
            return string(alerts), nil
        },
    )
    return t
}
```

**Layer 2: Agent 层（Executor）**

```go
// ai/agent/plan_execute_replan/executor.go
// Agent 配置中没有显式的错误处理配置
// 错误会传播到 Replanner，由 Replanner 决定如何处理
```

**Layer 3: Controller 层（HTTP）**

```go
// controller/chat/chat_v1_chat.go
out, err := runner.Invoke(ctx, userMessage, compose.WithCallbacks(...))
if err != nil {
    // 即使出错，也尝试返回已完成的部分结果
    if out != nil && out.Content != "" {
        return &vo.ChatRes{
            Answer:  out.Content,
            Warning: fmt.Sprintf("部分完成: %v", err),
        }
    }
    return nil, err
}
```

---

### 6.5 RAG 与向量检索

#### 题目 6-10：Milvus 向量数据库集成

> watchTower 如何集成 Milvus 实现 RAG？请从代码层面解释向量检索的完整流程。

**RAG 完整流程：**

```
文档文件 (markdown)
     │
     ▼
FileLoader (读取文件内容)
     │
     ▼
MarkdownSplitter (按标题/段落分块)
     │
     ▼
Doubao Embedding API (向量化，每块 2048 维)
     │
     ▼
Milvus Indexer (存入向量数据库)
     │
     ▼
Milvus Collection (id, vector, content, metadata)
```

**Milvus 客户端初始化：**

**Milvus 客户端初始化：**

```go
// common/milvus/milvusClient.go
func NewMilvusClient(ctx context.Context) (cli.Client, error) {
    // 注意：源码中字段名是 Address，不是 Uri
    addr := config.Conf.Milvus.Address
    if addr == "" {
        addr = "localhost:19530"
    }

    // 1. 先连接 default 数据库，检查 agent 数据库是否存在
    defaultClient, err := cli.NewClient(ctx, cli.Config{Address: addr, DBName: "default"})
    // 不存在则创建 agent 数据库

    // 2. 连接到 agent 数据库，检查 biz collection 是否存在
    agentClient, err := cli.NewClient(ctx, cli.Config{Address: addr, DBName: config.Conf.Milvus.DbName})

    // 3. 不存在则创建 collection，并创建索引
    dim := config.Conf.DoubaoEmbeddingModel.VectorDim  // 2048
    schema := &entity.Schema{
        CollectionName: config.Conf.Milvus.CollectionName,
        Fields:         collectionSchemaFields(dim),
    }
    agentClient.CreateCollection(ctx, schema, entity.DefaultShardNumber)
    // 创建 id / content / vector 三类 AUTOINDEX (L2 距离)

    defaultClient.Close()
    return agentClient, nil
}
```

**FloatVector 类型转换（关键坑）：**

```go
// common/milvus/retriver.go
// eino-ext 默认使用 BinaryVector，但 Milvus Collection 用的是 FloatVector
// 需要类型转换，否则检索失败

func floatVectorConverter(_ context.Context, vectors [][]float64) ([]entity.Vector, error) {
    out := make([]entity.Vector, 0, len(vectors))
    for _, v := range vectors {
        f32 := make([]float32, len(v))
        for i, x := range v {
            f32[i] = float32(x)  // float64 -> float32
        }
        out = append(out, entity.FloatVector(f32))
    }
    return out, nil
}
```

**向量检索代码：**

```go
// common/milvus/retriver.go
func (r *MilvusRetriever) Retrieve(ctx context.Context, query string) ([]*schema.Message, error) {
    vec, err := r.embedder(ctx, query)  // 查询向量化
    searchRes, err := r.client.Search(ctx, client.SearchParams{
        CollectionName: r.collectionName,
        Vector:         vec,
        Limit:          r.topK,
        WithVector:     false,
    })
    var results []*schema.Message
    for _, sr := range searchRes {
        results = append(results, &schema.Message{
            Role:       schema.RoleUser,
            Content:    sr.Entity["content"].(string),
        })
    }
    return results, nil
}
```

**Collection Schema：**

```go
// common/milvus/milvusClient.go
func collectionSchemaFields(dim string) []*entity.Field {
    return []*entity.Field{
        {Name: "id", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "256"}, PrimaryKey: true},
        {Name: "vector", DataType: entity.FieldTypeFloatVector, TypeParams: map[string]string{"dim": dim}},  // 2048维
        {Name: "content", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "8192"}},
        {Name: "metadata", DataType: entity.FieldTypeJSON},
    }
}
```

---

### 6.6 HTTP 与流式处理

#### 题目 6-11：SSE 流式响应实现

> watchTower 的 `/api/chat-stream` 接口使用 SSE 实现流式响应，请分析其实现原理。

**SSE 响应格式：**

```
HTTP/1.1 200 OK
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
X-Accel-Buffering: no

data: {"content": "Hello"}

data: {"content": ", "}

data: [DONE]
```

**watchTower 实现：**

```go
// controller/chat/chat_v1_chatStream.go
func ChatStream(c *gin.Context) {
    // 1. 设置 SSE headers
    c.Header("Content-Type", "text/event-stream")
    c.Header("X-Accel-Buffering", "no")  // 禁用 nginx 缓冲

    runner, _ := chat_workflow.BuildChatAgent(ctx)
    stream, err := runner.Stream(ctx, userMessage, compose.WithCallbacks(...))

    flusher, ok := c.Writer.(http.Flusher)
    if !ok {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "SSE not supported"})
        return
    }

    for {
        chunk, err := stream.Recv()
        if err == io.EOF {
            c.Writer.Write([]byte("data: [DONE]\n\n"))
            flusher.Flush()
            break
        }
        if err != nil {
            break
        }
        if len(chunk.Content) > 0 {
            fullResp.WriteString(chunk.Content)
            c.Writer.Write([]byte(fmt.Sprintf("data: %s\n\n", chunk.Content)))
            flusher.Flush()
        }
    }
}
```

**lookAheadStreamToolCallChecker（预读机制）：**

```go
// ai/agent/chat_workflow/flow.go
// 某些模型会在工具调用前输出前缀文本，需要预读检测

const maxLookAheadChunks = 20

// lookAheadStreamToolCallChecker reads up to maxLookAheadChunks chunks to
// detect tool calls, then returns false optimistically so the remaining
// stream can flow to the caller in real-time. This preserves streaming UX
// while still catching models (DeepSeek/Claude) that emit short preamble
// text before tool calls.
func lookAheadStreamToolCallChecker(_ context.Context, sr *schema.StreamReader[*schema.Message]) (bool, error) {
    defer sr.Close()
    for i := 0; i < maxLookAheadChunks; i++ {
        msg, err := sr.Recv()
        if err == io.EOF {
            return false, nil
        }
        if len(msg.ToolCalls) > 0 {
            return true, nil
        }
    }
    return false, nil
}
```

**关键设计：**

- `MaxStep: 25`：ReAct Agent 最多执行 25 步工具调用，防止死循环
- `lookAheadStreamToolCallChecker`：DeepSeek/Claude 等模型会在工具调用前输出短前缀文本，此函数预读最多 20 个 chunk 检测工具调用，返回 `true` 后流式输出给用户

**React Agent 中的工具注册：**

```go
// ai/agent/chat_workflow/flow.go
config := &react.AgentConfig{
    MaxStep:            25,
    ToolReturnDirectly: map[string]struct{}{},
    StreamToolCallChecker: lookAheadStreamToolCallChecker,
}
config.ToolCallingModel = chatModelIns11  // DsThinkChatModel

// 工具列表（MCP 日志 + Prometheus + MySQL + 时间 + 内部文档 + 文件搜索 + DuckDuckGo）
config.ToolsConfig.Tools = mcpTool
config.ToolsConfig.Tools = append(config.ToolsConfig.Tools,
    tools.NewPrometheusAlertsQueryTool(),
    tools.NewMysqlCrudTool(),
    tools.NewGetCurrentTimeTool(),
    tools.NewQueryInternalDocsTool(),
    tools.NewSearchFileTool(),
    searchTool,
)

---

### 6.7 设计模式

#### 题目 6-12：Model Factory 工厂模式

> watchTower 使用工厂模式管理多个 AI 模型，请分析其实现和扩展方式。

**工厂模式实现：**

​```go
// model/model_factory.go
const (
    DsThinkChatModelType = iota + 1  // = 1, 思考模型
    DsQuickChatModelType            // = 2, 快模型
    DoubaoEmbedderType              // = 3, Embedding 模型
)

type ModelCreator func(ctx context.Context, config *config.Config) AIModel

type AIModelFactory struct {
    creators map[int]ModelCreator
    mu      sync.RWMutex
}

func GetGlobalFactory() *AIModelFactory {
    factoryOnce.Do(func() {
        globalFactory = &AIModelFactory{
            creators: make(map[int]ModelCreator),
        }
        globalFactory.registerCreators()
    })
    return globalFactory
}

func (f *AIModelFactory) registerCreators() {
    f.creators[DsThinkChatModelType] = func(ctx context.Context, conf *config.Config) AIModel {
        return NewDsThinkChatModel(ctx, conf)
    }
    f.creators[DsQuickChatModelType] = func(ctx context.Context, conf *config.Config) AIModel {
        return NewDsQuickChatModel(ctx, conf)
    }
    f.creators[DoubaoEmbedderType] = func(ctx context.Context, conf *config.Config) AIModel {
        return NewDoubaoEmbedder(ctx, conf)
    }
}
```

**扩展新模型：**

```go
// Step 1: 添加类型常量
const (
    DsThinkChatModelType = iota + 1
    DsQuickChatModelType
    DoubaoEmbedderType
    ClaudeModelType  // 新增
)

// Step 2: 注册创建者
func (f *AIModelFactory) registerCreators() {
    f.creators[ClaudeModelType] = func(ctx context.Context, conf *config.Config) AIModel {
        return NewClaudeModel(ctx, conf)
    }
}
```

#### 题目 6-13：Skills 系统与动态提示词注入

> watchTower 的 Skills 系统如何实现动态提示词注入？请分析其实现原理。

**Skills 文件格式（Markdown + YAML Frontmatter）：**

```markdown
---
name: 告警处理手册
description: 各类服务告警的解释、排查步骤和错误码含义。
---

# 告警处理手册

## HighCPUUsage 告警

### 排查步骤
1. 检查 Pod 资源配额
2. 分析 GC 日志
3. 查看 pprof 火焰图
```

**解析 Frontmatter：**

```go
// ai/skills/loader.go
type Skill struct {
    Name        string `yaml:"name"`
    Description string `yaml:"description"`
    Content     string // Markdown 正文
}

func parseSkillFile(path string) (*Skill, error) {
    raw, _ := os.ReadFile(path)
    frontmatter, body, _ := splitFrontmatter(string(raw))
    skill := &Skill{}
    yaml.Unmarshal([]byte(frontmatter), skill)
    skill.Content = strings.TrimSpace(body)
    return skill, nil
}
```

**加载与注入：**

```go
func FormatForPrompt() string {
    list := LoadSkills()
    var b strings.Builder
    b.WriteString("## 已加载的 Skills（领域知识）\n")
    for i, skill := range list {
        b.WriteString(fmt.Sprintf("\n### Skill %d: %s\n", i+1, skill.Name))
        b.WriteString(skill.Content)
    }
    return b.String()
}
```

**Agent 中的使用：**

```go
// ai/agent/plan_execute_replan/workAgent_planExecuteReplan.go
if skillBlock := skills.FormatForPrompt(); skillBlock != "" {
    query = skillBlock + "\n" + query  // 注入到查询前
}
```

---

#### 题目 6-14：Go GMP 调度器原理

> watchTower 是一个纯 Go 项目。请解释 Go 语言的 GMP 调度模型，以及它与传统的线程模型的区別。

**GMP 模型：**

```
G (Goroutine)              M (Machine/Thread)             P (Processor)
  - 2KB 初始栈              - 系统线程                    - 持有 runq
  - 用户态调度              - 执行 G 代码                  - 最多 GOMAXPROCS 个
  - Go 运行时管理
```

**工作流程：**

```
1. M1 阻塞（syscall） → P 绑定 M2，继续调度 runq
2. G1 创建 G2 → G2 加入 M1 的 runq
3. M1 runq 过长（>256） → 一半 G 移到全局 GRQ
4. M1 空闲太久 → M1 被回收
```

**与线程的对比：**


| 维度   | Goroutine  | 线程            |
| ---- | ---------- | ------------- |
| 创建成本 | ~2KB       | ~1-8MB        |
| 切换成本 | 用户态 ~200ns | 内核态 ~10-100μs |
| 最大数量 | 数十万        | 数千            |


**watchTower 中的体现：**

```go
// AI Agent 并发调用多个工具时，大量 goroutine 被高效调度
ctx, cancel := context.WithTimeout(..., 3*time.Minute)
// 这里的 context.WithTimeout 依赖 Go 调度器
```

#### 题目 6-15：Go 内存泄漏排查

> watchTower 运行在 K8s 中，Pod 内存持续上涨但 CPU 正常。请描述 Go 程序内存泄漏的排查思路。

**排查步骤：**

```bash
# 1. 确认内存泄漏
kubectl top pods -n watchtower -l app=watchtower-backend

# 2. 导出堆内存 profile
kubectl exec -it -n watchtower <pod> -- \
  curl -s http://localhost:6872/debug/pprof/heap > heap.out

# 3. 本地分析
go tool pprof heap.out
# 输入 top，查看占用最多的对象

# 4. 查看 goroutine 数量（goroutine 泄漏）
kubectl exec -it -n watchtower <pod> -- \
  curl -s http://localhost:6872/debug/pprof/goroutine > goroutine.out

# 5. 火焰图
kubectl exec -it -n watchtower <pod> -- \
  curl -s http://localhost:6872/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof -http=:8080 cpu.prof
```

**常见泄漏场景：**


| 场景           | 指标表现             | 根因                      |
| ------------ | ---------------- | ----------------------- |
| Goroutine 泄漏 | goroutine 数量持续增长 | channel 阻塞、for 循环未退出    |
| 切片持续增长       | 内存持续上涨           | append 未截断、滑动窗口失效       |
| Map 持续增长     | 内存持续上涨           | 未清理过期 key               |
| Timer 未释放    | 内存缓慢增长           | `time.Timer` 未 `Stop()` |


#### 题目 6-16：Slice 扩容与 Map 底层实现

> Go 中 Slice 和 Map 的底层实现原理是什么？什么情况下会发生扩容？

**Slice 底层结构：**

```go
type slice struct {
    array unsafe.Pointer  // 指向底层数组
    len   int            // 当前长度
    cap   int            // 容量
}
```

**扩容规则：**

```go
// cap < 1024 时，每次扩容 2 倍
// cap >= 1024 时，每次扩容 1.25 倍

// 注意：append 可能返回新的 slice
s1 := append(s, 6)  // s1 != s，s1 是新的 slice
```

**Map 底层结构：**

```go
type hmap struct {
    count     int
    B         uint8    // 2^B 个桶
    buckets   unsafe.Pointer  // 桶数组
    oldbuckets unsafe.Pointer  // 渐进式扩容时的旧桶
}

// 每个桶 8 个 key-value 对，通过溢出桶处理哈希冲突
```

**Map 扩容触发条件：**

```go
// 1. 负载因子 > 6.5（count / 2^B > 6.5）
// 2. 溢出桶过多（noverflow > 2^B）
```

#### 题目 6-17：Gin 中间件与 HTTP 服务设计

> watchTower 使用 Gin 框架。请解释 Gin 中间件的执行原理，以及如何设计一个高效、可扩展的 HTTP 服务。

**Gin 中间件执行链：**

```go
func A(c *gin.Context) {
    fmt.Println("A before")
    c.Next()  // 调用下一个中间件
    fmt.Println("A after")
}

func B(c *gin.Context) {
    fmt.Println("B before")
    c.Next()
    fmt.Println("B after")
}

// 执行顺序: A before -> B before -> Handler -> B after -> A after
```

**watchTower 的 CORS 中间件：**

```go
// middleware/middleware.go
func CORSMiddleware(ctx *gin.Context) {
    if ctx.Request.Method == http.MethodOptions {
        ctx.AbortWithStatus(http.StatusNoContent)  // 终止后续处理
        return
    }
    ctx.Next()
}
```

**HTTP 服务设计：**

```go
// 优雅关闭
srv := &http.Server{Addr: ":6872", Handler: r}
go srv.ListenAndServe()

quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit

ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
srv.Shutdown(ctx)  // 等待处理中的请求完成
```

#### 题目 6-18：如何保证 AI Agent 的响应质量

> watchTower 的 AI Agent 可能在网络波动、模型幻觉、工具调用失败等情况下产生低质量输出。请从工程化角度说明如何保证输出质量。

**多层次质量保证体系：**

```
输入层: 输入验证、Skills 注入、历史窗口限制
    ↓
处理层: 超时控制、重试机制、降级策略、迭代控制
    ↓
输出层: 输出验证、部分结果返回、错误标记、Callback 记录
```

**代码示例：**

```go
// 即使出错，也返回已完成的部分结果
out, err := runner.Invoke(ctx, userMessage, compose.WithCallbacks(...))
if err != nil {
    if out != nil && out.Content != "" {
        return &vo.ChatRes{
            Answer:  out.Content,
            Warning: fmt.Sprintf("部分完成: %v", err),
        }
    }
    return nil, err
}
```

**质量优化策略：**

1. **输入层**：Query 长度限制、危险内容过滤、Skills 注入减少幻觉
2. **处理层**：超时（chat=3min/ai-ops=1min）、HTTP 重试 3 次
3. **输出层**：检查响应格式/长度、Warning 字段告知部分完成、全链路 Callback 审计

---

## 7. 综合设计题

### 7.1 Prometheus 相关面试题

#### 题目 7-1：Prometheus 架构原理

简述 Prometheus 的整体架构和工作原理，以及它与传统监控系统的区别。

**Prometheus 核心特性：**

- **拉模式（Pull）**：定期从 Target 拉取指标，而非被动接收
- **多维度数据模型**：指标名 + 标签集（Key-Value）
- **PromQL**：强大的查询语言
- **TSDB**：时序数据库，自研，支持高效压缩
- **服务发现**：Kubernetes、Consul、EC2 等
- **AlertManager**：独立的告警处理组件，支持去重、分组、抑制

---

#### 端到端案例：从服务报错到 watchTower 分析的全流程

**场景：** 线上 CPU 使用率突然飙高，触发告警。

**Step 1 — 监控指标采集（Prometheus Pull）**

```
服务实例 (10.0.0.5:9100)
     │
     │  node_cpu_usage > 0.8 持续 5 分钟
     ▼
被监控的机器上运行`node-exporter`暴露指标（9100端口的‘/metrics’接口）
     │  拉的服务器指标：CPU、内存、磁盘等
     │  每 15s 拉取一次 (scrape_interval)
     ▼
Prometheus Server
     │
     │  匹配告警规则: cpu_usage > 0.8 && rate(node_cpu[5m]) > 0.8
     ▼
AlertManager ←── 触发 firing 告警 (HighCPUUsage)
```

**Step 2 — watchTower 查询告警（查询接口：HTTP GET /api/v1/alerts）**

```
用户: "CPU 告警怎么解决？"

Plan-Execute-Replan Agent
     │
     │  Executor 调用 query_prometheus_alerts 工具
     │  GET http://mock-prometheus-svc.watchtower:9090/api/v1/alerts
     ▼
Prometheus 返回 JSON：
{
  "status": "success",
  "data": {
    "alerts": [{
      "labels": {
        "alertname": "HighCPUUsage",
        "instance": "10.0.0.5:9100",
        "severity": "warning"
      },
      "annotations": {
        "summary": "实例 CPU 使用率过高",
        "description": "10.0.0.5:9100 上 CPU 使用率已超过 80% 持续 5 分钟"
      },
      "state": "firing",
      "activeAt": "2026-05-20T09:00:00Z",
      "value": "8.5e-01"
    }]
  }
}
```

**Step 3 — 完整数据流图**

```
┌────────────────────────────────────────────────────────────────────────────────┐
│                                                                                │
│  端到端告警分析流程                                                               │
│                                                                                │
│  ┌──────────────┐     ┌──────────────┐     ┌─────────────┐                     │
│  │ 服务实例      │ ──▶ │ node-exporter│ ──▶ │  Prometheus │                      │
│  │ 10.0.0.5     │     │  :9100 暴露  │      │  每15s拉取   │                    │
│  └──────────────┘     └──────────────┘     └──────┬──────┘                     │
│                                                                                │
│                                      匹配告警规则 → HighCPUUsage firing          │
│                                                                                │
│                                              ┌─────▼──────┐                    │
│                                              │AlertManager│                    │
│                                              └────────────┘                    │
│                                                                                │
└────────────────────────────────────────────────────────────────────────────────┘
┌────────────────────────────────────────────────────────────────────────────────┐
│  watchTower Backend                                                            │
│  用户: "CPU 告警怎么处理？"                                                       │
│       │                                                                        │
│       ▼                                                                        │
│                                                                                │
│  ┌─────────────────────────────────────────────────────────┐                   │
│  │              Plan-Execute-Replan Agent                  │                   │
│                                                                                │
│  │  1. Planner (思考模型): 推理需查 Prometheus 告警           │                   │
│  │       │                                                 │                   │
│  │       ▼                                                 │                   │
│  │  2. Executor (快模型): 调用 query_prometheus_alerts      │                    │
│                                                                                │
│  │  工具返回: HighCPUUsage | 10.0.0.5:9100 | firing         │                   │
│  │       │                                                 │                   │
│  │       ▼                                                 │                   │
│  │  3. Replanner (思考模型): 生成处置建议                     │                   │
│  └─────────────────────────────────────────────────────────┘                   │
│                                                                                │
│       │                                                                        │
│       ▼                                                                        │
│  AI 回复: "10.0.0.5 CPU 超 80%，建议 kubectl top / scale"                        │
└────────────────────────────────────────────────────────────────────────────────┘
```

**核心代码路径：**

| 步骤 | 文件 | 关键代码 |
|------|------|----------|
| Prometheus 查询 | `ai/tools/query_metric_alerts.go` | `GET /api/v1/alerts` → `PrometheusAlert` |
| 工具注册 | `ai/tools/query_metric_alerts.go` | `NewPrometheusAlertsQueryTool()` |
| Agent 调用 | `ai/agent/plan_execute_replan/workAgent_planExecuteReplan.go` | `Executor` 调用工具 |


**与 Zabbix 的区别：**


| 维度   | Prometheus | Zabbix           |
| ---- | ---------- | ---------------- |
| 架构   | 拉模式        | 推/拉混合            |
| 数据模型 | 多维度标签      | 模板/主机            |
| 查询语言 | PromQL     | LLD + 函数         |
| 存储   | TSDB       | MySQL/PostgreSQL |


#### 题目 7-2：Prometheus 高可用方案

如何保证 Prometheus 的高可用？有哪些方案？

**方案一：联邦集群（Federation）**

- 按职责或地域拆分，每个 Prometheus 只抓取部分 Target
- Federation Server 汇总全局数据
- 缺点：数据分散，跨 Cell 查询复杂

**方案二：Remote Read + 远程存储**

- 本地保留短期数据（热数据）
- 远程存储保留长期数据（冷数据）

**方案三：Thanos 方案**

- Sidecar：与 Prometheus 同 Pod，通过 CSI 共享数据
- Store Gateway：查询远程对象存储（如 S3）
- Compact：压缩和降采样
- Receiver：接收 Remote Write
- Ruler：跨集群告警规则评估

#### 题目 7-3：PromQL 实战

用 PromQL 实现：查询过去 5 分钟内，HTTP 错误率最高的前 5 个服务。

```promql
# 基础版：计算 5 分钟错误率
sum(rate(http_requests_total{status=~"5.."}[5m]))
  by (service)
  /
sum(rate(http_requests_total[5m]))
  by (service)
  * 100

# 进阶版：按错误率排序，取 Top 5
topk(5,
  sum(rate(http_requests_total{status=~"5.."}[5m]))
    by (service, status)
  /
  sum(rate(http_requests_total[5m]))
    by (service)
)
```

**关键函数：**


| 函数                     | 用途         |
| ---------------------- | ---------- |
| `rate()`               | 计算每秒增长率    |
| `irate()`              | 计算即时增长率    |
| `increase()`           | 计算增量       |
| `predict_linear()`     | 线性预测       |
| `histogram_quantile()` | 计算分位数（P99） |


#### 题目 7-4：如何定位 CPU 突增问题

生产环境发现某服务的 CPU 使用率在 10:30 突然飙升至 100%，请描述排查思路。

**Step 1: 确认时间点**

```bash
kubectl top pods -n <namespace> --history=30m
kubectl logs -n <namespace> <pod-name> --since=30m | grep ERROR
```

**Step 2: 分析 Prometheus 指标**

```promql
container_cpu_usage_seconds_total{pod="<pod-name>"} [10m]
go_gc_duration_seconds_count{service="<service>"}
go_goroutines{service="<service>"}
```

**可能原因：**


| 原因     | 指标表现                      |
| ------ | ------------------------- |
| 突发流量   | QPS 上升，goroutine 增多       |
| GC STW | go_gc_duration_seconds 突增 |
| 死循环    | 单线程 CPU 100%，其他线程空闲       |
| 慢查询    | DB 查询延迟上升，连接池打满           |


### 7.2 日志采集相关面试题

#### 题目 7-5：日志采集架构设计

如果让你设计一套日志采集系统，需要支持：每天 10TB 日志量、100+ 服务、秒级查询延迟，你会如何设计？

**整体架构：**

```
业务服务 --> Filebeat/Fluentd --> Kafka --> Logstash --> Elasticsearch --> Kibana
                    │
                    v
              (本地缓冲: File / Redis)
```

**选型对比：**


| 组件   | ELK   | Loki | CLS (云服务) |
| ---- | ----- | ---- | --------- |
| 存储成本 | 高     | 低    | 中         |
| 查询延迟 | 毫秒级   | 秒级   | 秒级        |
| 扩展性  | 自管理复杂 | 简单   | 云托管       |


**关键设计决策：**

1. **日志格式标准化**：统一 JSON 格式
2. **采样策略**：ERROR 100%，WARN 50%，INFO 10%
3. **索引分层**：热数据 7 天（SSD）、温数据 8-30 天（HDD）、冷数据 30+ 天
4. **ILM 策略**：自动 rollover、shrink、freeze、delete

#### 题目 7-6：日志查询性能优化

日志量达到 PB 级别时，如何保证查询性能？

**1. 合理的分片设计**

```json
{ "settings": { "number_of_shards": 15, "refresh_interval": "30s", "translog": { "durability": "async" } } }
```

**2. 冷热分离架构**

```
Hot Node (SSD) --> Warm Node (SSD) --> Cold Node (HDD/S3)
  7天             8-30天              30天+
```

**3. ILM 策略**

```json
{ "policy": { "hot": { "actions": { "rollover": { "max_age": "7d" } } },
             "warm": { "min_age": "7d", "actions": { "shrink": {}, "forcemerge": {} } },
             "cold": { "min_age": "30d", "actions": { "freeze": {} } },
             "delete": { "min_age": "90d", "actions": { "delete": {} } } } }
```

**4. 查询优化**

```json
GET /logs-2026.05/_search
{ "_source": ["timestamp", "level", "message"],
  "query": { "bool": { "filter": [
    { "range": { "@timestamp": { "gte": "now-1h" } } },
    { "term": { "level": "ERROR" } }
  ]}}}
```

### 7.3 Kubernetes 相关面试题

#### 题目 7-7：K8s 中的网络通信

在 watchTower 项目中，watchtower 后端需要访问 mock-prometheus 服务。请描述 Pod 到 Pod 的网络通信路径。

```
watchtower-backend Pod
    │
    │ 1. DNS 解析
    │    mock-prometheus-svc.watchtower.svc.cluster.local
    │    ▼
    │ CoreDNS
    │    │ 返回 ClusterIP: 10.96.x.x
    │    ▼
    │ ClusterIP Service
    │    │ 2. kube-proxy 转发 (iptables / IPVS)
    │    ▼
    │ mock-prometheus Pod (endpoint)
```

**Service 类型对比：**


| 类型           | 访问范围   | 负载均衡       | 适用场景  |
| ------------ | ------ | ---------- | ----- |
| ClusterIP    | 集群内部   | kube-proxy | 内部服务  |
| NodePort     | 节点端口   | kube-proxy | 开发/测试 |
| LoadBalancer | 外部云 LB | 云厂商        | 生产环境  |


#### 题目 7-8：HPA 工作原理

watchTower 项目配置了 HPA。请描述 HPA 的工作原理，以及如何排查 HPA 不生效的问题。

```
用户配置 HPA (CPU 80%, 副本 2-10)
    │
    ▼
HPA Controller (kube-controller-manager)
    │ 1. 从 metrics-server 获取 Pod 指标
    │ 2. 计算当前利用率
    │ 3. 决定是否扩缩
    │ 4. 更新 Deployment 副本数
    ▼
metrics-server --> kubelet --> cAdvisor
```

**HPA 不生效的排查：**

```bash
kubectl get pods -n kube-system -l k8s-app=metrics-server
kubectl top pods -n watchtower
kubectl describe hpa -n watchtower watchtower-hpa
# 常见原因：metrics-server 未部署 / Pod 未设 resource requests / 副本数为 0
```

### 7.4 综合场景面试题

#### 题目 7-9：如何设计一个智能告警分析系统

基于 watchTower 项目，设计一个智能告警分析系统。当 Prometheus 产生告警时，自动分析根因并提供处置建议。

```
Prometheus --> AlertManager --> Webhook 接收 --> 告警处理服务
                                              │
                                              ▼
                                      Plan-Execute-Replan Agent
                                              │
              +------------------------------+------------------------------+
              │                              │                              │
              ▼                              ▼                              ▼
    query_prometheus_alerts         GetLogMcpTool()              Milvus RAG
              │                              │                              │
              +------------------------------+------------------------------+
                                              ▼
                                      分析报告输出
                                      - 根因分析
                                      - 处置建议
                                      - 相关日志
                                      - 参考文档
```

**技术选型：**


| 组件       | 选型                  | 理由            |
| -------- | ------------------- | ------------- |
| AI Agent | Plan-Execute-Replan | 复杂任务需要规划，动态调整 |
| 规划模型     | DeepSeek V3 (思考)    | 强推理能力，复杂分析    |
| 执行模型     | DeepSeek Quick (快)  | 简单工具调用，低延迟    |
| 向量数据库    | Milvus              | 开源，支持分布式      |
| 日志查询     | Tencent CLS (MCP)   | 托管服务，免运维      |


#### 题目 7-10：SRE 场景题

凌晨 2 点，你收到 watchTower 平台的 CPU 告警，值班群中大量用户反馈接口超时。请描述你的排查流程。

**P0 - 快速止血（5 分钟内）**

```bash
kubectl get pods -n watchtower -l app=watchtower-backend
kubectl logs -n watchtower <pod-name> --tail=100 | grep -E "ERROR|panic|timeout"
kubectl scale deployment watchtower-backend -n watchtower --replicas=10
```

**P1 - 定位根因（15 分钟内）**

```bash
kubectl top pods -n watchtower -l app=watchtower-backend
kubectl describe node <node-name> | grep -A10 "Conditions"
kubectl exec -it -n watchtower <pod> -- curl -s http://milvus-standalone.watchtower:19530/health
```

**P2 - 分析根因**

```bash
kubectl exec -it -n watchtower <pod> -- curl -s http://localhost:6872/debug/pprof/profile > cpu.prof
kubectl exec -it -n watchtower <pod> -- curl -s http://localhost:6872/debug/pprof/goroutine > goroutine.prof
```

**常见根因场景：**


| 场景       | 指标表现              | 快速修复    |
| -------- | ----------------- | ------- |
| 突发流量     | QPS 10x，请求排队      | 扩容      |
| 内存泄漏     | 内存持续上涨，GC 频繁      | 重启 Pod  |
| 数据库慢查询   | DB 连接池打满          | 杀掉慢查询   |
| AI 模型响应慢 | 请求堆积，goroutine 增多 | 限流 + 降级 |


#### 题目 7-11：职业规划与项目深度

在 watchTower 项目中，你负责哪些模块？你认为最有技术挑战的部分是什么？

**我的职责：**

1. **Prometheus 工具开发** - 设计 `query_metric_alerts` 工具，封装 Prometheus HTTP API
2. **MCP 日志客户端集成** - 集成腾讯云 CLS MCP 协议，实现 SSE 通信
3. **K8s 部署架构** - 设计高可用部署方案（2 副本 + HPA）
4. **AI Agent 工作流编排** - 设计 Plan-Execute-Replan Agent

**最有挑战的部分：Plan-Execute-Replan Agent 的迭代控制**

最大的挑战是如何控制 Agent 的迭代次数和终止条件：

```go
for iteration := 0; iteration < maxIterations; iteration++ {
    plan := planner.Generate(context)
    results := executor.Execute(plan)
    if replaner.IsComplete(results) {
        return replaner.Finalize(results)
    }
    context = replaner.Adjust(context, results)
}
```

**技术难点：**

- 如何让 Replanner 正确判断"分析是否充分" (①让LLM[Replanner]判断是否足以回答用户问题"为什么服务挂了"？如果不能，还需要什么信息？" ②配置规则，如几个计划点是否已达标)
- 如何避免无限循环（设置最大迭代次数 20）
- 如何处理部分工具调用失败（优雅降级）
- 如何优化工具调用的并行度（减少等待时间）

#### 题目 7-12：设计一个 Tool Calling 系统

假设让你从零设计一个 Tool Calling 系统（类似 LangChain/Eino 的工具调用），你会如何设计？

**整体架构：**

```
┌─────────────────────────────────────────────────────────────────┐
│                    Tool Calling System                          │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │ Tool Schema  │  │ Tool Registry│  │   Executor   │           │
│  │  定义接口     │  │   工具注册     │  │   调用执行    │           │
│  └──────────────┘  └──────────────┘  └──────────────┘           │
│         │                 │                   │                 │
│         └─────────────────┼───────────────────┘                 │
│                           ▼                                     │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                    Tool Manager                          │   │
│  │  - Register(tool)                                        │   │
│  │  - GetTool(name)                                         │   │
│  │  - CallTool(name, params)                                │   │
│  └──────────────────────────────────────────────────────────┘   │
│                           ▼                                     │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                     Agent Loop                           │   │
│  │  while not done:                                         │   │
│  │    response = llm.Generate(prompt + context)             │   │
│  │    if response.has_tool_call():                          │   │
│  │      result = tool_manager.Call(tool_call)               │   │
│  │      context += result                                   │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

**核心接口：**

```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() InputSchema
}

type ToolManager struct {
    registry ToolRegistry
    logger   Logger
}

func (m *ToolManager) Call(ctx context.Context, name string, args map[string]any) (string, error) {
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    return m.registry.Call(ctx, name, args)
}
```

**错误处理策略：**


| 错误类型     | 处理策略         | 示例             |
| -------- | ------------ | -------------- |
| 参数验证失败   | 返回错误，不重试     | invalid status |
| 网络超时     | 重试 3 次，指数退避  | Prometheus 不可达 |
| 工具 panic | 捕获并返回错误      | 内部 panic       |
| 工具不存在    | 返回错误         | 未知工具名          |
| 超时       | 取消上下文，返回超时错误 | 超过 30s         |


#### 题目 7-13：如何在生产环境中调试 AI Agent

作为 Go Agent 开发工程师，你如何在生产环境中调试 AI Agent？请从日志、监控、测试等方面说明。

**1. 日志方案**

```go
// 使用 Eino Callback 记录全链路日志
compose.WithCallbacks(logcallback.LogCallback(&LogCallbackConfig{
    Detail: true,
}))
```

**2. 监控指标**

```go
var (
    agentIterations = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Name: "agent_iterations_total",
        Help: "Total iterations per agent",
    }, []string{"agent_type", "status"})

    toolCalls = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "tool_calls_total",
        Help: "Total tool calls",
    }, []string{"tool_name", "status"})

    toolDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "tool_duration_seconds",
        Help:    "Tool call duration",
        Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
    }, []string{"tool_name"})
)
```

**3. 单元测试**

```go
// Mock 工具和 LLM 进行单元测试
mockLLM := &MockChatModel{
    Responses: []string{
        `I'll check the Prometheus alerts.`,
        `The alert is HighCPUUsage on node-1.`,
    },
}
executor := NewExecutorAgent(ctx, WithTools(mockPrometheus), WithModel(mockLLM))
result, err := executor.Execute(ctx, "Analyze HighCPUUsage alert")
```

**4. 集成测试**

```go
// 使用真实服务进行端到端测试
promServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    json.NewEncoder(w).Encode(PrometheusResponse{...})
}))
config.Conf.Prometheus.BaseUrl = promServer.URL
result, err := BuildPlanExecuteReplanAgent(ctx, "Query alerts")
```

---

## 8. 速查卡与参考答案

### 8.1 Prometheus 核心概念速记

```
Prometheus = 拉模式 + 多维度 + TSDB + PromQL + ServiceDiscovery
           + AlertManager + Federation + RemoteWrite

核心指标类型:
  - Counter (计数器): 只增不减, rate() / increase()
  - Gauge (仪表盘): 可增可减, 当前值
  - Histogram (直方图): 分布统计, histogram_quantile()
  - Summary (汇总): 客户端计算分位数

高可用方案:
  - Federation: 按职责拆分
  - Thanos/Cortex: 统一视图 + 长期存储
  - Remote Write: 写入远程存储
```

### 8.2 日志采集核心概念速记

```
日志采集架构 = 日志源 + Agent + 缓冲队列 + 存储 + 查询

常见 Agent:
  - Filebeat: 轻量，ELK 生态
  - Fluentd: 插件丰富，K8s 原生
  - Loggie: 字节跳动开源，云原生友好

MCP 协议:
  - Host: AI 应用
  - Client: MCP SDK
  - Server: 数据源提供者
  - Transport: stdio / SSE / HTTP

watchTower 中的日志查询流程:
  NewSSEMCPClient()
    --> DiscoverTools()
    --> 调用 CLS API
    --> SSE 流式返回结果
```

### 8.3 K8s 核心概念速记

```
K8s 网络:
  - Pod IP: 唯一，跨节点可达
  - Service DNS: <svc>.<ns>.svc.cluster.local
  - kube-proxy: iptables / IPVS 转发

HPA:
  - metrics-server: 提供 Pod 指标
  - 算法: ceil(current * (currentMetric / targetMetric))
  - stabilization: 缩容稳定窗口

StatefulSet:
  - 稳定网络标识 (ordinal index)
  - 稳定持久存储
  - 有序部署/扩缩/删除
  (etcd / Milvus / MinIO 使用 StatefulSet)
```

### 8.4 Go Agent 核心概念速记

```
【Eino 框架】
  - compose.NewGraph(): DAG 工作流编排
  - planexecute.New(): Plan-Execute-Replan Agent
  - react.AgentConfig: ReAct Agent 配置
  - compose.AllPredecessor: 等待所有前驱完成

【Go 并发】
  - context.Context: 超时控制、值传递、取消传播
  - sync.Once: 单例初始化
  - sync.RWMutex: 读多写少保护
  - defer cancel(): 防止 context 泄漏

【工具系统】
  - utils.InferOptionableTool(): 自动推断 JSON Schema
  - MCP: Model Context Protocol，统一工具调用协议
  - SSE: Server-Sent Events，日志流式返回

【RAG 流程】
  - Loader --> Splitter --> Embedder --> Indexer --> Milvus
  - floatVectorConverter(): 类型转换（float64 --> float32）
  - FloatVector vs BinaryVector: 必须匹配 Collection Schema

【流式处理】
  - text/event-stream: SSE Content-Type
  - X-Accel-Buffering: no: 禁用 nginx 缓冲
  - lookAheadStreamToolCallChecker: 预读检测工具调用

【设计模式】
  - Factory: ModelFactory 单例模式
  - Skill: YAML Frontmatter 解析，动态注入提示词
  - Callback: 全链路日志和监控
```

---

## 附录 A：相关文档链接


| 文档              | 链接                                                                                                                                                       |
| --------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Prometheus 官方文档 | [https://prometheus.io/docs/introduction/overview/](https://prometheus.io/docs/introduction/overview/)                                                   |
| Thanos 官方文档     | [https://thanos.io/](https://thanos.io/)                                                                                                                 |
| MCP 协议规范        | [https://modelcontextprotocol.io/](https://modelcontextprotocol.io/)                                                                                     |
| 腾讯云 CLS         | [https://cloud.tencent.com/document/product/614](https://cloud.tencent.com/document/product/614)                                                         |
| K8s HPA 文档      | [https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/) |
| Eino 框架文档       | [https://www.eino.dev/](https://www.eino.dev/)                                                                                                           |


## 附录 B：watchTower 项目关键文件索引


| 功能                 | 文件路径                                                          | 关键函数/结构体                         |
| ------------------ | ------------------------------------------------------------- | -------------------------------- |
| Plan-Execute Agent | `ai/agent/plan_execute_replan/workAgent_planExecuteReplan.go` | `BuildPlanExecuteReplanAgent()`  |
| Chat Workflow      | `ai/agent/chat_workflow/orchestration.go`                     | `compose.NewGraph()`             |
| Prometheus Tool    | `ai/tools/query_metric_alerts.go`                             | `NewPrometheusAlertsQueryTool()` |
| MCP Log Tool       | `ai/tools/query_log.go`                                       | `GetLogMcpTool()`                |
| Milvus Client      | `common/milvus/milvusClient.go`                               | `NewMilvusClient()`              |
| Retriever          | `common/milvus/retriver.go`                                   | `floatVectorConverter()`         |
| Model Factory      | `model/model_factory.go`                                      | `GetGlobalFactory()`             |
| Skills Loader      | `ai/skills/loader.go`                                         | `FormatForPrompt()`              |
| Memory             | `mem/mem.go`                                                  | `GetSimpleMemory()`              |
| Callback           | `common/log_callback/log_callback.go`                         | `LogCallback()`                  |
| Chat Handler       | `controller/chat/chat_v1_chat.go`                             | `Chat()`                         |
| Stream Handler     | `controller/chat/chat_v1_chatStream.go`                       | `ChatStream()`                   |
| AIOps Handler      | `controller/chat/chat_v1_ai_ops.go`                           | `AIOps()`                        |


## 附录 C：常用调试命令

```bash
# Prometheus
curl http://localhost:9090/api/v1/alerts
curl http://localhost:9090/api/v1/query?query=up

# Kubernetes
kubectl get all -n watchtower
kubectl describe hpa -n watchtower
kubectl top pods -n watchtower
kubectl logs -n watchtower -l app=watchtower-backend --tail=100 -f

# Milvus
kubectl exec -it -n watchtower milvus-standalone-0 -- \
  milvus-cli connect -h milvus-standalone -p 19530
kubectl get milvus -n watchtower

# 网络调试
kubectl exec -it -n watchtower <pod> -- \
  curl -v http://mock-prometheus-svc.watchtower:9090/api/v1/alerts
```

---

## 9. Skill 系统优化：向量相似度按需加载

### 现状问题

当前 `workAgent_planExecuteReplan.go` 中的 skill 注入是**全量加载 + 全量注入**：

```go
// ai/agent/plan_execute_replan/workAgent_planExecuteReplan.go
if skillBlock := skills.FormatForPrompt(); skillBlock != "" {
    query = skillBlock + "\n" + query
    // (这里未来可以考虑优化成向量相似度匹配对应的skill)
}
```

`skills.FormatForPrompt()` 会把所有 skill 的 name + description + content 全部拼接，即使某个 skill 和用户问题完全无关。

**问题：**

- 用户问"MySQL 慢查询"，但 `file_search.md`（文件搜索指南）也被注入，浪费 token
- 上下文窗口被无关 skill 占据，影响真正的分析质量
- skill 数量增长后，注入量无上限地膨胀

### 优化方案：向量相似度匹配

**思路：** 用户 query 和每个 skill 的 description 做 embedding 向量相似度，只注入 top-k 最相关的 skill。

```
用户输入: "CPU 告警怎么处理？"

     │
     ▼
Embedding 模型向量化
     │
     ▼
计算与所有 skill description 的余弦相似度
     │
     ├── alert_handling.md     → 0.92  ✅ 注入
     ├── replan_guide.md       → 0.78  ✅ 注入
     └── file_search.md        → 0.15  ❌ 跳过
     │
     ▼
只注入 top-2 的 skill，大幅减少 token 消耗
```

### 核心代码实现

**1. 新增 skill 匹配器** `ai/skills/matcher.go`：

```go
package skills

import (
	"context"
	"sort"
)

// MatchResult 单个 skill 的匹配结果
type MatchResult struct {
	Skill  *Skill
	Score  float64  // 余弦相似度，0~1
}

// MatchByEmbedding 对用户 query 进行向量化，然后与所有 skill 的 description
// 做余弦相似度，返回 top-k 最相关的 skill 列表。
func MatchByEmbedding(ctx context.Context, query string, topK int) []*Skill {
	// 1. 对 query 做 embedding
	queryVec := embed(ctx, query)

	// 2. 计算每个 skill 的相似度(不需要存取于向量库，直接比！！)
	all := LoadSkills()
	var results []MatchResult
	for _, skill := range all {
		descVec := embed(ctx, skill.Description)
		score := cosineSimilarity(queryVec, descVec)
		results = append(results, MatchResult{Skill: skill, Score: score})
	}

	// 3. 排序取 topK
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if topK > len(results) {
		topK = len(results)
	}
	var matched []*Skill
	for i := 0; i < topK; i++ {
		if results[i].Score > 0.3 { // 阈值过滤，低相关度 skill 不注入
			matched = append(matched, results[i].Skill)
		}
	}
	return matched
}

// cosineSimilarity 计算两个向量的余弦相似度
func cosineSimilarity(a, b []float32) float64 {
	dot := float64(0)
	normA := float64(0)
	normB := float64(0)
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
```

**2. 新增 skill Embedder 封装** `ai/skills/embedder.go`：

```go
package skills

import (
	"context"
	"watchTower/ai/model"
)

// embed 调用模型工厂的 embedding 模型，对文本进行向量化
func embed(ctx context.Context, text string) []float32 {
	factory := model.GetGlobalFactory()
	embCreator := factory.GetModelCreator(model.EmbeddingModelType)
	if embCreator == nil {
		return nil
	}
	embModel := embCreator(ctx, nil)
	vec, err := embModel.EmbedStrings(ctx, []string{text})
	if err != nil || len(vec) == 0 {
		return nil
	}
	return vec[0]
}
```

**3. 修改 `FormatForPrompt` 支持按需注入：**

```go
// FormatForPromptByQuery 只注入与 query 最相关的 topK 个 skill
func FormatForPromptByQuery(ctx context.Context, query string, topK int) string {
	matched := MatchByEmbedding(ctx, query, topK)
	if len(matched) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## 已加载的 Skills（按需匹配）\n")
	for i, skill := range matched {
		b.WriteString(fmt.Sprintf("\n### Skill %d: %s（相关度: %.2f）\n", i+1, skill.Name, ""))
		if skill.Description != "" {
			b.WriteString(fmt.Sprintf("说明: %s\n", skill.Description))
		}
		b.WriteString(skill.Content)
		b.WriteString("\n")
	}
	return b.String()
}
```

**4. 修改调用方，替换 TODO：**

```go
// ai/agent/plan_execute_replan/workAgent_planExecuteReplan.go
import "watchTower/ai/skills"

// 之前：全量注入
if skillBlock := skills.FormatForPrompt(); skillBlock != "" {
    query = skillBlock + "\n" + query
}

// 之后：按需注入（向量相似度 top-2）
if skillBlock := skills.FormatForPromptByQuery(ctx, query, 2); skillBlock != "" {
    query = skillBlock + "\n" + query
}
```

### 效果对比

| 维度 | 优化前（全量注入） | 优化后（按需注入） |
|------|------------------|------------------|
| 每次注入 skill 数 | 全部（3 个） | top-2（按 query 相关度） |
| token 消耗 | ~800 token/请求 | ~300~500 token/请求 |
| 相关 skill 命中 | 始终包含，也包含无关的 | 只保留高相关度 |
| 新增 skill 成本 | 每次请求都注入 | 只在相关时被注入 |

### 面试总结

> "skill 加载最初是全量注入，在 `workAgent_planExecuteReplan.go` 里留了 TODO。后来我把它优化成了向量相似度匹配：用户 query 和每个 skill 的 description 分别做 embedding，计算余弦相似度，只注入 top-2 最相关的 skill。这样每次请求的 token 消耗从 ~800 降到 ~300-500，而且新增 skill 时不会影响所有 query 的上下文大小。"

