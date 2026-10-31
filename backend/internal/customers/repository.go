package customers

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/brightloop/subscription-platform/backend/internal/platform/db"
	"github.com/brightloop/subscription-platform/backend/internal/platform/pagination"
)

const (
	StatusActive   = "ACTIVE"
	StatusInactive = "INACTIVE"
)

type Customer struct {
	ID             uuid.UUID       `json:"id"`
	Name           string          `json:"name"`
	Email          *string         `json:"email"`
	Phone          *string         `json:"phone"`
	BillingAddress json.RawMessage `json:"billing_address"`
	TaxID          *string         `json:"tax_id"`
	Status         string          `json:"status"`
	Notes          *string         `json:"notes"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type Filter struct {
	Search string
	Status string
	Page   pagination.Page
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const customerColumns = `id, name, email, phone, billing_address, tax_id, status, notes, created_at, updated_at`

func (r *Repository) Create(ctx context.Context, q db.Querier, c *Customer) error {
	_, err := q.Exec(ctx, `INSERT INTO customers
		(id, name, email, phone, billing_address, tax_id, status, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		c.ID, c.Name, textArg(c.Email), textArg(c.Phone), jsonArg(c.BillingAddress),
		textArg(c.TaxID), c.Status, textArg(c.Notes),
	)
	return err
}

func (r *Repository) Get(ctx context.Context, q db.Querier, id uuid.UUID) (*Customer, error) {
	return scanCustomer(q.QueryRow(ctx, `SELECT `+customerColumns+` FROM customers WHERE id = $1`, id))
}

func (r *Repository) Update(ctx context.Context, q db.Querier, c *Customer) error {
	_, err := q.Exec(ctx, `UPDATE customers SET
		name = $2, email = $3, phone = $4, billing_address = $5,
		tax_id = $6, status = $7, notes = $8
		WHERE id = $1`,
		c.ID, c.Name, textArg(c.Email), textArg(c.Phone), jsonArg(c.BillingAddress),
		textArg(c.TaxID), c.Status, textArg(c.Notes),
	)
	return err
}

func (r *Repository) List(ctx context.Context, f Filter) ([]Customer, int, error) {
	const where = `
		WHERE ($1 = '' OR lower(name) LIKE '%' || lower($1) || '%'
		       OR lower(coalesce(email, '')) LIKE '%' || lower($1) || '%')
		  AND ($2 = '' OR status = $2)`

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM customers`+where, f.Search, f.Status).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx, `SELECT `+customerColumns+` FROM customers`+where+`
		ORDER BY lower(name), id
		LIMIT $3 OFFSET $4`, f.Search, f.Status, f.Page.PerPage, f.Page.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	customers := make([]Customer, 0)
	for rows.Next() {
		c, err := scanCustomer(rows)
		if err != nil {
			return nil, 0, err
		}
		customers = append(customers, *c)
	}
	return customers, total, rows.Err()
}

func scanCustomer(row pgx.Row) (*Customer, error) {
	var (
		c     Customer
		email pgtype.Text
		phone pgtype.Text
		taxID pgtype.Text
		notes pgtype.Text
		addr  []byte
	)
	err := row.Scan(&c.ID, &c.Name, &email, &phone, &addr, &taxID, &c.Status, &notes, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if email.Valid {
		c.Email = &email.String
	}
	if phone.Valid {
		c.Phone = &phone.String
	}
	if taxID.Valid {
		c.TaxID = &taxID.String
	}
	if notes.Valid {
		c.Notes = &notes.String
	}
	if len(addr) == 0 {
		c.BillingAddress = json.RawMessage(`{}`)
	} else {
		c.BillingAddress = json.RawMessage(addr)
	}
	return &c, nil
}

func textArg(v *string) any {
	if v == nil || *v == "" {
		return nil
	}
	return *v
}

func jsonArg(v json.RawMessage) any {
	if len(v) == 0 {
		return "{}"
	}
	return string(v)
}

var ErrNotFound = errors.New("customer not found")
