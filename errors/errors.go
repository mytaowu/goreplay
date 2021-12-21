package errors

import "errors"

// ErrorStopped is the error returned when the go routines reading the input is stopped.
var ErrorStopped = errors.New("reading stopped")

// ErrorFilterFromIP is the error returned when filter packets based on IP
var ErrorFilterFromIP = errors.New("filter packets based on ip")
