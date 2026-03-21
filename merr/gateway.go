package merr

import (
	"context"
	"encoding/json"
	"net/http"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/status"

	runtime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
)

type gatewayErrorBody struct {
	Error gatewayErrorDetail `json:"error"`
}

type gatewayErrorDetail struct {
	Code            string                  `json:"code"`
	Message         string                  `json:"message"`
	Details         map[string]string       `json:"details,omitempty"`
	FieldViolations []gatewayFieldViolation `json:"field_violations,omitempty"`
}

type gatewayFieldViolation struct {
	Field       string `json:"field"`
	Description string `json:"description"`
}

// GatewayErrorHandler is a drop-in replacement for grpc-gateway's default
// error handler. It extracts ErrorInfo from the gRPC status details and
// writes a unified JSON error response that matches Minato's HTTP error shape.
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

	errCode := st.Code().String()
	errMessage := st.Message()
	var details map[string]string
	var fieldViolations []gatewayFieldViolation

	for _, d := range st.Details() {
		switch info := d.(type) {
		case *errdetails.ErrorInfo:
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

		case *errdetails.BadRequest:
			for _, fv := range info.FieldViolations {
				fieldViolations = append(fieldViolations, gatewayFieldViolation{
					Field:       fv.Field,
					Description: fv.Description,
				})
			}
		}
	}

	body := gatewayErrorBody{
		Error: gatewayErrorDetail{
			Code:            errCode,
			Message:         errMessage,
			Details:         details,
			FieldViolations: fieldViolations,
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
