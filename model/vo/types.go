package vo

type ChatReq struct {
	Id       string `json:"Id"`
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
	Result string   `json:"result"`
	Detail []string `json:"detail"`
}

type FileUploadReq struct {
}
type FileUploadRes struct {
	FileName string `json:"fileName" dc:"保存的文件名"`
	FilePath string `json:"filePath" dc:"文件保存路径"`
	FileSize int64  `json:"fileSize" dc:"文件大小(字节)"`
}
