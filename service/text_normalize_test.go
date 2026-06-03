package service

import "testing"

func TestNormalizeForSensitiveScan_Empty(t *testing.T) {
	if got := normalizeForSensitiveScan(""); got != "" {
		t.Errorf("empty should stay empty, got %q", got)
	}
}

func TestNormalizeForSensitiveScan_Lowercases(t *testing.T) {
	got := normalizeForSensitiveScan("Hello WORLD")
	if got != "hello world" {
		t.Errorf("expected lowercased, got %q", got)
	}
}

func TestNormalizeForSensitiveScan_StripsZeroWidth(t *testing.T) {
	// ZWSP (U+200B), ZWNJ (U+200C), ZWJ (U+200D), WORD JOINER (U+2060), BOM (U+FEFF)
	// 用 string(rune) 构造，避免源码里直接出现让 Go 词法器误判的字节
	zws := string([]rune{0x200B, 0x200C, 0x200D, 0x2060, 0xFEFF})
	got := normalizeForSensitiveScan("a" + zws + "b")
	if got != "ab" {
		t.Errorf("zero-width should be stripped, got %q", got)
	}
}

func TestNormalizeForSensitiveScan_NFKC(t *testing.T) {
	// 全角字母 + 全角数字
	full := "bad123" // NFKC 折叠后
	_ = full
	// 直接用 NFKC 输入：ASCII 是规范形式；为能验证，用全角数字
	fullwidthDigits := string([]rune{0xFF11, 0xFF12, 0xFF13})
	got := normalizeForSensitiveScan(fullwidthDigits)
	if got != "123" {
		t.Errorf("expected NFKC to fold fullwidth digits, got %q", got)
	}
}

func TestNormalizeForSensitiveScan_PreservesCasingAscii(t *testing.T) {
	got := normalizeForSensitiveScan("Already")
	if got != "already" {
		t.Errorf("expected lowercase, got %q", got)
	}
}

func TestContainsFormat(t *testing.T) {
	if containsFormat("hello world") {
		t.Errorf("plain ASCII should not trigger containsFormat")
	}
	if !containsFormat("h" + string(rune(0x200B))) {
		t.Errorf("ZWSP should trigger containsFormat")
	}
	if !containsFormat(string(rune(0xFEFF))) {
		t.Errorf("BOM should trigger containsFormat")
	}
}
