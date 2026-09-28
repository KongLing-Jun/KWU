package cloud

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestAccountViewFromAuthProjectsOnlyUISafeIdentity(t *testing.T) {
	lastSync := time.Date(2026, 7, 11, 8, 30, 0, 0, time.UTC)
	view := AccountViewFromAuth(AuthView{
		State: AccountAuthenticated,
		Binding: ProfileBinding{
			ActorID:           "local",
			ProfileID:         "default",
			CloudUserID:       "usr_01",
			Status:            BindingBound,
			BindingGeneration: 2,
		},
		User:       CloudUser{CloudUserID: "usr_01", DisplayName: "Fixture User"},
		LastSyncAt: lastSync,
	})

	if view.Status != AccountAuthenticated || view.ActorID != "local" || view.ProfileID != "default" || view.CloudUserID != "usr_01" || view.BindingStatus != BindingBound || view.BindingGeneration != 2 || view.User == nil || view.User.DisplayName != "Fixture User" || view.LastSyncAt == nil || !view.LastSyncAt.Equal(lastSync) {
		t.Fatalf("unexpected UI account view: %#v", view)
	}
}

func TestAccountViewFromAuthJSONExcludesSensitiveCredentials(t *testing.T) {
	view := AccountViewFromAuth(AuthView{
		State: AccountAuthenticated,
		Binding: ProfileBinding{
			ActorID:           "local",
			ProfileID:         "default",
			CloudUserID:       "usr_01",
			Status:            BindingBound,
			BindingGeneration: 2,
		},
		User: CloudUser{CloudUserID: "usr_01", DisplayName: "Fixture User"},
	})

	payload, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal UI account view: %v", err)
	}
	assertNoSensitiveJSONKeys(t, payload)
	for _, key := range []string{"status", "actor_id", "profile_id", "cloud_user_id", "binding_status", "binding_generation", "user"} {
		if !strings.Contains(string(payload), `"`+key+`"`) {
			t.Fatalf("UI account view JSON omitted required key %q: %s", key, payload)
		}
	}
}

func TestAccountViewTypeHasNoSensitiveJSONFields(t *testing.T) {
	assertNoSensitiveJSONFields(t, reflect.TypeOf(AccountView{}))
}

func TestAccountViewMapperCannotReceiveSessionCredentials(t *testing.T) {
	mapper := reflect.TypeOf(AccountViewFromAuth)
	if mapper.NumIn() != 1 || mapper.In(0) != reflect.TypeOf(AuthView{}) {
		t.Fatalf("account view mapper must accept only AuthView, got %v", mapper)
	}
	for _, field := range []string{"Session", "AccessToken", "RefreshToken", "Password", "EncryptedPassword"} {
		if _, exists := reflect.TypeOf(AuthView{}).FieldByName(field); exists {
			t.Fatalf("AuthView unexpectedly contains sensitive field %q", field)
		}
	}
}

func TestAccountViewFromAuthHidesCloudIdentityOutsideAuthenticatedState(t *testing.T) {
	for _, test := range []struct {
		name   string
		state  AccountStatus
		status BindingStatus
	}{
		{name: "anonymous", state: AccountAnonymous, status: BindingBound},
		{name: "authenticating", state: AccountAuthenticating, status: BindingBound},
		{name: "refreshing", state: AccountRefreshing, status: BindingBound},
		{name: "offline", state: AccountOffline, status: BindingBound},
		{name: "expired", state: AccountExpired, status: BindingBound},
		{name: "mismatch", state: AccountOffline, status: BindingMismatch},
	} {
		t.Run(test.name, func(t *testing.T) {
			view := AccountViewFromAuth(AuthView{
				State: test.state,
				Binding: ProfileBinding{
					ActorID:           "local",
					ProfileID:         "default",
					CloudUserID:       "usr_previous",
					Status:            test.status,
					BindingGeneration: 9,
				},
				User: CloudUser{CloudUserID: "usr_previous", DisplayName: "Previous User"},
			})

			if view.ActorID != "local" || view.ProfileID != "default" {
				t.Fatalf("current local identity was not retained: %#v", view)
			}
			if view.CloudUserID != "" || view.BindingStatus != "" || view.BindingGeneration != 0 || view.User != nil || view.LastSyncAt != nil {
				t.Fatalf("non-authenticated view leaked prior cloud identity: %#v", view)
			}

			payload, err := json.Marshal(view)
			if err != nil {
				t.Fatalf("marshal non-authenticated UI account view: %v", err)
			}
			assertJSONKeysAbsent(t, payload, "cloud_user_id", "binding_status", "binding_generation", "user", "display_name", "last_sync_at")
		})
	}
}

func assertNoSensitiveJSONKeys(t *testing.T, payload []byte) {
	t.Helper()
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		t.Fatalf("unmarshal UI account view: %v", err)
	}
	assertNoSensitiveJSONValue(t, value)
}

func assertNoSensitiveJSONValue(t *testing.T, value any) {
	t.Helper()
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			switch key {
			case "access_token", "refresh_token", "password", "encrypted_password":
				t.Fatalf("UI account view exposed sensitive JSON key %q", key)
			}
			assertNoSensitiveJSONValue(t, child)
		}
	case []any:
		for _, child := range value {
			assertNoSensitiveJSONValue(t, child)
		}
	}
}

func assertNoSensitiveJSONFields(t *testing.T, typ reflect.Type) {
	t.Helper()
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		jsonName := strings.Split(field.Tag.Get("json"), ",")[0]
		if jsonName == "" {
			jsonName = field.Name
		}
		switch jsonName {
		case "access_token", "refresh_token", "password", "encrypted_password":
			t.Fatalf("UI account view type exposes sensitive JSON field %q", jsonName)
		}
		assertNoSensitiveJSONFields(t, field.Type)
	}
}

func assertJSONKeysAbsent(t *testing.T, payload []byte, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if strings.Contains(string(payload), `"`+key+`"`) {
			t.Fatalf("UI account view unexpectedly contains key %q: %s", key, payload)
		}
	}
}
