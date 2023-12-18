package errors

import "errors"

var (
	// ErrResourceNotFound is returned when a resource is not found in the collection.
	ErrResourceNotFound = errors.New("resource not found")
	// ErrInvalidReference is returned when a reference is not valid.
	ErrInvalidReference = errors.New("invalid reference")
	// ErrCyclicDependency is returned when a cyclic dependency is found.
	ErrCyclicReference = errors.New("cyclic reference")
	// ErrSelfReference is returned when a resource references itself.
	ErrSelfReference = errors.New("self reference")
)
