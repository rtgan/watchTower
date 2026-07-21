package main

import (
	"context"
	"fmt"
	"log"
	"watchTower/common/config"
	"watchTower/common/metrics"
	"watchTower/common/trace"
	"watchTower/mem"
	"watchTower/middleware"
	"watchTower/router"

	"github.com/gin-gonic/gin"
)

func main() {
	conf, err := config.InitConfig()
	if err != nil {
		log.Fatalf("init config: %v", err)
	}

	// 注册 Prometheus 指标 + 订阅 trace span 钩子（latency/tool/token/deflection）
	metrics.Register()

	// trace 落盘目录（默认 traces，可被 conf.trace.dir 覆盖）
	if conf.Trace.Dir != "" {
		trace.DefaultStore.SetDir(conf.Trace.Dir)
	}
	// 可选：开启 OTel sink（结构化 trace 总是开，不受此开关影响）
	if conf.Trace.Enabled {
		svc := conf.Trace.ServiceName
		if svc == "" {
			svc = "watchTower"
		}
		dir := conf.Trace.Dir
		if dir == "" {
			dir = "traces"
		}
		if shutdown, e := trace.InitOTel(svc, dir); e != nil {
			log.Printf("[trace] OTel sink init failed (structured trace still active): %v", e)
		} else {
			defer func() { _ = shutdown(context.Background()) }()
		}
	}

	// 会话记忆：driver=redis 时切 RedisStore（默认 InMemory）
	if conf.Memory.Driver == "redis" {
		rs, e := mem.NewRedisStore(conf.Memory.RedisAddr, conf.Memory.RedisPassword, conf.Memory.RedisDB)
		if e != nil {
			log.Printf("[memory] Redis 不可用，回退 InMemory: %v", e)
		} else {
			mem.SetDefaultStore(rs)
			log.Printf("[memory] 使用 Redis 存储 %s", conf.Memory.RedisAddr)
		}
	}

	// MySQL 持久化存储（Phase 5）：配置 mysql_dsn 后自动使用 MySQLStore
	if conf.Memory.MySQLDSN != "" {
		ms, err := mem.NewMySQLStore(conf.Memory.MySQLDSN)
		if err != nil {
			log.Printf("[memory] MySQL 不可用，回退 InMemory: %v", err)
		} else {
			mem.SetDefaultStore(ms)
			log.Printf("[memory] 使用 MySQL 持久化存储")
			// 同时初始化全量历史存储（复用同一 gorm.DB）
			historyStore := mem.NewMySQLHistoryStore(ms.DB())
			mem.SetDefaultHistoryStore(historyStore)
		}
	}

	// 长期记忆（Milvus 语义检索）：仅当 long_term=true 且 MySQL 已配置时启用
	if conf.Memory.LongTerm && conf.Memory.MySQLDSN != "" {
		lt, err := mem.NewMilvusMemoryStore(context.Background())
		if err != nil {
			log.Printf("[memory] 长期记忆(Milvus)不可用: %v", err)
		} else {
			mem.SetDefaultLongTermMemory(lt)
			collName := conf.Milvus.MemoryCollectionName
			if collName == "" {
				collName = "conversation_memory"
			}
			log.Printf("[memory] 长期记忆(Milvus)已启用，集合: %s", collName)
		}
	}

	host := conf.Server.Host
	port := conf.Server.Port

	//启动HTTP服务
	if err := StartServer(host, port); err != nil {
		log.Fatalf("start server: %v", err)
	}
}

func StartServer(host string, port int) error {
	r := gin.Default()
	r.Use(middleware.CORSMiddleware) //设置跨域策略
	r = router.InitRouter(r)
	return r.Run(fmt.Sprintf("%s:%d", host, port))
}
