// Package entc 是本仓 Ent 代码生成的唯一入口。
//
// 背景（2026-11-12，B3）：每个服务原本各有一份 entc.go，内容除 Package 路径与
// Feature 列表外完全相同（4 份 / 46 行）。这类副本的漂移不会有任何门禁发现，
// 例如 ai 的 gen 目录里曾长期残留与自身 entc.go 矛盾的 privacy/entql 产物。
//
// 上收后服务侧只剩一个自动推导的 Package 与可选的 Feature 覆盖，因此：
//   - 生成入口只有一个，升级 ent 或调整特性只需改这里；
//   - Package 从 go.mod 推导，不再手写导入路径，避免改名漏改；
//   - Feature 集合集中定义，杜绝某服务悄悄少了 Privacy/EntQL。
package entc

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
	"github.com/go-kratos/kratos/v2/log"
)

// Options 描述一次生成。零值可用：Schema/Target 走默认，Package 自动推导，
// Features 取 DefaultFeatures。
type Options struct {
	// Schema 是 schema 目录，相对当前工作目录。默认 "./schema"。
	Schema string
	// Target 是生成物输出目录，相对当前工作目录。默认 "./gen"。
	Target string
	// Package 是生成物的 Go 导入路径。留空时按 go.mod 自动推导为
	// <module>/<Target 相对仓库根目录>。
	Package string
	// Features 覆盖 Ent 特性集合。留空时使用 DefaultFeatures。
	Features []gen.Feature
	// BuildTags 仅在显式调用 Generate 时使用，用于引导生成；普通服务入口无需设置。
	BuildTags []string
}

// DefaultFeatures 是本仓所有服务的标准 Ent 特性集合。
//
// Privacy 与 EntQL 是租户隐私策略的硬依赖：rule 包通过
// query.Filter().WhereTenantID(entql.Uint32EQ(...)) 施加隔离谓词，
// 缺少它们会导致租户隔离静默失效或编译失败。
func DefaultFeatures() []gen.Feature {
	return []gen.Feature{
		gen.FeatureUpsert,
		gen.FeatureModifier,
		gen.FeatureExecQuery,
		gen.FeatureIntercept,
		gen.FeatureEntQL,
		gen.FeatureLock,
		gen.FeaturePrivacy,
	}
}

// Generate 执行生成并返回错误。
//
// 传指针而不是值：Options 已增长到 96 字节（超过 gocritic hugeParam 阈值），
// 按值传递每次生成都多一次完整拷贝；同时 nil 是合法输入（等价于零值配置），
// 便于测试与 `Generate(nil)` 这类"全默认"调用。
func Generate(opts *Options) error {
	if opts == nil {
		opts = &Options{}
	}
	schemaDir := opts.Schema
	if schemaDir == "" {
		schemaDir = "./schema"
	}
	target := opts.Target
	if target == "" {
		target = "./gen"
	}
	pkg := opts.Package
	if pkg == "" {
		derived, err := derivePackage(target)
		if err != nil {
			return err
		}
		pkg = derived
	}
	// nil 表示"用默认特性集"；非 nil 的空切片表示"显式不要任何特性"
	// （version 服务沿用 Ent 默认布局且不启用任何 Feature，需要这个区分）。
	features := opts.Features
	if features == nil {
		features = DefaultFeatures()
	}

	// 引导生成：目标目录不存在时，schema 闭包里的部分文件（软删除钩子、租户策略）
	// 会引用尚不存在的生成物，导致 schema 包无法编译、生成无法启动。
	// 这些文件都带 `!ent_bootstrap` 构建标签，先用该标签生成一遍即可打破先有鸡
	// 还是先有蛋的问题；随后立刻用真实标签再生成一次，产出最终内容。
	if _, statErr := os.Stat(target); os.IsNotExist(statErr) {
		fmt.Fprintf(os.Stderr, "==> %s 不存在，先执行引导生成（-tags ent_bootstrap）\n", target)
		if err := runPass(schemaDir, target, pkg, features, []string{"ent_bootstrap"}, false, true); err != nil {
			return err
		}
	}
	return runPass(schemaDir, target, pkg, features, opts.BuildTags, true, false)
}

// runPass 执行一次 ent 生成。
//
// assert 为 false 时跳过租户策略断言：引导阶段策略文件被标签排除，必然不一致。
// placeholder 为 true 时即使在引导阶段也要写出（空的）租户派发表，否则真实 schema
// 在下一轮无法编译。
func runPass(schemaDir, target, pkg string, features []gen.Feature, tags []string, assert, placeholder bool) error {
	cfg := &gen.Config{
		Target:   target,
		Package:  pkg,
		Features: features,
		// 包装生成器：等 ent 写完自身产物后，再从同一张 schema 图推导并写出
		// 租户隔离派发表。放在 hook 里可以复用已加载的图，无需二次编译 schema。
		Hooks: []gen.Hook{func(next gen.Generator) gen.Generator {
			return gen.GenerateFunc(func(g *gen.Graph) error {
				// 先断言，后落盘：断言失败时不产生任何生成物，避免留下半成品。
				if assert {
					if err := assertTenantScope(g); err != nil {
						return err
					}
				}
				if err := next.Generate(g); err != nil {
					return err
				}
				return writeTenantFilter(target, g, placeholder)
			})
		}},
	}
	if len(tags) > 0 {
		cfg.BuildFlags = []string{"-tags", strings.Join(tags, ",")}
	}
	if err := entc.Generate(schemaDir, cfg); err != nil {
		return fmt.Errorf("running ent codegen in %s: %w", target, err)
	}
	return nil
}

// MustGenerate 执行生成，失败即终止进程（供代码生成入口使用）。
//
// 生成器没有上层可以返回错误，因此失败必须让进程非零退出，否则会出现
// "生成失败但过程静默通过"——那正是本仓要消除的一类缺陷。
func MustGenerate(opts *Options) {
	if err := Generate(opts); err != nil {
		log.Fatalf("ent codegen: %v", err)
	}
}

var moduleLine = regexp.MustCompile(`(?m)^module\s+(\S+)`)

// derivePackage 依据 go.mod 推导 target 目录的 Go 导入路径。
//
// 之所以推导而不是手写：包路径一旦与服务目录名脱节，生成物就会落在错误位置，
// 而这类错误只会在编译期以难以定位的形式暴露。
func derivePackage(target string) (string, error) {
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolving target %q: %w", target, err)
	}
	root, err := findModuleRoot(abs)
	if err != nil {
		return "", err
	}
	module, err := readModulePath(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", fmt.Errorf("resolving %q relative to %q: %w", abs, root, err)
	}
	if rel == "." {
		return module, nil
	}
	return module + "/" + filepath.ToSlash(rel), nil
}

func findModuleRoot(dir string) (string, error) {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %q", dir)
		}
		dir = parent
	}
}

func readModulePath(gomod string) (string, error) {
	content, err := os.ReadFile(gomod)
	if err != nil {
		return "", fmt.Errorf("reading %q: %w", gomod, err)
	}
	match := moduleLine.FindSubmatch(content)
	if match == nil {
		return "", fmt.Errorf("module directive not found in %q", gomod)
	}
	return strings.TrimSpace(string(match[1])), nil
}
