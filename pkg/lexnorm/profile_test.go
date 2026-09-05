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

package lexnorm_test

import (
	"testing"

	"github.com/stack-haven/lexnorm"
)

func TestProfileID_String(t *testing.T) {
	if got := (lexnorm.ProfileID("default")).String(); got != "default" {
		t.Errorf("ProfileID.String() = %q, want %q", got, "default")
	}
	if got := (lexnorm.ProfileID("")).String(); got != "" {
		t.Errorf("ProfileID(\"\").String() = %q, want empty", got)
	}
}

func TestProfileID_IsZero(t *testing.T) {
	if !(lexnorm.ProfileID("")).IsZero() {
		t.Error("empty ProfileID must be zero")
	}
	if (lexnorm.ProfileID("default")).IsZero() {
		t.Error("non-empty ProfileID must NOT be zero")
	}
}

func TestProfile_IsZero(t *testing.T) {
	if !(lexnorm.Profile{}).IsZero() {
		t.Error("zero Profile must be zero")
	}
	if (lexnorm.Profile{ID: "default"}).IsZero() {
		t.Error("Profile with ID must NOT be zero")
	}
	if (lexnorm.Profile{Version: "v1"}).IsZero() {
		t.Error("Profile with Version but empty ID must NOT be zero")
	}
}

func TestProfile_IsValid(t *testing.T) {
	tests := []struct {
		p     lexnorm.Profile
		valid bool
	}{
		{lexnorm.Profile{}, false},
		{lexnorm.Profile{ID: "default"}, true},
		{lexnorm.Profile{ID: "default", Version: "v1"}, true},
		{lexnorm.Profile{Version: "v1"}, false}, // empty ID is invalid
	}
	for _, tc := range tests {
		if got := tc.p.IsValid(); got != tc.valid {
			t.Errorf("Profile{%s,%s}.IsValid() = %v, want %v",
				tc.p.ID, tc.p.Version, got, tc.valid)
		}
	}
}

func TestProfile_String(t *testing.T) {
	tests := []struct {
		p    lexnorm.Profile
		want string
	}{
		{lexnorm.Profile{ID: "default"}, "Profile{default}"},
		{lexnorm.Profile{ID: "asr", Version: "v20240904"}, "Profile{asr@v20240904}"},
		{lexnorm.Profile{ID: "default", Version: ""}, "Profile{default}"},
	}
	for _, tc := range tests {
		if got := tc.p.String(); got != tc.want {
			t.Errorf("Profile.String() = %q, want %q", got, tc.want)
		}
	}
}

func TestProfile_NotTenant(t *testing.T) {
	// Documents that ProfileID is application-defined. It is NOT
	// interpreted as a tenant boundary by the engine.
	// Profile ≠ Tenant (invariant I11).
	customID := lexnorm.ProfileID("org-12345")
	p := lexnorm.Profile{ID: customID, Version: "v1"}
	if !p.IsValid() {
		t.Error("any non-empty ProfileID must be valid; engine does not interpret")
	}
}
