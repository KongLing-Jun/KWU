package serve

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/eventwire"
	"kwu/internal/expert"
	"kwu/internal/jobs"
	"kwu/internal/provider"
	"kwu/internal/store"
	"kwu/internal/temporal"
	"kwu/internal/userctx"
)

// fakeRunner stands in for an agent.Runner: it records the composed input and
// returns without emitting model events, so the controller's TurnDone is the
// observable signal.
type fakeRunner struct{ got chan string }

func (f fakeRunner) Run(_ context.Context, input string) error { f.got <- input; return nil }

func TestSubmitPropagatesClientTimezone(t *testing.T) {
	bc := NewBroadcaster()
	ctrl := control.New(control.Options{
		Runner:   fakeRunner{got: make(chan string, 2)},
		Sink:     bc,
		Temporal: temporal.New(temporal.Options{Configured: "UTC"}),
	})
	server := httptest.NewServer(New(ctrl, bc, config.ServeConfig{}).Handler())
	defer server.Close()
	defer ctrl.Close()

	resp, err := http.Post(server.URL+"/submit", "application/json", strings.NewReader(`{"input":"hi","timezone":"Asia/Shanghai"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if got := ctrl.Timezone(); got != "Asia/Shanghai" {
		t.Fatalf("timezone = %q", got)
	}

	resp, err = http.Post(server.URL+"/submit", "application/json", strings.NewReader(`{"input":"hi","timezone":"invalid/zone"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid timezone status = %d", resp.StatusCode)
	}
	if got := ctrl.Timezone(); got != "Asia/Shanghai" {
		t.Fatalf("invalid update changed timezone to %q", got)
	}
}

type failAfterWriter struct {
	header    http.Header
	writes    int
	failAfter int
	wrote     chan struct{}
}

func (w *failAfterWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *failAfterWriter) WriteHeader(int) {}

func (w *failAfterWriter) Write(b []byte) (int, error) {
	w.writes++
	if w.writes > w.failAfter {
		return 0, errors.New("write failed")
	}
	select {
	case w.wrote <- struct{}{}:
	default:
	}
	return len(b), nil
}

func (w *failAfterWriter) Flush() {}

func sameCleanPath(a, b string) bool {
	a = canonicalTestPath(a)
	b = canonicalTestPath(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func canonicalTestPath(path string) string {
	if realPath, err := filepath.EvalSymlinks(path); err == nil {
		path = realPath
	}
	return filepath.Clean(path)
}

func TestServeSubmitRunsAndBroadcastsTurnDone(t *testing.T) {
	bc := NewBroadcaster()
	got := make(chan string, 1)
	ctrl := control.New(control.Options{Runner: fakeRunner{got: got}, Sink: bc})
	srv := httptest.NewServer(New(ctrl, bc, config.ServeConfig{}).Handler())
	defer srv.Close()

	sub, cancel := bc.Subscribe() // observe the broadcast deterministically
	defer cancel()

	resp, err := http.Post(srv.URL+"/submit", "application/json", strings.NewReader(`{"input":"hi"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("submit status = %d, want 202", resp.StatusCode)
	}

	select {
	case in := <-got:
		if in != "hi" {
			t.Errorf("runner ran %q, want hi", in)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runner never ran")
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case data := <-sub:
			var w eventwire.Event
			if err := json.Unmarshal(data, &w); err == nil && w.Kind == "turn_done" {
				return
			}
		case <-deadline:
			t.Fatal("never saw turn_done on the stream")
		}
	}
}

func TestServeEndpoints(t *testing.T) {
	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Sink: bc}) // no runner needed for these
	srv := httptest.NewServer(New(ctrl, bc, config.ServeConfig{}).Handler())
	defer srv.Close()

	if resp, err := http.Get(srv.URL + "/history"); err != nil || resp.StatusCode != 200 {
		t.Fatalf("history = %v / %v", resp, err)
	}

	if resp, _ := http.Get(srv.URL + "/context"); resp.StatusCode != 200 {
		t.Errorf("context status = %d", resp.StatusCode)
	}

	resp, err := http.Post(srv.URL+"/plan", "application/json", strings.NewReader(`{"on":true}`))
	if err != nil || resp.StatusCode != http.StatusNoContent {
		t.Fatalf("plan = %v / status %d", err, resp.StatusCode)
	}
	if c := ctrl.Compose("x"); !strings.Contains(c, "Plan mode") {
		t.Error("/plan {on:true} should have enabled plan mode (Compose would prepend the marker)")
	}

	resp, err = http.Post(srv.URL+"/tool-approval-mode", "application/json", strings.NewReader(`{"mode":"auto"}`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("tool approval mode auto status = %d, want 204", resp.StatusCode)
	}
	resp.Body.Close()
	if got := ctrl.ToolApprovalMode(); got != control.ToolApprovalAuto {
		t.Fatalf("tool approval mode = %q, want auto", got)
	}
	resp, err = http.Post(srv.URL+"/tool-approval-mode", "application/json", strings.NewReader(`{"mode":"surprise"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid tool approval mode status = %d, want 400", resp.StatusCode)
	}

	if resp, _ := http.Post(srv.URL+"/submit", "application/json", strings.NewReader(`{}`)); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("empty submit should be 400, got %d", resp.StatusCode)
	}
}

func TestServeNewBindsAuthGateToControllerActorForNoneMode(t *testing.T) {
	bc := NewBroadcaster()
	ctrl := control.New(control.Options{
		Sink: bc,
		Actor: userctx.ActorContext{
			ActorID:       "local",
			ProfileID:     "profile-a",
			AuthMethod:    "local",
			WorkspaceRoot: "workspace",
		},
	})
	s := New(ctrl, bc, config.ServeConfig{})

	got := s.AuthActorContext()
	if got.ActorID != "local" || got.ProfileID != "profile-a" || got.AuthMethod != "local" || got.WorkspaceRoot != "workspace" {
		t.Fatalf("AuthActorContext() = %+v", got)
	}
}

func TestRunGracefulNormalShutdownReturnsNil(t *testing.T) {
	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Sink: bc})
	s := New(ctrl, bc, config.ServeConfig{})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.RunGraceful(ctx, addr)
	}()
	t.Cleanup(cancel)

	url := "http://" + addr + "/"
	deadline := time.Now().Add(2 * time.Second)
	for {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not start: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("RunGraceful normal shutdown returned %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RunGraceful did not return after context cancellation")
	}
}

func TestRunGracefulRejectsNonLoopbackWithoutAuth(t *testing.T) {
	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Sink: bc})
	s := New(ctrl, bc, config.ServeConfig{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.RunGraceful(ctx, "0.0.0.0:0")
	}()

	select {
	case err := <-errCh:
		if err == nil || !strings.Contains(err.Error(), "auth") {
			t.Fatalf("RunGraceful error = %v, want auth error", err)
		}
	case <-time.After(200 * time.Millisecond):
		cancel()
		<-errCh
		t.Fatal("RunGraceful accepted a non-loopback address without auth")
	}
}

func TestRunGracefulAllowsNonLoopbackWithAuth(t *testing.T) {
	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Sink: bc})
	s := New(ctrl, bc, config.ServeConfig{AuthMode: "token", Token: "secret"})

	if err := s.validateListenAuth("0.0.0.0:0"); err != nil {
		t.Fatalf("validateListenAuth with token auth returned %v, want nil", err)
	}
}

func TestListenAddrIsLoopback(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		{addr: "127.0.0.1:8787", want: true},
		{addr: "localhost:8787", want: true},
		{addr: "[::1]:8787", want: true},
		{addr: "0.0.0.0:8787", want: false},
		{addr: "[::]:8787", want: false},
		{addr: ":8787", want: false},
		{addr: "192.168.1.5:8787", want: false},
		{addr: "example.com:8787", want: false},
	}
	for _, tt := range tests {
		if got := listenAddrIsLoopback(tt.addr); got != tt.want {
			t.Errorf("listenAddrIsLoopback(%q) = %v, want %v", tt.addr, got, tt.want)
		}
	}
}

func TestEventsReturnsOnWriteError(t *testing.T) {
	bc := NewBroadcaster()
	s := &Server{bc: bc}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx)
	rw := &failAfterWriter{failAfter: 1, wrote: make(chan struct{}, 1)}
	done := make(chan struct{})
	go func() {
		s.events(rw, req)
		close(done)
	}()
	select {
	case <-rw.wrote:
	case <-time.After(time.Second):
		t.Fatal("events did not write the initial SSE frame")
	}
	if got := bc.Subscribers(); got != 1 {
		t.Fatalf("subscribers after connect = %d, want 1", got)
	}

	bc.Emit(event.Event{Kind: event.Text, Text: "hi"})
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		cancel()
		<-done
		t.Fatal("events did not return after write error")
	}
	if got := bc.Subscribers(); got != 0 {
		t.Fatalf("subscribers after write error = %d, want 0", got)
	}
}

func TestServeSubmitUsesControllerActorBinding(t *testing.T) {
	bc := NewBroadcaster()
	gotInput := make(chan string, 1)
	ctrl := control.New(control.Options{
		Runner: fakeRunner{got: gotInput},
		Sink:   bc,
		Actor: userctx.ActorContext{
			ActorID:    "serve",
			ProfileID:  "profile-a",
			AuthMethod: "serve-token",
		},
	})
	s := New(ctrl, bc, config.ServeConfig{AuthMode: "token", Token: "secret"})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/submit", strings.NewReader(`{"input":"hi"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: cookieToken, Value: "secret"})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("submit status = %d, want 202", resp.StatusCode)
	}
	select {
	case in := <-gotInput:
		if in != "hi" {
			t.Fatalf("runner input = %q, want hi", in)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runner never ran")
	}
	if actor := ctrl.ActorContext(); actor.ActorID != "serve" || actor.ProfileID != "profile-a" || actor.AuthMethod != "serve-token" {
		t.Fatalf("controller ActorContext() = %+v", actor)
	}
}

func TestServeApprovalModeOnlyMutatesCurrentController(t *testing.T) {
	bcA := NewBroadcaster()
	ctrlA := control.New(control.Options{Sink: bcA})
	srvA := httptest.NewServer(New(ctrlA, bcA, config.ServeConfig{}).Handler())
	defer srvA.Close()

	bcB := NewBroadcaster()
	ctrlB := control.New(control.Options{Sink: bcB})
	srvB := httptest.NewServer(New(ctrlB, bcB, config.ServeConfig{}).Handler())
	defer srvB.Close()

	desktopCtrl := control.New(control.Options{})
	desktopCtrl.SetToolApprovalMode(control.ToolApprovalYolo)

	resp, err := http.Post(srvA.URL+"/tool-approval-mode", "application/json", strings.NewReader(`{"mode":"auto"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("tool approval mode status = %d, want 204", resp.StatusCode)
	}
	if got := ctrlA.ToolApprovalMode(); got != control.ToolApprovalAuto {
		t.Fatalf("serve A mode = %q, want auto", got)
	}
	if got := ctrlB.ToolApprovalMode(); got != control.ToolApprovalAsk {
		t.Fatalf("serve B mode = %q, want unchanged ask", got)
	}
	if got := desktopCtrl.ToolApprovalMode(); got != control.ToolApprovalYolo {
		t.Fatalf("desktop mode = %q, want unchanged yolo", got)
	}

	resp, err = http.Post(srvB.URL+"/auto-approve-tools", "application/json", strings.NewReader(`{"on":true}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("legacy auto approve status = %d, want 204", resp.StatusCode)
	}
	if got := ctrlB.ToolApprovalMode(); got != control.ToolApprovalYolo {
		t.Fatalf("serve B legacy mode = %q, want yolo", got)
	}
	if got := ctrlA.ToolApprovalMode(); got != control.ToolApprovalAuto {
		t.Fatalf("serve A mode = %q, want still auto", got)
	}
	if got := desktopCtrl.ToolApprovalMode(); got != control.ToolApprovalYolo {
		t.Fatalf("desktop mode = %q, want unchanged yolo", got)
	}
}

func TestServeExpertEndpoints(t *testing.T) {
	bc := NewBroadcaster()
	testDir := t.TempDir()
	userExpertPath := filepath.Join(testDir, "experts.json")
	loader := func() (*expert.Catalog, error) {
		return expert.LoadCatalog(userExpertPath)
	}
	catalog, err := loader()
	if err != nil {
		t.Fatal(err)
	}
	builder := func(sessionExpertID, requestExpertID string) (string, expert.Selection, error) {
		currentCatalog, err := loader()
		if err != nil {
			return "", expert.Selection{}, err
		}
		selection, err := expert.ResolveSelection(currentCatalog, sessionExpertID, requestExpertID, "")
		if err != nil {
			return "", selection, err
		}
		sys := "base"
		if selection.Selected {
			sys = expert.ApplyPrompt(sys, selection.Expert)
		}
		return sys, selection, nil
	}
	exec := agent.New(nil, nil, agent.NewSession("base"), agent.Options{}, bc)
	ctrl := control.New(control.Options{
		Executor:            exec,
		SystemPrompt:        "base",
		Sink:                bc,
		Experts:             catalog,
		Expert:              expert.Selection{Source: expert.SourceNone},
		PromptBuilder:       builder,
		ExpertCatalogLoader: loader,
		UserExpertPath:      userExpertPath,
		SessionDir:          filepath.Join(testDir, "sessions"),
	})
	ctrl.EnsureSessionPath()
	srv := httptest.NewServer(New(ctrl, bc, config.ServeConfig{}).Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/experts")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /experts status = %d body=%s", resp.StatusCode, body)
	}
	if strings.Contains(string(body), `"prompt"`) {
		t.Fatalf("GET /experts leaked prompt field: %s", body)
	}
	var listedExperts []expert.Info
	if err := json.Unmarshal(body, &listedExperts); err != nil {
		t.Fatalf("decode GET /experts: %v", err)
	}
	var listedTeam *expert.Info
	for i := range listedExperts {
		if listedExperts[i].ID == "code-delivery-team" {
			listedTeam = &listedExperts[i]
			break
		}
	}
	if listedTeam == nil || listedTeam.Source != expert.SourceOfficial || listedTeam.Mode != expert.ModeTeam ||
		len(listedTeam.Members) == 0 || len(listedTeam.Prompts) < 3 {
		t.Fatalf("GET /experts team metadata = %+v, want safe official team details", listedTeam)
	}

	resp, err = http.Post(srv.URL+"/expert/duplicate", "application/json", strings.NewReader(`{"id":"code-delivery-team"}`))
	if err != nil {
		t.Fatal(err)
	}
	duplicateBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(duplicateBody), "cannot be duplicated") {
		t.Fatalf("POST /expert/duplicate official status=%d body=%s, want rejected", resp.StatusCode, duplicateBody)
	}

	resp, err = http.Post(srv.URL+"/expert", "application/json", strings.NewReader(`{"id":"testing"}`))
	if err != nil {
		t.Fatal(err)
	}
	var selected expert.State
	if err := json.NewDecoder(resp.Body).Decode(&selected); err != nil {
		t.Fatalf("decode SetExpert: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || selected.SelectedID != "testing" {
		t.Fatalf("POST /expert set status=%d state=%+v", resp.StatusCode, selected)
	}
	persisted, ok, err := agent.LoadSessionExpert(ctrl.SessionPath())
	if err != nil || !ok || persisted.ExpertID != "testing" {
		t.Fatalf("persisted expert metadata = %+v ok=%v err=%v, want testing identity", persisted, ok, err)
	}

	createBody := `{"name":"Local Reviewer","mode":"expert","description":"Reviews local changes.","category":"engineering","tags":["review"],"systemPrompt":"Review as a local user expert.","displayPrompt":"Review this change.","language":"auto","enabled":true}`
	resp, err = http.Post(srv.URL+"/user-expert", "application/json", strings.NewReader(createBody))
	if err != nil {
		t.Fatal(err)
	}
	var created expert.UserExpertInfo
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode CreateUserExpert: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || created.ID == "" {
		t.Fatalf("POST /user-expert status=%d created=%+v", resp.StatusCode, created)
	}
	if created.Source != expert.SourceUser || len(created.Prompts) != 3 || created.Prompts[0] != "Review this change." {
		t.Fatalf("POST /user-expert metadata = %+v, want user source and normalized prompts", created)
	}

	resp, err = http.Get(srv.URL + "/user-experts")
	if err != nil {
		t.Fatal(err)
	}
	var userExperts []expert.UserExpertInfo
	if err := json.NewDecoder(resp.Body).Decode(&userExperts); err != nil {
		t.Fatalf("decode user experts: %v", err)
	}
	resp.Body.Close()
	if len(userExperts) != 1 || userExperts[0].ID != created.ID {
		t.Fatalf("GET /user-experts = %+v, want created", userExperts)
	}

	resp, err = http.Post(srv.URL+"/expert", "application/json", strings.NewReader(`{"id":"`+created.ID+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewDecoder(resp.Body).Decode(&selected); err != nil {
		t.Fatalf("decode SetExpert user: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || selected.SelectedID != created.ID {
		t.Fatalf("POST /expert user status=%d state=%+v", resp.StatusCode, selected)
	}

	resp, err = http.Post(srv.URL+"/expert/favorite", "application/json", strings.NewReader(`{"id":"`+created.ID+`","favorite":true}`))
	if err != nil {
		t.Fatal(err)
	}
	var library expert.ExpertLibraryState
	if err := json.NewDecoder(resp.Body).Decode(&library); err != nil {
		t.Fatalf("decode favorite: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || len(library.FavoriteIDs) != 1 || library.FavoriteIDs[0] != created.ID {
		t.Fatalf("POST /expert/favorite status=%d library=%+v", resp.StatusCode, library)
	}

	resp, err = http.Get(srv.URL + "/expert-library")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewDecoder(resp.Body).Decode(&library); err != nil {
		t.Fatalf("decode library: %v", err)
	}
	resp.Body.Close()
	if len(library.RecentIDs) == 0 || library.RecentIDs[0] != created.ID {
		t.Fatalf("GET /expert-library = %+v, want recent created", library)
	}

	importPayload := `{"name":"Imported Expert","mode":"expert","description":"Imported schema.","category":"custom","tags":["imported"],"systemPrompt":"Act as imported expert.","displayPrompt":"Handle this as imported.","language":"auto","enabled":true}`
	resp, err = http.Post(srv.URL+"/expert/import", "application/json", strings.NewReader(`{"payload":`+strconv.Quote(importPayload)+`}`))
	if err != nil {
		t.Fatal(err)
	}
	var imported expert.UserExpertInput
	if err := json.NewDecoder(resp.Body).Decode(&imported); err != nil {
		t.Fatalf("decode import: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || imported.Name != "Imported Expert" {
		t.Fatalf("POST /expert/import status=%d imported=%+v", resp.StatusCode, imported)
	}
	if len(imported.Prompts) != 3 || imported.Prompts[0] != imported.DisplayPrompt {
		t.Fatalf("POST /expert/import prompts = %v display=%q, want normalized v1 import", imported.Prompts, imported.DisplayPrompt)
	}

	resp, err = http.Post(srv.URL+"/user-expert/delete", "application/json", strings.NewReader(`{"id":"`+created.ID+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("POST /user-expert/delete status=%d, want 204", resp.StatusCode)
	}

	resp, err = http.Get(srv.URL + "/expert")
	if err != nil {
		t.Fatal(err)
	}
	var current expert.State
	if err := json.NewDecoder(resp.Body).Decode(&current); err != nil {
		t.Fatalf("decode CurrentExpert: %v", err)
	}
	resp.Body.Close()
	if current.SelectedID != "" {
		t.Fatalf("GET /expert state=%+v, want cleared after deleting current user expert", current)
	}

	resp, err = http.Post(srv.URL+"/expert", "application/json", strings.NewReader(`{"id":""}`))
	if err != nil {
		t.Fatal(err)
	}
	var cleared expert.State
	if err := json.NewDecoder(resp.Body).Decode(&cleared); err != nil {
		t.Fatalf("decode ClearExpert: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || cleared.SelectedID != "" {
		t.Fatalf("POST /expert clear status=%d state=%+v", resp.StatusCode, cleared)
	}

	exec.Session().Add(provider.Message{Role: provider.RoleUser, Content: "already started"})
	resp, err = http.Post(srv.URL+"/expert", "application/json", strings.NewReader(`{"id":"testing"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("POST /expert after content status=%d, want 409", resp.StatusCode)
	}
}

func TestRemoveSessionFilesRemovesOwnedSidecarsInSessionDir(t *testing.T) {
	dir := t.TempDir()
	otherDir := t.TempDir()
	sessionPath := filepath.Join(dir, "same.jsonl")
	otherPath := filepath.Join(otherDir, "same.jsonl")
	writeSessionSidecars := func(path string) []string {
		t.Helper()
		if err := os.WriteFile(path, []byte(`{"role":"user","content":"hi"}`+"\n"), 0o644); err != nil {
			t.Fatalf("write session: %v", err)
		}
		for _, path := range []string{
			store.SessionMeta(path),
			store.SessionGoalState(path),
		} {
			if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
				t.Fatalf("write sidecar %s: %v", path, err)
			}
		}
		ckptDir := store.SessionCheckpointDir(path)
		if err := os.MkdirAll(ckptDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ckptDir, "1.json"), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		jobsDir := jobs.ArtifactDir(path)
		if err := os.MkdirAll(jobsDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(jobsDir, "job.log"), []byte("output"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := agent.MarkCleanupPending(path, "delete"); err != nil {
			t.Fatal(err)
		}
		return []string{
			path,
			store.SessionMeta(path),
			store.SessionGoalState(path),
			ckptDir,
			jobsDir,
			agent.CleanupPendingPath(path),
		}
	}
	ownedPaths := writeSessionSidecars(sessionPath)
	otherPaths := writeSessionSidecars(otherPath)
	absDir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	absSessionPath, err := filepath.Abs(sessionPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := removeSessionFiles(absDir, absSessionPath); err != nil {
		t.Fatalf("removeSessionFiles: %v", err)
	}

	for _, path := range ownedPaths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s should be removed, stat err = %v", path, err)
		}
	}
	for _, path := range otherPaths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("other profile sidecar %s should remain: %v", path, err)
		}
	}
}

func TestServeSubmitRejectsShellShortcut(t *testing.T) {
	bc := NewBroadcaster()
	got := make(chan string, 1)
	ctrl := control.New(control.Options{Runner: fakeRunner{got: got}, Sink: bc})
	srv := httptest.NewServer(New(ctrl, bc, config.ServeConfig{}).Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/submit", "application/json", strings.NewReader(`{"input":"!echo nope"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("shell submit status = %d, want 403", resp.StatusCode)
	}
	select {
	case in := <-got:
		t.Fatalf("runner should not run shell submit, got %q", in)
	default:
	}
}

func TestHistoryMessagesPreserveToolDetails(t *testing.T) {
	got := historyMessages([]provider.Message{
		{Role: provider.RoleUser, Content: "run command"},
		{Role: provider.RoleAssistant, Content: "checking", ReasoningContent: "think", ToolCalls: []provider.ToolCall{{
			ID: "call_1", Name: "bash", Arguments: `{"command":"pwd"}`,
		}}},
		{Role: provider.RoleTool, Name: "bash", ToolCallID: "call_1", Content: "/tmp/project\n"},
	})

	if len(got) != 3 {
		t.Fatalf("history length = %d, want 3", len(got))
	}
	if got[1].Reasoning != "think" {
		t.Fatalf("assistant reasoning = %q, want think", got[1].Reasoning)
	}
	if len(got[1].ToolCalls) != 1 || got[1].ToolCalls[0].ID != "call_1" || got[1].ToolCalls[0].Name != "bash" || got[1].ToolCalls[0].Arguments != `{"command":"pwd"}` {
		t.Fatalf("assistant tool calls not preserved: %+v", got[1].ToolCalls)
	}
	if got[2].ToolCallID != "call_1" || got[2].ToolName != "bash" || got[2].Content != "/tmp/project\n" {
		t.Fatalf("tool result details not preserved: %+v", got[2])
	}
}

func TestPreviewSessionFileStripsTransientReasoningLanguageBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	s := agent.NewSession("system")
	s.Add(provider.Message{Role: provider.RoleUser, Content: "<reasoning-language>\nVisible reasoning/thinking text preference: use English.\n</reasoning-language>\n\nExplain this module"})
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}

	preview, turns := previewSessionFile(path)
	if turns != 1 {
		t.Errorf("turns = %d, want 1", turns)
	}
	if preview != "Explain this module" {
		t.Errorf("preview = %q, want user prompt", preview)
	}
}

func TestServeCancelEndpoint(t *testing.T) {
	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Sink: bc})
	srv := httptest.NewServer(New(ctrl, bc, config.ServeConfig{}).Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/cancel", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("cancel status = %d, want 204", resp.StatusCode)
	}
}

func TestServeApproveMissingID(t *testing.T) {
	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Sink: bc})
	srv := httptest.NewServer(New(ctrl, bc, config.ServeConfig{}).Handler())
	defer srv.Close()

	// Missing id should return 400.
	resp, err := http.Post(srv.URL+"/approve", "application/json", strings.NewReader(`{"allow":true}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("approve missing id = %d, want 400", resp.StatusCode)
	}

	// Malformed JSON should return 400.
	resp2, _ := http.Post(srv.URL+"/approve", "application/json", strings.NewReader(`{bad`))
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("approve bad json = %d, want 400", resp2.StatusCode)
	}
}

func TestServeNewSessionEndpoint(t *testing.T) {
	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Sink: bc})
	srv := httptest.NewServer(New(ctrl, bc, config.ServeConfig{}).Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/new", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("new session = %d, want 204", resp.StatusCode)
	}
}

func TestServeCompactEndpoint(t *testing.T) {
	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Sink: bc})
	srv := httptest.NewServer(New(ctrl, bc, config.ServeConfig{}).Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/compact", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("compact = %d, want 204", resp.StatusCode)
	}
}

func TestServeHasNoBrowserPage(t *testing.T) {
	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Sink: bc})
	srv := httptest.NewServer(New(ctrl, bc, config.ServeConfig{}).Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("index status = %d, want 404", resp.StatusCode)
	}
}

func TestResumeRequiresSessionPathInsideSessionDir(t *testing.T) {
	dir := t.TempDir()
	active := filepath.Join(dir, "active.jsonl")
	inside := filepath.Join(dir, "inside.jsonl")
	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "outside.jsonl")
	for _, path := range []string{active, inside, outside} {
		if err := os.WriteFile(path, []byte(`{"role":"user","content":"hi"}`+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Sink: bc, SessionDir: dir, SessionPath: active})
	srv := httptest.NewServer(New(ctrl, bc, config.ServeConfig{}).Handler())
	defer srv.Close()

	post := func(path string) int {
		body, err := json.Marshal(map[string]string{"path": path})
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.Post(srv.URL+"/resume", "application/json", strings.NewReader(string(body)))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if got := post(outside); got != http.StatusForbidden {
		t.Fatalf("outside resume status = %d, want 403", got)
	}
	if got := post(inside); got != http.StatusNoContent {
		t.Fatalf("inside resume status = %d, want 204", got)
	}
	want, err := filepath.EvalSymlinks(inside)
	if err != nil {
		t.Fatal(err)
	}
	if got := filepath.Clean(ctrl.SessionPath()); got != filepath.Clean(want) {
		t.Fatalf("session path = %q, want %q", got, want)
	}
}

func TestResumeResolvesRelativePathWithinCurrentSessionDir(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "default-profile")
	otherDir := filepath.Join(root, "other-profile")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(otherDir, 0o755); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(dir, "active.jsonl")
	inside := filepath.Join(dir, "inside.jsonl")
	outside := filepath.Join(otherDir, "outside.jsonl")
	for _, path := range []string{active, inside, outside} {
		if err := os.WriteFile(path, []byte(`{"role":"user","content":"hi"}`+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Sink: bc, SessionDir: dir, SessionPath: active})
	srv := httptest.NewServer(New(ctrl, bc, config.ServeConfig{}).Handler())
	defer srv.Close()

	post := func(path string) int {
		body, err := json.Marshal(map[string]string{"path": path})
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.Post(srv.URL+"/resume", "application/json", strings.NewReader(string(body)))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if got := post("inside.jsonl"); got != http.StatusNoContent {
		t.Fatalf("relative inside resume status = %d, want 204", got)
	}
	if got := ctrl.SessionPath(); !sameCleanPath(got, inside) {
		t.Fatalf("session path = %q, want %q", got, inside)
	}
	if got := post("../other-profile/outside.jsonl"); got != http.StatusForbidden {
		t.Fatalf("relative outside resume status = %d, want 403", got)
	}
	if got := ctrl.SessionPath(); !sameCleanPath(got, inside) {
		t.Fatalf("session path after rejected resume = %q, want %q", got, inside)
	}
}

func TestResumeRejectsCleanupPendingSession(t *testing.T) {
	dir := t.TempDir()
	active := filepath.Join(dir, "active.jsonl")
	pending := filepath.Join(dir, "pending.jsonl")
	for _, path := range []string{active, pending} {
		if err := os.WriteFile(path, []byte(`{"role":"user","content":"hi"}`+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := agent.MarkCleanupPending(pending, "delete"); err != nil {
		t.Fatal(err)
	}

	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Sink: bc, SessionDir: dir, SessionPath: active})
	srv := httptest.NewServer(New(ctrl, bc, config.ServeConfig{}).Handler())
	defer srv.Close()

	body, err := json.Marshal(map[string]string{"path": pending})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(srv.URL+"/resume", "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("cleanup-pending resume status = %d, want 400", resp.StatusCode)
	}
	if got := filepath.Clean(ctrl.SessionPath()); got != filepath.Clean(active) {
		t.Fatalf("session path after rejected resume = %q, want active %q", got, active)
	}
}

func TestSessionsSkipsCleanupPending(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "default-profile")
	otherDir := filepath.Join(root, "other-profile")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(otherDir, 0o755); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(dir, "active.jsonl")
	pending := filepath.Join(dir, "pending.jsonl")
	for _, path := range []string{active, pending} {
		if err := os.WriteFile(path, []byte(`{"role":"user","content":"hi"}`+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	other := filepath.Join(otherDir, "other.jsonl")
	if err := os.WriteFile(other, []byte(`{"role":"user","content":"other profile"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := agent.MarkCleanupPending(pending, "delete"); err != nil {
		t.Fatal(err)
	}

	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Sink: bc, SessionDir: dir, SessionPath: active})
	srv := httptest.NewServer(New(ctrl, bc, config.ServeConfig{}).Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/sessions")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var got []struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "active" || filepath.Clean(got[0].Path) != filepath.Clean(active) {
		t.Fatalf("/sessions = %+v, want only active session", got)
	}
}

func TestServeSessionEndpointsUseControllerSessionDirWithTokenAuth(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "profile-a")
	otherDir := filepath.Join(root, "profile-b")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(otherDir, 0o755); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(dir, "active.jsonl")
	resume := filepath.Join(dir, "resume.jsonl")
	old := filepath.Join(dir, "old.jsonl")
	other := filepath.Join(otherDir, "other.jsonl")
	for _, path := range []string{active, resume, old, other} {
		if err := os.WriteFile(path, []byte(`{"role":"user","content":"hi"}`+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	bc := NewBroadcaster()
	ctrl := control.New(control.Options{
		Sink:        bc,
		SessionDir:  dir,
		SessionPath: active,
		Actor: userctx.ActorContext{
			ActorID:    "serve",
			ProfileID:  "profile-a",
			AuthMethod: "serve-token",
		},
	})
	s := New(ctrl, bc, config.ServeConfig{AuthMode: "token", Token: "secret"})
	if got := s.AuthActorContext(); got.ActorID != "serve" || got.ProfileID != "profile-a" || got.AuthMethod != "serve-token" {
		t.Fatalf("AuthActorContext() = %+v", got)
	}
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	do := func(method, path, body string) *http.Response {
		t.Helper()
		var reader io.Reader
		if body != "" {
			reader = strings.NewReader(body)
		}
		req, err := http.NewRequest(method, srv.URL+path, reader)
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(&http.Cookie{Name: cookieToken, Value: "secret"})
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	resp := do(http.MethodGet, "/sessions", "")
	var sessions []struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	for _, got := range sessions {
		if filepath.Dir(filepath.Clean(got.Path)) != filepath.Clean(dir) {
			t.Fatalf("/sessions included path outside controller dir: %+v", sessions)
		}
	}

	resp = do(http.MethodPost, "/resume", `{"path":"../profile-b/other.jsonl"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("outside resume status = %d, want 403", resp.StatusCode)
	}
	resp = do(http.MethodPost, "/resume", `{"path":"resume.jsonl"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("inside resume status = %d, want 204", resp.StatusCode)
	}
	if got := ctrl.SessionPath(); !sameCleanPath(got, resume) {
		t.Fatalf("session path = %q, want %q", got, resume)
	}

	resp = do(http.MethodPost, "/delete-session", `{"name":"old"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete old status = %d, want 204", resp.StatusCode)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old session still exists or stat failed unexpectedly: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("other profile session should remain: %v", err)
	}
}

func TestDeleteSessionRequiresSessionNameInsideSessionDir(t *testing.T) {
	dir := t.TempDir()
	active := filepath.Join(dir, "active.jsonl")
	old := filepath.Join(dir, "old.jsonl")
	for _, path := range []string{active, old} {
		if err := os.WriteFile(path, []byte(`{"role":"user","content":"hi"}`+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ref := "sa_20260102_030405_000000000_aabbccddeeff"
	writeServeSubagentArtifact(t, dir, ref, agent.BranchID(old))
	oldJobsDir := jobs.ArtifactDir(old)
	if err := os.MkdirAll(oldJobsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldJobsDir, "bash-1.log"), []byte("output"), 0o644); err != nil {
		t.Fatal(err)
	}
	sibling := dir + "-other"
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	escape := filepath.Join(sibling, "escape.jsonl")
	if err := os.WriteFile(escape, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Sink: bc, SessionDir: dir, SessionPath: active})
	srv := httptest.NewServer(New(ctrl, bc, config.ServeConfig{}).Handler())
	defer srv.Close()

	post := func(body string) int {
		resp, err := http.Post(srv.URL+"/delete-session", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if got := post(`{"path":"` + escape + `"}`); got != http.StatusBadRequest {
		t.Fatalf("legacy path delete status = %d, want 400", got)
	}
	if got := post(`{"name":"../` + filepath.Base(sibling) + `/escape"}`); got != http.StatusBadRequest {
		t.Fatalf("sibling traversal status = %d, want 400", got)
	}
	if _, err := os.Stat(escape); err != nil {
		t.Fatalf("sibling session was removed: %v", err)
	}
	if got := post(`{"name":"active"}`); got != http.StatusConflict {
		t.Fatalf("active delete status = %d, want 409", got)
	}
	if got := post(`{"name":"old"}`); got != http.StatusNoContent {
		t.Fatalf("valid delete status = %d, want 204", got)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old session still exists or stat failed unexpectedly: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "subagents", ref+".jsonl")); !os.IsNotExist(err) {
		t.Fatalf("old session subagent jsonl still exists or stat failed unexpectedly: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "subagents", ref+".meta.json")); !os.IsNotExist(err) {
		t.Fatalf("old session subagent meta still exists or stat failed unexpectedly: %v", err)
	}
	if _, err := os.Stat(oldJobsDir); !os.IsNotExist(err) {
		t.Fatalf("old session jobs sidecar still exists or stat failed unexpectedly: %v", err)
	}
}

func writeServeSubagentArtifact(t *testing.T, dir, ref, parentSession string) {
	t.Helper()
	subagentDir := filepath.Join(dir, "subagents")
	if err := os.MkdirAll(subagentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subagentDir, ref+".jsonl"), []byte(`{"role":"user","content":"sub"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(agent.SubagentMeta{
		Ref:           ref,
		Status:        agent.SubagentCompleted,
		Kind:          "task",
		Name:          "task",
		ParentSession: parentSession,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subagentDir, ref+".meta.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestServeSubmitMalformedJSON(t *testing.T) {
	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Sink: bc})
	srv := httptest.NewServer(New(ctrl, bc, config.ServeConfig{}).Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/submit", "application/json", strings.NewReader(`{not json`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("malformed submit = %d, want 400", resp.StatusCode)
	}
}

func TestServePlanMalformedJSON(t *testing.T) {
	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Sink: bc})
	srv := httptest.NewServer(New(ctrl, bc, config.ServeConfig{}).Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/plan", "application/json", strings.NewReader(`{bad`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("malformed plan = %d, want 400", resp.StatusCode)
	}
}

func TestServeRejectsLargeJSONBody(t *testing.T) {
	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Sink: bc})
	srv := httptest.NewServer(New(ctrl, bc, config.ServeConfig{}).Handler())
	defer srv.Close()

	body := `{"on":true,"padding":"` + strings.Repeat("x", 1<<20) + `"}`
	resp, err := http.Post(srv.URL+"/plan", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("large plan body status = %d, want 413", resp.StatusCode)
	}
}

func TestServeContextEndpoint(t *testing.T) {
	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Sink: bc})
	srv := httptest.NewServer(New(ctrl, bc, config.ServeConfig{}).Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/context")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("context status = %d", resp.StatusCode)
	}
	var body map[string]int
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode context: %v", err)
	}
	// Before any turn, used should be 0.
	if body["used"] != 0 {
		t.Errorf("used = %d, want 0", body["used"])
	}
}
