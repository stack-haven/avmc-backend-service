package listing

import (
	"fmt"
	"strconv"

	"go.einride.tech/aip/filtering"
	"go.einride.tech/aip/ordering"
	"go.einride.tech/aip/pagination"
	"google.golang.org/protobuf/proto"
)

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// Request is the minimal AIP list request contract used by generated list APIs.
type Request interface {
	filtering.Request
	ordering.Request
	pagination.Request
}

// Params contains parsed AIP list parameters.
type Params struct {
	PageSize  int
	Filter    filtering.Filter
	PageToken pagination.PageToken
	OrderBy   ordering.OrderBy
}

// ParseParams parses AIP filtering, ordering, pagination, and clamps page size.
func ParseParams(req Request, fields ...filtering.DeclarationOption) (Params, error) {
	declarationOptions := make([]filtering.DeclarationOption, 0, len(fields)+1)
	declarationOptions = append(declarationOptions, filtering.DeclareStandardFunctions())
	declarationOptions = append(declarationOptions, fields...)
	declarations, err := filtering.NewDeclarations(declarationOptions...)
	if err != nil {
		return Params{}, err
	}
	filter, err := filtering.ParseFilter(req, declarations)
	if err != nil {
		return Params{}, err
	}
	pageToken, err := parsePageToken(req)
	if err != nil {
		return Params{}, err
	}
	orderBy, err := ordering.ParseOrderBy(req)
	if err != nil {
		return Params{}, err
	}
	return Params{
		PageSize:  NormalizePageSize(req.GetPageSize()),
		Filter:    filter,
		PageToken: pageToken,
		OrderBy:   orderBy,
	}, nil
}

// parsePageToken accepts AIP opaque tokens and the numeric offsets used by the
// administration console's page-number grids.
func parsePageToken(req Request) (pagination.PageToken, error) {
	raw := req.GetPageToken()
	if raw == "" {
		return pagination.ParsePageToken(req)
	}
	offset, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return pagination.ParsePageToken(req)
	}
	if offset < 0 {
		return pagination.PageToken{}, fmt.Errorf("page token offset must not be negative")
	}

	cloned := proto.Clone(req)
	cloned.ProtoReflect().Clear(cloned.ProtoReflect().Descriptor().Fields().ByName("page_token"))
	normalized, ok := cloned.(pagination.Request)
	if !ok {
		return pagination.PageToken{}, fmt.Errorf("request does not implement pagination request")
	}
	token, err := pagination.ParsePageToken(normalized)
	if err != nil {
		return pagination.PageToken{}, err
	}
	token.Offset = offset
	return token, nil
}

// Option configures repository list queries.
type Option func(*Options)

// Options contains repository list query parameters.
type Options struct {
	Filter  filtering.Filter
	OrderBy ordering.OrderBy
	Offset  int
	Limit   int
}

// FilterOption sets the AIP filter.
func FilterOption(filter filtering.Filter) Option {
	return func(o *Options) {
		o.Filter = filter
	}
}

// OrderByOption sets the AIP ordering.
func OrderByOption(orderBy ordering.OrderBy) Option {
	return func(o *Options) {
		o.OrderBy = orderBy
	}
}

// OffsetOption sets the list offset and clamps negative values to zero.
func OffsetOption(offset int) Option {
	return func(o *Options) {
		if offset < 0 {
			offset = 0
		}
		o.Offset = offset
	}
}

// LimitOption sets the list limit and clamps unsafe values.
func LimitOption(limit int) Option {
	return func(o *Options) {
		if limit <= 0 {
			limit = DefaultPageSize
		}
		if limit > MaxPageSize {
			limit = MaxPageSize
		}
		o.Limit = limit
	}
}

func NormalizePageSize(size int32) int {
	if size <= 0 {
		return DefaultPageSize
	}
	if size > MaxPageSize {
		return MaxPageSize
	}
	return int(size)
}

func DefaultOrderBy(field ordering.Field) ordering.OrderBy {
	oby := ordering.OrderBy{}
	oby.Fields = append(oby.Fields, field)
	return oby
}
