package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/ganiramadhan/starter-go/internal/modules/ailog"
	"github.com/ganiramadhan/starter-go/internal/modules/category"
	"github.com/ganiramadhan/starter-go/internal/modules/subscription"
	"github.com/ganiramadhan/starter-go/internal/modules/transaction"
	"github.com/ganiramadhan/starter-go/internal/modules/wallet"
	aiplatform "github.com/ganiramadhan/starter-go/internal/platform/ai"
	"github.com/ganiramadhan/starter-go/internal/platform/storage"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const (
	confidenceReviewThreshold = 0.70

	featureCategorize    = "categorize"
	featureScanReceipt   = "scan_receipt"
	featureInsights      = "insights"
	featureSuggestBudget = "suggest_budget"
	featureChat          = "chat"

	freeScanReceiptMonthlyLimit = 10
	freeChatMonthlyLimit        = 20
	proChatMonthlyLimit         = 300
	proScanMonthlyLimit         = 100
	premiumChatMonthlyLimit     = 1200
	premiumScanMonthlyLimit     = 300
)

type Service interface {
	Categorize(ctx context.Context, userID uuid.UUID, req dto.CategorizeRequest) (dto.CategorizeResponse, error)
	ScanReceipt(ctx context.Context, userID uuid.UUID, req dto.ScanReceiptRequest) (dto.ScanReceiptResponse, error)
	PromoteScanImage(ctx context.Context, userID uuid.UUID, req dto.PromoteScanImageRequest) (dto.PromoteScanImageResponse, error)
	Insights(ctx context.Context, userID uuid.UUID, req dto.InsightsRequest) (dto.InsightsResponse, error)
	SuggestBudget(ctx context.Context, userID uuid.UUID, req dto.SuggestBudgetRequest) (dto.SuggestBudgetResponse, error)
	Chat(ctx context.Context, userID uuid.UUID, req dto.ChatRequest) (dto.ChatResponse, error)
}

type service struct {
	claude  *aiplatform.Client
	txns    transaction.Repository
	wallets wallet.Repository
	cats    category.Repository
	logs    ailog.Service
	storage storage.Storage
	model   string
	subs    subscription.Service
}

func NewService(claude *aiplatform.Client, txns transaction.Repository, wallets wallet.Repository, cats category.Repository, logs ailog.Service, store storage.Storage, model string, subs ...subscription.Service) Service {
	if model == "" {
		model = "claude-sonnet-4-5"
	}
	var subSvc subscription.Service
	if len(subs) > 0 {
		subSvc = subs[0]
	}
	return &service{claude: claude, txns: txns, wallets: wallets, cats: cats, logs: logs, storage: store, model: model, subs: subSvc}
}

func (s *service) enforceDailyQuota(ctx context.Context, userID uuid.UUID, features []string, limit int, message string) error {
	planCode := "free"
	hasActivePlan := false
	if s.subs != nil {
		code, active, err := s.subs.ActivePlanCode(ctx, userID)
		if err != nil {
			return err
		}
		planCode = code
		hasActivePlan = active
	}
	if s.logs == nil || limit <= 0 {
		return nil
	}

	now := time.Now()
	since := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	effectiveLimit := limit
	effectiveMessage := message
	if hasActivePlan {
		if containsFeature(features, featureScanReceipt) {
			effectiveLimit = proScanMonthlyLimit
			effectiveMessage = "Pro plan monthly OCR limit reached. Upgrade to Premium for more receipt scans"
			if strings.Contains(planCode, "premium") {
				effectiveLimit = premiumScanMonthlyLimit
				effectiveMessage = "Premium plan monthly OCR limit reached"
			}
		} else {
			effectiveLimit = proChatMonthlyLimit
			effectiveMessage = "Pro plan monthly AI limit reached. Upgrade to Premium for more AI prompts"
			if strings.Contains(planCode, "premium") {
				effectiveLimit = premiumChatMonthlyLimit
				effectiveMessage = "Premium plan monthly AI limit reached"
			}
		}
	}
	used, err := s.logs.CountByUserSince(ctx, userID, features, since)
	if err != nil {
		return err
	}
	if used >= int64(effectiveLimit) {
		return fiber.NewError(fiber.StatusForbidden, effectiveMessage)
	}
	return nil
}

func containsFeature(features []string, feature string) bool {
	for _, item := range features {
		if item == feature {
			return true
		}
	}
	return false
}

const systemPrompt = `You are SAKU, an AI assistant for personal finance.
Use the explicitly requested language when the prompt provides one; otherwise reply in the same language the user used.
When asked for structured data, return ONLY valid JSON without markdown fences.
Never include commentary outside the JSON object.
Be conservative with money data: never invent amounts, merchants, categories, dates, or line items.
If an amount, date, wallet, merchant, or transaction type is ambiguous, lower confidence instead of guessing.
For Indonesian users, understand everyday wording, typos, and shorthand such as "pake", "pakai", "rb", "ribu", "jt", "kemarin", "tadi pagi", "cash", "tunai", "dompet", and bank/e-wallet names.
SAKU is a review-first product: ambiguous outputs should be easy for the user to correct before saving.`

const chatSystemPrompt = `You are SAKU, a friendly assistant embedded INSIDE the SAKU personal-finance application.

SCOPE — you help with:
- The user's own personal finance: income, expenses, budgets, savings, wallets, categories, transactions.
- Reading, summarising, listing, comparing, ranking and re-formatting the user's transactions and totals.
- Recording a new transaction when the user clearly asks (just confirm in one short sentence).
- How to use SAKU features (scan receipt, budgets, dashboard, categories, wallets).
- Light financial tips & literacy directly relevant to managing personal money in Indonesia.

INTENT ROUTING:
- If the user asks a finance question, answer from the provided transaction/wallet context first.
- If the user writes a transaction-like sentence, help them review/confirm it rather than giving generic advice.
- If the user corrects a previous transaction ("eh maksudnya kemarin", "ganti wallet ke cash", "yang tadi 35 ribu"), treat it as a correction to the latest pending/recent transaction context.
- If the request is ambiguous, ask one concise clarification question instead of inventing details.

FOLLOW-UPS — IMPORTANT:
- The conversation history is provided to you. Treat short messages like "buat list",
  "format json", "buat tabel", "urutkan dari termahal", "jelaskan lebih detail",
  "jadikan markdown", "ringkas" as REFORMAT requests on the PREVIOUS assistant
  answer (or the user's known data) and comply faithfully.
- If the user asks for JSON or a list/table, output it cleanly (markdown allowed).
  Do NOT refuse just because the request mentions JSON / list / table.
- NEVER hallucinate transactions. Only mention numbers that appear in the provided context or previous assistant message.
- Treat exact summaries such as today_transactions, today_income, today_expense,
  requested_date_transactions, and month_transactions as authoritative database
  results. Never say there are no transactions when one of those values is above zero.
- Lead with the direct answer and core number first. Then add at most one useful
  interpretation or next action. Avoid formal filler such as "Berdasarkan data yang tersedia".
- If the user asks for a count, include the count plus income, expense, and net
  when those exact values are available. If there are transactions, mention up
  to three recent examples so the answer is easy to verify.
- Wallet-aware questions are common. If the user mentions a wallet, bank, e-wallet,
  "cash", "tunai", "dompet", "rekening", or asks remaining balance, use the wallet
  fields and wallet summaries in context. Do not say wallet data is unavailable when
  wallet context or transaction rows include wallet names.
- When the user provides a starting balance ("saldo awal", "modal awal", "awalnya"),
  calculate remaining balance as starting_balance + income - expense for the matching
  wallet/period. Explain the formula briefly.

REFUSE ONLY WHEN the user CLEARLY asks for something with no link at all to personal finance or SAKU (e.g. recipes, weather, lyrics, code help, jokes, politics, medical advice, game cheats, translations, general knowledge trivia). In that case reply with EXACTLY ONE short sentence in the user's language, e.g. "Maaf, saya hanya bisa membantu seputar keuangan pribadi dan fitur SAKU." — then STOP. Never tack on extra finance commentary after a refusal.

OUTPUT RULES:
- Reply in the required response language provided in the prompt. If none is provided, use English by default.
- Use Rupiah formatting like "Rp 25.000" when mentioning money.
- Keep prose answers concise (2-6 short sentences). For list/table/JSON requests, output may be longer — stay accurate.
- Do not use markdown emphasis like **bold**. Prefer clean professional plain text.
- For calculation questions, be exact and show the core numbers used. If data is
  partial because only a sample was loaded, say that clearly.
- If the user seems confused, tell them they can type "help" for guidance.
- Do NOT invent figures. If a number isn't in the context / previous answer, say you don't have it.
- If total_transactions is larger than the loaded sample, state which values are exact and which are based on the loaded summary.`

// ─── 1. Categorize from raw text ────────────────────────────────────────────

func (s *service) Categorize(ctx context.Context, userID uuid.UUID, req dto.CategorizeRequest) (dto.CategorizeResponse, error) {
	if err := s.enforceDailyQuota(ctx, userID, []string{featureCategorize, featureChat}, freeChatMonthlyLimit, "Free plan includes 20 AI chat prompts per month. Upgrade to Pro for 300 prompts/month"); err != nil {
		return dto.CategorizeResponse{}, err
	}
	cats := s.resolveCategories(userID, req.UserCategories)
	now := requestReferenceTime(req.ReferenceDate, req.Timezone)
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	tomorrow := now.AddDate(0, 0, 1).Format("2006-01-02")
	lang := preferredLanguage(req.Text, req.Language)
	descriptionLanguage := "English"
	if lang == "id" {
		descriptionLanguage = "Bahasa Indonesia"
	}

	prompt := fmt.Sprintf(`You will receive a free-form Indonesian/English text describing one OR MORE personal finance transactions.

Required output language for the "description" field: %s.

TODAY'S DATE is %s. Interpret relative Indonesian/English dates against this date:
- "hari ini" / "today" = %s
- "kemarin" / "yesterday" = %s
- "besok" / "tomorrow" = %s
- If no date is mentioned, use %s.

ALLOWED CATEGORIES (you MUST pick the BEST match from this exact list, case-insensitive):
%s

CATEGORY RULES — critical to avoid "Uncategorized" results:
- You MUST set "category" to one of the names above, EXACTLY as written.
- NEVER output "Uncategorized", "Unknown", "-", empty string, or any name not in the list.
- If no perfect match exists, pick the closest generic one from the list
  (e.g. "Lainnya" / "Other" / "Misc" if available; otherwise pick the broadest
  applicable category like "Shopping" / "Food & Beverage").
- Common mappings:
  restaurant/cafe/warung/makan → Makanan & Minuman / Food & Beverage;
  grab/gojek/taxi/transport/bensin → Transportasi / Transportation;
  tokopedia/shopee/mall/baju → Belanja / Shopping;
  pln/listrik/wifi/internet/pulsa/hosting/domain/server/vps/software/subscription → Tagihan / Bills;
  bioskop/spotify/netflix → Hiburan / Entertainment; transfer ke teman → Transfer; gaji/salary/payroll → Gaji / Salary.

GENERAL RULES:
1. Detect EVERY distinct transaction in the text. Split on conjunctions like
   ",", ".", "terus", "lalu", "kemudian", "dan", "then", new sentences,
   or whenever a new amount + new item/merchant is mentioned.
   IMPORTANT: do NOT split a sentence into multiple transactions when one
   amount covers multiple items at the same merchant, e.g. "kopi dan makanan
   di Houseplants 52 ribu" is ONE transaction.
2. Indonesian shorthand amounts:
   - "14rb" / "14 ribu" / "14k" → 14000
   - "121" alone (no rb/ribu/k) in a money context = 121000 ONLY if the
     surrounding sentence clearly talks about price; otherwise 121.
   - "696 ribu" → 696000; "5jt" / "5 juta" → 5000000.
3. For each transaction extract:
   - amount (number, no separators, no currency)
   - merchant_name (the place/store/source; "-" if not clear)
   - category (best match from the allowed list above — see CATEGORY RULES)
   - type: "expense" unless the text clearly says income/gaji/bonus/refund/masuk
   - confidence: 0..1, how sure you are
   - description: a SHORT (max 6 words) summary of THIS transaction only
   - date: transaction date as YYYY-MM-DD, using the relative date rules above
   - wallet_hint: the funding destination/source phrase if explicitly mentioned
     for THIS transaction, e.g. "Bank Mandiri", "Self Love Bank Jago",
     "Dana Darurat Bank Jago", "Savings", "Cash". Empty string if not mentioned.
     For allocation sentences with several amounts, use the nearest wallet phrase
     around each amount; do not copy one wallet_hint to every item.
4. Payment method / wallet phrases are NOT merchant names and should not change
   the category: "pake/pakai/menggunakan/via cash/tunai/Bank Mandiri/BCA/Jago/
   dompet Self Love" only describes the funding source.
5. Handle common typo/noisy user input gracefully, e.g. "menggunkaan" =
   "menggunakan", "pake" = "pakai", "rb" = "ribu".
6. NEVER merge multiple distinct purchases into one transaction.
7. NEVER invent transactions that are not in the text.
8. If the user mentions a recurring schedule ("setiap bulan", "mingguan",
   "monthly", "every month", "recurring"), still extract the current
   transaction normally and add "recurring_hint" with the schedule phrase.
   Do not create extra future transactions.
9. If the text contains "Informasi lanjutan dari pengguna" or
   "Additional detail from the user", the text after that label is a FOLLOW-UP
   that completes the transaction context before it. Merge those details into
   the incomplete transaction; do NOT create a separate transaction from the
   follow-up answer.

Text:
"""
%s
"""

Return ONLY this JSON shape (no markdown, no commentary):
{
  "transactions": [
    {"amount": number, "merchant_name": string, "category": string, "type": "income"|"expense", "confidence": 0..1, "description": string, "date": "YYYY-MM-DD", "wallet_hint": string, "recurring_hint": string}
  ]
}`,
		descriptionLanguage, today, today, yesterday, tomorrow, today, strings.Join(cats, ", "), req.Text)

	start := time.Now()
	raw, err := s.claude.AskWithSystem(ctx, systemPrompt, prompt)
	latency := int(time.Since(start).Milliseconds())

	if err != nil {
		s.record(userID, aiLogEntry{Feature: featureCategorize, Status: "failed", LatencyMs: latency, ErrMsg: err.Error(), Raw: map[string]any{"message": req.Text, "session_id": req.SessionID}})
		return dto.CategorizeResponse{}, err
	}

	parsed := parseJSON(raw)
	if parsed == nil {
		parsed = map[string]any{}
	}
	parsed["message"] = req.Text
	parsed["session_id"] = req.SessionID
	items := normalizeCategorizeItems(extractCategorizeItems(parsed), cats)

	out := dto.CategorizeResponse{
		RawResponse:  parsed,
		Transactions: items,
	}
	if len(items) > 0 {
		first := items[0]
		out.Amount = first.Amount
		out.MerchantName = first.MerchantName
		out.Category = first.Category
		out.Type = first.Type
		out.Confidence = first.Confidence
		out.Date = first.Date
	} else {
		out.Amount = getNumber(parsed, "amount")
		out.MerchantName = getString(parsed, "merchant_name")
		out.Category = normalizeCategoryChoice(cats, firstString(parsed, "category", "kategori"))
		out.Type = normalizeType(getString(parsed, "type"))
		out.Confidence = getNumber(parsed, "confidence")
		out.Date = firstString(parsed, "date", "transaction_date")
		if out.Amount > 0 || out.MerchantName != "" || out.Category != "" {
			out.Transactions = []dto.CategorizeItem{{
				Amount:       out.Amount,
				MerchantName: out.MerchantName,
				Category:     out.Category,
				Type:         out.Type,
				Confidence:   out.Confidence,
				Description:  req.Text,
				Date:         out.Date,
			}}
		}
	}
	out.NeedsReview = needsReview(out.Confidence, parsed == nil)
	out.MissingFields, out.ClarificationQuestion = categorizeClarification(out, req.Text, lang)
	out.NeedsClarification = len(out.MissingFields) > 0

	s.logLowConfidence(featureCategorize, out.Confidence, parsed)
	s.record(userID, aiLogEntry{
		Feature: featureCategorize, Status: "success", LatencyMs: latency,
		Confidence: &out.Confidence, Merchant: out.MerchantName, Category: out.Category,
		Amount: amountPtr(out.Amount), Raw: parsed,
	})
	return out, nil
}

func categorizeClarification(out dto.CategorizeResponse, input, language string) ([]string, string) {
	if out.Amount > 0 {
		return nil, ""
	}

	subject := strings.TrimSpace(out.MerchantName)
	if subject == "" || subject == "-" {
		if len(out.Transactions) > 0 {
			subject = strings.TrimSpace(out.Transactions[0].Description)
		}
	}
	if subject == "" {
		subject = conciseTransactionSubject(input)
	}

	if language == "id" {
		if subject != "" {
			return []string{"amount"}, fmt.Sprintf("Nominal untuk %s berapa? Contoh: Rp 500.000.", subject)
		}
		return []string{"amount"}, "Nominal transaksinya berapa? Contoh: Rp 500.000."
	}
	if subject != "" {
		return []string{"amount"}, fmt.Sprintf("How much was %s? For example: Rp 500,000.", subject)
	}
	return []string{"amount"}, "What was the transaction amount? For example: Rp 500,000."
}

func conciseTransactionSubject(input string) string {
	cleaned := strings.TrimSpace(input)
	cleaned = regexp.MustCompile(`(?i)^(tolong\s+)?(catat|bayar|beli|record|pay|paid)\s+`).ReplaceAllString(cleaned, "")
	cleaned = strings.TrimSpace(strings.Trim(cleaned, ".,!?"))
	words := strings.Fields(cleaned)
	if len(words) > 8 {
		words = words[:8]
	}
	return strings.Join(words, " ")
}

func extractCategorizeItems(parsed map[string]any) []dto.CategorizeItem {
	if parsed == nil {
		return nil
	}
	rawList, ok := parsed["transactions"].([]any)
	if !ok || len(rawList) == 0 {
		return nil
	}
	out := make([]dto.CategorizeItem, 0, len(rawList))
	for _, r := range rawList {
		obj, ok := r.(map[string]any)
		if !ok {
			continue
		}
		item := dto.CategorizeItem{
			Amount:        getNumber(obj, "amount"),
			MerchantName:  getString(obj, "merchant_name"),
			Category:      firstString(obj, "category", "kategori"),
			Type:          normalizeType(getString(obj, "type")),
			Confidence:    getNumber(obj, "confidence"),
			Description:   getString(obj, "description"),
			Date:          firstString(obj, "date", "transaction_date"),
			WalletHint:    firstString(obj, "wallet_hint", "wallet", "dompet", "rekening"),
			RecurringHint: firstString(obj, "recurring_hint", "recurring", "schedule"),
		}
		if item.Amount <= 0 && item.MerchantName == "" && item.Category == "" {
			continue
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeCategorizeItems(items []dto.CategorizeItem, allowed []string) []dto.CategorizeItem {
	for i := range items {
		items[i].Category = normalizeCategoryChoice(allowed, items[i].Category)
	}
	return items
}

func (s *service) ScanReceipt(ctx context.Context, userID uuid.UUID, req dto.ScanReceiptRequest) (dto.ScanReceiptResponse, error) {
	if err := s.enforceDailyQuota(ctx, userID, []string{featureScanReceipt}, freeScanReceiptMonthlyLimit, "Free plan includes 10 OCR scans per month. Upgrade to Pro for 100 scans/month"); err != nil {
		return dto.ScanReceiptResponse{}, err
	}
	cats := s.resolveCategories(userID, req.UserCategories)
	mediaType := req.MediaType
	if mediaType == "" {
		mediaType = "image/jpeg"
	}

	prompt := fmt.Sprintf(`This image is a receipt, invoice, payment proof, bank mutation, or e-wallet transaction screenshot.
Extract structured data with high precision and pick the best-fitting category from this exact list (case-insensitive): %s

CRITICAL RULES — read carefully BEFORE deciding the type and category:

1. "type" must be one of: "income" or "expense".
   - Default to "expense" unless there is unambiguous evidence the user RECEIVED money.
   - Most uploads are the user's own outgoing transfer/payment receipt → expense.
   - NEVER guess "income" just because a name on the receipt looks like a person.

2. Detect TRANSFER receipts (Jago, BCA, Mandiri, BRI, BNI, GoPay, OVO, DANA,
   ShopeePay, LinkAja, SeaBank, Jenius, Permata, etc.):
   - Indonesian banks usually show:
       * "Sumber akun" / "Sumber dana" / "Source" / "From" / "Dari" → SENDER (payer)
       * "Tujuan" / "Penerima" / "Destination" / "To" / "Kepada"   → RECIPIENT (payee)
   - The big name shown at the TOP of the receipt is usually the RECIPIENT
     (the person/account the transfer was sent TO), not the user.
   - The user uploading the receipt is almost always the SENDER → mark as EXPENSE.
   - Only mark as "income" when the receipt explicitly says: "diterima",
     "transfer masuk", "dana masuk", "received", "top-up berhasil", "refund",
     "credited to your account", or it is clearly a payroll/salary slip.

3. "category" rules — VERY IMPORTANT to avoid wrong categorization:
   - Pick the BEST matching category from the allowed list above and output it EXACTLY as written.
   - Never invent a category and never output Uncategorized/Unknown/Other unless that exact category exists in the list.
   - Peer-to-peer bank transfers WITHOUT a clear merchant context should
     be categorized as "Transfer" if available; otherwise pick "Lainnya"
     / "Other" / "Misc". Do NOT default to "Salary" / "Gaji" / "Income".
   - Use "Salary" / "Gaji" ONLY when the receipt explicitly mentions payroll,
     salary, gaji, payslip, or the sender is clearly a company/employer
     paying the user (employer name + matching keywords).
   - Use a food/transport/shopping/utility category ONLY when the merchant or
     description clearly matches that domain (e.g. Grab, Gojek, Tokopedia,
     Indomaret, PLN, Telkom, restaurant brand, etc.).
   - When in doubt, pick the most generic available category — never invent
     a category outside the allowed list.
   - If the image shows service items (laundry, food, groceries, subscription, bill)
     and also shows a payment status/transfer method, categorize the underlying service,
     not the transfer method.

4. "merchant_name" rules:
   - For a transfer OUT (expense): use the RECIPIENT's name / destination
     account holder shown at the top of the receipt.
   - For a received transfer (income): use the SENDER's name.
   - For a store/merchant receipt: use the merchant/store brand name.
   - NEVER use the user's own name (the sender) as merchant_name on an
     outgoing transfer.
   - If a receipt has item/service details and a separate payer/payee name,
     prefer the business/service provider as merchant_name. Do not use a random
     transfer recipient if it is not the merchant.

5. "amount" must be the numeric total in the receipt's currency. Strip currency
   symbols and thousand separators. Indonesian format "Rp 210.000" = 210000,
   "Rp 1.250.500,50" = 1250500.50. If you cannot find an amount, return 0.

6. Date rules:
   - Indonesian receipts commonly use DD/MM/YYYY or DD-MM-YYYY. Interpret
     "01/06/2026" as 2026-06-01, not 2006-01-06.
   - If both day and month are <= 12 and the locale is unclear, prefer
     Indonesian DD/MM/YYYY for receipts from Indonesian merchants/banks.
   - If only month/year or an unreadable date is visible, lower confidence
     below 0.7 so the UI can ask the user to review.

7. "confidence" should reflect how certain you are about (type + category +
   amount + merchant). If any of these is ambiguous, set confidence below 0.7
   so the UI can ask the user to review.

8. Description rules:
   - For store/service receipts, describe the actual goods or service naturally,
     e.g. "Laundry cuci kering gosok", "Belanja Indomaret: roti, susu".
   - For bank/e-wallet transfer receipts, DO NOT use "belanja" unless it is clearly a merchant payment.
     Use transfer wording, e.g. "Transfer ke BUDI", "Mutasi masuk dari PT ABC",
     "Top up GoPay", or "Pembayaran QRIS ke Merchant".
   - If line items clearly identify the service, the description must follow those line items
     rather than a generic transfer label.

9. "line_items" should be informative:
   - For store receipts, include item name, quantity when visible, final price, and discount when visible
     in one short string, e.g. "Susu UHT x2 - Rp 24.000 (diskon Rp 3.000)".
   - For transfer/bank mutation receipts, include fee/admin, source account, destination account,
     reference number, or status lines when visible.

Return ONLY this JSON (no commentary, no markdown fences):
{"amount": number, "merchant_name": string, "category": string, "type": "income"|"expense", "currency": string, "date": "YYYY-MM-DD", "confidence": 0..1, "description": string, "line_items": ["item/fee/detail with price or discount when visible"], "ocr_text": string}`,
		strings.Join(cats, ", "))

	start := time.Now()
	scanCtx, scanCancel := context.WithTimeout(ctx, 75*time.Second)
	raw, err := s.claude.AskImage(scanCtx, systemPrompt, prompt, mediaType, req.ImageBase64)
	scanCancel()
	latency := int(time.Since(start).Milliseconds())

	if err != nil {
		s.record(userID, aiLogEntry{Feature: featureScanReceipt, Status: "failed", LatencyMs: latency, ErrMsg: err.Error(), Raw: map[string]any{"media_type": mediaType}})
		return dto.ScanReceiptResponse{}, err
	}

	parsed := parseJSON(raw)
	out := dto.ScanReceiptResponse{
		Amount:       getNumber(parsed, "amount"),
		MerchantName: getString(parsed, "merchant_name"),
		Category:     normalizeCategoryChoice(cats, firstString(parsed, "category", "kategori")),
		Type:         normalizeType(getString(parsed, "type")),
		Currency:     defaultIfEmpty(getString(parsed, "currency"), "IDR"),
		Date:         getString(parsed, "date"),
		Confidence:   getNumber(parsed, "confidence"),
		Description:  getString(parsed, "description"),
		OCRText:      getString(parsed, "ocr_text"),
		LineItems:    getStringSlice(parsed, "line_items"),
		RawResponse:  parsed,
	}
	out.NeedsReview = needsReview(out.Confidence, parsed == nil)

	if parsed == nil {
		parsed = map[string]any{}
	}
	parsed["saved"] = false
	if s.storage != nil && req.ImageBase64 != "" {
		if data, derr := base64.StdEncoding.DecodeString(req.ImageBase64); derr == nil && len(data) > 0 {
			folder := fmt.Sprintf("Temp/AI/Scans/%s", userID.String())
			uploadCtx, uploadCancel := context.WithTimeout(ctx, 4*time.Second)
			if key, uerr := s.storage.UploadBytes(uploadCtx, data, mediaType, folder, ""); uerr == nil {
				parsed["image_key"] = key
				out.ImageKey = key
			} else {
				if isTransientObjectStorageError(uerr) {
					slog.Info("ai: scan receipt image upload skipped; object storage temporary error", "user_id", userID, "error", uerr)
				} else {
					slog.Warn("ai: scan receipt image upload failed", "user_id", userID, "error", uerr)
				}
			}
			uploadCancel()
		} else if derr != nil {
			slog.Warn("ai: scan receipt base64 decode failed", "user_id", userID, "error", derr)
		}
	}

	s.logLowConfidence(featureScanReceipt, out.Confidence, parsed)
	logID := s.record(userID, aiLogEntry{
		Feature: featureScanReceipt, Status: "success", LatencyMs: latency,
		Confidence: &out.Confidence, Merchant: out.MerchantName, Category: out.Category,
		Amount: &out.Amount, Raw: parsed,
	})
	if logID != uuid.Nil {
		out.LogID = logID.String()
	}
	return out, nil
}

func isTransientObjectStorageError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "statuscode: 502") ||
		strings.Contains(msg, "bad gateway") ||
		strings.Contains(msg, "xml syntax error") ||
		strings.Contains(msg, "exceeded maximum number of attempts") ||
		strings.Contains(msg, "closed by </body>")
}

func (s *service) PromoteScanImage(ctx context.Context, userID uuid.UUID, req dto.PromoteScanImageRequest) (dto.PromoteScanImageResponse, error) {
	if s.storage == nil {
		return dto.PromoteScanImageResponse{}, fmt.Errorf("storage is not configured")
	}
	oldKey := strings.TrimSpace(req.ImageKey)
	prefix := fmt.Sprintf("Temp/AI/Scans/%s/", userID.String())
	if !strings.HasPrefix(oldKey, prefix) {
		return dto.PromoteScanImageResponse{}, fmt.Errorf("%w: invalid scan image key", domain.ErrInvalidInput)
	}
	filename := oldKey[strings.LastIndex(oldKey, "/")+1:]
	newKey := fmt.Sprintf("AI/Scans/%s/%s", userID.String(), filename)
	if err := s.storage.Move(ctx, oldKey, newKey); err != nil {
		return dto.PromoteScanImageResponse{}, err
	}
	if s.logs != nil {
		var err error
		if strings.TrimSpace(req.LogID) != "" {
			if logID, perr := uuid.Parse(strings.TrimSpace(req.LogID)); perr == nil {
				err = s.logs.MarkScanSaved(ctx, userID, logID, newKey)
			} else {
				err = perr
			}
		} else {
			err = s.logs.PromoteImage(ctx, userID, oldKey, newKey)
		}
		if err != nil {
			slog.Warn("ai: update promoted scan image key failed", "user_id", userID, "error", err)
		}
	}
	return dto.PromoteScanImageResponse{ImageKey: newKey}, nil
}

func (s *service) Insights(ctx context.Context, userID uuid.UUID, req dto.InsightsRequest) (dto.InsightsResponse, error) {
	from, to := parseRange(req.From, req.To, 30)
	limit := req.Limit
	if limit == 0 {
		limit = 200
	}

	rows, _, err := s.txns.List(transaction.ListFilter{
		UserID: userID, From: &from, To: &to, Page: 1, Limit: limit,
	})
	if err != nil {
		return dto.InsightsResponse{}, fmt.Errorf("load transactions: %w", err)
	}

	income, expense, byCat := summarise(rows)
	prompt := fmt.Sprintf(`Analyse these personal finance transactions for the period %s to %s.

Totals: income=%.2f, expense=%.2f
Top categories (by spend): %s

Sample transactions (newest first):
%s

Return ONLY this JSON:
{
  "summary": "1-2 sentence overview in user language",
  "top_categories": ["..."],
  "recommendations": ["...", "..."],
  "anomalies": ["..."],
  "health_score": 0..100
}`,
		from.Format("2006-01-02"), to.Format("2006-01-02"),
		income, expense,
		topCategoriesString(byCat, 5),
		sampleRowsString(rows, 20))

	start := time.Now()
	raw, err := s.claude.AskWithSystem(ctx, systemPrompt, prompt)
	latency := int(time.Since(start).Milliseconds())

	if err != nil {
		s.record(userID, aiLogEntry{Feature: featureInsights, Status: "failed", LatencyMs: latency, ErrMsg: err.Error()})
		return dto.InsightsResponse{}, err
	}

	parsed := parseJSON(raw)
	out := dto.InsightsResponse{
		Summary:         getString(parsed, "summary"),
		TopCategories:   getStringSlice(parsed, "top_categories"),
		Recommendations: getStringSlice(parsed, "recommendations"),
		Anomalies:       getStringSlice(parsed, "anomalies"),
		HealthScore:     int(getNumber(parsed, "health_score")),
		Period:          fmt.Sprintf("%s to %s", from.Format("2006-01-02"), to.Format("2006-01-02")),
		TotalIncome:     income,
		TotalExpense:    expense,
		RawResponse:     parsed,
	}
	s.record(userID, aiLogEntry{Feature: featureInsights, Status: "success", LatencyMs: latency, Raw: parsed})
	return out, nil
}

func (s *service) SuggestBudget(ctx context.Context, userID uuid.UUID, req dto.SuggestBudgetRequest) (dto.SuggestBudgetResponse, error) {
	months := req.Months
	if months == 0 {
		months = 3
	}
	to := time.Now()
	from := to.AddDate(0, -months, 0)

	filter := transaction.ListFilter{
		UserID: userID, From: &from, To: &to, Page: 1, Limit: 1000, Type: "expense",
	}
	if req.WalletID != "" {
		if id, err := uuid.Parse(req.WalletID); err == nil {
			filter.WalletID = &id
		}
	}

	rows, _, err := s.txns.List(filter)
	if err != nil {
		return dto.SuggestBudgetResponse{}, fmt.Errorf("load transactions: %w", err)
	}

	_, _, byCat := summarise(rows)

	prompt := fmt.Sprintf(`Based on the user's last %d months of expense averages, suggest a sensible MONTHLY budget per category. Be conservative but realistic.

Per-category totals (last %d months in IDR):
%s

Return ONLY this JSON:
{
  "suggestions": [
    {"category": "...", "limit_amount": number, "period": "monthly", "reason": "..."}
  ],
  "notes": "short summary"
}`,
		months, months, topCategoriesString(byCat, 8))

	start := time.Now()
	raw, err := s.claude.AskWithSystem(ctx, systemPrompt, prompt)
	latency := int(time.Since(start).Milliseconds())

	if err != nil {
		s.record(userID, aiLogEntry{Feature: featureSuggestBudget, Status: "failed", LatencyMs: latency, ErrMsg: err.Error()})
		return dto.SuggestBudgetResponse{}, err
	}

	parsed := parseJSON(raw)
	out := dto.SuggestBudgetResponse{
		Suggestions: parseSuggestions(parsed),
		Notes:       getString(parsed, "notes"),
		RawResponse: parsed,
	}
	s.record(userID, aiLogEntry{Feature: featureSuggestBudget, Status: "success", LatencyMs: latency, Raw: parsed})
	return out, nil
}

func (s *service) Chat(ctx context.Context, userID uuid.UUID, req dto.ChatRequest) (dto.ChatResponse, error) {
	if err := s.enforceDailyQuota(ctx, userID, []string{featureCategorize, featureChat}, freeChatMonthlyLimit, "Free plan includes 20 AI chat prompts per month. Upgrade to Pro for 300 prompts/month"); err != nil {
		return dto.ChatResponse{}, err
	}
	lang := preferredLanguage(req.Message, req.Language)
	referenceNow := requestReferenceTime(req.ReferenceDate, req.Timezone)
	if isHelpMessage(req.Message) {
		reply := localizedChatHelp(lang)
		s.record(userID, aiLogEntry{Feature: featureChat, Status: "success", LatencyMs: 0, Raw: map[string]any{"message": req.Message, "reply": reply, "session_id": req.SessionID}})
		return dto.ChatResponse{Reply: reply}, nil
	}
	if isHardOffTopic(req.Message) {
		reply := localizedOffTopic(lang)
		s.record(userID, aiLogEntry{Feature: featureChat, Status: "success", LatencyMs: 0, Raw: map[string]any{"message": req.Message, "reply": reply, "refused": true, "session_id": req.SessionID}})
		return dto.ChatResponse{Reply: reply}, nil
	}
	if isTodayTransactionQuestion(req.Message) {
		now := referenceNow
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		rows, total, err := s.listTransactionsForLocalDay(userID, todayStart, 5000)
		if err != nil {
			return dto.ChatResponse{}, err
		}
		reply := buildTodayTransactionAnswer(rows, total, lang, todayStart, isTodayExpenseQuestion(req.Message))
		s.record(userID, aiLogEntry{Feature: featureChat, Status: "success", LatencyMs: 0, Raw: map[string]any{"message": req.Message, "reply": reply, "session_id": req.SessionID, "deterministic": true, "timezone": req.Timezone, "reference_date": req.ReferenceDate}})
		return dto.ChatResponse{Reply: reply}, nil
	}
	if isWalletStartingBalanceQuestion(req.Message) && s.wallets != nil {
		walletRows, walletErr := s.wallets.List(userID)
		if walletErr == nil {
			if matchedWallet := matchWalletFromMessage(req.Message, walletRows); matchedWallet != nil {
				allRows, totalRows, err := s.txns.List(transaction.ListFilter{
					UserID: userID, WalletID: &matchedWallet.ID, Page: 1, Limit: 5000,
				})
				if err != nil {
					return dto.ChatResponse{}, err
				}
				startingBalance, ok := parseStartingBalance(req.Message)
				if ok {
					reply := buildWalletStartingBalanceAnswer(*matchedWallet, allRows, totalRows, startingBalance, lang)
					s.record(userID, aiLogEntry{Feature: featureChat, Status: "success", LatencyMs: 0, Raw: map[string]any{"message": req.Message, "reply": reply, "session_id": req.SessionID, "deterministic": true, "wallet_id": matchedWallet.ID.String()}})
					return dto.ChatResponse{Reply: reply}, nil
				}
			}
		}
	}

	var promptBuilder strings.Builder

	if req.IncludeContext {
		to := referenceNow
		from := to.AddDate(0, -1, 0)
		todayStart := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, to.Location())
		monthStart := time.Date(to.Year(), to.Month(), 1, 0, 0, 0, 0, to.Location())
		recentRows, _, err := s.txns.List(transaction.ListFilter{
			UserID: userID, From: &from, To: &to, Page: 1, Limit: 100,
		})
		todayRows, _, todayErr := s.listTransactionsForLocalDay(userID, todayStart, 5000)
		monthRows, _, monthErr := s.txns.List(transaction.ListFilter{
			UserID: userID, From: &monthStart, To: &to, Page: 1, Limit: 500,
		})
		allRows, totalTransactions, allErr := s.txns.List(transaction.ListFilter{
			UserID: userID, Page: 1, Limit: 1000,
		})
		promptBuilder.WriteString(fmt.Sprintf("Exact date context:\n- today=%s\n- current_month=%s\n", todayStart.Format("2006-01-02"), monthStart.Format("2006-01")))
		if date, ok := extractSpecificDate(req.Message, referenceNow); ok {
			dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
			dateRows, dateTotal, dateErr := s.listTransactionsForLocalDay(userID, dayStart, 5000)
			if dateErr == nil {
				income, expense, byCat := summarise(dateRows)
				promptBuilder.WriteString(fmt.Sprintf("- requested_date=%s, requested_date_transactions=%d, requested_date_total_rows=%d, requested_date_income=%.2f, requested_date_expense=%.2f, requested_date_top_expense_categories=%s\n",
					dayStart.Format("2006-01-02"), len(dateRows), dateTotal, income, expense, topCategoriesString(byCat, 5)))
				if len(dateRows) > 0 {
					promptBuilder.WriteString("- requested_date_transaction_rows:\n")
					promptBuilder.WriteString(sampleRowsString(dateRows, 80))
					promptBuilder.WriteString("\n")
				}
			}
		}
		if s.wallets != nil {
			if walletRows, walletErr := s.wallets.List(userID); walletErr == nil && len(walletRows) > 0 {
				promptBuilder.WriteString("Wallet context:\n")
				promptBuilder.WriteString(walletContextString(walletRows))
				promptBuilder.WriteString("\n")
				if isListTransactionsQuestion(req.Message) {
					walletLookupText := req.Message + "\n" + chatHistoryText(req.History, 4)
					if matchedWallet := matchWalletFromMessage(walletLookupText, walletRows); matchedWallet != nil {
						walletTxRows, walletTotal, walletTxErr := s.txns.List(transaction.ListFilter{
							UserID: userID, WalletID: &matchedWallet.ID, Page: 1, Limit: 5000,
						})
						if walletTxErr == nil {
							income, expense, byCat := summarise(walletTxRows)
							promptBuilder.WriteString(fmt.Sprintf("Full wallet transaction context for %q:\n- wallet_total_transactions=%d\n- loaded_wallet_transactions=%d\n- wallet_income=%.2f, wallet_expense=%.2f\n- wallet_top_expense_categories=%s\n- wallet_transaction_rows:\n%s\n\n",
								matchedWallet.Name, walletTotal, len(walletTxRows), income, expense, topCategoriesString(byCat, 8), sampleRowsString(walletTxRows, 5000)))
						}
					} else if wantsAllTransactions(req.Message) {
						allListRows, allListTotal, allListErr := s.txns.List(transaction.ListFilter{
							UserID: userID, Page: 1, Limit: 5000,
						})
						if allListErr == nil {
							income, expense, byCat := summarise(allListRows)
							promptBuilder.WriteString(fmt.Sprintf("Full all-transaction list context:\n- total_transactions=%d\n- loaded_transactions=%d\n- income=%.2f, expense=%.2f\n- top_expense_categories=%s\n- transaction_rows:\n%s\n\n",
								allListTotal, len(allListRows), income, expense, topCategoriesString(byCat, 12), sampleRowsString(allListRows, 5000)))
						}
					}
				}
			}
		}
		if todayErr == nil {
			income, expense, byCat := summarise(todayRows)
			promptBuilder.WriteString(fmt.Sprintf("- today_transactions=%d, today_income=%.2f, today_expense=%.2f, today_top_expense_categories=%s\n", len(todayRows), income, expense, topCategoriesString(byCat, 5)))
			promptBuilder.WriteString(fmt.Sprintf("- today_by_wallet=%s\n", walletSummaryString(todayRows, 8)))
			if len(todayRows) > 0 {
				promptBuilder.WriteString("- today_transaction_rows:\n")
				promptBuilder.WriteString(sampleRowsString(todayRows, 20))
				promptBuilder.WriteString("\n")
			}
		}
		if monthErr == nil {
			income, expense, byCat := summarise(monthRows)
			promptBuilder.WriteString(fmt.Sprintf("- month_transactions=%d, month_income=%.2f, month_expense=%.2f, month_top_expense_categories=%s\n", len(monthRows), income, expense, topCategoriesString(byCat, 5)))
			promptBuilder.WriteString(fmt.Sprintf("- month_by_wallet=%s\n", walletSummaryString(monthRows, 8)))
		}
		promptBuilder.WriteString("- For questions about today/hari ini or this month/bulan ini, use these exact summaries first.\n\n")
		if allErr == nil && totalTransactions > 0 {
			income, expense, byCat := summarise(allRows)
			promptBuilder.WriteString(fmt.Sprintf("All-time transaction context:\n- total_transactions=%d\n- loaded_transactions_for_amount_summary=%d\n- income=%.2f, expense=%.2f\n- top expense categories: %s\n",
				totalTransactions, len(allRows), income, expense, topCategoriesString(byCat, 5)))
			promptBuilder.WriteString(fmt.Sprintf("- all_time_by_wallet_loaded=%s\n", walletSummaryString(allRows, 12)))
			if int64(len(allRows)) < totalTransactions {
				promptBuilder.WriteString("- Note: total_transactions is exact; income/expense/category summaries are based on loaded_transactions_for_amount_summary only.\n")
			}
			promptBuilder.WriteString("\n")
		}
		if err == nil && len(recentRows) > 0 {
			income, expense, byCat := summarise(recentRows)
			promptBuilder.WriteString(fmt.Sprintf("Context (last 30 days):\n- transaction_count=%d\n- income=%.2f, expense=%.2f\n- top expense categories: %s\n- by_wallet=%s\n- recent transactions:\n%s\n\n",
				len(recentRows), income, expense, topCategoriesString(byCat, 5), walletSummaryString(recentRows, 8), sampleRowsString(recentRows, 20)))
		}
	}

	if n := len(req.History); n > 0 {
		start := 0
		if n > 6 {
			start = n - 6
		}
		promptBuilder.WriteString("Previous conversation (oldest first):\n")
		for _, t := range req.History[start:] {
			role := strings.ToLower(strings.TrimSpace(t.Role))
			if role != "user" && role != "assistant" {
				role = "user"
			}
			content := strings.TrimSpace(t.Content)
			if content == "" {
				continue
			}
			if len(content) > 1200 {
				content = content[:1200] + "…"
			}
			promptBuilder.WriteString(fmt.Sprintf("[%s] %s\n", role, content))
		}
		promptBuilder.WriteString("\n")
	}

	promptBuilder.WriteString("Required response language: ")
	if lang == "en" {
		promptBuilder.WriteString("English. If the user's message is English, answer in English even if transaction names are Indonesian.\n\n")
	} else {
		promptBuilder.WriteString("Bahasa Indonesia.\n\n")
	}

	promptBuilder.WriteString("User question: ")
	promptBuilder.WriteString(req.Message)

	start := time.Now()
	reply, err := s.claude.AskWithSystem(ctx, chatSystemPrompt, promptBuilder.String())
	latency := int(time.Since(start).Milliseconds())

	if err != nil {
		s.record(userID, aiLogEntry{Feature: featureChat, Status: "failed", LatencyMs: latency, ErrMsg: err.Error(), Raw: map[string]any{"message": req.Message, "session_id": req.SessionID}})
		return dto.ChatResponse{}, err
	}
	reply = sanitiseChatReply(reply)
	reply = trimOffTopicPivot(reply)
	s.record(userID, aiLogEntry{Feature: featureChat, Status: "success", LatencyMs: latency, Raw: map[string]any{"message": req.Message, "reply": reply, "session_id": req.SessionID}})
	return dto.ChatResponse{Reply: reply}, nil
}

func isHelpMessage(msg string) bool {
	m := strings.ToLower(strings.TrimSpace(msg))
	return m == "help" || m == "bantuan" || m == "panduan"
}

func preferredLanguage(message, requested string) string {
	requested = strings.ToLower(strings.TrimSpace(requested))
	if requested == "en" || requested == "id" {
		return requested
	}
	m := strings.ToLower(strings.TrimSpace(message))
	englishHints := []string{"what", "which", "how", "why", "total", "spending", "income", "expense", "budget", "wallet", "category", "compare", "summarize", "show", "list", "this month", "last month"}
	indonesianHints := []string{"apa", "berapa", "mana", "bagaimana", "pengeluaran", "pemasukan", "dompet", "kategori", "bulan ini", "bulan lalu", "bandingkan", "ringkas", "tampilkan", "berikan", "semua", "transaksi", "transaksinya", "bukan", "cuma", "kemarin", "hari ini", "aja", "pakai", "pake", "menggunakan", "saldo", "sisa", "nominal", "jumlah", "rincian"}
	englishScore := 0
	indonesianScore := 0
	for _, hint := range englishHints {
		if strings.Contains(m, hint) {
			englishScore++
		}
	}
	for _, hint := range indonesianHints {
		if strings.Contains(m, hint) {
			indonesianScore++
		}
	}
	if englishScore > indonesianScore {
		return "en"
	}
	if indonesianScore > englishScore {
		return "id"
	}
	return "en"
}

func (s *service) listTransactionsForLocalDay(userID uuid.UUID, day time.Time, limit int) ([]domain.Transaction, int64, error) {
	if limit <= 0 {
		limit = 5000
	}
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	// Query a wider UTC-safe window, then enforce the user's calendar date in
	// application code. This handles local-midnight values stored on the prior
	// UTC date without allowing tomorrow's transactions into today's summary.
	from := start.UTC().Add(-24 * time.Hour)
	to := start.AddDate(0, 0, 1).UTC().Add(24*time.Hour - time.Nanosecond)
	rows, _, err := s.txns.List(transaction.ListFilter{
		UserID: userID,
		From:   &from,
		To:     &to,
		Page:   1,
		Limit:  limit,
	})
	if err != nil {
		return nil, 0, err
	}
	filtered := filterTransactionsForLocalDay(rows, start)
	return filtered, int64(len(filtered)), nil
}

func filterTransactionsForLocalDay(rows []domain.Transaction, day time.Time) []domain.Transaction {
	filtered := make([]domain.Transaction, 0, len(rows))
	for _, row := range rows {
		localDate := row.TransactionDate.In(day.Location())
		if localDate.Year() == day.Year() &&
			localDate.Month() == day.Month() &&
			localDate.Day() == day.Day() {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func requestReferenceTime(referenceDate, timezone string) time.Time {
	loc := defaultFinanceLocation()
	if strings.TrimSpace(timezone) != "" {
		if loaded, err := time.LoadLocation(strings.TrimSpace(timezone)); err == nil {
			loc = loaded
		}
	}
	value := strings.TrimSpace(referenceDate)
	if value == "" {
		return time.Now().In(loc)
	}
	if parsed, err := time.ParseInLocation("2006-01-02", value, loc); err == nil {
		now := time.Now().In(loc)
		return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), now.Hour(), now.Minute(), now.Second(), now.Nanosecond(), loc)
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.In(loc)
	}
	return time.Now().In(loc)
}

func defaultFinanceLocation() *time.Location {
	if loc, err := time.LoadLocation("Asia/Jakarta"); err == nil {
		return loc
	}
	return time.FixedZone("WIB", 7*60*60)
}

var specificDateRE = regexp.MustCompile(`(?i)(?:tanggal|tgl|date|on)\s+(\d{1,2})(?:[-/\s]+([a-zA-Z]+|\d{1,2}))?(?:[-/\s]+(\d{2,4}))?`)

func extractSpecificDate(message string, reference time.Time) (time.Time, bool) {
	text := strings.ToLower(strings.TrimSpace(message))
	match := specificDateRE.FindStringSubmatch(text)
	if len(match) < 2 {
		return time.Time{}, false
	}
	day, err := strconv.Atoi(match[1])
	if err != nil || day < 1 || day > 31 {
		return time.Time{}, false
	}
	month := int(reference.Month())
	year := reference.Year()
	if len(match) > 2 && strings.TrimSpace(match[2]) != "" {
		if parsedMonth, ok := parseMonthToken(match[2]); ok {
			month = parsedMonth
		}
	}
	if len(match) > 3 && strings.TrimSpace(match[3]) != "" {
		if parsedYear, yerr := strconv.Atoi(match[3]); yerr == nil {
			if parsedYear < 100 {
				parsedYear += 2000
			}
			year = parsedYear
		}
	}
	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, reference.Location())
	if date.Month() != time.Month(month) || date.Day() != day {
		return time.Time{}, false
	}
	return date, true
}

func parseMonthToken(token string) (int, bool) {
	t := strings.ToLower(strings.Trim(token, " .,/"))
	if t == "" {
		return 0, false
	}
	if n, err := strconv.Atoi(t); err == nil && n >= 1 && n <= 12 {
		return n, true
	}
	months := map[string]int{
		"jan": 1, "januari": 1, "january": 1,
		"feb": 2, "februari": 2, "february": 2,
		"mar": 3, "maret": 3, "march": 3,
		"apr": 4, "april": 4,
		"mei": 5, "may": 5,
		"jun": 6, "juni": 6, "june": 6,
		"jul": 7, "juli": 7, "july": 7,
		"agu": 8, "agustus": 8, "aug": 8, "august": 8,
		"sep": 9, "sept": 9, "september": 9,
		"okt": 10, "oktober": 10, "oct": 10, "october": 10,
		"nov": 11, "november": 11,
		"des": 12, "desember": 12, "dec": 12, "december": 12,
	}
	value, ok := months[t]
	return value, ok
}

func localizedChatHelp(lang string) string {
	if lang == "en" {
		return "Chatbot guide:\n1. Ask for spending summaries, largest categories, or month comparisons.\n2. Request a specific format such as a short list or table.\n3. Answers use the transaction data available in your account.\n4. To record a transaction, use NLP mode and write something like \"coffee 25k\"."
	}
	return "Panduan Chatbot:\n1. Tanyakan ringkasan pengeluaran, kategori terbesar, atau perbandingan bulan ini.\n2. Minta format spesifik seperti list singkat atau tabel.\n3. Jawaban memakai data transaksi yang tersedia di akunmu.\n4. Untuk mencatat transaksi, gunakan mode NLP dan tulis contoh seperti \"beli kopi 25rb\"."
}

func localizedOffTopic(lang string) string {
	if lang == "en" {
		return "Sorry, I can only help with personal finance and SAKU features. You can ask about spending, budgets, or your transactions."
	}
	return "Maaf, saya hanya bisa membantu seputar keuangan pribadi dan fitur SAKU. Boleh tanya soal pengeluaran, anggaran, atau transaksi kamu."
}

func isTodayExpenseQuestion(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	hasExpenseIntent := strings.Contains(text, "pengeluaran") || strings.Contains(text, "expense") || strings.Contains(text, "spending") || strings.Contains(text, "keluar")
	hasTodayIntent := strings.Contains(text, "hari ini") || strings.Contains(text, "today")
	return hasExpenseIntent && hasTodayIntent
}

func isTodayTransactionQuestion(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	hasTodayIntent := strings.Contains(text, "hari ini") || strings.Contains(text, "today")
	hasTransactionIntent := strings.Contains(text, "transaksi") ||
		strings.Contains(text, "transaction") ||
		strings.Contains(text, "pengeluaran") ||
		strings.Contains(text, "expense") ||
		strings.Contains(text, "spending") ||
		strings.Contains(text, "pemasukan") ||
		strings.Contains(text, "income")
	return hasTodayIntent && hasTransactionIntent
}

func buildTodayTransactionAnswer(rows []domain.Transaction, totalRows int64, lang string, day time.Time, expenseOnly bool) string {
	expenses := make([]domain.Transaction, 0, len(rows))
	totalExpense := 0.0
	totalIncome := 0.0
	byCat := map[string]float64{}
	for _, row := range rows {
		if row.Type == "expense" {
			expenses = append(expenses, row)
			totalExpense += row.Amount
			byCat[transactionCategoryName(row)] += row.Amount
		} else if row.Type == "income" {
			totalIncome += row.Amount
		}
	}
	dateLabel := formatChatDay(day, lang)
	if totalRows == 0 {
		if lang == "en" {
			return fmt.Sprintf("No transactions are recorded for today (%s). Income: %s, spending: %s, net cashflow: %s.", dateLabel, formatRupiah(0), formatRupiah(0), formatRupiah(0))
		}
		return fmt.Sprintf("Belum ada transaksi yang tercatat hari ini (%s). Pemasukan: %s, pengeluaran: %s, dan net cashflow: %s.", dateLabel, formatRupiah(0), formatRupiah(0), formatRupiah(0))
	}
	if expenseOnly {
		if len(expenses) == 0 {
			if lang == "en" {
				return fmt.Sprintf("There are %d transaction(s) today (%s), but none are expenses. Total income is %s.", totalRows, dateLabel, formatRupiah(totalIncome))
			}
			return fmt.Sprintf("Ada %d transaksi hari ini (%s), tetapi tidak ada pengeluaran. Total pemasukan hari ini %s.", totalRows, dateLabel, formatRupiah(totalIncome))
		}
		topCategory := topCategoryName(byCat)
		if lang == "en" {
			return fmt.Sprintf("Today's spending (%s) is %s from %d expense transaction(s). Largest category: %s.", dateLabel, formatRupiah(totalExpense), len(expenses), topCategory)
		}
		return fmt.Sprintf("Total pengeluaran hari ini (%s) adalah %s dari %d transaksi pengeluaran. Kategori terbesar: %s.", dateLabel, formatRupiah(totalExpense), len(expenses), topCategory)
	}

	mainRows := make([]string, 0, minInt(len(rows), 3))
	for i := 0; i < len(rows) && i < 3; i++ {
		row := rows[i]
		name := strings.TrimSpace(row.MerchantName)
		if name == "" {
			name = strings.TrimSpace(row.Description)
		}
		if name == "" {
			name = "Transaksi"
		}
		mainRows = append(mainRows, fmt.Sprintf("%s %s", name, formatRupiah(row.Amount)))
	}
	if lang == "en" {
		return fmt.Sprintf("You have %d transaction(s) today (%s). Income: %s, spending: %s, net cashflow: %s. Latest: %s.", totalRows, dateLabel, formatRupiah(totalIncome), formatRupiah(totalExpense), formatRupiah(totalIncome-totalExpense), strings.Join(mainRows, ", "))
	}
	return fmt.Sprintf("Hari ini (%s) ada %d transaksi. Pemasukan: %s, pengeluaran: %s, dan net cashflow: %s. Transaksi terbaru: %s.", dateLabel, totalRows, formatRupiah(totalIncome), formatRupiah(totalExpense), formatRupiah(totalIncome-totalExpense), strings.Join(mainRows, ", "))
}

func formatChatDay(day time.Time, lang string) string {
	if lang != "id" {
		return day.Format("02 January 2006")
	}
	months := [...]string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}
	return fmt.Sprintf("%d %s %d", day.Day(), months[int(day.Month())-1], day.Year())
}

func isWalletStartingBalanceQuestion(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	hasWalletIntent := strings.Contains(text, "bank") || strings.Contains(text, "dompet") || strings.Contains(text, "wallet") || strings.Contains(text, "rekening") || strings.Contains(text, "cash") || strings.Contains(text, "tunai")
	hasStart := strings.Contains(text, "saldo awal") || strings.Contains(text, "awalnya") || strings.Contains(text, "modal awal") || strings.Contains(text, "starting balance") || strings.Contains(text, "initial balance")
	hasResult := strings.Contains(text, "sisa") || strings.Contains(text, "saldo") || strings.Contains(text, "remaining") || strings.Contains(text, "balance")
	return hasWalletIntent && hasStart && hasResult
}

func isListTransactionsQuestion(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	hasListIntent := strings.Contains(text, "list") || strings.Contains(text, "daftar") || strings.Contains(text, "tampilkan") || strings.Contains(text, "show") || strings.Contains(text, "rincian") || strings.Contains(text, "detail")
	hasTxIntent := strings.Contains(text, "transaksi") || strings.Contains(text, "transactions") || strings.Contains(text, "pengeluaran") || strings.Contains(text, "expenses")
	hasFollowUpAll := strings.Contains(text, "semua") || strings.Contains(text, "all") || strings.Contains(text, "bukan cuma") || strings.Contains(text, "bukan hanya")
	return (hasListIntent && hasTxIntent) || (hasFollowUpAll && hasTxIntent)
}

func wantsAllTransactions(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(text, "semua") || strings.Contains(text, "all") || strings.Contains(text, "bukan cuma") || strings.Contains(text, "bukan hanya")
}

func chatHistoryText(history []dto.ChatTurn, maxTurns int) string {
	if len(history) == 0 || maxTurns <= 0 {
		return ""
	}
	start := len(history) - maxTurns
	if start < 0 {
		start = 0
	}
	parts := make([]string, 0, len(history)-start)
	for _, turn := range history[start:] {
		content := strings.TrimSpace(turn.Content)
		if content != "" {
			parts = append(parts, content)
		}
	}
	return strings.Join(parts, "\n")
}

func matchWalletFromMessage(message string, wallets []domain.Wallet) *domain.Wallet {
	text := normalizeWalletText(message)
	if text == "" {
		return nil
	}
	textTokens := tokenSet(text)
	var best *domain.Wallet
	bestScore := 0
	for i := range wallets {
		w := &wallets[i]
		name := normalizeWalletText(w.Name)
		if name == "" {
			continue
		}
		score := 0
		if strings.Contains(text, name) {
			score += 100
		}
		shortName := strings.TrimSpace(walletNoiseRE.ReplaceAllString(name, " "))
		shortName = strings.Join(strings.Fields(shortName), " ")
		if shortName != "" && strings.Contains(text, shortName) {
			score += 80
		}
		for _, token := range strings.Fields(shortName) {
			if len(token) >= 2 && textTokens[token] {
				score += 14
			}
		}
		if w.Type == domain.WalletTypeCash && (textTokens["cash"] || textTokens["tunai"] || textTokens["kas"]) {
			score += 90
		}
		if score > bestScore {
			bestScore = score
			best = w
		}
	}
	if bestScore < 24 {
		return nil
	}
	return best
}

var walletNoiseRE = regexp.MustCompile(`\b(bank|rekening|akun|account|wallet|dompet|dari|pake|pakai|menggunakan|gunakan|via)\b`)

func normalizeWalletText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "&", " ")
	parts := regexp.MustCompile(`[^a-z0-9]+`).Split(value, -1)
	return strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
}

func tokenSet(value string) map[string]bool {
	out := map[string]bool{}
	for _, token := range strings.Fields(value) {
		if len(token) >= 2 {
			out[token] = true
		}
	}
	return out
}

var startingBalanceRE = regexp.MustCompile(`(?i)(?:saldo awal(?:nya)?|awalnya|modal awal|starting balance|initial balance)[^\d]*(\d+(?:[.,]\d+)?)\s*(juta|jt|ribu|rb|k|m)?`)

func parseStartingBalance(message string) (float64, bool) {
	match := startingBalanceRE.FindStringSubmatch(strings.ToLower(message))
	if len(match) < 2 {
		return 0, false
	}
	raw := strings.ReplaceAll(match[1], ".", "")
	if strings.Contains(match[1], ",") {
		raw = strings.ReplaceAll(match[1], ".", "")
		raw = strings.ReplaceAll(raw, ",", ".")
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	unit := ""
	if len(match) > 2 {
		unit = strings.ToLower(match[2])
	}
	switch unit {
	case "juta", "jt", "m":
		value *= 1_000_000
	case "ribu", "rb", "k":
		value *= 1_000
	default:
		if value > 0 && value < 100 {
			value *= 1_000_000
		}
	}
	return value, true
}

func buildWalletStartingBalanceAnswer(wallet domain.Wallet, rows []domain.Transaction, totalRows int64, startingBalance float64, lang string) string {
	income := 0.0
	expense := 0.0
	for _, row := range rows {
		switch row.Type {
		case domain.TxnTypeIncome:
			income += row.Amount
		case domain.TxnTypeExpense:
			expense += row.Amount
		}
	}
	remaining := startingBalance + income - expense
	partial := ""
	if int64(len(rows)) < totalRows {
		if lang == "en" {
			partial = fmt.Sprintf(" This uses %d loaded transactions out of %d total for that wallet.", len(rows), totalRows)
		} else {
			partial = fmt.Sprintf(" Ini memakai %d transaksi yang termuat dari total %d transaksi di dompet itu.", len(rows), totalRows)
		}
	}
	if lang == "en" {
		return fmt.Sprintf("For %s, with starting balance %s, income %s, and expenses %s, the remaining balance should be %s. Formula: starting balance + income - expenses.%s", wallet.Name, formatRupiah(startingBalance), formatRupiah(income), formatRupiah(expense), formatRupiah(remaining), partial)
	}
	return fmt.Sprintf("Untuk %s, kalau saldo awalnya %s, pemasukan %s, dan pengeluaran %s, maka sisa saldo seharusnya %s. Rumusnya: saldo awal + pemasukan - pengeluaran.%s", wallet.Name, formatRupiah(startingBalance), formatRupiah(income), formatRupiah(expense), formatRupiah(remaining), partial)
}

func transactionCategoryName(row domain.Transaction) string {
	if row.Category != nil && strings.TrimSpace(row.Category.Name) != "" {
		return row.Category.Name
	}
	return "Tanpa Kategori"
}

func topCategoryName(byCat map[string]float64) string {
	bestName := "Tanpa Kategori"
	bestAmount := -1.0
	for name, amount := range byCat {
		if amount > bestAmount {
			bestName = name
			bestAmount = amount
		}
	}
	return bestName
}

func formatRupiah(amount float64) string {
	n := int64(amount + 0.5)
	raw := fmt.Sprintf("%d", n)
	if len(raw) <= 3 {
		return "Rp " + raw
	}
	parts := []string{}
	for len(raw) > 3 {
		parts = append([]string{raw[len(raw)-3:]}, parts...)
		raw = raw[:len(raw)-3]
	}
	parts = append([]string{raw}, parts...)
	return "Rp " + strings.Join(parts, ".")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func isHardOffTopic(msg string) bool {
	m := strings.ToLower(strings.TrimSpace(msg))
	if m == "" {
		return false
	}
	for _, kw := range financeKeywords {
		if strings.Contains(m, kw) {
			return false
		}
	}
	for _, kw := range hardOffTopicKeywords {
		if strings.Contains(m, kw) {
			return true
		}
	}
	return false
}

var financeKeywords = []string{
	"saku", "uang", "rupiah", "rp", "idr", "keuangan", "finance", "financial",
	"transaksi", "transaction", "pengeluaran", "expense", "pemasukan", "income",
	"gaji", "salary", "bonus", "freelance", "tagihan", "bill", "cicilan", "hutang", "utang",
	"tabungan", "saving", "invest", "saham", "reksa", "crypto", "deposito",
	"budget", "anggaran", "limit", "target",
	"kategori", "category", "dompet", "wallet", "rekening", "bank", "e-wallet", "ewallet",
	"belanja", "beli", "bayar", "transfer", "top up", "topup", "setor",
	"struk", "receipt", "scan", "ocr", "merchant", "toko", "warung",
	"bulan ini", "minggu ini", "hari ini", "laporan", "report", "summary",
	"dashboard", "grafik", "chart", "pie", "trend",
	"halo", "hai", "hello", "hey", "selamat", "makasih", "terima kasih", "thanks",
	"tolong", "bantu", "help",
}

var offTopicKeywords = []string{
	"resep", "recipe", "masak", "cooking", "kopi", "coffee", "teh ", "tea ",
	"nasi goreng", "mie ", "makanan enak", "how to cook", "how to make", "cara membuat",
	"cuaca", "weather", "hujan", "forecast",
	"berita", "news", "politik", "politic", "presiden", "election", "pemilu",
	"lagu", "lirik", "lyric", "song", "musik", "film", "movie", "anime",
	"game", "gaming", "mobile legend", "valorant", "genshin",
	"jokes", "joke", "lelucon", "humor", "funny", "lucu",
	"code", "coding", "program", "javascript", "python", "golang", "react",
	"obat", "penyakit", "diagnosa", "medical", "dokter",
	"hukum", "legal", "undang-undang", "pasal",
	"sejarah", "history", "matematika", "math", "fisika", "kimia",
	"translate", "terjemah", "english to", "bahasa inggris",
	"siapa kamu", "siapa anda", "who are you", "chatgpt", "openai", "claude", "gemini",
}

var hardOffTopicKeywords = []string{
	"resep", "recipe", "nasi goreng", "cara memasak", "cara membuat kue",
	"how to cook", "how to bake",
	"cuaca", "weather forecast", "prakiraan hujan",
	"lirik lagu", "lyric song", "chord lagu",
	"jadwal pertandingan", "hasil pertandingan", "mobile legend", "valorant", "genshin impact",
	"obat untuk", "diagnosa penyakit", "resep dokter",
	"undang-undang pasal", "pasal hukum",
	"matematika kelas", "pr matematika", "pr fisika", "pr kimia",
	"terjemahkan ke bahasa inggris", "translate to english",
	"siapa kamu sebenarnya", "chatgpt", "openai api", "gemini api",
}

func trimOffTopicPivot(reply string) string {
	lower := strings.ToLower(reply)
	markers := []string{
		"\nngomong-ngomong", "\nngomong,", "\nbtw", "\nby the way",
		"\noh ya,", "\nselain itu", "\nsebagai info", "\nsekedar info",
		"\nfun fact",
		". ngomong-ngomong", ". ngomong,", ". btw", ". by the way",
		". oh ya,", ". selain itu", ". sebagai info",
	}
	cut := -1
	for _, mk := range markers {
		if idx := strings.Index(lower, mk); idx >= 0 {
			if cut == -1 || idx < cut {
				cut = idx
			}
		}
	}
	if cut > 0 {
		if reply[cut] == '.' {
			return strings.TrimSpace(reply[:cut+1])
		}
		return strings.TrimSpace(reply[:cut])
	}
	return reply
}

func sanitiseChatReply(s string) string {
	t := strings.TrimSpace(s)
	if strings.HasPrefix(t, "```") {
		if i := strings.Index(t, "\n"); i >= 0 {
			t = t[i+1:]
		}
		if j := strings.LastIndex(t, "```"); j >= 0 {
			t = t[:j]
		}
		t = strings.TrimSpace(t)
	}
	if strings.HasPrefix(t, "{") && strings.HasSuffix(t, "}") {
		var m map[string]any
		if err := json.Unmarshal([]byte(t), &m); err == nil {
			for _, key := range []string{"message", "reply", "answer", "response", "text"} {
				if v, ok := m[key].(string); ok && strings.TrimSpace(v) != "" {
					return strings.TrimSpace(v)
				}
			}
		}
		return "Maaf, saya belum bisa memberikan jawaban yang tepat. Coba tanyakan ulang dengan kalimat yang berbeda."
	}
	t = strings.ReplaceAll(t, "**", "")
	t = strings.ReplaceAll(t, "__", "")
	return strings.TrimSpace(t)
}

func (s *service) resolveCategories(userID uuid.UUID, override []string) []string {
	if len(override) > 0 {
		return override
	}
	cats, err := s.cats.List(userID, "")
	if err != nil || len(cats) == 0 {
		return []string{"Food & Beverage", "Transportation", "Shopping", "Bills", "Entertainment", "Transfer", "Salary", "Investment", "Other"}
	}
	out := make([]string, 0, len(cats))
	for _, c := range cats {
		out = append(out, c.Name)
	}
	return out
}

func normalizeCategoryChoice(allowed []string, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallbackCategory(allowed)
	}
	rawKey := categoryKey(raw)
	for _, name := range allowed {
		if categoryKey(name) == rawKey {
			return name
		}
	}
	aliases := []struct {
		allowedKeys []string
		inputKeys   []string
	}{
		{[]string{"tagihan", "bills", "bill"}, []string{"bill", "bills", "billing", "utility", "utilities", "subscription", "subscriptions", "hosting", "hostinger", "domain", "server", "vps", "software", "internet", "wifi", "listrik", "pln", "pulsa"}},
		{[]string{"makananminuman", "foodbeverage", "food"}, []string{"food", "foodbeverage", "restaurant", "restoran", "cafe", "warung", "makan", "minum"}},
		{[]string{"transportasi", "transportation", "transport"}, []string{"transport", "transportation", "travel", "grab", "gojek", "taxi", "bensin", "parkir"}},
		{[]string{"belanja", "shopping", "shop"}, []string{"shopping", "shop", "store", "marketplace", "tokopedia", "shopee", "mall"}},
		{[]string{"hiburan", "entertainment"}, []string{"entertainment", "netflix", "spotify", "movie", "bioskop"}},
		{[]string{"gaji", "salary"}, []string{"salary", "payroll", "income", "wage"}},
		{[]string{"investasi", "investment"}, []string{"investment", "investasi"}},
		{[]string{"lainnya", "other", "misc"}, []string{"other", "misc", "miscellaneous", "uncategorized", "unknown"}},
	}
	for _, alias := range aliases {
		if containsString(alias.allowedKeys, rawKey) || containsString(alias.inputKeys, rawKey) {
			if match := findAllowedByAnyKey(allowed, alias.allowedKeys); match != "" {
				return match
			}
		}
	}
	return fallbackCategory(allowed)
}

func fallbackCategory(allowed []string) string {
	for _, key := range []string{"lainnya", "other"} {
		if match := findAllowedByKey(allowed, key); match != "" {
			return match
		}
	}
	if len(allowed) > 0 {
		return allowed[0]
	}
	return "Lainnya"
}

func findAllowedByKey(allowed []string, key string) string {
	for _, name := range allowed {
		if categoryKey(name) == key {
			return name
		}
	}
	return ""
}

func findAllowedByAnyKey(allowed []string, keys []string) string {
	for _, key := range keys {
		if match := findAllowedByKey(allowed, key); match != "" {
			return match
		}
	}
	return ""
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func categoryKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("&", "", "and", "", "/", "", "-", "", "_", "", " ", "")
	return replacer.Replace(value)
}

type aiLogEntry struct {
	Feature    string
	Status     string
	Confidence *float64
	LatencyMs  int
	Merchant   string
	Category   string
	Amount     *float64
	ErrMsg     string
	Raw        map[string]any
}

func (s *service) record(userID uuid.UUID, e aiLogEntry) uuid.UUID {
	if s.logs == nil {
		return uuid.Nil
	}
	rawStr := ""
	if e.Raw != nil {
		if b, err := json.Marshal(e.Raw); err == nil {
			rawStr = string(b)
		}
	}
	req := ailog.RecordInput{
		Feature:           e.Feature,
		Status:            e.Status,
		ConfidenceScore:   e.Confidence,
		ModelVersion:      s.model,
		LatencyMs:         e.LatencyMs,
		ExtractedAmount:   e.Amount,
		ExtractedMerchant: e.Merchant,
		ExtractedCategory: e.Category,
		ErrorMessage:      truncate(e.ErrMsg, 1900),
		RawResponse:       rawStr,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	id, err := s.logs.Record(ctx, userID, req)
	if err != nil {
		slog.Warn("ai: persist log failed",
			"feature", e.Feature, "user_id", userID, "error", err)
		return uuid.Nil
	}
	return id
}

func (s *service) logLowConfidence(feature string, confidence float64, parsed map[string]any) {
	if confidence > 0 && confidence < confidenceReviewThreshold {
		slog.Warn("ai: low confidence response",
			"feature", feature,
			"confidence", confidence,
			"threshold", confidenceReviewThreshold,
			"merchant", getString(parsed, "merchant_name"))
	}
}

func parseRange(fromStr, toStr string, defaultDays int) (time.Time, time.Time) {
	to := time.Now()
	from := to.AddDate(0, 0, -defaultDays)
	if t, err := time.Parse("2006-01-02", fromStr); err == nil {
		from = t
	}
	if t, err := time.Parse("2006-01-02", toStr); err == nil {
		to = t
	}
	return from, to
}

func summarise(rows []domain.Transaction) (income, expense float64, byCategory map[string]float64) {
	byCategory = map[string]float64{}
	for _, t := range rows {
		switch t.Type {
		case "income":
			income += t.Amount
		case "expense":
			expense += t.Amount
		}
		if t.Type == "expense" {
			byCategory[transactionCategoryName(t)] += t.Amount
		}
	}
	return
}

func topCategoriesString(byCat map[string]float64, n int) string {
	type kv struct {
		k string
		v float64
	}
	list := make([]kv, 0, len(byCat))
	for k, v := range byCat {
		list = append(list, kv{k, v})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].v > list[j].v })
	if n > len(list) {
		n = len(list)
	}
	parts := make([]string, 0, n)
	for i := 0; i < n; i++ {
		parts = append(parts, fmt.Sprintf("%s=%.0f", list[i].k, list[i].v))
	}
	return strings.Join(parts, ", ")
}

func walletContextString(wallets []domain.Wallet) string {
	lines := make([]string, 0, len(wallets))
	for _, w := range wallets {
		defaultLabel := ""
		if w.IsDefault {
			defaultLabel = ", default=true"
		}
		lines = append(lines, fmt.Sprintf("- wallet=%q, type=%s, current_balance=%.0f%s", w.Name, w.Type, w.BalanceCached, defaultLabel))
	}
	return strings.Join(lines, "\n")
}

func walletSummaryString(rows []domain.Transaction, n int) string {
	type summary struct {
		name    string
		income  float64
		expense float64
		count   int
	}
	byWallet := map[string]*summary{}
	for _, row := range rows {
		name := "Unknown wallet"
		if row.Wallet != nil && strings.TrimSpace(row.Wallet.Name) != "" {
			name = row.Wallet.Name
		}
		current := byWallet[name]
		if current == nil {
			current = &summary{name: name}
			byWallet[name] = current
		}
		current.count++
		switch row.Type {
		case domain.TxnTypeIncome:
			current.income += row.Amount
		case domain.TxnTypeExpense:
			current.expense += row.Amount
		}
	}
	list := make([]*summary, 0, len(byWallet))
	for _, item := range byWallet {
		list = append(list, item)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].income+list[i].expense > list[j].income+list[j].expense
	})
	if n > len(list) {
		n = len(list)
	}
	parts := make([]string, 0, n)
	for i := 0; i < n; i++ {
		item := list[i]
		parts = append(parts, fmt.Sprintf("%s: income=%.0f, expense=%.0f, net=%.0f, count=%d", item.name, item.income, item.expense, item.income-item.expense, item.count))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, "; ")
}

func sampleRowsString(rows []domain.Transaction, n int) string {
	if n > len(rows) {
		n = len(rows)
	}
	lines := make([]string, 0, n)
	for i := 0; i < n; i++ {
		t := rows[i]
		merchant := t.MerchantName
		if merchant == "" {
			merchant = t.Description
		}
		walletName := "Unknown wallet"
		if t.Wallet != nil && strings.TrimSpace(t.Wallet.Name) != "" {
			walletName = t.Wallet.Name
		}
		lines = append(lines, fmt.Sprintf("- %s [%s] wallet=%q category=%s amount=%.0f merchant_or_desc=%q",
			t.TransactionDate.Format("2006-01-02"), t.Type, walletName, transactionCategoryName(t), t.Amount, merchant))
	}
	return strings.Join(lines, "\n")
}

var jsonObjectRE = regexp.MustCompile(`(?s)\{.*\}`)

func parseJSON(raw string) map[string]any {
	s := stripFences(raw)
	var out map[string]any
	if err := json.Unmarshal([]byte(s), &out); err == nil {
		return out
	}
	if match := jsonObjectRE.FindString(s); match != "" {
		if err := json.Unmarshal([]byte(match), &out); err == nil {
			return out
		}
	}
	return nil
}

func stripFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if i := strings.Index(s, "\n"); i != -1 {
		s = s[i+1:]
	}
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func getString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v := getString(m, k); v != "" {
			return v
		}
	}
	return ""
}

func getNumber(m map[string]any, key string) float64 {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		f, _ := v.Float64()
		return f
	case string:
		return parseLooseNumber(v)
	}
	return 0
}

func parseLooseNumber(s string) float64 {
	clean := strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || r == '.' || r == ',' || r == '-' {
			return r
		}
		return -1
	}, s)
	if clean == "" {
		return 0
	}

	hasDot := strings.Contains(clean, ".")
	hasComma := strings.Contains(clean, ",")

	switch {
	case hasDot && hasComma:
		if strings.LastIndex(clean, ",") > strings.LastIndex(clean, ".") {
			clean = strings.ReplaceAll(clean, ".", "")
			clean = strings.Replace(clean, ",", ".", 1)
		} else {
			clean = strings.ReplaceAll(clean, ",", "")
		}
	case hasComma:
		if strings.Count(clean, ",") > 1 {
			clean = strings.ReplaceAll(clean, ",", "")
		} else {
			parts := strings.Split(clean, ",")
			if len(parts[1]) == 3 {
				clean = strings.ReplaceAll(clean, ",", "")
			} else {
				clean = strings.Replace(clean, ",", ".", 1)
			}
		}
	case hasDot:
		if strings.Count(clean, ".") > 1 {
			clean = strings.ReplaceAll(clean, ".", "")
		} else {
			parts := strings.Split(clean, ".")
			if len(parts[1]) == 3 {
				clean = strings.ReplaceAll(clean, ".", "")
			}
		}
	}

	var f float64
	if _, err := fmt.Sscanf(clean, "%f", &f); err != nil {
		return 0
	}
	return f
}

func getStringSlice(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}
	v, ok := m[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(v))
	for _, item := range v {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func parseSuggestions(m map[string]any) []dto.BudgetSuggestion {
	if m == nil {
		return nil
	}
	v, ok := m["suggestions"].([]any)
	if !ok {
		return nil
	}
	out := make([]dto.BudgetSuggestion, 0, len(v))
	for _, item := range v {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, dto.BudgetSuggestion{
			Category:    getString(obj, "category"),
			LimitAmount: getNumber(obj, "limit_amount"),
			Period:      defaultIfEmpty(getString(obj, "period"), "monthly"),
			Reason:      getString(obj, "reason"),
		})
	}
	return out
}

func normalizeType(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "income", "in", "credit", "pemasukan":
		return "income"
	case "expense", "out", "debit", "pengeluaran":
		return "expense"
	}
	return "expense"
}

func needsReview(confidence float64, parseFailed bool) bool {
	if parseFailed {
		return true
	}
	return confidence > 0 && confidence < confidenceReviewThreshold
}

func defaultIfEmpty(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func amountPtr(v float64) *float64 {
	if v == 0 {
		return nil
	}
	return &v
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

var _ = errors.New
