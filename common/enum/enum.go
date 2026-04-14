package enum

import "strings"

type Enum map[int]string

func (e Enum) Code(val string) int {
	if val == "" {
		return -1 //不存在或值无效
	}
	val = strings.TrimSpace(val)
	for c, v := range e {
		if val == v {
			return c
		}
	}
	return -1 //不存在或值无效
}

func (e Enum) Value(code int) string {
	if val, ok := e[code]; ok {
		return val
	}
	return ""
}
