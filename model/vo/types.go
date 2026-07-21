package vo

type ChatReq struct {
	Id       string `json:"Id"`
	UserId   string `json:"UserId"` // 用户标识，用于跨会话长期记忆；为空时仅使用会话级记忆
	Question string `json:"Question"`
}
type ChatRes struct {
	Answer string `json:"answer"`
}

type ChatStreamReq struct {
	Id       string `json:"Id"`
	Question string `json:"Question"`
}
type ChatStreamRes struct {
}

type AIOpsReq struct {
}
type AIOpsRes struct {
	Result  string   `json:"result"`
	Detail []string `json:"detail"`
	TraceID string  `json:"trace_id,omitempty"` // 对应 /api/traces/:id，便于排查本次诊断
}

type FileUploadReq struct {
}
type FileUploadRes struct {
	FileName string `json:"fileName" dc:"保存的文件名"`
	FilePath string `json:"filePath" dc:"文件保存路径"`
	FileSize int64  `json:"fileSize" dc:"文件大小(字节)"`
}
