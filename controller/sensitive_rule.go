package controller

import (
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
)

// 导入文件大小上限（10MB）
const sensitiveRuleImportMaxBytes int64 = 10 * 1024 * 1024

// 导入规则数量上限
const sensitiveRuleImportMaxRules = 50000

// GetSensitiveRules 获取所有敏感词规则
func GetSensitiveRules(c *gin.Context) {
	group := c.Query("group")
	rules := setting.GetSensitiveRulesByGroup(group)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    rules,
	})
}

// CreateSensitiveRule 创建敏感词规则
func CreateSensitiveRule(c *gin.Context) {
	var rule setting.SensitiveRuleEntry
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid request body: " + err.Error(),
		})
		return
	}

	if rule.Word == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "word is required",
		})
		return
	}

	// 验证 level
	if rule.Level != "block" && rule.Level != "warn" && rule.Level != "log" {
		rule.Level = "block"
	}

	if !setting.AppendSensitiveRule(rule) {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "rule already exists for this word and group",
		})
		return
	}

	// 保存到 options
	err := model.UpdateOption("SensitiveRules", setting.SensitiveRulesToOptionsJson())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "failed to save rule: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "rule created",
		"data":    rule,
	})
}

// UpdateSensitiveRule 更新敏感词规则
func UpdateSensitiveRule(c *gin.Context) {
	word := c.Query("word")
	group := c.Query("group")
	if word == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "word parameter is required",
		})
		return
	}

	var updateReq setting.SensitiveRuleEntry
	if err := c.ShouldBindJSON(&updateReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid request body: " + err.Error(),
		})
		return
	}

	// 验证 level
	if updateReq.Level != "block" && updateReq.Level != "warn" && updateReq.Level != "log" {
		updateReq.Level = "block"
	}

	if !setting.UpdateSensitiveRule(word, group, updateReq.Level, updateReq.Category) {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "rule not found",
		})
		return
	}

	// 保存到 options
	err := model.UpdateOption("SensitiveRules", setting.SensitiveRulesToOptionsJson())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "failed to save rule: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "rule updated",
	})
}

// DeleteSensitiveRule 删除敏感词规则
func DeleteSensitiveRule(c *gin.Context) {
	word := c.Query("word")
	group := c.Query("group")
	if word == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "word parameter is required",
		})
		return
	}

	if !setting.DeleteSensitiveRule(word, group) {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "rule not found",
		})
		return
	}

	// 保存到 options
	err := model.UpdateOption("SensitiveRules", setting.SensitiveRulesToOptionsJson())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "failed to save rule: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "rule deleted",
	})
}

// ExportSensitiveRules 导出敏感词规则为 CSV
func ExportSensitiveRules(c *gin.Context) {
	data, err := service.ExportRulesToCSV()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "export failed: " + err.Error(),
		})
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=sensitive_rules.csv")
	c.Data(http.StatusOK, "text/csv; charset=utf-8", data)
}

// ImportSensitiveRules 导入敏感词规则
func ImportSensitiveRules(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "file is required: " + err.Error(),
		})
		return
	}

	if file.Size > sensitiveRuleImportMaxBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"success": false,
			"message": "file too large: limit 10MB",
		})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "failed to open file: " + err.Error(),
		})
		return
	}
	defer f.Close()

	// 限制读取大小，防止恶意声明小 file.Size 但通过 multipart 注入
	data, err := io.ReadAll(io.LimitReader(f, sensitiveRuleImportMaxBytes+1))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "failed to read file: " + err.Error(),
		})
		return
	}
	if int64(len(data)) > sensitiveRuleImportMaxBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"success": false,
			"message": "file too large: limit 10MB",
		})
		return
	}

	group := c.PostForm("group")

	var imported int
	filename := file.Filename
	if len(filename) >= 4 && filename[len(filename)-4:] == ".csv" {
		imported, err = service.ImportRulesFromCSV(data, group, sensitiveRuleImportMaxRules)
	} else {
		imported, err = service.ImportRulesFromTXT(data, group, sensitiveRuleImportMaxRules)
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "import failed: " + err.Error(),
		})
		return
	}

	// 保存到 options
	saveErr := model.UpdateOption("SensitiveRules", setting.SensitiveRulesToOptionsJson())
	if saveErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "failed to save rules: " + saveErr.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "imported successfully",
		"data": gin.H{
			"imported": imported,
		},
	})
}
