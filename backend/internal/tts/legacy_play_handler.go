package tts

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"strings"
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
	if err := gdb.Table("messages").Select("content, msg_type").Where("id = ?", msgID).Row().Scan(&msg.Content, &msg.MsgType); err != nil {
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
