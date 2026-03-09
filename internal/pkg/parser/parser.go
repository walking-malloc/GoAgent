package parser

import (
	"log"

	"ragent-go/internal/config"
	"ragent-go/internal/pkg/tika"
)

var globalTikaParser *TikaParser

// InitParser 初始化解析器（需要在应用启动时调用）
func InitParser(cfg *config.Config) {
	if cfg.Tika.Enabled {
		tikaClient := tika.NewClient(cfg.Tika.URL(), cfg.Tika.Timeout)
		globalTikaParser = NewTikaParser(tikaClient)
		log.Printf("✅ Tika Server 已启用: %s", cfg.Tika.URL())
	} else {
		globalTikaParser = nil
		log.Println("⚠️  Tika 未启用，使用默认解析器（部分格式可能不可用）")
	}
	log.Println("✅ 文档解析器已初始化")
}

// GetTikaParser 获取 Tika 解析器
func GetTikaParser() *TikaParser {
	return globalTikaParser
}
