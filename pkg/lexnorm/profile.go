// Copyright 2024 The Ark Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lexnorm

import "fmt"

// ProfileID is a stable identifier for a Profile.
//
// ProfileID is application-defined (e.g., "default", "asr-prod-v3"). The
// engine does not interpret the value; it is used as a key into the
// ProfileResolver and as the ProfileID field in Result.RuntimeInfo.
type ProfileID string

// String returns the underlying string. It exists so ProfileID satisfies
// fmt.Stringer.
func (id ProfileID) String() string {
	return string(id)
}

// IsZero reports whether id is the empty string.
func (id ProfileID) IsZero() bool {
	return id == ""
}

// Profile identifies a normalization context.
//
// Profile is intentionally minimal: it carries identity (ID) and version
// (Version), but does NOT bundle Lexicon, Pipeline, Config, or any other
// runtime object. Bundling those would conflate identity with binding and
// make hot-swapping harder.
//
// # Profile Is Not Tenant
//
// Profile is a **normalization context identifier**, not a business
// tenant. The engine never interprets ProfileID as a tenant boundary;
// authorization and isolation are the caller's responsibility (see
// Architecture Invariant I11: Profile ≠ Tenant).
//
// # Profile Lifecycle
//
// Profile objects are value types and are immutable. Lexicon / Pipeline /
// Config associated with a Profile may change via atomic Snapshot swap
// (see Lexicon Store) without changing Profile identity.
type Profile struct {
	// ID is the stable identifier used to resolve a Profile via
	// ProfileResolver. Required.
	ID ProfileID

	// Version is an opaque version string (e.g., "v20240904-001",
	// "git-sha1:abc123"). Used for change detection and audit.
	// Optional: empty string is allowed when versioning is not yet
	// introduced.
	Version string
}

// IsZero reports whether p is the zero value.
func (p Profile) IsZero() bool {
	return p.ID.IsZero() && p.Version == ""
}

// IsValid reports whether p carries a non-empty ID.
func (p Profile) IsValid() bool {
	return !p.ID.IsZero()
}

// String returns "Profile{ID=vN}" or "Profile{ID}".
func (p Profile) String() string {
	if p.Version == "" {
		return fmt.Sprintf("Profile{%s}", p.ID)
	}
	return fmt.Sprintf("Profile{%s@%s}", p.ID, p.Version)
}
