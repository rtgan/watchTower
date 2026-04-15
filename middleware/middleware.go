package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var allowedOrigins = map[string]struct{}{
	"https://app.example.com": {},
	"http://localhost:8080":   {},
}

func isAllowedOrigin(origin string) bool {
	if _, ok := allowedOrigins[origin]; ok {
		return true
	}
	// 开发阶段：允许所有 localhost（任意端口）；上线时可删除此判断
	if strings.HasPrefix(origin, "http://localhost:") || strings.HasPrefix(origin, "http://127.0.0.1:") {
		return true
	}
	return false
}

/*
跨域策略是通过「在响应里写上这些头」告诉浏览器的.流程可以理解为：
step1：浏览器（或 JS）发请求时，可能会带上请求头 Origin，表示「我是从哪个网站来的」（例如 https://a.com）。
step2：你的服务器处理请求后，在响应里写上：①Access-Control-Allow-Origin————允许哪个 origin 读这个响应；②以及方法、头、是否带 cookie 等。
step3：浏览器根据响应头决定：前端 JS 能不能拿到响应体、能不能带 cookie 等。
*/
func CORSMiddleware(ctx *gin.Context) {
	origin := ctx.Request.Header.Get("Origin")

	// OPTIONS 预检必须在中间件里拦截，不能落到路由（路由只注册了 POST，OPTIONS 会 404）
	if ctx.Request.Method == http.MethodOptions {
		if origin != "" {
			if _, ok := allowedOrigins[origin]; ok {
				setCORSHeaders(ctx, origin)
			}
		}
		ctx.AbortWithStatus(http.StatusNoContent) // 204，无论是否白名单都要终止，避免落到路由然后 404
		return
	}

	// 非 OPTIONS 请求
	if origin == "" {
		ctx.Next() //没有Writer.Header().Setxxx到的跨域请求「响应了也不交给 JS」，报错 CORS policy blocked
		return
	}
	if _, ok := allowedOrigins[origin]; !ok {
		ctx.Next() //不在白名单：不写 Allow-Origin，浏览器会对该前端隐藏跨域响应
		return
	}
	setCORSHeaders(ctx, origin)
	ctx.Next()
}

func setCORSHeaders(ctx *gin.Context, origin string) {
	h := ctx.Writer.Header()
	h.Set("Access-Control-Allow-Origin", origin)
	h.Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
	h.Set("Access-Control-Allow-Credentials", "true")
}
