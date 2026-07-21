package eval

import "strings"

// CheckResult 单项确定性检查结果。
type CheckResult struct {
	Pass   bool     `json:"pass"`
	Hit    []string `json:"hit"`     // 命中的项
	Missed []string `json:"missed"`  // 未命中的项
	Detail string   `json:"detail"`  // 人类可读说明
}

func newCheck(haystack string, want []string, label string) CheckResult {
	res := CheckResult{}
	low := strings.ToLower(haystack)
	for _, w := range want {
		if w == "" {
			continue
		}
		if strings.Contains(low, strings.ToLower(w)) {
			res.Hit = append(res.Hit, w)
		} else {
			res.Missed = append(res.Missed, w)
		}
	}
	res.Pass = len(res.Missed) == 0
	res.Detail = label + ": " + summarize(res.Hit, res.Missed)
	return res
}

func summarize(hit, missed []string) string {
	if len(missed) == 0 {
		return "all hit (" + strings.Join(hit, ", ") + ")"
	}
	return "hit=[" + strings.Join(hit, ", ") + "] missed=[" + strings.Join(missed, ", ") + "]"
}

// joinDetail 把 agent 事件流拼成一个字符串，供子串检索。
func joinDetail(detail []string) string {
	return strings.ToLower(strings.Join(detail, "\n"))
}

// CheckTools 检查 MustCallTools 是否都出现在轨迹里。
func CheckTools(detail []string, must []string) CheckResult {
	return newCheck(joinDetail(detail), must, "tools")
}

// CheckKeywords 检查 ExpectedKeywords 是否出现在轨迹或报告里（query_log 的入参会留在 detail）。
func CheckKeywords(detail []string, report string, keywords []string) CheckResult {
	hay := joinDetail(detail) + "\n" + strings.ToLower(report)
	return newCheck(hay, keywords, "keywords")
}

// CheckErrorCodes 检查 ExpectedErrorCodes 是否出现在报告里。
func CheckErrorCodes(report string, codes []string) CheckResult {
	return newCheck(report, codes, "error_codes")
}

// CheckRootCause 检查期望根因是否出现在报告里。
func CheckRootCause(report string, rootCause string) CheckResult {
	if rootCause == "" {
		return CheckResult{Pass: true, Detail: "root_cause: no expectation set"}
	}
	return newCheck(report, []string{rootCause}, "root_cause")
}
