package config

import (
	"fmt"
	"log"
	"strings"

	"github.com/spf13/viper"
)

// Config 应用配置
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Milvus   MilvusConfig   `mapstructure:"milvus"`
	AI       AIConfig       `mapstructure:"ai"`
	Log      LogConfig      `mapstructure:"log"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"` // debug, release, test
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	MySQL MySQLConfig `mapstructure:"mysql"`
}

// MySQLConfig MySQL 配置
type MySQLConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	User            string `mapstructure:"user"`
	Password        string `mapstructure:"password"`
	DBName          string `mapstructure:"dbname"`
	Charset         string `mapstructure:"charset"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"` // 秒
}

// DSN 返回 MySQL DSN
func (m MySQLConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		m.User, m.Password, m.Host, m.Port, m.DBName, m.Charset)
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

// Addr 返回 Redis 地址
func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// MilvusConfig Milvus 配置
type MilvusConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// Addr 返回 Milvus 地址
func (m MilvusConfig) Addr() string {
	return fmt.Sprintf("%s:%d", m.Host, m.Port)
}

// AIConfig AI 配置
type AIConfig struct {
	DeepSeek  DeepSeekConfig  `mapstructure:"deepseek"`
	Dashscope DashscopeConfig `mapstructure:"dashscope"`
	Embedding EmbeddingConfig `mapstructure:"embedding"`
}

// OpenAIConfig OpenAI 配置
type DeepSeekConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
	Model   string `mapstructure:"model"`
}

// DashscopeConfig 阿里云百炼配置
type DashscopeConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
	Model   string `mapstructure:"model"`
}

// EmbeddingConfig Embedding 配置
type EmbeddingConfig struct {
	Model     string `mapstructure:"model"`
	Dimension int    `mapstructure:"dimension"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level    string `mapstructure:"level"`  // debug, info, warn, error
	Format   string `mapstructure:"format"` // json, text
	Output   string `mapstructure:"output"` // stdout, file
	FilePath string `mapstructure:"file_path"`
}

var globalConfig *Config

// Load 加载配置
func Load(configPath string) (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configPath)
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	// 设置默认值
	setDefaults()

	// 支持环境变量（必须在 ReadInConfig 之前设置才能覆盖配置）
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// 读取配置文件（环境变量会覆盖配置文件中的值）
	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Warning: Failed to read config file: %v, using defaults", err)
	}

	// 显式绑定环境变量（确保环境变量优先级最高）
	viper.BindEnv("database.mysql.host", "DATABASE_MYSQL_HOST")
	viper.BindEnv("redis.host", "REDIS_HOST")

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 调试：打印实际使用的配置值
	log.Printf("Database MySQL Host: %s", config.Database.MySQL.Host)
	log.Printf("Redis Host: %s", config.Redis.Host)

	globalConfig = &config
	return &config, nil
}

// Get 获取全局配置
func Get() *Config {
	return globalConfig
}

// setDefaults 设置默认值
func setDefaults() {
	// Server
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.mode", "debug")

	// Database
	viper.SetDefault("database.mysql.host", "localhost")
	viper.SetDefault("database.mysql.port", 3306)
	viper.SetDefault("database.mysql.user", "root")
	viper.SetDefault("database.mysql.password", "root")
	viper.SetDefault("database.mysql.dbname", "ragent")
	viper.SetDefault("database.mysql.charset", "utf8mb4")
	viper.SetDefault("database.mysql.max_open_conns", 100)
	viper.SetDefault("database.mysql.max_idle_conns", 10)
	viper.SetDefault("database.mysql.conn_max_lifetime", 3600)

	// Redis
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 10)

	// Milvus
	viper.SetDefault("milvus.host", "localhost")
	viper.SetDefault("milvus.port", 19530)
	viper.SetDefault("milvus.username", "")
	viper.SetDefault("milvus.password", "")

	// Log
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "json")
	viper.SetDefault("log.output", "stdout")
	viper.SetDefault("log.file_path", "logs/app.log")
}
