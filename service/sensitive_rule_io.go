package service

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/setting"
)

// ExportRulesToCSV 将敏感词规则导出为 CSV 格式
// 列：word, level, category, group
func ExportRulesToCSV() ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// 写入表头
	if err := writer.Write([]string{"word", "level", "category", "group"}); err != nil {
		return nil, fmt.Errorf("write csv header: %w", err)
	}

	for _, rule := range setting.SensitiveRules {
		record := []string{rule.Word, rule.Level, rule.Category, rule.Group}
		if err := writer.Write(record); err != nil {
			return nil, fmt.Errorf("write csv record: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("csv flush: %w", err)
	}

	return buf.Bytes(), nil
}

// ImportRulesFromCSV 从 CSV 数据导入敏感词规则
// 返回导入的规则数和错误
func ImportRulesFromCSV(data []byte, group string) (int, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	records, err := reader.ReadAll()
	if err != nil {
		return 0, fmt.Errorf("read csv: %w", err)
	}

	if len(records) == 0 {
		return 0, fmt.Errorf("csv is empty")
	}

	// 检查是否有表头，有则跳过
	startIdx := 0
	if len(records[0]) >= 2 && records[0][0] == "word" && records[0][1] == "level" {
		startIdx = 1
	}

	imported := 0
	for i := startIdx; i < len(records); i++ {
		record := records[i]
		if len(record) < 2 {
			continue
		}
		word := strings.TrimSpace(record[0])
		level := strings.TrimSpace(record[1])
		if word == "" || level == "" {
			continue
		}
		// 验证 level
		if level != "block" && level != "warn" && level != "log" {
			level = "block" // 默认 block
		}
		category := ""
		if len(record) > 2 {
			category = strings.TrimSpace(record[2])
		}
		ruleGroup := group
		if len(record) > 3 && strings.TrimSpace(record[3]) != "" {
			ruleGroup = strings.TrimSpace(record[3])
		}

		rule := setting.SensitiveRuleEntry{
			Word:     word,
			Level:    level,
			Category: category,
			Group:    ruleGroup,
		}
		setting.SensitiveRules = append(setting.SensitiveRules, rule)
		imported++
	}

	return imported, nil
}

// ImportRulesFromTXT 从 TXT 数据导入敏感词规则（每行一个词，默认 block 级别）
func ImportRulesFromTXT(data []byte, group string) (int, error) {
	lines := strings.Split(string(data), "\n")
	imported := 0

	for _, line := range lines {
		word := strings.TrimSpace(line)
		if word == "" || strings.HasPrefix(word, "#") {
			continue
		}
		word = strings.ToLower(word)

		rule := setting.SensitiveRuleEntry{
			Word:  word,
			Level: "block",
			Group: group,
		}
		setting.SensitiveRules = append(setting.SensitiveRules, rule)
		imported++
	}

	return imported, nil
}
