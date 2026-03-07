package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ragent-go/docs"
	"ragent-go/internal/api/handler"
	"ragent-go/internal/api/middleware"
	"ragent-go/internal/config"
	"ragent-go/internal/database"
	"ragent-go/internal/model"
	"ragent-go/internal/pkg/redis"
	"ragent-go/internal/repository"
	"ragent-go/internal/service"

	"github.com/gin-gonic/gin"
	redisv9 "github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
)

// @title           Ragent Go API
// @version         1.0
// @description     Ragent Go 项目 API 文档
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.example.com/support
// @contact.email  support@example.com

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description 使用 Bearer Token 进行认证，格式：Bearer {token}

var (
	db          *gorm.DB
	redisClient *redisv9.Client
)

func main() {
	// 加载配置（支持环境变量指定配置路径）
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "./configs"
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化数据库连接
	var initErr error
	db, redisClient, initErr = initDatabases(cfg)
	if initErr != nil {
		log.Fatalf("Failed to initialize databases: %v", initErr)
	}
	defer closeDatabases()

	// 设置 Gin 模式
	gin.SetMode(cfg.Server.Mode)

	// 创建 Gin 引擎
	r := gin.New()

	// 添加中间件
	r.Use(gin.Logger())
	r.Use(middleware.Recovery())
	r.Use(middleware.CORS())

	// 注册路由
	setupRoutes(r)

	// 启动服务器
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	// 在 goroutine 中启动服务器
	go func() {
		fmt.Printf("🚀 Server starting on %s\n", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 等待中断信号以优雅地关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("🛑 Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	fmt.Println("✅ Server exited")
}

// initDatabases 初始化所有数据库连接
func initDatabases(cfg *config.Config) (*gorm.DB, *redisv9.Client, error) {
	// 初始化 MySQL
	mysqlDB, err := database.InitMySQL(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("MySQL init failed: %w", err)
	}

	// 初始化 Redis
	rdb, err := database.InitRedis(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("Redis init failed: %w", err)
	}

	// Milvus 稍后添加
	// if err := database.InitMilvus(cfg); err != nil {
	// 	return nil, nil, fmt.Errorf("Milvus init failed: %w", err)
	// }

	return mysqlDB, rdb, nil
}

// closeDatabases 关闭所有数据库连接
func closeDatabases() {
	if err := database.CloseMySQL(db); err != nil {
		log.Printf("Error closing MySQL: %v", err)
	}
	if err := database.CloseRedis(redisClient); err != nil {
		log.Printf("Error closing Redis: %v", err)
	}
}

// healthCheck 健康检查
// @Summary 健康检查
// @Description 检查服务是否正常运行
// @Tags 系统
// @Produce json
// @Success 200 {object} map[string]string "成功"
// @Router /health [get]
func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// setupRoutes 设置路由
func setupRoutes(r *gin.Engine) {
	// 初始化 Swagger 文档
	docs.SwaggerInfo.BasePath = "/api/v1"

	// Swagger 文档 - ginSwagger 会自动从 docs 包读取文档
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 健康检查
	r.GET("/health", healthCheck)

	// 初始化服务（使用全局 db 和 redisClient 变量）
	userRepo := repository.NewUserRepository(db)
	tokenBlacklist := redis.NewTokenBlacklist(redisClient)

	// 设置 Token 黑名单到中间件
	middleware.SetTokenBlacklist(tokenBlacklist)

	authService := service.NewAuthService(userRepo, tokenBlacklist)
	userService := service.NewUserService(userRepo)

	// 初始化处理器
	authHandler := handler.NewAuthHandler(authService)
	userHandler := handler.NewUserHandler(userService)

	// API v1 路由组
	v1 := r.Group("/api/v1")
	{
		// 认证相关（不需要认证）
		auth := v1.Group("/auth")
		{
			auth.POST("/login", authHandler.Login)
			auth.POST("/logout", middleware.Auth(), authHandler.Logout) // 登出需要认证
		}

		// 用户相关（需要认证）
		users := v1.Group("/user")
		users.Use(middleware.Auth())
		{
			users.GET("/current", userHandler.GetCurrentUser)
			users.PUT("/password", userHandler.ChangePassword) // 修改密码
		}

		// 用户管理（仅管理员）
		userMgmt := v1.Group("/users")
		userMgmt.Use(middleware.Auth())
		userMgmt.Use(middleware.RequireRole(model.RoleAdmin))
		{
			userMgmt.GET("", userHandler.PageQuery)     // 分页查询
			userMgmt.POST("", userHandler.Create)       // 创建用户
			userMgmt.PUT("/:id", userHandler.Update)    // 更新用户
			userMgmt.DELETE("/:id", userHandler.Delete) // 删除用户
		}
	}
}
