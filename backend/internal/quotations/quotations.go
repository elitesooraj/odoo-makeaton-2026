// Package quotations owns quotation templates: reusable line sets that expand
// into a subscription in QUOTATION status. A quotation itself lives in the
// subscriptions module; this package only holds templates and the expansion.
package quotations

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/brightloop/subscription-platform/backend/internal/audit"
	"github.com/brightloop/subscription-platform/backend/internal/auth"
	"github.com/brightloop/subscription-platform/backend/internal/platform/db"
	"github.com/brightloop/subscription-platform/backend/internal/platform/httpx"
	"github.com/brightloop/subscription-platform/backend/internal/platform/id"
	"github.com/brightloop/subscription-platform/backend/internal/platform/pagination"
	"github.com/brightloop/subscription-platform/backend/internal/subscriptions"
)

var ErrNotFound = errors.New("quotation template not found")

type Template struct {
	ID           uuid.UUID      `json:"id"`
	Name         string         `json:"name"`
	ValidityDays int            `json:"validity_days"`
	Notes        *string        `json:"notes"`
	IsActive     bool           `json:"is_active"`
	Lines        []TemplateLine `json:"lines,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type TemplateLine struct {
	ID         uuid.UUID `json:"id"`
	TemplateID uuid.UUID `json:"template_id"`
	PlanID     uuid.UUID `json:"plan_id"`
	Quantity   int       `json:"quantity"`
	Position   int       `json:"position"`
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const columns = `id, name, validity_days, notes, is_active, created_at, updated_at`

func (r *Repository) Insert(ctx context.Context, q db.Querier, t *Template) error {
	_, err := q.Exec(ctx, `INSERT INTO quotation_templates (id, name, validity_days, notes)
		VALUES ($1, $2, $3, $4)`, t.ID, t.Name, t.ValidityDays, id.StringArg(t.Notes))
	return err
}

func (r *Repository) Get(ctx context.Context, q db.Querier, templateID uuid.UUID) (*Template, error) {
	return scan(q.QueryRow(ctx, `SELECT `+columns+` FROM quotation_templates WHERE id = $1`, templateID))
}

func (r *Repository) List(ctx context.Context, search string, activeOnly bool, page pagination.Page) ([]Template, int, error) {
	where := `WHERE ($1 = '' OR lower(name) LIKE '%' || lower($1) || '%')
		AND (NOT $2 OR is_active)`

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM quotation_templates `+where, search, activeOnly).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `SELECT `+columns+` FROM quotation_templates `+where+`
		ORDER BY lower(name), id LIMIT $3 OFFSET $4`, search, activeOnly, page.PerPage, page.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]Template, 0)
	for rows.Next() {
		t, err := scan(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *t)
	}
	return out, total, rows.Err()
}

func (r *Repository) Update(ctx context.Context, q db.Querier, t *Template) error {
	tag, err := q.Exec(ctx, `UPDATE quotation_templates SET name=$2, validity_days=$3,
		notes=$4, is_active=$5 WHERE id=$1`,
		t.ID, t.Name, t.ValidityDays, id.StringArg(t.Notes), t.IsActive)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) ReplaceLines(ctx context.Context, q db.Querier, templateID uuid.UUID, lines []TemplateLine) error {
	if _, err := q.Exec(ctx, `DELETE FROM quotation_template_lines WHERE template_id = $1`, templateID); err != nil {
		return err
	}
	for i := range lines {
		if _, err := q.Exec(ctx, `INSERT INTO quotation_template_lines
			(id, template_id, plan_id, quantity, position) VALUES ($1, $2, $3, $4, $5)`,
			id.New(), templateID, lines[i].PlanID, lines[i].Quantity, lines[i].Position); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) Lines(ctx context.Context, q db.Querier, templateID uuid.UUID) ([]TemplateLine, error) {
	rows, err := q.Query(ctx, `SELECT id, template_id, plan_id, quantity, position
		FROM quotation_template_lines WHERE template_id = $1 ORDER BY position, id`, templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]TemplateLine, 0)
	for rows.Next() {
		var l TemplateLine
		if err := rows.Scan(&l.ID, &l.TemplateID, &l.PlanID, &l.Quantity, &l.Position); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// PlanLoader is satisfied by plans.Service via the adapter in main and returns
// the neutral subscriptions.PlanSpec view.
type PlanLoader interface {
	Get(ctx context.Context, planID uuid.UUID) (*subscriptions.PlanSpec, error)
}

type Service struct {
	repo  *Repository
	audit *audit.Recorder
	subs  *subscriptions.Service
	plans PlanLoader
}

func NewService(pool *pgxpool.Pool, recorder *audit.Recorder, subs *subscriptions.Service, plans PlanLoader) *Service {
	return &Service{repo: NewRepository(pool), audit: recorder, subs: subs, plans: plans}
}

type LineInput struct {
	PlanID   string `json:"plan_id"`
	Quantity int    `json:"quantity"`
}

type Input struct {
	Name         string      `json:"name"`
	ValidityDays int         `json:"validity_days"`
	Notes        string      `json:"notes"`
	Lines        []LineInput `json:"lines"`
}

type UpdateInput struct {
	Name         *string      `json:"name"`
	ValidityDays *int         `json:"validity_days"`
	Notes        string       `json:"notes"`
	Lines        *[]LineInput `json:"lines"`
	IsActive     *bool        `json:"is_active"`
}

func (s *Service) Create(ctx context.Context, actor *auth.Actor, meta audit.Meta, in Input) (*Template, error) {
	t, lines, details := validate("", inName(in.Name), in.ValidityDays, inName(in.Notes), in.Lines)
	if len(details) > 0 {
		return nil, httpx.Validation(details)
	}
	if err := s.repo.Insert(ctx, s.repo.pool, t); err != nil {
		return nil, err
	}
	if err := s.repo.ReplaceLines(ctx, s.repo.pool, t.ID, lines); err != nil {
		return nil, err
	}
	return s.Get(ctx, t.ID)
}

func inName(v string) string { return strings.TrimSpace(v) }

func validate(_ string, name string, validityDays int, notes string, rawLines []LineInput) (*Template, []TemplateLine, map[string]string) {
	details := map[string]string{}
	if name == "" {
		details["name"] = "template name is required"
	}
	validity := validityDays
	if validity == 0 {
		validity = 30
	}
	if validity < 1 {
		details["validity_days"] = "must be at least 1"
	}
	lines := make([]TemplateLine, 0, len(rawLines))
	for i, in := range rawLines {
		planID, err := uuid.Parse(strings.TrimSpace(in.PlanID))
		if err != nil {
			details["lines.plan_id"] = "line " + itoa(i) + ": a valid plan id is required"
			continue
		}
		quantity := in.Quantity
		if quantity == 0 {
			quantity = 1
		}
		if quantity < 1 {
			details["lines.quantity"] = "line " + itoa(i) + ": quantity must be at least 1"
			continue
		}
		lines = append(lines, TemplateLine{TemplateID: uuid.Nil, PlanID: planID, Quantity: quantity, Position: i})
	}
	t := &Template{
		ID: id.New(), Name: name, ValidityDays: validity, IsActive: true,
	}
	if notes != "" {
		t.Notes = &notes
	}
	return t, lines, details
}

func (s *Service) Get(ctx context.Context, templateID uuid.UUID) (*Template, error) {
	t, err := s.repo.Get(ctx, s.repo.pool, templateID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, httpx.NotFound("quotation template not found")
		}
		return nil, err
	}
	lines, err := s.repo.Lines(ctx, s.repo.pool, templateID)
	if err != nil {
		return nil, err
	}
	t.Lines = lines
	return t, nil
}

func (s *Service) List(ctx context.Context, search string, activeOnly bool, page pagination.Page) ([]Template, int, error) {
	return s.repo.List(ctx, strings.TrimSpace(search), activeOnly, page)
}

func (s *Service) Update(ctx context.Context, actor *auth.Actor, meta audit.Meta, templateID uuid.UUID, in UpdateInput) (*Template, error) {
	current, err := s.Get(ctx, templateID)
	if err != nil {
		return nil, err
	}
	before := *current
	name := current.Name
	validity := current.ValidityDays
	notes := ""
	if current.Notes != nil {
		notes = *current.Notes
	}
	var rawLines []LineInput
	if in.Name != nil {
		name = strings.TrimSpace(*in.Name)
	}
	if in.ValidityDays != nil {
		validity = *in.ValidityDays
	}
	if in.Lines != nil {
		rawLines = *in.Lines
	}

	t, lines, details := validate("", name, validity, notes, rawLines)
	if len(details) > 0 {
		return nil, httpx.Validation(details)
	}
	t.ID = templateID
	t.IsActive = current.IsActive
	if in.Notes != "" {
		t.Notes = ptrNotes(in.Notes)
	} else {
		t.Notes = nil
	}
	if in.IsActive != nil {
		t.IsActive = *in.IsActive
	}
	if in.Lines != nil {
		// lines set from the validated collection
		if len(lines) == 0 {
			details["lines"] = "a template needs at least one line"
			return nil, httpx.Validation(details)
		}
	}

	err = s.replaceTemplate(ctx, actor, meta, templateID, before, t, lines, in.Lines != nil)
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, templateID)
}

// replaceTemplate writes the template inside one transaction and audits it.
func (s *Service) replaceTemplate(ctx context.Context, actor *auth.Actor, meta audit.Meta,
	templateID uuid.UUID, before Template, t *Template, lines []TemplateLine, replaceLines bool) error {
	return db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if err := s.repo.Update(ctx, tx, t); err != nil {
			return err
		}
		if replaceLines {
			if err := s.repo.ReplaceLines(ctx, tx, templateID, lines); err != nil {
				return err
			}
		}
		after := *t
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: uid(actor), ActorRole: role(actor),
			Action: "quotation_template.update", EntityType: "quotation_template",
			EntityID: templateID, Before: before, After: &after, Meta: meta,
		})
	})
}

func (s *Service) Deactivate(ctx context.Context, actor *auth.Actor, meta audit.Meta, templateID uuid.UUID) (*Template, error) {
	inactive := false
	return s.Update(ctx, actor, meta, templateID, UpdateInput{IsActive: &inactive})
}

type InstantiateInput struct {
	CustomerID       string  `json:"customer_id"`
	StartDate        *string `json:"start_date"`
	Quantity         int     `json:"quantity"`
	PaymentTermsDays int     `json:"payment_terms_days"`
	DiscountID       *string `json:"discount_id"`
	Notes            string  `json:"notes"`
}

// Instantiate expands the template into a subscription in QUOTATION status.
// Line prices and taxes come from the plan; the quote expiry comes from the
// template validity window.
func (s *Service) Instantiate(ctx context.Context, actor *auth.Actor, meta audit.Meta, templateID uuid.UUID, in InstantiateInput) (*subscriptions.Subscription, error) {
	template, err := s.Get(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if !template.IsActive {
		return nil, httpx.Conflict("this quotation template is not active")
	}
	if len(template.Lines) == 0 {
		return nil, httpx.Conflict("this quotation template has no lines")
	}

	details := map[string]string{}
	if _, err := uuid.Parse(strings.TrimSpace(in.CustomerID)); err != nil {
		details["customer_id"] = "a valid customer id is required"
	}
	// Validate plan availability and resolve pricing up front.
	lineInputs := make([]subscriptions.LineInput, 0, len(template.Lines))
	for _, line := range template.Lines {
		plan, err := s.plans.Get(ctx, line.PlanID)
		if err != nil {
			return nil, err
		}
		if !plan.IsActive {
			details["plan_id"] = "plan " + plan.Name + " is not active"
		}
		lineInputs = append(lineInputs, subscriptions.LineInput{
			ProductID:      plan.ProductID.String(),
			Description:    plan.Name,
			Quantity:       line.Quantity,
			UnitPriceCents: plan.PriceCents,
		})
		if plan.TaxID != nil {
			taxID := plan.TaxID.String()
			lineInputs[len(lineInputs)-1].TaxID = &taxID
		}
	}
	if len(details) > 0 {
		return nil, httpx.Validation(details)
	}

	quoteExpires := time.Now().UTC().AddDate(0, 0, template.ValidityDays)
	// The subscription's plan is taken from the template's first line. Templates
	// that mix plans are not supported by this thin implementation; the module
	// owner can relax this if mixed plans become a requirement.
	input := subscriptions.CreateInput{
		CustomerID:       strings.TrimSpace(in.CustomerID),
		PlanID:           template.Lines[0].PlanID.String(),
		Status:           subscriptions.StatusQuotation,
		PaymentTermsDays: in.PaymentTermsDays,
		Quantity:         in.Quantity,
		DiscountID:       in.DiscountID,
		QuoteExpiresAt:   ptrRFC3339(quoteExpires),
		Notes:            in.Notes,
		Lines:            lineInputs,
	}
	if in.StartDate != nil && strings.TrimSpace(*in.StartDate) != "" {
		input.StartDate = strings.TrimSpace(*in.StartDate)
	} else {
		input.StartDate = time.Now().UTC().Format("2006-01-02")
	}
	return s.subs.Create(ctx, actor, meta, input)
}

// Handlers ---------------------------------------------------------------

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Create(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	meta := auditMeta(c)
	var in Input
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	t, err := h.svc.Create(c.Request.Context(), actor, meta, in)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, t)
}

func (h *Handler) List(c *gin.Context) {
	page := pagination.Parse(c)
	items, total, err := h.svc.List(c.Request.Context(), c.Query("search"), c.Query("active") == "true", page)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.List(c, items, pagination.Meta(page, total))
}

func (h *Handler) Get(c *gin.Context) {
	templateID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.BadRequest("invalid template id"))
		return
	}
	t, err := h.svc.Get(c.Request.Context(), templateID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, t)
}

func (h *Handler) Update(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	meta := auditMeta(c)
	templateID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.BadRequest("invalid template id"))
		return
	}
	var in UpdateInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	t, err := h.svc.Update(c.Request.Context(), actor, meta, templateID, in)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, t)
}

func (h *Handler) Deactivate(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	meta := auditMeta(c)
	templateID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.BadRequest("invalid template id"))
		return
	}
	t, err := h.svc.Deactivate(c.Request.Context(), actor, meta, templateID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, t)
}

func (h *Handler) Instantiate(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	meta := auditMeta(c)
	templateID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.BadRequest("invalid template id"))
		return
	}
	var in InstantiateInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	sub, err := h.svc.Instantiate(c.Request.Context(), actor, meta, templateID, in)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, sub)
}

func auditMeta(c *gin.Context) audit.Meta {
	meta := httpx.ClientMetaFrom(c)
	return audit.Meta{IP: meta.IP, UserAgent: meta.UserAgent, RequestID: meta.RequestID}
}

func uid(actor *auth.Actor) *uuid.UUID {
	if actor == nil {
		return nil
	}
	return &actor.UserID
}

func role(actor *auth.Actor) string {
	if actor == nil {
		return ""
	}
	return actor.Role
}

func ptrNotes(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func ptrRFC3339(t time.Time) *string {
	v := t.Format(time.RFC3339)
	return &v
}

func itoa(n int) string { return strconv.Itoa(n) }

func scan(row pgx.Row) (*Template, error) {
	var (
		t     Template
		notes pgtype.Text
	)
	err := row.Scan(&t.ID, &t.Name, &t.ValidityDays, &notes, &t.IsActive, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if notes.Valid {
		v := notes.String
		t.Notes = &v
	}
	return &t, nil
}
