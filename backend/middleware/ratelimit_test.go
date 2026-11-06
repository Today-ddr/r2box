package middleware

import (
	"net/http/httptest"
	"testing"
)

func TestGetClientIPWithoutTrustedProxy(t *testing.T) {
	if err := InitTrustedProxies(nil); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.9:1234"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")

	if got := GetClientIP(req); got != "203.0.113.9" {
		t.Errorf("未配置受信代理时应忽略 XFF，got %q", got)
	}
}

func TestGetClientIPWithTrustedProxy(t *testing.T) {
	if err := InitTrustedProxies([]string{"127.0.0.1/32"}); err != nil {
		t.Fatal(err)
	}
	defer InitTrustedProxies(nil)

	cases := []struct {
		name   string
		remote string
		xff    string
		want   string
	}{
		{"无 XFF", "127.0.0.1:1234", "", "127.0.0.1"},
		{"单跳", "127.0.0.1:1234", "203.0.113.9", "203.0.113.9"},
		{"多跳取最右非受信", "127.0.0.1:1234", "1.1.1.1, 203.0.113.9", "203.0.113.9"},
		{"伪造前缀无效", "127.0.0.1:1234", "9.9.9.9, 203.0.113.9", "203.0.113.9"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = c.remote
			if c.xff != "" {
				req.Header.Set("X-Forwarded-For", c.xff)
			}
			if got := GetClientIP(req); got != c.want {
				t.Errorf("GetClientIP = %q, want %q", got, c.want)
			}
		})
	}
}

func TestInitTrustedProxiesInvalid(t *testing.T) {
	if err := InitTrustedProxies([]string{"not-a-cidr"}); err == nil {
		t.Error("非法网段应返回错误")
	}
	InitTrustedProxies(nil)
}
