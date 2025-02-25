// Code generated by protoc-gen-validate. DO NOT EDIT.
// source: admin/service/v1/post.proto

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

// Validate checks the field values on Post with the rules defined in the proto
// definition for this message. If any rules are violated, the first error
// encountered is returned, or nil if there are no violations.
func (m *Post) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on Post with the rules defined in the
// proto definition for this message. If any rules are violated, the result is
// a list of violation errors wrapped in PostMultiError, or nil if none found.
func (m *Post) ValidateAll() error {
	return m.validate(true)
}

func (m *Post) validate(all bool) error {
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
		return PostMultiError(errors)
	}

	return nil
}

// PostMultiError is an error wrapping multiple validation errors returned by
// Post.ValidateAll() if the designated constraints aren't met.
type PostMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m PostMultiError) Error() string {
	msgs := make([]string, 0, len(m))
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m PostMultiError) AllErrors() []error { return m }

// PostValidationError is the validation error returned by Post.Validate if the
// designated constraints aren't met.
type PostValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e PostValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e PostValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e PostValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e PostValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e PostValidationError) ErrorName() string { return "PostValidationError" }

// Error satisfies the builtin error interface
func (e PostValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sPost.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = PostValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = PostValidationError{}

// Validate checks the field values on CreatePostRequest with the rules defined
// in the proto definition for this message. If any rules are violated, the
// first error encountered is returned, or nil if there are no violations.
func (m *CreatePostRequest) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on CreatePostRequest with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// CreatePostRequestMultiError, or nil if none found.
func (m *CreatePostRequest) ValidateAll() error {
	return m.validate(true)
}

func (m *CreatePostRequest) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if all {
		switch v := interface{}(m.GetPost()).(type) {
		case interface{ ValidateAll() error }:
			if err := v.ValidateAll(); err != nil {
				errors = append(errors, CreatePostRequestValidationError{
					field:  "Post",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		case interface{ Validate() error }:
			if err := v.Validate(); err != nil {
				errors = append(errors, CreatePostRequestValidationError{
					field:  "Post",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		}
	} else if v, ok := interface{}(m.GetPost()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return CreatePostRequestValidationError{
				field:  "Post",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	// no validation rules for OperatorId

	if len(errors) > 0 {
		return CreatePostRequestMultiError(errors)
	}

	return nil
}

// CreatePostRequestMultiError is an error wrapping multiple validation errors
// returned by CreatePostRequest.ValidateAll() if the designated constraints
// aren't met.
type CreatePostRequestMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m CreatePostRequestMultiError) Error() string {
	msgs := make([]string, 0, len(m))
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m CreatePostRequestMultiError) AllErrors() []error { return m }

// CreatePostRequestValidationError is the validation error returned by
// CreatePostRequest.Validate if the designated constraints aren't met.
type CreatePostRequestValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e CreatePostRequestValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e CreatePostRequestValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e CreatePostRequestValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e CreatePostRequestValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e CreatePostRequestValidationError) ErrorName() string {
	return "CreatePostRequestValidationError"
}

// Error satisfies the builtin error interface
func (e CreatePostRequestValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sCreatePostRequest.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = CreatePostRequestValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = CreatePostRequestValidationError{}

// Validate checks the field values on CreatePostResponse with the rules
// defined in the proto definition for this message. If any rules are
// violated, the first error encountered is returned, or nil if there are no violations.
func (m *CreatePostResponse) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on CreatePostResponse with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// CreatePostResponseMultiError, or nil if none found.
func (m *CreatePostResponse) ValidateAll() error {
	return m.validate(true)
}

func (m *CreatePostResponse) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if len(errors) > 0 {
		return CreatePostResponseMultiError(errors)
	}

	return nil
}

// CreatePostResponseMultiError is an error wrapping multiple validation errors
// returned by CreatePostResponse.ValidateAll() if the designated constraints
// aren't met.
type CreatePostResponseMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m CreatePostResponseMultiError) Error() string {
	msgs := make([]string, 0, len(m))
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m CreatePostResponseMultiError) AllErrors() []error { return m }

// CreatePostResponseValidationError is the validation error returned by
// CreatePostResponse.Validate if the designated constraints aren't met.
type CreatePostResponseValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e CreatePostResponseValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e CreatePostResponseValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e CreatePostResponseValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e CreatePostResponseValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e CreatePostResponseValidationError) ErrorName() string {
	return "CreatePostResponseValidationError"
}

// Error satisfies the builtin error interface
func (e CreatePostResponseValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sCreatePostResponse.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = CreatePostResponseValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = CreatePostResponseValidationError{}

// Validate checks the field values on UpdatePostRequest with the rules defined
// in the proto definition for this message. If any rules are violated, the
// first error encountered is returned, or nil if there are no violations.
func (m *UpdatePostRequest) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on UpdatePostRequest with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// UpdatePostRequestMultiError, or nil if none found.
func (m *UpdatePostRequest) ValidateAll() error {
	return m.validate(true)
}

func (m *UpdatePostRequest) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for Id

	if all {
		switch v := interface{}(m.GetPost()).(type) {
		case interface{ ValidateAll() error }:
			if err := v.ValidateAll(); err != nil {
				errors = append(errors, UpdatePostRequestValidationError{
					field:  "Post",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		case interface{ Validate() error }:
			if err := v.Validate(); err != nil {
				errors = append(errors, UpdatePostRequestValidationError{
					field:  "Post",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		}
	} else if v, ok := interface{}(m.GetPost()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return UpdatePostRequestValidationError{
				field:  "Post",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	// no validation rules for OperatorId

	if len(errors) > 0 {
		return UpdatePostRequestMultiError(errors)
	}

	return nil
}

// UpdatePostRequestMultiError is an error wrapping multiple validation errors
// returned by UpdatePostRequest.ValidateAll() if the designated constraints
// aren't met.
type UpdatePostRequestMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m UpdatePostRequestMultiError) Error() string {
	msgs := make([]string, 0, len(m))
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m UpdatePostRequestMultiError) AllErrors() []error { return m }

// UpdatePostRequestValidationError is the validation error returned by
// UpdatePostRequest.Validate if the designated constraints aren't met.
type UpdatePostRequestValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e UpdatePostRequestValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e UpdatePostRequestValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e UpdatePostRequestValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e UpdatePostRequestValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e UpdatePostRequestValidationError) ErrorName() string {
	return "UpdatePostRequestValidationError"
}

// Error satisfies the builtin error interface
func (e UpdatePostRequestValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sUpdatePostRequest.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = UpdatePostRequestValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = UpdatePostRequestValidationError{}

// Validate checks the field values on UpdatePostResponse with the rules
// defined in the proto definition for this message. If any rules are
// violated, the first error encountered is returned, or nil if there are no violations.
func (m *UpdatePostResponse) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on UpdatePostResponse with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// UpdatePostResponseMultiError, or nil if none found.
func (m *UpdatePostResponse) ValidateAll() error {
	return m.validate(true)
}

func (m *UpdatePostResponse) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if len(errors) > 0 {
		return UpdatePostResponseMultiError(errors)
	}

	return nil
}

// UpdatePostResponseMultiError is an error wrapping multiple validation errors
// returned by UpdatePostResponse.ValidateAll() if the designated constraints
// aren't met.
type UpdatePostResponseMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m UpdatePostResponseMultiError) Error() string {
	msgs := make([]string, 0, len(m))
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m UpdatePostResponseMultiError) AllErrors() []error { return m }

// UpdatePostResponseValidationError is the validation error returned by
// UpdatePostResponse.Validate if the designated constraints aren't met.
type UpdatePostResponseValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e UpdatePostResponseValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e UpdatePostResponseValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e UpdatePostResponseValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e UpdatePostResponseValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e UpdatePostResponseValidationError) ErrorName() string {
	return "UpdatePostResponseValidationError"
}

// Error satisfies the builtin error interface
func (e UpdatePostResponseValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sUpdatePostResponse.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = UpdatePostResponseValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = UpdatePostResponseValidationError{}

// Validate checks the field values on DeletePostRequest with the rules defined
// in the proto definition for this message. If any rules are violated, the
// first error encountered is returned, or nil if there are no violations.
func (m *DeletePostRequest) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on DeletePostRequest with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// DeletePostRequestMultiError, or nil if none found.
func (m *DeletePostRequest) ValidateAll() error {
	return m.validate(true)
}

func (m *DeletePostRequest) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for Id

	// no validation rules for OperatorId

	if len(errors) > 0 {
		return DeletePostRequestMultiError(errors)
	}

	return nil
}

// DeletePostRequestMultiError is an error wrapping multiple validation errors
// returned by DeletePostRequest.ValidateAll() if the designated constraints
// aren't met.
type DeletePostRequestMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m DeletePostRequestMultiError) Error() string {
	msgs := make([]string, 0, len(m))
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m DeletePostRequestMultiError) AllErrors() []error { return m }

// DeletePostRequestValidationError is the validation error returned by
// DeletePostRequest.Validate if the designated constraints aren't met.
type DeletePostRequestValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e DeletePostRequestValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e DeletePostRequestValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e DeletePostRequestValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e DeletePostRequestValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e DeletePostRequestValidationError) ErrorName() string {
	return "DeletePostRequestValidationError"
}

// Error satisfies the builtin error interface
func (e DeletePostRequestValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sDeletePostRequest.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = DeletePostRequestValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = DeletePostRequestValidationError{}

// Validate checks the field values on DeletePostResponse with the rules
// defined in the proto definition for this message. If any rules are
// violated, the first error encountered is returned, or nil if there are no violations.
func (m *DeletePostResponse) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on DeletePostResponse with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// DeletePostResponseMultiError, or nil if none found.
func (m *DeletePostResponse) ValidateAll() error {
	return m.validate(true)
}

func (m *DeletePostResponse) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if len(errors) > 0 {
		return DeletePostResponseMultiError(errors)
	}

	return nil
}

// DeletePostResponseMultiError is an error wrapping multiple validation errors
// returned by DeletePostResponse.ValidateAll() if the designated constraints
// aren't met.
type DeletePostResponseMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m DeletePostResponseMultiError) Error() string {
	msgs := make([]string, 0, len(m))
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m DeletePostResponseMultiError) AllErrors() []error { return m }

// DeletePostResponseValidationError is the validation error returned by
// DeletePostResponse.Validate if the designated constraints aren't met.
type DeletePostResponseValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e DeletePostResponseValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e DeletePostResponseValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e DeletePostResponseValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e DeletePostResponseValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e DeletePostResponseValidationError) ErrorName() string {
	return "DeletePostResponseValidationError"
}

// Error satisfies the builtin error interface
func (e DeletePostResponseValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sDeletePostResponse.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = DeletePostResponseValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = DeletePostResponseValidationError{}

// Validate checks the field values on GetPostRequest with the rules defined in
// the proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *GetPostRequest) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on GetPostRequest with the rules defined
// in the proto definition for this message. If any rules are violated, the
// result is a list of violation errors wrapped in GetPostRequestMultiError,
// or nil if none found.
func (m *GetPostRequest) ValidateAll() error {
	return m.validate(true)
}

func (m *GetPostRequest) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for Id

	if len(errors) > 0 {
		return GetPostRequestMultiError(errors)
	}

	return nil
}

// GetPostRequestMultiError is an error wrapping multiple validation errors
// returned by GetPostRequest.ValidateAll() if the designated constraints
// aren't met.
type GetPostRequestMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m GetPostRequestMultiError) Error() string {
	msgs := make([]string, 0, len(m))
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m GetPostRequestMultiError) AllErrors() []error { return m }

// GetPostRequestValidationError is the validation error returned by
// GetPostRequest.Validate if the designated constraints aren't met.
type GetPostRequestValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e GetPostRequestValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e GetPostRequestValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e GetPostRequestValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e GetPostRequestValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e GetPostRequestValidationError) ErrorName() string { return "GetPostRequestValidationError" }

// Error satisfies the builtin error interface
func (e GetPostRequestValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sGetPostRequest.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = GetPostRequestValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = GetPostRequestValidationError{}

// Validate checks the field values on GetPostResponse with the rules defined
// in the proto definition for this message. If any rules are violated, the
// first error encountered is returned, or nil if there are no violations.
func (m *GetPostResponse) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on GetPostResponse with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// GetPostResponseMultiError, or nil if none found.
func (m *GetPostResponse) ValidateAll() error {
	return m.validate(true)
}

func (m *GetPostResponse) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if len(errors) > 0 {
		return GetPostResponseMultiError(errors)
	}

	return nil
}

// GetPostResponseMultiError is an error wrapping multiple validation errors
// returned by GetPostResponse.ValidateAll() if the designated constraints
// aren't met.
type GetPostResponseMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m GetPostResponseMultiError) Error() string {
	msgs := make([]string, 0, len(m))
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m GetPostResponseMultiError) AllErrors() []error { return m }

// GetPostResponseValidationError is the validation error returned by
// GetPostResponse.Validate if the designated constraints aren't met.
type GetPostResponseValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e GetPostResponseValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e GetPostResponseValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e GetPostResponseValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e GetPostResponseValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e GetPostResponseValidationError) ErrorName() string { return "GetPostResponseValidationError" }

// Error satisfies the builtin error interface
func (e GetPostResponseValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sGetPostResponse.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = GetPostResponseValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = GetPostResponseValidationError{}

// Validate checks the field values on ListPostRequest with the rules defined
// in the proto definition for this message. If any rules are violated, the
// first error encountered is returned, or nil if there are no violations.
func (m *ListPostRequest) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on ListPostRequest with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// ListPostRequestMultiError, or nil if none found.
func (m *ListPostRequest) ValidateAll() error {
	return m.validate(true)
}

func (m *ListPostRequest) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if len(errors) > 0 {
		return ListPostRequestMultiError(errors)
	}

	return nil
}

// ListPostRequestMultiError is an error wrapping multiple validation errors
// returned by ListPostRequest.ValidateAll() if the designated constraints
// aren't met.
type ListPostRequestMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m ListPostRequestMultiError) Error() string {
	msgs := make([]string, 0, len(m))
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m ListPostRequestMultiError) AllErrors() []error { return m }

// ListPostRequestValidationError is the validation error returned by
// ListPostRequest.Validate if the designated constraints aren't met.
type ListPostRequestValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e ListPostRequestValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e ListPostRequestValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e ListPostRequestValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e ListPostRequestValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e ListPostRequestValidationError) ErrorName() string { return "ListPostRequestValidationError" }

// Error satisfies the builtin error interface
func (e ListPostRequestValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sListPostRequest.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = ListPostRequestValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = ListPostRequestValidationError{}

// Validate checks the field values on ListPostResponse with the rules defined
// in the proto definition for this message. If any rules are violated, the
// first error encountered is returned, or nil if there are no violations.
func (m *ListPostResponse) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on ListPostResponse with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// ListPostResponseMultiError, or nil if none found.
func (m *ListPostResponse) ValidateAll() error {
	return m.validate(true)
}

func (m *ListPostResponse) validate(all bool) error {
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
					errors = append(errors, ListPostResponseValidationError{
						field:  fmt.Sprintf("Items[%v]", idx),
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			case interface{ Validate() error }:
				if err := v.Validate(); err != nil {
					errors = append(errors, ListPostResponseValidationError{
						field:  fmt.Sprintf("Items[%v]", idx),
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			}
		} else if v, ok := interface{}(item).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return ListPostResponseValidationError{
					field:  fmt.Sprintf("Items[%v]", idx),
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	}

	// no validation rules for Total

	if len(errors) > 0 {
		return ListPostResponseMultiError(errors)
	}

	return nil
}

// ListPostResponseMultiError is an error wrapping multiple validation errors
// returned by ListPostResponse.ValidateAll() if the designated constraints
// aren't met.
type ListPostResponseMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m ListPostResponseMultiError) Error() string {
	msgs := make([]string, 0, len(m))
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m ListPostResponseMultiError) AllErrors() []error { return m }

// ListPostResponseValidationError is the validation error returned by
// ListPostResponse.Validate if the designated constraints aren't met.
type ListPostResponseValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e ListPostResponseValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e ListPostResponseValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e ListPostResponseValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e ListPostResponseValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e ListPostResponseValidationError) ErrorName() string { return "ListPostResponseValidationError" }

// Error satisfies the builtin error interface
func (e ListPostResponseValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sListPostResponse.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = ListPostResponseValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = ListPostResponseValidationError{}
