package dto

type APIResponse struct {
	Status  string          `json:"status" example:"success"`
	Code    int             `json:"code" example:"200"`
	Message string          `json:"message" example:"Success"`
	Data    interface{}     `json:"data,omitempty"`
	Meta    *PaginationMeta `json:"meta,omitempty"`
}

type PaginationMeta struct {
	Page        int   `json:"page" example:"1"`
	Limit       int   `json:"limit" example:"10"`
	Total       int64 `json:"total" example:"100"`
	TotalPages  int   `json:"total_pages" example:"10"`
	HasNext     bool  `json:"has_next" example:"true"`
	HasPrevious bool  `json:"has_previous" example:"false"`
}

func NewMeta(page, limit int, total int64) *PaginationMeta {
	totalPages := 0
	if limit > 0 {
		totalPages = int(total) / limit
		if int(total)%limit > 0 {
			totalPages++
		}
	}
	return &PaginationMeta{
		Page:        page,
		Limit:       limit,
		Total:       total,
		TotalPages:  totalPages,
		HasNext:     page < totalPages,
		HasPrevious: page > 1,
	}
}
