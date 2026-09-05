package credential

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseExpiresAt parses an upstream expiry timestamp into time.Time.
//
// Supported formats (tried in order):
//  1. Epoch milliseconds (int64 or float64 from JSON number)
//  2. Epoch seconds (int64 or float64)
//  3. RFC3339 string
//  4. Returns zero time if value is nil / missing / unrecognized.
//
// Returns (t, true) on success, (zero, false) on missing/invalid.
func ParseExpiresAt(v any) (time.Time, bool) {
	if v == nil {
		return time.Time{}, false
	}
	switch x := v.(type) {
	case float64:
		// JSON numbers arrive as float64. Distinguish ms vs s by magnitude.
		// Anything > 1e12 is almost certainly ms (years > ~33000 in seconds).
		if x > 1e12 {
			return time.UnixMilli(int64(x)), true
		}
		return time.Unix(int64(x), 0), true
	case int64:
		if x > 1e12 {
			return time.UnixMilli(x), true
		}
		return time.Unix(x, 0), true
	case int:
		return ParseExpiresAt(int64(x))
	case json.Number:
		if n, err := x.Int64(); err == nil {
			return ParseExpiresAt(n)
		}
		if f, err := x.Float64(); err == nil {
			return ParseExpiresAt(f)
		}
	case string:
		if x == "" {
			return time.Time{}, false
		}
		if t, err := time.Parse(time.RFC3339, x); err == nil {
			return t, true
		}
		if n, err := strconv.ParseInt(x, 10, 64); err == nil {
			return ParseExpiresAt(n)
		}
	}
	return time.Time{}, false
}

// LookupPath walks a dotted path inside a nested map[string]any.
//
//   - "tenantId"       → m["tenantId"]
//   - "userInfo.nickname" → m["userInfo"]["nickname"]
//
// Returns nil if any segment is missing or the intermediate value is
// not a map[string]any.
func LookupPath(m map[string]any, path string) any {
	if m == nil || path == "" {
		return nil
	}
	parts := strings.Split(path, ".")
	cur := any(m)
	for _, p := range parts {
		mp, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur, ok = mp[p]
		if !ok {
			return nil
		}
	}
	return cur
}

// ExtractString returns v as a string, tolerating JSON number types
// (float64, int64, json.Number) by formatting them losslessly.
//
// Returns "" if v is nil or cannot be sensibly stringified.
func ExtractString(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(x, 10)
	case int:
		return strconv.Itoa(x)
	case json.Number:
		return x.String()
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ExtractInt32 returns v as int32, accepting JSON number variants.
// Returns (0, false) if v is nil or not numeric.
func ExtractInt32(v any) (int32, bool) {
	if v == nil {
		return 0, false
	}
	switch x := v.(type) {
	case float64:
		return int32(x), true
	case int64:
		return int32(x), true
	case int:
		return int32(x), true
	case json.Number:
		if n, err := x.Int64(); err == nil {
			return int32(n), true
		}
	case string:
		if n, err := strconv.ParseInt(x, 10, 32); err == nil {
			return int32(n), true
		}
	}
	return 0, false
}

// MapFromMapper projects a raw payload (e.g. a JSON object) into a
// CallerIdentity using the field names declared in FieldMapper.
//
//   - root is typically a map[string]any parsed from JSON.
//   - Dotted paths (e.g. "userInfo.nickname") are resolved via LookupPath.
//   - Unknown JSON keys are preserved in Identity.Raw.
//   - Missing keys leave the corresponding field at its zero value.
func MapFromMapper(root map[string]any, m FieldMapper) CallerIdentity {
	get := func(path string) any {
		if path == "" {
			return nil
		}
		return LookupPath(root, path)
	}
	id := CallerIdentity{
		TenantID:     ExtractString(get(m.TenantID)),
		UserID:       ExtractString(get(m.UserID)),
		UserName:     ExtractString(get(m.UserName)),
		DeptID:       ExtractString(get(m.DeptID)),
		AccessToken:  ExtractString(get(m.AccessToken)),
		RefreshToken: ExtractString(get(m.RefreshToken)),
		Scopes:       get(m.Scopes),
		Raw:          root,
	}
	if v, ok := ExtractInt32(get(m.UserType)); ok {
		id.UserType = v
	}
	if m.ExpiresAt != "" {
		if t, ok := ParseExpiresAt(get(m.ExpiresAt)); ok {
			id.ExpiresAt = t
		}
	}
	return id
}
