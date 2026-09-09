// Command fetch 抓取真实业务接口的 raw 响应并落盘到 data/*.raw.json。
//
// 数据源：
//   - admin-api/system/dept/list                    → data/dept.raw.json
//   - admin-api/qua/member-extended/page?selectAll   → data/member.raw.json
//
// 配置：
//   env API_BASE    默认 http://api.bdksim-pro.test.bedoke.com
//   env API_TOKEN   默认 <hardcoded test token>
//   env TENANT_ID   默认 1889501240003497986
//
// 运行：go run ./cmd/01-fetch
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	defaultAPIBase  = "http://api.bdksim-pro.test.bedoke.com"
	defaultToken    = "be3e0d79e55f4840a3605406bda0e32a"
	defaultTenantID = "1889501240003497986"
)

type endpoint struct {
	name    string
	path    string
	outFile string
}

func main() {
	apiBase := getenv("API_BASE", defaultAPIBase)
	token := getenv("API_TOKEN", defaultToken)
	tenantID := getenv("TENANT_ID", defaultTenantID)

	endpoints := []endpoint{
		{name: "dept", path: "/admin-api/system/dept/list", outFile: "dept.raw.json"},
		{name: "member", path: "/admin-api/qua/member-extended/page?&selectAll=true", outFile: "member.raw.json"},
	}

	// data/ 目录：相对于 cmd/01-fetch/main.go 的 ../../../data
	dataDir, err := resolveDataDir()
	if err != nil {
		log.Fatalf("resolve data dir: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("mkdir data dir: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}

	for _, ep := range endpoints {
		url := apiBase + ep.path
		body, err := fetch(client, url, token, tenantID)
		if err != nil {
			log.Fatalf("fetch %s: %v", ep.name, err)
		}

		// 解析校验
		var probe struct {
			Code int             `json:"code"`
			Msg  string          `json:"msg"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(body, &probe); err != nil {
			log.Fatalf("parse %s: %v", ep.name, err)
		}
		if probe.Code != 0 {
			log.Fatalf("%s returned code=%d msg=%q", ep.name, probe.Code, probe.Msg)
		}

		out := filepath.Join(dataDir, ep.outFile)
		if err := os.WriteFile(out, body, 0o644); err != nil {
			log.Fatalf("write %s: %v", out, err)
		}
		fmt.Printf("[OK] %s → %s (%d bytes, code=%d)\n", ep.name, out, len(body), probe.Code)
	}
}

func fetch(client *http.Client, url, token, tenantID string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("accept", "*/*")
	req.Header.Set("accept-language", "zh-CN,zh;q=0.9,en;q=0.8,ko;q=0.7")
	req.Header.Set("authorization", "Bearer "+token)
	req.Header.Set("dnt", "1")
	req.Header.Set("priority", "u=1, i")
	req.Header.Set("sec-ch-ua", `"Chromium";v="152", "Not?A_Brand";v="24", "Google Chrome";v="152"`)
	req.Header.Set("sec-ch-ua-mobile", "?1")
	req.Header.Set("sec-ch-ua-platform", `"iOS"`)
	req.Header.Set("sec-fetch-dest", "empty")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("sec-fetch-site", "same-site")
	req.Header.Set("tenant-id", tenantID)
	req.Header.Set("zone", "Asia/Shanghai")
	req.Header.Set("user-agent", "ark-lexnorm-example/0.1 (+fetch)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// resolveDataDir 返回 example/09-compare-engine-vs-llm/data 绝对路径。
//
// 兼容两种调用方式：
//   - go run ./cmd/01-fetch → cwd 是 example/09-compare-engine-vs-llm/
//   - go run ./example/09-compare-engine-vs-llm/cmd/01-fetch → cwd 是 lexnorm 根
func resolveDataDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	candidates := []string{
		filepath.Join(cwd, "data"),
		filepath.Join(cwd, "example", "09-compare-engine-vs-llm", "data"),
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c, nil
		}
	}
	// 兜底：直接返回 ./data（让后续 mkdir 创建）
	return candidates[0], nil
}