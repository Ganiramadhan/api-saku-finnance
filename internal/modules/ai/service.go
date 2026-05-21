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
	"strings"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/ganiramadhan/starter-go/internal/modules/ailog"
	"github.com/ganiramadhan/starter-go/internal/modules/category"
	"github.com/ganiramadhan/starter-go/internal/modules/transaction"
	aiplatform "github.com/ganiramadhan/starter-go/internal/platform/ai"
	"github.com/ganiramadhan/starter-go/internal/platform/storage"
	"github.com/google/uuid"
)

const (
	confidenceReviewThreshold = 0.70

	featureCategorize    = "categorize"
	featureScanReceipt   = "scan_receipt"
	featureInsights      = "insights"
	featureSuggestBudget = "suggest_budget"
	featureChat          = "chat"
)

type Service interface {
	Categorize(ctx context.Context, userID uuid.UUID, req dto.CategorizeRequest) (dto.CategorizeResponse, error)
	ScanReceipt(ctx context.Context, userID uuid.UUID, req dto.ScanReceiptRequest) (dto.ScanReceiptResponse, error)
	Insights(ctx context.Context, userID uuid.UUID, req dto.InsightsRequest) (dto.InsightsResponse, error)
	SuggestBudget(ctx context.Context, userID uuid.UUID, req dto.SuggestBudgetRequest) (dto.SuggestBudgetResponse, error)
	Chat(ctx context.Context, userID uuid.UUID, req dto.ChatRequest) (dto.ChatResponse, error)
}

type service struct {
	claude  *aiplatform.Client
	txns    transaction.Repository
	cats    category.Repository
	logs    ailog.Service
	storage storage.Storage
	model   string
}

func NewService(claude *aiplatform.Client, txns transaction.Repository, cats category.Repository, logs ailog.Service, store storage.Storage, model string) Service {
	if model == "" {
		model = "claude-sonnet-4-5"
	}
	return &service{claude: claude, txns: txns, cats: cats, logs: logs, storage: store, model: model}
}

const systemPrompt = `You are SAKU, an AI assistant for personal finance.
Always reply in the same language the user used.
When asked for structured data, return ONLY valid JSON without markdown fences.
Never include commentary outside the JSON object.`

const chatSystemPrompt = `You are SAKU, a friendly assistant embedded INSIDE the SAKU personal-finance application.

SCOPE — you help with:
- The user's own personal finance: income, expenses, budgets, savings, wallets, categories, transactions.
- Reading, summarising, listing, comparing, ranking and re-formatting the user's transactions and totals.
- Recording a new transaction when the user clearly asks (just confirm in one short sentence).
- How to use SAKU features (scan receipt, budgets, dashboard, categories, wallets).
- Light financial tips & literacy directly relevant to managing personal money in Indonesia.

FOLLOW-UPS — IMPORTANT:
- The conversation history is provided to you. Treat short messages like "buat list",
  "format json", "buat tabel", "urutkan dari termahal", "jelaskan lebih detail",
  "jadikan markdown", "ringkas" as REFORMAT requests on the PREVIOUS assistant
  answer (or the user's known data) and comply faithfully.
- If the user asks for JSON or a list/table, output it cleanly (markdown allowed).
  Do NOT refuse just because the request mentions JSON / list / table.
- NEVER hallucinate transactions. Only mention numbers that appear in the provided context or previous assistant message.

REFUSE ONLY WHEN the user CLEARLY asks for something with no link at all to personal finance or SAKU (e.g. recipes, weather, lyrics, code help, jokes, politics, medical advice, game cheats, translations, general knowledge trivia). In that case reply with EXACTLY ONE short sentence in the user's language, e.g. "Maaf, saya hanya bisa membantu seputar keuangan pribadi dan fitur SAKU." — then STOP. Never tack on extra finance commentary after a refusal.

OUTPUT RULES:
- Reply in the user's language (Bahasa Indonesia by default).
- Use Rupiah formatting like "Rp 25.000" when mentioning money.
- Keep prose answers concise (2-6 short sentences). For list/table/JSON requests, output may be longer — stay accurate.
- Do not use markdown emphasis like **bold**. Prefer clean professional plain text.
- If the user seems confused, tell them they can type "help" for guidance.
- Do NOT invent figures. If a number isn't in the context / previous answer, say you don't have it.`

// ─── 1. Categorize from raw text ────────────────────────────────────────────

func (s *service) Categorize(ctx context.Context, userID uuid.UUID, req dto.CategorizeRequest) (dto.CategorizeResponse, error) {
	cats := s.resolveCategories(userID, req.UserCategories)
	prompt := fmt.Sprintf(`You will receive a free-form Indonesian/English text describing one OR MORE personal finance transactions.

ALLOWED CATEGORIES (you MUST pick the BEST match from this exact list, case-insensitive):
%s

CATEGORY RULES — critical to avoid "Uncategorized" results:
- You MUST set "category" to one of the names above, EXACTLY as written.
- NEVER output "Uncategorized", "Unknown", "-", empty string, or any name not in the list.
- If no perfect match exists, pick the closest generic one from the list
  (e.g. "Lainnya" / "Other" / "Misc" if available; otherwise pick the broadest
  applicable category like "Shopping" / "Food & Beverage").
- Common mappings: restaurant/cafe/warung/makan → Food & Beverage; grab/gojek/taxi/transport → Transportation;
  tokopedia/shopee/mall/baju → Shopping; pln/listrik/wifi/internet/pulsa → Bills;
  bioskop/spotify/netflix → Entertainment; transfer ke teman → Transfer; gaji/salary/payroll → Salary.

GENERAL RULES:
1. Detect EVERY distinct transaction in the text. Split on conjunctions like
   ",", ".", "terus", "lalu", "kemudian", "dan", "then", new sentences,
   or whenever a new amount + new item/merchant is mentioned.
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
4. NEVER merge multiple distinct purchases into one transaction.
5. NEVER invent transactions that are not in the text.

Text:
"""
%s
"""

Return ONLY this JSON shape (no markdown, no commentary):
{
  "transactions": [
    {"amount": number, "merchant_name": string, "category": string, "type": "income"|"expense", "confidence": 0..1, "description": string}
  ]
}`,
		strings.Join(cats, ", "), req.Text)

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
	items := extractCategorizeItems(parsed)

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
	} else {
		out.Amount = getNumber(parsed, "amount")
		out.MerchantName = getString(parsed, "merchant_name")
		out.Category = firstString(parsed, "category", "kategori")
		out.Type = normalizeType(getString(parsed, "type"))
		out.Confidence = getNumber(parsed, "confidence")
		if out.Amount > 0 || out.MerchantName != "" || out.Category != "" {
			out.Transactions = []dto.CategorizeItem{{
				Amount:       out.Amount,
				MerchantName: out.MerchantName,
				Category:     out.Category,
				Type:         out.Type,
				Confidence:   out.Confidence,
				Description:  req.Text,
			}}
		}
	}
	out.NeedsReview = needsReview(out.Confidence, parsed == nil)

	s.logLowConfidence(featureCategorize, out.Confidence, parsed)
	s.record(userID, aiLogEntry{
		Feature: featureCategorize, Status: "success", LatencyMs: latency,
		Confidence: &out.Confidence, Merchant: out.MerchantName, Category: out.Category,
		Amount: amountPtr(out.Amount), Raw: parsed,
	})
	return out, nil
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
			Amount:       getNumber(obj, "amount"),
			MerchantName: getString(obj, "merchant_name"),
			Category:     firstString(obj, "category", "kategori"),
			Type:         normalizeType(getString(obj, "type")),
			Confidence:   getNumber(obj, "confidence"),
			Description:  getString(obj, "description"),
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

func (s *service) ScanReceipt(ctx context.Context, userID uuid.UUID, req dto.ScanReceiptRequest) (dto.ScanReceiptResponse, error) {
	cats := s.resolveCategories(userID, req.UserCategories)
	mediaType := req.MediaType
	if mediaType == "" {
		mediaType = "image/jpeg"
	}

	prompt := fmt.Sprintf(`This image is a receipt or bank/e-wallet transaction screenshot.
Extract the structured data and pick the best-fitting category from this list (case-insensitive): %s

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
   - Pick the BEST matching category from the allowed list above.
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

4. "merchant_name" rules:
   - For a transfer OUT (expense): use the RECIPIENT's name / destination
     account holder shown at the top of the receipt.
   - For a received transfer (income): use the SENDER's name.
   - For a store/merchant receipt: use the merchant/store brand name.
   - NEVER use the user's own name (the sender) as merchant_name on an
     outgoing transfer.

5. "amount" must be the numeric total in the receipt's currency. Strip currency
   symbols and thousand separators. Indonesian format "Rp 210.000" = 210000,
   "Rp 1.250.500,50" = 1250500.50. If you cannot find an amount, return 0.

6. "confidence" should reflect how certain you are about (type + category +
   amount + merchant). If any of these is ambiguous, set confidence below 0.7
   so the UI can ask the user to review.

Return ONLY this JSON (no commentary, no markdown fences):
{"amount": number, "merchant_name": string, "category": string, "type": "income"|"expense", "currency": string, "date": "YYYY-MM-DD", "confidence": 0..1, "ocr_text": string}`,
		strings.Join(cats, ", "))

	start := time.Now()
	raw, err := s.claude.AskImage(ctx, systemPrompt, prompt, mediaType, req.ImageBase64)
	latency := int(time.Since(start).Milliseconds())

	if err != nil {
		s.record(userID, aiLogEntry{Feature: featureScanReceipt, Status: "failed", LatencyMs: latency, ErrMsg: err.Error(), Raw: map[string]any{"media_type": mediaType}})
		return dto.ScanReceiptResponse{}, err
	}

	parsed := parseJSON(raw)
	out := dto.ScanReceiptResponse{
		Amount:       getNumber(parsed, "amount"),
		MerchantName: getString(parsed, "merchant_name"),
		Category:     firstString(parsed, "category", "kategori"),
		Type:         normalizeType(getString(parsed, "type")),
		Currency:     defaultIfEmpty(getString(parsed, "currency"), "IDR"),
		Date:         getString(parsed, "date"),
		Confidence:   getNumber(parsed, "confidence"),
		OCRText:      getString(parsed, "ocr_text"),
		RawResponse:  parsed,
	}
	out.NeedsReview = needsReview(out.Confidence, parsed == nil)

	if parsed == nil {
		parsed = map[string]any{}
	}
	if s.storage != nil && req.ImageBase64 != "" {
		if data, derr := base64.StdEncoding.DecodeString(req.ImageBase64); derr == nil && len(data) > 0 {
			folder := fmt.Sprintf("ai-scans/%s", userID.String())
			if key, uerr := s.storage.UploadBytes(ctx, data, mediaType, folder, ""); uerr == nil {
				parsed["image_key"] = key
			} else {
				slog.Warn("ai: scan receipt image upload failed", "user_id", userID, "error", uerr)
			}
		} else if derr != nil {
			slog.Warn("ai: scan receipt base64 decode failed", "user_id", userID, "error", derr)
		}
	}

	s.logLowConfidence(featureScanReceipt, out.Confidence, parsed)
	s.record(userID, aiLogEntry{
		Feature: featureScanReceipt, Status: "success", LatencyMs: latency,
		Confidence: &out.Confidence, Merchant: out.MerchantName, Category: out.Category,
		Amount: &out.Amount, Raw: parsed,
	})
	return out, nil
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
	if isHelpMessage(req.Message) {
		reply := "Panduan Chatbot:\n1. Tanyakan ringkasan pengeluaran, kategori terbesar, atau perbandingan bulan ini.\n2. Minta format spesifik seperti list singkat atau tabel.\n3. Jawaban memakai data transaksi yang tersedia di akunmu.\n4. Untuk mencatat transaksi, gunakan mode NLP dan tulis contoh seperti \"beli kopi 25rb\"."
		s.record(userID, aiLogEntry{Feature: featureChat, Status: "success", LatencyMs: 0, Raw: map[string]any{"message": req.Message, "reply": reply, "session_id": req.SessionID}})
		return dto.ChatResponse{Reply: reply}, nil
	}
	if isHardOffTopic(req.Message) {
		reply := "Maaf, saya hanya bisa membantu seputar keuangan pribadi dan fitur SAKU. Boleh tanya soal pengeluaran, anggaran, atau transaksi kamu."
		s.record(userID, aiLogEntry{Feature: featureChat, Status: "success", LatencyMs: 0, Raw: map[string]any{"message": req.Message, "reply": reply, "refused": true, "session_id": req.SessionID}})
		return dto.ChatResponse{Reply: reply}, nil
	}

	var promptBuilder strings.Builder

	if req.IncludeContext {
		to := time.Now()
		from := to.AddDate(0, -1, 0)
		rows, _, err := s.txns.List(transaction.ListFilter{
			UserID: userID, From: &from, To: &to, Page: 1, Limit: 50,
		})
		if err == nil && len(rows) > 0 {
			income, expense, byCat := summarise(rows)
			promptBuilder.WriteString(fmt.Sprintf("Context (last 30 days):\n- income=%.2f, expense=%.2f\n- top expense categories: %s\n- recent transactions:\n%s\n\n",
				income, expense, topCategoriesString(byCat, 5), sampleRowsString(rows, 15)))
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

func (s *service) record(userID uuid.UUID, e aiLogEntry) {
	if s.logs == nil {
		return
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
	if err := s.logs.Record(ctx, userID, req); err != nil {
		slog.Warn("ai: persist log failed",
			"feature", e.Feature, "user_id", userID, "error", err)
	}
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
			key := "Tanpa Kategori"
			if t.Category != nil && t.Category.Name != "" {
				key = t.Category.Name
			}
			byCategory[key] += t.Amount
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

func sampleRowsString(rows []domain.Transaction, n int) string {
	if n > len(rows) {
		n = len(rows)
	}
	lines := make([]string, 0, n)
	for i := 0; i < n; i++ {
		t := rows[i]
		cat := "Tanpa Kategori"
		if t.Category != nil && t.Category.Name != "" {
			cat = t.Category.Name
		}
		merchant := t.MerchantName
		if merchant == "" {
			merchant = t.Description
		}
		lines = append(lines, fmt.Sprintf("- %s [%s] %s %.0f (%s)",
			t.TransactionDate.Format("2006-01-02"), t.Type, cat, t.Amount, merchant))
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
