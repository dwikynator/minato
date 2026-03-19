package minato_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dwikynator/minato"
)

// helper: decode JSON body into a map.
func decodeBody(t *testing.T, resp *http.Response) map[string]string {
	t.Helper()
	var m map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	return m
}

// helper: check HTTP status.
func assertStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Errorf("status: got %d, want %d", resp.StatusCode, want)
	}
}

// ─── helpers shared across test cases ────────────────────────────────────────

func newTestServer(t *testing.T) (*minato.Server, *httptest.Server) {
	t.Helper()
	srv := minato.New()
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)
	return srv, ts
}

// ─── tests ────────────────────────────────────────────────────────────────────

// TestAdapter_HappyPath_201_WithHeaders — success path returning 201 + custom headers.
func TestAdapter_HappyPath_201_WithHeaders(t *testing.T) {
	srv := minato.New()

	type Req struct {
		ID int `path:"id"`
	}
	type Res struct {
		Message string `json:"message"`
	}

	minato.Post(srv.Router(), "/items/{id}", func(_ context.Context, req Req) (minato.Response[Res], error) {
		return minato.Created(Res{Message: "ok"}).
			SetHeader("X-Item-ID", "42").
			AddHeader("Set-Cookie", "token=abc; HttpOnly"), nil
	})

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/items/42", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertStatus(t, resp, http.StatusCreated)
	if ct := resp.Header.Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type: got %q, want application/json; charset=utf-8", ct)
	}
	if v := resp.Header.Get("X-Item-ID"); v != "42" {
		t.Errorf("X-Item-ID: got %q, want 42", v)
	}
	if v := resp.Header.Get("Set-Cookie"); !strings.Contains(v, "token=abc") {
		t.Errorf("Set-Cookie: got %q, want to contain token=abc", v)
	}
}

// TestAdapter_NoContent_204 — 204 must have no body and no Content-Type.
func TestAdapter_NoContent_204(t *testing.T) {
	srv := minato.New()

	minato.Delete(srv.Router(), "/items/{id}", func(_ context.Context, _ struct{}) (minato.Response[struct{}], error) {
		return minato.NoContent(), nil
	})

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/items/1", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertStatus(t, resp, http.StatusNoContent)
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		t.Errorf("Content-Type must be empty for 204, got %q", ct)
	}
}

// TestAdapter_BindError_Returns400 — coercion failure must produce 400 with field details.
func TestAdapter_BindError_Returns400(t *testing.T) {
	srv := minato.New()

	type Req struct {
		Page int `query:"page"`
	}
	minato.Get(srv.Router(), "/items", func(_ context.Context, req Req) (minato.Response[struct{}], error) {
		return minato.OK(struct{}{}), nil
	})

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/items?page=banana")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertStatus(t, resp, http.StatusBadRequest)
	body := decodeBody(t, resp)
	if body["error"] != "invalid parameter" {
		t.Errorf("error: got %q, want \"invalid parameter\"", body["error"])
	}
	if body["field"] != "page" {
		t.Errorf("field: got %q, want \"page\"", body["field"])
	}
}

// TestAdapter_HandlerError_Returns500_NoLeak — unhandled handler errors must return
// a safe 500 body and must NOT expose the raw error string.
func TestAdapter_HandlerError_Returns500_NoLeak(t *testing.T) {
	srv := minato.New()

	minato.Get(srv.Router(), "/boom", func(_ context.Context, _ struct{}) (minato.Response[struct{}], error) {
		return minato.Response[struct{}]{}, errors.New("db: postgres://secret@host/proddb")
	})

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/boom")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertStatus(t, resp, http.StatusInternalServerError)
	body := decodeBody(t, resp)
	if body["error"] != "internal server error" {
		t.Errorf("error: got %q, want \"internal server error\"", body["error"])
	}
	if strings.Contains(body["error"], "postgres") {
		t.Error("500 response must not leak internal error details")
	}
}

// TestAdapter_WithErrorMapper — per-route mapper translates domain errors to specific codes.
func TestAdapter_WithErrorMapper(t *testing.T) {
	srv := minato.New()

	errNotFound := errors.New("not found")

	mapper := minato.ErrorMapper(func(_ context.Context, err error) minato.ErrorResponse {
		if errors.Is(err, errNotFound) {
			return minato.ErrorResponse{
				Status: http.StatusNotFound,
				Body:   map[string]string{"error": "resource not found"},
			}
		}
		return minato.ErrorResponse{
			Status: http.StatusInternalServerError,
			Body:   map[string]string{"error": "internal server error"},
		}
	})

	minato.Get(srv.Router(), "/items/{id}", func(_ context.Context, _ struct{}) (minato.Response[struct{}], error) {
		return minato.Response[struct{}]{}, errNotFound
	}, minato.WithErrorMapper(mapper))

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/items/1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertStatus(t, resp, http.StatusNotFound)
	body := decodeBody(t, resp)
	if body["error"] != "resource not found" {
		t.Errorf("error: got %q, want \"resource not found\"", body["error"])
	}
}

// TestAdapter_WithValidator — validator failure must produce 400 (via ValidationError wrap).
// The adapter wraps the raw validator error in *minato.ValidationError before passing
// to the error mapper, so defaultErrorMapper returns 400 rather than 500.
func TestAdapter_WithValidator(t *testing.T) {
	srv := minato.New()

	// Implement the Validator interface inline via a local type.
	type validatorImpl struct{ msg string }

	// Use an anonymous func adapter to satisfy the interface.
	var v minato.Validator = validatorFunc(func(val any) error {
		return errors.New("name is required")
	})

	type Req struct {
		Name string `json:"name"`
	}
	minato.Post(srv.Router(), "/items", func(_ context.Context, req Req) (minato.Response[struct{}], error) {
		return minato.OK(struct{}{}), nil
	}, minato.WithValidator(v))

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/items", "application/json", strings.NewReader(`{"name":""}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertStatus(t, resp, http.StatusBadRequest)
	body := decodeBody(t, resp)
	if body["error"] != "name is required" {
		t.Errorf("error: got %q, want \"name is required\"", body["error"])
	}
}

// TestAdapter_WithStrictJSON — unknown JSON keys must be rejected with 400.
func TestAdapter_WithStrictJSON(t *testing.T) {
	srv := minato.New()

	type Req struct {
		Name string `json:"name"`
	}
	minato.Post(srv.Router(), "/items", func(_ context.Context, req Req) (minato.Response[struct{}], error) {
		return minato.OK(struct{}{}), nil
	}, minato.WithStrictJSON(true))

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	body := strings.NewReader(`{"name": "Alice", "evil": "injected"}`)
	resp, err := http.Post(ts.URL+"/items", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertStatus(t, resp, http.StatusBadRequest)
}

// TestAdapter_WithMaxBodyBytes — payloads exceeding the cap must be rejected with 400.
func TestAdapter_WithMaxBodyBytes(t *testing.T) {
	srv := minato.New()

	type Req struct {
		Name string `json:"name"`
	}
	minato.Post(srv.Router(), "/items", func(_ context.Context, req Req) (minato.Response[struct{}], error) {
		return minato.OK(struct{}{}), nil
	}, minato.WithMaxBodyBytes(10)) // 10-byte cap

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	bigBody := strings.NewReader(`{"name": "this body is definitely more than 10 bytes"}`)
	resp, err := http.Post(ts.URL+"/items", "application/json", bigBody)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertStatus(t, resp, http.StatusBadRequest)
}
