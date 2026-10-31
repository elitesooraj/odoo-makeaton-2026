package subscriptions

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/brightloop/subscription-platform/backend/internal/platform/db"
	"github.com/brightloop/subscription-platform/backend/internal/platform/id"
	"github.com/brightloop/subscription-platform/backend/internal/platform/pagination"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const columns = `id, number, customer_id, plan_id, status, start_date, expiry_date,
	payment_terms_days, quantity, discount_id, next_billing_date, quote_expires_at,
	notes, created_by, activated_at, paused_at, closed_at, created_at, updated_at`

const lineColumns = `id, subscription_id, product_id, variant_id, description,
	quantity, unit_price_cents, tax_id, position`

// billingColumns is the SELECT list for ListForBilling: subscription columns
// plus the plan columns the billing worker needs, in one JOIN.
const billingColumns = `s.id, s.number, s.customer_id, s.plan_id, s.status, s.start_date, s.expiry_date,
	s.payment_terms_days, s.quantity, s.discount_id, s.next_billing_date, s.quote_expires_at,
	s.notes, s.created_by, s.activated_at, s.paused_at, s.closed_at, s.created_at, s.updated_at,
	p.id, p.product_id, p.tax_id, p.name, p.interval, p.price_cents, p.minimum_quantity,
	p.auto_close, p.auto_renew, p.allow_pause, p.allow_early_close, p.is_active`

func (r *Repository) Insert(ctx context.Context, q db.Querier, s *Subscription) error {
	_, err := q.Exec(ctx, `INSERT INTO subscriptions
		(id, number, customer_id, plan_id, status, start_date, expiry_date,
		 payment_terms_days, quantity, discount_id, next_billing_date, quote_expires_at,
		 notes, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		s.ID, s.Number, s.CustomerID, s.PlanID, s.Status, id.DateArg(&s.StartDate),
		id.DateArg(s.ExpiryDate), s.PaymentTermsDays, s.Quantity, id.UUIDArg(s.DiscountID),
		id.DateArg(s.NextBillingDate), s.QuoteExpiresAt, id.StringArg(s.Notes),
		id.UUIDArg(s.CreatedBy))
	return err
}

func (r *Repository) Get(ctx context.Context, q db.Querier, subscriptionID uuid.UUID) (*Subscription, error) {
	return scan(q.QueryRow(ctx, `SELECT `+columns+` FROM subscriptions WHERE id = $1`, subscriptionID))
}

func (r *Repository) List(ctx context.Context, f Filter) ([]Subscription, int, error) {
	where := `WHERE ($1::uuid IS NULL OR customer_id = $1)
		AND ($2 = '' OR status = $2)
		AND ($3 = '' OR number LIKE '%' || $3 || '%')`

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM subscriptions `+where,
		id.UUIDArg(f.CustomerID), f.Status, strings.TrimSpace(f.Search)).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `SELECT `+columns+` FROM subscriptions `+where+`
		ORDER BY created_at DESC, id LIMIT $4 OFFSET $5`,
		id.UUIDArg(f.CustomerID), f.Status, strings.TrimSpace(f.Search),
		f.Page.PerPage, f.Page.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]Subscription, 0)
	for rows.Next() {
		s, err := scan(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *s)
	}
	return out, total, rows.Err()
}

// ListForBilling returns ACTIVE subscriptions with next_billing_date due on or
// before `due`. Used by the billing worker.
func (r *Repository) ListForBilling(ctx context.Context, due time.Time, page pagination.Page) ([]Subscription, int, error) {
	const where = `WHERE s.status = 'ACTIVE' AND s.next_billing_date <= $1`

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM subscriptions s `+where, id.DateArg(&due)).Scan(&total); err != nil {
		return nil, 0, err
	}
	// JOIN plans so the billing worker gets plan flags in one query instead
	// of a separate PlanFlagsByID round-trip per subscription (N+1).
	rows, err := r.pool.Query(ctx, `SELECT `+billingColumns+` FROM subscriptions s
		LEFT JOIN plans p ON p.id = s.plan_id `+where+`
		ORDER BY s.next_billing_date, s.id LIMIT $2 OFFSET $3`,
		id.DateArg(&due), page.PerPage, page.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]Subscription, 0)
	for rows.Next() {
		s, err := scanForBilling(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *s)
	}
	return out, total, rows.Err()
}

func (r *Repository) UpdateFields(ctx context.Context, q db.Querier, s *Subscription) error {
	tag, err := q.Exec(ctx, `UPDATE subscriptions SET start_date=$2, expiry_date=$3,
		payment_terms_days=$4, quantity=$5, discount_id=$6, next_billing_date=$7,
		quote_expires_at=$8, notes=$9
		WHERE id=$1`,
		s.ID, id.DateArg(&s.StartDate), id.DateArg(s.ExpiryDate), s.PaymentTermsDays,
		s.Quantity, id.UUIDArg(s.DiscountID), id.DateArg(s.NextBillingDate),
		s.QuoteExpiresAt, id.StringArg(s.Notes))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) SetStatus(ctx context.Context, q db.Querier, subscriptionID uuid.UUID, status string) error {
	tag, err := q.Exec(ctx, `UPDATE subscriptions SET status = $2 WHERE id = $1`, subscriptionID, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) MarkActivated(ctx context.Context, q db.Querier, subscriptionID uuid.UUID, at time.Time) error {
	tag, err := q.Exec(ctx, `UPDATE subscriptions SET status='ACTIVE', activated_at=$2 WHERE id=$1`, subscriptionID, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) MarkPaused(ctx context.Context, q db.Querier, subscriptionID uuid.UUID, at time.Time) error {
	_, err := q.Exec(ctx, `UPDATE subscriptions SET status='PAUSED', paused_at=$2 WHERE id=$1`, subscriptionID, at)
	return err
}

func (r *Repository) MarkResumed(ctx context.Context, q db.Querier, subscriptionID uuid.UUID, nextBilling time.Time) error {
	_, err := q.Exec(ctx, `UPDATE subscriptions SET status='ACTIVE', paused_at=NULL,
		next_billing_date=$2 WHERE id=$1`, subscriptionID, id.DateArg(&nextBilling))
	return err
}

func (r *Repository) MarkClosed(ctx context.Context, q db.Querier, subscriptionID uuid.UUID, at time.Time) error {
	_, err := q.Exec(ctx, `UPDATE subscriptions SET status='CLOSED', closed_at=$2,
		next_billing_date=NULL WHERE id=$1`, subscriptionID, at)
	return err
}

func (r *Repository) SetNextBillingDate(ctx context.Context, q db.Querier, subscriptionID uuid.UUID, d time.Time) error {
	_, err := q.Exec(ctx, `UPDATE subscriptions SET next_billing_date=$2 WHERE id=$1`, subscriptionID, id.DateArg(&d))
	return err
}

func (r *Repository) InsertLine(ctx context.Context, q db.Querier, l *Line) error {
	_, err := q.Exec(ctx, `INSERT INTO subscription_lines
		(id, subscription_id, product_id, variant_id, description, quantity, unit_price_cents, tax_id, position)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		l.ID, l.SubscriptionID, l.ProductID, id.UUIDArg(l.VariantID), id.StringArg(l.Description),
		l.Quantity, l.UnitPriceCents, id.UUIDArg(l.TaxID), l.Position)
	return err
}

func (r *Repository) Lines(ctx context.Context, q db.Querier, subscriptionID uuid.UUID) ([]Line, error) {
	rows, err := q.Query(ctx, `SELECT `+lineColumns+` FROM subscription_lines
		WHERE subscription_id = $1 ORDER BY position, created_at`, subscriptionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Line, 0)
	for rows.Next() {
		l, err := scanLine(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *l)
	}
	return out, rows.Err()
}

func (r *Repository) GetLine(ctx context.Context, q db.Querier, subscriptionID, lineID uuid.UUID) (*Line, error) {
	return scanLine(q.QueryRow(ctx, `SELECT `+lineColumns+` FROM subscription_lines
		WHERE id = $1 AND subscription_id = $2`, lineID, subscriptionID))
}

func (r *Repository) UpdateLine(ctx context.Context, q db.Querier, l *Line) error {
	tag, err := q.Exec(ctx, `UPDATE subscription_lines SET product_id=$2, variant_id=$3,
		description=$4, quantity=$5, unit_price_cents=$6, tax_id=$7, position=$8
		WHERE id=$1 AND subscription_id=$9`,
		l.ID, l.ProductID, id.UUIDArg(l.VariantID), id.StringArg(l.Description),
		l.Quantity, l.UnitPriceCents, id.UUIDArg(l.TaxID), l.Position, l.SubscriptionID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteLine(ctx context.Context, q db.Querier, subscriptionID, lineID uuid.UUID) error {
	tag, err := q.Exec(ctx, `DELETE FROM subscription_lines WHERE id = $1 AND subscription_id = $2`,
		lineID, subscriptionID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) CustomerExists(ctx context.Context, q db.Querier, customerID uuid.UUID) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM customers WHERE id = $1)`, customerID).Scan(&ok)
	return ok, err
}

// PlanFlagsByID loads the plan columns the state machine and the billing
// worker need, straight from the database.
func (r *Repository) PlanFlagsByID(ctx context.Context, q db.Querier, planID uuid.UUID) (*PlanSpec, error) {
	row := q.QueryRow(ctx, `SELECT id, product_id, tax_id, name, interval, price_cents,
		minimum_quantity, auto_close, auto_renew, allow_pause, allow_early_close, is_active
		FROM plans WHERE id = $1`, planID)
	var (
		f     PlanSpec
		taxID pgtype.UUID
	)
	if err := row.Scan(&f.ID, &f.ProductID, &taxID, &f.Name, &f.Interval, &f.PriceCents,
		&f.MinimumQuantity, &f.AutoClose, &f.AutoRenew, &f.AllowPause, &f.AllowEarlyClose, &f.IsActive); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if taxID.Valid {
		v := uuid.UUID(taxID.Bytes)
		f.TaxID = &v
	}
	return &f, nil
}
