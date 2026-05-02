package app

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ganiramadhan/starter-go/internal/config"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/ganiramadhan/starter-go/internal/middleware"
	"github.com/ganiramadhan/starter-go/internal/modules/auth"
	"github.com/ganiramadhan/starter-go/internal/modules/user"
	"github.com/ganiramadhan/starter-go/internal/platform/cache"
	"github.com/ganiramadhan/starter-go/internal/platform/database"
	"github.com/ganiramadhan/starter-go/internal/platform/storage"
	"github.com/ganiramadhan/starter-go/internal/routes"
	"github.com/ganiramadhan/starter-go/pkg/jwt"
	"github.com/ganiramadhan/starter-go/pkg/validator"

	_ "github.com/ganiramadhan/starter-go/docs"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/gofiber/swagger"
	"gorm.io/gorm"
)

type App struct {
	cfg       *config.Config
	fiber     *fiber.App
	db        *gorm.DB
	cache     cache.Cache
	storage   storage.Storage
	validator *validator.Validator
}

func New(cfg *config.Config) (*App, error) {
	a := &App{cfg: cfg, validator: validator.New()}

	if err := a.initDatabase(); err != nil {
		return nil, err
	}
	a.initCache()
	if err := a.initStorage(); err != nil {
		return nil, err
	}
	a.initHTTP()
	return a, nil
}

func (a *App) initDatabase() error {
	db, err := database.Connect(database.Config{
		Host:            a.cfg.Database.Host,
		Port:            a.cfg.Database.Port,
		User:            a.cfg.Database.User,
		Password:        a.cfg.Database.Password,
		Name:            a.cfg.Database.Name,
		SSLMode:         a.cfg.Database.SSLMode,
		MaxOpenConns:    a.cfg.Database.MaxOpenConns,
		MaxIdleConns:    a.cfg.Database.MaxIdleConns,
		ConnMaxLifetime: a.cfg.Database.ConnMaxLifetime,
	})
	if err != nil {
		return err
	}
	a.db = db
	log.Println("database: connected")

	if err := database.Migrate(db); err != nil {
		log.Printf("database: migration warning: %v", err)
	} else {
		log.Println("database: migrated")
	}
	return nil
}

func (a *App) initCache() {
	rc, err := cache.NewRedis(cache.RedisConfig{
		Host:     a.cfg.Redis.Host,
		Port:     a.cfg.Redis.Port,
		Password: a.cfg.Redis.Password,
		DB:       a.cfg.Redis.DB,
		PoolSize: a.cfg.Redis.PoolSize,
	})
	if err != nil {
		log.Printf("cache: %v - falling back to no-op cache", err)
		a.cache = cache.Noop{}
		return
	}
	a.cache = rc
	log.Println("cache: redis connected")
}

func (a *App) initStorage() error {
	st, err := storage.NewS3(storage.S3Config{
		AccessKey:    a.cfg.S3.AccessKey,
		SecretKey:    a.cfg.S3.SecretKey,
		Region:       a.cfg.S3.Region,
		Endpoint:     a.cfg.S3.Endpoint,
		Bucket:       a.cfg.S3.Bucket,
		UsePathStyle: a.cfg.S3.UsePathStyle,
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := st.EnsureBucket(ctx); err != nil {
		log.Printf("storage: ensure bucket warning: %v", err)
	}
	a.storage = st
	log.Printf("storage: s3 ready (bucket=%s)", a.cfg.S3.Bucket)
	return nil
}

func (a *App) initHTTP() {
	a.fiber = fiber.New(fiber.Config{
		AppName:      "Starter Go API",
		BodyLimit:    10 * 1024 * 1024,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
		ErrorHandler: middleware.ErrorHandler,
	})

	a.fiber.Use(requestid.New())
	a.fiber.Use(fiberlogger.New(fiberlogger.Config{
		Format:     "${time} | ${status} | ${latency} | ${ip} | ${method} ${path} | rid=${locals:requestid} | ${error}\n",
		TimeFormat: "2006-01-02 15:04:05",
	}))
	a.fiber.Use(recover.New())
	a.fiber.Use(cors.New(cors.Config{
		AllowOrigins: a.cfg.CORS.AllowOrigins,
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization,X-Request-ID",
	}))

	a.fiber.Get("/health", a.healthCheck)
	a.fiber.Get("/swagger/*", swagger.HandlerDefault)

	jwtMgr := jwt.New(a.cfg.JWT.Secret, a.cfg.JWT.TTL)

	// Repositories
	userRepo := user.NewRepository(a.db)

	// Services
	userSvc := user.NewService(userRepo, a.storage)
	authSvc := auth.NewService(userRepo, jwtMgr)

	routes.Register(a.fiber, routes.Handlers{
		Auth: auth.NewHandler(authSvc, a.validator),
		User: user.NewHandler(userSvc, a.storage, a.validator),
	}, jwtMgr)
}

// Run starts the HTTP server and blocks until SIGINT/SIGTERM triggers shutdown.
func (a *App) Run() error {
	defer a.close()

	go func() {
		addr := ":" + a.cfg.App.Port
		log.Printf("server: listening on %s", addr)
		if err := a.fiber.Listen(addr); err != nil {
			log.Printf("server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("server: shutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.fiber.ShutdownWithContext(ctx); err != nil {
		log.Printf("server: forced shutdown: %v", err)
		return err
	}
	log.Println("server: stopped")
	return nil
}

func (a *App) close() {
	if a.cache != nil {
		if err := a.cache.Close(); err != nil {
			log.Printf("cache: close: %v", err)
		}
	}
	if a.db != nil {
		if err := database.Close(a.db); err != nil {
			log.Printf("database: close: %v", err)
		}
	}
}

func (a *App) healthCheck(c *fiber.Ctx) error {
	dbStatus := "connected"
	if err := database.Ping(a.db); err != nil {
		dbStatus = "disconnected"
	}
	cacheStatus := "connected"
	if err := a.cache.Ping(c.Context()); err != nil {
		cacheStatus = "disconnected"
	}

	status := "healthy"
	code := fiber.StatusOK
	if dbStatus == "disconnected" {
		status = "degraded"
		code = fiber.StatusServiceUnavailable
	}
	return c.Status(code).JSON(dto.APIResponse{
		Status:  status,
		Code:    code,
		Message: "health",
		Data: fiber.Map{
			"services": fiber.Map{
				"database": dbStatus,
				"cache":    cacheStatus,
			},
		},
	})
}
