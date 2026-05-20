package errx

import "fmt"
import "example.com/aim/shared/errno"

func Required(name string) error {
	return errno.New(errno.ErrBadRequest, fmt.Sprintf("bad_request: %s is required", name))
}

func MustPositiveInteger(name string) error {
	return errno.New(errno.ErrBadRequest, fmt.Sprintf("bad_request: %s must be a positive integer", name))
}

func MustNonNegativeInteger(name string) error {
	return errno.New(errno.ErrBadRequest, fmt.Sprintf("bad_request: %s must be a non-negative integer", name))
}

func MustBeBool(name string) error {
	return errno.New(errno.ErrBadRequest, fmt.Sprintf("bad_request: %s must be true or false", name))
}

func NilDependency(name string) error {
	return errno.New(errno.ErrInternalError, fmt.Sprintf("internal: %s is nil", name))
}

func EmptyInput(name string) error {
	return errno.New(errno.ErrBadRequest, fmt.Sprintf("bad_request: %s is empty", name))
}
