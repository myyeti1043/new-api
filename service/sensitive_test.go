package service

import (
	"testing"

	"github.com/QuantumNous/new-api/setting"
)

func TestSensitiveWordContains_Empty(t *testing.T) {
	if ok, _ := SensitiveWordContains(""); ok {
		t.Errorf("empty text should not match")
	}
}

func TestSensitiveWordContains_Match(t *testing.T) {
	// 注入测试词表（用公共方法保证走线程安全路径）
	setting.SetSensitiveWords([]string{"badword", "forbidden"})
	defer setting.SetSensitiveWords(nil)

	ok, words := SensitiveWordContains("this contains badword here")
	if !ok {
		t.Fatalf("expected match for badword")
	}
	found := false
	for _, w := range words {
		if w == "badword" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected badword in hits, got %v", words)
	}
}

func TestSensitiveWordContains_CaseInsensitive(t *testing.T) {
	setting.SetSensitiveWords([]string{"BadWord"})
	defer setting.SetSensitiveWords(nil)

	ok, _ := SensitiveWordContains("text BADWORD here")
	if !ok {
		t.Errorf("case-insensitive match should work")
	}
}

func TestSensitiveWordContains_UnicodeBypass(t *testing.T) {
	setting.SetSensitiveWords([]string{"badword"})
	defer setting.SetSensitiveWords(nil)

	// 零宽字符插在中间
	bypass := "b​ad‌word"
	ok, _ := SensitiveWordContains(bypass)
	if !ok {
		t.Errorf("zero-width bypass should be detected after normalization, got false")
	}
}

func TestSensitiveWordContains_FullwidthBypass(t *testing.T) {
	setting.SetSensitiveWords([]string{"badword"})
	defer setting.SetSensitiveWords(nil)

	// 全角字母绕过
	bypass := "ｂａｄｗｏｒｄ"
	ok, _ := SensitiveWordContains(bypass)
	if !ok {
		t.Errorf("NFKC bypass should be detected, got false")
	}
}

func TestCheckSensitiveTextWithLevel_BlockFromFlatList(t *testing.T) {
	setting.SetSensitiveWords([]string{"evil"})
	setting.SetSensitiveRules(nil)
	defer setting.SetSensitiveWords(nil)

	hits := CheckSensitiveTextWithLevel("the evil text", "")
	if len(hits) == 0 {
		t.Fatalf("expected at least one hit")
	}
	if hits[0].Level != "block" {
		t.Errorf("flat list should default to block, got %q", hits[0].Level)
	}
}

func TestCheckSensitiveTextWithLevel_RulesWithGroup(t *testing.T) {
	setting.SetSensitiveRules([]setting.SensitiveRuleEntry{
		{Word: "warnme", Level: "warn", Category: "test", Group: "g1"},
		{Word: "blockme", Level: "block", Category: "test", Group: "g1"},
	})
	setting.SetSensitiveWords(nil)
	defer setting.SetSensitiveRules(nil)

	// 空 group: 应当匹配所有分组
	hits := CheckSensitiveTextWithLevel("text warnme and blockme", "")
	if len(hits) < 2 {
		t.Fatalf("expected >=2 hits, got %d: %+v", len(hits), hits)
	}
	var warnHit, blockHit bool
	for _, h := range hits {
		if h.Word == "warnme" && h.Level == "warn" {
			warnHit = true
		}
		if h.Word == "blockme" && h.Level == "block" {
			blockHit = true
		}
	}
	if !warnHit || !blockHit {
		t.Errorf("missing expected hits, got %+v", hits)
	}
}

func TestCheckSensitiveTextWithLevel_GroupFilter(t *testing.T) {
	setting.SetSensitiveRules([]setting.SensitiveRuleEntry{
		{Word: "ing1", Level: "warn", Group: "g1"},
		{Word: "ing2", Level: "warn", Group: "g2"},
	})
	setting.SetSensitiveWords(nil)
	defer setting.SetSensitiveRules(nil)

	hits := CheckSensitiveTextWithLevel("text ing1 ing2", "g1")
	for _, h := range hits {
		if h.Word == "ing2" {
			t.Errorf("group filter failed: got hit for ing2 under g1: %+v", h)
		}
	}
}
