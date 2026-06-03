package controller

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/samber/lo"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func relayHandler(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	var err *types.NewAPIError
	switch info.RelayMode {
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		err = relay.ImageHelper(c, info)
	case relayconstant.RelayModeAudioSpeech:
		fallthrough
	case relayconstant.RelayModeAudioTranslation:
		fallthrough
	case relayconstant.RelayModeAudioTranscription:
		err = relay.AudioHelper(c, info)
	case relayconstant.RelayModeRerank:
		err = relay.RerankHelper(c, info)
	case relayconstant.RelayModeEmbeddings:
		err = relay.EmbeddingHelper(c, info)
	case relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesCompact:
		err = relay.ResponsesHelper(c, info)
	default:
		err = relay.TextHelper(c, info)
	}
	return err
}

func geminiRelayHandler(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	var err *types.NewAPIError
	if strings.Contains(c.Request.URL.Path, "embed") {
		err = relay.GeminiEmbeddingHandler(c, info)
	} else {
		err = relay.GeminiHelper(c, info)
	}
	return err
}

func Relay(c *gin.Context, relayFormat types.RelayFormat) {

	requestId := c.GetString(common.RequestIdKey)
	//group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	//originalModel := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)

	var (
		newAPIError *types.NewAPIError
		ws          *websocket.Conn
	)

	if relayFormat == types.RelayFormatOpenAIRealtime {
		var err error
		ws, err = upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			helper.WssError(c, ws, types.NewError(err, types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry()).ToOpenAIError())
			return
		}
		defer ws.Close()
	}

	defer func() {
		if newAPIError != nil {
			logger.LogError(c, fmt.Sprintf("relay error: %s", common.LocalLogPreview(newAPIError.Error())))
			newAPIError.SetMessage(common.MessageWithRequestId(newAPIError.Error(), requestId))
			switch relayFormat {
			case types.RelayFormatOpenAIRealtime:
				helper.WssError(c, ws, newAPIError.ToOpenAIError())
			case types.RelayFormatClaude:
				c.JSON(newAPIError.StatusCode, gin.H{
					"type":  "error",
					"error": newAPIError.ToClaudeError(),
				})
			default:
				c.JSON(newAPIError.StatusCode, gin.H{
					"error": newAPIError.ToOpenAIError(),
				})
			}
		}
	}()

	request, err := helper.GetAndValidateRequest(c, relayFormat)
	if err != nil {
		// Map "request body too large" to 413 so clients can handle it correctly
		if common.IsRequestBodyTooLargeError(err) || errors.Is(err, common.ErrRequestBodyTooLarge) {
			newAPIError = types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
		} else {
			newAPIError = types.NewError(err, types.ErrorCodeInvalidRequest)
		}
		return
	}

	relayInfo, err := relaycommon.GenRelayInfo(c, relayFormat, request, ws)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeGenRelayInfoFailed)
		return
	}

	needSensitiveCheck := setting.ShouldCheckPromptSensitive()
	needCountToken := constant.CountToken
	// Avoid building huge CombineText (strings.Join) when token counting and sensitive check are both disabled.
	var meta *types.TokenCountMeta
	if needSensitiveCheck || needCountToken || setting.PIIConfig.Enabled {
		meta = request.GetTokenCountMeta()
	} else {
		meta = fastTokenCountMetaForPricing(request)
	}

	if needSensitiveCheck && meta != nil {
		hits := service.CheckSensitiveTextWithLevel(meta.CombineText, relayInfo.TokenGroup)
		if len(hits) > 0 {
			// 提取所有命中的词
			words := make([]string, 0, len(hits))
			for _, hit := range hits {
				words = append(words, hit.Word)
			}
			logger.LogWarn(c, fmt.Sprintf("user sensitive words detected: %s", strings.Join(words, ", ")))

			// 判断最高级别
			hasBlock := false
			maxLevel := "log"
			for _, hit := range hits {
				if hit.Level == "block" {
					hasBlock = true
					maxLevel = "block"
				} else if hit.Level == "warn" && maxLevel != "block" {
					maxLevel = "warn"
				}
			}

			// 记录审计日志（异步，不阻塞请求）
			userId := c.GetInt("id")
			username := c.GetString("username")
			userGroup := c.GetString("group")
			requestId := c.GetString(common.RequestIdKey)
			tokenName := c.GetString("token_name")
			modelName := relayInfo.OriginModelName
			needRecordIp := false
			if settingMap, ipErr := model.GetUserSetting(userId, false); ipErr == nil && settingMap.RecordIpLog {
				needRecordIp = true
			}
			auditDetail := model.AuditLogDetail{
				Direction:      "input",
				SensitiveWords: words,
				Action: func() string {
					if hasBlock {
						return "blocked"
					}
					return "warned"
				}(),
				ContentPreview: truncateAndMaskContent(meta.CombineText, 200),
				RuleLevel:      maxLevel,
				Category:       hits[0].Category,
			}
			auditLog := &model.Log{
				UserId:    userId,
				Username:  username,
				CreatedAt: common.GetTimestamp(),
				Type:      model.LogTypeAudit,
				Content:   "sensitive words detected",
				TokenName: tokenName,
				ModelName: modelName,
				Group:     userGroup,
				Ip: func() string {
					if needRecordIp {
						return c.ClientIP()
					}
					return ""
				}(),
				RequestId: requestId,
			}
			if detailBytes, err := common.Marshal(auditDetail); err == nil {
				auditLog.Other = string(detailBytes)
			}
			service.RecordAuditLog(auditLog)

			// block 级别阻断请求
			if hasBlock {
				newAPIError = types.NewError(errors.New("sensitive words detected"), types.ErrorCodeSensitiveWordsDetected)
				return
			}
		}
	}

	// 输入端 PII 检查（独立于敏感词检查和 token 计数）
	if setting.PIIConfig.Enabled && meta != nil && meta.CombineText != "" {
		enabledTypes := setting.GetEnabledPIITypes()
		if len(enabledTypes) > 0 {
			// 使用 per-type action 处理
			shouldBlock, shouldMask, _, findings := service.ApplyInputPIIFilter(meta.CombineText)
			if len(findings) > 0 {
				// 记录审计日志
				piiTypes := make([]string, 0, len(findings))
				for _, f := range findings {
					piiTypes = append(piiTypes, f.Type)
				}
				logger.LogWarn(c, fmt.Sprintf("PII detected in input: %s", strings.Join(piiTypes, ", ")))

				// 决定最终 action 描述
				actionDesc := "log"
				if shouldBlock {
					actionDesc = "blocked"
				} else if shouldMask {
					actionDesc = "masked"
				}

				if setting.PIIConfig.AuditLogEnabled {
					userId := c.GetInt("id")
					username := c.GetString("username")
					userGroup := c.GetString("group")
					requestId := c.GetString(common.RequestIdKey)
					tokenName := c.GetString("token_name")
					modelName := relayInfo.OriginModelName
					needRecordIp := false
					if settingMap, ipErr := model.GetUserSetting(userId, false); ipErr == nil && settingMap.RecordIpLog {
						needRecordIp = true
					}
					auditDetail := model.AuditLogDetail{
						Direction:      "input",
						SensitiveWords: piiTypes,
						Action:         actionDesc,
						ContentPreview: truncateAndMaskContent(meta.CombineText, 200),
						RuleLevel:      actionDesc,
						Category:       "pii",
					}
					auditLog := &model.Log{
						UserId:    userId,
						Username:  username,
						CreatedAt: common.GetTimestamp(),
						Type:      model.LogTypeAudit,
						Content:   "PII detected in input",
						TokenName: tokenName,
						ModelName: modelName,
						Group:     userGroup,
						Ip: func() string {
							if needRecordIp {
								return c.ClientIP()
							}
							return ""
						}(),
						RequestId: requestId,
					}
					if detailBytes, err := common.Marshal(auditDetail); err == nil {
						auditLog.Other = string(detailBytes)
					}
					service.RecordAuditLog(auditLog)
				}

				// block 级别阻断请求
				if shouldBlock {
					newAPIError = types.NewError(errors.New("PII detected in input"), types.ErrorCodeSensitiveWordsDetected)
					return
				}
				// mask 级别：实际改写请求体（messages/prompt 等）
				if shouldMask && len(findings) > 0 {
					if request == nil {
						logger.LogWarn(c, "PII mask requested but request is nil")
					} else if !applyPIIMaskToRequest(request, findings) {
						logger.LogWarn(c, "PII mask: no text fields were updated on the request")
					}
				}
			}
		}
	}

	tokens, err := service.EstimateRequestToken(c, meta, relayInfo)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeCountTokenFailed)
		return
	}

	relayInfo.SetEstimatePromptTokens(tokens)

	priceData, err := helper.ModelPriceHelper(c, relayInfo, tokens, meta)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithStatusCode(http.StatusBadRequest))
		return
	}

	// common.SetContextKey(c, constant.ContextKeyTokenCountMeta, meta)

	if priceData.FreeModel {
		logger.LogInfo(c, fmt.Sprintf("模型 %s 免费，跳过预扣费", relayInfo.OriginModelName))
	} else {
		newAPIError = service.PreConsumeBilling(c, priceData.QuotaToPreConsume, relayInfo)
		if newAPIError != nil {
			return
		}
	}

	defer func() {
		// Only return quota if downstream failed and quota was actually pre-consumed
		if newAPIError != nil {
			newAPIError = service.NormalizeViolationFeeError(newAPIError)
			if relayInfo.Billing != nil {
				relayInfo.Billing.Refund(c)
			}
			service.ChargeViolationFeeIfNeeded(c, relayInfo, newAPIError)
		}
	}()

	retryParam := &service.RetryParam{
		Ctx:        c,
		TokenGroup: relayInfo.TokenGroup,
		ModelName:  relayInfo.OriginModelName,
		Retry:      common.GetPointer(0),
	}
	relayInfo.RetryIndex = 0
	relayInfo.LastError = nil

	for ; retryParam.GetRetry() <= common.RetryTimes; retryParam.IncreaseRetry() {
		relayInfo.RetryIndex = retryParam.GetRetry()
		channel, channelErr := getChannel(c, relayInfo, retryParam)
		if channelErr != nil {
			logger.LogError(c, channelErr.Error())
			newAPIError = channelErr
			break
		}

		addUsedChannel(c, channel.Id)
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			// Ensure consistent 413 for oversized bodies even when error occurs later (e.g., retry path)
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				newAPIError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
			} else {
				newAPIError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			}
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)

		switch relayFormat {
		case types.RelayFormatOpenAIRealtime:
			newAPIError = relay.WssHelper(c, relayInfo)
		case types.RelayFormatClaude:
			newAPIError = relay.ClaudeHelper(c, relayInfo)
		case types.RelayFormatGemini:
			newAPIError = geminiRelayHandler(c, relayInfo)
		default:
			newAPIError = relayHandler(c, relayInfo)
		}

		if newAPIError == nil {
			relayInfo.LastError = nil

			// 输出端敏感词检查（per-rule level）
			// 注意：对于非流式响应，handler 已经在写客户端前完成了 block/mask 处理；
			// 此处保留作为防御性兜底，主要处理流式场景（流式仅做审计日志）。
			if setting.ShouldCheckCompletionSensitive() {
				outputText := relayInfo.OutputResponseText.String()
				if outputText != "" {
					hits := service.CheckSensitiveTextWithLevel(outputText, relayInfo.TokenGroup)
					if len(hits) > 0 {
						words := make([]string, 0, len(hits))
						for _, hit := range hits {
							words = append(words, hit.Word)
						}
						logger.LogWarn(c, fmt.Sprintf("output sensitive words detected: %s", strings.Join(words, ", ")))

						// 判断最高级别
						hasBlock := false
						maxLevel := "log"
						for _, hit := range hits {
							if hit.Level == "block" {
								hasBlock = true
								maxLevel = "block"
							} else if hit.Level == "warn" && maxLevel != "block" {
								maxLevel = "warn"
							}
						}

						// 记录审计日志
						userId := c.GetInt("id")
						username := c.GetString("username")
						userGroup := c.GetString("group")
						requestId := c.GetString(common.RequestIdKey)
						tokenName := c.GetString("token_name")
						modelName := relayInfo.OriginModelName
						needRecordIp := false
						if settingMap, ipErr := model.GetUserSetting(userId, false); ipErr == nil && settingMap.RecordIpLog {
							needRecordIp = true
						}
						auditDetail := model.AuditLogDetail{
							Direction:      "output",
							SensitiveWords: words,
							Action: func() string {
								if hasBlock {
									return "blocked"
								}
								return "warned"
							}(),
							ContentPreview: truncateAndMaskContent(outputText, 200),
							RuleLevel:      maxLevel,
							Category:       hits[0].Category,
						}
						auditLog := &model.Log{
							UserId:    userId,
							Username:  username,
							CreatedAt: common.GetTimestamp(),
							Type:      model.LogTypeAudit,
							Content:   "output sensitive words detected",
							TokenName: tokenName,
							ModelName: modelName,
							Group:     userGroup,
							Ip: func() string {
								if needRecordIp {
									return c.ClientIP()
								}
								return ""
							}(),
							RequestId: requestId,
						}
						if detailBytes, err := common.Marshal(auditDetail); err == nil {
							auditLog.Other = string(detailBytes)
						}
						service.RecordAuditLog(auditLog)

						// 非流式请求且 block 级别时返回错误（handler 通常已先阻断；此处兜底）
						if hasBlock && !relayInfo.IsStream {
							newAPIError = types.NewError(errors.New("output sensitive words detected"), types.ErrorCodeSensitiveWordsDetected)
							return
						}
						// 流式请求或 warn/log 级别仅记录日志
					}
				}
			}

			// 输出端 PII 检查（per-type action）
			if setting.PIIConfig.Enabled {
				outputText := relayInfo.OutputResponseText.String()
				if outputText != "" {
					shouldBlock, shouldMask, _, findings := computeOutputPIIAction(outputText)
					_ = shouldMask // 非流式时 handler 已处理 mask
					if len(findings) > 0 {
						piiTypes := make([]string, 0, len(findings))
						for _, f := range findings {
							piiTypes = append(piiTypes, f.Type)
						}
						logger.LogWarn(c, fmt.Sprintf("PII detected in output: %s", strings.Join(piiTypes, ", ")))

						if setting.PIIConfig.AuditLogEnabled {
							userId := c.GetInt("id")
							username := c.GetString("username")
							userGroup := c.GetString("group")
							requestId := c.GetString(common.RequestIdKey)
							tokenName := c.GetString("token_name")
							modelName := relayInfo.OriginModelName
							needRecordIp := false
							if settingMap, ipErr := model.GetUserSetting(userId, false); ipErr == nil && settingMap.RecordIpLog {
								needRecordIp = true
							}
							actionDesc := "log"
							if shouldBlock {
								actionDesc = "blocked"
							} else if shouldMask {
								actionDesc = "masked"
							}
							auditDetail := model.AuditLogDetail{
								Direction:      "output",
								SensitiveWords: piiTypes,
								Action:         actionDesc,
								ContentPreview: truncateAndMaskContent(outputText, 200),
								RuleLevel:      actionDesc,
								Category:       "pii",
							}
							auditLog := &model.Log{
								UserId:    userId,
								Username:  username,
								CreatedAt: common.GetTimestamp(),
								Type:      model.LogTypeAudit,
								Content:   "PII detected in output",
								TokenName: tokenName,
								ModelName: modelName,
								Group:     userGroup,
								Ip: func() string {
									if needRecordIp {
										return c.ClientIP()
									}
									return ""
								}(),
								RequestId: requestId,
							}
							if detailBytes, err := common.Marshal(auditDetail); err == nil {
								auditLog.Other = string(detailBytes)
							}
							service.RecordAuditLog(auditLog)
						}

						// block 级别且非流式时阻断（handler 已先阻断；此处兜底）
						if shouldBlock && !relayInfo.IsStream {
							newAPIError = types.NewError(errors.New("PII detected in output"), types.ErrorCodeSensitiveWordsDetected)
							return
						}
					}
				}
			}

			return
		}

		newAPIError = service.NormalizeViolationFeeError(newAPIError)
		relayInfo.LastError = newAPIError

		processChannelError(c, *types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, common.GetContextKeyString(c, constant.ContextKeyChannelKey), channel.GetAutoBan()), newAPIError)

		if !shouldRetry(c, newAPIError, common.RetryTimes-retryParam.GetRetry()) {
			break
		}
	}

	useChannel := c.GetStringSlice("use_channel")
	if len(useChannel) > 1 {
		retryLogStr := fmt.Sprintf("重试：%s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		logger.LogInfo(c, retryLogStr)
	}
	if newAPIError != nil {
		gopool.Go(func() {
			perfmetrics.RecordRelaySample(relayInfo, false, 0)
		})
	}
}

var upgrader = websocket.Upgrader{
	Subprotocols: []string{"realtime"}, // WS 握手支持的协议，如果有使用 Sec-WebSocket-Protocol，则必须在此声明对应的 Protocol TODO add other protocol
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许跨域
	},
}

func addUsedChannel(c *gin.Context, channelId int) {
	useChannel := c.GetStringSlice("use_channel")
	useChannel = append(useChannel, fmt.Sprintf("%d", channelId))
	c.Set("use_channel", useChannel)
}

func fastTokenCountMetaForPricing(request dto.Request) *types.TokenCountMeta {
	if request == nil {
		return &types.TokenCountMeta{}
	}
	meta := &types.TokenCountMeta{
		TokenType: types.TokenTypeTokenizer,
	}
	switch r := request.(type) {
	case *dto.GeneralOpenAIRequest:
		maxCompletionTokens := lo.FromPtrOr(r.MaxCompletionTokens, uint(0))
		maxTokens := lo.FromPtrOr(r.MaxTokens, uint(0))
		if maxCompletionTokens > maxTokens {
			meta.MaxTokens = int(maxCompletionTokens)
		} else {
			meta.MaxTokens = int(maxTokens)
		}
	case *dto.OpenAIResponsesRequest:
		meta.MaxTokens = int(lo.FromPtrOr(r.MaxOutputTokens, uint(0)))
	case *dto.ClaudeRequest:
		meta.MaxTokens = int(lo.FromPtr(r.MaxTokens))
	case *dto.ImageRequest:
		// Pricing for image requests depends on ImagePriceRatio; safe to compute even when CountToken is disabled.
		return r.GetTokenCountMeta()
	default:
		// Best-effort: leave CombineText empty to avoid large allocations.
	}
	return meta
}

func getChannel(c *gin.Context, info *relaycommon.RelayInfo, retryParam *service.RetryParam) (*model.Channel, *types.NewAPIError) {
	if info.ChannelMeta == nil {
		autoBan := c.GetBool("auto_ban")
		autoBanInt := 1
		if !autoBan {
			autoBanInt = 0
		}
		return &model.Channel{
			Id:      c.GetInt("channel_id"),
			Type:    c.GetInt("channel_type"),
			Name:    c.GetString("channel_name"),
			AutoBan: &autoBanInt,
		}, nil
	}
	channel, selectGroup, err := service.CacheGetRandomSatisfiedChannel(retryParam)

	info.PriceData.GroupRatioInfo = helper.HandleGroupRatio(c, info)

	if err != nil {
		return nil, types.NewError(fmt.Errorf("获取分组 %s 下模型 %s 的可用渠道失败（retry）: %s", selectGroup, info.OriginModelName, err.Error()), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	if channel == nil {
		return nil, types.NewError(fmt.Errorf("分组 %s 下模型 %s 的可用渠道不存在（retry）", selectGroup, info.OriginModelName), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}

	newAPIError := middleware.SetupContextForSelectedChannel(c, channel, info.OriginModelName)
	if newAPIError != nil {
		return nil, newAPIError
	}
	return channel, nil
}

func shouldRetry(c *gin.Context, openaiErr *types.NewAPIError, retryTimes int) bool {
	if openaiErr == nil {
		return false
	}
	if service.ShouldSkipRetryAfterChannelAffinityFailure(c) {
		return false
	}
	if types.IsChannelError(openaiErr) {
		return true
	}
	if types.IsSkipRetryError(openaiErr) {
		return false
	}
	if retryTimes <= 0 {
		return false
	}
	if _, ok := c.Get("specific_channel_id"); ok {
		return false
	}
	code := openaiErr.StatusCode
	if code >= 200 && code < 300 {
		return false
	}
	if code < 100 || code > 599 {
		return true
	}
	if operation_setting.IsAlwaysSkipRetryCode(openaiErr.GetErrorCode()) {
		return false
	}
	return operation_setting.ShouldRetryByStatusCode(code)
}

func processChannelError(c *gin.Context, channelError types.ChannelError, err *types.NewAPIError) {
	logger.LogError(c, fmt.Sprintf("channel error (channel #%d, status code: %d): %s", channelError.ChannelId, err.StatusCode, common.LocalLogPreview(err.Error())))
	// 不要使用context获取渠道信息，异步处理时可能会出现渠道信息不一致的情况
	// do not use context to get channel info, there may be inconsistent channel info when processing asynchronously
	if service.ShouldDisableChannel(err) && channelError.AutoBan {
		gopool.Go(func() {
			service.DisableChannel(channelError, err.ErrorWithStatusCode())
		})
	}

	if constant.ErrorLogEnabled && types.IsRecordErrorLog(err) {
		// 保存错误日志到mysql中
		userId := c.GetInt("id")
		tokenName := c.GetString("token_name")
		modelName := c.GetString("original_model")
		tokenId := c.GetInt("token_id")
		userGroup := c.GetString("group")
		channelId := c.GetInt("channel_id")
		other := make(map[string]interface{})
		if c.Request != nil && c.Request.URL != nil {
			other["request_path"] = c.Request.URL.Path
		}
		other["error_type"] = err.GetErrorType()
		other["error_code"] = err.GetErrorCode()
		other["status_code"] = err.StatusCode
		other["channel_id"] = channelId
		other["channel_name"] = c.GetString("channel_name")
		other["channel_type"] = c.GetInt("channel_type")
		adminInfo := make(map[string]interface{})
		adminInfo["use_channel"] = c.GetStringSlice("use_channel")
		isMultiKey := common.GetContextKeyBool(c, constant.ContextKeyChannelIsMultiKey)
		if isMultiKey {
			adminInfo["is_multi_key"] = true
			adminInfo["multi_key_index"] = common.GetContextKeyInt(c, constant.ContextKeyChannelMultiKeyIndex)
		}
		service.AppendChannelAffinityAdminInfo(c, adminInfo)
		other["admin_info"] = adminInfo
		startTime := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
		if startTime.IsZero() {
			startTime = time.Now()
		}
		useTimeSeconds := int(time.Since(startTime).Seconds())
		model.RecordErrorLog(c, userId, channelId, modelName, tokenName, err.MaskSensitiveErrorWithStatusCode(), tokenId, useTimeSeconds, common.GetContextKeyBool(c, constant.ContextKeyIsStream), userGroup, other)
	}

}

func RelayMidjourney(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatMjProxy, nil, nil)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"description": fmt.Sprintf("failed to generate relay info: %s", err.Error()),
			"type":        "upstream_error",
			"code":        4,
		})
		return
	}

	var mjErr *dto.MidjourneyResponse
	switch relayInfo.RelayMode {
	case relayconstant.RelayModeMidjourneyNotify:
		mjErr = relay.RelayMidjourneyNotify(c)
	case relayconstant.RelayModeMidjourneyTaskFetch, relayconstant.RelayModeMidjourneyTaskFetchByCondition:
		mjErr = relay.RelayMidjourneyTask(c, relayInfo.RelayMode)
	case relayconstant.RelayModeMidjourneyTaskImageSeed:
		mjErr = relay.RelayMidjourneyTaskImageSeed(c)
	case relayconstant.RelayModeSwapFace:
		mjErr = relay.RelaySwapFace(c, relayInfo)
	default:
		mjErr = relay.RelayMidjourneySubmit(c, relayInfo)
	}
	//err = relayMidjourneySubmit(c, relayMode)
	log.Println(mjErr)
	if mjErr != nil {
		statusCode := http.StatusBadRequest
		if mjErr.Code == 30 {
			mjErr.Result = "当前分组负载已饱和，请稍后再试，或升级账户以提升服务质量。"
			statusCode = http.StatusTooManyRequests
		}
		c.JSON(statusCode, gin.H{
			"description": fmt.Sprintf("%s %s", mjErr.Description, mjErr.Result),
			"type":        "upstream_error",
			"code":        mjErr.Code,
		})
		channelId := c.GetInt("channel_id")
		logger.LogError(c, fmt.Sprintf("relay error (channel #%d, status code %d): %s", channelId, statusCode, fmt.Sprintf("%s %s", mjErr.Description, mjErr.Result)))
	}
}

func RelayNotImplemented(c *gin.Context) {
	err := types.OpenAIError{
		Message: "API not implemented",
		Type:    "new_api_error",
		Param:   "",
		Code:    "api_not_implemented",
	}
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": err,
	})
}

func RelayNotFound(c *gin.Context) {
	err := types.OpenAIError{
		Message: fmt.Sprintf("Invalid URL (%s %s)", c.Request.Method, c.Request.URL.Path),
		Type:    "invalid_request_error",
		Param:   "",
		Code:    "",
	}
	c.JSON(http.StatusNotFound, gin.H{
		"error": err,
	})
}

func RelayTaskFetch(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &dto.TaskError{
			Code:       "gen_relay_info_failed",
			Message:    err.Error(),
			StatusCode: http.StatusInternalServerError,
		})
		return
	}
	if taskErr := relay.RelayTaskFetch(c, relayInfo.RelayMode); taskErr != nil {
		respondTaskError(c, taskErr)
	}
}

func RelayTask(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &dto.TaskError{
			Code:       "gen_relay_info_failed",
			Message:    err.Error(),
			StatusCode: http.StatusInternalServerError,
		})
		return
	}

	if taskErr := relay.ResolveOriginTask(c, relayInfo); taskErr != nil {
		respondTaskError(c, taskErr)
		return
	}

	var result *relay.TaskSubmitResult
	var taskErr *dto.TaskError
	defer func() {
		if taskErr != nil && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
	}()

	retryParam := &service.RetryParam{
		Ctx:        c,
		TokenGroup: relayInfo.TokenGroup,
		ModelName:  relayInfo.OriginModelName,
		Retry:      common.GetPointer(0),
	}

	for ; retryParam.GetRetry() <= common.RetryTimes; retryParam.IncreaseRetry() {
		var channel *model.Channel

		if lockedCh, ok := relayInfo.LockedChannel.(*model.Channel); ok && lockedCh != nil {
			channel = lockedCh
			if retryParam.GetRetry() > 0 {
				if setupErr := middleware.SetupContextForSelectedChannel(c, channel, relayInfo.OriginModelName); setupErr != nil {
					taskErr = service.TaskErrorWrapperLocal(setupErr.Err, "setup_locked_channel_failed", http.StatusInternalServerError)
					break
				}
			}
		} else {
			var channelErr *types.NewAPIError
			channel, channelErr = getChannel(c, relayInfo, retryParam)
			if channelErr != nil {
				logger.LogError(c, channelErr.Error())
				taskErr = service.TaskErrorWrapperLocal(channelErr.Err, "get_channel_failed", http.StatusInternalServerError)
				break
			}
		}

		addUsedChannel(c, channel.Id)
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusRequestEntityTooLarge)
			} else {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusBadRequest)
			}
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)

		result, taskErr = relay.RelayTaskSubmit(c, relayInfo)
		if taskErr == nil {
			break
		}

		if !taskErr.LocalError {
			processChannelError(c,
				*types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey,
					common.GetContextKeyString(c, constant.ContextKeyChannelKey), channel.GetAutoBan()),
				types.NewOpenAIError(taskErr.Error, types.ErrorCodeBadResponseStatusCode, taskErr.StatusCode))
		}

		if !shouldRetryTaskRelay(c, channel.Id, taskErr, common.RetryTimes-retryParam.GetRetry()) {
			break
		}
	}

	useChannel := c.GetStringSlice("use_channel")
	if len(useChannel) > 1 {
		retryLogStr := fmt.Sprintf("重试：%s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		logger.LogInfo(c, retryLogStr)
	}

	// ── 成功：结算 + 日志 + 插入任务 ──
	if taskErr == nil {
		if settleErr := service.SettleBilling(c, relayInfo, result.Quota); settleErr != nil {
			common.SysError("settle task billing error: " + settleErr.Error())
		}
		service.LogTaskConsumption(c, relayInfo)

		task := model.InitTask(result.Platform, relayInfo)
		task.PrivateData.UpstreamTaskID = result.UpstreamTaskID
		task.PrivateData.BillingSource = relayInfo.BillingSource
		task.PrivateData.SubscriptionId = relayInfo.SubscriptionId
		task.PrivateData.TokenId = relayInfo.TokenId
		task.PrivateData.BillingContext = &model.TaskBillingContext{
			ModelPrice:      relayInfo.PriceData.ModelPrice,
			GroupRatio:      relayInfo.PriceData.GroupRatioInfo.GroupRatio,
			ModelRatio:      relayInfo.PriceData.ModelRatio,
			OtherRatios:     relayInfo.PriceData.OtherRatios,
			OriginModelName: relayInfo.OriginModelName,
			PerCallBilling:  common.StringsContains(constant.TaskPricePatches, relayInfo.OriginModelName) || relayInfo.PriceData.UsePrice,
		}
		task.Quota = result.Quota
		task.Data = result.TaskData
		task.Action = relayInfo.Action
		if insertErr := task.Insert(); insertErr != nil {
			common.SysError("insert task error: " + insertErr.Error())
		}
	}

	if taskErr != nil {
		respondTaskError(c, taskErr)
	}
}

// respondTaskError 统一输出 Task 错误响应（含 429 限流提示改写）
func respondTaskError(c *gin.Context, taskErr *dto.TaskError) {
	if taskErr.StatusCode == http.StatusTooManyRequests {
		taskErr.Message = "当前分组上游负载已饱和，请稍后再试"
	}
	c.JSON(taskErr.StatusCode, taskErr)
}

func shouldRetryTaskRelay(c *gin.Context, channelId int, taskErr *dto.TaskError, retryTimes int) bool {
	if taskErr == nil {
		return false
	}
	if service.ShouldSkipRetryAfterChannelAffinityFailure(c) {
		return false
	}
	if retryTimes <= 0 {
		return false
	}
	if _, ok := c.Get("specific_channel_id"); ok {
		return false
	}
	if taskErr.StatusCode == http.StatusTooManyRequests {
		return true
	}
	if taskErr.StatusCode == 307 {
		return true
	}
	if taskErr.StatusCode/100 == 5 {
		// 超时不重试
		if operation_setting.IsAlwaysSkipRetryStatusCode(taskErr.StatusCode) {
			return false
		}
		return true
	}
	if taskErr.StatusCode == http.StatusBadRequest {
		return false
	}
	if taskErr.StatusCode == 408 {
		// azure处理超时不重试
		return false
	}
	if taskErr.LocalError {
		return false
	}
	if taskErr.StatusCode/100 == 2 {
		return false
	}
	return true
}

// truncateAndMaskContent 截断内容并脱敏（用于审计日志预览）
func truncateAndMaskContent(content string, maxLen int) string {
	runes := []rune(content)
	if len(runes) > maxLen {
		content = string(runes[:maxLen]) + "..."
	}
	// 简单脱敏：将连续数字替换为 ****
	result := strings.Builder{}
	digitCount := 0
	for _, r := range content {
		if r >= '0' && r <= '9' {
			digitCount++
		} else {
			if digitCount > 4 {
				result.WriteString("****")
			} else if digitCount > 0 {
				for i := 0; i < digitCount; i++ {
					result.WriteByte('*')
				}
			}
			digitCount = 0
			result.WriteRune(r)
		}
	}
	if digitCount > 4 {
		result.WriteString("****")
	} else if digitCount > 0 {
		for i := 0; i < digitCount; i++ {
			result.WriteByte('*')
		}
	}
	return result.String()
}

// computeOutputPIIAction 根据 setting.PIIConfig 和 per-type action 决定输出侧 PII 的 action。
// 独立函数，便于在 controller 端和 channel handler 端复用同一逻辑。
func computeOutputPIIAction(text string) (shouldBlock, shouldMask bool, maskedText string, findings []service.PIIFinding) {
	enabledTypes := setting.GetEnabledPIITypes()
	if len(enabledTypes) == 0 {
		return false, false, text, nil
	}
	findings = service.CheckPIIText(text, enabledTypes)
	if len(findings) == 0 {
		return false, false, text, nil
	}
	for _, f := range findings {
		action := setting.GetPIITypeAction(f.Type, "output")
		switch action {
		case "block":
			shouldBlock = true
		case "mask":
			shouldMask = true
		}
	}
	if shouldMask {
		maskedText = service.MaskPIIText(text, findings)
	}
	return
}

// applyPIIMaskToRequest 在请求体层对常见 DTO 类型做 PII 脱敏。
// 返回是否实际修改了请求文本。
func applyPIIMaskToRequest(request dto.Request, findings []service.PIIFinding) bool {
	if request == nil || len(findings) == 0 {
		return false
	}
	changed := false
	switch r := request.(type) {
	case *dto.GeneralOpenAIRequest:
		masked, _ := service.MaskOpenAIRequestMessages(r.Messages, findings)
		if masked {
			changed = true
		}
		if r.Prompt != nil {
			newPrompt, ok := service.MaskOpenAIRequestPrompt(r.Prompt, findings)
			if ok {
				r.Prompt = newPrompt
				changed = true
			}
		}
		if r.Input != nil {
			newInput, ok := service.MaskOpenAIRequestPrompt(r.Input, findings)
			if ok {
				r.Input = newInput
				changed = true
			}
		}
	case *dto.ClaudeRequest:
		// 转为 dto.Message 形式做脱敏
		msgs := make([]dto.Message, 0, len(r.Messages))
		for _, m := range r.Messages {
			msgs = append(msgs, dto.Message{
				Role:    m.Role,
				Content: m.Content,
			})
		}
		masked, _ := service.MaskOpenAIRequestMessages(msgs, findings)
		if masked {
			changed = true
			for i := range msgs {
				r.Messages[i].Content = msgs[i].Content
			}
		}
		// System 字段：string 或 []any
		if r.System != nil {
			newSystem, ok := service.MaskOpenAIRequestPrompt(r.System, findings)
			if ok {
				r.System = newSystem
				changed = true
			}
		}
	case *dto.OpenAIResponsesRequest:
		// Input 字段是 json.RawMessage，单独处理
		if len(r.Input) > 0 {
			var asAny any
			if err := common.UnmarshalJsonStr(string(r.Input), &asAny); err == nil && asAny != nil {
				newInput, ok := service.MaskOpenAIRequestPrompt(asAny, findings)
				if ok {
					if buf, err := common.Marshal(newInput); err == nil {
						r.Input = buf
						changed = true
					}
				}
			}
		}
		if len(r.Instructions) > 0 {
			var asStr string
			if err := common.UnmarshalJsonStr(string(r.Instructions), &asStr); err == nil && asStr != "" {
				masked := service.MaskPIIText(asStr, findings)
				if masked != asStr {
					if buf, err := common.Marshal(masked); err == nil {
						r.Instructions = buf
						changed = true
					}
				}
			}
		}
	}
	return changed
}
