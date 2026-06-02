package service

import (
	"regexp"
	"strings"
)

// PIIFinding 表示一条 PII 检测结果
type PIIFinding struct {
	Type     string // phone, idcard, bankcard, email, ipv4
	Start    int    // 在原文中的起始位置（rune index）
	End      int    // 在原文中的结束位置（rune index，不含）
	Original string // 原始匹配文本
}

// piiPatterns 按类型存储编译后的正则
var piiPatterns = map[string]*regexp.Regexp{
	"phone":    regexp.MustCompile(`1[3-9]\d{9}`),
	"idcard":   regexp.MustCompile(`[1-9]\d{5}(19|20)\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[\dXx]`),
	"bankcard": regexp.MustCompile(`[1-9]\d{15,18}`),
	"email":    regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
	"ipv4":     regexp.MustCompile(`((25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(25[0-5]|2[0-4]\d|[01]?\d\d?)`),
}

// CheckPIIText 检测文本中的 PII 信息
// types 为要检测的类型列表，为空时检测所有类型
func CheckPIIText(text string, types []string) []PIIFinding {
	if len(text) == 0 {
		return nil
	}

	checkTypes := types
	if len(checkTypes) == 0 {
		checkTypes = []string{"phone", "idcard", "bankcard", "email", "ipv4"}
	}

	runes := []rune(text)
	var findings []PIIFinding

	for _, piiType := range checkTypes {
		pattern, ok := piiPatterns[piiType]
		if !ok {
			continue
		}
		matches := pattern.FindAllStringIndex(text, -1)
		for _, match := range matches {
			// 将 byte index 转换为 rune index
			start := len([]rune(text[:match[0]]))
			end := len([]rune(text[:match[1]]))
			findings = append(findings, PIIFinding{
				Type:     piiType,
				Start:    start,
				End:      end,
				Original: string(runes[start:end]),
			})
		}
	}

	return findings
}

// MaskPIIText 将文本中的 PII 替换为掩码
func MaskPIIText(text string, findings []PIIFinding) string {
	if len(findings) == 0 {
		return text
	}

	runes := []rune(text)
	var builder strings.Builder
	builder.Grow(len(text))

	lastEnd := 0
	for _, f := range findings {
		if f.Start < lastEnd {
			continue // 跳过重叠区域
		}
		builder.WriteString(string(runes[lastEnd:f.Start]))
		mask := buildMask(f)
		builder.WriteString(mask)
		lastEnd = f.End
	}
	builder.WriteString(string(runes[lastEnd:]))

	return builder.String()
}

// buildMask 根据 PII 类型生成掩码
func buildMask(finding PIIFinding) string {
	switch finding.Type {
	case "phone":
		runes := []rune(finding.Original)
		if len(runes) >= 7 {
			return string(runes[:3]) + "****" + string(runes[len(runes)-4:])
		}
		return "****"
	case "idcard":
		runes := []rune(finding.Original)
		if len(runes) >= 10 {
			return string(runes[:6]) + "********" + string(runes[len(runes)-4:])
		}
		return "********"
	case "bankcard":
		runes := []rune(finding.Original)
		if len(runes) >= 8 {
			return string(runes[:4]) + " **** **** " + string(runes[len(runes)-4:])
		}
		return "****"
	case "email":
		parts := strings.Split(finding.Original, "@")
		if len(parts) == 2 {
			name := parts[0]
			if len([]rune(name)) > 2 {
				runes := []rune(name)
				return string(runes[:2]) + "***@" + parts[1]
			}
			return "***@" + parts[1]
		}
		return "***"
	case "ipv4":
		parts := strings.Split(finding.Original, ".")
		if len(parts) == 4 {
			return parts[0] + "." + parts[1] + ".*.*"
		}
		return "***"
	default:
		return "****"
	}
}

// GetDefaultPIITypes 返回默认启用的 PII 类型列表
func GetDefaultPIITypes() []string {
	return []string{"phone", "idcard", "bankcard", "email", "ipv4"}
}
