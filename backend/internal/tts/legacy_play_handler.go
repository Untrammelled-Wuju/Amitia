package tts

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/requestidentity"
	"gorm.io/gorm"
)

func HandlePlayMessage(c *gin.Context, db interface{}) {
	msgID := c.Param("messageId")
	if msgID == "" {
		c.JSON(400, gin.H{"error": "missing messageId"})
		return
	}
	type Msg struct {
		Content string
		MsgType string
	}
	var msg Msg
	gdb := db.(*gorm.DB)
	owner := requestidentity.NormalizeUserID(requestidentity.ResolveGin(c, ""))
	query := gdb.Table("messages AS m").Select("m.content, m.msg_type").Joins("JOIN conversations AS conv ON conv.id = m.conversation_id").Where("m.id = ? AND conv.deleted_at IS NULL", msgID)
	if config.AppCfg != nil && strings.EqualFold(strings.TrimSpace(config.AppCfg.Security.Mode), "local_single_user") {
		query = query.Where("(conv.user_id = ? OR conv.user_id = '' OR conv.user_id IS NULL OR conv.user_id = ?)", owner, requestidentity.DefaultUserID)
	} else {
		query = query.Where("conv.user_id = ?", owner)
	}
	if err := query.Row().Scan(&msg.Content, &msg.MsgType); err != nil {
		c.JSON(404, gin.H{"error": "message not found"})
		return
	}
	if msg.Content == "" {
		c.JSON(404, gin.H{"error": "empty message"})
		return
	}
	repo := NewRepository(gdb)
	svc := NewService(repo)
	result, err := svc.SynthesizeWithActive(msg.Content)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	audioPath := "data/tts_cache/" + strings.TrimPrefix(result.AudioURL, "/audio/")
	c.File(audioPath)
}

func GetActiveConfig(db *gorm.DB) (*TtsConfig, error) {
	var cfg TtsConfig
	if err := db.Table("tts_configs").Where("is_active = 1").Limit(1).First(&cfg).Error; err != nil {
		return nil, err
	}
	return &cfg, nil
}
