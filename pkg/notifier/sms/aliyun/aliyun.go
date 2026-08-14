package aliyun

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"backend-service/pkg/notifier"
)

func init() {
	// 注册阿里云短信渠道。configJSON 为 Config 结构。
	notifier.Register("aliyun-sms", func(raw json.RawMessage) (notifier.Sender, error) {
		var cfg Config
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return nil, err
		}
		return New(cfg)
	})
}

// Config 阿里云短信配置。
type Config struct {
	Endpoint        string `json:"endpoint"`
	AccessKeyID     string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
	SignName        string `json:"sign_name"`
	TemplateCode    string `json:"template_code"`
}

type client struct {
	cfg    Config
	client *http.Client
}

// New 创建阿里云短信发送器。
func New(cfg Config) (notifier.Sender, error) {
	if cfg.Endpoint == "" {
		cfg.Endpoint = "dysmsapi.aliyuncs.com"
	}
	if cfg.AccessKeyID == "" || cfg.AccessKeySecret == "" {
		return nil, fmt.Errorf("%w: access_key_id and access_key_secret required", notifier.ErrInvalidConfig)
	}
	return &client{cfg: cfg, client: &http.Client{Timeout: 10 * time.Second}}, nil
}

func (c *client) Channel() string { return "sms" }

func (c *client) Send(ctx context.Context, msg notifier.Message) (notifier.Result, error) {
	result := notifier.Result{}
	var lastErr error
	for _, recipient := range msg.Recipients {
		if strings.TrimSpace(recipient.Phone) == "" {
			result.FailCount++
			continue
		}
		if err := c.sendOne(ctx, recipient.Phone, msg); err != nil {
			result.FailCount++
			lastErr = err
			continue
		}
		result.SuccessCount++
	}
	if result.SuccessCount == 0 {
		if result.FailCount > 0 && lastErr != nil {
			return result, fmt.Errorf("sms: all recipients failed: %w", lastErr)
		}
		return result, notifier.ErrRecipientRequired
	}
	return result, nil
}

func (c *client) sendOne(ctx context.Context, phone string, msg notifier.Message) error {
	params := map[string]string{
		"Action":         "SendSms",
		"Version":        "2017-05-25",
		"Format":         "JSON",
		"RegionId":       "cn-hangzhou",
		"PhoneNumbers":   phone,
		"SignName":       c.cfg.SignName,
		"TemplateCode":   c.cfg.TemplateCode,
		"TemplateParam":  templateParam(msg.Variables),
		"AccessKeyId":    c.cfg.AccessKeyID,
		"SignatureMethod": "HMAC-SHA1",
		"SignatureVersion": "1.0",
		"SignatureNonce":  nonce(),
		"Timestamp":       time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}
	params["Signature"] = sign(c.cfg.AccessKeySecret, params)

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("https://%s/", c.cfg.Endpoint), strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("sms: send: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var aliyunResp struct {
		Code    string `json:"Code"`
		Message string `json:"Message"`
	}
	_ = json.Unmarshal(body, &aliyunResp)
	if resp.StatusCode != http.StatusOK || aliyunResp.Code != "OK" {
		return fmt.Errorf("sms: send failed: code=%s message=%s", aliyunResp.Code, aliyunResp.Message)
	}
	return nil
}

// templateParam 把变量渲染成阿里云短信模板参数 JSON（如 {"code":"1234"}）。
func templateParam(variables map[string]string) string {
	if len(variables) == 0 {
		return "{}"
	}
	b, err := json.Marshal(variables)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// nonce 生成签名随机数。
func nonce() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// sign 计算阿里云 RPC 签名（HMAC-SHA1，V1.0）。
func sign(secret string, params map[string]string) string {
	// 1. 参数按 key 字典序排序，构造 canonicalized query string
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "Signature" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, percentEncode(k)+"="+percentEncode(params[k]))
	}
	canonical := strings.Join(parts, "&")

	// 2. StringToSign = HTTPMethod & %2F & canonicalized
	stringToSign := "POST&%2F&" + percentEncode(canonical)

	// 3. Signature = base64(HMAC-SHA1(secret&, stringToSign))
	mac := hmac.New(sha1.New, []byte(secret+"&"))
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// percentEncode 阿里云 RPC 特殊编码：空格→%20、*→%2A、%7E→~
func percentEncode(value string) string {
	encoded := url.QueryEscape(value)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "*", "%2A")
	encoded = strings.ReplaceAll(encoded, "%7E", "~")
	return encoded
}
