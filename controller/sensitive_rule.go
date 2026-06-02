package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
)

// GetSensitiveRules 获取所有敏感词规则
func GetSensitiveRules(c *gin.Context) {
	group := c.Query("group")
	rules := setting.SensitiveRules

	// 按 group 过滤
	if group != "" {
		var filtered []setting.SensitiveRuleEntry
		for _, r := range rules {
			if r.Group == "" || r.Group == group {
				filtered = append(filtered, r)
			}
		}
		rules = filtered
	}

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

	// 检查重复（word + group 唯一）
	for _, r := range setting.SensitiveRules {
		if r.Word == rule.Word && r.Group == rule.Group {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": "rule already exists for this word and group",
			})
			return
		}
	}

	setting.SensitiveRules = append(setting.SensitiveRules, rule)

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

	found := false
	for i, r := range setting.SensitiveRules {
		if r.Word == word && r.Group == group {
			setting.SensitiveRules[i].Level = updateReq.Level
			if updateReq.Category != "" {
				setting.SensitiveRules[i].Category = updateReq.Category
			}
			found = true
			break
		}
	}

	if !found {
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

	found := false
	var newRules []setting.SensitiveRuleEntry
	for _, r := range setting.SensitiveRules {
		if r.Word == word && r.Group == group {
			found = true
			continue
		}
		newRules = append(newRules, r)
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "rule not found",
		})
		return
	}

	setting.SensitiveRules = newRules

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

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "failed to open file: " + err.Error(),
		})
		return
	}
	defer f.Close()

	data := make([]byte, file.Size)
	_, err = f.Read(data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "failed to read file: " + err.Error(),
		})
		return
	}

	group := c.PostForm("group")

	var imported int
	filename := file.Filename
	if len(filename) >= 4 && filename[len(filename)-4:] == ".csv" {
		imported, err = service.ImportRulesFromCSV(data, group)
	} else {
		imported, err = service.ImportRulesFromTXT(data, group)
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
