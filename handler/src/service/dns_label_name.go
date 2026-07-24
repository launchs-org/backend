package service

import (
	"errors"
	"regexp"
)

// ErrInvalidResourceName はリソース名が DNS ラベルの形式に違反している場合のエラー
var ErrInvalidResourceName = errors.New("invalid name: must be lowercase alphanumeric or hyphens, max 63 chars, no leading/trailing hyphens")

// dnsLabelNamePattern は有効なリソース名のパターン（英小文字・数字・ハイフン、先頭末尾はハイフン不可、最大63文字）
// k8s の DNS ラベル制約（RFC 1123）に合わせているため、日本語・大文字・記号・空白は全て拒否される
var dnsLabelNamePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// validateDNSLabelName は name が DNS ラベル形式に一致するか検証する
func validateDNSLabelName(name string) error {
	if !dnsLabelNamePattern.MatchString(name) { // パターンに一致しない場合はエラーを返す
		return ErrInvalidResourceName
	}
	return nil // 一致した場合は nil を返す
}
