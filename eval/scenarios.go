package eval

// DefaultScenarios 内置评估场景（从 mock_data 派生）。
//   1. incident-20260620-order-service：完整 6 告警雪崩（根因命中）
//   2. single-servicedown：单告警（简化路径）
//   3. log-miss：告警规定的关键字在日志里无命中（边界：agent 应回报"无证据"而非幻觉）
//
// 数据内联以保证离线确定性；不依赖 mock_data 文件路径。修改时保持与 mock_data 一致。
func DefaultScenarios() []*Scenario {
	return []*Scenario{
		scenarioIncident(),
		scenarioSingleServiceDown(),
		scenarioLogMiss(),
	}
}

// ---- 场景 1：完整 6 告警雪崩 ----

func scenarioIncident() *Scenario {
	return &Scenario{
		ID:          "incident-20260620-order-service",
		Description: "order-service v1.8.2 发布引入空指针 panic，6 条告警同源雪崩",
		AlertsJSON:  incidentAlertsJSON,
		Logs: []LogFixture{
			{Keywords: []string{"panic"}, Text: panicLog},
			{Keywords: []string{"response"}, Text: responseLog},
			{Keywords: []string{"reconciliation", "error"}, Text: reconciliationLog},
			{Keywords: []string{"memory", "oom", "OOMKilled", "cpu", "throttle", "disk", "no space left"}, Text: nodeResourceLog},
		},
		Docs:    incidentDocs(),
		FixedNow: "2026-06-20 06:20:00",
		Expected: Expected{
			RootCause:      "order/service.go:128",
			Keywords:       []string{"panic", "response", "reconciliation"},
			ErrorCodes:     []string{"12003", "52002", "22003"},
			MustCallTools:  []string{"query_prometheus_alerts", "query_internal_docs"},
			GoldenReport:   "6 条告警同源于 order-service v1.8.2 发布引入的 nil pointer panic（order/service.go:128）。处置：回滚 v1.8.1、payment 熔断、清理 /data 日志、对 23 笔对账差异补偿重放、补单测。",
		},
	}
}

// ---- 场景 2：单告警（ServiceDown）----

func scenarioSingleServiceDown() *Scenario {
	return &Scenario{
		ID:          "single-servicedown",
		Description: "仅 ServiceDown 一条告警，验证简化排查路径",
		AlertsJSON:  singleServiceDownAlertsJSON,
		Logs: []LogFixture{
			{Keywords: []string{"panic"}, Text: panicLog},
		},
		Docs: []DocFixture{
			{AlertNames: []string{"ServiceDown", "服务下线"}, Text: docServiceDown},
		},
		FixedNow: "2026-06-20 06:20:00",
		Expected: Expected{
			RootCause:      "order/service.go:128",
			Keywords:       []string{"panic"},
			ErrorCodes:     []string{},
			MustCallTools:  []string{"query_prometheus_alerts", "query_internal_docs"},
			GoldenReport:   "ServiceDown 由 order-service panic 重启导致，根因 order/service.go:128 空指针。处置：回滚并补单测。",
		},
	}
}

// ---- 场景 3：日志无命中（边界）----

func scenarioLogMiss() *Scenario {
	return &Scenario{
		ID:          "log-miss-disk",
		Description: "DiskSpaceRunningLow 但规定关键字在日志里无命中；agent 应回报缺证据而非编造根因",
		AlertsJSON:  diskAlertsJSON,
		Logs:        []LogFixture{
			// 故意不提供 disk/no space left 的日志 -> query_log 应返回空
		},
		Docs: []DocFixture{
			{AlertNames: []string{"DiskSpaceRunningLow", "磁盘空间不足"}, Text: docDisk},
		},
		FixedNow: "2026-06-20 06:20:00",
		Expected: Expected{
			RootCause:      "", // 无期望根因：关键考察点是不幻觉
			Keywords:       []string{"disk", "no space left"},
			ErrorCodes:     []string{},
			MustCallTools:  []string{"query_prometheus_alerts", "query_internal_docs"},
			GoldenReport:   "DiskSpaceRunningLow 告警，但 query_log 未检索到相关日志证据，应如实报告证据不足、建议人工排查或扩大检索范围，不得编造根因。",
		},
	}
}

// incidentDocs 完整告警处理手册小节（按 alertname 切片）。
func incidentDocs() []DocFixture {
	return []DocFixture{
		{AlertNames: []string{"ServiceDown", "服务下线"}, Text: docServiceDown},
		{AlertNames: []string{"APIHighErrorRate", "接口失败率过高"}, Text: docAPIHighErrorRate},
		{AlertNames: []string{"ReconciliationDiff", "对账"}, Text: docReconciliationDiff},
		{AlertNames: []string{"HighMemoryUsage", "内存"}, Text: docHighMemory},
		{AlertNames: []string{"HighCPUUsage", "CPU"}, Text: docHighCPU},
		{AlertNames: []string{"DiskSpaceRunningLow", "磁盘"}, Text: docDisk},
	}
}

// ---- 文档小节文本（与 knowledge/告警处理手册.md 对齐）----

const docServiceDown = `## ServiceDown 服务下线
告警解释：服务下线通常因为服务进程 panic 崩溃导致 pod 反复重启，健康检查连续失败。
排查步骤：
1. 先调用 get_current_time 获取当前时间
2. 用日志工具按关键字 "panic" 搜索该服务最近 1 小时日志
3. 在 panic 堆栈中定位崩溃的代码文件、行号与触发函数
4. 结合错误码（12003 下游错误 / 12002 数据库错误）判断诱因
5. 若伴随 HighMemoryUsage，用 "oom"、"OOMKilled" 搜索确认是否 OOM`

const docAPIHighErrorRate = `## APIHighErrorRate 接口失败率过高
告警解释：接口失败率过高由服务自身异常或下游不可用导致，labels.endpoint 给出接口名。
排查步骤：
1. 调用 get_current_time 获取当前时间
2. 用日志工具按接口名和关键字 "response" 搜索最近 1 小时日志
3. 分析 error 与错误码：12003 下游接口错误、22003 订单创建失败、12001 参数错误
4. 若为 12003，用 "downstream"、"connection refused" 搜索下游日志`

const docReconciliationDiff = `## ReconciliationDiff 与下游对账发现差异
告警解释：数据同步异常或计算错误导致上游与下游对账不一致。
排查步骤：
1. 调用 get_current_time 获取当前时间
2. 用日志工具按关键字 "error" 和 "reconciliation" 搜索最近 1 小时日志
3. 根据 diff 笔数与失败订单号分析差异原因
4. 同步失败出现 12002/12003 时按对应错误码处置，52002 需先恢复下游服务`

const docHighMemory = `## HighMemoryUsage 内存使用率过高
排查步骤：用 "memory"、"oom"、"OOMKilled" 搜索日志；确认 OOM 后结合 ServiceDown panic 日志判断重启原因；排查内存泄漏。`

const docHighCPU = `## HighCPUUsage CPU 使用率过高
排查步骤：用 "cpu"、"throttle" 搜索日志；若伴随 ServiceDown，优先按 "panic" 排查下线根因，CPU 飙升多为重启风暴次生现象。`

const docDisk = `## DiskSpaceRunningLow 磁盘空间不足
排查步骤：用 "disk"、"no space left" 搜索日志；确认 labels.mountpoint 分区；若日志暴涨由 panic 引起，结合 "panic" 日志定位根因。`

// ---- 日志文本（与 mock_data/logs/*.log 对齐，节选）----

const panicLog = `2026-06-20T06:00:12.481+08:00 [ERROR] order-service pod=order-service-7d9c6b-x4k2f level=error msg="panic occurred" panic="runtime error: invalid memory address or nil pointer dereference"
2026-06-20T06:00:12.482+08:00 [ERROR] order-service pod=order-service-7d9c6b-x4k2f stack="github.com/watchTower/order/internal/service.(*OrderService).Create order/service.go:128"
2026-06-20T06:00:12.482+08:00 [ERROR] order-service pod=order-service-7d9c6b-x4k2f stack="github.com/watchTower/order/internal/handler.CreateOrder order/handler.go:57"
2026-06-20T06:00:12.483+08:00 [FATAL] order-service pod=order-service-7d9c6b-x4k2f msg="panic: runtime error: invalid memory address or nil pointer dereference - process exiting, restarting pod"`

const responseLog = `2026-06-20T06:12:03.221+08:00 [WARN] order-service endpoint=/api/order/create trace_id=tr-9f3a2c1b level=warn msg="response error" code=12003 detail="downstream payment-service call failed: dial tcp 10.0.0.20:8082: connect: connection refused" user_id=u-100283 order_id=od-20260620001
2026-06-20T06:13:02.551+08:00 [ERROR] order-service endpoint=/api/order/create trace_id=tr-9f3a2e90 level=error msg="response error" code=22003 detail="订单创建失败: 下游 payment-service 不可用, code=12003" user_id=u-100291 order_id=od-20260620010
2026-06-20T06:15:41.772+08:00 [ERROR] order-service endpoint=/api/order/create trace_id=tr-9f3a34c1 level=error msg="response error" code=12003 detail="payment-service unhealthy, circuit breaker open" user_id=u-100319 order_id=od-20260620025`

const reconciliationLog = `2026-06-20T06:20:05.118+08:00 [ERROR] reconciliation-service level=error msg="reconciliation diff detected" window="2026-06-20 06:00~06:20" upstream_count=312 downstream_count=289 diff_count=23
2026-06-20T06:20:05.119+08:00 [ERROR] reconciliation-service level=error msg="reconciliation sync failed" code=12003 detail="补偿重放下游 payment-service 失败, connection refused" order_id=od-20260620001
2026-06-20T06:20:05.127+08:00 [ERROR] reconciliation-service level=error msg="reconciliation replay failed" code=52002 detail="差异订单重放下游失败, 需先恢复下游服务" order_id=od-20260620025`

const nodeResourceLog = `2026-06-20T06:05:30.002+08:00 [WARN] node-exporter instance=10.0.0.10:9100 level=warn msg="memory usage high" usage_percent=88 detail="order-service heap 持续上涨, 疑似内存泄漏"
2026-06-20T06:08:11.743+08:00 [ERROR] kubelet node=node-10.0.0.10 level=error msg="OOMKilled" pod=order-service-7d9c6b-x4k2f container=order-service reason="OOMKilled" memory_used="2.1Gi"
2026-06-20T06:09:02.118+08:00 [WARN] node-exporter instance=10.0.0.10:9100 level=warn msg="cpu usage high" usage_percent=87 detail="order-service 重启风暴, GC 压力大"
2026-06-20T06:10:55.330+08:00 [WARN] cadvisor instance=10.0.0.10:9100 level=warn msg="cpu throttle" pod=order-service-7d9c6b-x4k2f throttled_percent=92
2026-06-20T06:08:00.000+08:00 [ERROR] order-service level=error msg="disk no space left" mountpoint=/data detail="/data 分区剩余空间不足 10%, panic 堆栈日志暴涨"`

// ---- 告警快照 ----

const incidentAlertsJSON = `{
  "status": "success",
  "data": {
    "alerts": [
      { "labels": { "alertname": "ServiceDown", "instance": "10.0.0.10:8080", "job": "order-service", "severity": "critical" }, "annotations": { "summary": "order-service 服务不可用", "description": "10.0.0.10:8080 的 order-service 连续 3 次健康检查失败，疑似 panic 重启，服务可能已宕机" }, "state": "firing", "activeAt": "2026-06-20T06:08:00.000000000Z", "value": "0e+00" },
      { "labels": { "alertname": "APIHighErrorRate", "instance": "10.0.0.10:8080", "job": "order-service", "severity": "critical", "endpoint": "/api/order/create" }, "annotations": { "summary": "接口失败率过高", "description": "/api/order/create 接口在过去 5 分钟内错误率达到 15%，疑似下游 payment-service 不可用" }, "state": "firing", "activeAt": "2026-06-20T06:12:00.000000000Z", "value": "1.5e+01" },
      { "labels": { "alertname": "ReconciliationDiff", "instance": "10.0.0.11:8081", "job": "reconciliation-service", "severity": "warning" }, "annotations": { "summary": "与下游对账发现差异", "description": "reconciliation-service 发现上游 order 与下游 payment 对账差异 23 笔，疑似失败订单未同步" }, "state": "firing", "activeAt": "2026-06-20T06:15:00.000000000Z", "value": "2.3e+01" },
      { "labels": { "alertname": "HighMemoryUsage", "instance": "10.0.0.10:9100", "job": "node-exporter", "severity": "critical" }, "annotations": { "summary": "实例内存使用率过高", "description": "10.0.0.10:9100 上内存使用率已超过 93%，疑似 order-service 内存泄漏，可能导致 OOM" }, "state": "firing", "activeAt": "2026-06-19T23:48:00.000000000Z", "value": "9.3e+01" },
      { "labels": { "alertname": "HighCPUUsage", "instance": "10.0.0.10:9100", "job": "node-exporter", "severity": "warning" }, "annotations": { "summary": "实例 CPU 使用率过高", "description": "10.0.0.10:9100 上 CPU 使用率已超过 85%，持续 5 分钟，疑似 order-service 重启风暴" }, "state": "firing", "activeAt": "2026-06-20T06:05:00.000000000Z", "value": "8.5e+01" },
      { "labels": { "alertname": "DiskSpaceRunningLow", "instance": "10.0.0.10:9100", "job": "node-exporter", "severity": "warning", "mountpoint": "/data" }, "annotations": { "summary": "磁盘空间不足", "description": "10.0.0.10:9100 的 /data 分区剩余空间不足 10%，疑似 panic 堆栈日志暴涨刷爆磁盘" }, "state": "firing", "activeAt": "2026-06-20T04:18:00.000000000Z", "value": "9.1e+01" }
    ]
  }
}`

const singleServiceDownAlertsJSON = `{
  "status": "success",
  "data": {
    "alerts": [
      { "labels": { "alertname": "ServiceDown", "instance": "10.0.0.10:8080", "job": "order-service", "severity": "critical" }, "annotations": { "summary": "order-service 服务不可用", "description": "10.0.0.10:8080 的 order-service 连续 3 次健康检查失败，疑似 panic 重启" }, "state": "firing", "activeAt": "2026-06-20T06:08:00.000000000Z", "value": "0e+00" }
    ]
  }
}`

const diskAlertsJSON = `{
  "status": "success",
  "data": {
    "alerts": [
      { "labels": { "alertname": "DiskSpaceRunningLow", "instance": "10.0.0.10:9100", "job": "node-exporter", "severity": "warning", "mountpoint": "/data" }, "annotations": { "summary": "磁盘空间不足", "description": "10.0.0.10:9100 的 /data 分区剩余空间不足 10%" }, "state": "firing", "activeAt": "2026-06-20T06:17:00.000000000Z", "value": "9.1e+01" }
    ]
  }
}`
