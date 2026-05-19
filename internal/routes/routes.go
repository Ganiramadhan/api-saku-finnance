package routes

import (
	"github.com/ganiramadhan/starter-go/internal/middleware"
	"github.com/ganiramadhan/starter-go/internal/modules/ai"
	"github.com/ganiramadhan/starter-go/internal/modules/ailog"
	"github.com/ganiramadhan/starter-go/internal/modules/auth"
	"github.com/ganiramadhan/starter-go/internal/modules/budget"
	"github.com/ganiramadhan/starter-go/internal/modules/category"
	"github.com/ganiramadhan/starter-go/internal/modules/savingsgoal"
	"github.com/ganiramadhan/starter-go/internal/modules/splitbill"
	"github.com/ganiramadhan/starter-go/internal/modules/subscription"
	"github.com/ganiramadhan/starter-go/internal/modules/transaction"
	"github.com/ganiramadhan/starter-go/internal/modules/user"
	"github.com/ganiramadhan/starter-go/internal/modules/wallet"
	"github.com/ganiramadhan/starter-go/pkg/jwt"
	"github.com/gofiber/fiber/v2"
)

type Handlers struct {
	Auth         *auth.Handler
	User         *user.Handler
	Wallet       *wallet.Handler
	Category     *category.Handler
	Transaction  *transaction.Handler
	AILog        *ailog.Handler
	Budget       *budget.Handler
	SavingsGoal  *savingsgoal.Handler
	Subscription *subscription.Handler
	SplitBill    *splitbill.Handler
	AI           *ai.Handler
}

func Register(app *fiber.App, h Handlers, jwtMgr *jwt.Manager) {
	authRequired := middleware.AuthRequired(jwtMgr)

	v1 := app.Group("/api/v1")

	// ---------------- Public ----------------
	authPub := v1.Group("/auth")
	authPub.Post("/login", h.Auth.Login)
	authPub.Post("/register", h.Auth.Register)
	authPub.Post("/google", h.Auth.GoogleLogin)

	// Public subscription catalog + Midtrans webhook
	subsPub := v1.Group("/subscriptions")
	subsPub.Get("/plans", h.Subscription.ListPlans)
	subsPub.Post("/webhook", h.Subscription.Webhook)

	// ---------------- User (authenticated) ----------------
	authPriv := v1.Group("/auth", authRequired)
	authPriv.Post("/change-password", h.Auth.ChangePassword)

	usersMe := v1.Group("/users", authRequired)
	usersMe.Get("/me", h.User.Me)
	usersMe.Put("/me", h.User.UpdateMe)
	usersMe.Post("/upload-photo", h.User.UploadPhoto)
	usersMe.Delete("/me/photo", h.User.DeleteMyPhoto)

	// ----- SAKU finance modules (tenant-scoped) -----
	wallets := v1.Group("/wallets", authRequired)
	wallets.Get("", h.Wallet.List)
	wallets.Post("", h.Wallet.Create)
	wallets.Get("/:id", h.Wallet.Get)
	wallets.Put("/:id", h.Wallet.Update)
	wallets.Delete("/:id", h.Wallet.Delete)

	cats := v1.Group("/categories", authRequired)
	cats.Get("", h.Category.List)
	cats.Post("", h.Category.Create)
	cats.Get("/:id", h.Category.Get)
	cats.Put("/:id", h.Category.Update)
	cats.Delete("/:id", h.Category.Delete)

	txns := v1.Group("/transactions", authRequired)
	txns.Get("", h.Transaction.List)
	txns.Get("/export", h.Transaction.Export)
	txns.Post("", h.Transaction.Create)
	txns.Get("/:id", h.Transaction.Get)
	txns.Put("/:id", h.Transaction.Update)
	txns.Delete("/:id", h.Transaction.Delete)

	aiLogs := v1.Group("/ai-logs", authRequired)
	aiLogs.Get("", h.AILog.List)

	budgets := v1.Group("/budgets", authRequired)
	budgets.Get("", h.Budget.List)
	budgets.Post("", h.Budget.Create)
	budgets.Get("/:id", h.Budget.Get)
	budgets.Put("/:id", h.Budget.Update)
	budgets.Delete("/:id", h.Budget.Delete)

	goals := v1.Group("/savings-goals", authRequired)
	goals.Get("", h.SavingsGoal.List)
	goals.Post("", h.SavingsGoal.Create)
	goals.Get("/:id", h.SavingsGoal.Get)
	goals.Put("/:id", h.SavingsGoal.Update)
	goals.Delete("/:id", h.SavingsGoal.Delete)
	goals.Post("/:id/contribute", h.SavingsGoal.Contribute)
	goals.Get("/:id/contributions", h.SavingsGoal.ListContributions)

	// Authed subscription endpoints
	subs := v1.Group("/subscriptions", authRequired)
	subs.Get("/me", h.Subscription.MySubscriptions)
	subs.Get("/me/active", h.Subscription.ActiveSubscription)
	subs.Post("/checkout", h.Subscription.Checkout)

	// AI categorization
	aiGroup := v1.Group("/ai", authRequired)
	aiGroup.Post("/categorize", h.AI.Categorize)
	aiGroup.Post("/scan-receipt", h.AI.ScanReceipt)
	aiGroup.Post("/insights", h.AI.Insights)
	aiGroup.Post("/suggest-budget", h.AI.SuggestBudget)
	aiGroup.Post("/chat", h.AI.Chat)

	// Split Bill — owner-scoped CRUD + WhatsApp share.
	split := v1.Group("/split-bills", authRequired)
	split.Get("/", h.SplitBill.List)
	split.Post("/", h.SplitBill.Create)
	split.Get("/:id", h.SplitBill.Get)
	split.Put("/:id", h.SplitBill.Update)
	split.Delete("/:id", h.SplitBill.Delete)
	split.Patch("/:id/participants/:pid/paid", h.SplitBill.MarkParticipantPaid)
	split.Get("/:id/share", h.SplitBill.Share)

	// ---------------- Admin (auth + role=admin) ----------------
	admin := v1.Group("/admin", authRequired, middleware.RequireAdmin)
	adminUsers := admin.Group("/users")
	adminUsers.Get("", h.User.List)
	adminUsers.Get("/:id", h.User.Get)
	adminUsers.Post("", h.User.Create)
	adminUsers.Put("/:id", h.User.Update)
	adminUsers.Delete("/:id", h.User.Delete)
	admin.Get("/subscriptions", h.Subscription.ListAllAdmin)
	// ---------------- Super Admin (auth + role=super_admin) ----------------
	superAdmin := v1.Group("/admin", authRequired, middleware.RequireSuperAdmin)
	superAdmin.Get("/ai-logs", h.AILog.ListAll)
}
