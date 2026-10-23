package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"mercado/internal/auth"
	"mercado/internal/store"
)

type authUserKey struct{}

type userResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Roles     []string  `json:"roles"`
	CreatedAt time.Time `json:"created_at"`
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type sellerOnboardingRequest struct {
	DisplayName string `json:"display_name"`
	Document    string `json:"document"`
	Contact     string `json:"contact"`
}

type sellerProfileResponse struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	Contact     string    `json:"contact"`
	CreatedAt   time.Time `json:"created_at"`
}

type publicSellerResponse struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

var documentCleanup = regexp.MustCompile(`[^A-Za-z0-9]`)

func toUserResponse(user store.User) userResponse {
	roles := user.Roles
	if roles == nil {
		roles = []string{}
	}
	return userResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		Roles:     roles,
		CreatedAt: user.CreatedAt,
	}
}

func CurrentUser(r *http.Request) (store.User, bool) {
	user, ok := r.Context().Value(authUserKey{}).(store.User)
	return user, ok
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var input registerRequest
	if !decodeJSON(w, r, &input) {
		return
	}

	name := strings.TrimSpace(input.Name)
	email, emailNormalized, emailValid := normalizeEmail(input.Email)
	details := make([]FieldError, 0)
	if len([]rune(name)) < 2 || len([]rune(name)) > 120 {
		details = append(details, FieldError{Field: "name", Message: "name must have between 2 and 120 characters"})
	}
	if !emailValid {
		details = append(details, FieldError{Field: "email", Message: "email must be valid"})
	}
	if len([]rune(input.Password)) < 12 || len([]rune(input.Password)) > 128 {
		details = append(details, FieldError{Field: "password", Message: "password must have between 12 and 128 characters"})
	}
	if len(details) > 0 {
		writeValidationError(w, r, details)
		return
	}

	passwordHash, err := auth.HashPassword(input.Password)
	if err != nil {
		s.logger.Error("hash password", "correlation_id", CorrelationID(r), "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	user, err := s.store.CreateUser(r.Context(), name, email, emailNormalized, passwordHash)
	if errors.Is(err, store.ErrEmailExists) {
		writeError(w, r, http.StatusConflict, "email_already_registered", "email already registered")
		return
	}
	if err != nil {
		s.logger.Error("create user", "correlation_id", CorrelationID(r), "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, toUserResponse(user))
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var input loginRequest
	if !decodeJSON(w, r, &input) {
		return
	}

	email, emailNormalized, emailValid := normalizeEmail(input.Email)
	if !emailValid || input.Password == "" {
		writeError(w, r, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		return
	}
	if s.loginRateLimited(w, r, emailNormalized) {
		return
	}

	user, passwordHash, err := s.store.UserByEmail(r.Context(), emailNormalized)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		return
	}
	if err != nil {
		s.logger.Error("select user for login", "correlation_id", CorrelationID(r), "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	valid, err := auth.VerifyPassword(input.Password, passwordHash)
	if err != nil || !valid {
		writeError(w, r, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		return
	}

	token, tokenHash, err := auth.NewSessionToken()
	if err != nil {
		s.logger.Error("generate session token", "correlation_id", CorrelationID(r), "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	expiresAt := time.Now().Add(s.cfg.SessionTTL)
	if _, err := s.store.CreateSession(r.Context(), user.ID, tokenHash, expiresAt); err != nil {
		s.logger.Error("create session", "correlation_id", CorrelationID(r), "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	s.setSessionCookie(w, r, token, expiresAt)
	user.Email = email
	writeJSON(w, http.StatusOK, toUserResponse(user))
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	token := sessionToken(r, s.cfg.SessionCookieName)
	if token != "" {
		if err := s.store.RevokeSession(r.Context(), auth.HashSessionToken(token)); err != nil {
			s.logger.Error("revoke session", "correlation_id", CorrelationID(r), "error", err)
		}
	}
	s.clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}
	writeJSON(w, http.StatusOK, toUserResponse(user))
}

func (s *Server) becomeSeller(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}

	var input sellerOnboardingRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	displayName := strings.TrimSpace(input.DisplayName)
	document := strings.TrimSpace(input.Document)
	documentNormalized := strings.ToUpper(documentCleanup.ReplaceAllString(document, ""))
	contact := strings.TrimSpace(input.Contact)

	details := make([]FieldError, 0)
	if len([]rune(displayName)) < 2 || len([]rune(displayName)) > 120 {
		details = append(details, FieldError{Field: "display_name", Message: "display name must have between 2 and 120 characters"})
	}
	if len(documentNormalized) < 11 || len(documentNormalized) > 32 {
		details = append(details, FieldError{Field: "document", Message: "document must have between 11 and 32 alphanumeric characters"})
	}
	if len([]rune(contact)) < 5 || len([]rune(contact)) > 254 {
		details = append(details, FieldError{Field: "contact", Message: "contact must have between 5 and 254 characters"})
	}
	if len(details) > 0 {
		writeValidationError(w, r, details)
		return
	}

	profile, err := s.store.CreateSellerProfile(r.Context(), user.ID, displayName, document, documentNormalized, contact)
	if errors.Is(err, store.ErrDocumentExists) {
		writeError(w, r, http.StatusConflict, "document_already_registered", "document already registered")
		return
	}
	if err != nil {
		s.logger.Error("create seller profile", "correlation_id", CorrelationID(r), "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, sellerProfileResponse{
		ID:          profile.ID,
		DisplayName: profile.DisplayName,
		Contact:     profile.Contact,
		CreatedAt:   profile.CreatedAt,
	})
}

func (s *Server) publicSeller(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "sellerId"))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "seller not found")
		return
	}
	profile, err := s.store.SellerByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "not_found", "seller not found")
		return
	}
	if err != nil {
		s.logger.Error("select public seller", "correlation_id", CorrelationID(r), "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, publicSellerResponse{
		ID:          profile.ID,
		DisplayName: profile.DisplayName,
		CreatedAt:   profile.CreatedAt,
	})
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := sessionToken(r, s.cfg.SessionCookieName)
		if token == "" {
			writeError(w, r, http.StatusUnauthorized, "unauthenticated", "authentication required")
			return
		}
		user, err := s.store.UserBySession(r.Context(), auth.HashSessionToken(token))
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusUnauthorized, "unauthenticated", "authentication required")
			return
		}
		if err != nil {
			s.logger.Error("select session", "correlation_id", CorrelationID(r), "error", err)
			writeError(w, r, http.StatusInternalServerError, "internal_error", "internal error")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), authUserKey{}, user)))
	})
}

func (s *Server) requireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := CurrentUser(r)
			if !ok {
				writeError(w, r, http.StatusUnauthorized, "unauthenticated", "authentication required")
				return
			}
			for _, candidate := range user.Roles {
				if candidate == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeError(w, r, http.StatusForbidden, "forbidden", "forbidden")
		})
	}
}

func (s *Server) csrfProtection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" {
			if !s.allowedOrigin(r, origin) {
				writeError(w, r, http.StatusForbidden, "csrf_origin_invalid", "origin not allowed")
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		if strings.EqualFold(r.Header.Get("Sec-Fetch-Site"), "cross-site") {
			writeError(w, r, http.StatusForbidden, "csrf_origin_invalid", "origin not allowed")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) allowedOrigin(r *http.Request, origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	normalized := parsed.Scheme + "://" + parsed.Host
	for _, allowed := range s.cfg.AllowedOrigins {
		if strings.EqualFold(normalized, allowed) {
			return true
		}
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return strings.EqualFold(normalized, scheme+"://"+r.Host)
}

func (s *Server) loginRateLimited(w http.ResponseWriter, r *http.Request, emailNormalized string) bool {
	if !s.cfg.RateLimitEnabled || s.limiter == nil {
		return false
	}
	key := "login:" + hashKey(emailNormalized) + ":" + hashKey(clientIP(r, s.cfg.TrustProxy))
	allowed, retryAfter, err := s.limiter.Allow(r.Context(), key, s.cfg.LoginRateLimit)
	if err != nil {
		s.metrics.rateLimitFailOpen.Inc()
		s.logger.Warn("login rate limit unavailable, failing open", "correlation_id", CorrelationID(r), "error", err)
		return false
	}
	if allowed {
		return false
	}
	seconds := int(retryAfter / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeError(w, r, http.StatusTooManyRequests, "rate_limited", "too many login attempts")
	return true
}

func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.SessionSecure || isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.SessionSecure || isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
	})
}

func sessionToken(r *http.Request, name string) string {
	cookie, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func normalizeEmail(raw string) (string, string, bool) {
	email := strings.TrimSpace(raw)
	address, err := mail.ParseAddress(email)
	if err != nil || !strings.EqualFold(address.Address, email) {
		return email, strings.ToLower(email), false
	}
	return address.Address, strings.ToLower(address.Address), true
}

func chiURLParam(r *http.Request, name string) string {
	return chi.URLParam(r, name)
}
