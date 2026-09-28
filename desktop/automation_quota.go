package main

import (
	"log"
	"strings"
	"time"

	"kwu/internal/automation"
	"kwu/internal/cloud"
)

func (a *App) automationTaskQuotaResolver() *automation.FileTaskQuotaResolver {
	return automation.NewFileTaskQuotaResolver("", automation.TaskQuotaStateLoading)
}

func (a *App) persistAutomationTaskQuota(view CloudBootstrapView) {
	snapshot, ok := automationTaskQuotaSnapshot(view)
	if !ok {
		return
	}
	if err := a.automationTaskQuotaResolver().Save(snapshot); err != nil {
		log.Printf("[automation] persist local task quota: %v", err)
	}
}

func (a *App) clearAutomationTaskQuota() {
	if err := a.automationTaskQuotaResolver().Clear(); err != nil {
		log.Printf("[automation] clear local task quota: %v", err)
	}
}

func (a *App) persistAutomationTaskQuotaFromPayment(view CloudPaymentOperationView) {
	if view.Subscription == nil {
		return
	}
	a.persistAutomationTaskQuota(CloudBootstrapView{
		Available:    true,
		Account:      view.Account,
		Subscription: view.Subscription,
	})
}

func automationTaskQuotaSnapshot(view CloudBootstrapView) (automation.TaskQuotaSnapshot, bool) {
	if view.Account.Status == cloud.AccountAnonymous {
		return automation.TaskQuotaSnapshot{Plan: automation.TaskQuotaPlanAnonymous}, true
	}
	if view.Subscription == nil {
		return automation.TaskQuotaSnapshot{}, false
	}
	current := view.Subscription.Current
	switch current.Status {
	case cloud.SubscriptionFree, cloud.SubscriptionExpired:
		return automation.TaskQuotaSnapshot{Plan: automation.TaskQuotaPlanFree}, true
	case cloud.SubscriptionActive:
		plan := strings.ToLower(strings.TrimSpace(current.Plan))
		switch plan {
		case automation.TaskQuotaPlanLite, automation.TaskQuotaPlanPlus, automation.TaskQuotaPlanPro:
			if current.PeriodEnd == nil || !current.PeriodEnd.After(time.Now().UTC()) {
				return automation.TaskQuotaSnapshot{Plan: automation.TaskQuotaPlanFree}, true
			}
			return automation.TaskQuotaSnapshot{Plan: plan, PeriodEnd: current.PeriodEnd}, true
		default:
			return automation.TaskQuotaSnapshot{Plan: automation.TaskQuotaPlanFree}, true
		}
	default:
		return automation.TaskQuotaSnapshot{}, false
	}
}
