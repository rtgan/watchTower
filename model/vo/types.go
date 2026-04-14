package vo

import "github.com/cloudwego/eino/schema"

type UserMessage struct {
	ID      string            `json:"Id"`
	Query   string            `json:"Question"`
	History []*schema.Message `json:"History"`
}
type ChatRes struct {
	Answer string `json:"answer"`
}
