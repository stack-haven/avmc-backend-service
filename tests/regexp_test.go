package main

import (
	"regexp"
	"testing"
)

func Test_Birthday(t *testing.T) {
	var pattern = `^[1-9]{4}-(0[1-9]|1[0-2])-(0[1-9]|[1-2][0-9]|3[0-1])$`
	var str = "1991-08-22"
	t.Logf("\n\t 正则规则：%s \n\t 验证：%s", pattern, str)
	var _UpdateUserReq_Data_Birthday_Pattern = regexp.MustCompile(pattern)
	if !_UpdateUserReq_Data_Birthday_Pattern.MatchString(str) {
		t.Fatal("验证失败")
	}
	t.Log("验证成功")
}
