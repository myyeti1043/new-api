package service

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting"
)

// SensitiveHit 表示一条敏感词命中结果
type SensitiveHit struct {
	Word     string // 命中的敏感词
	Level    string // 级别：block/warn/log
	Category string // 分类标签
}

func CheckSensitiveMessages(messages []dto.Message) ([]string, error) {
	if len(messages) == 0 {
		return nil, nil
	}

	for _, message := range messages {
		arrayContent := message.ParseContent()
		for _, m := range arrayContent {
			if m.Type == "image_url" {
				// TODO: check image url
				continue
			}
			// 检查 text 是否为空
			if m.Text == "" {
				continue
			}
			if ok, words := SensitiveWordContains(m.Text); ok {
				return words, errors.New("sensitive words detected")
			}
		}
	}
	return nil, nil
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
	checkText := strings.ToLower(text)
	return AcSearch(checkText, words, true)
}

// CheckSensitiveTextWithLevel 检测文本中的敏感词，返回命中列表（含 Level）
// group 为空时匹配所有分组的规则
func CheckSensitiveTextWithLevel(text string, group string) []SensitiveHit {
	if len(text) == 0 {
		return nil
	}

	checkText := strings.ToLower(text)
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
		ruleWord := strings.ToLower(rule.Word)
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

// SensitiveWordReplace 敏感词替换，返回是否包含敏感词和替换后的文本
func SensitiveWordReplace(text string, returnImmediately bool) (bool, []string, string) {
	words := setting.GetSensitiveWords()
	if len(words) == 0 {
		return false, nil, text
	}
	checkText := strings.ToLower(text)
	m := getOrBuildAC(words)
	hits := m.MultiPatternSearch([]rune(checkText), returnImmediately)
	if len(hits) > 0 {
		words := make([]string, 0, len(hits))
		var builder strings.Builder
		builder.Grow(len(text))
		lastPos := 0

		for _, hit := range hits {
			pos := hit.Pos
			word := string(hit.Word)
			builder.WriteString(text[lastPos:pos])
			builder.WriteString("**###**")
			lastPos = pos + len(word)
			words = append(words, word)
		}
		builder.WriteString(text[lastPos:])
		return true, words, builder.String()
	}
	return false, nil, text
}
