package invoices

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
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const columns = `id, number, customer_id, subscription_id, status, issue_date, due_date,
	period_start, period_end, subtotal_cents, discount_cents, tax_cents, total_cents,
	balance_cents, notes, confirmed_at, sent_at, paid_at, cancelled_at, created_by,
	created_at, updated_at`

const lineColumns = `id, invoice_id, product_id, variant_id, description, quantity,
	unit_price_cents, tax_rate_bp, tax_cents, discount_cents, line_total_cents, position`

func (r *Repository) Insert(ctx context.Context, q db.Querier, i *Invoice) error {
	_, err := q.Exec(ctx, `INSERT INTO invoices
		(id, number, customer_id, subscription_id, status, issue_date, due_date,
		 period_start, period_end, subtotal_cents, discount_cents, tax_cents,
		 total_cents, balance_cents, notes, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		i.ID, i.Number, i.CustomerID, id.UUIDArg(i.SubscriptionID), i.Status,
		id.DateArg(&i.IssueDate), id.DateArg(&i.DueDate), id.DateArg(i.PeriodStart),
		id.DateArg(i.PeriodEnd), i.SubtotalCents, i.DiscountCents, i.TaxCents,
		i.TotalCents, i.BalanceCents, id.StringArg(i.Notes), id.UUIDArg(i.CreatedBy))
	return err
}

func (r *Repository) Get(ctx context.Context, q db.Querier, invoiceID uuid.UUID) (*Invoice, error) {
	return scan(q.QueryRow(ctx, `SELECT `+columns+` FROM invoices WHERE id = $1`, invoiceID))
}

func (r *Repository) List(ctx context.Context, f Filter) ([]Invoice, int, error) {
	where := `WHERE ($1::uuid IS NULL OR customer_id = $1)
		AND ($2::uuid IS NULL OR subscription_id = $2)
		AND ($3 = '' OR status = $3)
		AND ($4 = '' OR number LIKE '%' || $4 || '%')
		AND (NOT $5 OR (
			status = 'CONFIRMED' AND balance_cents > 0 AND due_date < current_date))`

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM invoices `+where,
		id.UUIDArg(f.CustomerID), id.UUIDArg(f.SubscriptionID), f.Status,
		strings.TrimSpace(f.Search), f.Overdue).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `SELECT `+columns+` FROM invoices `+where+`
		ORDER BY issue_date DESC, created_at DESC, id LIMIT $6 OFFSET $7`,
		id.UUIDArg(f.CustomerID), id.UUIDArg(f.SubscriptionID), f.Status,
		strings.TrimSpace(f.Search), f.Overdue, f.Page.PerPage, f.Page.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]Invoice, 0)
	for rows.Next() {
		i, err := scan(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *i)
	}
	return out, total, rows.Err()
}

func (r *Repository) Lines(ctx context.Context, q db.Querier, invoiceID uuid.UUID) ([]Line, error) {
	rows, err := q.Query(ctx, `SELECT `+lineColumns+` FROM invoice_lines
		WHERE invoice_id = $1 ORDER BY position, created_at`, invoiceID)
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

func (r *Repository) InsertLine(ctx context.Context, q db.Querier, l *Line) error {
	_, err := q.Exec(ctx, `INSERT INTO invoice_lines
		(id, invoice_id, product_id, variant_id, description, quantity,
		 unit_price_cents, tax_rate_bp, tax_cents, discount_cents, line_total_cents, position)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		l.ID, l.InvoiceID, id.UUIDArg(l.ProductID), id.UUIDArg(l.VariantID), l.Description,
		l.Quantity, l.UnitPriceCents, l.TaxRateBP, l.TaxCents, l.DiscountCents,
		l.LineTotalCents, l.Position)
	return err
}

func (r *Repository) UpdateLineAmounts(ctx context.Context, q db.Querier, l *Line) error {
	_, err := q.Exec(ctx, `UPDATE invoice_lines SET tax_cents = $2, discount_cents = $3,
		line_total_cents = $4 WHERE id = $1`, l.ID, l.TaxCents, l.DiscountCents, l.LineTotalCents)
	return err
}

// ReplaceLines deletes and rewrites the lines of a DRAFT invoice.
func (r *Repository) ReplaceLines(ctx context.Context, q db.Querier, invoiceID uuid.UUID) error {
	_, err := q.Exec(ctx, `DELETE FROM invoice_lines WHERE invoice_id = $1`, invoiceID)
	return err
}

func (r *Repository) UpdateDraft(ctx context.Context, q db.Querier, i *Invoice) error {
	tag, err := q.Exec(ctx, `UPDATE invoices SET due_date = $2, notes = $3,
		period_start = $4, period_end = $5
		WHERE id = $1 AND status = 'DRAFT'`,
		i.ID, id.DateArg(&i.DueDate), id.StringArg(i.Notes),
		id.DateArg(i.PeriodStart), id.DateArg(i.PeriodEnd))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("invoice is not a draft")
	}
	return nil
}

func (r *Repository) UpdateTotals(ctx context.Context, q db.Querier, invoiceID uuid.UUID, subtotal, discount, tax, total, balance int64) error {
	_, err := q.Exec(ctx, `UPDATE invoices SET subtotal_cents = $2, discount_cents = $3,
		tax_cents = $4, total_cents = $5, balance_cents = $6
		WHERE id = $1 AND status = 'DRAFT'`,
		invoiceID, subtotal, discount, tax, total, balance)
	return err
}

func (r *Repository) SetStatus(ctx context.Context, q db.Querier, invoiceID uuid.UUID, status string, at time.Time) error {
	var query string
	switch status {
	case StatusConfirmed:
		query = `UPDATE invoices SET status='CONFIRMED', confirmed_at=$2, balance_cents=total_cents
			WHERE id=$1 AND status='DRAFT' AND total_cents > 0`
	case StatusPaid:
		query = `UPDATE invoices SET status='PAID', paid_at=$2 WHERE id=$1 AND status='CONFIRMED'`
	case StatusCancelled:
		query = `UPDATE invoices SET status='CANCELLED', cancelled_at=$2 WHERE id=$1 AND status='DRAFT'`
	default:
		return errors.New("unsupported status transition")
	}
	tag, err := q.Exec(ctx, query, invoiceID, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrTransition{to: status}
	}
	return nil
}

// ErrTransition marks an illegal lifecycle move; httpError maps it to 409.

func (r *Repository) CancelConfirmed(ctx context.Context, q db.Querier, invoiceID uuid.UUID, at time.Time) error {
	// Only an unpaid confirmed invoice may be cancelled.
	tag, err := q.Exec(ctx, `UPDATE invoices SET status='CANCELLED', cancelled_at=$2
		WHERE id=$1 AND status='CONFIRMED' AND balance_cents = total_cents`, invoiceID, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrTransition{to: StatusCancelled}
	}
	return nil
}

func (r *Repository) MarkSent(ctx context.Context, q db.Querier, invoiceID uuid.UUID, at time.Time) error {
	tag, err := q.Exec(ctx, `UPDATE invoices SET sent_at = $2 WHERE id = $1 AND status = 'CONFIRMED'`,
		invoiceID, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrTransition{to: "sent"}
	}
	return nil
}

func (r *Repository) ApplyPayment(ctx context.Context, q db.Querier, invoiceID uuid.UUID, amount int64, at time.Time) error {
	tag, err := q.Exec(ctx, `UPDATE invoices SET balance_cents = balance_cents - $2,
		status = CASE WHEN balance_cents - $2 = 0 AND status = 'CONFIRMED' THEN 'PAID'
			WHEN balance_cents - $2 > 0 AND status = 'PAID' THEN 'CONFIRMED' ELSE status END,
		paid_at = CASE WHEN balance_cents - $2 = 0 AND status = 'CONFIRMED' THEN $3
			WHEN balance_cents - $2 > 0 AND status = 'PAID' THEN NULL ELSE paid_at END
		WHERE id = $1 AND status IN ('CONFIRMED','PAID') AND balance_cents >= $2`,
		invoiceID, amount, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrTransition{to: "paid"}
	}
	return nil
}

// TaxDetail is the tax snapshot the service needs when building a line.

func scan(row pgx.Row) (*Invoice, error) {
	var (
		i            Invoice
		issueDate    pgtype.Date
		dueDate      pgtype.Date
		subscription pgtype.UUID
		periodStart  pgtype.Date
		periodEnd    pgtype.Date
		notes        pgtype.Text
		confirmedAt  pgtype.Timestamptz
		sentAt       pgtype.Timestamptz
		paidAt       pgtype.Timestamptz
		cancelledAt  pgtype.Timestamptz
		createdBy    pgtype.UUID
	)
	err := row.Scan(&i.ID, &i.Number, &i.CustomerID, &subscription, &i.Status,
		&issueDate, &dueDate, &periodStart, &periodEnd, &i.SubtotalCents,
		&i.DiscountCents, &i.TaxCents, &i.TotalCents, &i.BalanceCents, &notes,
		&confirmedAt, &sentAt, &paidAt, &cancelledAt, &createdBy,
		&i.CreatedAt, &i.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if subscription.Valid {
		v := uuid.UUID(subscription.Bytes)
		i.SubscriptionID = &v
	}
	if issueDate.Valid {
		i.IssueDate = issueDate.Time
	}
	if dueDate.Valid {
		i.DueDate = dueDate.Time
	}
	if periodStart.Valid {
		t := periodStart.Time
		i.PeriodStart = &t
	}
	if periodEnd.Valid {
		t := periodEnd.Time
		i.PeriodEnd = &t
	}
	if notes.Valid {
		v := notes.String
		i.Notes = &v
	}
	timestamp := func(ts pgtype.Timestamptz) *time.Time {
		if !ts.Valid {
			return nil
		}
		v := ts.Time
		return &v
	}
	i.ConfirmedAt = timestamp(confirmedAt)
	i.SentAt = timestamp(sentAt)
	i.PaidAt = timestamp(paidAt)
	i.CancelledAt = timestamp(cancelledAt)
	if createdBy.Valid {
		v := uuid.UUID(createdBy.Bytes)
		i.CreatedBy = &v
	}
	return &i, nil
}

func scanLine(row pgx.Row) (*Line, error) {
	var (
		l         Line
		productID pgtype.UUID
		variantID pgtype.UUID
	)
	err := row.Scan(&l.ID, &l.InvoiceID, &productID, &variantID, &l.Description,
		&l.Quantity, &l.UnitPriceCents, &l.TaxRateBP, &l.TaxCents, &l.DiscountCents,
		&l.LineTotalCents, &l.Position)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if productID.Valid {
		v := uuid.UUID(productID.Bytes)
		l.ProductID = &v
	}
	if variantID.Valid {
		v := uuid.UUID(variantID.Bytes)
		l.VariantID = &v
	}
	return &l, nil
}
