package service

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

const (
	supportAttachmentCleanupInitialDelay = time.Minute
	supportAttachmentCleanupInterval     = 24 * time.Hour
	supportAttachmentCleanupBatchSize    = 100
)

var supportAttachmentCleanupOnce sync.Once

// StartSupportAttachmentCleanupTask 启动已关闭客服会话留存清理任务。
// 启动后延迟 1 分钟执行一次，之后每 24 小时执行一次。
func StartSupportAttachmentCleanupTask() {
	supportAttachmentCleanupOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		go func() {
			timer := time.NewTimer(supportAttachmentCleanupInitialDelay)
			defer timer.Stop()
			<-timer.C
			runSupportAttachmentCleanup()

			ticker := time.NewTicker(supportAttachmentCleanupInterval)
			defer ticker.Stop()
			for range ticker.C {
				runSupportAttachmentCleanup()
			}
		}()
	})
}

func runSupportAttachmentCleanup() {
	if err := CleanupSupportAttachments(); err != nil {
		common.SysError("support attachment cleanup failed: " + err.Error())
	}
}

// CleanupSupportAttachments 删除超过留存期的已关闭会话及其附件文件。
// 文件目录删除失败时保留数据库记录，等待下一轮重试。
func CleanupSupportAttachments() error {
	days := system_setting.GetSupportChatSettings().RetentionDays
	if days <= 0 {
		return nil
	}

	cutoff := time.Now().Unix() - int64(days)*86400
	var errs []error
	removedConversations := 0
	for {
		conversations, err := model.ListClosedSupportConversationsBefore(cutoff, supportAttachmentCleanupBatchSize)
		if err != nil {
			common.SysError("failed to list closed support conversations for cleanup: " + err.Error())
			errs = append(errs, err)
			break
		}
		if len(conversations) == 0 {
			break
		}

		deletedInBatch := 0
		for _, conversation := range conversations {
			if err := RemoveSupportConversationDir(conversation.Id); err != nil {
				common.SysError(fmt.Sprintf(
					"failed to remove support attachment directory for conversation %d: %v",
					conversation.Id, err,
				))
				errs = append(errs, err)
				continue
			}
			if err := model.DeleteSupportConversationData(conversation.Id); err != nil {
				common.SysError(fmt.Sprintf(
					"failed to delete support conversation data for conversation %d: %v",
					conversation.Id, err,
				))
				errs = append(errs, err)
				continue
			}
			deletedInBatch++
			removedConversations++
		}

		// 没有进展时退出，避免文件删除持续失败导致死循环。
		if len(conversations) < supportAttachmentCleanupBatchSize || deletedInBatch == 0 {
			break
		}
	}

	if err := CleanupOrphanSupportAttachmentFiles(); err != nil {
		common.SysError("failed to clean orphan support attachment files: " + err.Error())
		errs = append(errs, err)
	}
	common.SysLog(fmt.Sprintf("support attachment cleanup removed %d closed conversations", removedConversations))
	return errors.Join(errs...)
}
