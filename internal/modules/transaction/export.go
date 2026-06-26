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

	if err := buildTransactionWorkbook(xf, rows, walletMap, catMap, f); err != nil {
		return nil, "", err
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

func buildTransactionWorkbook(
	xf *excelize.File,
	rows []domain.Transaction,
	walletMap map[uuid.UUID]string,
	catMap map[uuid.UUID]string,
	filter ExportFilter,
) error {
	sheet := "Transaksi"
	idx, err := xf.NewSheet(sheet)
	if err != nil {
		return err
	}
	_ = xf.DeleteSheet("Sheet1")
	xf.SetActiveSheet(idx)

	titleStyle, _ := xf.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "17120F", Size: 22},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"FF9D8D"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    fullBorder("17120F", 2),
	})
	subtitleStyle, _ := xf.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "6F625B", Size: 10},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"FFF8F4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    fullBorder("E7D8CF", 1),
	})
	summaryLabelStyle, _ := xf.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "6F625B", Size: 9},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"FFF8F4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
		Border:    fullBorder("E7D8CF", 1),
	})
	summaryIncomeStyle, _ := xf.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Color: "065F46", Size: 15},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"ECFDF5"}, Pattern: 1},
		CustomNumFmt: stringPtr(`"Rp" #,##0;[Red]-"Rp" #,##0`),
		Alignment:    &excelize.Alignment{Horizontal: "left", Vertical: "center"},
		Border:       fullBorder("A7F3D0", 1),
	})
	summaryExpenseStyle, _ := xf.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Color: "9F342A", Size: 15},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"FFE4DC"}, Pattern: 1},
		CustomNumFmt: stringPtr(`"Rp" #,##0;[Red]-"Rp" #,##0`),
		Alignment:    &excelize.Alignment{Horizontal: "left", Vertical: "center"},
		Border:       fullBorder("FFC6BA", 1),
	})
	summaryNetStyle, _ := xf.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Color: "17120F", Size: 15},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"FDDF82"}, Pattern: 1},
		CustomNumFmt: stringPtr(`"Rp" #,##0;[Red]-"Rp" #,##0`),
		Alignment:    &excelize.Alignment{Horizontal: "left", Vertical: "center"},
		Border:       fullBorder("D8B85B", 1),
	})
	hdrStyle, _ := xf.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "17120F"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"FF9D8D"}, Pattern: 1},
		Alignment: &excelize.Alignment{
			Horizontal: "center", Vertical: "center",
		},
		Border: fullBorder("17120F", 2),
	})
	moneyStyle, _ := xf.NewStyle(&excelize.Style{
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"FFFAF6"}, Pattern: 1},
		Font:         &excelize.Font{Color: "342B27", Size: 10},
		CustomNumFmt: stringPtr(`"Rp" #,##0;[Red]-"Rp" #,##0`),
		Alignment:    &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		Border:       fullBorder("D8C8BE", 1),
	})
	altMoneyStyle, _ := xf.NewStyle(&excelize.Style{
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"F6EEE8"}, Pattern: 1},
		Font:         &excelize.Font{Color: "342B27", Size: 10},
		CustomNumFmt: stringPtr(`"Rp" #,##0;[Red]-"Rp" #,##0`),
		Alignment:    &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		Border:       fullBorder("D8C8BE", 1),
	})
	dateStyle, _ := xf.NewStyle(&excelize.Style{
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"FFFAF6"}, Pattern: 1},
		Font:         &excelize.Font{Color: "342B27", Size: 10},
		CustomNumFmt: stringPtr("dd mmm yyyy hh:mm"),
		Alignment:    &excelize.Alignment{Vertical: "center"},
		Border:       fullBorder("D8C8BE", 1),
	})
	altDateStyle, _ := xf.NewStyle(&excelize.Style{
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"F6EEE8"}, Pattern: 1},
		Font:         &excelize.Font{Color: "342B27", Size: 10},
		CustomNumFmt: stringPtr("dd mmm yyyy hh:mm"),
		Alignment:    &excelize.Alignment{Vertical: "center"},
		Border:       fullBorder("D8C8BE", 1),
	})
	bodyStyle, _ := xf.NewStyle(&excelize.Style{
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"FFFAF6"}, Pattern: 1},
		Font:      &excelize.Font{Color: "342B27", Size: 10},
		Alignment: &excelize.Alignment{Vertical: "center"},
		Border:    fullBorder("D8C8BE", 1),
	})
	altBodyStyle, _ := xf.NewStyle(&excelize.Style{
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"F6EEE8"}, Pattern: 1},
		Font:      &excelize.Font{Color: "342B27", Size: 10},
		Alignment: &excelize.Alignment{Vertical: "center"},
		Border:    fullBorder("D8C8BE", 1),
	})
	totalLabelStyle, _ := xf.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "17120F"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"FDDF82"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		Border:    fullBorder("17120F", 2),
	})
	totalMoneyStyle, _ := xf.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Color: "17120F", Size: 12},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"FDDF82"}, Pattern: 1},
		CustomNumFmt: stringPtr(`"Rp" #,##0;[Red]-"Rp" #,##0`),
		Alignment:    &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		Border:       fullBorder("17120F", 2),
	})

	_ = xf.MergeCell(sheet, "B1", "H2")
	_ = xf.SetCellValue(sheet, "B1", "SAKU · LAPORAN TRANSAKSI")
	_ = xf.SetCellStyle(sheet, "B1", "H2", titleStyle)
	_ = xf.MergeCell(sheet, "B3", "H3")
	_ = xf.SetCellValue(sheet, "B3", exportSubtitle(filter, len(rows)))
	_ = xf.SetCellStyle(sheet, "B3", "H3", subtitleStyle)

	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].TransactionDate.Before(rows[j].TransactionDate)
	})
	var totalIncome, totalExpense float64
	for _, txn := range rows {
		if txn.Type == domain.TxnTypeIncome {
			totalIncome += txn.Amount
		} else {
			totalExpense += txn.Amount
		}
	}

	_ = xf.MergeCell(sheet, "B5", "C5")
	_ = xf.MergeCell(sheet, "B6", "C7")
	_ = xf.MergeCell(sheet, "D5", "E5")
	_ = xf.MergeCell(sheet, "D6", "E7")
	_ = xf.MergeCell(sheet, "F5", "H5")
	_ = xf.MergeCell(sheet, "F6", "H7")
	_ = xf.SetCellValue(sheet, "B5", "TOTAL PEMASUKAN")
	_ = xf.SetCellValue(sheet, "B6", totalIncome)
	_ = xf.SetCellValue(sheet, "D5", "TOTAL PENGELUARAN")
	_ = xf.SetCellValue(sheet, "D6", totalExpense)
	_ = xf.SetCellValue(sheet, "F5", "NET CASHFLOW")
	_ = xf.SetCellValue(sheet, "F6", totalIncome-totalExpense)
	_ = xf.SetCellStyle(sheet, "B5", "C5", summaryLabelStyle)
	_ = xf.SetCellStyle(sheet, "D5", "E5", summaryLabelStyle)
	_ = xf.SetCellStyle(sheet, "F5", "H5", summaryLabelStyle)
	_ = xf.SetCellStyle(sheet, "B6", "C7", summaryIncomeStyle)
	_ = xf.SetCellStyle(sheet, "D6", "E7", summaryExpenseStyle)
	_ = xf.SetCellStyle(sheet, "F6", "H7", summaryNetStyle)

	headers := []string{"Tanggal", "Tipe", "Kategori", "Dompet", "Deskripsi", "Merchant", "Jumlah"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+2, 9)
		_ = xf.SetCellValue(sheet, cell, h)
	}
	_ = xf.SetCellStyle(sheet, "B9", "H9", hdrStyle)
	_ = xf.SetRowHeight(sheet, 9, 30)

	for i, t := range rows {
		r := i + 10
		rowStyle := bodyStyle
		rowDateStyle := dateStyle
		rowMoneyStyle := moneyStyle
		if i%2 == 1 {
			rowStyle = altBodyStyle
			rowDateStyle = altDateStyle
			rowMoneyStyle = altMoneyStyle
		}
		_ = xf.SetCellStyle(sheet, fmt.Sprintf("B%d", r), fmt.Sprintf("H%d", r), rowStyle)
		_ = xf.SetCellValue(sheet, fmt.Sprintf("B%d", r), t.TransactionDate)
		_ = xf.SetCellStyle(sheet, fmt.Sprintf("B%d", r), fmt.Sprintf("B%d", r), rowDateStyle)

		typeLabel := "Pengeluaran"
		if t.Type == domain.TxnTypeIncome {
			typeLabel = "Pemasukan"
		}
		_ = xf.SetCellValue(sheet, fmt.Sprintf("C%d", r), typeLabel)
		_ = xf.SetCellValue(sheet, fmt.Sprintf("D%d", r), catMap[t.CategoryID])
		_ = xf.SetCellValue(sheet, fmt.Sprintf("E%d", r), walletMap[t.WalletID])
		_ = xf.SetCellValue(sheet, fmt.Sprintf("F%d", r), t.Description)
		_ = xf.SetCellValue(sheet, fmt.Sprintf("G%d", r), t.MerchantName)
		_ = xf.SetCellValue(sheet, fmt.Sprintf("H%d", r), t.Amount)
		_ = xf.SetCellStyle(sheet, fmt.Sprintf("H%d", r), fmt.Sprintf("H%d", r), rowMoneyStyle)
		_ = xf.SetRowHeight(sheet, r, 24)
	}

	dataEnd := len(rows) + 9
	if dataEnd < 10 {
		dataEnd = 10
		_ = xf.SetCellValue(sheet, "B10", "Belum ada transaksi pada periode ini")
		_ = xf.MergeCell(sheet, "B10", "H10")
		_ = xf.SetCellStyle(sheet, "B10", "H10", bodyStyle)
	}
	_ = xf.AutoFilter(sheet, fmt.Sprintf("B9:H%d", dataEnd), []excelize.AutoFilterOptions{})

	totalRow := dataEnd + 2
	_ = xf.MergeCell(sheet, fmt.Sprintf("B%d", totalRow), fmt.Sprintf("G%d", totalRow))
	_ = xf.SetCellValue(sheet, fmt.Sprintf("B%d", totalRow), "TOTAL NET CASHFLOW")
	_ = xf.SetCellValue(sheet, fmt.Sprintf("H%d", totalRow), totalIncome-totalExpense)
	_ = xf.SetCellStyle(sheet, fmt.Sprintf("B%d", totalRow), fmt.Sprintf("G%d", totalRow), totalLabelStyle)
	_ = xf.SetCellStyle(sheet, fmt.Sprintf("H%d", totalRow), fmt.Sprintf("H%d", totalRow), totalMoneyStyle)
	_ = xf.SetRowHeight(sheet, totalRow, 30)

	widths := map[string]float64{"A": 3, "B": 20, "C": 14, "D": 22, "E": 22, "F": 40, "G": 22, "H": 16, "I": 3}
	for col, w := range widths {
		_ = xf.SetColWidth(sheet, col, col, w)
	}
	_ = xf.SetRowHeight(sheet, 1, 28)
	_ = xf.SetRowHeight(sheet, 2, 22)
	_ = xf.SetRowHeight(sheet, 3, 22)
	showGridLines := false
	zoom := 90.0
	_ = xf.SetSheetView(sheet, 0, &excelize.ViewOptions{ShowGridLines: &showGridLines, ZoomScale: &zoom})
	_ = xf.SetPanes(sheet, &excelize.Panes{
		Freeze: true, YSplit: 9, TopLeftCell: "B10", ActivePane: "bottomLeft",
		Selection: []excelize.Selection{{SQRef: "B10", ActiveCell: "B10", Pane: "bottomLeft"}},
	})
	_ = xf.SetPageLayout(sheet, &excelize.PageLayoutOptions{
		Orientation: stringPtr("landscape"),
		Size:        intPtr(9),
		FitToWidth:  intPtr(1),
	})
	horizontal := true
	_ = xf.SetPageMargins(sheet, &excelize.PageLayoutMarginsOptions{
		Left: float64Ptr(0.35), Right: float64Ptr(0.35), Top: float64Ptr(0.5), Bottom: float64Ptr(0.5),
		Header: float64Ptr(0.2), Footer: float64Ptr(0.2), Horizontally: &horizontal,
	})
	_ = xf.SetSheetProps(sheet, &excelize.SheetPropsOptions{FitToPage: boolPtr(true)})
	_ = xf.SetHeaderFooter(sheet, &excelize.HeaderFooterOptions{
		AlignWithMargins: &horizontal,
		DifferentFirst:   false,
		OddFooter:        "&CSAKU Finance · &P / &N",
	})
	return nil
}

func exportSubtitle(filter ExportFilter, count int) string {
	period := "Semua periode"
	if filter.From != nil && filter.To != nil {
		period = fmt.Sprintf("%s – %s", filter.From.Format("02 Jan 2006"), filter.To.Format("02 Jan 2006"))
	} else if filter.From != nil {
		period = "Mulai " + filter.From.Format("02 Jan 2006")
	} else if filter.To != nil {
		period = "Sampai " + filter.To.Format("02 Jan 2006")
	}
	return fmt.Sprintf("%s  •  %d transaksi  •  Dibuat %s", period, count, time.Now().Format("02 Jan 2006 15:04"))
}

func stringPtr(value string) *string    { return &value }
func intPtr(value int) *int             { return &value }
func float64Ptr(value float64) *float64 { return &value }
func boolPtr(value bool) *bool          { return &value }

func fullBorder(color string, style int) []excelize.Border {
	return []excelize.Border{
		{Type: "left", Color: color, Style: style},
		{Type: "right", Color: color, Style: style},
		{Type: "top", Color: color, Style: style},
		{Type: "bottom", Color: color, Style: style},
	}
}
