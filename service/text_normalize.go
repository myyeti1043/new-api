package service

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// normalizeForSensitiveScan 对用于敏感词扫描的输入文本做防御性归一化：
//  1. NFKC 归一化：把全角/兼容字符（ＡＢＣ、全角空格、罗马数字等）映射到标准等价物；
//  2. 剥离 Unicode 零宽/格式字符（\p{Cf} 类别里的 ZWSP、ZWNJ、ZWJ、BOM、软连字等）。
//
// 这样常见的"同形/分隔"绕过会被还原成可匹配的形态。
//
// 注意：该函数只对用于匹配的副本做归一化，不会修改调用方传入的原始文本；
// 原始文本的命中区间仍以 rune index 表达，与归一化后的索引可能不再 1:1，
// 因此下游应当对归一化后的字符串独立计算。
func normalizeForSensitiveScan(text string) string {
	if text == "" {
		return text
	}
	// 1. NFKC
	s := norm.NFKC.String(text)
	// 2. 剥离 \p{Cf}
	if !containsFormat(s) {
		return strings.ToLower(s)
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.Is(unicode.Cf, r) {
			continue
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}

// normalizeForPIIScan 在敏感词归一化基础上额外把字符串按 rune 切片返回。
// PII 检测同时需要原始 byte 索引与归一化文本，因此调用方拿到的 text
// 应当来自该函数返回的归一化字符串（而不是原始输入），索引也相应地用
// 归一化字符串重新计算。
func normalizeForPIIScan(text string) string {
	return normalizeForSensitiveScan(text)
}

// containsFormat 快速判断字符串中是否存在 \p{Cf} 字符，
// 在没有命中时省去分配 Builder 的开销。
func containsFormat(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Cf, r) {
			return true
		}
	}
	return false
}
