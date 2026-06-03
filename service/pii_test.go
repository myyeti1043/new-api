package service

import (
	"strings"
	"testing"
)

func TestCheckPIIText_Phone_Basic(t *testing.T) {
	cases := []struct {
		name      string
		text      string
		wantTypes []string
	}{
		{"valid 11-digit", "call me at 13800138000", []string{"phone"}},
		{"valid with parens", "(13800138000) ok", []string{"phone"}},
		{"embedded in long number rejected", "id 113800138000999 is bad", nil},
		{"too short rejected", "138001", nil},
		{"invalid prefix rejected", "12800138000", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CheckPIIText(tc.text, nil)
			if len(got) == 0 && len(tc.wantTypes) == 0 {
				return
			}
			if len(got) != len(tc.wantTypes) {
				t.Fatalf("got %d hits, want %d (text=%q)", len(got), len(tc.wantTypes), tc.text)
			}
			for i, w := range tc.wantTypes {
				if got[i].Type != w {
					t.Errorf("hit[%d].Type = %q, want %q", i, got[i].Type, w)
				}
			}
		})
	}
}

func TestCheckPIIText_IDCard_Valid(t *testing.T) {
	// 11010519491231002X — 校验位计算：sum mod 11 = 8 → 'X'
	id := "11010519491231002X"
	got := CheckPIIText("id:"+id, nil)
	if len(got) == 0 || got[0].Type != "idcard" {
		t.Fatalf("expected idcard hit, got %+v", got)
	}
	// Original 是归一化文本上的切片（lowercased），校验位小写 x
	if got[0].Original != "11010519491231002x" {
		t.Errorf("Original = %q, want lowercased 11010519491231002x", got[0].Original)
	}
}

func TestCheckPIIText_IDCard_InvalidChecksum(t *testing.T) {
	// 同号但末位改成 0 — 校验位应当是 'X' 而不是 '0'
	id := "110105194912310020"
	got := CheckPIIText("id:"+id, nil)
	for _, f := range got {
		if f.Type == "idcard" {
			t.Errorf("expected no idcard hit for bad checksum, got %+v", f)
		}
	}
}

func TestCheckPIIText_Bankcard_Luhn(t *testing.T) {
	// 测试用 Luhn 合法卡号（虚构）
	valid := "4111111111111111"
	if !validateLuhn(valid) {
		t.Fatalf("setup: 4111111111111111 should be Luhn-valid")
	}
	got := CheckPIIText("card:"+valid, nil)
	found := false
	for _, f := range got {
		if f.Type == "bankcard" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected bankcard hit for %s, got %+v", valid, got)
	}

	// 同长度但 Luhn 失败
	invalid := "4111111111111112"
	if validateLuhn(invalid) {
		t.Fatalf("setup: 4111111111111112 should be Luhn-invalid")
	}
	got = CheckPIIText("card:"+invalid, nil)
	for _, f := range got {
		if f.Type == "bankcard" {
			t.Errorf("expected no bankcard hit for bad Luhn, got %+v", f)
		}
	}
}

func TestCheckPIIText_Email(t *testing.T) {
	cases := []struct {
		text   string
		expect bool
	}{
		{"mail me at user@example.com", true},
		{"a@b.co", true},
		{"not an email", false},
		{"missing domain @example.com", false},
		{"missing local user@", false},
		{"no tld user@example", false},
	}
	for _, tc := range cases {
		got := CheckPIIText(tc.text, []string{"email"})
		hit := len(got) > 0
		if hit != tc.expect {
			t.Errorf("text=%q got hit=%v, want %v", tc.text, hit, tc.expect)
		}
	}
}

func TestCheckPIIText_IPv4_ExcludesPort(t *testing.T) {
	cases := []struct {
		text   string
		expect bool
	}{
		{"server 192.168.1.1 reachable", true},
		{"connect to 10.0.0.1:8080", false},
		{"ip=10.0.0.1&port=80", false},
		{"path /1.2.3.4/file", false},
	}
	for _, tc := range cases {
		got := CheckPIIText(tc.text, []string{"ipv4"})
		hit := len(got) > 0
		if hit != tc.expect {
			t.Errorf("text=%q got hit=%v, want %v", tc.text, hit, tc.expect)
		}
	}
}

func TestCheckPIIText_UnicodeBypass(t *testing.T) {
	// 零宽字符插在号码中间，应被归一化后命中
	withZWS := "call ​13‍800‌138​000"
	got := CheckPIIText(withZWS, nil)
	if len(got) == 0 {
		t.Fatalf("zero-width bypass should still be detected: %+v", got)
	}
	if got[0].Type != "phone" {
		t.Errorf("expected phone hit, got %+v", got[0])
	}
}

func TestCheckPIIText_UnicodeNFKC(t *testing.T) {
	// 全角 11 位手机号
	fullwidth := "call "
	for _, r := range "13800138000" {
		fullwidth += string(rune(int(r) - '0' + 0xFF10))
	}
	got := CheckPIIText(fullwidth, nil)
	if len(got) == 0 || got[0].Type != "phone" {
		t.Fatalf("expected NFKC normalized phone hit, got %+v", got)
	}
	if got[0].Original != "13800138000" {
		t.Errorf("expected folded to ASCII, got %q", got[0].Original)
	}
}

func TestMaskPIIText_Phone(t *testing.T) {
	findings := CheckPIIText("call 13800138000", nil)
	masked := MaskPIIText("call 13800138000", findings)
	if strings.Contains(masked, "13800138000") {
		t.Errorf("phone should be masked, got %q", masked)
	}
	if !strings.Contains(masked, "****") {
		t.Errorf("masked output should contain ****, got %q", masked)
	}
}

func TestMaskPIIText_NoOverlap(t *testing.T) {
	findings := []PIIFinding{
		{Type: "phone", Start: 0, End: 11, Original: "13800138000"},
		{Type: "phone", Start: 5, End: 16, Original: "13800138000"}, // 重叠
	}
	masked := MaskPIIText("13800138000 extra", findings)
	if strings.Count(masked, "****") != 1 {
		t.Errorf("overlapping findings should produce single mask, got %q", masked)
	}
}

func TestMaskPIIText_OutOfRange(t *testing.T) {
	// findings.End 越界应被忽略，不应 panic
	findings := []PIIFinding{
		{Type: "phone", Start: 0, End: 9999, Original: "13800138000"},
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("MaskPIIText panicked on out-of-range: %v", r)
		}
	}()
	_ = MaskPIIText("short", findings)
}

func TestValidateCNIDChecksum(t *testing.T) {
	cases := []struct {
		id   string
		want bool
	}{
		{"11010519491231002X", true},
		{"110105194912310020", false},
		{"11010519491231003X", false}, // 错的校验位
		{"", false},
		{"12345", false}, // 长度不对
	}
	for _, tc := range cases {
		if got := validateCNIDChecksum(tc.id); got != tc.want {
			t.Errorf("validateCNIDChecksum(%q) = %v, want %v", tc.id, got, tc.want)
		}
	}
}

func TestValidateLuhn(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"4111111111111111", true},
		{"5500000000000004", true},
		{"340000000000009", true},
		{"4111111111111112", false},
		{"1234567812345670", true},
		{"1234567812345678", false},
		{"abc123", false},
		{"", false},
		{"12", false}, // 太短
	}
	for _, tc := range cases {
		if got := validateLuhn(tc.s); got != tc.want {
			t.Errorf("validateLuhn(%q) = %v, want %v", tc.s, got, tc.want)
		}
	}
}
