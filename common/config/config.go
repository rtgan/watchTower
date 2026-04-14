package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	logcallback "watchTower/common/log_callback"

	"github.com/spf13/viper"
)

var Conf *Config

func InitConfig() (*Config, error) {
	root, err := moduleRoot()
	if err != nil {
		log.Fatalf("find module root: %v", err)
	}
	cfgPath := filepath.Join(root, "etc", "conf.yml")

	v := viper.New()
	v.SetConfigFile(cfgPath)

	if err := v.ReadInConfig(); err != nil {
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
	DbName         string `mapstructure:"db_name"`
	CollectionName string `mapstructure:"collection_name"`
}
