package transaction

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/modules/category"
	"github.com/ganiramadhan/starter-go/internal/modules/wallet"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

type ExportFilter struct {
	UserID     uuid.UUID
	WalletID   *uuid.UUID
	CategoryID *uuid.UUID
	Type       string
	From       *time.Time
	To         *time.Time
}

type ExportService interface {
	ExportXLSX(ctx context.Context, f ExportFilter) (data []byte, filename string, err error)
}

type exportSvc struct {
	repo    Repository
	wallets wallet.Repository
	cats    category.Repository
}

func NewExportService(r Repository, w wallet.Repository, c category.Repository) ExportService {
	return &exportSvc{repo: r, wallets: w, cats: c}
}

func (e *exportSvc) ExportXLSX(_ context.Context, f ExportFilter) ([]byte, string, error) {
	lf := ListFilter{
		UserID:     f.UserID,
		WalletID:   f.WalletID,
		CategoryID: f.CategoryID,
		Type:       f.Type,
		From:       f.From,
		To:         f.To,
		Page:       1,
		Limit:      50000,
	}
	rows, _, err := e.repo.List(lf)
	if err != nil {
		return nil, "", err
	}

	walletMap := map[uuid.UUID]string{}
	if ws, err := e.wallets.List(f.UserID); err == nil {
		for _, w := range ws {
			walletMap[w.ID] = w.Name
		}
	}
	catMap := map[uuid.UUID]string{}
	if cs, err := e.cats.List(f.UserID, ""); err == nil {
		for _, c := range cs {
			catMap[c.ID] = c.Name
		}
	}

	xf := excelize.NewFile()
	defer func() { _ = xf.Close() }()

	sheet := "Transaksi"
	idx, err := xf.NewSheet(sheet)
	if err != nil {
		return nil, "", err
	}
	_ = xf.DeleteSheet("Sheet1")
	xf.SetActiveSheet(idx)

	// Header style
	hdrStyle, _ := xf.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"1F2937"}, Pattern: 1},
		Alignment: &excelize.Alignment{
			Horizontal: "center", Vertical: "center",
		},
	})
	moneyStyle, _ := xf.NewStyle(&excelize.Style{
		NumFmt: 4, // #,##0.00
	})
	dateStyle, _ := xf.NewStyle(&excelize.Style{
		NumFmt: 22, // m/d/yyyy h:mm
	})

	headers := []string{"Tanggal", "Tipe", "Kategori", "Dompet", "Deskripsi", "Merchant", "Jumlah"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = xf.SetCellValue(sheet, cell, h)
	}
	_ = xf.SetCellStyle(sheet, "A1", fmt.Sprintf("%c1", 'A'+len(headers)-1), hdrStyle)

	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].TransactionDate.Before(rows[j].TransactionDate)
	})

	var totalIncome, totalExpense float64
	for i, t := range rows {
		r := i + 2
		_ = xf.SetCellValue(sheet, fmt.Sprintf("A%d", r), t.TransactionDate)
		_ = xf.SetCellStyle(sheet, fmt.Sprintf("A%d", r), fmt.Sprintf("A%d", r), dateStyle)

		typeLabel := "Pengeluaran"
		if t.Type == domain.TxnTypeIncome {
			typeLabel = "Pemasukan"
			totalIncome += t.Amount
		} else {
			totalExpense += t.Amount
		}
		_ = xf.SetCellValue(sheet, fmt.Sprintf("B%d", r), typeLabel)
		_ = xf.SetCellValue(sheet, fmt.Sprintf("C%d", r), catMap[t.CategoryID])
		_ = xf.SetCellValue(sheet, fmt.Sprintf("D%d", r), walletMap[t.WalletID])
		_ = xf.SetCellValue(sheet, fmt.Sprintf("E%d", r), t.Description)
		_ = xf.SetCellValue(sheet, fmt.Sprintf("F%d", r), t.MerchantName)
		_ = xf.SetCellValue(sheet, fmt.Sprintf("G%d", r), t.Amount)
		_ = xf.SetCellStyle(sheet, fmt.Sprintf("G%d", r), fmt.Sprintf("G%d", r), moneyStyle)
	}

	// Summary footer
	summaryRow := len(rows) + 3
	_ = xf.SetCellValue(sheet, fmt.Sprintf("F%d", summaryRow), "Total Pemasukan")
	_ = xf.SetCellValue(sheet, fmt.Sprintf("G%d", summaryRow), totalIncome)
	_ = xf.SetCellStyle(sheet, fmt.Sprintf("G%d", summaryRow), fmt.Sprintf("G%d", summaryRow), moneyStyle)

	_ = xf.SetCellValue(sheet, fmt.Sprintf("F%d", summaryRow+1), "Total Pengeluaran")
	_ = xf.SetCellValue(sheet, fmt.Sprintf("G%d", summaryRow+1), totalExpense)
	_ = xf.SetCellStyle(sheet, fmt.Sprintf("G%d", summaryRow+1), fmt.Sprintf("G%d", summaryRow+1), moneyStyle)

	_ = xf.SetCellValue(sheet, fmt.Sprintf("F%d", summaryRow+2), "Net Cashflow")
	_ = xf.SetCellValue(sheet, fmt.Sprintf("G%d", summaryRow+2), totalIncome-totalExpense)
	_ = xf.SetCellStyle(sheet, fmt.Sprintf("G%d", summaryRow+2), fmt.Sprintf("G%d", summaryRow+2), moneyStyle)

	// Auto-ish column widths
	widths := map[string]float64{"A": 20, "B": 14, "C": 22, "D": 22, "E": 40, "F": 22, "G": 16}
	for col, w := range widths {
		_ = xf.SetColWidth(sheet, col, col, w)
	}

	var buf bytes.Buffer
	if err := xf.Write(&buf); err != nil {
		return nil, "", err
	}

	stamp := time.Now().Format("2006-01-02")
	if f.From != nil && f.To != nil {
		stamp = fmt.Sprintf("%s_%s", f.From.Format("2006-01-02"), f.To.Format("2006-01-02"))
	}
	return buf.Bytes(), fmt.Sprintf("transaksi-%s.xlsx", stamp), nil
}
