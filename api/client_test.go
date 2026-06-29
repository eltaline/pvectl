package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClient_defaults(t *testing.T) {
	c, err := NewClient("https://pve.example.com:8006")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.baseURL != "https://pve.example.com:8006" {
		t.Errorf("baseURL = %q, want %q", c.baseURL, "https://pve.example.com:8006")
	}
	if c.maxRetries != defaultMaxRetries {
		t.Errorf("maxRetries = %d, want %d", c.maxRetries, defaultMaxRetries)
	}
}

func TestNewClient_trailingSlash(t *testing.T) {
	c, err := NewClient("https://pve.example.com:8006/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.baseURL != "https://pve.example.com:8006" {
		t.Errorf("baseURL = %q, want trailing slash stripped", c.baseURL)
	}
}

func TestNewClient_invalidCAFile(t *testing.T) {
	_, err := NewClient("https://pve.example.com:8006", WithCAFile("/nonexistent/ca.pem"))
	if err == nil {
		t.Fatal("expected error for missing CA file")
	}
}

func TestGet_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/version" {
			t.Errorf("path = %q, want /api2/json/version", r.URL.Path)
		}
		if r.Method != "GET" {
			t.Errorf("method = %q, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{
			Data: json.RawMessage(`{"version":"8.0"}`),
		})
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := c.Get("/version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(data), "8.0") {
		t.Errorf("data = %s, want version 8.0", string(data))
	}
}

func TestGet_apiError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(Response{
			Errors: map[string]string{"auth": "invalid token"},
		})
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = c.Get("/nodes")
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	pErr, ok := err.(*ProxmoxError)
	if !ok {
		t.Fatalf("expected ProxmoxError, got %T", err)
	}
	if pErr.StatusCode != 403 {
		t.Errorf("StatusCode = %d, want 403", pErr.StatusCode)
	}
	if pErr.Errors["auth"] != "invalid token" {
		t.Errorf("Errors = %v, want auth: invalid token", pErr.Errors)
	}
}

func TestRetry_on5xx(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts <= 2 {
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte(`{"data":null}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{
			Data: json.RawMessage(`"ok"`),
		})
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, WithMaxRetries(3))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := c.Get("/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != `"ok"` {
		t.Errorf("data = %s, want \"ok\"", string(data))
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestRetry_exhausted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"data":null}`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, WithMaxRetries(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = c.Get("/test")
	if err == nil {
		t.Fatal("expected error after retries exhausted")
	}
	if !strings.Contains(err.Error(), "retries") {
		t.Errorf("error = %q, want mention of retries", err.Error())
	}
}

func TestTokenAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		expected := "PVEAPIToken=user@pve!mytoken=secret-uuid"
		if auth != expected {
			t.Errorf("Authorization = %q, want %q", auth, expected)
		}
		json.NewEncoder(w).Encode(Response{Data: json.RawMessage(`null`)})
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, WithTokenAuth("user@pve!mytoken", "secret-uuid"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c.Get("/test")
}

func TestPost_withBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		var m map[string]string
		json.NewDecoder(r.Body).Decode(&m)
		if m["node"] != "pve1" {
			t.Errorf("body node = %q, want pve1", m["node"])
		}
		json.NewEncoder(w).Encode(Response{Data: json.RawMessage(`"UPID:pve1:123"`)})
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := c.Post("/nodes", strings.NewReader(`{"node":"pve1"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(data), "UPID") {
		t.Errorf("data = %s, want UPID", string(data))
	}
}

func TestVerboseLogging(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(Response{Data: json.RawMessage(`null`)})
	}))
	defer srv.Close()

	var buf bytes.Buffer
	c, err := NewClient(srv.URL, WithVerbosity(VerbBasic), WithLogger(&buf))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	c.Get("/test")
	log := buf.String()
	if !strings.Contains(log, "> GET") {
		t.Errorf("log missing request line: %s", log)
	}
	if !strings.Contains(log, "< 200") {
		t.Errorf("log missing response status: %s", log)
	}
}

func TestVerboseDetailLogging(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(Response{Data: json.RawMessage(`null`)})
	}))
	defer srv.Close()

	var buf bytes.Buffer
	c, err := NewClient(srv.URL, WithVerbosity(VerbDetail), WithLogger(&buf))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	c.Post("/test", strings.NewReader(`{"key":"val"}`))
	log := buf.String()
	if !strings.Contains(log, "body:") {
		t.Errorf("detail log missing body: %s", log)
	}
}

func TestProxmoxError_messageFormat(t *testing.T) {
	e := &ProxmoxError{StatusCode: 400, Status: "400 Bad Request", Message: "invalid param"}
	if !strings.Contains(e.Error(), "invalid param") {
		t.Errorf("error = %q, want 'invalid param'", e.Error())
	}

	e2 := &ProxmoxError{StatusCode: 400, Status: "400 Bad Request", Errors: map[string]string{"vmid": "required"}}
	if !strings.Contains(e2.Error(), "vmid: required") {
		t.Errorf("error = %q, want 'vmid: required'", e2.Error())
	}

	e3 := &ProxmoxError{StatusCode: 500, Status: "500 Internal Server Error"}
	if !strings.Contains(e3.Error(), "500") {
		t.Errorf("error = %q, want '500'", e3.Error())
	}
}

func TestTicketAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("PVEAuthCookie")
		if err != nil || cookie.Value != "PVE:user@pam:12345678::sha256" {
			t.Errorf("expected PVEAuthCookie, got err=%v cookie=%v", err, cookie)
		}
		// GET requests should not have CSRF token.
		if r.Method == "GET" {
			if r.Header.Get("CSRFPreventionToken") != "" {
				t.Error("GET should not have CSRFPreventionToken")
			}
		}
		json.NewEncoder(w).Encode(Response{Data: json.RawMessage(`null`)})
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, WithTicketAuth("PVE:user@pam:12345678::sha256", "csrf-token-value"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c.Get("/test")
}

func TestTicketAuth_CSRF(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		csrf := r.Header.Get("CSRFPreventionToken")
		if csrf != "csrf-token-value" {
			t.Errorf("CSRFPreventionToken = %q, want %q", csrf, "csrf-token-value")
		}
		json.NewEncoder(w).Encode(Response{Data: json.RawMessage(`null`)})
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, WithTicketAuth("PVE:ticket", "csrf-token-value"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c.Post("/test", strings.NewReader(`{}`))
}

func TestAuthenticate_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/access/ticket" {
			t.Errorf("path = %q, want /api2/json/access/ticket", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", ct)
		}
		r.ParseForm()
		if r.FormValue("username") != "root@pam" {
			t.Errorf("username = %q, want root@pam", r.FormValue("username"))
		}
		json.NewEncoder(w).Encode(Response{
			Data: json.RawMessage(`{"ticket":"PVE:root@pam:12345::sha","CSRFPreventionToken":"csrf123","username":"root@pam"}`),
		})
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result, err := c.Authenticate("root@pam", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ticket != "PVE:root@pam:12345::sha" {
		t.Errorf("ticket = %q", result.Ticket)
	}
	if result.CSRFToken != "csrf123" {
		t.Errorf("csrf = %q", result.CSRFToken)
	}
	if result.Username != "root@pam" {
		t.Errorf("username = %q", result.Username)
	}
}

func TestAuthenticate_failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(Response{
			Errors: map[string]string{"username": "invalid credentials"},
		})
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = c.Authenticate("root@pam", "wrong")
	if err == nil {
		t.Fatal("expected error for bad credentials")
	}
}

func TestDecodeResponse_nonJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("<html>Bad Gateway</html>"))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, WithMaxRetries(0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = c.Get("/test")
	if err == nil {
		t.Fatal("expected error for non-JSON 502 response")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Errorf("error = %q, want mention of 502", err.Error())
	}
}
