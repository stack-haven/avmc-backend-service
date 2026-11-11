// Package runtimeconfig 提供各服务共用的配置装载机制：读文件 → 应用环境变量覆盖
// → 由调用方执行本服务的校验策略。
//
// 设计边界（重要）：
//   - 本包只承载**机制**：文件加载、环境变量覆盖、生产环境判定、可复用的校验原语。
//   - **校验策略留在各服务**。平台/evie/ai/version 的必填项集合、中间件加固要求
//     并不相同（例如 version 冻结中、只校验最少的监听与数据源；ai 额外要求 SSE 配置），
//     把这些差异硬塞进一个带开关的 Profile 只会把"策略"伪装成"配置"。
//   - 因此本包不导出统一的 Validate：服务用下面的原语组合出自己的 Validate。
//
// 历史背景：这套逻辑此前在 5 个服务里各有一份副本（147–641 行不等，5 个不同 hash），
// 其中 edai 缺失 production 守卫。机制部分现已收敛，策略部分显式保留在服务内。
package runtimeconfig

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"backend-service/pkg/conf"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Profile 描述一个服务读取环境变量的约定。
//
// 这些差异是真实存在的，不是配置化洁癖：每个服务的环境变量前缀、
// JWT 密钥变量名、以及"哪些变量代表生产环境"都不一样。
type Profile struct {
	// EnvPrefix 是服务自有环境变量前缀，例如 "platform_ADMIN"、"EVIE"。
	// 用于拼出 <EnvPrefix>_HTTP_ADDR、<EnvPrefix>_DB_SOURCE 等。
	EnvPrefix string

	// JWTKeyEnv 是 JWT 签名密钥的环境变量名。
	// 为空时使用全仓默认的 "ARK_JWT_KEY"。
	JWTKeyEnv string

	// ProductionEnvs 是"值为 prod/production 即视为生产环境"的环境变量名列表。
	// 顺序敏感（保持与服务原有行为一致），为空时回退到
	// {EnvPrefix + "_ENV", "DEPLOY_ENV", "APP_ENV"}。
	ProductionEnvs []string
}

// jwtKeyEnv 返回实际使用的 JWT 密钥环境变量名。
func (p Profile) jwtKeyEnv() string {
	if p.JWTKeyEnv != "" {
		return p.JWTKeyEnv
	}
	return "ARK_JWT_KEY"
}

// productionEnvs 返回实际使用的生产环境判定变量名。
func (p Profile) productionEnvs() []string {
	if len(p.ProductionEnvs) > 0 {
		return p.ProductionEnvs
	}
	return []string{p.EnvPrefix + "_ENV", "DEPLOY_ENV", "APP_ENV"}
}

// Load 读取配置文件并解码为 Bootstrap。**不应用环境变量覆盖、不做校验** ——
// 这两步由调用方显式执行，以保证各服务能保持自己的顺序与策略。
func Load(path string) (*conf.Bootstrap, error) {
	c := config.New(
		config.WithSource(
			file.NewSource(path),
		),
	)
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

// ApplyEnvOverrides 应用所有服务共有的环境变量覆盖：HTTP/gRPC 监听地址与数据源。
//
// 只覆盖"部署时必须可注入"的字段；其余配置一律走配置文件，
// 避免配置来源过度分散。
func ApplyEnvOverrides(bc *conf.Bootstrap, p Profile) {
	if bc == nil {
		return
	}
	if bc.Server != nil {
		if bc.Server.Http != nil {
			if v := strings.TrimSpace(os.Getenv(p.EnvPrefix + "_HTTP_ADDR")); v != "" {
				bc.Server.Http.Addr = v
			}
		}
		if bc.Server.Grpc != nil {
			if v := strings.TrimSpace(os.Getenv(p.EnvPrefix + "_GRPC_ADDR")); v != "" {
				bc.Server.Grpc.Addr = v
			}
		}
	}
	if bc.Data != nil {
		applyDataEnv(bc.Data, p)
	}
}

// ApplyServerEnvOverrides 应用与 HTTP 中间件相关的可选覆盖：
// Swagger 开关、CORS 白名单、JWT 签名密钥。
//
// 与 ApplyEnvOverrides 拆开是为了让不涉及这些配置的服务（如 version）
// 保持精确的覆盖范围，而不是被动接受一批它从未读取过的环境变量。
func ApplyServerEnvOverrides(bc *conf.Bootstrap, p Profile) {
	if bc == nil || bc.Server == nil || bc.Server.Http == nil {
		return
	}
	httpCfg := bc.Server.Http
	if v, ok := EnvBool(p.EnvPrefix + "_ENABLE_SWAGGER"); ok {
		httpCfg.EnableSwagger = v
	}
	if v := SplitCSV(os.Getenv(p.EnvPrefix + "_CORS_ORIGINS")); len(v) > 0 {
		if httpCfg.Cors == nil {
			httpCfg.Cors = &conf.Server_HTTP_CORS{}
		}
		httpCfg.Cors.Origins = v
	}
	if httpCfg.Middleware != nil && httpCfg.Middleware.Auth != nil {
		if v := strings.TrimSpace(os.Getenv(p.jwtKeyEnv())); v != "" {
			httpCfg.Middleware.Auth.Key = v
		}
	}
}

// ApplySSEEnvOverrides 应用 SSE 监听配置覆盖（仅持有 SSE 服务的服务需要使用）。
func ApplySSEEnvOverrides(bc *conf.Bootstrap, p Profile) {
	if bc == nil || bc.Server == nil || bc.Server.Sse == nil {
		return
	}
	if v := strings.TrimSpace(os.Getenv(p.EnvPrefix + "_SSE_ADDR")); v != "" {
		bc.Server.Sse.Addr = v
	}
	if v := strings.TrimSpace(os.Getenv(p.EnvPrefix + "_SSE_PATH")); v != "" {
		bc.Server.Sse.Path = v
	}
}

// IsProduction 判断当前进程是否运行在生产环境（大小写与空白不敏感）。
func IsProduction(p Profile) bool {
	for _, key := range p.productionEnvs() {
		switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
		case "prod", "production":
			return true
		}
	}
	return false
}

// PositiveDuration 校验一个时长期望大于零。name 用于错误信息。
func PositiveDuration(name string, value *durationpb.Duration) error {
	if value == nil || value.AsDuration() <= 0 {
		return ConfigError(fmt.Sprintf("%s must be greater than zero", name))
	}
	return nil
}

// Limiter 校验限流器配置：只允许 bbr 或关闭。
func Limiter(name string, limiter *conf.Middleware_RateLimiter) error {
	if limiter == nil {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(limiter.Name)) {
	case "", "off", "none", "disabled", "bbr":
		return nil
	default:
		return ConfigError(fmt.Sprintf("%s supports only bbr or disabled", name))
	}
}

// UnsafeProductionDatabaseSource 判断数据源在生产环境是否不可接受
// （root 凭据或占位符）。
func UnsafeProductionDatabaseSource(source string) bool {
	source = strings.ToLower(strings.TrimSpace(source))
	return strings.HasPrefix(source, "root:") ||
		strings.HasPrefix(source, "root@") ||
		strings.Contains(source, "replace-with")
}

// SplitCSV 按逗号切分并去掉空白项与空项。
func SplitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if v := strings.TrimSpace(part); v != "" {
			values = append(values, v)
		}
	}
	return values
}

// EnvBool 读取布尔型环境变量；未设置或无法解析时返回 ok=false。
func EnvBool(key string) (bool, bool) {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return false, false
	}
	v, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false, false
	}
	return v, true
}

// ConfigError 表示配置缺失或不合法。
//
// 用命名类型而非 fmt.Errorf，是为了让错误信息在日志里保持单行、可直接比对
// （各服务的配置测试依赖这一点）。
type ConfigError string

func (e ConfigError) Error() string {
	return string(e)
}

// applyDataEnv 应用数据源相关覆盖。
func applyDataEnv(data *conf.Data, p Profile) {
	if data.Database != nil {
		if v := strings.TrimSpace(os.Getenv(p.EnvPrefix + "_DB_DRIVER")); v != "" {
			data.Database.Driver = v
		}
		if v := strings.TrimSpace(os.Getenv(p.EnvPrefix + "_DB_SOURCE")); v != "" {
			data.Database.Source = v
		}
		if v, ok := EnvBool(p.EnvPrefix + "_DB_MIGRATE"); ok {
			data.Database.Migrate = v
		}
		if v, ok := EnvBool(p.EnvPrefix + "_DB_DEBUG"); ok {
			data.Database.Debug = v
		}
	}
	if data.Redis != nil {
		if v := strings.TrimSpace(os.Getenv(p.EnvPrefix + "_REDIS_ADDR")); v != "" {
			data.Redis.Addr = v
		}
		if v, ok := os.LookupEnv(p.EnvPrefix + "_REDIS_PASSWORD"); ok {
			data.Redis.Password = v
		}
	}
}
