package setting

import (
	"encoding/json"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

// PIITypeConfig 单个 PII 类型的配置
type PIITypeConfig struct {
	Enabled bool   `json:"enabled"`
	Action  string `json:"action"` // mask / block / log
}

// PIICOnfig PII 检测全局配置
type PIICOnfig struct {
	Enabled           bool                    `json:"enabled"`
	InputAction       string                  `json:"input_action"`       // 输入端默认动作：mask / block / log
	OutputAction      string                  `json:"output_action"`      // 输出端默认动作：mask / block / log
	AuditLogEnabled   bool                    `json:"audit_log_enabled"`  // 是否记录审计日志
	Types             map[string]PIITypeConfig `json:"types"`             // 各类型独立配置
}

// piiMu 保护 PIIConfig 的并发读写。
// 设置变更与运行期读取都会走这一把锁。
var piiMu sync.RWMutex

// DefaultPIIConfig 默认 PII 配置
var PIIConfig = PIICOnfig{
	Enabled:         false,
	InputAction:     "mask",
	OutputAction:    "mask",
	AuditLogEnabled: true,
	Types: map[string]PIITypeConfig{
		"phone":    {Enabled: true, Action: "mask"},
		"idcard":   {Enabled: true, Action: "mask"},
		"bankcard": {Enabled: true, Action: "mask"},
		"email":    {Enabled: false, Action: "log"},
		"ipv4":     {Enabled: false, Action: "log"},
	},
}

// SnapshotPIIConfig 返回 PIIConfig 快照（线程安全）
func SnapshotPIIConfig() PIICOnfig {
	piiMu.RLock()
	defer piiMu.RUnlock()
	typesCopy := make(map[string]PIITypeConfig, len(PIIConfig.Types))
	for k, v := range PIIConfig.Types {
		typesCopy[k] = v
	}
	return PIICOnfig{
		Enabled:         PIIConfig.Enabled,
		InputAction:     PIIConfig.InputAction,
		OutputAction:    PIIConfig.OutputAction,
		AuditLogEnabled: PIIConfig.AuditLogEnabled,
		Types:           typesCopy,
	}
}

// PIICOnfigToOptionsJson 将 PIICOnfig 序列化为 JSON 字符串
func PIICOnfigToOptionsJson() string {
	snap := SnapshotPIIConfig()
	b, err := common.Marshal(snap)
	if err != nil {
		common.SysError("PIICOnfigToOptionsJson marshal error: " + err.Error())
		return "{}"
	}
	return string(b)
}

// PIICOnfigFromOptionsJson 从 JSON 字符串反序列化 PIICOnfig
func PIICOnfigFromOptionsJson(s string) {
	if s == "" {
		return
	}
	var config PIICOnfig
	if err := json.Unmarshal([]byte(s), &config); err != nil {
		common.SysError("PIICOnfigFromOptionsJson unmarshal error: " + err.Error())
		return
	}
	piiMu.Lock()
	PIIConfig = config
	piiMu.Unlock()
}

// ShouldCheckPIIOnInput 是否应该在输入端检查 PII
func ShouldCheckPIIOnInput() bool {
	piiMu.RLock()
	defer piiMu.RUnlock()
	return PIIConfig.Enabled && PIIConfig.InputAction != "log"
}

// ShouldCheckPIIOnOutput 是否应该在输出端检查 PII
func ShouldCheckPIIOnOutput() bool {
	piiMu.RLock()
	defer piiMu.RUnlock()
	return PIIConfig.Enabled && PIIConfig.OutputAction != "log"
}

// GetEnabledPIITypes 获取启用的 PII 类型列表
func GetEnabledPIITypes() []string {
	piiMu.RLock()
	defer piiMu.RUnlock()
	var types []string
	for piiType, config := range PIIConfig.Types {
		if config.Enabled {
			types = append(types, piiType)
		}
	}
	return types
}

// GetPIITypeAction 获取指定 PII 类型的动作（如果未配置则返回默认动作）
func GetPIITypeAction(piiType string, direction string) string {
	piiMu.RLock()
	defer piiMu.RUnlock()
	if config, ok := PIIConfig.Types[piiType]; ok {
		if config.Action != "" {
			return config.Action
		}
	}
	// 返回默认动作
	if direction == "input" {
		return PIIConfig.InputAction
	}
	return PIIConfig.OutputAction
}
