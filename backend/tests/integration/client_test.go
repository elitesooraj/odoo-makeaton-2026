package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"
)

// This file is the suite's Postman: a tiny HTTP client with its own cookie
// jar (one jar = one logged-in identity), JSON helpers, and the response
// envelope from docs/CONVENTIONS.md: { data, meta } or { error }.

// envelope mirrors the response contract. Data/Meta stay raw so each test
// unmarshals exactly the shape it cares about.
type envelope struct {
	Data  json.RawMessage `json:"data"`
	Meta  json.RawMessage `json:"meta"`
	Error *apiError       `json:"error"`
}

type apiError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details"`
}

func (e *apiError) String() string {
	if e == nil {
		return "<no error>"
	}
	if len(e.Details) > 0 {
		return fmt.Sprintf("%s: %s %v", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// client is one identity: its own cookie jar, its own session.
type client struct {
	t    *testing.T
	http *http.Client
}

func newClient(t *testing.T) *client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	return &client{t: t, http: &http.Client{Jar: jar}}
}

// do performs one API call and returns the status code and parsed envelope.
// A nil body sends no payload; a map/struct body is marshalled as JSON.
func (c *client) do(method, path string, body any) (int, *envelope) {
	c.t.Helper()

	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			c.t.Fatalf("marshal request body: %v", err)
		}
		rdr = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, server.URL+path, rdr)
	if err != nil {
		c.t.Fatalf("build request %s %s: %v", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatalf("read response: %v", err)
	}

	env := &envelope{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, env); err != nil {
			c.t.Fatalf("%s %s: response is not the JSON envelope: %v\nbody: %s",
				method, path, err, raw)
		}
	}
	return resp.StatusCode, env
}

// mustOK asserts 2xx plus a data payload and decodes data into want.
func (c *client) mustOK(method, path string, body any, want any) *envelope {
	c.t.Helper()
	status, env := c.do(method, path, body)
	if status < 200 || status > 299 {
		c.t.Fatalf("%s %s: status %d, want 2xx; error: %s", method, path, status, env.Error)
	}
	if want != nil {
		c.decode(env, want)
	}
	return env
}

// mustStatus asserts an exact status code and returns the envelope so the
// caller can inspect the error contract.
func (c *client) mustStatus(method, path string, body any, wantStatus int) *envelope {
	c.t.Helper()
	status, env := c.do(method, path, body)
	if status != wantStatus {
		c.t.Fatalf("%s %s: status %d, want %d; body: %s / error: %s",
			method, path, status, wantStatus, string(env.Data), env.Error)
	}
	return env
}

func (c *client) decode(env *envelope, want any) {
	c.t.Helper()
	if env.Data == nil {
		c.t.Fatalf("expected data in the envelope, got error: %s", env.Error)
	}
	if err := json.Unmarshal(env.Data, want); err != nil {
		c.t.Fatalf("decode data into %T: %v\nraw: %s", want, err, env.Data)
	}
}

// adminClient is a fresh session logged in as the bootstrap admin.
func adminClient(t *testing.T) *client {
	t.Helper()
	c := newClient(t)
	c.mustOK(http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    adminEmail,
		"password": adminPassword,
	}, nil)
	return c
}

// registeredUser is what /auth/register returns for a CUSTOMER account.
type registeredUser struct {
	ID         string  `json:"id"`
	Email      string  `json:"email"`
	Role       string  `json:"role"`
	CustomerID *string `json:"customer_id"`
}

// customerClient registers a brand-new CUSTOMER (which also creates the
// linked customer record and starts a session) and returns the client plus
// the user DTO with its customer_id.
func customerClient(t *testing.T, email string) (*client, *registeredUser) {
	t.Helper()
	c := newClient(t)
	var user registeredUser
	c.mustOK(http.MethodPost, "/api/v1/auth/register", map[string]string{
		"email":        email,
		"password":     "Cust0mer!Pass",
		"full_name":    "Test Customer",
		"company_name": companyFor(email),
	}, &user)
	if user.Role != "CUSTOMER" {
		t.Fatalf("registered user role = %q, want CUSTOMER", user.Role)
	}
	if user.CustomerID == nil {
		t.Fatal("registered customer has no customer_id; portal tests need it")
	}
	return c, &user
}

func companyFor(email string) string {
	return "Company of " + email // unique company name per registration
}

// catalogFixture is a ready-made catalog: product, tax, plan and discount.
// interval defaults to WEEKLY so billing math avoids the month-end
// normalization quirk documented in money_test.go.
type catalogFixture struct {
	ProductID  string `json:"product_id"`
	TaxID      string `json:"tax_id"`
	PlanID     string `json:"plan_id"`
	DiscountID string `json:"discount_id"`
}

func createCatalog(t *testing.T, c *client, interval string) *catalogFixture {
	t.Helper()

	var product struct {
		ID string `json:"id"`
	}
	c.mustOK(http.MethodPost, "/api/v1/products", map[string]any{
		"name": "Water Delivery", "description": "Test product", "is_active": true,
	}, &product)

	// 18% exclusive VAT: added on top of line totals.
	var tax struct {
		ID string `json:"id"`
	}
	c.mustOK(http.MethodPost, "/api/v1/taxes", map[string]any{
		"name": "VAT 18", "rate_bp": 1800, "is_inclusive": false, "is_active": true,
	}, &tax)

	var plan struct {
		ID string `json:"id"`
	}
	c.mustOK(http.MethodPost, "/api/v1/plans", map[string]any{
		"product_id": product.ID, "name": "Test plan " + interval, "interval": interval,
		"price_cents": 14900, "minimum_quantity": 1, "auto_renew": true,
		"allow_pause": true, "allow_early_close": true, "tax_id": tax.ID, "is_active": true,
	}, &plan)

	var discount struct {
		ID string `json:"id"`
	}
	c.mustOK(http.MethodPost, "/api/v1/discounts", map[string]any{
		"name": "Launch 10", "kind": "PERCENT", "value_bp": 1000, "is_active": true,
	}, &discount)

	return &catalogFixture{
		ProductID: product.ID, TaxID: tax.ID, PlanID: plan.ID, DiscountID: discount.ID,
	}
}

// customerRecord is a back-office customer created by the admin.
func createCustomer(t *testing.T, c *client, name string) string {
	t.Helper()
	var out struct {
		ID string `json:"id"`
	}
	c.mustOK(http.MethodPost, "/api/v1/customers", map[string]any{
		"name": name, "email": safeEmail(name),
	}, &out)
	return out.ID
}

// safeEmail turns an arbitrary display name into a syntactically valid
// email address, which the customer validator requires.
func safeEmail(name string) string {
	clean := strings.ToLower(name)
	clean = strings.ReplaceAll(clean, " ", ".")
	clean = strings.ReplaceAll(clean, ",", "")
	if clean == "" {
		clean = "customer"
	}
	return clean + "@brightloop.test"
}

// subscriptionDTO is the slice of /subscriptions responses these tests need.
type subscriptionDTO struct {
	ID               string  `json:"id"`
	Number           string  `json:"number"`
	CustomerID       string  `json:"customer_id"`
	PlanID           string  `json:"plan_id"`
	Status           string  `json:"status"`
	NextBillingDate  *string `json:"next_billing_date"`
	QuoteExpiresAt   *string `json:"quote_expires_at"`
	PaymentTermsDays int     `json:"payment_terms_days"`
	Quantity         int     `json:"quantity"`
	Lines            []struct {
		ID             string  `json:"id"`
		Quantity       int     `json:"quantity"`
		UnitPriceCents int64   `json:"unit_price_cents"`
		TaxID          *string `json:"tax_id"`
	} `json:"lines"`
}

// createSubscription makes a DRAFT subscription for a customer on a plan.
func createSubscription(t *testing.T, c *client, customerID, planID string) *subscriptionDTO {
	t.Helper()
	var sub subscriptionDTO
	c.mustOK(http.MethodPost, "/api/v1/subscriptions", map[string]any{
		"customer_id": customerID, "plan_id": planID, "status": "DRAFT",
	}, &sub)
	return &sub
}

// addLine puts a priced, taxed line on a DRAFT/QUOTATION subscription.
func addLine(t *testing.T, c *client, subID string, fix *catalogFixture, quantity int) {
	t.Helper()
	c.mustOK(http.MethodPost, "/api/v1/subscriptions/"+subID+"/lines", map[string]any{
		"product_id": fix.ProductID, "quantity": quantity,
		"unit_price_cents": 14900, "tax_id": fix.TaxID,
		"description": "Weekly 10L",
	}, nil)
}

// activate walks DRAFT -> CONFIRMED -> ACTIVE and returns the live record.
func activate(t *testing.T, c *client, subID string) *subscriptionDTO {
	t.Helper()
	c.mustOK(http.MethodPost, "/api/v1/subscriptions/"+subID+"/confirm", map[string]any{}, nil)
	var sub subscriptionDTO
	c.mustOK(http.MethodPost, "/api/v1/subscriptions/"+subID+"/activate", map[string]any{}, &sub)
	return &sub
}

// invoiceDTO carries the money fields the tests assert on.
type invoiceDTO struct {
	ID             string  `json:"id"`
	Number         string  `json:"number"`
	CustomerID     string  `json:"customer_id"`
	SubscriptionID *string `json:"subscription_id"`
	Status         string  `json:"status"`
	SubtotalCents  int64   `json:"subtotal_cents"`
	DiscountCents  int64   `json:"discount_cents"`
	TaxCents       int64   `json:"tax_cents"`
	TotalCents     int64   `json:"total_cents"`
	BalanceCents   int64   `json:"balance_cents"`
	Lines          []struct {
		Quantity       int   `json:"quantity"`
		UnitPriceCents int64 `json:"unit_price_cents"`
		TaxCents       int64 `json:"tax_cents"`
		LineTotalCents int64 `json:"line_total_cents"`
	} `json:"lines"`
}
