package service

import (
	"regexp"
	"strings"
)

// PIIFinding 表示一条 PII 检测结果
type PIIFinding struct {
	Type     string // phone, idcard, bankcard, email, ipv4
	Start    int    // 在原文（已归一化）中的起始位置（rune index）
	End      int    // 在原文（已归一化）中的结束位置（rune index，不含）
	Original string // 归一化文本中的匹配片段
}

// piiRawPatterns 用于初筛的正则：所有匹配都再用 post-filter 校验，
// 既能保留正则的高性能，又能通过 Go RE2 不支持 look-around 的限制。
//
// 关键约束：Go 的 regexp/re2 不支持 (?<!\d) / (?!\d) 之类的 lookaround，
// 所以边界检查 + 校验位/算法验证都在 Go 代码里手工完成。
var piiRawPatterns = map[string]*regexp.Regexp{
	"phone":    regexp.MustCompile(`1[3-9]\d{9}`),
	"idcard":   regexp.MustCompile(`[1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]`),
	"bankcard": regexp.MustCompile(`[1-9]\d{15,18}`),
	"email":    regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`),
	"ipv4":     regexp.MustCompile(`(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.(?:25[0-5]|2[0-4]\d|[01]?\d\d?)`),
}

// isDigitByte 判断 byte 是否是十进制数字。
func isDigitByte(b byte) bool { return b >= '0' && b <= '9' }

// isWordByte 判断 byte 是否是单词字符（数字/字母）。
func isWordByte(b byte) bool {
	return isDigitByte(b) || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

// hasWordBoundary 返回在 text 字节串中，[start,end) 区间的左右是否都没有
// 单词字符（"边界" = 字符串头 / 字符串尾 / 非单词字符）。
// 用于在不依赖 lookaround 的前提下过滤掉"嵌在长串里"的误命中。
func hasWordBoundary(text string, start, end int) bool {
	if start > 0 && isWordByte(text[start-1]) {
		return false
	}
	if end < len(text) && isWordByte(text[end]) {
		return false
	}
	return true
}

// hasInvalidNeighbor 判断 [start,end) 区间的左侧紧邻字符是否是特定的
// "非独立使用"标志（用于 ipv4 排除 ip:port、url 参数等）。
func hasInvalidNeighbor(text string, start, end int) bool {
	if start > 0 {
		switch text[start-1] {
		case ':', '=', '/', '?', '#', '%':
			return true
		}
	}
	if end < len(text) {
		switch text[end] {
		case ':', '=', '/', '?', '#', '%':
			return true
		}
	}
	return false
}

// validateCNIDChecksum 校验 18 位中国大陆居民身份证号最后一位的 mod-11 加权校验码。
// 加权因子: Wi = 2^(17-i) mod 11；校验码 = (12 - sum(Wi*Ai) mod 11) mod 11，
// 对应字符表 "10X98765432"。
func validateCNIDChecksum(id string) bool {
	if len(id) != 18 {
		return false
	}
	weights := [17]int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
	mapping := "10X98765432"
	sum := 0
	for i := 0; i < 17; i++ {
		c := id[i]
		if !isDigitByte(c) {
			return false
		}
		sum += int(c-'0') * weights[i]
	}
	expected := mapping[sum%11]
	last := id[17]
	if last == 'x' || last == 'X' {
		last = 'X'
	}
	return string(last) == string(expected)
}

// validateLuhn 校验数字串是否符合 Luhn 算法（mod-10）。
// 银行卡 / 身份证号尾段常用此校验。
func validateLuhn(s string) bool {
	if len(s) < 12 || len(s) > 19 {
		return false
	}
	sum := 0
	alt := false
	for i := len(s) - 1; i >= 0; i-- {
		c := s[i]
		if !isDigitByte(c) {
			return false
		}
		n := int(c - '0')
		if alt {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		alt = !alt
	}
	return sum%10 == 0
}

// validateEmailShape 对邮箱做最简合法性检查：拆分后两侧都非空，
// 顶级域长度 ≥ 2，且本地部分不含控制字符。
func validateEmailShape(email string) bool {
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		return false
	}
	local, domain := email[:at], email[at+1:]
	if local == "" || domain == "" {
		return false
	}
	if !strings.Contains(domain, ".") {
		return false
	}
	if len(domain) < 3 {
		// a@b 这种不算
		return false
	}
	return true
}

// CheckPIIText 检测文本中的 PII 信息。
// types 为要检测的类型列表，为空时检测所有类型。
//
// 处理流程：
//  1. NFKC + 零宽字符归一化（抵御同形 / 不可见分隔符绕过）；
//  2. 正则粗筛；
//  3. word boundary + 校验位/Luhn 精筛（剔除伪命中）。
//
// 返回的 PIIFinding.Start/End 是归一化字符串上的 rune index；
// Original 是归一化后的原文片段。
func CheckPIIText(text string, types []string) []PIIFinding {
	if len(text) == 0 {
		return nil
	}

	normalized := normalizeForPIIScan(text)
	if normalized == "" {
		return nil
	}

	checkTypes := types
	if len(checkTypes) == 0 {
		checkTypes = []string{"phone", "idcard", "bankcard", "email", "ipv4"}
	}

	runes := []rune(normalized)
	var findings []PIIFinding

	for _, piiType := range checkTypes {
		pattern, ok := piiRawPatterns[piiType]
		if !ok {
			continue
		}
		matches := pattern.FindAllStringIndex(normalized, -1)
		for _, match := range matches {
			startByte, endByte := match[0], match[1]
			// 1) word boundary：剔除"嵌在长串里"的伪命中
			if !hasWordBoundary(normalized, startByte, endByte) {
				continue
			}
			candidate := normalized[startByte:endByte]

			// 2) 类型特定的进一步校验
			switch piiType {
			case "phone":
				// phone 字段是 11 位手机号，必须是纯数字
				if !isAllDigits(candidate) {
					continue
				}
			case "idcard":
				// 18 位身份证：mod-11 校验
				if !validateCNIDChecksum(candidate) {
					continue
				}
			case "bankcard":
				// 16~19 位银行卡：Luhn 校验
				if !validateLuhn(candidate) {
					continue
				}
			case "email":
				if !validateEmailShape(candidate) {
					continue
				}
			case "ipv4":
				// 排除 URL 端口 / query 参数的误命中
				if hasInvalidNeighbor(normalized, startByte, endByte) {
					continue
				}
			}

			// 把 byte index 转为 rune index（按归一化字符串算）
			startRune := len([]rune(normalized[:startByte]))
			endRune := startRune + len([]rune(candidate))
			if endRune > len(runes) {
				endRune = len(runes)
			}
			findings = append(findings, PIIFinding{
				Type:     piiType,
				Start:    startRune,
				End:      endRune,
				Original: string(runes[startRune:endRune]),
			})
		}
	}

	return findings
}

// isAllDigits 字符串是否全是 ASCII 数字。
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isDigitByte(s[i]) {
			return false
		}
	}
	return true
}

// MaskPIIText 将文本中的 PII 替换为掩码。
// 接受原始文本（未归一化），但 findings 区间以归一化文本为基准 —
// 调用方应先在归一化文本上识别 findings，再用同一归一化文本调此函数。
// 命中区间用 rune index 表达。
func MaskPIIText(text string, findings []PIIFinding) string {
	if len(findings) == 0 {
		return text
	}

	runes := []rune(text)
	var builder strings.Builder
	builder.Grow(len(text))

	lastEnd := 0
	for _, f := range findings {
		if f.Start < lastEnd || f.Start < 0 || f.End > len(runes) {
			continue // 跳过重叠 / 越界
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
