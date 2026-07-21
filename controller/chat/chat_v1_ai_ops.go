package chat

import (
	"context"
	"net/http"
	"time"
	"watchTower/ai/agent/plan_execute_replan"
	"watchTower/ai/agent/supervisor"
	"watchTower/common/config"
	"watchTower/common/trace"
	"watchTower/model/vo"

	"github.com/gin-gonic/gin"
)

func AIOps(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Minute)
	defer cancel()

	query := `
"1. 你是一个智能的服务告警分析助手,首先调用工具query_prometheus_alerts获取所有活跃的告警。"
"2. 分别根据告警的名称调用工具query_internal_docs，获取告警名对应的处理方案。"
"3. 完全遵循内部文档的内容进行查询和分析,不允许使用文档外的任何信息。"
"4. 涉及到时间的参数都需要先通过工具get_current_time获取当前时间,再结合工具的时间要求进行传参。"
"5. 涉及到日志的查询,需要先通过日志工具获取相关日志信息。"
"6. 分别将告警对应查询到的信息进行总结分析,最后生成告警运维分析报告，格式如下：
告警分析报告
---
# 告警处理详情
## 活跃告警清单
## 告警根因分析N(第N个告警)
## 处理方案执行N(第N个告警)
## 结论
`

	// 请求级 trace：plan_execute_replan / supervisor 从 ctx 取 Recorder，捕获事件流
	rec := trace.NewRecorder(query, time.Now().Format(time.RFC3339Nano))
	ctx = trace.WithRecorder(ctx, rec)

	// 按策略选择执行器：supervisor（默认，并行子 agent）或 plan_execute（回退）
	var (
		resp   string
		detail []string
		err    error
	)
	if config.Conf != nil && config.Conf.Agent.Strategy == "plan_execute" {
		//planner/executor/replanner是流水线形式
		resp, detail, err = plan_execute_replan.BuildPlanExecuteReplanAgent(ctx, query)
	} else { //supervisor 扮演的就是 orchestrator（编排者），只是流程代码写死，不由LLM动态决策：triage（取告警清单）→ fan-out（分发任务）→ synthesize（汇总）
		resp, detail, err = supervisor.BuildSupervisorAgent(ctx, query)
	}
	status := "ok"
	if err != nil {
		status = "error"
		run := rec.Finalize(status, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "trace_id": run.ID})
		return
	}
	if resp == "" {
		status = "error"
		run := rec.Finalize(status, nil)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "内部错误", "trace_id": run.ID})
		return
	}
	run := rec.Finalize(status, err)
	res := &vo.AIOpsRes{
		Result:  resp,
		Detail:  detail,
		TraceID: run.ID,
	}
	c.JSON(http.StatusOK, res)
}
