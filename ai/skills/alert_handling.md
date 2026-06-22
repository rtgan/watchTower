---
name: 告警处理手册
description: 各类服务告警的解释、排查步骤和错误码含义。当分析告警、排查故障、解读错误码时参考此文档。
---

# 告警处理手册

## ServiceDown 服务下线
告警解释：服务下线通常因为服务进程 panic 崩溃导致 pod 反复重启，健康检查连续失败。
排查步骤：
1. 先调用 get_current_time 获取当前时间
2. 用日志工具按关键字 "panic" 搜索该服务最近 1 小时日志
3. 在 panic 堆栈中定位崩溃的代码文件、行号与触发函数
4. 结合错误码（12003 下游错误 / 12002 数据库错误）判断诱因
5. 若伴随 HighMemoryUsage，用 "oom"、"OOMKilled" 搜索确认是否 OOM

## APIHighErrorRate 接口失败率过高
告警解释：接口失败率过高由服务自身异常或下游不可用导致，labels.endpoint 给出接口名。
排查步骤：
1. 调用 get_current_time 获取当前时间
2. 用日志工具按接口名和关键字 "response" 搜索最近 1 小时日志
3. 分析 error 与错误码：12003 下游接口错误、12002 数据库更新失败、12001 参数错误
4. 若为 12003，用 "downstream"、"connection refused" 搜索下游日志

## ReconciliationDiff 与下游对账发现差异
告警解释：数据同步异常或计算错误导致上游与下游对账不一致。
排查步骤：
1. 调用 get_current_time 获取当前时间
2. 用日志工具按关键字 "error" 和 "reconciliation" 搜索最近 1 小时日志
3. 根据 diff 笔数与失败订单号分析差异原因
4. 同步失败出现 12002/12003 时按对应错误码处置

## HighMemoryUsage 内存使用率过高
排查步骤：用 "memory"、"oom"、"OOMKilled" 搜索日志；确认 OOM 后结合 ServiceDown panic 日志判断重启原因；排查内存泄漏。

## HighCPUUsage CPU 使用率过高
排查步骤：用 "cpu"、"throttle" 搜索日志；若伴随 ServiceDown，优先按 "panic" 排查下线根因，CPU 飙升多为重启风暴次生现象。

## DiskSpaceRunningLow 磁盘空间不足
排查步骤：用 "disk"、"no space left" 搜索日志；确认 labels.mountpoint 分区；若日志暴涨由 panic 引起，结合 "panic" 日志定位根因。

## 服务错误码与常见原因
- 12001：调用接口参数错误（如类型不匹配）
- 12002：数据库更新失败（数据库问题，建议排查日志）
- 12003：下游接口错误（下游接口报错或不可用）
- 12004：实例不存在（上游传了错误的实例 id）

更详细的分服务错误码见知识库文档《服务错误类型》。
