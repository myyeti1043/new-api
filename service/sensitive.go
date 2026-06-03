package service

import (
	"strings"

	"github.com/QuantumNous/new-api/setting"
)

// SensitiveHit 表示一条敏感词命中结果
type SensitiveHit struct {
	Word     string // 命中的敏感词
	Level    string // 级别：block/warn/log
	Category string // 分类标签
}

func CheckSensitiveText(text string) (bool, []string) {
	return SensitiveWordContains(text)
}

// SensitiveWordContains 是否包含敏感词，返回是否包含敏感词和敏感词列表
func SensitiveWordContains(text string) (bool, []string) {
	words := setting.GetSensitiveWords()
	if len(words) == 0 {
		return false, nil
	}
	if len(text) == 0 {
		return false, nil
	}
	checkText := normalizeForSensitiveScan(text)
	return AcSearch(checkText, words, true)
}

// CheckSensitiveTextWithLevel 检测文本中的敏感词，返回命中列表（含 Level）
// group 为空时匹配所有分组的规则
func CheckSensitiveTextWithLevel(text string, group string) []SensitiveHit {
	if len(text) == 0 {
		return nil
	}

	checkText := normalizeForSensitiveScan(text)
	var hits []SensitiveHit

	// 1. 检查旧版扁平敏感词列表（视为 block 级别）
	words := setting.GetSensitiveWords()
	if len(words) > 0 {
		if ok, hits2 := AcSearch(checkText, words, true); ok {
			for _, word := range hits2 {
				hits = append(hits, SensitiveHit{
					Word:  word,
					Level: "block",
				})
			}
		}
	}

	// 2. 检查分级敏感词规则
	rules := setting.GetSensitiveRulesByGroup(group)
	for _, rule := range rules {
		ruleWord := normalizeForSensitiveScan(strings.ToLower(rule.Word))
		if ruleWord == "" {
			continue
		}
		if strings.Contains(checkText, ruleWord) {
			hits = append(hits, SensitiveHit{
				Word:     rule.Word,
				Level:    rule.Level,
				Category: rule.Category,
			})
		}
	}

	return hits
}
