package audit

import (
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/ganiramadhan/starter-go/pkg/httpx"
	"github.com/gofiber/fiber/v2"
)

type Handler struct{ service Service }

func NewHandler(s Service) *Handler { return &Handler{service: s} }

// List godoc
// @Summary  List audit log entries (admin only)
// @Tags     Audit
// @Produce  json
// @Param    table       query string false "Table name, e.g. wallets, transactions"
// @Param    record_id   query string false "Audited row's primary key"
// @Param    actor_id    query string false "Application user UUID"
// @Param    request_id  query string false "Correlate every row one HTTP request touched"
// @Param    action      query string false "INSERT | UPDATE | DELETE"
// @Param    direct_only query bool   false "Only changes made outside the API (direct SQL)"
// @Param    from        query string false "RFC3339 or YYYY-MM-DD"
// @Param    to          query string false "RFC3339 or YYYY-MM-DD"
// @Param    page        query int    false "Page (default 1)"
// @Param    limit       query int    false "Page size (default 20, max 200)"
// @Success  200 {object} dto.APIResponse{data=[]dto.AuditLogResponse}
// @Security BearerAuth
// @Router   /api/v1/admin/audit-logs [get]
func (h *Handler) List(c *fiber.Ctx) error {
	var q dto.AuditLogListQuery
	if err := c.QueryParser(&q); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid query parameters")
	}
	if q.Page == 0 {
		q.Page = c.QueryInt("page", 1)
	}
	if q.Limit == 0 {
		q.Limit = c.QueryInt("limit", 20)
	}
	out, meta, err := h.service.List(c.Context(), q)
	if err != nil {
		return err
	}
	return httpx.List(c, "Audit logs retrieved", out, meta)
}

// Get godoc
// @Summary  Get one audit log entry (admin only)
// @Tags     Audit
// @Produce  json
// @Param    id path string true "Audit log UUID"
// @Success  200 {object} dto.APIResponse{data=dto.AuditLogResponse}
// @Security BearerAuth
// @Router   /api/v1/admin/audit-logs/{id} [get]
func (h *Handler) Get(c *fiber.Ctx) error {
	id, err := httpx.ParseUUID(c, "id")
	if err != nil {
		return err
	}
	out, err := h.service.Get(c.Context(), id)
	if err != nil {
		return err
	}
	return httpx.OK(c, "Audit log retrieved", out)
}

// ForRecord godoc
// @Summary  Full change history of one row (admin only)
// @Tags     Audit
// @Produce  json
// @Param    table     path string true "Table name, e.g. wallets"
// @Param    recordId  path string true "Row primary key"
// @Param    page      query int false "Page (default 1)"
// @Param    limit     query int false "Page size (default 20, max 200)"
// @Success  200 {object} dto.APIResponse{data=[]dto.AuditLogResponse}
// @Security BearerAuth
// @Router   /api/v1/admin/audit-logs/records/{table}/{recordId} [get]
func (h *Handler) ForRecord(c *fiber.Ctx) error {
	table := c.Params("table")
	recordID := c.Params("recordId")
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	out, meta, err := h.service.ForRecord(c.Context(), table, recordID, page, limit)
	if err != nil {
		return err
	}
	return httpx.List(c, "Audit log history retrieved", out, meta)
}
