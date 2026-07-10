package telegram

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"mime"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	aimodule "github.com/ganiramadhan/starter-go/internal/modules/ai"
	"github.com/ganiramadhan/starter-go/internal/modules/category"
	"github.com/ganiramadhan/starter-go/internal/modules/transaction"
	"github.com/ganiramadhan/starter-go/internal/modules/user"
	"github.com/ganiramadhan/starter-go/internal/modules/wallet"
	"github.com/google/uuid"
)

const pendingPreviewTTL = 15 * time.Minute

var moneyPattern = regexp.MustCompile(`(?i)(^|[\s])(\d{1,3}([.,]\d{3})+|\d+)(\s?(rb|ribu|k|jt|juta|m|million))?([\s]|$)`)

type Service interface {
	HandleUpdate(ctx context.Context, update Update) error
}

type service struct {
	users        user.Repository
	wallets      wallet.Repository
	categories   category.Repository
	transactions transaction.Service
	ai           aimodule.Service
	bot          Client
	pendingMu    sync.Mutex
	pending      map[int64]pendingPreview
	historyMu    sync.Mutex
	history      map[int64][]dto.ChatTurn
}

type pendingPreview struct {
	UserID    uuid.UUID
	Items     []pendingTransaction
	Total     float64
	CreatedAt time.Time
	ImageKey  string
	AILogID   string
}

type pendingTransaction struct {
	Request      dto.CreateTransactionRequest
	WalletName   string
	CategoryName string
	Description  string
}

func NewService(
	users user.Repository,
	wallets wallet.Repository,
	categories category.Repository,
	transactions transaction.Service,
	ai aimodule.Service,
	bot Client,
) Service {
	return &service{
		users:        users,
		wallets:      wallets,
		categories:   categories,
		transactions: transactions,
		ai:           ai,
		bot:          bot,
		pending:      map[int64]pendingPreview{},
		history:      map[int64][]dto.ChatTurn{},
	}
}

func (s *service) HandleUpdate(ctx context.Context, update Update) error {
	if update.CallbackQuery != nil {
		return s.handleCallback(ctx, *update.CallbackQuery)
	}
	if update.Message == nil {
		return nil
	}
	chatID := update.Message.Chat.ID
	text := strings.TrimSpace(update.Message.Text)
	if chatID == 0 {
		return nil
	}

	chatIDText := strconv.FormatInt(chatID, 10)
	u, err := s.users.FindByTelegramChatID(chatIDText)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return s.bot.SendMessage(ctx, chatID, unboundMessage(chatIDText))
		}
		return err
	}
	s.rememberTelegramUsername(chatIDText, update.Message.From, update.Message.Chat)

	if fileID, fileName := receiptImageFile(*update.Message); fileID != "" {
		caption := strings.TrimSpace(update.Message.Caption)
		reply, err := s.buildReceiptPreview(ctx, u.ID, fileID, fileName, caption)
		if err != nil {
			return s.bot.SendMessage(ctx, chatID, friendlyReceiptError(err))
		}
		s.setPending(chatID, reply)
		return s.bot.SendMessage(ctx, chatID, renderPreview(reply), WithInlineKeyboard(confirmKeyboard()))
	}

	if text == "" {
		return nil
	}

	lower := strings.ToLower(text)
	switch {
	case lower == "/start" || lower == "/help" || lower == "help":
		return s.bot.SendMessage(ctx, chatID, helpMessage())
	case isConfirmText(lower):
		reply, err := s.confirmPending(ctx, chatID, u.ID)
		if err != nil {
			return s.bot.SendMessage(ctx, chatID, friendlyError(err))
		}
		return s.bot.SendMessage(ctx, chatID, reply)
	case isCancelText(lower):
		s.clearPending(chatID)
		return s.bot.SendMessage(ctx, chatID, "Preview transaksi dibatalkan.")
	}

	if strings.HasPrefix(lower, "/preview") {
		text = strings.TrimSpace(text[len("/preview"):])
	}
	if text == "" {
		return s.bot.SendMessage(ctx, chatID, "Tulis transaksi setelah /preview, contoh: /preview beli kopi 25rb pake cash")
	}

	if !looksLikeTransaction(text) {
		reply, err := s.answerFinanceQuestion(ctx, u.ID, text, chatID)
		if err != nil {
			return s.bot.SendMessage(ctx, chatID, friendlyChatError(err))
		}
		return s.bot.SendMessage(ctx, chatID, reply)
	}

	preview, err := s.buildPreview(ctx, u.ID, text)
	if err != nil {
		return s.bot.SendMessage(ctx, chatID, friendlyError(err))
	}
	if len(preview.Items) == 0 {
		return s.bot.SendMessage(ctx, chatID, "Aku belum menemukan nominal transaksi yang jelas. Coba tulis seperti: beli kopi 25rb pake cash.")
	}
	s.setPending(chatID, preview)
	return s.bot.SendMessage(ctx, chatID, renderPreview(preview), WithInlineKeyboard(confirmKeyboard()))
}

func (s *service) buildReceiptPreview(ctx context.Context, userID uuid.UUID, fileID, fileName, caption string) (pendingPreview, error) {
	if s.bot == nil {
		return pendingPreview{}, errors.New("telegram bot is not configured")
	}
	file, err := s.bot.GetFile(ctx, fileID)
	if err != nil {
		return pendingPreview{}, err
	}
	data, mediaType, err := s.bot.DownloadFile(ctx, file.FilePath)
	if err != nil {
		return pendingPreview{}, err
	}
	if len(data) == 0 {
		return pendingPreview{}, errors.New("telegram image is empty")
	}
	mediaType = normalizeTelegramMediaType(mediaType, fallbackText(file.FilePath, fileName))
	if !isSupportedReceiptMedia(mediaType) {
		return pendingPreview{}, errors.New("unsupported receipt image")
	}
	cats, err := s.categories.List(userID, "")
	if err != nil {
		return pendingPreview{}, err
	}
	out, err := s.ai.ScanReceipt(ctx, userID, dto.ScanReceiptRequest{
		ImageBase64:    base64.StdEncoding.EncodeToString(data),
		MediaType:      mediaType,
		UserCategories: uniqueCategoryNames(cats),
	})
	if err != nil {
		return pendingPreview{}, err
	}
	return s.previewFromReceipt(ctx, userID, out, caption)
}

func (s *service) previewFromReceipt(ctx context.Context, userID uuid.UUID, out dto.ScanReceiptResponse, caption string) (pendingPreview, error) {
	if out.Amount <= 0 {
		return pendingPreview{}, errors.New("receipt amount missing")
	}
	cats, err := s.categories.List(userID, "")
	if err != nil {
		return pendingPreview{}, err
	}
	wallets, err := s.wallets.List(userID)
	if err != nil {
		return pendingPreview{}, err
	}
	if len(wallets) == 0 {
		return pendingPreview{}, errors.New("wallet required")
	}
	w := resolveWallet(wallets, caption)
	c := resolveCategory(cats, out.Category, out.Type)
	confidence := out.Confidence
	description := fallbackText(out.Description, out.MerchantName, "Scan struk Telegram")
	req := dto.CreateTransactionRequest{
		WalletID:        w.ID,
		CategoryID:      c.ID,
		Amount:          out.Amount,
		Type:            normalizeType(out.Type),
		Description:     description,
		MerchantName:    cleanMerchant(out.MerchantName),
		TransactionDate: parseTransactionDate(out.Date),
		Source:          domain.TxnSourceAIOCR,
		ConfidenceScore: &confidence,
	}
	preview := pendingPreview{
		UserID:    userID,
		CreatedAt: time.Now(),
		ImageKey:  out.ImageKey,
		AILogID:   out.LogID,
	}
	preview.Items = append(preview.Items, pendingTransaction{
		Request:      req,
		WalletName:   w.Name,
		CategoryName: c.Name,
		Description:  description,
	})
	preview.Total = signedTotal(req.Type, req.Amount)
	return preview, nil
}

func (s *service) handleCallback(ctx context.Context, callback CallbackQuery) error {
	if callback.Message == nil {
		return nil
	}
	chatID := callback.Message.Chat.ID
	chatIDText := strconv.FormatInt(chatID, 10)
	u, err := s.users.FindByTelegramChatID(chatIDText)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return s.bot.SendMessage(ctx, chatID, unboundMessage(chatIDText))
		}
		return err
	}
	s.rememberTelegramUsername(chatIDText, &callback.From, callback.Message.Chat)
	switch callback.Data {
	case "saku:tx:confirm":
		_ = s.bot.ClearInlineKeyboard(ctx, chatID, callback.Message.MessageID)
		reply, err := s.confirmPending(ctx, chatID, u.ID)
		if err != nil {
			return s.bot.SendMessage(ctx, chatID, friendlyError(err))
		}
		return s.bot.SendMessage(ctx, chatID, reply)
	case "saku:tx:cancel":
		_ = s.bot.ClearInlineKeyboard(ctx, chatID, callback.Message.MessageID)
		s.clearPending(chatID)
		return s.bot.SendMessage(ctx, chatID, "Preview transaksi dibatalkan. Kirim ulang kalau mau dicatat lagi.")
	default:
		return nil
	}
}

func (s *service) rememberTelegramUsername(chatID string, from *User, chat Chat) {
	username := ""
	if from != nil {
		username = strings.TrimSpace(from.Username)
	}
	if username == "" {
		username = strings.TrimSpace(chat.Username)
	}
	if username == "" {
		return
	}
	if err := s.users.UpdateTelegramUsernameByChatID(chatID, username); err != nil {
		// Non-critical profile metadata; do not fail the Telegram update.
		return
	}
}

func (s *service) answerFinanceQuestion(ctx context.Context, userID uuid.UUID, text string, chatID int64) (string, error) {
	history := s.getHistory(chatID)
	jakartaNow := time.Now().In(jakartaLocation())
	out, err := s.ai.Chat(ctx, userID, dto.ChatRequest{
		Message:        text,
		IncludeContext: true,
		History:        history,
		SessionID:      "telegram-chat:" + strconv.FormatInt(chatID, 10),
		Language:       "id",
		ReferenceDate:  jakartaNow.Format(time.RFC3339),
		Timezone:       "Asia/Jakarta",
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(out.Reply) == "" {
		return "Aku belum menemukan jawaban dari data SAKU kamu.", nil
	}
	s.addHistory(chatID, "user", text)
	s.addHistory(chatID, "assistant", out.Reply)
	return out.Reply, nil
}

func (s *service) buildPreview(ctx context.Context, userID uuid.UUID, text string) (pendingPreview, error) {
	cats, err := s.categories.List(userID, "")
	if err != nil {
		return pendingPreview{}, err
	}
	wallets, err := s.wallets.List(userID)
	if err != nil {
		return pendingPreview{}, err
	}
	if len(wallets) == 0 {
		return pendingPreview{}, errors.New("wallet required")
	}

	out, err := s.ai.Categorize(ctx, userID, dto.CategorizeRequest{
		Text:           text,
		UserCategories: uniqueCategoryNames(cats),
		SessionID:      "telegram-nlp:" + strconv.FormatInt(time.Now().UnixNano(), 10),
		Language:       "id",
		ReferenceDate:  time.Now().In(jakartaLocation()).Format(time.RFC3339),
		Timezone:       "Asia/Jakarta",
	})
	if err != nil {
		return pendingPreview{}, err
	}

	items := out.Transactions
	if len(items) == 0 && out.Amount > 0 {
		items = []dto.CategorizeItem{{
			Amount:       out.Amount,
			MerchantName: out.MerchantName,
			Category:     out.Category,
			Type:         out.Type,
			Confidence:   out.Confidence,
			Date:         out.Date,
			Description:  out.MerchantName,
		}}
	}

	preview := pendingPreview{UserID: userID, CreatedAt: time.Now()}
	for _, item := range items {
		if item.Amount <= 0 {
			continue
		}
		w := resolveWallet(wallets, item.WalletHint)
		c := resolveCategory(cats, item.Category, item.Type)
		confidence := item.Confidence
		description := fallbackText(item.Description, item.MerchantName)
		req := dto.CreateTransactionRequest{
			WalletID:        w.ID,
			CategoryID:      c.ID,
			Amount:          item.Amount,
			Type:            normalizeType(item.Type),
			Description:     description,
			MerchantName:    cleanMerchant(item.MerchantName),
			TransactionDate: parseTransactionDate(item.Date),
			Source:          domain.TxnSourceAPI,
			ConfidenceScore: &confidence,
		}
		preview.Items = append(preview.Items, pendingTransaction{
			Request:      req,
			WalletName:   w.Name,
			CategoryName: c.Name,
			Description:  description,
		})
		preview.Total += signedTotal(req.Type, req.Amount)
	}
	return preview, nil
}

func (s *service) confirmPending(ctx context.Context, chatID int64, userID uuid.UUID) (string, error) {
	preview, ok := s.getPending(chatID)
	if !ok || len(preview.Items) == 0 {
		return "Belum ada preview transaksi aktif. Kirim transaksi dulu, contoh: beli kopi 25rb pake cash.", nil
	}
	if preview.UserID != userID {
		s.clearPending(chatID)
		return "Preview transaksi sudah tidak valid. Kirim transaksi lagi untuk membuat preview baru.", nil
	}
	lines := make([]string, 0, len(preview.Items))
	for _, item := range preview.Items {
		if _, err := s.transactions.Create(ctx, userID, item.Request); err != nil {
			return "", err
		}
		lines = append(lines, fmt.Sprintf("Tersimpan: %s %s, %s, %s, %s.", signPrefix(item.Request.Type), formatRupiah(item.Request.Amount), item.Description, item.CategoryName, item.WalletName))
	}
	if preview.ImageKey != "" {
		_, _ = s.ai.PromoteScanImage(ctx, userID, dto.PromoteScanImageRequest{
			ImageKey: preview.ImageKey,
			LogID:    preview.AILogID,
		})
	}
	s.clearPending(chatID)
	return "✅ Transaksi berhasil disimpan.\n" + strings.Join(lines, "\n") + "\nTotal: " + formatSignedRupiah(preview.Total), nil
}

func (s *service) setPending(chatID int64, preview pendingPreview) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	s.pending[chatID] = preview
}

func (s *service) getPending(chatID int64) (pendingPreview, bool) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	preview, ok := s.pending[chatID]
	if !ok || time.Since(preview.CreatedAt) > pendingPreviewTTL {
		delete(s.pending, chatID)
		return pendingPreview{}, false
	}
	return preview, true
}

func (s *service) clearPending(chatID int64) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	delete(s.pending, chatID)
}

func (s *service) getHistory(chatID int64) []dto.ChatTurn {
	s.historyMu.Lock()
	defer s.historyMu.Unlock()
	rows := s.history[chatID]
	out := make([]dto.ChatTurn, len(rows))
	copy(out, rows)
	return out
}

func (s *service) addHistory(chatID int64, role, content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}
	s.historyMu.Lock()
	defer s.historyMu.Unlock()
	rows := append(s.history[chatID], dto.ChatTurn{Role: role, Content: content})
	if len(rows) > 8 {
		rows = rows[len(rows)-8:]
	}
	s.history[chatID] = rows
}

func renderPreview(preview pendingPreview) string {
	lines := []string{"🧾 Preview transaksi", "Cek detailnya sebelum disimpan:"}
	for i, item := range preview.Items {
		lines = append(lines, fmt.Sprintf("%d. %s %s", i+1, signPrefix(item.Request.Type), formatRupiah(item.Request.Amount)))
		lines = append(lines, fmt.Sprintf("   %s · %s · %s", item.Description, item.CategoryName, item.WalletName))
		lines = append(lines, fmt.Sprintf("   Tanggal: %s", item.Request.TransactionDate.Format("02 Jan 2006")))
	}
	lines = append(lines, "Total: "+formatSignedRupiah(preview.Total))
	lines = append(lines, "Pilih Simpan untuk mencatat, atau Batalkan kalau belum sesuai.")
	return strings.Join(lines, "\n")
}

func confirmKeyboard() InlineKeyboardMarkup {
	return InlineKeyboardMarkup{
		InlineKeyboard: [][]InlineKeyboardButton{{
			{Text: "✅ Simpan", CallbackData: "saku:tx:confirm"},
			{Text: "✕ Batalkan", CallbackData: "saku:tx:cancel"},
		}},
	}
}

func looksLikeTransaction(text string) bool {
	lower := normalizeText(text)
	if !moneyPattern.MatchString(text) {
		return false
	}
	questionMarkers := []string{"berapa", "total", "list", "daftar", "ringkasan", "saldo", "pengeluaranku", "pemasukanku"}
	actionMarkers := []string{"beli", "bayar", "jajan", "makan", "minum", "catat", "pemasukan", "gaji", "bonus", "refund", "lunch", "dinner", "coffee", "kopi", "transfer"}
	hasQuestion := containsAny(lower, questionMarkers)
	hasAction := containsAny(lower, actionMarkers)
	return hasAction || !hasQuestion
}

func isConfirmText(text string) bool {
	text = normalizeText(text)
	text = strings.TrimPrefix(text, "/")
	return text == "simpan" || text == "confirm" || text == "konfirmasi" || text == "ya" || text == "yes"
}

func isCancelText(text string) bool {
	text = normalizeText(text)
	text = strings.TrimPrefix(text, "/")
	return text == "batal" || text == "cancel" || text == "batalkan" || text == "tidak" || text == "no"
}

func containsAny(text string, values []string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}

func receiptImageFile(message Message) (fileID, fileName string) {
	if len(message.Photo) > 0 {
		best := message.Photo[0]
		bestScore := int64(best.Width * best.Height)
		if best.FileSize > 0 {
			bestScore = best.FileSize
		}
		for _, photo := range message.Photo[1:] {
			score := int64(photo.Width * photo.Height)
			if photo.FileSize > 0 {
				score = photo.FileSize
			}
			if score > bestScore {
				best = photo
				bestScore = score
			}
		}
		return best.FileID, "telegram-photo.jpg"
	}
	if message.Document != nil && strings.HasPrefix(strings.ToLower(message.Document.MimeType), "image/") {
		return message.Document.FileID, message.Document.FileName
	}
	return "", ""
}

func normalizeTelegramMediaType(mediaType, fileName string) string {
	mediaType = strings.TrimSpace(strings.Split(mediaType, ";")[0])
	if mediaType == "" || mediaType == "application/octet-stream" {
		mediaType = mime.TypeByExtension(strings.ToLower(filepath.Ext(fileName)))
		mediaType = strings.TrimSpace(strings.Split(mediaType, ";")[0])
	}
	if mediaType == "image/jpg" {
		return "image/jpeg"
	}
	return mediaType
}

func isSupportedReceiptMedia(mediaType string) bool {
	switch mediaType {
	case "image/jpeg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}

func resolveWallet(wallets []domain.Wallet, hint string) domain.Wallet {
	if len(wallets) == 0 {
		return domain.Wallet{}
	}
	needle := normalizeText(hint)
	if needle != "" {
		for _, w := range wallets {
			if normalizeText(w.Name) == needle || strings.Contains(normalizeText(w.Name), needle) || strings.Contains(needle, normalizeText(w.Name)) {
				return w
			}
		}
		if strings.Contains(needle, "cash") || strings.Contains(needle, "tunai") {
			for _, w := range wallets {
				if strings.Contains(normalizeText(w.Name), "cash") || w.Type == "cash" {
					return w
				}
			}
		}
	}
	for _, w := range wallets {
		if w.IsDefault {
			return w
		}
	}
	return wallets[0]
}

func resolveCategory(categories []domain.Category, name, txType string) domain.Category {
	normalizedType := normalizeType(txType)
	needle := normalizeText(name)
	for _, c := range categories {
		if c.Type == normalizedType && normalizeText(c.Name) == needle {
			return c
		}
	}
	for _, c := range categories {
		if c.Type == normalizedType && (strings.Contains(normalizeText(c.Name), needle) || strings.Contains(needle, normalizeText(c.Name))) {
			return c
		}
	}
	for _, c := range categories {
		if c.Type == normalizedType {
			return c
		}
	}
	if len(categories) > 0 {
		return categories[0]
	}
	return domain.Category{}
}

func uniqueCategoryNames(categories []domain.Category) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(categories))
	for _, c := range categories {
		name := strings.TrimSpace(c.Name)
		if name == "" || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func parseTransactionDate(value string) time.Time {
	loc := jakartaLocation()
	value = strings.TrimSpace(value)
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if parsed, err := time.ParseInLocation(layout, value, loc); err == nil {
			return parsed
		}
	}
	return time.Now().In(loc)
}

func jakartaLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*60*60)
	}
	return loc
}

func normalizeType(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "income") {
		return "income"
	}
	return "expense"
}

func normalizeText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", " ")
	value = strings.ReplaceAll(value, "_", " ")
	value = strings.Join(strings.Fields(value), " ")
	return value
}

func fallbackText(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && value != "-" {
			return value
		}
	}
	return "Transaksi Telegram"
}

func cleanMerchant(value string) string {
	value = strings.TrimSpace(value)
	if value == "-" {
		return ""
	}
	return value
}

func signPrefix(txType string) string {
	if normalizeType(txType) == "income" {
		return "+"
	}
	return "-"
}

func signedTotal(txType string, amount float64) float64 {
	if normalizeType(txType) == "income" {
		return amount
	}
	return -amount
}

func formatSignedRupiah(amount float64) string {
	prefix := ""
	if amount < 0 {
		prefix = "-"
		amount = math.Abs(amount)
	}
	return prefix + formatRupiah(amount)
}

func formatRupiah(amount float64) string {
	n := int64(math.Round(amount))
	raw := strconv.FormatInt(n, 10)
	var out []byte
	for i, c := range reverse(raw) {
		if i > 0 && i%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, byte(c))
	}
	return "Rp " + string(reverse(string(out)))
}

func reverse(value string) string {
	runes := []rune(value)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func helpMessage() string {
	return "✨ SAKU Telegram Bot\n\nKirim transaksi seperti: beli kopi 25rb pake cash.\nBot akan membuat preview dulu sebelum menyimpan.\n\nKamu juga bisa bertanya soal data SAKU di web, misalnya:\n• berapa pengeluaranku hari ini?\n• berikan list transaksi aku pada bulan mei\n\nPerintah cepat:\n/simpan atau Simpan untuk menyimpan preview\n/batal atau Batalkan untuk membatalkan preview"
}

func unboundMessage(chatID string) string {
	return "Telegram kamu belum terhubung ke akun SAKU.\n\nChat ID: " + chatID + "\n\nBuka Profile di SAKU, masukkan Chat ID ini di kartu Telegram Bot, lalu coba chat: beli kopi 25rb pake cash."
}

func friendlyError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "subscription has expired") {
		return "Langganan SAKU kamu sudah habis. Perpanjang paket dulu supaya bisa lanjut memakai AI dan Telegram."
	}
	if strings.Contains(msg, "limit") || strings.Contains(msg, "quota") {
		return "Kuota AI bulanan kamu sudah habis. Upgrade ke Pro atau Premium supaya bisa lanjut mencatat transaksi dan bertanya lewat Telegram."
	}
	if strings.Contains(msg, "free plan includes") || strings.Contains(msg, "prompt ai") || strings.Contains(msg, "monthly ai") {
		return "Kuota AI paket Free sudah terpakai bulan ini. Upgrade ke Pro untuk kuota AI yang lebih besar dan tetap bisa pakai Telegram."
	}
	if strings.Contains(msg, "wallet required") {
		return "Kamu belum punya wallet di SAKU. Buat wallet dulu dari aplikasi, lalu coba lagi."
	}
	return "Maaf, pesan belum bisa diproses. Coba tulis lebih jelas, misalnya: beli kopi 25rb pake cash."
}

func friendlyChatError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "subscription has expired") {
		return "Langganan SAKU kamu sudah habis. Perpanjang paket dulu supaya bisa lanjut bertanya dari Telegram."
	}
	if strings.Contains(msg, "limit") || strings.Contains(msg, "quota") || strings.Contains(msg, "free plan includes") || strings.Contains(msg, "monthly ai") {
		return "Kuota AI bulanan kamu sudah habis. Upgrade ke Pro atau Premium supaya bisa lanjut bertanya soal cashflow dan transaksi dari Telegram."
	}
	return "Maaf, aku belum bisa mengambil ringkasan data SAKU kamu sekarang. Coba ulang sebentar lagi, atau cek langsung dari dashboard web."
}

func friendlyReceiptError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "unsupported receipt image") {
		return "Format gambar belum didukung. Kirim foto struk dalam format JPG, PNG, atau WebP ya."
	}
	if strings.Contains(msg, "amount missing") {
		return "Aku belum bisa membaca nominal dari struk itu. Coba kirim foto yang lebih jelas atau scan dari aplikasi SAKU."
	}
	if strings.Contains(msg, "exceeds 8mb") {
		return "Ukuran gambar terlalu besar. Coba kirim foto struk yang lebih ringan, maksimal 8 MB."
	}
	return friendlyError(err)
}
