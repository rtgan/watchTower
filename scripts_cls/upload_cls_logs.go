// upload_cls_logs.go — 把 mock_data/logs/*.log 逐行作为日志上传到腾讯云 CLS 指定主题，
// 供 Agent 的 query_log（走 CLS MCP）按关键字检索。
//
// 使用腾讯云官方 CLS SDK（github.com/tencentcloud/tencentcloud-cls-sdk-go），
// 内置正确签名与 protobuf 协议，避免手撸签名的坑。
//
// 用法:
//   export CLS_SECRET_ID="AKIDxxxx"
//   export CLS_SECRET_KEY="xxxx"
//   export CLS_TOPIC_ID="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
//   export CLS_ENDPOINT="ap-chongqing.cls.tencentcs.com"   # 主题所在地域的外网接入域名
//   export CLS_LOG_DIR="/Users/you/watchTower/mock_data/logs"
//   go run ./scripts_cls/upload_cls_logs.go
//
// 上传后，Agent 在 CLS 控制台或 MCP 里按 "panic"/"response"/"reconciliation"/"oom" 等关键字即可命中。
package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	cls "github.com/tencentcloud/tencentcloud-cls-sdk-go"
)

func main() {
	topicID := os.Getenv("CLS_TOPIC_ID")
	secretID := os.Getenv("CLS_SECRET_ID")
	secretKey := os.Getenv("CLS_SECRET_KEY")
	endpoint := os.Getenv("CLS_ENDPOINT")
	logDir := os.Getenv("CLS_LOG_DIR")
	if logDir == "" {
		logDir = "./mock_data/logs"
	}

	missing := []string{}
	if topicID == "" {
		missing = append(missing, "CLS_TOPIC_ID")
	}
	if secretID == "" {
		missing = append(missing, "CLS_SECRET_ID")
	}
	if secretKey == "" {
		missing = append(missing, "CLS_SECRET_KEY")
	}
	if endpoint == "" {
		missing = append(missing, "CLS_ENDPOINT")
	}
	if len(missing) > 0 {
		log.Fatalf("缺少环境变量: %v\n请先在腾讯云 CLS 控制台创建日志主题，并设置上述环境变量", missing)
	}

	// SDK 要求 endpoint 带 https:// 前缀
	if !strings.HasPrefix(endpoint, "http") {
		endpoint = "https://" + endpoint
	}

	files, err := filepath.Glob(filepath.Join(logDir, "*.log"))
	if err != nil {
		log.Fatalf("扫描日志目录失败: %v", err)
	}
	if len(files) == 0 {
		log.Fatalf("目录 %s 下没有 *.log 文件", logDir)
	}

	producerConfig := cls.GetDefaultAsyncProducerClientConfig()
	producerConfig.Endpoint = endpoint
	producerConfig.AccessKeyID = secretID
	producerConfig.AccessKeySecret = secretKey
	producerConfig.Retries = 5

	producer, err := cls.NewAsyncProducerClient(producerConfig)
	if err != nil {
		log.Fatalf("创建 CLS producer 失败: %v", err)
	}
	producer.Start()
	defer producer.Close(5000)

	var wg sync.WaitGroup
	cb := &resultCallback{} // 记录上传失败信息
	successCount := 0
	failCount := 0

	for _, f := range files {
		service := serviceFromFile(f)
		logs, err := readLogs(f, service)
		if err != nil {
			log.Printf("[FAIL] 读取 %s: %v", f, err)
			failCount++
			continue
		}
		if len(logs) == 0 {
			continue
		}
		wg.Add(1)
		go func(path string, logs []*cls.Log) {
			defer wg.Done()
			if err := producer.SendLogList(topicID, logs, cb); err != nil {
				log.Printf("[FAIL] %s: %v", path, err)
				failCount++
				return
			}
			log.Printf("[OK] %s (%d 条)", path, len(logs))
			successCount++
		}(f, logs)
	}
	wg.Wait()

	log.Printf("上传完成: 成功 %d 个文件, 失败 %d。等待刷盘...", successCount, failCount)
	// Close 会触发剩余 batch 发送并等待回调
	if err := producer.Close(10000); err != nil {
		log.Printf("producer close 警告: %v", err)
	}
	log.Println("完成。可在 CLS 控制台检索，或让 Agent 通过 query_log 按关键字查询。")
}

// serviceFromFile 由文件名推断 service 字段，便于 Agent 按服务过滤
func serviceFromFile(path string) string {
	name := strings.ToLower(filepath.Base(path))
	switch {
	case strings.Contains(name, "reconciliation"):
		return "reconciliation-service"
	case strings.Contains(name, "node") || strings.Contains(name, "resource"):
		return "node-exporter"
	default: // panic / response / order-create 都属 order-service
		return "order-service"
	}
}

// readLogs 把一个日志文件每行转为一条 CLS Log，时间散布在过去 1 小时
func readLogs(path, service string) ([]*cls.Log, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var logs []*cls.Log
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	baseTime := time.Now().Unix()
	i := 0
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			continue
		}
		i++
		contents := map[string]string{
			"service": service,
			"level":   parseLevel(line),
			"msg":     line,
		}
		// 时间散布在过去 1 小时内，便于"最近1小时"检索命中
		logs = append(logs, cls.NewCLSLog(baseTime-int64(3600-i*8), contents))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("扫描 %s: %w", path, err)
	}
	return logs, nil
}

func parseLevel(line string) string {
	switch {
	case strings.Contains(line, "[FATAL]"):
		return "fatal"
	case strings.Contains(line, "[ERROR]"):
		return "error"
	case strings.Contains(line, "[WARN]"):
		return "warn"
	case strings.Contains(line, "[INFO]"):
		return "info"
	}
	return "info"
}

// resultCallback 实现 cls.CallBack，记录上传失败信息
type resultCallback struct{}

func (c *resultCallback) Success(result *cls.Result) {}
func (c *resultCallback) Fail(result *cls.Result) {
	log.Printf("[CLS FAIL] code=%s msg=%s reqId=%s", result.GetErrorCode(), result.GetErrorMessage(), result.GetRequestId())
}
