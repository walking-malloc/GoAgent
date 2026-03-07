package database

import (
	"fmt"
	"log"
	"time"

	"ragent-go/internal/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// InitMySQL 初始化 MySQL 连接
func InitMySQL(cfg *config.Config) (*gorm.DB, error) {
	mysqlCfg := cfg.Database.MySQL

	// 配置 GORM
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	// 连接数据库
	db, err := gorm.Open(mysql.Open(mysqlCfg.DSN()), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MySQL: %w", err)
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(mysqlCfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(mysqlCfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(mysqlCfg.ConnMaxLifetime) * time.Second)

	// 测试连接
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping MySQL: %w", err)
	}

	// 显式设置连接字符集为 utf8mb4（确保客户端字符集正确）
	// 设置多个字符集相关参数，确保完全兼容
	charsetSQLs := []string{
		"SET NAMES utf8mb4 COLLATE utf8mb4_unicode_ci",
		"SET CHARACTER SET utf8mb4",
		"SET character_set_client = utf8mb4",
		"SET character_set_connection = utf8mb4",
		"SET character_set_results = utf8mb4",
		"SET collation_connection = utf8mb4_unicode_ci",
	}
	
	for _, sql := range charsetSQLs {
		if err := db.Exec(sql).Error; err != nil {
			log.Printf("Warning: failed to execute '%s': %v", sql, err)
		}
	}

	log.Println("✅ MySQL connected successfully with utf8mb4 charset")
	return db, nil
}

// CloseMySQL 关闭 MySQL 连接
func CloseMySQL(db *gorm.DB) error {
	if db != nil {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}
