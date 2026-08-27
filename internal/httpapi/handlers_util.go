package httpapi

import (
	"html"
	"strconv"
	"strings"
)

// atoiSafe 将字符串安全转为 int64（失败返回 0）。
func atoiSafe(s string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// itoaSafe 将 int64 转为字符串。
func itoaSafe(n int64) string {
	return strconv.FormatInt(n, 10)
}

// escapeHTML 对文本做 HTML 转义，防止复核页注入。
func escapeHTML(s string) string {
	return html.EscapeString(s)
}
