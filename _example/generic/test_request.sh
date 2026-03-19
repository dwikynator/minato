#!/bin/bash
# test_request.sh — smoke-test the _example/generic server
# Run the server first: go run ./_example/generic/
# Then in another terminal: bash _example/generic/test_request.sh

BASE="http://localhost:3000"

sep() { echo; echo "────────────────────────────────"; echo "$1"; echo "────────────────────────────────"; }

# 1. Health check (struct{} no-input handler)
sep "GET /healthz — struct{} handler"
curl -s "$BASE/healthz" | jq .

# 2. Create order — all sources in one request
#    path: tenant_id=t-acme
#    query: source=web, dry_run=true
#    header: Authorization
#    cookie: session_id
#    json body: payload.sku + payload.quantity
sep "POST /tenants/t-acme/orders — all sources"
curl -s -X POST "$BASE/tenants/t-acme/orders?source=web&dry_run=true" \
  -H "Authorization: Bearer my-token" \
  -H "Content-Type: application/json" \
  -b "session_id=sess-xyz" \
  -d '{"payload": {"sku": "macbook-pro", "quantity": 2}}' \
  -i

# 3. Validation error — missing sku (WithValidator rejects)
sep "POST /tenants/t-acme/orders — missing sku → 400 validation error"
curl -s -X POST "$BASE/tenants/t-acme/orders" \
  -H "Authorization: Bearer my-token" \
  -H "Content-Type: application/json" \
  -d '{"payload": {"sku": "", "quantity": 1}}' | jq .

# 4. Strict JSON — unknown field rejected (WithStrictJSON)
sep "POST /tenants/t-acme/orders — unknown field → 400"
curl -s -X POST "$BASE/tenants/t-acme/orders" \
  -H "Authorization: Bearer my-token" \
  -H "Content-Type: application/json" \
  -d '{"payload": {"sku": "item-1", "quantity": 1}, "evil": "injected"}' | jq .

# 5. Get order — custom error mapper turns ErrOrderNotFound → 404
sep "GET /tenants/t-acme/orders/missing — custom mapper → 404"
curl -s "$BASE/tenants/t-acme/orders/missing" | jq .

# 6. Get order — success
sep "GET /tenants/t-acme/orders/ord-abc-123 — success → 200"
curl -s "$BASE/tenants/t-acme/orders/ord-abc-123" | jq .

# 7. Delete order — 204 No Content
sep "DELETE /tenants/t-acme/orders/ord-abc-123 — 204 No Content"
curl -s -X DELETE "$BASE/tenants/t-acme/orders/ord-abc-123" -i

echo
echo "All requests completed."
