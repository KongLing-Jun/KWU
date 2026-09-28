package main

import (
	"reflect"
	"strings"
	"testing"

	"kwu/internal/project"
)

func TestWorkBuddyProjectsV2WailsSurfaceKeepsScopeAndUnreadBoundaries(t *testing.T) {
	pageType := reflect.TypeOf(CloudProjectActivityPageView{})
	for fieldName, want := range map[string]string{
		"ProjectScopeKey": "projectScopeKey",
		"ProjectID":       "projectId",
		"Items":           "items",
		"NextCursor":      "nextCursor",
		"ReadCursor":      "readCursor",
	} {
		field, ok := pageType.FieldByName(fieldName)
		if !ok || strings.Split(field.Tag.Get("json"), ",")[0] != want {
			t.Fatalf("CloudProjectActivityPageView.%s json tag=%q", fieldName, field.Tag.Get("json"))
		}
	}
	activityType := reflect.TypeOf(CloudProjectActivityView{})
	if unread, ok := activityType.FieldByName("Unread"); !ok || unread.Tag.Get("json") != "unread" {
		t.Fatalf("Activity unread field=%+v", unread)
	}
	for fieldName, want := range map[string]string{
		"OtherMembersUnread":     "otherMembersUnread",
		"UnreadOtherMemberCount": "unreadOtherMemberCount",
		"OtherMemberReadStates":  "otherMemberReadStates",
	} {
		field, ok := activityType.FieldByName(fieldName)
		if !ok || strings.Split(field.Tag.Get("json"), ",")[0] != want {
			t.Fatalf("CloudProjectActivityView.%s json tag=%q", fieldName, field.Tag.Get("json"))
		}
	}
	for _, method := range []string{
		"ListCloudProjects", "ListJoinedCloudProjects", "ListCloudProjectMembers", "JoinCloudProject", "CreateCloudProjectInvite", "AcceptCloudProjectInvite",
		"ListCloudProjectActivities", "ListCloudProjectMessages", "PostCloudProjectMessage", "MarkCloudProjectActivityRead", "MarkCloudProjectMessageRead",
		"ListCloudProjectLabels", "CreateCloudProjectLabel", "PatchCloudProjectLabel", "DeleteCloudProjectLabel", "PutCloudProjectWorkItemLabel",
		"ListProjectConnectorAvailability", "InstallProjectConnector",
	} {
		if _, ok := reflect.TypeOf((*App)(nil)).MethodByName(method); !ok {
			t.Fatalf("App.%s is not bound on the production Wails surface", method)
		}
	}
	for _, removedMethod := range []string{
		"ListCloudProjectTasks", "ListCloudProjectAutomations", "AddCloudProjectAutomation", "LinkCloudProjectAutomationTask",
		"ClaimCloudProjectAutomationRun", "GetCloudProjectAutomationRun", "ReclaimCloudProjectAutomationRun",
		"AbandonCloudProjectAutomationRun", "CompleteCloudProjectAutomationRun", "ExecuteCloudProjectAutomationRun",
	} {
		if _, ok := reflect.TypeOf((*App)(nil)).MethodByName(removedMethod); ok {
			t.Fatalf("removed project automation method App.%s is still bound", removedMethod)
		}
	}
}

func TestCloudProjectActivityViewPreservesEmptyMessageReadReceipt(t *testing.T) {
	hasUnread := false
	unreadCount := int64(0)
	states := []project.ProjectMessageMemberReadState{}
	view := cloudProjectActivityView(project.ProjectActivity{
		OtherMembersUnread:     &hasUnread,
		UnreadOtherMemberCount: &unreadCount,
		OtherMemberReadStates:  &states,
	})
	if view.OtherMembersUnread == nil || *view.OtherMembersUnread || view.UnreadOtherMemberCount == nil || *view.UnreadOtherMemberCount != 0 || view.OtherMemberReadStates == nil || len(*view.OtherMemberReadStates) != 0 {
		t.Fatalf("message read receipt = %#v", view)
	}
}
