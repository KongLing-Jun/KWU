package main

import (
	"context"
	"errors"
	"log/slog"
	"math/big"
	"time"

	cloud "kwu/internal/cloud"
	project "kwu/internal/project"
	"kwu/internal/userctx"
)

type CloudAccountView struct {
	Status            cloud.AccountStatus `json:"status"`
	ProfileID         string              `json:"profile_id"`
	CloudUserID       string              `json:"cloud_user_id,omitempty"`
	BindingStatus     cloud.BindingStatus `json:"binding_status"`
	BindingGeneration uint64              `json:"binding_generation,omitempty"`
	DisplayName       string              `json:"display_name,omitempty"`
	Role              string              `json:"role,omitempty"`
	LastSyncAt        *time.Time          `json:"last_sync_at,omitempty"`
}

type CloudBalanceView = cloud.BalanceView

type CloudCurrentSubscriptionView struct {
	Plan      string                   `json:"plan"`
	Status    cloud.SubscriptionStatus `json:"status"`
	PeriodEnd *time.Time               `json:"period_end,omitempty"`
	Version   uint64                   `json:"version"`
	UpdatedAt *time.Time               `json:"updated_at,omitempty"`
}

type CloudCapabilitiesView struct {
	Registration     bool   `json:"registration"`
	RegistrationMode string `json:"registration_mode"`
	Payments         bool   `json:"payments"`
	Realtime         bool   `json:"realtime"`
}

type CloudSubscriptionView struct {
	Plans   []cloud.SubscriptionPlan     `json:"plans"`
	Current CloudCurrentSubscriptionView `json:"current"`
}

type CloudOperationError struct {
	Code         cloud.ErrorCode `json:"code"`
	Message      string          `json:"message"`
	RequestID    string          `json:"request_id,omitempty"`
	DiagnosticID string          `json:"diagnostic_id,omitempty"`
}

type CloudPaymentEntryView struct {
	EntryType cloud.PaymentEntryType `json:"entry_type"`
	QRCode    string                 `json:"qr_code,omitempty"`
	CodeURL   string                 `json:"code_url,omitempty"`
	QRCodeURL string                 `json:"qr_code_url,omitempty"`
	ExpiresAt time.Time              `json:"expires_at"`
}

type CloudPaymentView struct {
	OrderID            string                 `json:"order_id"`
	PlanID             string                 `json:"plan_id,omitempty"`
	OrderType          cloud.PaymentOrderType `json:"order_type"`
	Plan               string                 `json:"plan"`
	FromPlan           string                 `json:"from_plan,omitempty"`
	Amount             string                 `json:"amount"`
	AmountUnit         cloud.AmountUnit       `json:"amount_unit,omitempty"`
	Currency           string                 `json:"currency"`
	PaymentChannel     cloud.PaymentChannel   `json:"payment_channel"`
	State              cloud.PaymentUIState   `json:"state"`
	Payable            bool                   `json:"payable"`
	EntitlementGranted bool                   `json:"entitlement_granted"`
	ExpiresAt          time.Time              `json:"expires_at"`
	PaymentEntry       *CloudPaymentEntryView `json:"payment_entry,omitempty"`
	Version            uint64                 `json:"version,omitempty"`
}

type CloudPaymentOperationView struct {
	Account      CloudAccountView       `json:"account"`
	Payment      CloudPaymentView       `json:"payment"`
	Balance      *CloudBalanceView      `json:"balance,omitempty"`
	Subscription *CloudSubscriptionView `json:"subscription,omitempty"`
}

type CloudMaaSRechargeOrderView struct {
	cloud.MaaSRechargeOrder
	CreditDelta       string  `json:"credit_delta,omitempty"`
	RemoteCreditAfter *string `json:"remote_credit_after,omitempty"`
	CreditPerCNY      string  `json:"credit_per_cny,omitempty"`
}

type CloudMaaSRechargeKeyBalanceView struct {
	cloud.MaaSRechargeKeyBalance
	AvailableCredits        string `json:"available_credits"`
	PackageAvailableCredits string `json:"package_available_credits"`
}

type CloudBootstrapView struct {
	Available      bool                   `json:"available"`
	Capabilities   CloudCapabilitiesView  `json:"capabilities"`
	Account        CloudAccountView       `json:"account"`
	Balance        *CloudBalanceView      `json:"balance,omitempty"`
	Subscription   *CloudSubscriptionView `json:"subscription,omitempty"`
	OperationError *CloudOperationError   `json:"operation_error,omitempty"`
	Candidate      *CloudAccountView      `json:"candidate,omitempty"`
	Payments       []CloudPaymentView     `json:"payments,omitempty"`
}

// CloudCachedAssetsView combines the current complete binding identity with
// its last confirmed asset snapshots. Missing snapshots remain absent.
type CloudCachedAssetsView struct {
	ProfileID         string                 `json:"profile_id"`
	CloudUserID       string                 `json:"cloud_user_id"`
	BindingGeneration uint64                 `json:"binding_generation"`
	Balance           *CloudBalanceView      `json:"balance,omitempty"`
	Subscription      *CloudSubscriptionView `json:"subscription,omitempty"`
}

type CloudRegisterRequest struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
	SMSCode  string `json:"sms_code"`
}

type CloudCaptchaConfigView struct {
	Provider string `json:"provider"`
	Region   string `json:"region"`
	Prefix   string `json:"prefix"`
	SceneID  string `json:"scene_id"`
}

type CloudSendRegisterSMSRequest struct {
	Phone           string `json:"phone"`
	CaptchaPassed   bool   `json:"captcha_passed"`
	CaptchaTicket   string `json:"captcha_ticket"`
	CaptchaProvider string `json:"captcha_provider"`
}

type CloudSendPasswordResetSMSRequest struct {
	Phone           string `json:"phone"`
	CaptchaTicket   string `json:"captcha_ticket"`
	CaptchaProvider string `json:"captcha_provider"`
}

type CloudResetPasswordRequest struct {
	Phone       string `json:"phone"`
	NewPassword string `json:"new_password"`
	SMSCode     string `json:"sms_code"`
}

type CloudLoginRequest struct {
	Phone           string `json:"phone"`
	Password        string `json:"password"`
	CaptchaPassed   bool   `json:"captcha_passed"`
	CaptchaTicket   string `json:"captcha_ticket"`
	CaptchaProvider string `json:"captcha_provider"`
}

func (a *App) CloudBootstrap() (CloudBootstrapView, error) {
	view, _, err := a.sharedCloudBootstrap("cloud.bootstrap")
	return view, cloudWailsError("cloud.bootstrap", err)
}

func (a *App) sharedCloudBootstrap(operation string) (CloudBootstrapView, userctx.ActorContext, error) {
	return a.runSharedCloudBootstrap(operation, false)
}

func (a *App) runSharedCloudBootstrap(operation string, retainStartupResult bool) (CloudBootstrapView, userctx.ActorContext, error) {
	runtime, err := a.cloudRuntimeOrUnavailable()
	if err != nil {
		return CloudBootstrapView{}, userctx.ActorContext{}, err
	}
	for {
		actor := a.cloudActorContext().Normalize()
		actorKey := actor.ActorID + "\x00" + actor.ProfileID
		if current := runtime.projectBootstrap.Load(); current != nil {
			joinedRunning := false
			select {
			case <-current.done:
			default:
				joinedRunning = true
				current.waiters.Add(1)
			}
			select {
			case <-current.done:
			case <-a.bootContext().Done():
				return CloudBootstrapView{}, actor, a.bootContext().Err()
			}
			if current.retainResult {
				consumed := runtime.projectBootstrap.CompareAndSwap(current, nil)
				if !joinedRunning && !consumed {
					continue
				}
			}
			if current.actorKey == actorKey && sameDesktopActorProfile(actor, a.cloudActorContext()) &&
				runtime.projectBootstrapResultReusable(current.scope, current.view, current.err) {
				return current.view, actor, current.err
			}
			continue
		}
		flight := &cloudProjectBootstrapFlight{actorKey: actorKey, retainResult: retainStartupResult, done: make(chan struct{})}
		if !runtime.projectBootstrap.CompareAndSwap(nil, flight) {
			continue
		}
		if err := runtime.beginOperation(); err != nil {
			flight.err = err
			close(flight.done)
			runtime.projectBootstrap.CompareAndSwap(flight, nil)
			return CloudBootstrapView{}, actor, err
		}
		lockedActor := a.cloudActorContext().Normalize()
		if !sameDesktopActorProfile(actor, lockedActor) {
			runtime.endOperation()
			close(flight.done)
			runtime.projectBootstrap.CompareAndSwap(flight, nil)
			continue
		}
		result, resultErr := a.cloudBootstrapForActor(operation, runtime, actor)
		flight.scope, _ = runtime.captureProjectBootstrapScope(actor)
		if resultErr == nil && cloudViewHasAuthenticatedBinding(result) {
			a.refreshCloudProjectDirectoriesAfterAuthentication()
		}
		if a.cloudBootstrapBeforePublish != nil {
			a.cloudBootstrapBeforePublish()
		}
		publishCurrent := sameDesktopActorProfile(actor, a.cloudActorContext()) &&
			runtime.projectBootstrapScopeCurrent(flight.scope)
		if publishCurrent && resultErr == nil && cloudViewHasAuthenticatedBinding(result) {
			a.refreshDesktopSubscriptionModelsBestEffort(actor)
			resultErr = a.restoreAuthenticatedSubscriptionTabs(actor)
		}
		publishCurrent = publishCurrent && sameDesktopActorProfile(actor, a.cloudActorContext()) &&
			runtime.projectBootstrapScopeCurrent(flight.scope)
		if publishCurrent {
			a.persistAutomationTaskQuota(result)
		}
		runtime.endOperation()
		flight.view = result
		flight.err = resultErr
		close(flight.done)
		if !flight.retainResult || flight.waiters.Load() > 0 || !publishCurrent ||
			!runtime.projectBootstrapResultReusable(flight.scope, result, resultErr) {
			runtime.projectBootstrap.CompareAndSwap(flight, nil)
		}
		if publishCurrent {
			return result, actor, resultErr
		}
	}
}

func cloudViewHasAuthenticatedBinding(view CloudBootstrapView) bool {
	return view.Account.Status == cloud.AccountAuthenticated &&
		view.Account.BindingStatus == cloud.BindingBound
}

func (a *App) cloudBootstrapForActor(operation string, runtime *cloudRuntime, actor userctx.ActorContext) (CloudBootstrapView, error) {
	epoch := runtime.beginProjectRefresh(a.bootContext(), actor)
	view, err := a.cloudBootstrapForActorAttempt(operation, runtime, actor)
	runtime.finishProjectBootstrap(epoch, actor, view, err)
	return view, err
}

func (a *App) cloudBootstrapForActorAttempt(operation string, runtime *cloudRuntime, actor userctx.ActorContext) (CloudBootstrapView, error) {
	auth, err := runtime.recoverAuth(a.bootContext(), actor)
	var rejected *cloud.CredentialStateError
	if errors.As(err, &rejected) {
		if auth.State != cloud.AccountExpired {
			auth = cloud.AuthView{
				State: cloud.AccountExpired,
				Binding: cloud.ProfileBinding{
					ActorID: actor.ActorID, ProfileID: actor.ProfileID, Status: cloud.BindingExpired,
				},
			}
		}
		err = nil
	}
	view := CloudBootstrapView{Available: true, Capabilities: cloudCapabilitiesView(runtime), Account: cloudAccountView(actor.ProfileID, auth, cloud.AccountState{})}
	if auth.State != cloud.AccountAuthenticated && auth.State != cloud.AccountOffline {
		runtime.replaceProjectServices(project.CapabilityUnavailable, nil)
	}
	reconcileErr := a.reconcileDesktopBillingAuth(runtime, actor, auth)
	if err != nil {
		return view, errors.Join(err, reconcileErr)
	}
	if reconcileErr != nil {
		return view, reconcileErr
	}
	if runtime.remoteAccount != nil {
		if auth.State != cloud.AccountAuthenticated && auth.State != cloud.AccountOffline {
			return view, nil
		}
		state, err := runtime.remoteAccount.CurrentUser(a.bootContext(), actor)
		if err != nil {
			current, _ := runtime.auth.State(a.bootContext(), actor)
			_ = a.reconcileDesktopBillingAuth(runtime, actor, current)
			view.Account = cloudAccountView(actor.ProfileID, current, cloud.AccountState{})
			view.OperationError = cloudOperationError(operation, err)
			return view, nil
		}
		current, err := runtime.auth.State(a.bootContext(), actor)
		if err != nil {
			return view, err
		}
		if err := a.reconcileDesktopBillingAuth(runtime, actor, current); err != nil {
			return view, err
		}
		view.Account = cloudAccountView(actor.ProfileID, current, state)
		scope, err := current.Binding.Scope()
		if err != nil {
			return view, err
		}
		stored, err := runtime.assets.StoreBalance(a.bootContext(), scope, state.Balance)
		if err != nil {
			return view, err
		}
		view.Balance = cloudBalanceView(stored)
		view.Subscription, err = a.refreshRemoteSubscription(runtime, scope, state.Subscription)
		if err != nil {
			view.OperationError = cloudOperationError(operation, err)
		}
		if runtime.paymentRecovery != nil {
			recovered, recoveryErr := runtime.paymentRecovery.Restore(a.bootContext(), actor)
			if recoveryErr != nil {
				return view, recoveryErr
			}
			view.Payments = make([]CloudPaymentView, 0, len(recovered))
			for _, item := range recovered {
				if item.Err != nil {
					return view, item.Err
				}
				view.Payments = append(view.Payments, cloudPaymentView(item.View))
			}
		}
		return view, nil
	}
	if auth.State != cloud.AccountAuthenticated {
		return view, nil
	}
	scope, err := auth.Binding.Scope()
	if err != nil {
		return view, err
	}
	state, err := runtime.accounts.GetCurrentUser(a.bootContext(), scope)
	if err != nil {
		return view, err
	}
	view.Account = cloudAccountView(actor.ProfileID, auth, state)
	recovered, err := runtime.paymentRecovery.Restore(a.bootContext(), actor)
	if err != nil {
		return view, err
	}
	view.Payments = make([]CloudPaymentView, 0, len(recovered))
	for _, payment := range recovered {
		if payment.Err != nil {
			return view, payment.Err
		}
		view.Payments = append(view.Payments, cloudPaymentView(payment.View))
	}
	balance, err := runtime.balance.Refresh(a.bootContext(), actor)
	if err != nil {
		return view, err
	}
	subscription, err := runtime.subscription.Refresh(a.bootContext(), actor)
	if err != nil {
		return view, err
	}
	view.Balance = cloudBalanceView(balance)
	view.Subscription = cloudSubscriptionView(subscription)
	if err := runtime.ensureEventWorker(actor, scope); err != nil {
		return view, err
	}
	return view, nil
}

func (a *App) CloudCreateSubscriptionOrder(plan, channel string) (CloudPaymentOperationView, error) {
	const operation = "cloud.createSubscriptionOrder"
	runtime, actor, end, err := a.cloudRuntimeForActiveOwnerOperation()
	if err != nil {
		return CloudPaymentOperationView{}, cloudPaymentWailsError(operation, err)
	}
	defer func() {
		if end != nil {
			end()
		}
	}()
	if !runtime.capabilities.Payments {
		return CloudPaymentOperationView{}, cloudPaymentWailsError(operation, cloudCapabilityUnavailable("payments"))
	}
	payment, err := runtime.payment.CreateSubscriptionOrder(a.bootContext(), actor, plan, cloud.PaymentChannel(channel))
	if err != nil {
		return CloudPaymentOperationView{}, cloudPaymentWailsError(operation, err)
	}
	view, err := a.cloudPaymentOperationForActor(runtime, actor, payment)
	end()
	end = nil
	if err == nil && view.Payment.EntitlementGranted {
		a.refreshDesktopSubscriptionModelsBestEffort(actor)
	}
	a.persistAutomationTaskQuotaFromPayment(view)
	return view, cloudPaymentWailsError(operation, err)
}

func (a *App) CloudCreateUpgradeOrder(targetPlan, channel string) (CloudPaymentOperationView, error) {
	const operation = "cloud.createUpgradeOrder"
	runtime, actor, end, err := a.cloudRuntimeForActiveOwnerOperation()
	if err != nil {
		return CloudPaymentOperationView{}, cloudPaymentWailsError(operation, err)
	}
	defer end()
	if !runtime.capabilities.Payments {
		return CloudPaymentOperationView{}, cloudPaymentWailsError(operation, cloudCapabilityUnavailable("payments"))
	}
	payment, err := runtime.payment.CreateUpgradeOrder(a.bootContext(), actor, targetPlan, cloud.PaymentChannel(channel))
	if err != nil {
		return CloudPaymentOperationView{}, cloudPaymentWailsError(operation, err)
	}
	view, err := a.cloudPaymentOperationForActor(runtime, actor, payment)
	a.persistAutomationTaskQuotaFromPayment(view)
	return view, cloudPaymentWailsError(operation, err)
}

func (a *App) CloudConfirmPayment(orderID string) (CloudPaymentOperationView, error) {
	const operation = "cloud.confirmPayment"
	runtime, actor, end, err := a.cloudRuntimeForActiveOwnerOperation()
	if err != nil {
		return CloudPaymentOperationView{}, cloudPaymentWailsError(operation, err)
	}
	defer end()
	if !runtime.capabilities.Payments {
		return CloudPaymentOperationView{}, cloudPaymentWailsError(operation, cloudCapabilityUnavailable("payments"))
	}
	payment, err := runtime.payment.ClientPaidUnverified(a.bootContext(), actor, orderID)
	if err != nil {
		return CloudPaymentOperationView{}, cloudPaymentWailsError(operation, err)
	}
	view, err := a.cloudPaymentOperationForActor(runtime, actor, payment)
	a.persistAutomationTaskQuotaFromPayment(view)
	return view, cloudPaymentWailsError(operation, err)
}

func (a *App) CloudRefreshPayment(orderID string) (CloudPaymentOperationView, error) {
	const operation = "cloud.refreshPayment"
	runtime, actor, end, err := a.cloudRuntimeForActiveOwnerOperation()
	if err != nil {
		return CloudPaymentOperationView{}, cloudPaymentWailsError(operation, err)
	}
	defer func() {
		if end != nil {
			end()
		}
	}()
	if !runtime.capabilities.Payments {
		return CloudPaymentOperationView{}, cloudPaymentWailsError(operation, cloudCapabilityUnavailable("payments"))
	}
	payment, err := runtime.payment.RefreshOrder(a.bootContext(), actor, orderID)
	if err != nil {
		return CloudPaymentOperationView{}, cloudPaymentWailsError(operation, err)
	}
	view, err := a.cloudPaymentOperationForActor(runtime, actor, payment)
	end()
	end = nil
	if err == nil && view.Payment.EntitlementGranted {
		a.refreshDesktopSubscriptionModelsBestEffort(actor)
	}
	a.persistAutomationTaskQuotaFromPayment(view)
	return view, cloudPaymentWailsError(operation, err)
}

func (a *App) CloudCreateMaaSRechargeOrder(amount, paymentMethod, idempotencyKey string) (CloudMaaSRechargeOrderView, error) {
	const operation = "cloud.createMaaSRechargeOrder"
	runtime, actor, end, err := a.cloudRuntimeForActiveOwnerOperation()
	if err != nil {
		return CloudMaaSRechargeOrderView{}, cloudPaymentWailsError(operation, err)
	}
	defer end()
	if runtime.maasRecharge == nil {
		return CloudMaaSRechargeOrderView{}, cloudPaymentWailsError(operation, cloudCapabilityUnavailable("maas recharge"))
	}
	auth, err := runtime.auth.State(a.bootContext(), actor)
	if err != nil {
		return CloudMaaSRechargeOrderView{}, cloudPaymentWailsError(operation, err)
	}
	scope, err := auth.Binding.Scope()
	if err != nil {
		return CloudMaaSRechargeOrderView{}, cloudPaymentWailsError(operation, err)
	}
	order, err := runtime.maasRecharge.Create(a.bootContext(), scope, amount, paymentMethod, idempotencyKey)
	if err != nil {
		return CloudMaaSRechargeOrderView{}, cloudPaymentWailsError(operation, err)
	}
	return a.cloudMaaSRechargeOrderView(runtime, actor, order), nil
}

func (a *App) CloudGetMaaSRechargeOrder(orderID string) (CloudMaaSRechargeOrderView, error) {
	const operation = "cloud.getMaaSRechargeOrder"
	runtime, actor, end, err := a.cloudRuntimeForActiveOwnerOperation()
	if err != nil {
		return CloudMaaSRechargeOrderView{}, cloudPaymentWailsError(operation, err)
	}
	defer end()
	if runtime.maasRecharge == nil {
		return CloudMaaSRechargeOrderView{}, cloudPaymentWailsError(operation, cloudCapabilityUnavailable("maas recharge"))
	}
	auth, err := runtime.auth.State(a.bootContext(), actor)
	if err != nil {
		return CloudMaaSRechargeOrderView{}, cloudPaymentWailsError(operation, err)
	}
	scope, err := auth.Binding.Scope()
	if err != nil {
		return CloudMaaSRechargeOrderView{}, cloudPaymentWailsError(operation, err)
	}
	order, err := runtime.maasRecharge.Get(a.bootContext(), scope, orderID)
	if err != nil {
		return CloudMaaSRechargeOrderView{}, cloudPaymentWailsError(operation, err)
	}
	return a.cloudMaaSRechargeOrderView(runtime, actor, order), nil
}

func (a *App) CloudGetMaaSRechargeKeyBalance() (CloudMaaSRechargeKeyBalanceView, error) {
	const operation = "cloud.getMaaSRechargeKeyBalance"
	runtime, actor, end, err := a.cloudRuntimeForActiveOwnerOperation()
	if err != nil {
		return CloudMaaSRechargeKeyBalanceView{}, cloudPaymentWailsError(operation, err)
	}
	defer end()
	if runtime.maasBalance == nil || runtime.maasRechargeBalance == nil {
		return CloudMaaSRechargeKeyBalanceView{}, cloudPaymentWailsError(operation, cloudCapabilityUnavailable("maas recharge balance"))
	}
	auth, err := runtime.auth.State(a.bootContext(), actor)
	if err != nil {
		return CloudMaaSRechargeKeyBalanceView{}, cloudPaymentWailsError(operation, err)
	}
	scope, err := auth.Binding.Scope()
	if err != nil {
		return CloudMaaSRechargeKeyBalanceView{}, cloudPaymentWailsError(operation, err)
	}
	packageBalance, err := runtime.maasBalance.Get(a.bootContext(), scope)
	if err != nil {
		logMaaSBalanceQueryFailure("subscription_key", err)
		return CloudMaaSRechargeKeyBalanceView{}, cloudPaymentWailsError(operation, err)
	}
	rechargeBalance, err := runtime.maasRechargeBalance.Get(a.bootContext(), scope)
	if err != nil {
		logMaaSBalanceQueryFailure("recharge_key", err)
		return CloudMaaSRechargeKeyBalanceView{}, cloudPaymentWailsError(operation, err)
	}
	view := CloudMaaSRechargeKeyBalanceView{
		MaaSRechargeKeyBalance:  rechargeBalance,
		AvailableCredits:        rechargeBalance.AvailableBalance,
		PackageAvailableCredits: packageBalance.AvailableBalance,
	}
	if rechargeBalance.NotFound {
		view.CreditPerCNY = packageBalance.CreditPerCNY
	}
	return view, nil
}

func logMaaSBalanceQueryFailure(stage string, err error) {
	attributes := []any{"stage", stage}
	var typed *cloud.Error
	if errors.As(err, &typed) {
		attributes = append(attributes,
			"response_structure", "cloud_error_envelope",
			"error_code", typed.Code,
			"http_status", typed.HTTPStatus,
			"request_id", typed.RequestID,
			"retryable", typed.Retryable,
		)
	} else {
		attributes = append(attributes, "response_structure", "unknown")
	}
	slog.Warn("MaaS credit balance query failed", attributes...)
}

func (a *App) cloudMaaSRechargeOrderView(runtime *cloudRuntime, actor userctx.ActorContext, order cloud.MaaSRechargeOrder) CloudMaaSRechargeOrderView {
	view := CloudMaaSRechargeOrderView{MaaSRechargeOrder: order}
	balance, found, err := a.cachedMaaSCreditBalance(runtime, actor)
	if err != nil || !found || balance.CreditPerCNY == "" {
		return view
	}
	creditDelta, err := maasQuotaCredits(order.QuotaDelta, order.QuotaPerUSD, order.CNYPerUSD, balance.CreditPerCNY)
	if err != nil {
		return view
	}
	view.CreditDelta = creditDelta
	view.CreditPerCNY = balance.CreditPerCNY
	if order.RemoteQuotaAfter != nil {
		remoteCreditAfter, convertErr := maasQuotaCredits(*order.RemoteQuotaAfter, order.QuotaPerUSD, order.CNYPerUSD, balance.CreditPerCNY)
		if convertErr == nil {
			view.RemoteCreditAfter = &remoteCreditAfter
		}
	}
	return view
}

func (a *App) refreshMaaSCreditBalance(runtime *cloudRuntime, actor userctx.ActorContext) (cloud.BalanceView, error) {
	if runtime.remoteAccount != nil {
		state, err := runtime.remoteAccount.CurrentUser(a.bootContext(), actor)
		if err != nil {
			return cloud.BalanceView{}, err
		}
		auth, err := runtime.auth.State(a.bootContext(), actor)
		if err != nil {
			return cloud.BalanceView{}, err
		}
		scope, err := auth.Binding.Scope()
		if err != nil {
			return cloud.BalanceView{}, err
		}
		return runtime.assets.StoreBalance(a.bootContext(), scope, state.Balance)
	}
	if runtime.balance == nil {
		return cloud.BalanceView{}, errors.New("cloud: balance refresh is unavailable")
	}
	return runtime.balance.Refresh(a.bootContext(), actor)
}

func (a *App) cachedMaaSCreditBalance(runtime *cloudRuntime, actor userctx.ActorContext) (cloud.BalanceView, bool, error) {
	auth, err := runtime.auth.State(a.bootContext(), actor)
	if err != nil {
		return cloud.BalanceView{}, false, err
	}
	scope, err := auth.Binding.Scope()
	if err != nil {
		return cloud.BalanceView{}, false, err
	}
	return runtime.assets.LoadBalance(a.bootContext(), scope)
}

func maasQuotaCredits(quota int64, quotaPerUSD, cnyPerUSD, creditPerCNY string) (string, error) {
	if quota < 0 {
		return "", errors.New("cloud: MaaS quota must not be negative")
	}
	quotaRate, quotaOK := new(big.Rat).SetString(quotaPerUSD)
	cnyRate, cnyOK := new(big.Rat).SetString(cnyPerUSD)
	creditRate, creditOK := new(big.Rat).SetString(creditPerCNY)
	if !quotaOK || !cnyOK || !creditOK || quotaRate.Sign() <= 0 || cnyRate.Sign() <= 0 || creditRate.Sign() <= 0 {
		return "", errors.New("cloud: MaaS credit conversion is unavailable")
	}
	credits := new(big.Rat).SetInt64(quota)
	credits.Mul(credits, cnyRate)
	credits.Mul(credits, creditRate)
	credits.Quo(credits, quotaRate)
	return credits.FloatString(4), nil
}

func subtractCredits(total, deduction string) (string, error) {
	totalCredits, totalOK := new(big.Rat).SetString(total)
	deductionCredits, deductionOK := new(big.Rat).SetString(deduction)
	if !totalOK || !deductionOK || totalCredits.Sign() < 0 || deductionCredits.Sign() < 0 {
		return "", errors.New("cloud: invalid credit balance")
	}
	remaining := new(big.Rat).Sub(totalCredits, deductionCredits)
	if remaining.Sign() < 0 {
		return "", errors.New("cloud: recharge credits exceed available credits")
	}
	return remaining.FloatString(4), nil
}

func (a *App) cloudPaymentOperationForActor(runtime *cloudRuntime, actor userctx.ActorContext, payment cloud.PaymentView) (CloudPaymentOperationView, error) {
	if runtime.remoteAccount != nil {
		auth, err := runtime.auth.State(a.bootContext(), actor)
		if err != nil {
			return CloudPaymentOperationView{}, err
		}
		view := CloudPaymentOperationView{
			Account: cloudAccountView(actor.ProfileID, auth, cloud.AccountState{}),
			Payment: cloudPaymentView(payment),
		}
		if payment.EntitlementGranted {
			state, stateErr := runtime.remoteAccount.CurrentUser(a.bootContext(), actor)
			if stateErr != nil {
				return CloudPaymentOperationView{}, stateErr
			}
			view.Account = cloudAccountView(actor.ProfileID, auth, state)
			view.Balance = cloudBalanceView(state.Balance)
			scope, scopeErr := auth.Binding.Scope()
			if scopeErr != nil {
				return CloudPaymentOperationView{}, scopeErr
			}
			view.Subscription, err = a.refreshRemoteSubscription(runtime, scope, state.Subscription)
			if err != nil {
				return CloudPaymentOperationView{}, err
			}
		}
		return view, nil
	}
	auth, err := runtime.auth.State(a.bootContext(), actor)
	if err != nil {
		return CloudPaymentOperationView{}, err
	}
	scope, err := auth.Binding.Scope()
	if err != nil {
		return CloudPaymentOperationView{}, err
	}
	state, err := runtime.accounts.GetCurrentUser(a.bootContext(), scope)
	if err != nil {
		return CloudPaymentOperationView{}, err
	}
	view := CloudPaymentOperationView{
		Account: cloudAccountView(actor.ProfileID, auth, state),
		Payment: cloudPaymentView(payment),
	}
	if !payment.EntitlementGranted {
		return view, nil
	}
	balance, found, err := runtime.balance.View(a.bootContext(), actor)
	if err != nil {
		return CloudPaymentOperationView{}, err
	}
	if !found {
		return CloudPaymentOperationView{}, errors.New("cloud: granted payment balance snapshot is unavailable")
	}
	subscription, found, err := runtime.subscription.View(a.bootContext(), actor)
	if err != nil {
		return CloudPaymentOperationView{}, err
	}
	if !found {
		return CloudPaymentOperationView{}, errors.New("cloud: granted payment subscription snapshot is unavailable")
	}
	view.Balance = cloudBalanceView(balance)
	view.Subscription = cloudSubscriptionView(subscription)
	return view, nil
}

func cloudBalanceView(snapshot cloud.BalanceSnapshot) *CloudBalanceView {
	view := CloudBalanceView(snapshot)
	return &view
}

func cloudPaymentView(view cloud.PaymentView) CloudPaymentView {
	order := view.Result.Order
	result := CloudPaymentView{
		OrderID: order.OrderID, PlanID: order.PlanID, OrderType: order.OrderType, Plan: order.Plan, FromPlan: order.FromPlan,
		Amount: order.Amount, AmountUnit: order.AmountUnit, Currency: order.Currency,
		PaymentChannel: order.PaymentChannel, State: view.State, Payable: view.Payable,
		EntitlementGranted: view.EntitlementGranted, ExpiresAt: order.ExpiresAt, Version: order.Version,
	}
	if view.Payable {
		entry := view.Result.Entry
		result.PaymentEntry = &CloudPaymentEntryView{
			EntryType: entry.EntryType, QRCode: entry.QRCode, CodeURL: entry.CodeURL, QRCodeURL: entry.QRCodeURL, ExpiresAt: entry.ExpiresAt,
		}
	}
	return result
}

func (a *App) CloudCaptchaConfig() (CloudCaptchaConfigView, error) {
	const operation = "cloud.captchaConfig"
	runtime, err := a.cloudRuntimeForOperation()
	if err != nil {
		return CloudCaptchaConfigView{}, cloudWailsError(operation, err)
	}
	defer runtime.endOperation()
	config, err := runtime.auth.CaptchaConfig(a.bootContext())
	if err != nil {
		return CloudCaptchaConfigView{}, cloudWailsError(operation, err)
	}
	return CloudCaptchaConfigView{Provider: config.Provider, Region: config.Region, Prefix: config.Prefix, SceneID: config.SceneID}, nil
}

func (a *App) CloudSendRegisterSMS(request CloudSendRegisterSMSRequest) error {
	const operation = "cloud.sendRegisterSMS"
	runtime, actor, end, err := a.cloudRuntimeForActiveOwnerOperation()
	if err != nil {
		return cloudWailsError(operation, err)
	}
	defer end()
	if runtime.capabilities.RegistrationMode == cloudRegistrationFixture {
		return cloudWailsError(operation, runtime.auth.SendRegisterSMS(a.bootContext(), actor, request.CaptchaPassed, request.Phone))
	}
	return cloudWailsError(operation, runtime.auth.SendRegisterSMSWithProof(a.bootContext(), actor, request.Phone, cloud.CaptchaProof{
		Ticket: request.CaptchaTicket, Provider: request.CaptchaProvider,
	}))
}

func (a *App) CloudSendPasswordResetSMS(request CloudSendPasswordResetSMSRequest) error {
	const operation = "cloud.sendPasswordResetSMS"
	runtime, err := a.cloudRuntimeForOperation()
	if err != nil {
		return cloudWailsError(operation, err)
	}
	defer runtime.endOperation()
	_, err = runtime.auth.SendPasswordResetSMSWithProof(a.bootContext(), request.Phone, cloud.CaptchaProof{
		Ticket: request.CaptchaTicket, Provider: request.CaptchaProvider,
	})
	return cloudWailsError(operation, err)
}

func (a *App) CloudResetPassword(request CloudResetPasswordRequest) error {
	const operation = "cloud.resetPassword"
	runtime, err := a.cloudRuntimeForOperation()
	if err != nil {
		return cloudWailsError(operation, err)
	}
	defer runtime.endOperation()
	_, err = runtime.auth.ResetPassword(a.bootContext(), cloud.PasswordResetRequest{
		Phone: request.Phone, NewPassword: request.NewPassword, SMSCode: request.SMSCode,
	})
	return cloudWailsError(operation, err)
}

func (a *App) CloudRegister(request CloudRegisterRequest) (CloudBootstrapView, error) {
	const operation = "cloud.register"
	runtime, actor, end, err := a.cloudRuntimeForActiveOwnerOperation()
	if err != nil {
		return CloudBootstrapView{}, cloudWailsError(operation, err)
	}
	projectEpoch := runtime.beginProjectBootstrap(actor)
	auth, operationErr := runtime.auth.Register(a.bootContext(), actor, cloud.RegisterInput{
		Phone: request.Phone, Password: request.Password, SMSCode: request.SMSCode,
	})
	if operationErr != nil {
		runtime.settleProjectBootstrap(projectEpoch, cloudProjectReadinessFailed, actor, auth.Binding, "cloud registration failed")
		view, authErr := a.cloudAuthenticationError(operation, runtime, actor, auth, operationErr)
		end()
		return view, authErr
	}
	a.clearAutomationTaskQuota()
	view, err := a.cloudBootstrapForActor(operation, runtime, actor)
	if err == nil && cloudViewHasAuthenticatedBinding(view) {
		a.refreshCloudProjectDirectoriesAfterAuthentication()
	}
	end()
	if err == nil && cloudViewHasAuthenticatedBinding(view) {
		a.refreshDesktopSubscriptionModelsBestEffort(actor)
		err = a.restoreAuthenticatedSubscriptionTabs(actor)
	}
	a.persistAutomationTaskQuota(view)
	return view, cloudWailsError(operation, err)
}

func (a *App) CloudLogin(request CloudLoginRequest) (CloudBootstrapView, error) {
	const operation = "cloud.login"
	runtime, actor, end, err := a.cloudRuntimeForActiveOwnerOperation()
	if err != nil {
		return CloudBootstrapView{}, cloudWailsError(operation, err)
	}
	projectEpoch := runtime.beginProjectBootstrap(actor)
	auth, operationErr := runtime.auth.Login(a.bootContext(), actor, cloud.LoginInput{
		Phone: request.Phone, Password: request.Password, CaptchaPassed: request.CaptchaPassed,
		CaptchaTicket: request.CaptchaTicket, CaptchaProvider: request.CaptchaProvider,
	})
	if operationErr != nil {
		runtime.settleProjectBootstrap(projectEpoch, cloudProjectReadinessFailed, actor, auth.Binding, "cloud login failed")
		view, authErr := a.cloudAuthenticationError(operation, runtime, actor, auth, operationErr)
		end()
		return view, authErr
	}
	if scope, scopeErr := a.currentProjectScopeForActor(a.bootContext(), actor); scopeErr == nil {
		a.closeCloudProjectTabsOutsideScope(actor, &scope)
	} else {
		a.closeCloudProjectTabsOutsideScope(actor, nil)
	}
	a.emitProjectTreeChanged()
	a.clearAutomationTaskQuota()
	view, err := a.cloudBootstrapForActor(operation, runtime, actor)
	if err == nil && cloudViewHasAuthenticatedBinding(view) {
		a.refreshCloudProjectDirectoriesAfterAuthentication()
	}
	end()
	if err == nil && cloudViewHasAuthenticatedBinding(view) {
		a.refreshDesktopSubscriptionModelsBestEffort(actor)
		err = a.restoreAuthenticatedSubscriptionTabs(actor)
	}
	a.persistAutomationTaskQuota(view)
	return view, cloudWailsError(operation, err)
}

func (a *App) CloudSwitchAccount(request CloudLoginRequest) (CloudBootstrapView, error) {
	const operation = "cloud.switchAccount"
	runtime, actor, end, err := a.cloudRuntimeForActiveOwnerOperation()
	if err != nil {
		return CloudBootstrapView{}, cloudWailsError(operation, err)
	}
	defer func() {
		if end != nil {
			end()
		}
	}()
	input, err := runtime.auth.PrepareLoginInput(a.bootContext(), cloud.LoginInput{
		Phone: request.Phone, Password: request.Password, CaptchaPassed: request.CaptchaPassed,
		CaptchaTicket: request.CaptchaTicket, CaptchaProvider: request.CaptchaProvider,
	})
	if err != nil {
		current, _ := runtime.auth.State(a.bootContext(), actor)
		return CloudBootstrapView{Available: true, Account: cloudAccountView(actor.ProfileID, current, cloud.AccountState{})}, cloudWailsError(operation, err)
	}
	preparation, err := runtime.auth.PrepareAccountSwitch(a.bootContext(), actor, input)
	if err != nil {
		current, _ := runtime.auth.State(a.bootContext(), actor)
		return CloudBootstrapView{Available: true, Account: cloudAccountView(actor.ProfileID, current, cloud.AccountState{})}, cloudWailsError(operation, err)
	}
	// An event may be inside the billing manager. Cancel and drain its context
	// before waiting for billing ownership so the transition has one lock order.
	runtime.stopEventWorker()
	if err := a.quiesceDesktopBillingWorker(); err != nil {
		current, _ := runtime.auth.State(a.bootContext(), actor)
		return CloudBootstrapView{Available: true, Account: cloudAccountView(actor.ProfileID, current, cloud.AccountState{})}, cloudWailsError(operation, err)
	}
	projectEpoch := runtime.beginProjectBootstrap(actor)
	auth, err := runtime.auth.CommitAccountSwitch(a.bootContext(), preparation)
	if err != nil {
		runtime.settleProjectBootstrap(projectEpoch, cloudProjectReadinessFailed, actor, auth.Binding, "cloud account switch failed")
		return CloudBootstrapView{Available: true, Account: cloudAccountView(actor.ProfileID, auth, cloud.AccountState{})}, cloudWailsError(operation, err)
	}
	if scope, scopeErr := a.currentProjectScopeForActor(a.bootContext(), actor); scopeErr == nil {
		a.closeCloudProjectTabsOutsideScope(actor, &scope)
	} else {
		a.closeCloudProjectTabsOutsideScope(actor, nil)
	}
	a.emitProjectTreeChanged()
	a.clearFeedbackImagesForProfile(actor.ProfileID)
	a.clearAutomationTaskQuota()
	view, err := a.cloudBootstrapForActor(operation, runtime, actor)
	if err != nil {
		view.Available = true
		view.Account = cloudAccountView(actor.ProfileID, auth, cloud.AccountState{})
		view.Candidate = nil
		view.OperationError = cloudOperationError(operation, err)
		return view, nil
	}
	a.clearDesktopSubscriptionModels()
	a.refreshCloudProjectDirectoriesAfterAuthentication()
	a.refreshDesktopSubscriptionModelsBestEffort(actor)
	end()
	end = nil
	err = a.restoreAuthenticatedSubscriptionTabs(actor)
	a.persistAutomationTaskQuota(view)
	return view, cloudWailsError(operation, err)
}

func (a *App) CloudLogout() (CloudBootstrapView, error) {
	const operation = "cloud.logout"
	runtime, actor, end, err := a.cloudRuntimeForActiveOwnerOperation()
	if err != nil {
		return CloudBootstrapView{}, cloudWailsError(operation, err)
	}
	defer end()
	runtime.stopEventWorker()
	if err := a.quiesceDesktopBillingWorker(); err != nil {
		return CloudBootstrapView{Available: true}, cloudWailsError(operation, err)
	}
	projectEpoch := runtime.beginProjectBootstrap(actor)
	logoutErr := runtime.auth.Logout(a.bootContext(), actor)
	runtime.invalidateEventWorkerResults(actor)
	if logoutErr != nil {
		auth, stateErr := runtime.auth.State(a.bootContext(), actor)
		phase := cloudProjectReadinessFailed
		reason := "cloud logout failed"
		if stateErr == nil && auth.State == cloud.AccountAnonymous {
			phase = cloudProjectReadinessUnavailable
			reason = "cloud account is anonymous"
		}
		runtime.settleProjectBootstrap(projectEpoch, phase, actor, auth.Binding, reason)
		if stateErr == nil {
			_ = a.reconcileDesktopBillingAuth(runtime, actor, auth)
		}
		if stateErr == nil && auth.State == cloud.AccountAnonymous {
			a.closeCloudProjectTabsOutsideScope(actor, nil)
			a.emitProjectTreeChanged()
			a.clearFeedbackImagesForProfile(actor.ProfileID)
			a.clearDesktopSubscriptionModels()
			view := CloudBootstrapView{
				Available: true, Capabilities: cloudCapabilitiesView(runtime),
				Account:        cloudAccountView(actor.ProfileID, auth, cloud.AccountState{}),
				OperationError: cloudOperationError(operation, logoutErr),
			}
			a.persistAutomationTaskQuota(view)
			return view, nil
		}
		return CloudBootstrapView{Available: true}, cloudWailsError(operation, logoutErr)
	}
	a.closeCloudProjectTabsOutsideScope(actor, nil)
	a.emitProjectTreeChanged()
	a.clearFeedbackImagesForProfile(actor.ProfileID)
	a.clearAutomationTaskQuota()
	a.clearDesktopSubscriptionModels()
	view, err := a.cloudBootstrapForActor(operation, runtime, actor)
	a.persistAutomationTaskQuota(view)
	return view, cloudWailsError(operation, err)
}

func (a *App) CloudRefreshAssets() (CloudBootstrapView, error) {
	const operation = "cloud.refreshAssets"
	runtime, actor, end, err := a.cloudRuntimeForActiveOwnerOperation()
	if err != nil {
		return CloudBootstrapView{}, cloudWailsError(operation, err)
	}
	view, err := a.cloudBootstrapForActor(operation, runtime, actor)
	if err == nil && cloudViewHasAuthenticatedBinding(view) {
		a.refreshCloudProjectDirectoriesAfterAuthentication()
	}
	end()
	if err == nil && cloudViewHasAuthenticatedBinding(view) {
		a.refreshDesktopSubscriptionModelsBestEffort(actor)
		err = a.restoreAuthenticatedSubscriptionTabs(actor)
	}
	a.persistAutomationTaskQuota(view)
	return view, cloudWailsError(operation, err)
}

// CloudCachedAssets returns only locally confirmed snapshots for the active
// account. Unlike CloudRefreshAssets, it performs no recovery or remote call.
func (a *App) CloudCachedAssets() (CloudCachedAssetsView, error) {
	const operation = "cloud.cachedAssets"
	runtime, actor, end, err := a.cloudRuntimeForActiveOwnerOperation()
	if err != nil {
		return CloudCachedAssetsView{}, cloudWailsError(operation, err)
	}
	defer end()
	if runtime.bindings == nil || runtime.assets == nil {
		return CloudCachedAssetsView{}, cloudWailsError(operation, &cloud.Error{
			Code: cloud.ErrorCode("CLOUD_UNAVAILABLE"), Message: "cloud features are unavailable",
		})
	}

	ctx := a.bootContext()
	scope, err := currentCloudAssetScope(ctx, runtime.bindings, actor)
	if err != nil {
		return CloudCachedAssetsView{}, cloudWailsError(operation, err)
	}
	view := CloudCachedAssetsView{
		ProfileID: scope.ProfileID, CloudUserID: scope.CloudUserID, BindingGeneration: scope.BindingGeneration,
	}
	if balance, found, loadErr := runtime.assets.LoadBalance(ctx, scope); loadErr != nil {
		return CloudCachedAssetsView{}, cloudWailsError(operation, loadErr)
	} else if found {
		view.Balance = cloudBalanceView(balance)
	}
	if subscription, found, loadErr := runtime.assets.LoadSubscription(ctx, scope); loadErr != nil {
		return CloudCachedAssetsView{}, cloudWailsError(operation, loadErr)
	} else if found {
		view.Subscription = cloudSubscriptionView(subscription)
	}
	confirmed, err := currentCloudAssetScope(ctx, runtime.bindings, actor)
	if err != nil {
		return CloudCachedAssetsView{}, cloudWailsError(operation, err)
	}
	if confirmed != scope {
		return CloudCachedAssetsView{}, cloudWailsError(operation, &cloud.Error{
			Code: cloud.ErrorProfileScopeMismatch, Message: "cloud: asset cache binding changed during read",
		})
	}
	return view, nil
}

func currentCloudAssetScope(ctx context.Context, bindings cloud.BindingResolver, actor userctx.ActorContext) (cloud.BindingScope, error) {
	actor = actor.Normalize()
	binding, err := bindings.ResolveBinding(ctx, actor)
	if err != nil {
		return cloud.BindingScope{}, err
	}
	if binding.ActorID != actor.ActorID || binding.ProfileID != actor.ProfileID || binding.Status != cloud.BindingBound {
		return cloud.BindingScope{}, &cloud.Error{
			Code: cloud.ErrorProfileScopeMismatch, Message: "cloud: asset cache requires the actor's bound profile",
		}
	}
	return binding.Scope()
}

func (a *App) cloudAuthenticationError(operation string, runtime *cloudRuntime, actor userctx.ActorContext, auth cloud.AuthView, err error) (CloudBootstrapView, error) {
	if reconcileErr := a.reconcileDesktopBillingAuth(runtime, actor, auth); reconcileErr != nil {
		err = errors.Join(err, reconcileErr)
	}
	safe := cloudWailsError(operation, err)
	var typed *cloud.Error
	if !errors.As(safe, &typed) || typed.Code != cloud.ErrorProfileScopeMismatch {
		return CloudBootstrapView{
			Available: true,
			Account:   cloudAccountView(actor.ProfileID, auth, cloud.AccountState{}),
		}, safe
	}
	runtime.beginProjectBootstrap(actor)
	view, bootstrapErr := a.cloudBootstrapForActor(operation, runtime, actor)
	candidate := cloudAccountView(actor.ProfileID, auth, cloud.AccountState{})
	view.OperationError = &CloudOperationError{Code: typed.Code, Message: typed.Message, RequestID: typed.RequestID, DiagnosticID: cloudErrorDiagnosticID(safe)}
	view.Candidate = &candidate
	if bootstrapErr != nil {
		// A profile mismatch is a recoverable account-choice outcome. Preserve the
		// token-safe current and candidate identities even when current-account
		// hydration fails, otherwise the UI has no way to offer that choice.
		view.Available = true
		view.Capabilities = cloudCapabilitiesView(runtime)
	}
	return view, nil
}

func (a *App) reconcileDesktopBillingAuth(runtime *cloudRuntime, actor userctx.ActorContext, auth cloud.AuthView) error {
	// A binding/session transition must cancel the old event caller before the
	// billing manager waits for its operation lock. The same exact worker may
	// remain running because reconcileAuth only reauthorizes that same scope.
	if desktopCloudFakeEnabled() && runtime != nil {
		runtime.stopEventWorkerForAuthTransition(actor, auth)
	}
	var billingErr error
	if a != nil && a.billingWorkerRuntime != nil {
		billingErr = a.billingWorkerRuntime.reconcileAuth(a.bootContext(), actor, auth)
	}
	if a != nil && (auth.State == cloud.AccountExpired ||
		auth.Binding.Status == cloud.BindingExpired ||
		auth.Binding.Status == cloud.BindingMismatch ||
		auth.Binding.Status == cloud.BindingUnbound) {
		if runtime != nil {
			epoch := runtime.beginProjectBootstrap(actor)
			runtime.settleProjectBootstrap(epoch, cloudProjectReadinessUnavailable, actor, auth.Binding, "cloud authentication is unavailable")
		}
		a.clearDesktopSubscriptionModels()
	}
	if desktopCloudFakeEnabled() && runtime != nil && billingErr != nil {
		runtime.stopEventWorker()
	}
	return billingErr
}

func (a *App) quiesceDesktopBillingWorker() error {
	if a == nil || a.billingWorkerRuntime == nil {
		return nil
	}
	return a.billingWorkerRuntime.quiesce()
}

func (a *App) cloudRuntimeForActiveOwnerOperation() (*cloudRuntime, userctx.ActorContext, func(), error) {
	if a == nil {
		return nil, userctx.ActorContext{}, nil, &cloud.Error{Code: cloud.ErrorCode("CLOUD_UNAVAILABLE"), Message: "cloud features are unavailable"}
	}
	runtime, err := a.cloudRuntimeForOperation()
	if err != nil {
		return nil, userctx.ActorContext{}, nil, err
	}
	actor := a.cloudActorContext()
	return runtime, actor, func() {
		runtime.endOperation()
	}, nil
}

type cloudDiagnosticError struct {
	err          error
	diagnosticID string
}

func (e *cloudDiagnosticError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	if e.diagnosticID == "" {
		return e.err.Error()
	}
	return e.err.Error() + " [diagnostic_id=" + e.diagnosticID + "]"
}

func (e *cloudDiagnosticError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func cloudWailsError(operation string, err error) error {
	return sanitizeCloudWailsError(operation, err, false)
}

func sanitizeCloudWailsError(operation string, err error, payment bool) error {
	if err == nil {
		return nil
	}
	diagnosticID := recordCloudRequestFailure(operation, err)
	var safe error
	if payment {
		var typed *cloud.Error
		if errors.As(err, &typed) {
			if message, ok := cloudPaymentWailsSafeMessage(typed.Code); ok {
				safe = cloud.AttachRequestID(&cloud.Error{
					Code:      typed.Code,
					Message:   message,
					Retryable: typed.Retryable,
				}, typed.RequestID)
			}
		}
	}
	if safe == nil {
		safe = safeGenericCloudWailsError(err)
	}
	return &cloudDiagnosticError{err: safe, diagnosticID: diagnosticID}
}

func safeGenericCloudWailsError(err error) error {
	if errors.Is(err, cloud.ErrCaptchaVerificationRequired) {
		return &cloud.Error{Code: cloud.ErrorSliderCaptchaRequired, Message: "captcha verification is required"}
	}
	if errors.Is(err, cloud.ErrCaptchaPermitExpired) {
		return &cloud.Error{Code: cloud.ErrorSliderCaptchaExpired, Message: "captcha verification expired"}
	}
	var typed *cloud.Error
	if errors.As(err, &typed) {
		if message, ok := cloudWailsSafeMessage(typed.Code); ok {
			return cloud.AttachRequestID(&cloud.Error{Code: typed.Code, Message: message}, typed.RequestID)
		}
		return cloud.AttachRequestID(&cloud.Error{Code: cloud.ErrorCode("CLOUD_ERROR"), Message: "cloud request failed"}, typed.RequestID)
	}
	return &cloud.Error{Code: cloud.ErrorCode("CLOUD_ERROR"), Message: "cloud request failed"}
}

// cloudPaymentWailsError exposes only the stable payment error vocabulary. It
// keeps retryability because payment polling needs that fact, while still
// replacing provider/database details with bounded client-facing messages.
func cloudPaymentWailsError(operation string, err error) error {
	return sanitizeCloudWailsError(operation, err, true)
}

func cloudPaymentWailsSafeMessage(code cloud.ErrorCode) (string, bool) {
	messages := map[cloud.ErrorCode]string{
		cloud.ErrorCode("VALIDATION_FAILED"):              "payment request is invalid or unavailable",
		cloud.ErrorCode("NOT_FOUND"):                      "payment order was not found",
		cloud.ErrorCode("CONFLICT"):                       "payment request conflicts with the current subscription",
		cloud.ErrorCode("IDEMPOTENCY_CONFLICT"):           "payment request conflicts with an existing order",
		cloud.ErrorCode("UPSTREAM_ERROR"):                 "payment provider is temporarily unavailable",
		cloud.ErrorCode("INTERNAL_ERROR"):                 "cloud payment service is temporarily unavailable",
		cloud.ErrorCode("CONCURRENT_OPERATION_RETRYABLE"): "another payment operation is in progress; try again",
		cloud.ErrorNetworkUnavailable:                     "cloud network is unavailable",
	}
	message, ok := messages[code]
	return message, ok
}

func cloudOperationError(operation string, err error) *CloudOperationError {
	safe := cloudWailsError(operation, err)
	var typed *cloud.Error
	if errors.As(safe, &typed) {
		return &CloudOperationError{Code: typed.Code, Message: typed.Message, RequestID: typed.RequestID, DiagnosticID: cloudErrorDiagnosticID(safe)}
	}
	return &CloudOperationError{Code: cloud.ErrorCode("CLOUD_ERROR"), Message: "cloud request failed", DiagnosticID: cloudErrorDiagnosticID(safe)}
}

func cloudErrorDiagnosticID(err error) string {
	var diagnostic *cloudDiagnosticError
	if errors.As(err, &diagnostic) {
		return diagnostic.diagnosticID
	}
	return ""
}

func cloudWailsSafeMessage(code cloud.ErrorCode) (string, bool) {
	messages := map[cloud.ErrorCode]string{
		cloud.ErrorCode("CLOUD_UNAVAILABLE"):              "cloud features are unavailable",
		cloud.ErrorCode("CAPABILITY_UNAVAILABLE"):         "cloud capability is not available in this version",
		cloud.ErrorCode("VALIDATION_FAILED"):              "authentication request is invalid",
		cloud.ErrorCode("INVALID_CREDENTIALS"):            "account does not exist or password is incorrect",
		cloud.ErrorCode("TOKEN_EXPIRED"):                  "cloud session expired; sign in again",
		cloud.ErrorCode("REFRESH_TOKEN_INVALID"):          "cloud session expired; sign in again",
		cloud.ErrorCode("REFRESH_TOKEN_EXPIRED"):          "cloud session expired; sign in again",
		cloud.ErrorCode("REFRESH_TOKEN_REUSED"):           "cloud session expired; sign in again",
		cloud.ErrorCode("DEVICE_MISMATCH"):                "cloud session belongs to another device; sign in again",
		cloud.ErrorCode("ACCOUNT_DISABLED"):               "cloud account is disabled",
		cloud.ErrorCode("CLOUD_PROTOCOL_ERROR"):           "cloud response is invalid",
		cloud.ErrorAuthRequired:                           "authentication is required",
		cloud.ErrorSliderCaptchaRequired:                  "captcha verification is required",
		cloud.ErrorSliderCaptchaExpired:                   "captcha verification expired",
		cloud.ErrorCode("SLIDER_CAPTCHA_REUSED"):          "captcha verification was already used",
		cloud.ErrorCode("AUTH_RATE_LIMITED"):              "too many authentication requests; try again later",
		cloud.ErrorCode("AUTH_CRYPTO_CONFIG_UNAVAILABLE"): "registration encryption is temporarily unavailable",
		cloud.ErrorCode("PHONE_INVALID"):                  "phone number is invalid",
		cloud.ErrorCode("SMS_RATE_LIMITED"):               "SMS requests are too frequent; try again later",
		cloud.ErrorCode("SMS_PROVIDER_UNAVAILABLE"):       "SMS service is temporarily unavailable",
		cloud.ErrorCode("PASSWORD_INVALID"):               "password must contain 8 to 20 characters",
		cloud.ErrorSMSCodeInvalid:                         "SMS code is invalid",
		cloud.ErrorCode("SMS_CODE_EXPIRED"):               "SMS code has expired",
		cloud.ErrorCode("SMS_CODE_ATTEMPTS_EXCEEDED"):     "too many invalid SMS code attempts",
		cloud.ErrorCode("PHONE_ALREADY_REGISTERED"):       "phone number is already registered",
		cloud.ErrorCode("INTERNAL_ERROR"):                 "cloud authentication service is temporarily unavailable",
		cloud.ErrorPasswordDecryptFailed:                  "password could not be verified",
		cloud.ErrorSignupRewardGranted:                    "signup reward was already granted",
		cloud.ErrorProfileScopeMismatch:                   "account belongs to another profile",
		cloud.ErrorNetworkUnavailable:                     "cloud network is unavailable",
		cloud.ErrorPaymentNotVerified:                     "payment has not been verified",
		cloud.ErrorOrderAlreadyGranted:                    "payment entitlements are already granted",
		cloud.ErrorUpgradeNotAllowed:                      "subscription upgrade is not allowed",
	}
	message, ok := messages[code]
	return message, ok
}

func cloudCapabilityUnavailable(capability string) error {
	return &cloud.Error{Code: cloud.ErrorCode("CAPABILITY_UNAVAILABLE"), Message: "cloud " + capability + " capability is unavailable"}
}

func (a *App) cloudRuntimeOrUnavailable() (*cloudRuntime, error) {
	if a == nil || a.cloudRuntime == nil || !a.cloudRuntime.available || a.cloudRuntime.closed.Load() || a.cloudRuntime.initErr != nil {
		return nil, &cloud.Error{Code: cloud.ErrorCode("CLOUD_UNAVAILABLE"), Message: "cloud features are unavailable"}
	}
	return a.cloudRuntime, nil
}

func (a *App) cloudRuntimeForOperation() (*cloudRuntime, error) {
	runtime, err := a.cloudRuntimeOrUnavailable()
	if err != nil {
		return nil, err
	}
	if err := runtime.beginOperation(); err != nil {
		return nil, err
	}
	return runtime, nil
}

func cloudAccountView(profileID string, auth cloud.AuthView, state cloud.AccountState) CloudAccountView {
	view := CloudAccountView{
		Status:            auth.State,
		ProfileID:         profileID,
		CloudUserID:       auth.Binding.CloudUserID,
		BindingStatus:     auth.Binding.Status,
		BindingGeneration: auth.Binding.BindingGeneration,
		DisplayName:       auth.User.DisplayName,
		Role:              auth.User.Role,
	}
	lastSync := auth.LastSyncAt
	if state.User.CloudUserID != "" {
		view.CloudUserID = state.User.CloudUserID
		view.DisplayName = state.User.DisplayName
		view.Role = state.User.Role
		lastSync = state.LastSyncAt
	}
	if !lastSync.IsZero() {
		view.LastSyncAt = &lastSync
	}
	if auth.State == cloud.AccountExpired {
		view.DisplayName = ""
		view.Role = ""
	}
	return view
}

func cloudSubscriptionView(view cloud.SubscriptionView) *CloudSubscriptionView {
	current := CloudCurrentSubscriptionView{
		Plan:    view.Subscription.Plan,
		Status:  view.Subscription.Status,
		Version: view.Subscription.Version,
	}
	if !view.Subscription.UpdatedAt.IsZero() {
		updatedAt := view.Subscription.UpdatedAt
		current.UpdatedAt = &updatedAt
	}
	if !view.Subscription.PeriodEnd.IsZero() {
		periodEnd := view.Subscription.PeriodEnd
		current.PeriodEnd = &periodEnd
	}
	return &CloudSubscriptionView{
		Plans:   append([]cloud.SubscriptionPlan(nil), view.Plans...),
		Current: current,
	}
}

func (a *App) refreshRemoteSubscription(runtime *cloudRuntime, scope cloud.BindingScope, subscription cloud.Subscription) (*CloudSubscriptionView, error) {
	if runtime.planClient == nil {
		return cloudRemoteSubscriptionView(subscription, nil), nil
	}
	plans, err := runtime.planClient.ListSubscriptionPlans(a.bootContext(), scope)
	if err != nil {
		slog.Warn(
			"cloud subscription plan catalog refresh failed; using authoritative subscription without plans",
			"err", err,
			"profile_id", scope.ProfileID,
			"binding_generation", scope.BindingGeneration,
		)
		return cloudRemoteSubscriptionView(subscription, nil), nil
	}
	view := cloud.SubscriptionView{Plans: plans, Subscription: subscription}
	if err := runtime.assets.StoreSubscription(a.bootContext(), scope, view); err != nil {
		return cloudRemoteSubscriptionView(subscription, plans), err
	}
	return cloudRemoteSubscriptionView(subscription, plans), nil
}

func cloudRemoteSubscriptionView(subscription cloud.Subscription, plans []cloud.SubscriptionPlan) *CloudSubscriptionView {
	current := CloudCurrentSubscriptionView{Plan: subscription.Plan, Status: subscription.Status, Version: subscription.Version}
	if !subscription.PeriodEnd.IsZero() {
		periodEnd := subscription.PeriodEnd
		current.PeriodEnd = &periodEnd
	}
	return &CloudSubscriptionView{Plans: append([]cloud.SubscriptionPlan(nil), plans...), Current: current}
}

func cloudCapabilitiesView(runtime *cloudRuntime) CloudCapabilitiesView {
	if runtime == nil {
		return CloudCapabilitiesView{}
	}
	return CloudCapabilitiesView{
		Registration:     runtime.capabilities.Registration,
		RegistrationMode: string(runtime.capabilities.RegistrationMode),
		Payments:         runtime.capabilities.Payments,
		Realtime:         runtime.capabilities.Realtime,
	}
}
