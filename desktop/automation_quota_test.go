package main

import (
	"testing"
	"time"

	"kwu/internal/automation"
	"kwu/internal/cloud"
)

func setAnonymousAutomationQuota(t *testing.T, app *App) {
	t.Helper()
	if err := app.automationTaskQuotaResolver().Save(automation.TaskQuotaSnapshot{
		Plan: automation.TaskQuotaPlanAnonymous,
	}); err != nil {
		t.Fatalf("save anonymous automation quota: %v", err)
	}
}

func TestAutomationTaskQuotaSnapshotMapsLocalAccountEntitlement(t *testing.T) {
	periodEnd := time.Now().UTC().Add(time.Hour)
	tests := []struct {
		name string
		view CloudBootstrapView
		plan string
		ok   bool
	}{
		{
			name: "anonymous",
			view: CloudBootstrapView{Account: CloudAccountView{Status: cloud.AccountAnonymous}},
			plan: automation.TaskQuotaPlanAnonymous,
			ok:   true,
		},
		{
			name: "free",
			view: CloudBootstrapView{
				Account:      CloudAccountView{Status: cloud.AccountAuthenticated},
				Subscription: &CloudSubscriptionView{Current: CloudCurrentSubscriptionView{Plan: "free", Status: cloud.SubscriptionFree}},
			},
			plan: automation.TaskQuotaPlanFree,
			ok:   true,
		},
		{
			name: "active plus",
			view: CloudBootstrapView{
				Account:      CloudAccountView{Status: cloud.AccountOffline},
				Subscription: &CloudSubscriptionView{Current: CloudCurrentSubscriptionView{Plan: "plus", Status: cloud.SubscriptionActive, PeriodEnd: &periodEnd}},
			},
			plan: automation.TaskQuotaPlanPlus,
			ok:   true,
		},
		{
			name: "loading without subscription",
			view: CloudBootstrapView{Account: CloudAccountView{Status: cloud.AccountRefreshing}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := automationTaskQuotaSnapshot(test.view)
			if ok != test.ok || got.Plan != test.plan {
				t.Fatalf("snapshot = %+v, %v, want plan %q, ok %v", got, ok, test.plan, test.ok)
			}
		})
	}
}

func TestAutomationTaskQuotaSnapshotDowngradesExpiredPaidPlan(t *testing.T) {
	periodEnd := time.Now().UTC().Add(-time.Second)
	snapshot, ok := automationTaskQuotaSnapshot(CloudBootstrapView{
		Account: CloudAccountView{Status: cloud.AccountOffline},
		Subscription: &CloudSubscriptionView{Current: CloudCurrentSubscriptionView{
			Plan: "pro", Status: cloud.SubscriptionActive, PeriodEnd: &periodEnd,
		}},
	})
	if !ok || snapshot.Plan != automation.TaskQuotaPlanFree {
		t.Fatalf("snapshot = %+v, %v, want Free", snapshot, ok)
	}
}
