package trace

import (
	"context"
	"strconv"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// EinoHandler 把 Recorder 包装成 eino callback Handler。
//
// 嵌套靠 context 链维护：OnStart 把新 span 写入 ctx；OnEnd/OnError 结束 span 并把 ctx 恢复到父 span。
// 这样即使存在并行分支（eino 会 fork ctx），每个分支也能独立维护自己的 span 栈，不会互相串扰。
//
// 覆盖流式与非流式两种回调：流式输入/输出用占位摘要（避免抽干 stream 影响业务）。
func EinoHandler(r *Recorder) callbacks.Handler {
	if r == nil {
		return noopHandler() // 防御：调用方传 nil 时返回可安全调用的 no-op handler
	}
	b := callbacks.NewHandlerBuilder()

	b.OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
		s, ctx2 := r.StartSpan(ctx, info.Name, string(info.Component), MarshalInput(input))
		s.Type = string(info.Type)
		return ctx2
	})
	b.OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
		s := spanFromCtx(ctx)
		// ChatModel：从回调输出抽取 token 用量写入 span.Attrs，供 metrics 订阅统计成本。
		if string(info.Component) == "ChatModel" {
			captureTokenUsage(s, info.Name, output)
		}
		r.EndSpan(ctx, s, MarshalInput(output), nil)
		return popSpan(ctx, s)
	})
	b.OnErrorFn(func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
		s := spanFromCtx(ctx)
		r.EndSpan(ctx, s, "", err)
		return popSpan(ctx, s)
	})
	b.OnStartWithStreamInputFn(func(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
		if input != nil {
			input.Close()
		}
		s, ctx2 := r.StartSpan(ctx, info.Name, string(info.Component), "<stream input>")
		s.Type = string(info.Type)
		return ctx2
	})
	b.OnEndWithStreamOutputFn(func(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
		if output != nil {
			output.Close()
		}
		s := spanFromCtx(ctx)
		r.EndSpan(ctx, s, "<stream output>", nil)
		return popSpan(ctx, s)
	})
	return b.Build()
}

// noopHandler 返回一个所有方法都是 no-op 的 handler（用于 Recorder 为 nil 的防御场景）。
// 注意：callbacks.NewHandlerBuilder().Build() 产生的空 handler 在被调用时会因 fn 为 nil 而 panic，
// 故此处显式设置所有 fn 为空操作。
func noopHandler() callbacks.Handler {
	b := callbacks.NewHandlerBuilder()
	b.OnStartFn(func(ctx context.Context, _ *callbacks.RunInfo, _ callbacks.CallbackInput) context.Context { return ctx })
	b.OnEndFn(func(ctx context.Context, _ *callbacks.RunInfo, _ callbacks.CallbackOutput) context.Context { return ctx })
	b.OnErrorFn(func(ctx context.Context, _ *callbacks.RunInfo, _ error) context.Context { return ctx })
	b.OnStartWithStreamInputFn(func(ctx context.Context, _ *callbacks.RunInfo, in *schema.StreamReader[callbacks.CallbackInput]) context.Context {
		if in != nil {
			in.Close()
		}
		return ctx
	})
	b.OnEndWithStreamOutputFn(func(ctx context.Context, _ *callbacks.RunInfo, out *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
		if out != nil {
			out.Close()
		}
		return ctx
	})
	return b.Build()
}

// popSpan 结束 span 后把 ctx 中的当前 span 恢复为父 span（维护栈）。
func popSpan(ctx context.Context, s *Span) context.Context {
	if s == nil {
		return ctx
	}
	if s.parent != nil {
		return context.WithValue(ctx, spanKey{}, s.parent)
	}
	return context.WithValue(ctx, spanKey{}, (*Span)(nil))
}

// captureTokenUsage 从 ChatModel 回调输出抽取 token 用量写入 span.Attrs。
// metrics 的 span 钩子读取这些 attrs 累加 llm_tokens_total。
func captureTokenUsage(s *Span, name string, output callbacks.CallbackOutput) {
	mo := model.ConvCallbackOutput(output)
	if mo == nil || mo.TokenUsage == nil {
		return
	}
	if s.Attrs == nil {
		s.Attrs = map[string]string{}
	}
	if name != "" {
		s.Attrs["model"] = name
	}
	s.Attrs["prompt_tokens"] = strconv.Itoa(mo.TokenUsage.PromptTokens)
	s.Attrs["completion_tokens"] = strconv.Itoa(mo.TokenUsage.CompletionTokens)
	s.Attrs["total_tokens"] = strconv.Itoa(mo.TokenUsage.TotalTokens)
}
