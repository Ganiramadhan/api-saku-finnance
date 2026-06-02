package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ganiramadhan/starter-go/internal/config"
	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/ganiramadhan/starter-go/internal/middleware"
	aimodule "github.com/ganiramadhan/starter-go/internal/modules/ai"
	"github.com/ganiramadhan/starter-go/internal/modules/ailog"
	"github.com/ganiramadhan/starter-go/internal/modules/auth"
	"github.com/ganiramadhan/starter-go/internal/modules/budget"
	"github.com/ganiramadhan/starter-go/internal/modules/category"
	"github.com/ganiramadhan/starter-go/internal/modules/notification"
	"github.com/ganiramadhan/starter-go/internal/modules/savingsgoal"
	"github.com/ganiramadhan/starter-go/internal/modules/splitbill"
	"github.com/ganiramadhan/starter-go/internal/modules/subscription"
	"github.com/ganiramadhan/starter-go/internal/modules/support"
	"github.com/ganiramadhan/starter-go/internal/modules/transaction"
	"github.com/ganiramadhan/starter-go/internal/modules/upcomingbilling"
	"github.com/ganiramadhan/starter-go/internal/modules/user"
	"github.com/ganiramadhan/starter-go/internal/modules/wallet"
	aiplatform "github.com/ganiramadhan/starter-go/internal/platform/ai"
	"github.com/ganiramadhan/starter-go/internal/platform/cache"
	"github.com/ganiramadhan/starter-go/internal/platform/database"
	"github.com/ganiramadhan/starter-go/internal/platform/mailer"
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
	subSvc    subscription.Service
	stopJobs  context.CancelFunc
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

	// Seed system categories
	if err := database.SeedSystemCategories(db); err != nil {
		log.Printf("database: seed warning: %v", err)
	}

	// Seed subscription plans
	if err := database.SeedPlans(db); err != nil {
		log.Printf("database: seed plans warning: %v", err)
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
	a.fiber.Use(middleware.SecurityHeaders())
	a.fiber.Use(cors.New(cors.Config{
		AllowOrigins:     a.cfg.CORS.AllowOrigins,
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Request-ID",
		AllowCredentials: true,
	}))

	a.fiber.Get("/health", a.healthCheck)
	a.fiber.Get("/swagger/*", swagger.HandlerDefault)

	jwtMgr := jwt.New(a.cfg.JWT.Secret, a.cfg.JWT.TTL)

	// Repositories
	userRepo := user.NewRepository(a.db)
	walletRepo := wallet.NewRepository(a.db)
	categoryRepo := category.NewRepository(a.db)
	txnRepo := transaction.NewRepository(a.db)
	aiLogRepo := ailog.NewRepository(a.db)
	budgetRepo := budget.NewRepository(a.db)
	goalRepo := savingsgoal.NewRepository(a.db)
	splitRepo := splitbill.NewRepository(a.db)
	subRepo := subscription.NewRepository(a.db)
	billingRepo := upcomingbilling.NewRepository(a.db)
	notificationRepo := notification.NewRepository(a.db)
	supportRepo := support.NewRepository(a.db)
	mailClient := mailer.New(mailer.Config{
		Mailer:     a.cfg.Mail.Mailer,
		Host:       a.cfg.Mail.Host,
		Port:       a.cfg.Mail.Port,
		Username:   a.cfg.Mail.Username,
		Password:   a.cfg.Mail.Password,
		Encryption: a.cfg.Mail.Encryption,
		FromEmail:  a.cfg.Mail.FromEmail,
		FromName:   a.cfg.Mail.FromName,
	})
	mailClient = mailer.NewAsync(mailClient, 200)

	// Services
	userSvc := user.NewService(userRepo, a.storage)
	authSvc := auth.NewService(userRepo, jwtMgr, a.cfg.Google.ClientID, mailClient)
	midtransClient := subscription.NewMidtransClient(a.cfg.Midtrans.ServerKey, a.cfg.Midtrans.IsProduction)
	subSvc := subscription.NewService(subRepo, userRepo, midtransClient, mailClient, a.cfg.Midtrans.ClientKey, a.cfg.Midtrans.IsProduction)
	a.subSvc = subSvc
	walletSvc := wallet.NewService(walletRepo, subSvc)
	categorySvc := category.NewService(categoryRepo)
	txnSvc := transaction.NewService(txnRepo, walletRepo, categoryRepo)
	txnExportSvc := transaction.NewExportService(txnRepo, walletRepo, categoryRepo)
	aiLogSvc := ailog.NewService(aiLogRepo, a.storage)
	budgetSvc := budget.NewService(budgetRepo, walletRepo, categoryRepo)
	goalSvc := savingsgoal.NewService(goalRepo)
	splitSvc := splitbill.NewService(splitRepo)
	billingSvc := upcomingbilling.NewService(billingRepo, subSvc)
	notificationSvc := notification.NewService(notificationRepo)
	supportSvc := support.NewService(supportRepo, a.storage)

	// AI (Claude)
	claudeClient := aiplatform.NewClient(a.cfg.Claude.APIKey, a.cfg.Claude.Model)
	aiSvc := aimodule.NewService(claudeClient, txnRepo, walletRepo, categoryRepo, aiLogSvc, a.storage, a.cfg.Claude.Model, subSvc)

	txnHandler := transaction.NewHandler(txnSvc, a.validator)
	txnHandler.SetExportService(txnExportSvc)

	handlers := routes.Handlers{
		Auth:         auth.NewHandler(authSvc, a.validator, a.cfg.Turnstile.SecretKey),
		User:         user.NewHandler(userSvc, a.storage, a.validator),
		Wallet:       wallet.NewHandler(walletSvc, a.validator),
		Category:     category.NewHandler(categorySvc, a.validator),
		Transaction:  txnHandler,
		AILog:        ailog.NewHandler(aiLogSvc, a.validator),
		Budget:       budget.NewHandler(budgetSvc, a.validator),
		SavingsGoal:  savingsgoal.NewHandler(goalSvc, a.validator),
		Subscription: subscription.NewHandler(subSvc, a.validator),
		SplitBill:    splitbill.NewHandler(splitSvc, a.validator, subSvc),
		Billing:      upcomingbilling.NewHandler(billingSvc, a.validator),
		AI:           aimodule.NewHandler(aiSvc, a.validator),
		Notification: notification.NewHandler(notificationSvc, a.validator),
	}.WithSupport(support.NewHandler(supportSvc, a.storage, a.validator))
	routes.Register(a.fiber, handlers, jwtMgr)

	a.startReminderJob(billingRepo, subRepo, notificationSvc, mailClient)
}

func (a *App) startReminderJob(
	billingRepo upcomingbilling.Repository,
	subRepo subscription.Repository,
	notificationSvc notification.Service,
	mailClient mailer.Mailer,
) {
	ctx, cancel := context.WithCancel(context.Background())
	a.stopJobs = cancel
	go func() {
		a.expirePendingSubscriptions(subRepo)
		a.cleanupTempStorage()
		a.runReminderPass(ctx, billingRepo, subRepo, notificationSvc, mailClient)
		reminderTicker := time.NewTicker(24 * time.Hour)
		expiryTicker := time.NewTicker(5 * time.Minute)
		storageTicker := time.NewTicker(1 * time.Hour)
		defer reminderTicker.Stop()
		defer expiryTicker.Stop()
		defer storageTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-expiryTicker.C:
				a.expirePendingSubscriptions(subRepo)
			case <-storageTicker.C:
				a.cleanupTempStorage()
			case <-reminderTicker.C:
				a.expirePendingSubscriptions(subRepo)
				a.runReminderPass(ctx, billingRepo, subRepo, notificationSvc, mailClient)
			}
		}
	}()
}

func (a *App) expirePendingSubscriptions(subRepo subscription.Repository) {
	if err := subRepo.ExpirePendingBefore(time.Now().UTC()); err != nil {
		log.Printf("subscription cleanup: expire pending subscriptions: %v", err)
	}
}

func (a *App) cleanupTempStorage() {
	if a.storage == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	deleted, err := a.storage.DeletePrefixOlderThan(ctx, "Temp/", 24*time.Hour)
	if err != nil {
		log.Printf("storage cleanup: delete temp objects: %v", err)
		return
	}
	if deleted > 0 {
		log.Printf("storage cleanup: deleted %d temp objects", deleted)
	}
}

func (a *App) runReminderPass(
	ctx context.Context,
	billingRepo upcomingbilling.Repository,
	subRepo subscription.Repository,
	notificationSvc notification.Service,
	mailClient mailer.Mailer,
) {
	today := time.Now().UTC()
	if billings, err := billingRepo.ListActiveWithUsers(); err != nil {
		log.Printf("reminder: list upcoming billings: %v", err)
	} else {
		for _, item := range billings {
			if item.User == nil || daysUntilUTC(today, item.DueDate) != 7 {
				continue
			}
			title := "Bill reminder in 7 days"
			message := fmt.Sprintf("%s is due on %s for %s.", item.Name, item.DueDate.Format("02 Jan 2006"), formatMoney(item.Amount, item.Currency))
			_ = notificationSvc.Create(ctx, domain.Notification{
				UserID: item.UserID, Type: "upcoming_billing_reminder", Title: title, Message: message, RefType: "upcoming_billing", RefID: item.ID.String(),
			})
			_ = mailClient.Send(item.User.Email, title, message+"\n\nOpen SAKU to review cashflow and prepare the payment.")
		}
	}

	subs, err := subRepo.ListActiveForReminder()
	if err != nil {
		log.Printf("reminder: list subscriptions: %v", err)
		return
	}
	for _, sub := range subs {
		if sub.User == nil || sub.Plan == nil {
			continue
		}
		due := sub.NextBillingAt
		if due == nil {
			due = sub.EndsAt
		}
		if due == nil {
			continue
		}
		window := 7
		if sub.Plan.Period == domain.PlanPeriodYearly {
			window = 30
		}
		if daysUntilUTC(today, *due) != window {
			continue
		}
		title := fmt.Sprintf("Subscription reminder in %d days", window)
		message := fmt.Sprintf("Your %s subscription renews on %s for %s.", sub.Plan.Name, due.Format("02 Jan 2006"), formatMoney(sub.Amount, sub.Currency))
		_ = notificationSvc.Create(ctx, domain.Notification{
			UserID: sub.UserID, Type: "subscription_reminder", Title: title, Message: message, RefType: "subscription", RefID: sub.ID.String(),
		})
		_ = mailClient.Send(sub.User.Email, title, message+"\n\nOpen SAKU to manage your subscription.")
	}
}

func formatMoney(amount float64, currency string) string {
	if strings.EqualFold(currency, "IDR") || currency == "" {
		return "Rp " + formatThousandsID(amount)
	}
	return fmt.Sprintf("%.0f %s", amount, currency)
}

func formatThousandsID(amount float64) string {
	raw := fmt.Sprintf("%.0f", amount)
	var b strings.Builder
	for i, r := range raw {
		if i > 0 && (len(raw)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func daysUntilUTC(now, target time.Time) int {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	due := target.UTC()
	end := time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, time.UTC)
	return int(end.Sub(start).Hours() / 24)
}

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
	if a.stopJobs != nil {
		a.stopJobs()
	}
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
