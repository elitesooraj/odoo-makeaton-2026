package subscriptions

import "fmt"

// Status is a subscription lifecycle state.
type Status string

const (
	StatusDraft     Status = "DRAFT"
	StatusQuotation Status = "QUOTATION"
	StatusConfirmed Status = "CONFIRMED"
	StatusActive    Status = "ACTIVE"
	StatusClosed    Status = "CLOSED"
)

// UnknownStatusError reports a status that is not part of the subscription
// lifecycle.
type UnknownStatusError struct {
	Status Status
}

func (e *UnknownStatusError) Error() string {
	return fmt.Sprintf("unknown subscription status %q", e.Status)
}

// InvalidTransitionError reports a transition that the lifecycle does not allow.
type InvalidTransitionError struct {
	From Status
	To   Status
}

func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("invalid subscription status transition from %q to %q", e.From, e.To)
}

// Valid reports whether s is one of the five lifecycle statuses.
func (s Status) Valid() bool {
	switch s {
	case StatusDraft, StatusQuotation, StatusConfirmed, StatusActive, StatusClosed:
		return true
	default:
		return false
	}
}

// CanTransition reports whether the lifecycle allows from to move to to.
// Same-status transitions and transitions out of CLOSED are not allowed.
func CanTransition(from, to Status) bool {
	switch from {
	case StatusDraft:
		return to == StatusQuotation
	case StatusQuotation:
		return to == StatusConfirmed
	case StatusConfirmed:
		return to == StatusActive
	case StatusActive:
		return to == StatusClosed
	default:
		return false
	}
}

// ValidateTransition checks a lifecycle transition and returns a descriptive
// error when either status is unknown or the transition is not allowed.
func ValidateTransition(from, to Status) error {
	if !from.Valid() {
		return &UnknownStatusError{Status: from}
	}
	if !to.Valid() {
		return &UnknownStatusError{Status: to}
	}
	if !CanTransition(from, to) {
		return &InvalidTransitionError{From: from, To: to}
	}
	return nil
}
