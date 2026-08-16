package biz

import "testing"

func TestRedactJSON(t *testing.T) {
	got := redactJSON(`{"code":"order_status","username":"admin","password":"123456","nested":{"access_token":"token","sms_code":"123456"}}`)
	want := `{"code":"order_status","nested":{"access_token":"***","sms_code":"***"},"password":"***","username":"admin"}`
	if got != want {
		t.Fatalf("redactJSON() = %s, want %s", got, want)
	}
	if got := redactJSON("not-json"); got != "[unavailable]" {
		t.Fatalf("invalid payload must not be persisted: %s", got)
	}
}
