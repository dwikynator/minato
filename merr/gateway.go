package merr

import (
	"context"
	"encoding/json"
	"net/http"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/status"

	runtime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
)

// gatewayErrorBody is the JSON structure written by GatewayErrorHandler.
type gatewayErrorBody struct {
	Error gatewayErrorDetail `json:"error"`
}

type gatewayErrorDetail struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// GatewayErrorHandler is a drop-in replacement for grpc-gateway's default
// error handler. It extracts ErrorInfo from the gRPC status details and
// writes a unified JSON error response that matches Minato's HTTP error shape.
//
// Usage in main.go:
//
//	server := minato.New(
//	    minato.WithGatewayMuxOptions(
//	        runtime.WithErrorHandler(merr.GatewayErrorHandler),
//	    ),
//	)
func GatewayErrorHandler(
	ctx context.Context,
	mux *runtime.ServeMux,
	marshaler runtime.Marshaler,
	w http.ResponseWriter,
	r *http.Request,
	err error,
) {
	st := status.Convert(err)
	httpStatus := runtime.HTTPStatusFromCode(st.Code())

	// Default values — used when no ErrorInfo detail is attached
	errCode := st.Code().String()
	errMessage := st.Message()
	var details map[string]string

	// Extract ErrorInfo from the status details if present
	for _, d := range st.Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok {
			if info.Reason != "" {
				errCode = info.Reason
			}
			if info.Domain != "" || len(info.Metadata) > 0 {
				details = make(map[string]string)
				if info.Domain != "" {
					details["domain"] = info.Domain
				}
				for k, v := range info.Metadata {
					details[k] = v
				}
			}
			break // only use the first ErrorInfo
		}
	}

	body := gatewayErrorBody{
		Error: gatewayErrorDetail{
			Code:    errCode,
			Message: errMessage,
			Details: details,
		},
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(body)
}

// WithGatewayErrorHandler is a runtime.ServeMuxOption that sets the error handler
// to GatewayErrorHandler.
//
// Usage in main.go:
//
//	server := minato.New(
//	    minato.WithGatewayMuxOptions(
//	        merr.WithGatewayErrorHandler(),
//	    ),
//	)
func WithGatewayErrorHandler() runtime.ServeMuxOption {
	return runtime.WithErrorHandler(GatewayErrorHandler)
}
