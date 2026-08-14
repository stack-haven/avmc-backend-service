package aliyun

import (
	"testing"
)

func TestPercentEncode(t *testing.T) {
	cases := map[string]string{
		"abc":     "abc",
		"a b":     "a%20b",
		"a*b":     "a%2Ab",
		"a~b":     "a~b",
		"中文":     "%E4%B8%AD%E6%96%87",
		"a+b":     "a%2Bb",
		"a/b?c=d": "a%2Fb%3Fc%3Dd",
	}
	for input, want := range cases {
		if got := percentEncode(input); got != want {
			t.Errorf("percentEncode(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSignDeterministicAndNonEmpty(t *testing.T) {
	params := map[string]string{
		"Action":      "SendSms",
		"Version":     "2017-05-25",
		"PhoneNumbers": "13800138000",
		"SignName":    "测试",
		"AccessKeyId": "ak",
	}
	secret := "secret"
	sig1 := sign(secret, params)
	sig2 := sign(secret, params)
	if sig1 == "" {
		t.Fatal("signature is empty")
	}
	if sig1 != sig2 {
		t.Fatal("signature is not deterministic")
	}
	// 修改参数应改变签名
	params["SignName"] = "其他"
	if sign(secret, params) == sig1 {
		t.Fatal("signature did not change when params changed")
	}
}

func TestTemplateParam(t *testing.T) {
	if got := templateParam(nil); got != "{}" {
		t.Errorf("templateParam(nil) = %q, want {}", got)
	}
	got := templateParam(map[string]string{"code": "1234"})
	if got != `{"code":"1234"}` {
		t.Errorf("templateParam = %q", got)
	}
}

func TestNewRejectsMissingCredentials(t *testing.T) {
	if _, err := New(Config{AccessKeyID: "", AccessKeySecret: "sk"}); err == nil {
		t.Fatal("New() with empty access_key_id = nil, want error")
	}
	if _, err := New(Config{AccessKeyID: "ak", AccessKeySecret: ""}); err == nil {
		t.Fatal("New() with empty access_key_secret = nil, want error")
	}
}
