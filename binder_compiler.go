package minato

import (
	"fmt"
	"reflect"
)

type bindSource string

const (
	sourcePath   bindSource = "path"
	sourceQuery  bindSource = "query"
	sourceHeader bindSource = "header"
	sourceCookie bindSource = "cookie"
	sourceForm   bindSource = "form"
)

// fieldPlan stores how to populate one specific struct field.
type fieldPlan struct {
	Index    int          // struct field index
	Source   bindSource   // WHere the data comes from
	Key      string       // They key to extract (e.g. "tenant_id")
	IsSlice  bool         // True if []T, False if scalar T
	BaseType reflect.Kind // The underlying scalar type (string, int, bool, etc)
}

// binderPlan is the precompiled mapping for a specific Req struct type.
type binderPlan struct {
	ReqType reflect.Type // The actual reflect.Type of Req
	Fields  []fieldPlan
}

// compileBinder analyzes a generic type T (must be a struct) and returns a plan.
// It runs exactly once at route registration, never at request time.
//
// Algorithm: single O(N) pass over struct fields, bucketing plans by source,
// then emitting the buckets in ascending precedence order so the resulting
// Fields slice is already precedence-sorted for the runtime binder.
func compileBinder[T any]() (*binderPlan, error) {
	var zero T
	t := reflect.TypeOf(zero)

	// Gate 1: Must be a struct type.
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("minato: Req type must be a struct, got %v", t.Kind())
	}

	// Fast path: struct{} - no fields to plan, binder is no-op
	if t.NumField() == 0 {
		return &binderPlan{ReqType: t}, nil
	}

	// precedence defines the ordering from lowest → highest.
	// This is also the emit order, so path (last) wins.
	precedence := []bindSource{sourceForm, sourceQuery, sourceHeader, sourceCookie, sourcePath}

	// visit each field once, tag-check all 5 sources
	// in constant inner work, and place the result in the matching bucket.
	// A struct field may declare at most one non-JSON source tag (path/query/header/cookie/form),
	// so we break out of the source loop as soon as we find a match.
	buckets := make(map[bindSource][]fieldPlan, len(precedence))

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		for _, src := range precedence {
			tag := field.Tag.Get(string(src))
			if tag == "" {
				continue
			}

			// Detect whether the field is a slice type.
			// Slice fields collect ALL repeated values from their source (e.g. ?tag=a&tag=b → ["a","b"]).
			// Scalar fields only take the first value.
			isSlice := field.Type.Kind() == reflect.Slice
			baseKind := field.Type.Kind()
			if isSlice {
				baseKind = field.Type.Elem().Kind()
			}

			buckets[src] = append(buckets[src], fieldPlan{
				Index:    i,
				Source:   src,
				Key:      tag,
				IsSlice:  isSlice,
				BaseType: baseKind,
			})
			break // A field may only carry one non-JSON source tag; stop checking remaining sources.
		}
	}
	// Emit buckets in ascending precedence order to build the final Fields slice.
	// Because sourcePath is last in precedence, path fields end up at the tail -
	// the runtime binder processes them last and they naturally win.
	plan := &binderPlan{ReqType: t}
	for _, src := range precedence {
		plan.Fields = append(plan.Fields, buckets[src]...)
	}

	return plan, nil
}
