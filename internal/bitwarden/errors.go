package bitwarden

import (
	"errors"
	"fmt"
)

// 失败原因的分类
const (
	errKindConfig   = "config"   // 属性没配
	errKindAuth     = "auth"     // 凭据不对（Client ID/Secret 或主密码）
	errKindNetwork  = "network"  // 连不上服务端
	errKindUpstream = "upstream" // 服务端返回异常（HTTP 5xx、返回内容不合预期）
	errKindInternal = "internal" // 内部问题（写库、备份）
)

// kindedError 带分类的错误
type kindedError struct {
	kind string
	hint string
	msg  string
}

func (e *kindedError) Error() string { return e.msg }

func errOf(kind, hint, format string, args ...interface{}) error {
	return &kindedError{kind: kind, hint: hint, msg: fmt.Sprintf(format, args...)}
}

// classify 把错误映射成（分类, 建议）
func classify(err error) (kind, hint string) {
	var ke *kindedError
	if errors.As(err, &ke) && ke.kind != "" {
		return ke.kind, ke.hint
	}
	return errKindInternal, "查看系统日志原文"
}
