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

	"ragent-go/internal/api/handler"
	"ragent-go/internal/api/middleware"
	"ragent-go/internal/config"
	"ragent-go/internal/database"
	"ragent-go/internal/model"
	"ragent-go/internal/pkg/ai"
	"ragent-go/internal/pkg/milvus"
	"ragent-go/internal/pkg/redis"
	"ragent-go/internal/repository"
	"ragent-go/internal/service"

	"ragent-go/docs"

	"github.com/gin-gonic/gin"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
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
	db           *gorm.DB
	redisClient  *redisv9.Client
	milvusClient client.Client
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
	db, redisClient, milvusClient, initErr = initDatabases(cfg)
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
	setupRoutes(r, cfg)

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
func initDatabases(cfg *config.Config) (*gorm.DB, *redisv9.Client, client.Client, error) {
	// 初始化 MySQL
	mysqlDB, err := database.InitMySQL(cfg)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("MySQL init failed: %w", err)
	}

	// 初始化 Redis
	rdb, err := database.InitRedis(cfg)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("Redis init failed: %w", err)
	}

	// 初始化 Milvus（如果连接失败，使用 Mock）
	var milvusCli client.Client
	milvusCli, err = database.InitMilvus(cfg)
	if err != nil {
		log.Printf("⚠️  Milvus init failed: %v, using MockCollectionManager", err)
		milvusCli = nil // nil 表示使用 Mock
	}

	return mysqlDB, rdb, milvusCli, nil
}

// closeDatabases 关闭所有数据库连接
func closeDatabases() {
	if err := database.CloseMySQL(db); err != nil {
		log.Printf("Error closing MySQL: %v", err)
	}
	if err := database.CloseRedis(redisClient); err != nil {
		log.Printf("Error closing Redis: %v", err)
	}
	if err := database.CloseMilvus(milvusClient); err != nil {
		log.Printf("Error closing Milvus: %v", err)
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
func setupRoutes(r *gin.Engine, cfg *config.Config) {
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
		users.Use(middleware.OptionalAuth())
		{
			users.GET("/current", userHandler.GetCurrentUser)
			users.PUT("/password", userHandler.ChangePassword) // 修改密码
		}

		// 用户管理（仅管理员）
		userMgmt := v1.Group("/users")
		userMgmt.Use(middleware.OptionalAuth())
		userMgmt.Use(middleware.RequireRole(model.RoleAdmin))
		{
			userMgmt.GET("", userHandler.PageQuery)     // 分页查询
			userMgmt.POST("", userHandler.Create)       // 创建用户
			userMgmt.PUT("/:id", userHandler.Update)    // 更新用户
			userMgmt.DELETE("/:id", userHandler.Delete) // 删除用户
		}

		// 知识库相关（需要认证）
		knowledgeBase := v1.Group("/knowledge-base")
		// knowledgeBase.Use(middleware.OptionalAuth())
		{
			// 初始化知识库服务
			kbRepo := repository.NewKnowledgeBaseRepository(db)

			collectionMgr := milvus.NewRealCollectionManager(milvusClient)
			if collectionMgr == nil {
				log.Println("⚠️  Milvus client is not available")
				return
			}
			// 从配置获取默认向量维度，如果没有则使用 1024
			defaultDim := 1024
			if cfg.AI.Embedding.Dimension > 0 {
				defaultDim = cfg.AI.Embedding.Dimension
			}

			kbService := service.NewKnowledgeBaseService(kbRepo, collectionMgr, defaultDim)
			kbHandler := handler.NewKnowledgeBaseHandler(kbService)

			knowledgeBase.POST("", kbHandler.Create)       // 创建知识库
			knowledgeBase.GET("/:id", kbHandler.GetByID)   // 获取知识库详情
			knowledgeBase.GET("", kbHandler.PageQuery)     // 分页查询知识库列表
			knowledgeBase.PUT("/:id", kbHandler.Update)    // 更新知识库
			knowledgeBase.DELETE("/:id", kbHandler.Delete) // 删除知识库
		}

		// 初始化共享服务（文档和问答都需要）
		embeddingSvc := ai.NewEmbeddingService(cfg)
		vectorMgr := milvus.NewVectorManager(milvusClient)
		kbRepo := repository.NewKnowledgeBaseRepository(db)

		// 文档相关（需要认证）
		documents := v1.Group("/documents")
		// documents.Use(middleware.Auth())
		{
			// 初始化文档服务
			docRepo := repository.NewDocumentRepository(db)
			chunkRepo := repository.NewDocumentChunkRepository(db)

			// 分块服务配置
			chunkService := service.NewChunkService(1000, 200) // 1000字符分块，200字符重叠

			// 文件上传目录
			uploadBasePath := "uploads"
			if err := os.MkdirAll(uploadBasePath, 0755); err != nil {
				log.Printf("Warning: failed to create upload directory: %v", err)
			}

			docService := service.NewDocumentService(
				docRepo,
				chunkRepo,
				kbRepo,
				chunkService,
				embeddingSvc,
				vectorMgr,
				uploadBasePath,
			)
			docHandler := handler.NewDocumentHandler(docService)

			documents.POST("", docHandler.UploadDocument)                  // 上传文档
			documents.GET("/:id", docHandler.GetDocumentByID)              // 获取文档详情
			documents.GET("/:id/progress", docHandler.GetDocumentProgress) // 获取文档处理进度
			documents.GET("", docHandler.ListDocuments)                    // 分页查询文档列表
			documents.DELETE("/:id", docHandler.DeleteDocument)            // 删除文档
		}

		// RAG问答相关（需要认证）
		chat := v1.Group("/chat")
		// chat.Use(middleware.OptionalAuth())
		{
			// 初始化RAG问答服务
			retrievalSvc := service.NewRetrievalService(embeddingSvc, vectorMgr)
			llmSvc := ai.NewLLMService(cfg)
			chatService := service.NewChatService(retrievalSvc, llmSvc, kbRepo)
			chatHandler := handler.NewChatHandler(chatService)

			chat.POST("", chatHandler.Chat) // RAG智能问答
		}
	}
}
