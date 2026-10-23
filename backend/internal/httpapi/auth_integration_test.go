package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"mercado/internal/config"
	"mercado/internal/ratelimit"
	"mercado/internal/reference"
	"mercado/internal/store"
	"mercado/migrations"
)

func authTestServer(t *testing.T, mutate func(*Deps)) *Server {
	t.Helper()
	if !dockerAvailable(t) {
		t.Skip("Docker is not available")
	}

	ctx := context.Background()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:16-alpine",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_USER":     "mercado",
				"POSTGRES_PASSWORD": "mercado",
				"POSTGRES_DB":       "mercado",
			},
			WaitingFor: wait.ForAll(
				wait.ForListeningPort("5432/tcp").WithStartupTimeout(90*time.Second),
				wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(90*time.Second),
			),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("container port: %v", err)
	}
	dsn := fmt.Sprintf("postgres://mercado:mercado@%s:%s/mercado?sslmode=disable", host, port.Port())

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set dialect: %v", err)
	}
	if err := goose.Up(db, "."); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := reference.Seed(ctx, pool, 1); err != nil {
		t.Fatalf("seed reference data: %v", err)
	}

	deps := Deps{
		Config: config.Config{
			ServiceName:       "mercado-api",
			Version:           "test",
			Commit:            "test",
			ReadinessTimeout:  time.Second,
			MaxBodyBytes:      1 << 20,
			ShutdownTimeout:   time.Second,
			RateLimitEnabled:  false,
			SessionCookieName: "mercado_session",
			SessionTTL:        time.Hour,
			AllowedOrigins:    []string{"http://web.test"},
			LoginRateLimit:    10,
		},
		Store: store.New(pool),
	}
	if mutate != nil {
		mutate(&deps)
	}
	return New(deps)
}

func dockerAvailable(t *testing.T) bool {
	t.Helper()
	if os.Getenv("DOCKER_HOST") != "" {
		return true
	}
	_, err := os.Stat("/var/run/docker.sock")
	return err == nil
}

func authJSON(method, path, body string) *http.Request {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://web.test")
	return request
}

func registerAndLogin(t *testing.T, server *Server, email string) (UserCookie, store.User) {
	t.Helper()
	response := perform(server.Handler(), authJSON(http.MethodPost, "/v1/auth/register",
		fmt.Sprintf(`{"name":"Ana","email":%q,"password":"senha-super-segura"}`, email)))
	if response.Code != http.StatusCreated {
		t.Fatalf("register = %d: %s", response.Code, response.Body.String())
	}
	var user store.User
	if err := json.Unmarshal(response.Body.Bytes(), &user); err != nil {
		t.Fatalf("decode user: %v", err)
	}

	response = perform(server.Handler(), authJSON(http.MethodPost, "/v1/auth/login",
		fmt.Sprintf(`{"email":%q,"password":"senha-super-segura"}`, email)))
	if response.Code != http.StatusOK {
		t.Fatalf("login = %d: %s", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("login did not set a session cookie")
	}
	return UserCookie{Name: cookies[0].Name, Value: cookies[0].Value, Secure: cookies[0].Secure, HttpOnly: cookies[0].HttpOnly, SameSite: cookies[0].SameSite}, user
}

type UserCookie struct {
	Name     string
	Value    string
	Secure   bool
	HttpOnly bool
	SameSite http.SameSite
}

func TestRegisterLoginMeLogout(t *testing.T) {
	server := authTestServer(t, nil)
	cookie, user := registerAndLogin(t, server, "ana@example.com")

	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("unexpected cookie attributes: %+v", cookie)
	}
	if cookie.Secure {
		t.Fatal("local http cookie should not be Secure")
	}

	me := authJSON(http.MethodGet, "/v1/auth/me", "")
	me.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
	response := perform(server.Handler(), me)
	if response.Code != http.StatusOK {
		t.Fatalf("GET /me = %d: %s", response.Code, response.Body.String())
	}
	var current userResponse
	if err := json.Unmarshal(response.Body.Bytes(), &current); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	if current.ID != user.ID || len(current.Roles) != 1 || current.Roles[0] != "buyer" {
		t.Fatalf("unexpected me: %+v", current)
	}

	logout := authJSON(http.MethodPost, "/v1/auth/logout", "")
	logout.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
	response = perform(server.Handler(), logout)
	if response.Code != http.StatusNoContent {
		t.Fatalf("logout = %d: %s", response.Code, response.Body.String())
	}
	if cleared := response.Result().Cookies(); len(cleared) == 0 || cleared[0].MaxAge >= 0 {
		t.Fatalf("logout did not clear cookie: %+v", cleared)
	}

	reused := authJSON(http.MethodGet, "/v1/auth/me", "")
	reused.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
	if response := perform(server.Handler(), reused); response.Code != http.StatusUnauthorized {
		t.Fatalf("reused session = %d, want 401", response.Code)
	}
}

func TestRegisterValidationAndEnumeration(t *testing.T) {
	server := authTestServer(t, nil)
	cookie, _ := registerAndLogin(t, server, "ana@example.com")

	response := perform(server.Handler(), authJSON(http.MethodPost, "/v1/auth/register",
		`{"name":"Ana","email":"ANA@example.com","password":"senha-super-segura"}`))
	if response.Code != http.StatusConflict || decodeError(t, response).Error.Code != "email_already_registered" {
		t.Fatalf("duplicate email = %d: %s", response.Code, response.Body.String())
	}

	response = perform(server.Handler(), authJSON(http.MethodPost, "/v1/auth/register",
		`{"name":"Ana","email":"outra@example.com","password":"curta"}`))
	body := decodeError(t, response)
	if response.Code != http.StatusBadRequest || body.Error.Code != "validation_error" || len(body.Error.Details) != 1 || body.Error.Details[0].Field != "password" {
		t.Fatalf("weak password = %d: %s", response.Code, response.Body.String())
	}

	response = perform(server.Handler(), authJSON(http.MethodPost, "/v1/auth/register",
		`{"name":"Ana","email":"invalido","password":"senha-super-segura"}`))
	body = decodeError(t, response)
	if response.Code != http.StatusBadRequest || body.Error.Details[0].Field != "email" {
		t.Fatalf("invalid email = %d: %s", response.Code, response.Body.String())
	}

	unknown := perform(server.Handler(), authJSON(http.MethodPost, "/v1/auth/login",
		`{"email":"ninguem@example.com","password":"senha-super-segura"}`))
	wrong := perform(server.Handler(), authJSON(http.MethodPost, "/v1/auth/login",
		`{"email":"ana@example.com","password":"senha-errada-mesmo"}`))
	if unknown.Code != http.StatusUnauthorized || wrong.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected login status: %d/%d", unknown.Code, wrong.Code)
	}
	if decodeError(t, unknown).Error.Message != decodeError(t, wrong).Error.Message {
		t.Fatal("login reveals whether the account exists")
	}

	logout := authJSON(http.MethodPost, "/v1/auth/logout", "")
	logout.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
	perform(server.Handler(), logout)
}

func TestRolesAndOnboarding(t *testing.T) {
	server := authTestServer(t, nil)
	cookie, user := registerAndLogin(t, server, "ana@example.com")

	adminRoute := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
	server.Router().With(server.authenticate, server.requireRole("admin")).Get("/test/admin", adminRoute)

	request := authJSON(http.MethodGet, "/test/admin", "")
	request.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
	if response := perform(server.Handler(), request); response.Code != http.StatusForbidden {
		t.Fatalf("buyer on admin route = %d, want 403", response.Code)
	}

	noSession := authJSON(http.MethodGet, "/test/admin", "")
	if response := perform(server.Handler(), noSession); response.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous on admin route = %d, want 401", response.Code)
	}

	onboarding := authJSON(http.MethodPost, "/v1/seller/onboarding",
		`{"display_name":"Loja da Ana","document":"123.456.789-01","contact":"ana@example.com"}`)
	onboarding.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
	response := perform(server.Handler(), onboarding)
	if response.Code != http.StatusCreated {
		t.Fatalf("onboarding = %d: %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "123.456.789-01") {
		t.Fatal("onboarding response exposed the document")
	}
	var profile sellerProfileResponse
	if err := json.Unmarshal(response.Body.Bytes(), &profile); err != nil {
		t.Fatalf("decode profile: %v", err)
	}

	public := perform(server.Handler(), httptest.NewRequest(http.MethodGet, "/v1/sellers/"+profile.ID.String(), nil))
	if public.Code != http.StatusOK {
		t.Fatalf("public seller = %d: %s", public.Code, public.Body.String())
	}
	if strings.Contains(public.Body.String(), "document") || strings.Contains(public.Body.String(), "contact") {
		t.Fatalf("public seller exposed private fields: %s", public.Body.String())
	}

	duplicate := authJSON(http.MethodPost, "/v1/seller/onboarding",
		`{"display_name":"Outra Loja","document":"12345678901","contact":"outra@example.com"}`)
	duplicate.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
	response = perform(server.Handler(), duplicate)
	if response.Code != http.StatusConflict || decodeError(t, response).Error.Code != "document_already_registered" {
		t.Fatalf("duplicate document = %d: %s", response.Code, response.Body.String())
	}

	missing := authJSON(http.MethodPost, "/v1/seller/onboarding", `{}`)
	missing.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
	response = perform(server.Handler(), missing)
	if response.Code != http.StatusBadRequest || len(decodeError(t, response).Error.Details) != 3 {
		t.Fatalf("missing onboarding fields = %d: %s", response.Code, response.Body.String())
	}

	if err := server.store.AddRole(context.Background(), user.ID, "admin"); err != nil {
		t.Fatalf("add admin role: %v", err)
	}
	request = authJSON(http.MethodGet, "/test/admin", "")
	request.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
	if response := perform(server.Handler(), request); response.Code != http.StatusOK {
		t.Fatalf("admin on admin route = %d, want 200", response.Code)
	}
}

func TestCSRFOriginProtection(t *testing.T) {
	server := authTestServer(t, nil)

	invalid := authJSON(http.MethodPost, "/v1/auth/register",
		`{"name":"Ana","email":"ana@example.com","password":"senha-super-segura"}`)
	invalid.Header.Set("Origin", "http://evil.test")
	response := perform(server.Handler(), invalid)
	if response.Code != http.StatusForbidden || decodeError(t, response).Error.Code != "csrf_origin_invalid" {
		t.Fatalf("invalid origin = %d: %s", response.Code, response.Body.String())
	}

	crossSite := authJSON(http.MethodPost, "/v1/auth/register",
		`{"name":"Ana","email":"ana@example.com","password":"senha-super-segura"}`)
	crossSite.Header.Del("Origin")
	crossSite.Header.Set("Sec-Fetch-Site", "cross-site")
	if response := perform(server.Handler(), crossSite); response.Code != http.StatusForbidden {
		t.Fatalf("cross-site request = %d, want 403", response.Code)
	}

	allowed := authJSON(http.MethodPost, "/v1/auth/register",
		`{"name":"Ana","email":"ana@example.com","password":"senha-super-segura"}`)
	if response := perform(server.Handler(), allowed); response.Code != http.StatusCreated {
		t.Fatalf("allowed origin = %d: %s", response.Code, response.Body.String())
	}
}

func TestLoginRateLimit(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	server := authTestServer(t, func(deps *Deps) {
		deps.Config.RateLimitEnabled = true
		deps.Config.RateLimitIP = 1000
		deps.Config.LoginRateLimit = 2
		deps.Limiter = ratelimit.New(client, time.Minute)
	})

	var last *httptest.ResponseRecorder
	for i := 0; i < 3; i++ {
		last = perform(server.Handler(), authJSON(http.MethodPost, "/v1/auth/login",
			`{"email":"ninguem@example.com","password":"senha-super-segura"}`))
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("third login = %d, want 429: %s", last.Code, last.Body.String())
	}
	if last.Header().Get("Retry-After") == "" {
		t.Fatal("429 response is missing Retry-After")
	}
}
