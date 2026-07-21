---
name: 告警处理速查（路由）
description: 告警名 -> 应搜日志关键字 + 相关错误码 的速查路由。分析告警、排查故障、解读错误码时使用；完整排查步骤与处置建议用 query_internal_docs 检索知识库《告警处理手册》。
---

# 告警处理速查（告警名 -> 关键字）

本速查常驻 system prompt，**只给"告警名 -> 应搜日志关键字 + 相关错误码"的快速映射**。
完整排查步骤、处置建议**不在此文件**：对每条告警调用 `query_internal_docs(告警名)` 从知识库《告警处理手册》检索。
单一事实源 = `knowledge/告警处理手册.md` + `knowledge/服务错误类型.md`；本文件只做路由，不重复细节，避免漂移。

## 告警名 -> 日志关键字 + 错误码

| 告警名 (alertname) | query_log 关键字 | 相关错误码 |
|---|---|---|
| ServiceDown | panic；伴 HighMemoryUsage 再搜 oom / OOMKilled | 12003 / 12002 |
| APIHighErrorRate | response（+ 接口名）；12003 再搜 downstream / connection refused | 12003 / 22003 / 12002 / 12001 |
| ReconciliationDiff | error、reconciliation | 12002 / 12003 / 52002 |
| HighMemoryUsage | memory、oom、OOMKilled | - |
| HighCPUUsage | cpu、throttle（伴 ServiceDown 优先按 panic） | - |
| DiskSpaceRunningLow | disk、no space left（确认 labels.mountpoint 分区） | - |

## 错误码释义
错误码定义统一见知识库《服务错误类型》（`query_internal_docs` 检索）。上表"相关错误码"列只列该告警常伴随的码号供快速定位，不在此重复定义，避免漂移。

## 标准排查流程
1. `query_prometheus_alerts` 拿活跃告警
2. 对每条告警 `query_internal_docs(告警名)` 取完整排查步骤（知识库）
3. `get_current_time` 确定时间窗
4. `query_log(上表关键字)` 取日志
5. 用错误码释义（知识库《服务错误类型》）解读
6. 汇总成告警分析报告
