package routes

import (
	"time"

	"github.com/ganiramadhan/starter-go/internal/middleware"
	"github.com/ganiramadhan/starter-go/internal/modules/ai"
	"github.com/ganiramadhan/starter-go/internal/modules/ailog"
	"github.com/ganiramadhan/starter-go/internal/modules/auth"
	"github.com/ganiramadhan/starter-go/internal/modules/budget"
	"github.com/ganiramadhan/starter-go/internal/modules/category"
	"github.com/ganiramadhan/starter-go/internal/modules/notification"
	"github.com/ganiramadhan/starter-go/internal/modules/savingsgoal"
	"github.com/ganiramadhan/starter-go/internal/modules/splitbill"
	"github.com/ganiramadhan/starter-go/internal/modules/subscription"
	"github.com/ganiramadhan/starter-go/internal/modules/support"
	"github.com/ganiramadhan/starter-go/internal/modules/telegram"
	"github.com/ganiramadhan/starter-go/internal/modules/transaction"
	"github.com/ganiramadhan/starter-go/internal/modules/upcomingbilling"
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
	Billing      *upcomingbilling.Handler
	AI           *ai.Handler
	Notification *notification.Handler
	Support      *support.Handler
	Telegram     *telegram.Handler
}

func (h Handlers) WithSupport(s *support.Handler) Handlers {
	h.Support = s
	return h
}

func Register(app *fiber.App, h Handlers, jwtMgr *jwt.Manager) {
	authRequired := middleware.AuthRequired(jwtMgr)

	v1 := app.Group("/api/v1")

	// ---------------- Public ----------------
	authPub := v1.Group("/auth")
	authPub.Post("/login", middleware.LoginRateLimiter(), h.Auth.Login)
	authPub.Post("/register", middleware.SensitiveRateLimiter(5, time.Minute, 10*time.Minute, "Too many registration attempts. Please try again later."), h.Auth.Register)
	authPub.Post("/register/verify", middleware.SensitiveRateLimiter(8, time.Minute, 10*time.Minute, "Too many verification attempts. Please try again later."), h.Auth.VerifyRegistration)
	authPub.Post("/register/resend-otp", middleware.SensitiveRateLimiter(3, time.Minute, 10*time.Minute, "Too many OTP requests. Please try again later."), h.Auth.ResendRegistrationOTP)
	authPub.Post("/google", middleware.SensitiveRateLimiter(10, time.Minute, 5*time.Minute, "Too many Google login attempts. Please try again later."), h.Auth.GoogleLogin)
	authPub.Post("/forgot-password", middleware.SensitiveRateLimiter(3, time.Minute, 10*time.Minute, "Too many password reset attempts. Please try again later."), h.Auth.ForgotPassword)
	authPub.Post("/reset-password", middleware.SensitiveRateLimiter(5, time.Minute, 10*time.Minute, "Too many password reset attempts. Please try again later."), h.Auth.ResetPassword)

	// Public subscription catalog + Midtrans webhook
	subsPub := v1.Group("/subscriptions")
	subsPub.Get("/plans", h.Subscription.ListPlans)
	subsPub.Post("/webhook", h.Subscription.Webhook)

	if h.Telegram != nil {
		telegramPub := v1.Group("/telegram")
		telegramPub.Post("/webhook/:secret", h.Telegram.Webhook)
	}

	// ---------------- User (authenticated) ----------------
	authPriv := v1.Group("/auth", authRequired)
	authPriv.Post("/logout", h.Auth.Logout)
	authPriv.Post("/change-password", h.Auth.ChangePassword)

	usersMe := v1.Group("/users", authRequired)
	usersMe.Get("/me", h.User.Me)
	usersMe.Put("/me", h.User.UpdateMe)
	usersMe.Put("/me/email", h.User.ChangeEmail)
	usersMe.Post("/me/telegram", h.User.BindTelegram)
	usersMe.Delete("/me/telegram", h.User.DisconnectTelegram)
	usersMe.Delete("/me", h.User.DeleteMe)
	usersMe.Post("/upload-photo", h.User.UploadPhoto)
	usersMe.Delete("/me/photo", h.User.DeleteMyPhoto)

	// ----- SAKU finance modules (tenant-scoped) -----
	wallets := v1.Group("/wallets", authRequired)
	wallets.Get("", h.Wallet.List)
	wallets.Post("", h.Wallet.Create)
	wallets.Get("/transfers", h.Wallet.ListTransfers)
	wallets.Delete("/transfers", h.Wallet.DeleteTransfers)
	wallets.Post("/transfer", h.Wallet.Transfer)
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
	aiLogs.Post("/bulk-delete", h.AILog.DeleteMany)
	aiLogs.Delete("/:id", h.AILog.Delete)

	budgets := v1.Group("/budgets", authRequired)
	budgets.Get("", h.Budget.List)
	budgets.Post("", h.Budget.Create)
	budgets.Get("/:id", h.Budget.Get)
	budgets.Put("/:id", h.Budget.Update)
	budgets.Delete("/:id", h.Budget.Delete)

	billings := v1.Group("/upcoming-billings", authRequired)
	billings.Get("", h.Billing.List)
	billings.Post("", h.Billing.Create)
	billings.Put("/:id", h.Billing.Update)
	billings.Delete("/:id", h.Billing.Delete)

	notifications := v1.Group("/notifications", authRequired)
	notifications.Get("", h.Notification.List)
	notifications.Post("/read-all", h.Notification.MarkAllRead)
	notifications.Post("/:id/read", h.Notification.MarkRead)

	supportTickets := v1.Group("/support-tickets", authRequired)
	supportTickets.Get("", h.Support.List)
	supportTickets.Post("", h.Support.Create)
	supportTickets.Post("/attachments", h.Support.UploadAttachment)
	supportTickets.Post("/:id/reply", h.Support.Reply)

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
	subs.Post("/checkout", middleware.SensitiveRateLimiter(6, time.Minute, 5*time.Minute, "Too many checkout attempts. Please try again later."), h.Subscription.Checkout)
	subs.Post("/voucher/validate", middleware.SensitiveRateLimiter(12, time.Minute, 5*time.Minute, "Too many voucher attempts. Please try again later."), h.Subscription.ValidateVoucher)
	subs.Post("/confirm", middleware.SensitiveRateLimiter(10, time.Minute, 5*time.Minute, "Too many payment confirmation attempts. Please try again later."), h.Subscription.ConfirmCheckout)
	subs.Post("/:id/renew-invoice", middleware.SensitiveRateLimiter(6, time.Minute, 5*time.Minute, "Too many invoice attempts. Please try again later."), h.Subscription.RenewInvoice)
	subs.Post("/:id/cancel", h.Subscription.Cancel)

	// AI categorization
	aiGroup := v1.Group("/ai", authRequired)
	aiTextLimiter := middleware.SensitiveRateLimiter(30, time.Minute, 2*time.Minute, "Too many AI requests. Please wait a moment before trying again.")
	aiVisionLimiter := middleware.SensitiveRateLimiter(12, time.Minute, 5*time.Minute, "Too many receipt scan requests. Please wait a moment before trying again.")
	aiGroup.Post("/categorize", aiTextLimiter, h.AI.Categorize)
	aiGroup.Post("/scan-receipt", aiVisionLimiter, h.AI.ScanReceipt)
	aiGroup.Post("/scan-receipt/promote-image", h.AI.PromoteScanImage)
	aiGroup.Post("/insights", aiTextLimiter, h.AI.Insights)
	aiGroup.Post("/suggest-budget", aiTextLimiter, h.AI.SuggestBudget)
	aiGroup.Post("/chat", aiTextLimiter, h.AI.Chat)
	aiGroup.Get("/chat-history", h.AILog.ListChatHistory)
	aiGroup.Get("/nlp-history", h.AILog.ListNLPHistory)
	aiGroup.Get("/scan-receipt-history", h.AILog.ListScanReceiptHistory)

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
	admin.Get("/vouchers", h.Subscription.ListVouchersAdmin)
	admin.Post("/vouchers", h.Subscription.CreateVoucherAdmin)
	admin.Put("/vouchers/:id", h.Subscription.UpdateVoucherAdmin)
	admin.Delete("/vouchers/:id", h.Subscription.DeleteVoucherAdmin)
	admin.Get("/support-tickets", h.Support.List)
	admin.Post("/support-tickets/attachments", h.Support.UploadAttachment)
	admin.Post("/support-tickets/:id/reply", h.Support.Reply)
	admin.Patch("/support-tickets/:id/status", h.Support.UpdateStatus)
	// ---------------- Super Admin (auth + role=super_admin) ----------------
	superAdmin := v1.Group("/admin", authRequired, middleware.RequireSuperAdmin)
	superAdmin.Get("/ai-logs", h.AILog.ListAll)
}
