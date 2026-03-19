package minato

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// BindError is returned when coercion fails
type BindError struct {
	Field  string
	Source string
	Err    error
}

func (e *BindError) Error() string {
	return e.Source + " param error: " + e.Err.Error()
}

// bindConfig holds per-request-but-static settings derived from RouteOptions.
type bindConfig struct {
	MaxBodyBytes int64 // 0 = unlimited
	StrictJSON   bool  // disallow unknown JSON fields
}

// bindRequest creates a new instance of T and populates it from the HTTP request.
func bindRequest[T any](r *http.Request, plan *binderPlan, cfg bindConfig) (T, error) {
	var req T

	// Fast path for struct{} - nothing to bind
	if len(plan.Fields) == 0 && plan.ReqType.NumField() == 0 {
		return req, nil
	}

	reqValue := reflect.ValueOf(&req).Elem()

	// Step 1 (lowest precedence): Decode JSON body.
	if r.Body != nil {
		body := r.Body
		if cfg.MaxBodyBytes > 0 {
			body = http.MaxBytesReader(nil, body, cfg.MaxBodyBytes)
		}
		dec := json.NewDecoder(body)
		if cfg.StrictJSON {
			// Reject unknown JSON keys not present in the struct.
			dec.DisallowUnknownFields()
		}
		err := dec.Decode(&req)
		if err != nil && !errors.Is(err, io.EOF) {
			return req, &BindError{Source: "json", Err: err}
		}
	}

	// Step 2-6: Apply non-JSON sources from plan.Fields, which is already sorted
	// in ascending order by compileBinder (form -> query -> header -> cookie -> path).
	// Each consecutive write can safely overwrite a previous lower-precedence value.
	_ = r.ParseForm() // required before accessing r.Form

	for _, fp := range plan.Fields {
		var values []string

		switch fp.Source {
		case sourceForm:
			values = r.Form[fp.Key]
		case sourceQuery:
			values = r.URL.Query()[fp.Key]
		case sourceHeader:
			values = r.Header.Values(fp.Key) // Returns all repeated header values
		case sourceCookie:
			if c, err := r.Cookie(fp.Key); err == nil {
				values = []string{c.Value} // Can expand to all matching cookies later
			}
		case sourcePath:
			// chi.URLParam returns the matched route parameter (highest precedence)
			if val := chi.URLParam(r, fp.Key); val != "" {
				values = []string{val}
			}

		}

		if len(values) == 0 {
			continue // no data provided
		}

		// Coerce and set values via reflect
		fieldVal := reqValue.Field(fp.Index)

		if fp.IsSlice {
			// []T: collect all repeated values (e.g. ?tag=a&tag=b -> []string{"a", "b"})
			slice := reflect.MakeSlice(fieldVal.Type(), len(values), len(values))
			for i, v := range values {
				if err := coerceScalar(v, fp.BaseType, slice.Index(i)); err != nil {
					return req, &BindError{Field: fp.Key, Source: string(fp.Source), Err: err}
				}
			}
			fieldVal.Set(slice)
		} else {
			// Scalar: use only the first value
			if err := coerceScalar(values[0], fp.BaseType, fieldVal); err != nil {
				return req, &BindError{Field: fp.Key, Source: string(fp.Source), Err: err}
			}
		}
	}
	return req, nil
}

// coerceScalar converts raw string s into the target reflect.Kind and writes it into val.
// Supported: string, bool, int*, uint*, float* - matches the Phase 2 coercion table.
func coerceScalar(s string, kind reflect.Kind, val reflect.Value) error {
	switch kind {
	case reflect.String:
		val.SetString(s)
	case reflect.Bool:
		b, err := strconv.ParseBool(s) // accepts "true", "1", "false", "0"
		if err != nil {
			return err
		}
		val.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		val.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		// Parse Uint for unsigned integer types (e.g. IDs, port numbers).
		u, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return err
		}
		val.SetUint(u)
	case reflect.Float32, reflect.Float64:
		// val.Type().Bits() returns 32 or 64, matching the field's actual precision.
		f, err := strconv.ParseFloat(s, val.Type().Bits())
		if err != nil {
			return err
		}
		val.SetFloat(f)
	default:
		return errors.New("unsupported type for non-JSON binding; use a string/bool/int/uint/float or a slice thereof")
	}
	return nil
}
