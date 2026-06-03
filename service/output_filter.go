package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// OutputFilterResult 输出过滤检查结果
type OutputFilterResult struct {
	// ShouldBlock 命中 block 级别，请求应当阻断
	ShouldBlock bool
	// ShouldMask 命中 mask 级别，请求体需要脱敏
	ShouldMask bool
	// SensitiveHits 命中的敏感词
	SensitiveHits []SensitiveHit
	// SensitiveBlockedHits 命中 block 级别的敏感词
	SensitiveBlockedHits []SensitiveHit
	// PIIFindings 命中的 PII
	PIIFindings []PIIFinding
	// PIIMaskedText 脱敏后的完整文本（仅在 ShouldMask=true 时有效）
	PIIMaskedText string
}

// HasAnyHit 是否有任何命中（block 或 mask）
func (r *OutputFilterResult) HasAnyHit() bool {
	return r != nil && (len(r.SensitiveHits) > 0 || len(r.PIIFindings) > 0)
}

// ToAPIError 将阻断结果转换为 NewAPIError
func (r *OutputFilterResult) ToAPIError(reason string) *types.NewAPIError {
	if r == nil || !r.ShouldBlock {
		return nil
	}
	if reason == "" {
		reason = "output filtered by sensitive rule / PII"
	}
	return types.NewError(errors.New(reason), types.ErrorCodeSensitiveWordsDetected)
}

// ApplyOutputFilter 对 info.OutputResponseText 中累积的输出文本执行敏感词/PII 检查。
// 必须在写客户端前调用以保证非流式响应可在写前阻断/脱敏。
func ApplyOutputFilter(c *gin.Context, info *relaycommon.RelayInfo) OutputFilterResult {
	var result OutputFilterResult
	text := info.OutputResponseText.String()
	if text == "" {
		return result
	}

	// 1. 敏感词检查（分级）
	if setting.ShouldCheckCompletionSensitive() {
		hits := CheckSensitiveTextWithLevel(text, info.TokenGroup)
		if len(hits) > 0 {
			result.SensitiveHits = hits
			for _, hit := range hits {
				if hit.Level == "block" {
					result.ShouldBlock = true
					result.SensitiveBlockedHits = append(result.SensitiveBlockedHits, hit)
				}
			}
		}
	}

	// 2. PII 检查（per-type action）
	if setting.PIIConfig.Enabled {
		enabledTypes := setting.GetEnabledPIITypes()
		if len(enabledTypes) > 0 {
			findings := CheckPIIText(text, enabledTypes)
			if len(findings) > 0 {
				result.PIIFindings = findings
				for _, f := range findings {
					action := setting.GetPIITypeAction(f.Type, "output")
					switch action {
					case "block":
						result.ShouldBlock = true
					case "mask":
						result.ShouldMask = true
					}
				}
				if result.ShouldMask {
					result.PIIMaskedText = MaskPIIText(text, findings)
				}
			}
		}
	}

	if result.HasAnyHit() {
		logger.LogWarn(c, fmt.Sprintf("output filter hits: sensitive=%d pii=%d block=%v mask=%v",
			len(result.SensitiveHits), len(result.PIIFindings), result.ShouldBlock, result.ShouldMask))
	}

	return result
}

// ApplyInputPIIFilter 对输入文本执行 PII 检查。
// 独立于敏感词检查和 token 计数；按每种 PII 类型的 action 配置分别处理。
func ApplyInputPIIFilter(text string) (shouldBlock, shouldMask bool, maskedText string, findings []PIIFinding) {
	if !setting.PIIConfig.Enabled || text == "" {
		return false, false, text, nil
	}
	enabledTypes := setting.GetEnabledPIITypes()
	if len(enabledTypes) == 0 {
		return false, false, text, nil
	}
	findings = CheckPIIText(text, enabledTypes)
	if len(findings) == 0 {
		return false, false, text, nil
	}
	for _, f := range findings {
		action := setting.GetPIITypeAction(f.Type, "input")
		switch action {
		case "block":
			shouldBlock = true
		case "mask":
			shouldMask = true
		}
	}
	if shouldMask {
		maskedText = MaskPIIText(text, findings)
	}
	return
}

// MaskOpenAIRequestMessages 按 PII findings 对 OpenAI 风格 messages 的 text 段做脱敏。
// 仅对 text 类型 content 起作用（不修改 image_url、input_audio、file、video_url）。
// 返回是否实际进行了脱敏。
func MaskOpenAIRequestMessages(messages []dto.Message, findings []PIIFinding) (bool, []int) {
	if len(findings) == 0 || len(messages) == 0 {
		return false, nil
	}
	maskedAny := false
	maskedIndices := make([]int, 0)
	for i := range messages {
		switch m := messages[i].Content.(type) {
		case string:
			if m == "" {
				continue
			}
			masked := MaskPIIText(m, findings)
			if masked != m {
				messages[i].SetStringContent(masked)
				maskedAny = true
				maskedIndices = append(maskedIndices, i)
			}
		case []any:
			arr := m
			changed := false
			for j, item := range arr {
				cm, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if cm["type"] == dto.ContentTypeText {
					if text, ok := cm["text"].(string); ok && text != "" {
						newText := MaskPIIText(text, findings)
						if newText != text {
							cm["text"] = newText
							arr[j] = cm
							changed = true
						}
					}
				}
			}
			if changed {
				messages[i].Content = arr
				maskedAny = true
				maskedIndices = append(maskedIndices, i)
			}
		}
	}
	return maskedAny, maskedIndices
}

// MaskOpenAIRequestPrompt 对 OpenAI 风格 prompt 字段做 PII 脱敏
// prompt 可能是 string 或 []any（字符串数组）
func MaskOpenAIRequestPrompt(prompt any, findings []PIIFinding) (any, bool) {
	if len(findings) == 0 || prompt == nil {
		return prompt, false
	}
	switch v := prompt.(type) {
	case string:
		masked := MaskPIIText(v, findings)
		if masked != v {
			return masked, true
		}
	case []any:
		changed := false
		for i, item := range v {
			if s, ok := item.(string); ok {
				masked := MaskPIIText(s, findings)
				if masked != s {
					v[i] = masked
					changed = true
				}
			}
		}
		if changed {
			return v, true
		}
	}
	return prompt, false
}

// RebuildOpenAIResponseBodyWithMaskedText 重新构造 responseBody，使 choices 中的文本被 PIIMaskedText 替换。
// simpleResponse: 解析后的 OpenAI 响应
// originalBody: 当前 responseBody（包含 usage 等）
// maskedText: 整段 response text 经 PII 脱敏后的字符串
//
// 算法：扫描 simpleResponse.Choices，按文本在 response text 序列中的位置
// 顺序切片 maskedText，替换到对应 choice。
func RebuildOpenAIResponseBodyWithMaskedText(simpleResponse interface{}, originalBody []byte, maskedText string) []byte {
	type messageLike struct {
		Content interface{} `json:"content"`
	}
	type choiceLike struct {
		Message messageLike `json:"message"`
	}
	type respLike struct {
		Choices []choiceLike `json:"choices"`
	}
	var r respLike
	if err := common.Unmarshal(originalBody, &r); err != nil {
		// fallback：原样返回
		return originalBody
	}
	if len(r.Choices) == 0 {
		return originalBody
	}
	// 将 maskedText 按原始每条 choice 的文本长度切分
	cursor := 0
	mrunes := []rune(maskedText)
	for i := range r.Choices {
		original := extractChoiceText(r.Choices[i].Message.Content)
		end := cursor + len([]rune(original))
		if end > len(mrunes) {
			end = len(mrunes)
		}
		if cursor >= len(mrunes) {
			r.Choices[i].Message.Content = ""
		} else {
			r.Choices[i].Message.Content = string(mrunes[cursor:end])
		}
		cursor = end
	}
	// 重新 marshal，但保留原始的其它字段（usage, id, object, created, model 等）
	var top map[string]interface{}
	if err := common.Unmarshal(originalBody, &top); err != nil {
		return originalBody
	}
	// 替换 choices
	choicesRaw, _ := common.Marshal(r.Choices)
	var choicesAny []interface{}
	if err := common.Unmarshal(choicesRaw, &choicesAny); err == nil {
		top["choices"] = choicesAny
	}
	out, err := common.Marshal(top)
	if err != nil {
		return originalBody
	}
	return out
}

// extractChoiceText 抽取 choice.message.content 的文本表示
func extractChoiceText(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		var sb strings.Builder
		for _, item := range v {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if m["type"] == "text" {
				if t, ok := m["text"].(string); ok {
					sb.WriteString(t)
				}
			}
		}
		return sb.String()
	}
	return ""
}

// RealtimeDeltaFilter 用于 Realtime WebSocket 逐 delta 过滤。
//  - MaskedDelta: 经过 PII 脱敏后的 delta（首尾可能有差异）；
//  - ShouldBlock: 累计敏感词/PII 触发阻断，应当向客户端发送 error 事件并停止；
//  - BlockReason: 阻断原因（用于 error 事件 message 字段）。
type RealtimeDeltaFilter struct {
	MaskedDelta string
	ShouldBlock bool
	BlockReason string
}

// ApplyRealtimeDeltaFilter 对 Realtime WebSocket 的一条文本 delta 做 PII / 敏感词检查。
//
//  - delta: 来自 `response.text.delta` / `response.audio_transcript.delta` 等事件；
//  - info:  当前请求的 RelayInfo，OutputResponseText 用于累积；
//  - 返回:  过滤结果，调用方决定是否改写 delta 或终止连接。
//
// 行为约定：
//  1. 总是把 delta 追加到 info.OutputResponseText（用 masked 后的版本以保证后续
//     敏感词累计检查看到的是脱敏后的内容，而不是原始 PII 触发假阳性）；
//  2. PII 配置启用时，delta 走 PII 引擎脱敏，区间由原始 delta 决定；
//  3. 累计文本命中 block 级敏感词时返回 ShouldBlock=true；
//  4. PII per-type action=block 时返回 ShouldBlock=true。
func ApplyRealtimeDeltaFilter(c *gin.Context, info *relaycommon.RelayInfo, delta string) RealtimeDeltaFilter {
	res := RealtimeDeltaFilter{MaskedDelta: delta}
	if delta == "" {
		return res
	}

	// 1. PII 引擎对 delta 脱敏（区间按 delta 自身计算）
	if setting.PIIConfig.Enabled {
		enabledTypes := setting.GetEnabledPIITypes()
		if len(enabledTypes) > 0 {
			findings := CheckPIIText(delta, enabledTypes)
			if len(findings) > 0 {
				hasMask := false
				for _, f := range findings {
					action := setting.GetPIITypeAction(f.Type, "output")
					switch action {
					case "block":
						res.ShouldBlock = true
						res.BlockReason = fmt.Sprintf("PII type %s blocked", f.Type)
					case "mask":
						hasMask = true
					}
				}
				if hasMask && !res.ShouldBlock {
					res.MaskedDelta = MaskPIIText(delta, findings)
				}
			}
		}
	}

	if res.ShouldBlock {
		return res
	}

	// 2. 把 masked delta 累积到 info
	if info != nil {
		info.OutputResponseText.WriteString(res.MaskedDelta)
		// 3. 对累计文本做敏感词 block 检查
		if setting.ShouldCheckCompletionSensitive() {
			hits := CheckSensitiveTextWithLevel(info.OutputResponseText.String(), info.TokenGroup)
			for _, hit := range hits {
				if hit.Level == "block" {
					res.ShouldBlock = true
					res.BlockReason = fmt.Sprintf("sensitive word %q blocked", hit.Word)
					break
				}
			}
		}
	}

	if res.ShouldBlock {
		logger.LogWarn(c, fmt.Sprintf("realtime output block: %s", res.BlockReason))
	}
	return res
}
