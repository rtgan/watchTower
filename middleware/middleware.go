package middleware

import "github.com/gin-gonic/gin"

var allowedOrigins = map[string]struct{}{
	"https://app.example.com": {},
	"http://localhost:8000":   {},
}

/*
跨域策略是通过「在响应里写上这些头」告诉浏览器的.流程可以理解为：
step1：浏览器（或 JS）发请求时，可能会带上请求头 Origin，表示「我是从哪个网站来的」（例如 https://a.com）。
step2：你的服务器处理请求后，在响应里写上：①Access-Control-Allow-Origin————允许哪个 origin 读这个响应；②以及方法、头、是否带 cookie 等。
step3：浏览器根据响应头决定：前端 JS 能不能拿到响应体、能不能带 cookie 等。
*/
func CORSMiddleware(ctx *gin.Context) {
	origin := ctx.Request.Header.Get("Origin")
	if origin == "" {
		ctx.Next() //没有Writer.Header().Setxxx到的跨域请求「响应了也不交给 JS」，报错 CORS policy blocked
		return
	}
	if _, ok := allowedOrigins[origin]; !ok {
		// 不在白名单：不写 Allow-Origin，浏览器会对该前端隐藏跨域响应
		ctx.Next() //没有Writer.Header().Setxxx到的跨域请求「响应了也不交给 JS」，报错 CORS policy blocked
		return
	}
	ctx.Writer.Header().Set("Access-Control-Allow-Origin", origin)
	ctx.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	ctx.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
	ctx.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
	ctx.Next()
}
