package openai

import (
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func OaiResponsesCompactionHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var compactResp dto.OpenAIResponsesCompactionResponse
	if err := common.Unmarshal(responseBody, &compactResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError := compactResp.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	// 累积响应文本到 RelayInfo，供输出端敏感词/PII 检查
	if info != nil {
		// Output 字段是 json.RawMessage，递归提取其中的 text 字段
		extractTextFromRawJSON(compactResp.Output, info.OutputResponseText)
		// 写客户端前执行输出过滤
		if filterResult := service.ApplyOutputFilter(c, info); filterResult.HasAnyHit() {
			if filterResult.ShouldBlock {
				return nil, filterResult.ToAPIError("output sensitive words detected")
			}
			if filterResult.ShouldMask {
				responseBody = service.RebuildOpenAIResponseBodyWithMaskedText(compactResp, responseBody, filterResult.PIIMaskedText)
			}
		}
	}

	service.IOCopyBytesGracefully(c, resp, responseBody)

	usage := dto.Usage{}
	if compactResp.Usage != nil {
		usage.PromptTokens = compactResp.Usage.InputTokens
		usage.CompletionTokens = compactResp.Usage.OutputTokens
		usage.TotalTokens = compactResp.Usage.TotalTokens
		if compactResp.Usage.InputTokensDetails != nil {
			usage.PromptTokensDetails.CachedTokens = compactResp.Usage.InputTokensDetails.CachedTokens
		}
	}

	return &usage, nil
}

// extractTextFromRawJSON 递归遍历任意 JSON 数据，提取所有 string 类型的 "text" 字段值，
// 拼接到 builder 中。用于 compaction 这种 response 字段是 json.RawMessage 的场景。
func extractTextFromRawJSON(data []byte, builder interface{ WriteString(string) (int, error) }) {
	if len(data) == 0 {
		return
	}
	var v interface{}
	if err := common.Unmarshal(data, &v); err != nil {
		return
	}
	collectTextFromValue(v, builder)
}

func collectTextFromValue(v interface{}, builder interface{ WriteString(string) (int, error) }) {
	switch val := v.(type) {
	case map[string]interface{}:
		// text 字段直接抽取
		if t, ok := val["text"].(string); ok && t != "" {
			builder.WriteString(t)
		}
		// 继续遍历其它字段
		for _, child := range val {
			collectTextFromValue(child, builder)
		}
	case []interface{}:
		for _, item := range val {
			collectTextFromValue(item, builder)
		}
	}
}
