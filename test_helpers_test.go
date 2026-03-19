package minato_test

// validatorFunc lets you satisfy minato.Validator with an anonymous function.
type validatorFunc func(v any) error

func (f validatorFunc) Validate(v any) error { return f(v) }
