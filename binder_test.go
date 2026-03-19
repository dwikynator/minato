package minato

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// helper: failed the test if err is not nil.
func mustNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// helper: fail the test if err IS nil.
func mustError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

// helper: fail the test if got != want.
func assertEqual[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// ─── compileBinder tests ──────────────────────────────────────────────────────

// TestCompileBinder_Validations verifies that only struct types are accepted.
func TestCompileBinder_Validations(t *testing.T) {
	// struct{} must compile without error (zero-field fast path).
	_, err := compileBinder[struct{}]()
	mustNoError(t, err)

	// A normal struct must compile without error.
	type ValidReq struct {
		ID   string   `path:"id"`
		Tags []string `query:"tag"`
	}
	plan, err := compileBinder[ValidReq]()
	mustNoError(t, err)
	if len(plan.Fields) != 2 {
		t.Errorf("expected 2 fields in plan, got %d", len(plan.Fields))
	}

	// Non-struct types must be rejected.
	_, err = compileBinder[int]()
	mustError(t, err)

	_, err = compileBinder[map[string]any]()
	mustError(t, err)
}

// TestCompileBinder_PrecedenceOrder verifies that plan.Fields is emitted in
// ascending precedence order: form → query → header → cookie → path.
// This ordering is what makes the runtime binder implement "path wins" correctly.
func TestCompileBinder_PrecedenceOrder(t *testing.T) {
	// Fields are declared in reverse precedence to ensure the compiler sorts them.
	type Req struct {
		ID      string `path:"id"`
		Token   string `header:"Authorization"`
		Q       string `query:"q"`
		Session string `cookie:"session"`
		Field   string `form:"field"`
	}

	plan, err := compileBinder[Req]()
	mustNoError(t, err)

	want := []bindSource{sourceForm, sourceQuery, sourceHeader, sourceCookie, sourcePath}
	if len(plan.Fields) != len(want) {
		t.Fatalf("expected %d fields, got %d", len(want), len(plan.Fields))
	}
	for i, f := range plan.Fields {
		if f.Source != want[i] {
			t.Errorf("Fields[%d]: got source %q, want %q", i, f.Source, want[i])
		}
	}
}

// ─── bindRequest tests ────────────────────────────────────────────────────────

// helper: inject chi path params into a request context.
func withChiParams(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// TestBindRequest_AllSources exercises every supported source tag and scalar type.
func TestBindRequest_AllSources(t *testing.T) {
	type ComplexReq struct {
		TenantID string   `path:"tenant_id"`
		Limit    int      `query:"limit"`
		Active   bool     `query:"active"`
		Score    float64  `query:"score"`
		Count    uint     `query:"count"`
		Auth     string   `header:"Authorization"`
		Session  string   `cookie:"session"`
		Tags     []string `query:"tag"`
		Payload  struct {
			Name string `json:"name"`
		} `json:"payload"`
	}

	plan, _ := compileBinder[ComplexReq]()

	body := strings.NewReader(`{"payload": {"name": "Alice"}}`)
	req, _ := http.NewRequest(http.MethodPost, "/", body)
	req = withChiParams(req, map[string]string{"tenant_id": "t-123"})

	q := req.URL.Query()
	q.Set("limit", "50")
	q.Set("active", "true")
	q.Set("score", "9.5")
	q.Set("count", "7")
	q.Add("tag", "go")
	q.Add("tag", "http")
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Authorization", "Bearer token123")
	req.AddCookie(&http.Cookie{Name: "session", Value: "sess_abc"})

	result, err := bindRequest[ComplexReq](req, plan, bindConfig{})
	mustNoError(t, err)

	assertEqual(t, result.TenantID, "t-123")
	assertEqual(t, result.Limit, 50)
	assertEqual(t, result.Active, true)
	assertEqual(t, result.Score, 9.5)
	assertEqual(t, result.Count, uint(7))
	assertEqual(t, result.Auth, "Bearer token123")
	assertEqual(t, result.Session, "sess_abc")
	assertEqual(t, result.Payload.Name, "Alice")

	// Repeated query params must produce a slice.
	if len(result.Tags) != 2 || result.Tags[0] != "go" || result.Tags[1] != "http" {
		t.Errorf("Tags: got %v, want [go http]", result.Tags)
	}
}

// TestBindRequest_PathOverridesJSON is the critical security test.
// A JSON body value for the same semantic field must be overwritten by the path param.
func TestBindRequest_PathOverridesJSON(t *testing.T) {
	type Req struct {
		TenantID string `path:"tenant_id" json:"tenant_id"`
	}

	plan, _ := compileBinder[Req]()

	body := strings.NewReader(`{"tenant_id": "attacker"}`)
	req, _ := http.NewRequest(http.MethodPost, "/", body)
	req = withChiParams(req, map[string]string{"tenant_id": "t-legit"})

	result, err := bindRequest[Req](req, plan, bindConfig{})
	mustNoError(t, err)

	// Path must win; the attacker's body value must be discarded.
	assertEqual(t, result.TenantID, "t-legit")
}

// TestBindRequest_CoercionError verifies a BindError is returned when type coercion fails.
func TestBindRequest_CoercionError(t *testing.T) {
	type Req struct {
		ID int `path:"id"`
	}
	plan, _ := compileBinder[Req]()

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req = withChiParams(req, map[string]string{"id": "not-a-number"})

	_, err := bindRequest[Req](req, plan, bindConfig{})
	mustError(t, err)

	bindErr, ok := err.(*BindError)
	if !ok {
		t.Fatalf("expected *BindError, got %T", err)
	}
	assertEqual(t, bindErr.Field, "id")
	assertEqual(t, bindErr.Source, "path")
}

// TestBindRequest_StrictJSON verifies unknown JSON fields are rejected.
func TestBindRequest_StrictJSON(t *testing.T) {
	type Req struct {
		Name string `json:"name"`
	}
	plan, _ := compileBinder[Req]()

	body := strings.NewReader(`{"name": "Alice", "unknown": "bad"}`)
	req, _ := http.NewRequest(http.MethodPost, "/", body)

	_, err := bindRequest[Req](req, plan, bindConfig{StrictJSON: true})
	mustError(t, err)
}

// TestBindRequest_MaxBodyBytes verifies oversized payloads are rejected.
func TestBindRequest_MaxBodyBytes(t *testing.T) {
	type Req struct {
		Name string `json:"name"`
	}
	plan, _ := compileBinder[Req]()

	body := strings.NewReader(`{"name": "Alice"}`)
	req, _ := http.NewRequest(http.MethodPost, "/", body)

	_, err := bindRequest[Req](req, plan, bindConfig{MaxBodyBytes: 3}) // 3-byte cap
	mustError(t, err)
}

// TestBindRequest_EmptyBody_IsNotAnError verifies io.EOF from an empty body is silently ignored.
func TestBindRequest_EmptyBody_IsNotAnError(t *testing.T) {
	type Req struct {
		ID string `path:"id"`
	}
	plan, _ := compileBinder[Req]()

	req, _ := http.NewRequest(http.MethodGet, "/", nil) // nil body → no JSON
	req = withChiParams(req, map[string]string{"id": "abc"})

	result, err := bindRequest[Req](req, plan, bindConfig{})
	mustNoError(t, err)
	assertEqual(t, result.ID, "abc")
}
