package minato_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dwikynator/minato"
	"github.com/dwikynator/minato/middleware"
	"github.com/go-chi/chi/v5"
)

type benchmarkNoopLogger struct{}

func (benchmarkNoopLogger) Info(string, ...any)  {}
func (benchmarkNoopLogger) Error(string, ...any) {}

func BenchmarkHTTP(b *testing.B) {
	baselineStatic := buildBaselineHandler(false)
	baselineParam := buildBaselineHandler(true)
	minatoBareStatic := buildMinatoHandler(false, false)
	minatoBareParam := buildMinatoHandler(true, false)
	minatoStackedStatic := buildMinatoHandler(false, true)
	minatoStackedParam := buildMinatoHandler(true, true)

	b.Run("Baseline/chi/static", func(b *testing.B) {
		runHTTPBenchmark(b, baselineStatic, "/ping", false)
	})

	b.Run("Minato/bare/static", func(b *testing.B) {
		runHTTPBenchmark(b, minatoBareStatic, "/ping", false)
	})

	b.Run("Minato/stacked/static", func(b *testing.B) {
		b.Run("request-id-missing", func(b *testing.B) {
			runHTTPBenchmark(b, minatoStackedStatic, "/ping", false)
		})
		b.Run("request-id-present", func(b *testing.B) {
			runHTTPBenchmark(b, minatoStackedStatic, "/ping", true)
		})
	})

	b.Run("Baseline/chi/param", func(b *testing.B) {
		runHTTPBenchmark(b, baselineParam, "/users/123", false)
	})

	b.Run("Minato/bare/param", func(b *testing.B) {
		runHTTPBenchmark(b, minatoBareParam, "/users/123", false)
	})

	b.Run("Minato/stacked/param", func(b *testing.B) {
		b.Run("request-id-missing", func(b *testing.B) {
			runHTTPBenchmark(b, minatoStackedParam, "/users/123", false)
		})
		b.Run("request-id-present", func(b *testing.B) {
			runHTTPBenchmark(b, minatoStackedParam, "/users/123", true)
		})
	})
}

func runHTTPBenchmark(b *testing.B, h http.Handler, path string, withRequestID bool) {
	b.Helper()

	// Sanity check before timing.
	checkReq := httptest.NewRequest(http.MethodGet, path, nil)
	if withRequestID {
		checkReq.Header.Set("X-Request-ID", "bench-request-id")
	}
	checkRec := httptest.NewRecorder()
	h.ServeHTTP(checkRec, checkReq)
	if checkRec.Code != http.StatusOK {
		b.Fatalf("unexpected status: got=%d want=%d", checkRec.Code, http.StatusOK)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if withRequestID {
			req.Header.Set("X-Request-ID", "bench-request-id")
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
	}
}

func buildBaselineHandler(withParamRoute bool) http.Handler {
	r := chi.NewMux()
	if withParamRoute {
		r.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "id")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"` + id + `","status":"ok"}`))
		})
		return r
	}

	r.Get("/ping", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	return r
}

func buildMinatoHandler(withParamRoute bool, withStack bool) http.Handler {
	s := minato.New()
	if withStack {
		noop := benchmarkNoopLogger{}
		s.Use(middleware.RequestID())
		s.Use(middleware.Recovery(middleware.WithRecoveryPrinter(noop)))
		s.Use(middleware.CORS())
		s.Use(middleware.Logger(middleware.WithLogPrinter(noop)))
	}

	if withParamRoute {
		s.Router().Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "id")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"` + id + `","status":"ok"}`))
		})
		return s.Router()
	}

	s.Router().Get("/ping", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	return s.Router()
}
