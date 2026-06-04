package biz

import "testing"

func TestValidatePassword(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{name: "strong", password: "Str0ng!Admin#2026"},
		{name: "too short", password: "Aa1!short", wantErr: true},
		{name: "missing uppercase", password: "lowercase1!password", wantErr: true},
		{name: "missing lowercase", password: "UPPERCASE1!PASSWORD", wantErr: true},
		{name: "missing digit", password: "NoDigits!Password", wantErr: true},
		{name: "missing symbol", password: "NoSymbols123Password", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidatePassword(tt.password); (err != nil) != tt.wantErr {
				t.Fatalf("ValidatePassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
