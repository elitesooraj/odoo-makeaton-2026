package customers

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/brightloop/subscription-platform/backend/internal/audit"
	"github.com/brightloop/subscription-platform/backend/internal/auth"
	"github.com/brightloop/subscription-platform/backend/internal/platform/db"
	"github.com/brightloop/subscription-platform/backend/internal/platform/httpx"
	"github.com/brightloop/subscription-platform/backend/internal/platform/id"
	"github.com/brightloop/subscription-platform/backend/internal/platform/pagination"
	"github.com/brightloop/subscription-platform/backend/internal/platform/validate"
)

type CreateInput struct {
	Name           string          `json:"name"`
	Email          string          `json:"email"`
	Phone          string          `json:"phone"`
	BillingAddress json.RawMessage `json:"billing_address"`
	TaxID          string          `json:"tax_id"`
	Notes          string          `json:"notes"`
}

type UpdateInput struct {
	Name           *string         `json:"name"`
	Email          *string         `json:"email"`
	Phone          *string         `json:"phone"`
	BillingAddress json.RawMessage `json:"billing_address"`
	TaxID          *string         `json:"tax_id"`
	Notes          *string         `json:"notes"`
	Status         *string         `json:"status"`
}

type Service struct {
	repo  *Repository
	audit *audit.Recorder
}

func NewService(repo *Repository, recorder *audit.Recorder) *Service {
	return &Service{repo: repo, audit: recorder}
}

// CreateForRegistration is called by auth inside the registration transaction.
func (s *Service) CreateForRegistration(ctx context.Context, q db.Querier, name, email string) (uuid.UUID, error) {
	c := &Customer{
		ID:             id.New(),
		Name:           name,
		Email:          &email,
		Status:         StatusActive,
		BillingAddress: json.RawMessage(`{}`),
	}
	if err := s.repo.Create(ctx, q, c); err != nil {
		return uuid.Nil, err
	}
	return c.ID, nil
}

func (s *Service) Create(ctx context.Context, actor *auth.Actor, meta audit.Meta, in CreateInput) (*Customer, error) {
	name := strings.TrimSpace(in.Name)
	details := map[string]string{}
	if name == "" {
		details["name"] = "customer name is required"
	}
	email := strings.TrimSpace(in.Email)
	if email != "" && !validate.Email(email) {
		details["email"] = "enter a valid email address"
	}
	if len(details) > 0 {
		return nil, httpx.Validation(details)
	}

	c := &Customer{
		ID:             id.New(),
		Name:           name,
		Email:          optional(email),
		Phone:          optional(strings.TrimSpace(in.Phone)),
		BillingAddress: normalizeAddress(in.BillingAddress),
		TaxID:          optional(strings.TrimSpace(in.TaxID)),
		Status:         StatusActive,
		Notes:          optional(strings.TrimSpace(in.Notes)),
	}

	var stored *Customer
	err := db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if err := s.repo.Create(ctx, tx, c); err != nil {
			return err
		}
		created, err := s.repo.Get(ctx, tx, c.ID)
		if err != nil {
			return err
		}
		stored = created
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID:    &actor.UserID,
			ActorRole:  actor.Role,
			Action:     "customer.create",
			EntityType: "customer",
			EntityID:   created.ID,
			After:      created,
			Meta:       meta,
		})
	})
	if err != nil {
		return nil, err
	}
	return stored, nil
}

func (s *Service) Update(ctx context.Context, actor *auth.Actor, meta audit.Meta, id uuid.UUID, in UpdateInput) (*Customer, error) {
	current, err := s.repo.Get(ctx, s.repo.pool, id)
	if err != nil {
		return nil, httpError(err)
	}

	before := *current
	details := map[string]string{}

	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			details["name"] = "customer name is required"
		}
		current.Name = name
	}
	if in.Email != nil {
		email := strings.TrimSpace(*in.Email)
		if email != "" && !validate.Email(email) {
			details["email"] = "enter a valid email address"
		}
		current.Email = optional(email)
	}
	if in.Phone != nil {
		current.Phone = optional(strings.TrimSpace(*in.Phone))
	}
	if in.TaxID != nil {
		current.TaxID = optional(strings.TrimSpace(*in.TaxID))
	}
	if in.Notes != nil {
		current.Notes = optional(strings.TrimSpace(*in.Notes))
	}
	if len(in.BillingAddress) > 0 {
		current.BillingAddress = normalizeAddress(in.BillingAddress)
	}
	if in.Status != nil {
		status := strings.ToUpper(strings.TrimSpace(*in.Status))
		if status != StatusActive && status != StatusInactive {
			details["status"] = "status must be ACTIVE or INACTIVE"
		}
		current.Status = status
	}
	if len(details) > 0 {
		return nil, httpx.Validation(details)
	}

	var stored *Customer
	err = db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if err := s.repo.Update(ctx, tx, current); err != nil {
			return err
		}
		updated, err := s.repo.Get(ctx, tx, current.ID)
		if err != nil {
			return err
		}
		stored = updated
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID:    &actor.UserID,
			ActorRole:  actor.Role,
			Action:     "customer.update",
			EntityType: "customer",
			EntityID:   updated.ID,
			Before:     before,
			After:      updated,
			Meta:       meta,
		})
	})
	if err != nil {
		return nil, err
	}
	return stored, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Customer, error) {
	c, err := s.repo.Get(ctx, s.repo.pool, id)
	if err != nil {
		return nil, httpError(err)
	}
	return c, nil
}

func (s *Service) List(ctx context.Context, f Filter) ([]Customer, int, error) {
	return s.repo.List(ctx, f)
}

// Deactivate is the only delete path: customers are never removed because
// subscriptions and invoices reference them forever.
func (s *Service) Deactivate(ctx context.Context, actor *auth.Actor, meta audit.Meta, id uuid.UUID) (*Customer, error) {
	current, err := s.repo.Get(ctx, s.repo.pool, id)
	if err != nil {
		return nil, httpError(err)
	}
	if current.Status == StatusInactive {
		return current, nil
	}
	before := *current
	current.Status = StatusInactive

	var stored *Customer
	err = db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if err := s.repo.Update(ctx, tx, current); err != nil {
			return err
		}
		updated, err := s.repo.Get(ctx, tx, current.ID)
		if err != nil {
			return err
		}
		stored = updated
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID:    &actor.UserID,
			ActorRole:  actor.Role,
			Action:     "customer.deactivate",
			EntityType: "customer",
			EntityID:   updated.ID,
			Before:     before,
			After:      updated,
			Meta:       meta,
		})
	})
	if err != nil {
		return nil, err
	}
	return stored, nil
}

func httpError(err error) error {
	if err == ErrNotFound {
		return httpx.NotFound("customer not found")
	}
	return err
}

func optional(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func normalizeAddress(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage(`{}`)
	}
	return raw
}

// PageFilter builds the repository filter from query parameters.
func PageFilter(search, status string, page pagination.Page) Filter {
	return Filter{
		Search: strings.TrimSpace(search),
		Status: strings.ToUpper(strings.TrimSpace(status)),
		Page:   page,
	}
}
