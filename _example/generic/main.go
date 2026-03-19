// Package main demonstrates the Minato Generic Handler feature.
//
// Run:   go run ./example/generic/
// Test:  bash _example/generic/test_request.sh
package main

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/dwikynator/minato"
)

// ─── Domain errors ──────────────────────────────────────────────────────────

var ErrOrderNotFound = errors.New("order not found")

// ─── Custom error mapper ─────────────────────────────────────────────────────

// orderErrorMapper maps domain errors to HTTP responses.
// Swap this in per-route via minato.WithErrorMapper(orderErrorMapper).
func orderErrorMapper(ctx context.Context, err error) minato.ErrorResponse {
	if errors.Is(err, ErrOrderNotFound) {
		return minato.ErrorResponse{
			Status: http.StatusNotFound,
			Body:   map[string]string{"error": "order not found"},
		}
	}
	// Fallback — never leak internal details.
	return minato.ErrorResponse{
		Status: http.StatusInternalServerError,
		Body:   map[string]string{"error": "internal server error"},
	}
}

// ─── Simple pluggable validator (no third-party library needed) ───────────────

type basicValidator struct{}

func (basicValidator) Validate(v any) error {
	// In a real app, use go-playground/validator or ozzo-validation here.
	// The Minato adapter wraps the returned error in *minato.ValidationError
	// so defaultErrorMapper can detect it without knowing which library is used.
	if req, ok := v.(CreateOrderRequest); ok {
		if req.Payload.SKU == "" {
			return errors.New("sku is required")
		}
	}
	return nil
}

// ─── Request / Response types ─────────────────────────────────────────────────

// CreateOrderRequest binds from four HTTP sources simultaneously:
//   - path:   /tenants/{tenant_id}
//   - query:  ?source=web&dry_run=true
//   - header: Authorization
//   - cookie: session_id
//   - json:   body {"payload": {...}}
type CreateOrderRequest struct {
	TenantID  string `path:"tenant_id"`
	Source    string `query:"source"`
	DryRun    bool   `query:"dry_run"`
	Token     string `header:"Authorization"`
	SessionID string `cookie:"session_id"`
	Payload   struct {
		SKU      string `json:"sku"`
		Quantity int    `json:"quantity"`
	} `json:"payload"`
}

type OrderResponse struct {
	OrderID  string `json:"order_id"`
	TenantID string `json:"tenant_id"`
	DryRun   bool   `json:"dry_run"`
}

// GetOrderRequest binds from path only.
type GetOrderRequest struct {
	TenantID string `path:"tenant_id"`
	OrderID  string `path:"order_id"`
}

// ─── Handler functions (pure Go — no http.Request/ResponseWriter) ─────────────

func CreateOrderHandler(ctx context.Context, req CreateOrderRequest) (minato.Response[OrderResponse], error) {
	if req.Token == "" {
		return minato.Response[OrderResponse]{}, errors.New("missing authorization")
	}

	res := OrderResponse{
		OrderID:  "ord-abc-123",
		TenantID: req.TenantID,
		DryRun:   req.DryRun,
	}

	return minato.Created(res).
		SetHeader("Location", "/tenants/"+req.TenantID+"/orders/ord-abc-123").
		AddHeader("Set-Cookie", "last_order=ord-abc-123; Path=/; HttpOnly"), nil
}

func GetOrderHandler(ctx context.Context, req GetOrderRequest) (minato.Response[OrderResponse], error) {
	if req.OrderID == "missing" {
		return minato.Response[OrderResponse]{}, ErrOrderNotFound
	}

	return minato.OK(OrderResponse{
		OrderID:  req.OrderID,
		TenantID: req.TenantID,
	}), nil
}

func DeleteOrderHandler(ctx context.Context, req GetOrderRequest) (minato.Response[struct{}], error) {
	if req.OrderID == "missing" {
		return minato.Response[struct{}]{}, ErrOrderNotFound
	}
	// 204 No Content: no body, no Content-Type header.
	return minato.NoContent(), nil
}

// ─── Health handler (no input — struct{} Req) ─────────────────────────────────

type HealthResponse struct {
	Status string `json:"status"`
}

func HealthHandler(ctx context.Context, _ struct{}) (minato.Response[HealthResponse], error) {
	return minato.OK(HealthResponse{Status: "ok"}), nil
}

// ─── main ─────────────────────────────────────────────────────────────────────

func main() {
	server := minato.New(minato.WithAddr(":3000"))
	router := server.Router()

	// GET /healthz — no input handler (struct{})
	minato.Get(router, "/healthz", HealthHandler)

	// POST /tenants/{tenant_id}/orders
	// Demonstrates:
	//   - Binding from path, query, header, cookie, JSON body simultaneously.
	//   - WithValidator: validates Payload.SKU is non-empty.
	//   - WithStrictJSON: rejects unknown JSON keys.
	//   - WithMaxBodyBytes: rejects payloads > 64 KiB.
	//   - WithErrorMapper: maps domain errors to structured HTTP responses.
	minato.Post(
		router,
		"/tenants/{tenant_id}/orders",
		CreateOrderHandler,
		minato.WithValidator(basicValidator{}),
		minato.WithStrictJSON(true),
		minato.WithMaxBodyBytes(64*1024),
		minato.WithErrorMapper(orderErrorMapper),
	)

	// GET /tenants/{tenant_id}/orders/{order_id}
	// Demonstrates: custom error mapper turning ErrOrderNotFound → 404.
	minato.Get(
		router,
		"/tenants/{tenant_id}/orders/{order_id}",
		GetOrderHandler,
		minato.WithErrorMapper(orderErrorMapper),
	)

	// DELETE /tenants/{tenant_id}/orders/{order_id}
	// Demonstrates: 204 No Content response (no body written).
	minato.Delete(
		router,
		"/tenants/{tenant_id}/orders/{order_id}",
		DeleteOrderHandler,
		minato.WithErrorMapper(orderErrorMapper),
	)

	log.Println("Listening on :3000 — run test_request.sh to try it out")
	if err := server.Run(); err != nil {
		log.Fatal(err)
	}
}
