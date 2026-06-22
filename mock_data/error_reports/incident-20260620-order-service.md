# 故障报告 incident-20260620-order-service

- 故障编号：INC-20260620-001
- 故障等级：P1
- 故障时间：2026-06-20 06:00 ~ 06:45 (UTC+8)
- 受影响服务：order-service（主）、payment-service（下游被牵连）、reconciliation-service（对账）
- 受影响接口：/api/order/create（失败率 15%）
- 报错错误码：12003（下游接口错误）、22003（订单创建失败）、52002（补偿重放失败）、OOMKilled

## 一、告警清单（query_prometheus_alerts）
| 告警名 | 实例 | 严重度 | 激活时间 | 说明 |
|---|---|---|---|---|
| ServiceDown | 10.0.0.10:8080 | critical | 06:08 | order-service panic 重启 |
| APIHighErrorRate | 10.0.0.10:8080 | critical | 06:12 | /api/order/create 失败率 15% |
| ReconciliationDiff | 10.0.0.11:8081 | warning | 06:15 | 对账差异 23 笔 |
| HighMemoryUsage | 10.0.0.10:9100 | critical | 06:05 | 内存 93%，内存泄漏 |
| HighCPUUsage | 10.0.0.10:9100 | warning | 06:05 | CPU 85%，重启风暴 |
| DiskSpaceRunningLow | 10.0.0.10:9100 | warning | 06:17 | /data 剩余 8% |

## 二、根因分析
05:55 发布 order-service v1.8.2，引入 nil pointer bug（order/service.go:128 `OrderService.Create`）。下单时触发 panic，pod 反复重启：
1. panic 重启 → 触发 **ServiceDown**。
2. 重启间隙无法稳定调用下游 payment-service → /api/order/create 返回 12003 → 触发 **APIHighErrorRate**。
3. 内存泄漏叠加重启 → OOMKilled → 触发 **HighMemoryUsage**；重启风暴 + GC → 触发 **HighCPUUsage**。
4. panic 堆栈日志高频写入刷爆 /data → 触发 **DiskSpaceRunningLow**。
5. 06:00~06:20 期间失败订单未同步下游 → 对账差异 23 笔 → 触发 **ReconciliationDiff**。

## 三、证据链（工具排查到的日志）
- query_log("panic") → `order-service-panic.log`：panic 堆栈指向 order/service.go:128 OrderService.Create，05:55 发布后开始。
- query_log("response", /api/order/create) → `order-create-response.log`：失败码 12003/22003，下游 connection refused。
- query_log("error","reconciliation") → `reconciliation-error.log`：对账差异 23 笔，重放失败 52002。
- query_log("memory"/"oom"/"cpu"/"disk") → `node-resource.log`：OOMKilled、cpu throttle、/data no space left。

## 四、处置与策略
1. 立即回滚 order-service 至 v1.8.1，消除 panic 根因（ServiceDown / CPU / 内存 / 磁盘连锁恢复）。
2. 对 payment-service 做熔断降级，待 order-service 稳定后恢复调用（APIHighErrorRate 消除）。
3. 清理 /data 旧日志并配置日志轮转，恢复写盘。
4. 对 23 笔对账差异订单执行补偿重放（ReconciliationDiff 闭环）。
5. 修复 v1.8.2 空指针 bug 后灰度发布，补充单测覆盖 OrderService.Create 空值分支。

## 五、复盘结论
6 条告警同源于一次发布引入的 panic。Agent 应能通过"告警名 → 内部文档 → 日志关键字 → 错误码"链路定位到 order/service.go:128 空指针这一单一根因，并给出回滚 + 补偿的处置策略。
