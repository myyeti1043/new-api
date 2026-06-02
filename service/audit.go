package service

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
)

var auditLogCh = make(chan *model.Log, 10000)
var auditLogDroppedCount atomic.Int64

func init() {
	go auditLogWriter()
}

// RecordAuditLog 异步写入审计日志，缓冲区满时丢弃（非阻塞）
func RecordAuditLog(log *model.Log) {
	if !setting.AuditLogEnabled {
		return
	}
	select {
	case auditLogCh <- log:
		// ok
	default:
		auditLogDroppedCount.Add(1)
		common.SysError("audit log buffer full, dropping log")
	}
}

func auditLogWriter() {
	batch := make([]*model.Log, 0, 100)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case log := <-auditLogCh:
			batch = append(batch, log)
			if len(batch) >= 100 {
				flushAuditLogs(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				flushAuditLogs(batch)
				batch = batch[:0]
			}
			// 定期输出丢弃计数，便于运维发现
			if dropped := auditLogDroppedCount.Swap(0); dropped > 0 {
				common.SysError(fmt.Sprintf("audit log dropped count: %d", dropped))
			}
		}
	}
}

func flushAuditLogs(logs []*model.Log) {
	if err := model.LOG_DB.CreateInBatches(logs, 100).Error; err != nil {
		common.SysError("failed to flush audit logs: " + err.Error())
	}
}
