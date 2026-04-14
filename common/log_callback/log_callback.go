package logcallback

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/callbacks"
)

type LogCallbackConfig struct {
	Detail bool `mapstructure:"detail"`
	Debug  bool `mapstructure:"debug"`
}

// 可以通过'全局注入'——callbacks.AppendGlobalHandlers(logcallback.LogCallback(nil))，来记录所有组件的输入和输出。
// 或者 '局部注入'——如compose.WithCallbacks(logcallback.LogCallback(nil))针对某个运行单位进行注入
func LogCallback(config *LogCallbackConfig) callbacks.Handler {
	if config == nil {
		config = &LogCallbackConfig{
			Detail: true,
		}
	}

	builder := callbacks.NewHandlerBuilder()
	builder.OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
		fmt.Printf("[view start]:[%s:%s:%s]\n", info.Component, info.Type, info.Name)
		if config.Detail {
			var b []byte
			if config.Debug {
				b, _ = json.MarshalIndent(input, "", "  ") //json会带上缩进，更易读
			} else {
				b, _ = json.Marshal(input)
			}
			fmt.Printf("[input data]:%s\n", string(b))
		}
		return ctx
	})
	builder.OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
		fmt.Printf("[view end]:[%s:%s:%s]\n", info.Component, info.Type, info.Name)
		if config.Detail {
			var b []byte
			if config.Debug {
				b, _ = json.MarshalIndent(output, "", "  ") //json会带上缩进，更易读
			} else {
				b, _ = json.Marshal(output)
			}
			fmt.Printf("[output data]:%s\n", string(b))
		}
		return ctx
	})

	return builder.Build()
}
