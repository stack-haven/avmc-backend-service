// Command evie-toolctl · main.go
//
// evie/tool 运维 CLI 子工具：
//   - validate-config: 对配置文件做启动期结构校验（复用 internal/conf.Validate）
//   - dump-vocab:      打印词库快照（占位，后续 phase 实现）
//   - version:         打印服务名 / 版本 / commit
//
// 设计目标：
//   - 零外部依赖（仅 Kratos config + 标准库）
//   - 子命令易扩展，flag 与 Kratos server 解耦
//   - 输出人类可读 + 退出码语义化
//
// 不做的事（避免越权）：
//   - 不写文件 / 不调用外部服务
//   - 不做迁移 / 不发请求
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"

	"backend-service/app/evie/tool/internal/conf"
)

// 编译期 ldflags 可注入
var (
	cliName    = "evie-toolctl"
	cliVersion = "0.1.0"
)

const usage = `evie-toolctl · evie/tool 运维 CLI

用法:
  evie-toolctl <subcommand> [flags]

子命令:
  validate-config   校验配置文件（结构 + 关键字段）
  dump-vocab        打印词库快照（占位）
  version           打印版本信息

通用 flag:
  -h, --help        显示帮助
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	sub := os.Args[1]
	args := os.Args[2:]

	switch sub {
	case "validate-config":
		os.Exit(runValidateConfig(args))
	case "dump-vocab":
		os.Exit(runDumpVocab(args))
	case "version", "--version", "-v":
		printVersion()
		os.Exit(0)
	case "-h", "--help", "help":
		fmt.Print(usage)
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "未知子命令: %q\n\n%s", sub, usage)
		os.Exit(2)
	}
}

// runValidateConfig 加载并校验配置文件。
// 退出码：0=OK，1=校验失败，2=IO/解析失败。
func runValidateConfig(args []string) int {
	fs := flag.NewFlagSet("validate-config", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	filePath := fs.String("f", "configs/config.yaml", "配置文件路径（YAML，支持 -f config.demo.yaml）")
	quiet := fs.Bool("q", false, "校验通过时不输出 OK")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	cfg, err := loadConfig(*filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ 加载配置失败 %s: %v\n", *filePath, err)
		return 2
	}

	if err := conf.Validate(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "✗ 配置校验失败 %s\n", *filePath)
		printValidationError(err)
		return 1
	}

	if !*quiet {
		fmt.Printf("✓ 配置校验通过: %s\n", *filePath)
		fmt.Printf("  · server.http.addr = %s\n", cfg.Server.Http.Addr)
		fmt.Printf("  · server.grpc.addr = %s\n", cfg.Server.Grpc.Addr)
		if cfg.Asr != nil && cfg.Asr.Providers != nil {
			enabled := enabledProviders(cfg.Asr.Providers)
			fmt.Printf("  · asr providers    = %s\n", strings.Join(enabled, ","))
		}
		if cfg.Enhancement != nil && cfg.Enhancement.Pipeline != nil {
			fmt.Printf("  · pipeline.steps   = %d\n", len(cfg.Enhancement.Pipeline))
		}
	}
	return 0
}

// runDumpVocab 占位：v0.x 仅打印提示。完整实现见 EXECUTION_PLAN 6.x。
func runDumpVocab(args []string) int {
	_ = args
	fmt.Fprintln(os.Stderr, "✗ dump-vocab 尚未实现；将在 Phase 6 补齐（见 .agents/EXECUTION_PLAN.md）")
	return 1
}

func printVersion() {
	fmt.Printf("%s %s\n", cliName, cliVersion)
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" || s.Key == "vcs.time" {
				fmt.Printf("  %s = %s\n", s.Key, s.Value)
			}
		}
	}
}

// loadConfig 用 Kratos config file source 加载 YAML 配置。
func loadConfig(path string) (*conf.Bootstrap, error) {
	c := config.New(config.WithSource(file.NewSource(path)))
	defer c.Close()
	if err := c.Load(); err != nil {
		return nil, err
	}
	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		return nil, err
	}
	return &bc, nil
}

// printValidationError 把 conf.Validate 的错误拆成多行（errs slice 用换行拼接）。
func printValidationError(err error) {
	// conf.Validate 返回的错误信息以 "; " 拼接（按现有实现）。
	parts := strings.Split(err.Error(), "; ")
	sort.Strings(parts) // 确定性输出
	for _, p := range parts {
		fmt.Fprintf(os.Stderr, "  · %s\n", p)
	}
}

// enabledProviders 提取已启用的 provider 列表（确定性）。
func enabledProviders(p *conf.Asr_Providers) []string {
	out := []string{}
	if p.Funasr.GetEnabled() {
		out = append(out, "funasr")
	}
	if p.Xunfei.GetEnabled() {
		out = append(out, "xunfei")
	}
	return out
}

// 确保 import 被使用（防止以后 import 删除导致 build break）
var _ = context.Background
