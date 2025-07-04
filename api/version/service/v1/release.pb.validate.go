// Code generated by protoc-gen-validate. DO NOT EDIT.
// source: version/service/v1/release.proto

package v1

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

// Validate checks the field values on Release with the rules defined in the
// proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *Release) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on Release with the rules defined in the
// proto definition for this message. If any rules are violated, the result is
// a list of violation errors wrapped in ReleaseMultiError, or nil if none found.
func (m *Release) ValidateAll() error {
	return m.validate(true)
}

func (m *Release) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for Id

	if m.CreatedAt != nil {
		// no validation rules for CreatedAt
	}

	if m.UpdatedAt != nil {
		// no validation rules for UpdatedAt
	}

	if m.State != nil {
		// no validation rules for State
	}

	if m.Remark != nil {
		// no validation rules for Remark
	}

	if m.Sort != nil {
		// no validation rules for Sort
	}

	if m.Name != nil {
		// no validation rules for Name
	}

	if len(errors) > 0 {
		return ReleaseMultiError(errors)
	}

	return nil
}

// ReleaseMultiError is an error wrapping multiple validation errors returned
// by Release.ValidateAll() if the designated constraints aren't met.
type ReleaseMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m ReleaseMultiError) Error() string {
	msgs := make([]string, 0, len(m))
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m ReleaseMultiError) AllErrors() []error { return m }

// ReleaseValidationError is the validation error returned by Release.Validate
// if the designated constraints aren't met.
type ReleaseValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e ReleaseValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e ReleaseValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e ReleaseValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e ReleaseValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e ReleaseValidationError) ErrorName() string { return "ReleaseValidationError" }

// Error satisfies the builtin error interface
func (e ReleaseValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sRelease.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = ReleaseValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = ReleaseValidationError{}

// Validate checks the field values on CreateReleaseRequest with the rules
// defined in the proto definition for this message. If any rules are
// violated, the first error encountered is returned, or nil if there are no violations.
func (m *CreateReleaseRequest) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on CreateReleaseRequest with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// CreateReleaseRequestMultiError, or nil if none found.
func (m *CreateReleaseRequest) ValidateAll() error {
	return m.validate(true)
}

func (m *CreateReleaseRequest) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if all {
		switch v := interface{}(m.GetRelease()).(type) {
		case interface{ ValidateAll() error }:
			if err := v.ValidateAll(); err != nil {
				errors = append(errors, CreateReleaseRequestValidationError{
					field:  "Release",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		case interface{ Validate() error }:
			if err := v.Validate(); err != nil {
				errors = append(errors, CreateReleaseRequestValidationError{
					field:  "Release",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		}
	} else if v, ok := interface{}(m.GetRelease()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return CreateReleaseRequestValidationError{
				field:  "Release",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	// no validation rules for OperatorId

	if len(errors) > 0 {
		return CreateReleaseRequestMultiError(errors)
	}

	return nil
}

// CreateReleaseRequestMultiError is an error wrapping multiple validation
// errors returned by CreateReleaseRequest.ValidateAll() if the designated
// constraints aren't met.
type CreateReleaseRequestMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m CreateReleaseRequestMultiError) Error() string {
	msgs := make([]string, 0, len(m))
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m CreateReleaseRequestMultiError) AllErrors() []error { return m }

// CreateReleaseRequestValidationError is the validation error returned by
// CreateReleaseRequest.Validate if the designated constraints aren't met.
type CreateReleaseRequestValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e CreateReleaseRequestValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e CreateReleaseRequestValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e CreateReleaseRequestValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e CreateReleaseRequestValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e CreateReleaseRequestValidationError) ErrorName() string {
	return "CreateReleaseRequestValidationError"
}

// Error satisfies the builtin error interface
func (e CreateReleaseRequestValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sCreateReleaseRequest.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = CreateReleaseRequestValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = CreateReleaseRequestValidationError{}

// Validate checks the field values on CreateReleaseResponse with the rules
// defined in the proto definition for this message. If any rules are
// violated, the first error encountered is returned, or nil if there are no violations.
func (m *CreateReleaseResponse) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on CreateReleaseResponse with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// CreateReleaseResponseMultiError, or nil if none found.
func (m *CreateReleaseResponse) ValidateAll() error {
	return m.validate(true)
}

func (m *CreateReleaseResponse) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if len(errors) > 0 {
		return CreateReleaseResponseMultiError(errors)
	}

	return nil
}

// CreateReleaseResponseMultiError is an error wrapping multiple validation
// errors returned by CreateReleaseResponse.ValidateAll() if the designated
// constraints aren't met.
type CreateReleaseResponseMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m CreateReleaseResponseMultiError) Error() string {
	msgs := make([]string, 0, len(m))
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m CreateReleaseResponseMultiError) AllErrors() []error { return m }

// CreateReleaseResponseValidationError is the validation error returned by
// CreateReleaseResponse.Validate if the designated constraints aren't met.
type CreateReleaseResponseValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e CreateReleaseResponseValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e CreateReleaseResponseValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e CreateReleaseResponseValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e CreateReleaseResponseValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e CreateReleaseResponseValidationError) ErrorName() string {
	return "CreateReleaseResponseValidationError"
}

// Error satisfies the builtin error interface
func (e CreateReleaseResponseValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sCreateReleaseResponse.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = CreateReleaseResponseValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = CreateReleaseResponseValidationError{}

// Validate checks the field values on UpdateReleaseRequest with the rules
// defined in the proto definition for this message. If any rules are
// violated, the first error encountered is returned, or nil if there are no violations.
func (m *UpdateReleaseRequest) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on UpdateReleaseRequest with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// UpdateReleaseRequestMultiError, or nil if none found.
func (m *UpdateReleaseRequest) ValidateAll() error {
	return m.validate(true)
}

func (m *UpdateReleaseRequest) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for Id

	if all {
		switch v := interface{}(m.GetRelease()).(type) {
		case interface{ ValidateAll() error }:
			if err := v.ValidateAll(); err != nil {
				errors = append(errors, UpdateReleaseRequestValidationError{
					field:  "Release",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		case interface{ Validate() error }:
			if err := v.Validate(); err != nil {
				errors = append(errors, UpdateReleaseRequestValidationError{
					field:  "Release",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		}
	} else if v, ok := interface{}(m.GetRelease()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return UpdateReleaseRequestValidationError{
				field:  "Release",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	// no validation rules for OperatorId

	if len(errors) > 0 {
		return UpdateReleaseRequestMultiError(errors)
	}

	return nil
}

// UpdateReleaseRequestMultiError is an error wrapping multiple validation
// errors returned by UpdateReleaseRequest.ValidateAll() if the designated
// constraints aren't met.
type UpdateReleaseRequestMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m UpdateReleaseRequestMultiError) Error() string {
	msgs := make([]string, 0, len(m))
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m UpdateReleaseRequestMultiError) AllErrors() []error { return m }

// UpdateReleaseRequestValidationError is the validation error returned by
// UpdateReleaseRequest.Validate if the designated constraints aren't met.
type UpdateReleaseRequestValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e UpdateReleaseRequestValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e UpdateReleaseRequestValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e UpdateReleaseRequestValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e UpdateReleaseRequestValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e UpdateReleaseRequestValidationError) ErrorName() string {
	return "UpdateReleaseRequestValidationError"
}

// Error satisfies the builtin error interface
func (e UpdateReleaseRequestValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sUpdateReleaseRequest.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = UpdateReleaseRequestValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = UpdateReleaseRequestValidationError{}

// Validate checks the field values on UpdateReleaseResponse with the rules
// defined in the proto definition for this message. If any rules are
// violated, the first error encountered is returned, or nil if there are no violations.
func (m *UpdateReleaseResponse) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on UpdateReleaseResponse with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// UpdateReleaseResponseMultiError, or nil if none found.
func (m *UpdateReleaseResponse) ValidateAll() error {
	return m.validate(true)
}

func (m *UpdateReleaseResponse) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if len(errors) > 0 {
		return UpdateReleaseResponseMultiError(errors)
	}

	return nil
}

// UpdateReleaseResponseMultiError is an error wrapping multiple validation
// errors returned by UpdateReleaseResponse.ValidateAll() if the designated
// constraints aren't met.
type UpdateReleaseResponseMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m UpdateReleaseResponseMultiError) Error() string {
	msgs := make([]string, 0, len(m))
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m UpdateReleaseResponseMultiError) AllErrors() []error { return m }

// UpdateReleaseResponseValidationError is the validation error returned by
// UpdateReleaseResponse.Validate if the designated constraints aren't met.
type UpdateReleaseResponseValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e UpdateReleaseResponseValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e UpdateReleaseResponseValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e UpdateReleaseResponseValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e UpdateReleaseResponseValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e UpdateReleaseResponseValidationError) ErrorName() string {
	return "UpdateReleaseResponseValidationError"
}

// Error satisfies the builtin error interface
func (e UpdateReleaseResponseValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sUpdateReleaseResponse.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = UpdateReleaseResponseValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = UpdateReleaseResponseValidationError{}

// Validate checks the field values on DeleteReleaseRequest with the rules
// defined in the proto definition for this message. If any rules are
// violated, the first error encountered is returned, or nil if there are no violations.
func (m *DeleteReleaseRequest) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on DeleteReleaseRequest with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// DeleteReleaseRequestMultiError, or nil if none found.
func (m *DeleteReleaseRequest) ValidateAll() error {
	return m.validate(true)
}

func (m *DeleteReleaseRequest) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for Id

	// no validation rules for OperatorId

	if len(errors) > 0 {
		return DeleteReleaseRequestMultiError(errors)
	}

	return nil
}

// DeleteReleaseRequestMultiError is an error wrapping multiple validation
// errors returned by DeleteReleaseRequest.ValidateAll() if the designated
// constraints aren't met.
type DeleteReleaseRequestMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m DeleteReleaseRequestMultiError) Error() string {
	msgs := make([]string, 0, len(m))
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m DeleteReleaseRequestMultiError) AllErrors() []error { return m }

// DeleteReleaseRequestValidationError is the validation error returned by
// DeleteReleaseRequest.Validate if the designated constraints aren't met.
type DeleteReleaseRequestValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e DeleteReleaseRequestValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e DeleteReleaseRequestValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e DeleteReleaseRequestValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e DeleteReleaseRequestValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e DeleteReleaseRequestValidationError) ErrorName() string {
	return "DeleteReleaseRequestValidationError"
}

// Error satisfies the builtin error interface
func (e DeleteReleaseRequestValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sDeleteReleaseRequest.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = DeleteReleaseRequestValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = DeleteReleaseRequestValidationError{}

// Validate checks the field values on DeleteReleaseResponse with the rules
// defined in the proto definition for this message. If any rules are
// violated, the first error encountered is returned, or nil if there are no violations.
func (m *DeleteReleaseResponse) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on DeleteReleaseResponse with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// DeleteReleaseResponseMultiError, or nil if none found.
func (m *DeleteReleaseResponse) ValidateAll() error {
	return m.validate(true)
}

func (m *DeleteReleaseResponse) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if len(errors) > 0 {
		return DeleteReleaseResponseMultiError(errors)
	}

	return nil
}

// DeleteReleaseResponseMultiError is an error wrapping multiple validation
// errors returned by DeleteReleaseResponse.ValidateAll() if the designated
// constraints aren't met.
type DeleteReleaseResponseMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m DeleteReleaseResponseMultiError) Error() string {
	msgs := make([]string, 0, len(m))
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m DeleteReleaseResponseMultiError) AllErrors() []error { return m }

// DeleteReleaseResponseValidationError is the validation error returned by
// DeleteReleaseResponse.Validate if the designated constraints aren't met.
type DeleteReleaseResponseValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e DeleteReleaseResponseValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e DeleteReleaseResponseValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e DeleteReleaseResponseValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e DeleteReleaseResponseValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e DeleteReleaseResponseValidationError) ErrorName() string {
	return "DeleteReleaseResponseValidationError"
}

// Error satisfies the builtin error interface
func (e DeleteReleaseResponseValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sDeleteReleaseResponse.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = DeleteReleaseResponseValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = DeleteReleaseResponseValidationError{}

// Validate checks the field values on GetReleaseRequest with the rules defined
// in the proto definition for this message. If any rules are violated, the
// first error encountered is returned, or nil if there are no violations.
func (m *GetReleaseRequest) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on GetReleaseRequest with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// GetReleaseRequestMultiError, or nil if none found.
func (m *GetReleaseRequest) ValidateAll() error {
	return m.validate(true)
}

func (m *GetReleaseRequest) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for Id

	if len(errors) > 0 {
		return GetReleaseRequestMultiError(errors)
	}

	return nil
}

// GetReleaseRequestMultiError is an error wrapping multiple validation errors
// returned by GetReleaseRequest.ValidateAll() if the designated constraints
// aren't met.
type GetReleaseRequestMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m GetReleaseRequestMultiError) Error() string {
	msgs := make([]string, 0, len(m))
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m GetReleaseRequestMultiError) AllErrors() []error { return m }

// GetReleaseRequestValidationError is the validation error returned by
// GetReleaseRequest.Validate if the designated constraints aren't met.
type GetReleaseRequestValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e GetReleaseRequestValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e GetReleaseRequestValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e GetReleaseRequestValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e GetReleaseRequestValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e GetReleaseRequestValidationError) ErrorName() string {
	return "GetReleaseRequestValidationError"
}

// Error satisfies the builtin error interface
func (e GetReleaseRequestValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sGetReleaseRequest.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = GetReleaseRequestValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = GetReleaseRequestValidationError{}

// Validate checks the field values on GetReleaseResponse with the rules
// defined in the proto definition for this message. If any rules are
// violated, the first error encountered is returned, or nil if there are no violations.
func (m *GetReleaseResponse) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on GetReleaseResponse with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// GetReleaseResponseMultiError, or nil if none found.
func (m *GetReleaseResponse) ValidateAll() error {
	return m.validate(true)
}

func (m *GetReleaseResponse) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if len(errors) > 0 {
		return GetReleaseResponseMultiError(errors)
	}

	return nil
}

// GetReleaseResponseMultiError is an error wrapping multiple validation errors
// returned by GetReleaseResponse.ValidateAll() if the designated constraints
// aren't met.
type GetReleaseResponseMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m GetReleaseResponseMultiError) Error() string {
	msgs := make([]string, 0, len(m))
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m GetReleaseResponseMultiError) AllErrors() []error { return m }

// GetReleaseResponseValidationError is the validation error returned by
// GetReleaseResponse.Validate if the designated constraints aren't met.
type GetReleaseResponseValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e GetReleaseResponseValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e GetReleaseResponseValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e GetReleaseResponseValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e GetReleaseResponseValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e GetReleaseResponseValidationError) ErrorName() string {
	return "GetReleaseResponseValidationError"
}

// Error satisfies the builtin error interface
func (e GetReleaseResponseValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sGetReleaseResponse.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = GetReleaseResponseValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = GetReleaseResponseValidationError{}

// Validate checks the field values on ListReleaseRequest with the rules
// defined in the proto definition for this message. If any rules are
// violated, the first error encountered is returned, or nil if there are no violations.
func (m *ListReleaseRequest) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on ListReleaseRequest with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// ListReleaseRequestMultiError, or nil if none found.
func (m *ListReleaseRequest) ValidateAll() error {
	return m.validate(true)
}

func (m *ListReleaseRequest) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if all {
		switch v := interface{}(m.GetPaging()).(type) {
		case interface{ ValidateAll() error }:
			if err := v.ValidateAll(); err != nil {
				errors = append(errors, ListReleaseRequestValidationError{
					field:  "Paging",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		case interface{ Validate() error }:
			if err := v.Validate(); err != nil {
				errors = append(errors, ListReleaseRequestValidationError{
					field:  "Paging",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		}
	} else if v, ok := interface{}(m.GetPaging()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return ListReleaseRequestValidationError{
				field:  "Paging",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	if len(errors) > 0 {
		return ListReleaseRequestMultiError(errors)
	}

	return nil
}

// ListReleaseRequestMultiError is an error wrapping multiple validation errors
// returned by ListReleaseRequest.ValidateAll() if the designated constraints
// aren't met.
type ListReleaseRequestMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m ListReleaseRequestMultiError) Error() string {
	msgs := make([]string, 0, len(m))
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m ListReleaseRequestMultiError) AllErrors() []error { return m }

// ListReleaseRequestValidationError is the validation error returned by
// ListReleaseRequest.Validate if the designated constraints aren't met.
type ListReleaseRequestValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e ListReleaseRequestValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e ListReleaseRequestValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e ListReleaseRequestValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e ListReleaseRequestValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e ListReleaseRequestValidationError) ErrorName() string {
	return "ListReleaseRequestValidationError"
}

// Error satisfies the builtin error interface
func (e ListReleaseRequestValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sListReleaseRequest.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = ListReleaseRequestValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = ListReleaseRequestValidationError{}

// Validate checks the field values on ListReleaseResponse with the rules
// defined in the proto definition for this message. If any rules are
// violated, the first error encountered is returned, or nil if there are no violations.
func (m *ListReleaseResponse) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on ListReleaseResponse with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// ListReleaseResponseMultiError, or nil if none found.
func (m *ListReleaseResponse) ValidateAll() error {
	return m.validate(true)
}

func (m *ListReleaseResponse) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	for idx, item := range m.GetItems() {
		_, _ = idx, item

		if all {
			switch v := interface{}(item).(type) {
			case interface{ ValidateAll() error }:
				if err := v.ValidateAll(); err != nil {
					errors = append(errors, ListReleaseResponseValidationError{
						field:  fmt.Sprintf("Items[%v]", idx),
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			case interface{ Validate() error }:
				if err := v.Validate(); err != nil {
					errors = append(errors, ListReleaseResponseValidationError{
						field:  fmt.Sprintf("Items[%v]", idx),
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			}
		} else if v, ok := interface{}(item).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return ListReleaseResponseValidationError{
					field:  fmt.Sprintf("Items[%v]", idx),
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	}

	// no validation rules for Total

	if len(errors) > 0 {
		return ListReleaseResponseMultiError(errors)
	}

	return nil
}

// ListReleaseResponseMultiError is an error wrapping multiple validation
// errors returned by ListReleaseResponse.ValidateAll() if the designated
// constraints aren't met.
type ListReleaseResponseMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m ListReleaseResponseMultiError) Error() string {
	msgs := make([]string, 0, len(m))
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m ListReleaseResponseMultiError) AllErrors() []error { return m }

// ListReleaseResponseValidationError is the validation error returned by
// ListReleaseResponse.Validate if the designated constraints aren't met.
type ListReleaseResponseValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e ListReleaseResponseValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e ListReleaseResponseValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e ListReleaseResponseValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e ListReleaseResponseValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e ListReleaseResponseValidationError) ErrorName() string {
	return "ListReleaseResponseValidationError"
}

// Error satisfies the builtin error interface
func (e ListReleaseResponseValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sListReleaseResponse.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = ListReleaseResponseValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = ListReleaseResponseValidationError{}
