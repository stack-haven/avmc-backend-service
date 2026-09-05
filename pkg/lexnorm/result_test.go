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
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stack-haven/lexnorm"
)

func TestResult_IsZero(t *testing.T) {
	if !(lexnorm.Result{}).IsZero() {
		t.Error("zero Result must be zero")
	}
	r := lexnorm.Result{Text: "abc"}
	if r.IsZero() {
		t.Error("Result with Text must NOT be zero")
	}
}

func TestResult_HasChanges(t *testing.T) {
	var r lexnorm.Result
	if r.HasChanges() {
		t.Error("empty Result must not HasChanges")
	}
	r.Changes = []lexnorm.Change{{From: "a", To: "b"}}
	if !r.HasChanges() {
		t.Error("Result with Changes must HasChanges")
	}
}

func TestResult_HasErrors(t *testing.T) {
	var r lexnorm.Result
	if r.HasErrors() {
		t.Error("empty Result must not HasErrors")
	}
	r.Errors = []error{errors.New("x")}
	if !r.HasErrors() {
		t.Error("Result with Errors must HasErrors")
	}
}

func TestResult_D3_AllFieldsPreserved(t *testing.T) {
	// D3: Result must retain ALL fields from both 1.0 and 1.1.
	// This test documents the field surface and fails if any field
	// is removed (via direct field access).
	r := lexnorm.Result{
		Text:        "normalized",
		Original:    "raw text",
		Status:      lexnorm.StatusSuccess,
		Changes:     []lexnorm.Change{{Span: lexnorm.Span{0, 3}, From: "raw", To: "fix"}},
		Suggestions: []lexnorm.Change{{Span: lexnorm.Span{4, 7}, From: "txt", To: "text", Applied: false}},
		Errors:      nil,
		Err:         nil,
		Duration:    100 * time.Microsecond,
		Steps:       []lexnorm.StepTiming{{Processor: "alias", ChangeCount: 1, Duration: 50 * time.Microsecond}},
		Runtime: lexnorm.RuntimeInfo{
			ProfileID:      "default",
			LexiconVersion: "v1",
		},
	}

	// Spot-check each field.
	if r.Text != "normalized" {
		t.Error("Text field missing (D3)")
	}
	if r.Original != "raw text" {
		t.Error("Original field missing (D3)")
	}
	if r.Status != lexnorm.StatusSuccess {
		t.Error("Status field missing")
	}
	if len(r.Changes) != 1 {
		t.Error("Changes field missing")
	}
	if len(r.Suggestions) != 1 {
		t.Error("Suggestions field missing (1.1+)")
	}
	if r.Duration != 100*time.Microsecond {
		t.Error("Duration field missing (D3 / 1.0)")
	}
	if len(r.Steps) != 1 {
		t.Error("Steps field missing (D3 / 1.0)")
	}
	if r.Runtime.ProfileID != "default" {
		t.Error("Runtime field missing (1.1+)")
	}
}

func TestResult_String(t *testing.T) {
	r := lexnorm.Result{
		Text:     "normalized text",
		Status:   lexnorm.StatusSuccess,
		Changes:  []lexnorm.Change{{}, {}},
		Duration: 500 * time.Microsecond,
	}
	got := r.String()
	if !strings.Contains(got, "status=success") {
		t.Errorf("Result.String() missing status: %q", got)
	}
	if !strings.Contains(got, "text_len=15") {
		t.Errorf("Result.String() missing text_len: %q", got)
	}
	if !strings.Contains(got, "changes=2") {
		t.Errorf("Result.String() missing changes count: %q", got)
	}
	if !strings.Contains(got, "duration=") {
		t.Errorf("Result.String() missing duration: %q", got)
	}
}

func TestStepTiming_IsZero(t *testing.T) {
	if !(lexnorm.StepTiming{}).IsZero() {
		t.Error("zero StepTiming must be zero")
	}
	st := lexnorm.StepTiming{Processor: "alias", Duration: time.Microsecond}
	if st.IsZero() {
		t.Error("populated StepTiming must NOT be zero")
	}
}

func TestStepTiming_HasError(t *testing.T) {
	var st lexnorm.StepTiming
	if st.HasError() {
		t.Error("zero StepTiming must not HasError")
	}
	st.Error = errors.New("x")
	if !st.HasError() {
		t.Error("StepTiming with Error must HasError")
	}
}

func TestStepTiming_Fields(t *testing.T) {
	st := lexnorm.StepTiming{
		Processor:        "alias",
		ProcessorVersion: "v2.0.0",
		Action:           lexnorm.ActionReplace,
		ChangeCount:      5,
		Duration:         100 * time.Microsecond,
		Error:            nil,
	}
	if st.Processor != "alias" {
		t.Error("Processor field mismatch")
	}
	if st.ProcessorVersion != "v2.0.0" {
		t.Error("ProcessorVersion field mismatch")
	}
	if st.Action != lexnorm.ActionReplace {
		t.Error("Action field mismatch")
	}
	if st.ChangeCount != 5 {
		t.Error("ChangeCount field mismatch")
	}
	if st.Duration != 100*time.Microsecond {
		t.Error("Duration field mismatch")
	}
}
