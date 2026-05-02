package dto

import "testing"

func TestNewMeta(t *testing.T) {
	tests := []struct {
		name      string
		page      int
		limit     int
		total     int64
		wantPages int
		wantNext  bool
		wantPrev  bool
	}{
		{"empty", 1, 10, 0, 0, false, false},
		{"single page", 1, 10, 5, 1, false, false},
		{"exact multiple", 1, 10, 20, 2, true, false},
		{"middle page", 2, 10, 25, 3, true, true},
		{"last page", 3, 10, 25, 3, false, true},
		{"zero limit", 1, 0, 5, 0, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMeta(tt.page, tt.limit, tt.total)
			if m.TotalPages != tt.wantPages {
				t.Errorf("TotalPages = %d, want %d", m.TotalPages, tt.wantPages)
			}
			if m.HasNext != tt.wantNext {
				t.Errorf("HasNext = %v, want %v", m.HasNext, tt.wantNext)
			}
			if m.HasPrevious != tt.wantPrev {
				t.Errorf("HasPrevious = %v, want %v", m.HasPrevious, tt.wantPrev)
			}
			if m.Total != tt.total {
				t.Errorf("Total = %d, want %d", m.Total, tt.total)
			}
		})
	}
}
