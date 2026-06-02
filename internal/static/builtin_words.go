package static

import (
	_ "embed"
	"strings"
)

//go:embed builtin_words.txt
var builtinWordsRaw string

// GetBuiltinWords 解析内置敏感词库，返回去重后的词列表
// # 开头的行为注释，空行自动跳过
func GetBuiltinWords() []string {
	lines := strings.Split(builtinWordsRaw, "\n")
	seen := make(map[string]bool)
	var words []string

	for _, line := range lines {
		word := strings.TrimSpace(line)
		if word == "" || strings.HasPrefix(word, "#") {
			continue
		}
		word = strings.ToLower(word)
		if !seen[word] {
			seen[word] = true
			words = append(words, word)
		}
	}

	return words
}
