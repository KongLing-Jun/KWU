package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type SubscriptionPlan struct {
	PlanID        string `json:"plan_id"`
	Plan          string `json:"plan"`
	DisplayName   string `json:"display_name"`
	PriceAmount   string `json:"price_amount"`
	Currency      string `json:"currency"`
	CreditAmount  string `json:"credit_amount"`
	PeriodDays    uint32 `json:"period_days"`
	Enabled       bool   `json:"enabled"`
	SortOrder     int32  `json:"sort_order"`
	ConfigVersion string `json:"config_version"`
}

func (p SubscriptionPlan) Validate() error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{"plan", p.Plan},
		{"display_name", p.DisplayName},
		{"price_amount", p.PriceAmount},
		{"currency", p.Currency},
		{"credit_amount", p.CreditAmount},
	} {
		if err := requireTrimmed(field.name, field.value); err != nil {
			return err
		}
	}
	if p.Currency != "CNY" {
		return fmt.Errorf("cloud: unsupported plan currency %q", p.Currency)
	}
	if p.PeriodDays == 0 || strings.TrimSpace(p.ConfigVersion) == "" {
		return fmt.Errorf("cloud: plan period_days and config_version are required")
	}
	return nil
}

// UnmarshalJSON accepts a legacy numeric value only when reading existing
// local snapshots. Cloud responses use an opaque string version.
func (p *SubscriptionPlan) UnmarshalJSON(data []byte) error {
	var legacy struct {
		PlanID        string          `json:"plan_id"`
		Plan          string          `json:"plan"`
		DisplayName   string          `json:"display_name"`
		PriceAmount   string          `json:"price_amount"`
		Currency      string          `json:"currency"`
		CreditAmount  string          `json:"credit_amount"`
		PeriodDays    uint32          `json:"period_days"`
		Enabled       bool            `json:"enabled"`
		SortOrder     int32           `json:"sort_order"`
		ConfigVersion json.RawMessage `json:"config_version"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return err
	}
	if len(legacy.ConfigVersion) == 0 || bytes.Equal(legacy.ConfigVersion, []byte("null")) {
		return fmt.Errorf("cloud: config_version is required")
	}
	var version string
	if err := json.Unmarshal(legacy.ConfigVersion, &version); err != nil {
		var numeric json.Number
		if numberErr := json.Unmarshal(legacy.ConfigVersion, &numeric); numberErr != nil {
			return err
		}
		version = numeric.String()
	}
	*p = SubscriptionPlan{PlanID: legacy.PlanID, Plan: legacy.Plan, DisplayName: legacy.DisplayName, PriceAmount: legacy.PriceAmount, Currency: legacy.Currency, CreditAmount: legacy.CreditAmount, PeriodDays: legacy.PeriodDays, Enabled: legacy.Enabled, SortOrder: legacy.SortOrder, ConfigVersion: version}
	return nil
}

type PaymentChannel string

const (
	PaymentChannelAlipay PaymentChannel = "alipay"
	PaymentChannelWeChat PaymentChannel = "wechat"
)

func (c PaymentChannel) Validate() error {
	switch c {
	case PaymentChannelAlipay, PaymentChannelWeChat:
		return nil
	default:
		return fmt.Errorf("cloud: invalid payment channel %q", c)
	}
}

type PaymentOrderType string

const (
	PaymentOrderSubscriptionPurchase PaymentOrderType = "subscription_purchase"
	PaymentOrderSubscriptionUpgrade  PaymentOrderType = "subscription_upgrade"
)

type PaymentStatus string

const (
	PaymentCreated                    PaymentStatus = "created"
	PaymentPending                    PaymentStatus = "payment_pending"
	PaymentCallbackReceived           PaymentStatus = "callback_received"
	PaymentVerifying                  PaymentStatus = "verifying"
	PaymentPaidVerified               PaymentStatus = "paid_verified"
	PaymentGranting                   PaymentStatus = "granting"
	PaymentGranted                    PaymentStatus = "granted"
	PaymentClosed                     PaymentStatus = "closed"
	PaymentTimeout                    PaymentStatus = "timeout"
	PaymentFailed                     PaymentStatus = "failed"
	PaymentVerifyFailedManualRequired PaymentStatus = "verify_failed_manual_required"
	// PaymentClientPaidUnverified is a transient local signal, never a persisted order status.
	PaymentClientPaidUnverified PaymentStatus = "client_paid_unverified"
)

type PaymentGrantStatus string

const (
	GrantNotGranted PaymentGrantStatus = "not_granted"
	GrantGranted    PaymentGrantStatus = "granted"
)

type AmountUnit string

const (
	AmountUnitYuan AmountUnit = "yuan"
	AmountUnitFen  AmountUnit = "fen"
)

type PaymentUIState string

const (
	PaymentUIStateCreated        PaymentUIState = "order_created"
	PaymentUIStatePaying         PaymentUIState = "paying"
	PaymentUIStateConfirming     PaymentUIState = "payment_confirming"
	PaymentUIStateSucceeded      PaymentUIState = "payment_succeeded"
	PaymentUIStateClosed         PaymentUIState = "payment_closed"
	PaymentUIStateTimeout        PaymentUIState = "payment_timeout"
	PaymentUIStateFailed         PaymentUIState = "payment_failed"
	PaymentUIStateManualRequired PaymentUIState = "payment_manual_required"
)

// MapPaymentStatus is the single conservative cloud-order-to-UI mapping.
// In particular, only granted is represented as payment_succeeded.
func MapPaymentStatus(status PaymentStatus) (PaymentUIState, error) {
	switch status {
	case PaymentCreated:
		return PaymentUIStateCreated, nil
	case PaymentPending:
		return PaymentUIStatePaying, nil
	case PaymentCallbackReceived, PaymentVerifying, PaymentPaidVerified, PaymentGranting, PaymentClientPaidUnverified:
		return PaymentUIStateConfirming, nil
	case PaymentGranted:
		return PaymentUIStateSucceeded, nil
	case PaymentClosed:
		return PaymentUIStateClosed, nil
	case PaymentTimeout:
		return PaymentUIStateTimeout, nil
	case PaymentFailed:
		return PaymentUIStateFailed, nil
	case PaymentVerifyFailedManualRequired:
		return PaymentUIStateManualRequired, nil
	default:
		return "", fmt.Errorf("cloud: invalid payment status %q", status)
	}
}

func validatePaymentStatusTransition(current, next PaymentStatus) error {
	if current == PaymentClientPaidUnverified || next == PaymentClientPaidUnverified {
		return fmt.Errorf("cloud: client_paid_unverified must not be persisted")
	}
	if _, err := MapPaymentStatus(current); err != nil {
		return err
	}
	if _, err := MapPaymentStatus(next); err != nil {
		return err
	}
	if current == next {
		return nil
	}
	if paymentStatusTerminal(current) {
		return fmt.Errorf("cloud: terminal payment status %q cannot transition to %q", current, next)
	}
	if paymentStatusTerminal(next) {
		return nil
	}
	if paymentStatusRank(next) <= paymentStatusRank(current) {
		return fmt.Errorf("cloud: payment status cannot regress from %q to %q", current, next)
	}
	return nil
}

func paymentStatusTerminal(status PaymentStatus) bool {
	switch status {
	case PaymentGranted, PaymentClosed, PaymentTimeout, PaymentFailed, PaymentVerifyFailedManualRequired:
		return true
	default:
		return false
	}
}

func paymentStatusRank(status PaymentStatus) int {
	switch status {
	case PaymentCreated:
		return 1
	case PaymentPending:
		return 2
	case PaymentCallbackReceived:
		return 3
	case PaymentVerifying:
		return 4
	case PaymentPaidVerified:
		return 5
	case PaymentGranting:
		return 6
	default:
		return 0
	}
}

type PaymentOrder struct {
	Scope          BindingScope       `json:"scope"`
	OrderID        string             `json:"order_id"`
	PlanID         string             `json:"plan_id,omitempty"`
	OrderType      PaymentOrderType   `json:"order_type"`
	Plan           string             `json:"plan"`
	FromPlan       string             `json:"from_plan,omitempty"`
	Amount         string             `json:"amount"`
	AmountUnit     AmountUnit         `json:"amount_unit"`
	Currency       string             `json:"currency"`
	PaymentChannel PaymentChannel     `json:"payment_channel"`
	Status         PaymentStatus      `json:"status"`
	GrantStatus    PaymentGrantStatus `json:"grant_status"`
	ExpiresAt      time.Time          `json:"expires_at"`
	Version        uint64             `json:"version,omitempty"`
}

func (o PaymentOrder) Validate() error {
	if err := o.Scope.Validate(); err != nil {
		return err
	}
	if err := o.validateRequiredFields(); err != nil {
		return err
	}
	if err := o.validateOrderType(); err != nil {
		return err
	}
	if o.Currency != "CNY" {
		return fmt.Errorf("cloud: unsupported payment currency %q", o.Currency)
	}
	if err := o.PaymentChannel.Validate(); err != nil {
		return err
	}
	if err := o.validateChannelAmountUnit(); err != nil {
		return err
	}
	if err := o.validatePersistedStatus(); err != nil {
		return err
	}
	if err := o.validateGrantStatus(); err != nil {
		return err
	}
	if o.ExpiresAt.IsZero() {
		return fmt.Errorf("cloud: payment order expires_at is required")
	}
	return nil
}

func (o PaymentOrder) validateRequiredFields() error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{"order_id", o.OrderID},
		{"plan", o.Plan},
		{"amount", o.Amount},
		{"currency", o.Currency},
	} {
		if err := requireTrimmed(field.name, field.value); err != nil {
			return err
		}
	}
	if o.FromPlan != "" && strings.TrimSpace(o.FromPlan) != o.FromPlan {
		return fmt.Errorf("cloud: from_plan must be trimmed")
	}
	return nil
}

func (o PaymentOrder) validateOrderType() error {
	switch o.OrderType {
	case PaymentOrderSubscriptionPurchase:
		if o.FromPlan != "" {
			return fmt.Errorf("cloud: subscription purchase must not include from_plan")
		}
	case PaymentOrderSubscriptionUpgrade:
		if err := requireTrimmed("from_plan", o.FromPlan); err != nil {
			return err
		}
		if o.FromPlan == o.Plan {
			return fmt.Errorf("cloud: subscription upgrade requires different source and target plans")
		}
	default:
		return fmt.Errorf("cloud: invalid payment order type %q", o.OrderType)
	}
	return nil
}

func (o PaymentOrder) validateChannelAmountUnit() error {
	if o.AmountUnit == "" {
		return nil
	}
	if (o.PaymentChannel == PaymentChannelAlipay && o.AmountUnit != AmountUnitYuan) ||
		(o.PaymentChannel == PaymentChannelWeChat && o.AmountUnit != AmountUnitFen) {
		return fmt.Errorf("cloud: payment channel and amount_unit do not match")
	}
	return nil
}

func (o PaymentOrder) validatePersistedStatus() error {
	if o.Status == PaymentClientPaidUnverified {
		return fmt.Errorf("cloud: client_paid_unverified must not be persisted")
	}
	_, err := MapPaymentStatus(o.Status)
	return err
}

func (o PaymentOrder) validateGrantStatus() error {
	if (o.Status == PaymentGranted && o.GrantStatus != GrantGranted) ||
		(o.Status != PaymentGranted && o.GrantStatus != GrantNotGranted) {
		return fmt.Errorf("cloud: payment status and grant_status do not match")
	}
	return nil
}

type PaymentEntryType string

const (
	PaymentEntryQRCode   PaymentEntryType = "qr_code"
	PaymentEntryNativeQR PaymentEntryType = "native_qr"
)

type PaymentEntry struct {
	Channel   PaymentChannel   `json:"channel"`
	EntryType PaymentEntryType `json:"entry_type"`
	QRCode    string           `json:"qr_code,omitempty"`
	CodeURL   string           `json:"code_url,omitempty"`
	QRCodeURL string           `json:"qr_code_url,omitempty"`
	ExpiresAt time.Time        `json:"expires_at"`
}

func (e PaymentEntry) Validate() error {
	if err := e.Channel.Validate(); err != nil {
		return err
	}
	if e.ExpiresAt.IsZero() {
		return fmt.Errorf("cloud: payment entry expires_at is required")
	}
	if strings.TrimSpace(e.QRCodeURL) != "" {
		return e.validateV3()
	}
	switch e.Channel {
	case PaymentChannelAlipay:
		if e.EntryType != PaymentEntryQRCode || e.CodeURL != "" {
			return fmt.Errorf("cloud: invalid alipay payment entry")
		}
		return requireTrimmed("qr_code", e.QRCode)
	case PaymentChannelWeChat:
		if e.EntryType != PaymentEntryNativeQR || e.QRCode != "" {
			return fmt.Errorf("cloud: invalid wechat payment entry")
		}
		return requireTrimmed("code_url", e.CodeURL)
	default:
		return fmt.Errorf("cloud: invalid payment entry channel %q", e.Channel)
	}
}

func (e PaymentEntry) validateV3() error {
	if e.QRCode != "" || e.CodeURL != "" {
		return fmt.Errorf("cloud: v3 payment entry must use only qr_code_url")
	}
	return requireTrimmed("qr_code_url", e.QRCodeURL)
}

type PaymentOrderResult struct {
	Order PaymentOrder `json:"payment_order"`
	Entry PaymentEntry `json:"payment_entry"`
}

func (r PaymentOrderResult) Validate() error {
	if err := r.Order.Validate(); err != nil {
		return err
	}
	if r.Entry == (PaymentEntry{}) {
		switch r.Order.Status {
		case PaymentCreated, PaymentFailed:
			return nil
		default:
			return fmt.Errorf("cloud: payment entry is required for status %q", r.Order.Status)
		}
	}
	if err := r.Entry.Validate(); err != nil {
		return err
	}
	if r.Order.PaymentChannel != r.Entry.Channel || !r.Order.ExpiresAt.Equal(r.Entry.ExpiresAt) {
		return fmt.Errorf("cloud: payment order and entry do not match")
	}
	return nil
}

type CreateSubscriptionOrderRequest struct {
	Scope          BindingScope   `json:"scope"`
	PlanID         string         `json:"plan_id"`
	Plan           string         `json:"plan,omitempty"`
	PaymentChannel PaymentChannel `json:"payment_channel"`
	IdempotencyKey string         `json:"idempotency_key"`
}

func (r CreateSubscriptionOrderRequest) Validate() error {
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(r.PlanID) == "" {
		if err := requireTrimmed("plan", r.Plan); err != nil {
			return err
		}
	} else {
		if err := requireTrimmed("plan_id", r.PlanID); err != nil {
			return err
		}
	}
	if err := r.PaymentChannel.Validate(); err != nil {
		return err
	}
	return requireTrimmed("idempotency_key", r.IdempotencyKey)
}

type CreateUpgradeOrderRequest struct {
	Scope          BindingScope   `json:"scope"`
	TargetPlan     string         `json:"target_plan"`
	PaymentChannel PaymentChannel `json:"payment_channel"`
	IdempotencyKey string         `json:"idempotency_key"`
}

func (r CreateUpgradeOrderRequest) Validate() error {
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	if err := requireTrimmed("target_plan", r.TargetPlan); err != nil {
		return err
	}
	if err := r.PaymentChannel.Validate(); err != nil {
		return err
	}
	return requireTrimmed("idempotency_key", r.IdempotencyKey)
}

type CloudPaymentClient interface {
	ListSubscriptionPlans(context.Context, BindingScope) ([]SubscriptionPlan, error)
	CreateSubscriptionOrder(context.Context, CreateSubscriptionOrderRequest) (PaymentOrderResult, error)
	CreateUpgradeOrder(context.Context, CreateUpgradeOrderRequest) (PaymentOrderResult, error)
	GetPaymentOrder(context.Context, BindingScope, string) (PaymentOrder, error)
	VerifyPaymentOrder(context.Context, BindingScope, string) (PaymentOrder, error)
}

// SubscriptionPlanClient is the read-only plan boundary used by account
// assets. Payment creation and progression remain outside the Task 1 runtime.
type SubscriptionPlanClient interface {
	ListSubscriptionPlans(context.Context, BindingScope) ([]SubscriptionPlan, error)
}
