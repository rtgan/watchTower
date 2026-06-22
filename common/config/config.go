package config

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	logcallback "watchTower/common/log_callback"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

var Conf *Config

func InitConfig() (*Config, error) {
	root, err := moduleRoot()
	if err != nil {
		log.Fatalf("find module root: %v", err)
	}

	// 1. 加载 .env（不存在不报错；含敏感密钥，不应提交 git）
	//    优先项目根 .env，其次 etc/.env
	for _, p := range []string{filepath.Join(root, ".env"), filepath.Join(root, "etc", ".env")} {
		if _, err := os.Stat(p); err == nil {
			_ = godotenv.Load(p)
			break
		}
	}

	cfgPath := filepath.Join(root, "etc", "conf.yml")

	// 2. 读取原始配置内容，对 ${VAR} 占位符做环境变量展开，再交给 viper。
	//    这样 conf.yml 可保留占位符（如 api_key: "${ARK_API_KEY}"），真实值放 .env。
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		log.Fatalf("read config file: %v", err)
	}
	expanded := os.ExpandEnv(string(raw))

	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(bytes.NewReader([]byte(expanded))); err != nil {
		log.Fatalf("read config: %v", err)
	}

	Conf = &Config{}
	if err := v.Unmarshal(Conf); err != nil {
		log.Fatalf("unmarshal: %v", err)
	}
	return Conf, nil
}

// 获取mod模块根目录
func moduleRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", wd)
		}
		dir = parent
	}
}

// Viper认定的注解是mapstructure
type Config struct {
	Server               ServerConfig                  `mapstructure:"server"`
	Logger               LoggerConfig                  `mapstructure:"logger"`
	LogCallback          logcallback.LogCallbackConfig `mapstructure:"log_callback"`
	DsThinkChatModel     DsThinkChatModelConfig        `mapstructure:"ds_think_chat_model"`
	DsQuickChatModel     DsQuickChatModelConfig        `mapstructure:"ds_quick_chat_model"`
	DoubaoEmbeddingModel DoubaoEmbeddingModelConfig    `mapstructure:"doubao_embedding_model"`
	FileDir              string                        `mapstructure:"file_dir"`
	McpUrl               string                        `mapstructure:"mcp_url"`
	Prometheus           PrometheusConfig              `mapstructure:"prometheus"`
	Milvus               MilvusConfig                  `mapstructure:"milvus"`
	// Google Custom Search JSON API，用于 google_search 工具；不配则不在 Agent 中注册该工具
	GoogleSearch         GoogleSearchConfig            `mapstructure:"google_search"`
	// 腾讯云 CLS 直连配置（query_log 工具直接调 SearchLog API，绕开 MCP）。
	// 不配则 query_log 不可用（executor 会降级跳过）。
	CLS                  CLSConfig                     `mapstructure:"cls"`
}

// CLSConfig 腾讯云 CLS 日志检索直连配置
type CLSConfig struct {
	SecretID  string `mapstructure:"secret_id"`
	SecretKey string `mapstructure:"secret_key"`
	TopicID   string `mapstructure:"topic_id"`
	Endpoint  string `mapstructure:"endpoint"`  // API 域名，如 cls.tencentcloudapi.com
	Region    string `mapstructure:"region"`    // 地域，如 ap-chongqing
}

type GoogleSearchConfig struct {
	ApiKey         string `mapstructure:"api_key"`
	SearchEngineID string `mapstructure:"search_engine_id"`
}

type ServerConfig struct {
	AppName     string `mapstructure:"appName"`
	Host        string `mapstructure:"host"`
	Port        int    `mapstructure:"port"`
	SwaggerPath string `mapstructure:"swaggerPath"`
}

type LoggerConfig struct {
	Level  string `mapstructure:"level"`
	Stdout bool   `mapstructure:"stdout"`
}

type DsThinkChatModelConfig struct {
	ApiKey  string `mapstructure:"api_key"`
	BaseUrl string `mapstructure:"base_url"`
	Model   string `mapstructure:"model"`
}

type DsQuickChatModelConfig struct {
	ApiKey  string `mapstructure:"api_key"`
	BaseUrl string `mapstructure:"base_url"`
	Model   string `mapstructure:"model"`
}

type DoubaoEmbeddingModelConfig struct {
	ApiKey    string `mapstructure:"api_key"`
	Model     string `mapstructure:"model"`
	VectorDim string `mapstructure:"vector_dim"`
}

type PrometheusConfig struct {
	BaseUrl string `mapstructure:"base_url"`
}

type MilvusConfig struct {
	Address        string `mapstructure:"address"`
	DbName         string `mapstructure:"db_name"`
	CollectionName string `mapstructure:"collection_name"`
}
