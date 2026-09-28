package serve

import (
	"context"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"kwu/internal/config"
	"kwu/internal/userctx"
)

// authMode represents the authentication mode for the serve API.
type authMode int

const (
	authInvalid  authMode = iota // invalid config; deny all requests
	authNone                     // no authentication (default, backward-compatible)
	authToken                    // pre-shared token in URL or cookie
	authPassword                 // password exchange with bcrypt verification
)

const (
	cookieToken     = "kwu_token"         // holds the token for token mode
	cookieSession   = "kwu_session"       // holds the HMAC-signed session for password mode
	tokenByteLen    = 32                  // 256-bit random token
	sessionDuration = 30 * 24 * time.Hour // how long a password session lasts
	bcryptCost      = 12                  // bcrypt cost factor
	pbkdf2Iter      = 4096                // deterministic session-key derivation from password_hash
)

// NormalizeAuthMode normalizes and validates the serve auth mode.
func NormalizeAuthMode(mode string) (string, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "none"
	}
	switch mode {
	case "none", "token", "password":
		return mode, nil
	default:
		return "", fmt.Errorf("auth mode must be none, token, or password, got %q", mode)
	}
}

// ActorContextForServe maps the public serve auth mode to a controlled runtime
// actor using the default local profile. Auth secrets never participate in the
// internal actor/profile identity.
func ActorContextForServe(authMode, workspaceRoot string) (userctx.ActorContext, error) {
	return ActorContextForServeWithActor(authMode, userctx.Default(workspaceRoot))
}

// ActorContextForServeWithActor maps the public serve auth mode onto an actor
// derived from the caller's current profile. "none" keeps the caller actor for
// local compatibility; token/password use a single serve actor within that same
// profile.
func ActorContextForServeWithActor(authMode string, actor userctx.ActorContext) (userctx.ActorContext, error) {
	mode, err := NormalizeAuthMode(authMode)
	if err != nil {
		return userctx.ActorContext{}, err
	}
	actor = actor.Normalize()
	switch mode {
	case "token", "password":
		return serveAuthActor(actor, mode), nil
	default:
		return actor, nil
	}
}

// rateLimit tracks login attempts per IP for brute-force protection.
type rateLimit struct {
	mu       sync.Mutex
	attempts map[string]*rateWindow
}

type rateWindow struct {
	count int
	start time.Time
}

const (
	rateLimitMax = 5
	rateLimitWin = time.Minute
)

func newRateLimit() *rateLimit {
	rl := &rateLimit{attempts: make(map[string]*rateWindow)}
	go rl.cleanupLoop()
	return rl
}

// cleanupLoop periodically purges expired rate-limit windows so the map does not
// grow without bound over the lifetime of a long-running server.
func (rl *rateLimit) cleanupLoop() {
	ticker := time.NewTicker(2 * rateLimitWin)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, w := range rl.attempts {
			if now.Sub(w.start) > rateLimitWin {
				delete(rl.attempts, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// allow reports whether the IP is allowed to attempt login. It also cleans up
// expired windows.
func (rl *rateLimit) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	w, ok := rl.attempts[ip]
	if !ok || now.Sub(w.start) > rateLimitWin {
		rl.attempts[ip] = &rateWindow{count: 1, start: now}
		return true
	}
	w.count++
	return w.count <= rateLimitMax
}

// authGate is the authentication middleware and its runtime state.
type authGate struct {
	mode         authMode
	actor        userctx.ActorContext
	token        string     // pre-shared token (token mode)
	passwordHash string     // bcrypt hash for password verification (password mode)
	sessKey      []byte     // HMAC key for session signing (password mode, generated at startup)
	behindProxy  bool       // trust X-Forwarded-For / X-Forwarded-Proto headers
	rateLimit    *rateLimit // per-IP rate limiter for /login
}

type serveActorContextKey struct{}

// newAuthGate creates the auth middleware from the serve config. For token mode
// without a configured token, it generates a random one.
func newAuthGate(cfg config.ServeConfig, defaultActor userctx.ActorContext) *authGate {
	ag := &authGate{
		actor:       defaultActor.Normalize(),
		rateLimit:   newRateLimit(),
		behindProxy: cfg.BehindProxy,
	}
	mode, err := NormalizeAuthMode(cfg.AuthMode)
	if err != nil {
		ag.mode = authInvalid
		return ag
	}
	switch mode {
	case "token":
		ag.mode = authToken
		ag.actor = serveAuthActor(ag.actor, "token")
		ag.token = strings.TrimSpace(cfg.Token)
		if ag.token == "" {
			ag.token = generateToken()
		}
	case "password":
		ag.mode = authPassword
		ag.actor = serveAuthActor(ag.actor, "password")
		ag.passwordHash = strings.TrimSpace(cfg.PasswordHash)
		ag.sessKey = sessionKeyForPasswordHash(ag.passwordHash)
	default:
		ag.mode = authNone
	}
	return ag
}

func serveAuthActor(actor userctx.ActorContext, mode string) userctx.ActorContext {
	actor = actor.Normalize()
	actor.ActorID = "serve"
	actor.AuthMethod = "serve-" + mode
	actor.RemoteIdentity = nil
	return actor.Normalize()
}

// ActorContext returns the single runtime actor bound to this auth gate.
func (ag *authGate) ActorContext() userctx.ActorContext { return cloneServeActorContext(ag.actor) }

func cloneServeActorContext(actor userctx.ActorContext) userctx.ActorContext {
	if actor.RemoteIdentity != nil {
		remote := *actor.RemoteIdentity
		actor.RemoteIdentity = &remote
	}
	return actor
}

func actorContextFromRequest(r *http.Request) (userctx.ActorContext, bool) {
	actor, ok := r.Context().Value(serveActorContextKey{}).(userctx.ActorContext)
	if !ok {
		return userctx.ActorContext{}, false
	}
	return cloneServeActorContext(actor), true
}

// Token returns the shared token (empty if not in token mode).
func (ag *authGate) Token() string { return ag.token }

// Mode returns the auth mode name as a string.
func (ag *authGate) Mode() string {
	switch ag.mode {
	case authToken:
		return "token"
	case authPassword:
		return "password"
	case authInvalid:
		return "invalid"
	default:
		return "none"
	}
}

// HashPassword returns a bcrypt hash of the given password. Exported for use by
// the CLI `--hash-password` flag.
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func sessionKeyForPasswordHash(passwordHash string) []byte {
	if passwordHash != "" {
		key, err := pbkdf2.Key(sha256.New, passwordHash, []byte("kwu serve session key"), pbkdf2Iter, 32)
		if err != nil {
			panic("serve/auth: pbkdf2 failed: " + err.Error())
		}
		return key
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		// crypto/rand.Read cannot fail on modern systems; panic rather than
		// fall back to a deterministic key that would weaken every session.
		panic("serve/auth: crypto/rand.Read failed: " + err.Error())
	}
	return key
}

// middleware returns an http.Handler that wraps next with authentication checks.
// In password mode, POST /login is handled directly because it accepts a form
// body while the command API requires JSON.
func (ag *authGate) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(context.WithValue(r.Context(), serveActorContextKey{}, ag.ActorContext()))
		if ag.mode == authInvalid {
			ag.deny(w, r)
			return
		}
		if ag.mode == authNone {
			next.ServeHTTP(w, r)
			return
		}
		// /login and /login/ are handled directly by the auth gate — they must
		// not pass through the CSRF guard (which rejects non-JSON POSTs).
		if r.URL.Path == "/login" || r.URL.Path == "/login/" {
			ag.handleLogin(w, r)
			return
		}
		if ag.mode == authToken {
			ag.checkToken(w, r, next)
			return
		}
		// password mode
		ag.checkSession(w, r, next)
	})
}

// checkToken validates the token from cookie or query parameter. If the query
// parameter is valid, it sets a cookie and redirects to strip the token from the
// URL (preventing it from leaking via browser history or referrer headers).
func (ag *authGate) checkToken(w http.ResponseWriter, r *http.Request, next http.Handler) {
	// 1. Check cookie first (fast path).
	if c, err := r.Cookie(cookieToken); err == nil && strings.TrimSpace(c.Value) != "" {
		if subtle.ConstantTimeCompare([]byte(c.Value), []byte(ag.token)) == 1 {
			next.ServeHTTP(w, r)
			return
		}
	}

	// 2. Check query parameter.
	if q := r.URL.Query().Get("token"); q != "" {
		if subtle.ConstantTimeCompare([]byte(q), []byte(ag.token)) == 1 {
			// Set a persistent cookie so future requests (including SSE) are
			// authenticated without the token in the URL.
			ag.setAuthCookie(w, r, &http.Cookie{
				Name:     cookieToken,
				Value:    ag.token,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
				MaxAge:   int(sessionDuration.Seconds()),
			})
			// Redirect to the same path without the token query parameter.
			cleanURL := *r.URL
			qry := cleanURL.Query()
			qry.Del("token")
			cleanURL.RawQuery = qry.Encode()
			if cleanURL.RawQuery == "" {
				cleanURL.RawQuery = ""
			}
			redirectToSafeTarget(w, r, cleanURL.RequestURI(), http.StatusFound)
			return
		}
	}

	// 3. Not authenticated.
	ag.deny(w, r)
}

// checkSession validates the HMAC-signed session cookie for password mode.
// Unauthenticated requests receive 401; the serve package exposes no login page.
func (ag *authGate) checkSession(w http.ResponseWriter, r *http.Request, next http.Handler) {
	// Check session cookie.
	if c, err := r.Cookie(cookieSession); err == nil {
		if ag.verifySession(c.Value) {
			next.ServeHTTP(w, r)
			return
		}
	}

	ag.deny(w, r)
}

// deny sends a 401 response. The message is intentionally generic to avoid
// leaking information about which auth mode is active.
func (ag *authGate) deny(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte("Unauthorized\n"))
}

// handleLogin exchanges a password for a signed session cookie. No HTML login
// page is served.
func (ag *authGate) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	ag.loginSubmit(w, r)
}

// loginSubmit verifies the password and issues a session cookie.
func (ag *authGate) loginSubmit(w http.ResponseWriter, r *http.Request) {
	// Rate limit.
	ip := ag.clientIP(r)
	if !ag.rateLimit.allow(ip) {
		slog.Warn("serve/auth: rate-limited login attempt", "ip", ip)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("尝试次数过多，请稍后再试。\n"))
		return
	}

	// Parse the password from the form.
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	password := r.FormValue("password")
	if password == "" {
		ag.deny(w, r)
		return
	}

	// Verify against the stored bcrypt hash.
	if ag.passwordHash == "" {
		slog.Error("serve/auth: cannot verify password — no password_hash configured")
		ag.deny(w, r)
		return
	}

	// Verify against bcrypt hash.
	if err := bcrypt.CompareHashAndPassword([]byte(ag.passwordHash), []byte(password)); err != nil {
		ag.deny(w, r)
		return
	}

	// Create and sign a session.
	session := ag.signSession()

	// Set the authenticated session cookie.
	ag.setAuthCookie(w, r, &http.Cookie{
		Name:     cookieSession,
		Value:    session,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionDuration.Seconds()),
	})

	w.WriteHeader(http.StatusNoContent)
}

func (ag *authGate) setAuthCookie(w http.ResponseWriter, r *http.Request, c *http.Cookie) {
	c.Secure = ag.authCookieSecure(r)
	// codeql[go/cookie-secure-not-set] Loopback HTTP serve must keep local browser auth usable; non-loopback HTTP still sets Secure cookies.
	http.SetCookie(w, c)
}

func (ag *authGate) authCookieSecure(r *http.Request) bool {
	if ag.isTLS(r) {
		return true
	}
	host := r.Host
	if host == "" && r.URL != nil {
		host = r.URL.Host
	}
	return !isLoopbackHost(host)
}

func safeRedirectTarget(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.ReplaceAll(raw, "\\", "/")
	if i := strings.IndexByte(raw, '#'); i >= 0 {
		raw = raw[:i]
	}
	if raw == "" {
		return "/"
	}
	if raw != "/" && (len(raw) <= 1 || raw[0] != '/' || raw[1] == '/' || raw[1] == '\\') {
		return "/"
	}
	u, err := url.Parse(raw)
	if err != nil || u == nil || u.IsAbs() || u.Hostname() != "" {
		return "/"
	}
	path := strings.ReplaceAll(u.Path, "\\", "/")
	if path == "" {
		return "/"
	}
	if path != "/" && (len(path) <= 1 || path[0] != '/' || path[1] == '/' || path[1] == '\\') {
		return "/"
	}
	return u.RequestURI()
}

func redirectToSafeTarget(w http.ResponseWriter, r *http.Request, raw string, status int) {
	target := safeRedirectTarget(raw)
	target = strings.ReplaceAll(target, "\\", "/")
	u, err := url.Parse(target)
	if err == nil && u != nil && !u.IsAbs() && u.Hostname() == "" {
		redirect := u.RequestURI()
		if redirect == "/" {
			http.Redirect(w, r, "/", status)
			return
		}
		if len(redirect) > 1 && redirect[0] == '/' && redirect[1] != '/' && redirect[1] != '\\' {
			http.Redirect(w, r, redirect, status)
			return
		}
	}
	http.Redirect(w, r, "/", status)
}

// signSession creates a new HMAC-signed session token valid for sessionDuration.
// Format: base64url(expiry_base10|random_16_bytes).hex(hmac_sha256)
func (ag *authGate) signSession() string {
	expiry := time.Now().Add(sessionDuration).Unix()
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		// crypto/rand.Read cannot fail on modern systems; panic rather than
		// fall back to an all-zero nonce. Forging a cookie still requires the
		// PBKDF2-derived sessKey, so this is not an auth bypass, but a constant
		// nonce weakens session token uniqueness/unpredictability and is the
		// same anti-pattern generateToken/sessionKeyForPasswordHash panic on.
		panic("serve/auth: crypto/rand.Read failed: " + err.Error())
	}

	payload := strconv.FormatInt(expiry, 10) + "|" + base64.RawURLEncoding.EncodeToString(nonce)
	mac := hmac.New(sha256.New, ag.sessKey)
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))

	return payload + "." + sig
}

// verifySession checks that a session token is valid (HMAC matches and not expired).
func (ag *authGate) verifySession(token string) bool {
	// Split payload.signature
	dot := strings.LastIndexByte(token, '.')
	if dot < 0 {
		return false
	}
	payload, sigHex := token[:dot], token[dot+1:]

	// Verify HMAC (constant-time via hmac.Equal; handles length mismatch
	// internally so we don't leak timing information from a pre-check).
	mac := hmac.New(sha256.New, ag.sessKey)
	mac.Write([]byte(payload))
	expected := mac.Sum(nil)
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}
	if !hmac.Equal(sig, expected) {
		return false
	}

	// Check expiry (format: "unix_timestamp|base64nonce").
	pipe := strings.IndexByte(payload, '|')
	if pipe < 0 {
		return false
	}
	expiry, err := strconv.ParseInt(payload[:pipe], 10, 64)
	if err != nil {
		return false
	}
	return time.Now().Unix() < expiry
}

// generateToken returns a cryptographically random URL-safe token.
func generateToken() string {
	b := make([]byte, tokenByteLen)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand.Read failure is fatal for token generation.
		panic("serve/auth: crypto/rand.Read failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// clientIP extracts the client IP from the request. When behindProxy is true,
// it trusts the leftmost entry in X-Forwarded-For (set by a trusted reverse
// proxy). Otherwise it uses RemoteAddr directly — X-Forwarded-For is ignored
// because an attacker can forge it.
func (ag *authGate) clientIP(r *http.Request) string {
	if ag.behindProxy {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			if i := strings.IndexByte(fwd, ','); i >= 0 {
				return strings.TrimSpace(fwd[:i])
			}
			return strings.TrimSpace(fwd)
		}
	}
	// Strip port from RemoteAddr.
	addr := r.RemoteAddr
	if i := strings.LastIndexByte(addr, ':'); i >= 0 {
		return addr[:i]
	}
	return addr
}

// isTLS reports whether the request arrived over TLS. It trusts
// X-Forwarded-Proto only when behindProxy is true.
func (ag *authGate) isTLS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if ag.behindProxy {
		return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	}
	return false
}

func isLoopbackHost(hostport string) bool {
	hostport = strings.TrimSpace(hostport)
	if hostport == "" {
		return false
	}
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
