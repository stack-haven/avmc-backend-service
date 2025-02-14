// Code generated by protoc-gen-validate. DO NOT EDIT.
// source: common/conf/tracer.proto

package conf

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/types/known/anypb"
)

// ensure the imports are used
var (
	_ = bytes.MinRead
	_ = errors.New("")
	_ = fmt.Print
	_ = utf8.UTFMax
	_ = (*regexp.Regexp)(nil)
	_ = (*strings.Reader)(nil)
	_ = net.IPv4len
	_ = time.Duration(0)
	_ = (*url.URL)(nil)
	_ = (*mail.Address)(nil)
	_ = anypb.Any{}
	_ = sort.Sort
)

// Validate checks the field values on Tracer with the rules defined in the
// proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *Tracer) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on Tracer with the rules defined in the
// proto definition for this message. If any rules are violated, the result is
// a list of violation errors wrapped in TracerMultiError, or nil if none found.
func (m *Tracer) ValidateAll() error {
	return m.validate(true)
}

func (m *Tracer) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for Batcher

	// no validation rules for Endpoint

	// no validation rules for Sampler

	// no validation rules for Env

	// no validation rules for Insecure

	if len(errors) > 0 {
		return TracerMultiError(errors)
	}

	return nil
}

// TracerMultiError is an error wrapping multiple validation errors returned by
// Tracer.ValidateAll() if the designated constraints aren't met.
type TracerMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m TracerMultiError) Error() string {
	msgs := make([]string, 0, len(m))
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m TracerMultiError) AllErrors() []error { return m }

// TracerValidationError is the validation error returned by Tracer.Validate if
// the designated constraints aren't met.
type TracerValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e TracerValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e TracerValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e TracerValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e TracerValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e TracerValidationError) ErrorName() string { return "TracerValidationError" }

// Error satisfies the builtin error interface
func (e TracerValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sTracer.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = TracerValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = TracerValidationError{}
