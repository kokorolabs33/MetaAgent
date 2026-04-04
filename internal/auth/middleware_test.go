package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"taskhub/internal/ctxutil"
)

func TestRequireAuth_LocalMode_InjectsLocalUser(t *testing.T) {
	m := &Middleware{LocalMode: true}

	var gotUserID, gotRole string
	handler := m.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := ctxutil.UserFromCtx(r.Context())
		if u != nil {
			gotUserID = u.ID
		}
		gotRole = ctxutil.RoleFromCtx(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if gotUserID != "local-user" {
		t.Errorf("user ID = %q, want 'local-user'", gotUserID)
	}
	if gotRole != "admin" {
		t.Errorf("role = %q, want 'admin'", gotRole)
	}
}

func TestRequireAuth_LocalMode_UserFields(t *testing.T) {
	m := &Middleware{LocalMode: true}

	var gotEmail, gotName string
	handler := m.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := ctxutil.UserFromCtx(r.Context())
		if u != nil {
			gotEmail = u.Email
			gotName = u.Name
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if gotEmail != "local@taskhub.local" {
		t.Errorf("email = %q, want 'local@taskhub.local'", gotEmail)
	}
	if gotName != "Local User" {
		t.Errorf("name = %q, want 'Local User'", gotName)
	}
}

func TestRequireAuth_CloudMode_NoCookie_Returns401(t *testing.T) {
	m := &Middleware{
		LocalMode: false,
		Sessions:  nil, // won't be reached
	}

	handler := m.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without cookie")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}

	// Check response body
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != "unauthorized" {
		t.Errorf("error = %q, want 'unauthorized'", body["error"])
	}
}

func TestRequireAuth_CloudMode_InvalidCookieName_Returns401(t *testing.T) {
	m := &Middleware{
		LocalMode: false,
		Sessions:  nil,
	}

	handler := m.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called with wrong cookie")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	req.AddCookie(&http.Cookie{Name: "wrong_cookie", Value: "some-value"})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestRequireAuth_CloudMode_ResponseContentType(t *testing.T) {
	m := &Middleware{
		LocalMode: false,
		Sessions:  nil,
	}

	handler := m.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestRequireAuth_LocalMode_PassesThroughToHandler(t *testing.T) {
	m := &Middleware{LocalMode: true}

	called := false
	handler := m.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("expected handler to be called in local mode")
	}
}
