package control

import (
	"testing"

	"kwu/internal/permission"
)

func TestDashboardAddWidgetAlwaysRequiresFreshApproval(t *testing.T) {
	if !requiresFreshApprovalTool("dashboard_add_widget") {
		t.Fatal("dashboard writer must require fresh approval")
	}
	manager := newApprovalManager(permission.Policy{Mode: permission.Allow}, ToolApprovalYolo, 0)
	if manager.preApproved("dashboard_add_widget", "") {
		t.Fatal("yolo posture must not bypass dashboard approval")
	}
	manager.grantSession("dashboard_add_widget", "")
	if manager.preApproved("dashboard_add_widget", "") {
		t.Fatal("session grant must not bypass dashboard approval")
	}
}
