package transaction

import (
	"testing"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

func TestBuildTransactionWorkbookCreatesStyledReportLayout(t *testing.T) {
	walletID := uuid.New()
	categoryID := uuid.New()
	rows := []domain.Transaction{
		{
			WalletID:        walletID,
			CategoryID:      categoryID,
			Amount:          1500000,
			Type:            domain.TxnTypeIncome,
			Description:     "Project payment",
			MerchantName:    "Client",
			TransactionDate: time.Date(2026, 6, 18, 9, 30, 0, 0, time.UTC),
		},
		{
			WalletID:        walletID,
			CategoryID:      categoryID,
			Amount:          500000,
			Type:            domain.TxnTypeExpense,
			Description:     "VPS Ganipedia",
			MerchantName:    "Sumopods",
			TransactionDate: time.Date(2026, 6, 19, 10, 0, 0, 0, time.UTC),
		},
	}
	xf := excelize.NewFile()
	defer func() { _ = xf.Close() }()

	if err := buildTransactionWorkbook(
		xf,
		rows,
		map[uuid.UUID]string{walletID: "Bank Jago"},
		map[uuid.UUID]string{categoryID: "Tagihan"},
		ExportFilter{},
	); err != nil {
		t.Fatalf("build workbook: %v", err)
	}

	assertCell := func(cell, want string) {
		t.Helper()
		got, err := xf.GetCellValue("Transaksi", cell)
		if err != nil {
			t.Fatalf("read %s: %v", cell, err)
		}
		if got != want {
			t.Fatalf("%s = %q, want %q", cell, got, want)
		}
	}
	assertCell("B1", "SAKU · LAPORAN TRANSAKSI")
	assertCell("B5", "TOTAL PEMASUKAN")
	assertCell("D5", "TOTAL PENGELUARAN")
	assertCell("F5", "NET CASHFLOW")
	assertCell("B9", "Tanggal")
	assertCell("G11", "Sumopods")
	assertCell("B13", "TOTAL NET CASHFLOW")

	titleStyle, err := xf.GetCellStyle("Transaksi", "B1")
	if err != nil || titleStyle == 0 {
		t.Fatalf("expected styled title, style=%d err=%v", titleStyle, err)
	}
	totalStyle, err := xf.GetCellStyle("Transaksi", "H13")
	if err != nil || totalStyle == 0 {
		t.Fatalf("expected highlighted total, style=%d err=%v", totalStyle, err)
	}
	bodyStyle, err := xf.GetCellStyle("Transaksi", "D10")
	if err != nil || bodyStyle == 0 {
		t.Fatalf("expected full-border body style, style=%d err=%v", bodyStyle, err)
	}
	bodyDefinition, err := xf.GetStyle(bodyStyle)
	if err != nil {
		t.Fatalf("read body style: %v", err)
	}
	if len(bodyDefinition.Border) != 4 {
		t.Fatalf("expected body cells to have four borders, got %#v", bodyDefinition.Border)
	}
	titleDefinition, err := xf.GetStyle(titleStyle)
	if err != nil {
		t.Fatalf("read title style: %v", err)
	}
	if titleDefinition.Alignment == nil || titleDefinition.Alignment.Horizontal != "center" {
		t.Fatalf("expected centered title alignment, got %#v", titleDefinition.Alignment)
	}

	path := t.TempDir() + "/transactions.xlsx"
	if err := xf.SaveAs(path); err != nil {
		t.Fatalf("save workbook: %v", err)
	}
	reopened, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("reopen generated workbook: %v", err)
	}
	_ = reopened.Close()
}
