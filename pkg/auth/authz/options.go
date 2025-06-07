package authz

// EngineType 授权引擎类型
type EngineType string

// 授权引擎类型常量
const (
	// EngineCasbin Casbin授权引擎
	EngineCasbin EngineType = "casbin"
	// EngineOPA OPA授权引擎
	EngineOPA EngineType = "opa"
	// EngineZanzibar Zanzibar授权引擎
	EngineZanzibar EngineType = "zanzibar"
)

// ModelFormat 模型格式类型
type ModelFormat string

// 模型格式常量
const (
	// ModelFormatText 文本格式
	ModelFormatText ModelFormat = "text"
	// ModelFormatFile 文件格式
	ModelFormatFile ModelFormat = "file"
)

// AdapterType 适配器类型
type AdapterType string

// 适配器类型常量
const (
	// AdapterFile 文件适配器
	AdapterFile AdapterType = "file"
	// AdapterMySQL MySQL适配器
	AdapterMySQL AdapterType = "mysql"
	// AdapterPostgres PostgreSQL适配器
	AdapterPostgres AdapterType = "postgres"
	// AdapterMongoDB MongoDB适配器
	AdapterMongoDB AdapterType = "mongodb"
	// AdapterRedis Redis适配器
	AdapterRedis AdapterType = "redis"
	// AdapterMemory 内存适配器
	AdapterMemory AdapterType = "memory"
)

// Mode 授权模式类型
type Mode string

// 授权模式常量
const (
	// ModeLocal 本地模式
	ModeLocal Mode = "local"
	// ModeRemote 远程模式
	ModeRemote Mode = "remote"
)

// Options 授权器配置选项
type Options struct {
	// EngineType 授权引擎类型
	EngineType EngineType
	// ModelFormat 模型格式
	ModelFormat ModelFormat
	// ModelText 模型文本
	ModelText string
	// ModelFile 模型文件路径
	ModelFile string
	// AdapterType 适配器类型
	AdapterType AdapterType
	// AdapterDSN 适配器数据源名称
	AdapterDSN string
	// AutoSave 是否自动保存策略
	AutoSave bool
	// AutoNotifyWatcher 是否自动通知观察者
	AutoNotifyWatcher bool
	// EnableLog 是否启用日志
	EnableLog bool
	// EnableWatcher 是否启用观察者
	EnableWatcher bool
	// WatcherType 观察者类型
	WatcherType string
	// WatcherOptions 观察者选项
	WatcherOptions map[string]interface{}
	// EnableCache 是否启用缓存
	EnableCache bool
	// CacheType 缓存类型
	CacheType string
	// CacheOptions 缓存选项
	CacheOptions map[string]interface{}
	// EnableRBAC 是否启用RBAC
	EnableRBAC bool
	// EnableABAC 是否启用ABAC
	EnableABAC bool
	// EnableREBAC 是否启用ReBAC
	EnableREBAC bool
	// ProviderOptions 提供者特定选项
	ProviderOptions interface{}
	// Mode 授权模式
	Mode Mode
	// RemoteURL 远程服务URL
	RemoteURL string
}

// Option 选项函数类型
type Option func(*Options)

// DefaultOptions 默认选项
func DefaultOptions() *Options {
	return &Options{
		EngineType:        EngineCasbin,
		ModelFormat:       ModelFormatText,
		AdapterType:       AdapterMemory,
		AutoSave:          true,
		AutoNotifyWatcher: true,
		EnableLog:         false,
		EnableWatcher:     false,
		EnableCache:       false,
		EnableRBAC:        true,
		EnableABAC:        false,
		EnableREBAC:       false,
		ProviderOptions:   make(map[string]interface{}),
		WatcherOptions:    make(map[string]interface{}),
		CacheOptions:      make(map[string]interface{}),
		Mode:              ModeLocal,
	}
}

// WithEngineType 设置授权引擎类型
func WithEngineType(engineType EngineType) Option {
	return func(o *Options) {
		o.EngineType = engineType
	}
}

// WithModelFormat 设置模型格式
func WithModelFormat(modelFormat ModelFormat) Option {
	return func(o *Options) {
		o.ModelFormat = modelFormat
	}
}

// WithModelText 设置模型文本
func WithModelText(modelText string) Option {
	return func(o *Options) {
		o.ModelFormat = ModelFormatText
		o.ModelText = modelText
	}
}

// WithModelFile 设置模型文件路径
func WithModelFile(modelFile string) Option {
	return func(o *Options) {
		o.ModelFormat = ModelFormatFile
		o.ModelFile = modelFile
	}
}

// WithAdapterType 设置适配器类型
func WithAdapterType(adapterType AdapterType) Option {
	return func(o *Options) {
		o.AdapterType = adapterType
	}
}

// WithAdapterDSN 设置适配器数据源名称
func WithAdapterDSN(adapterDSN string) Option {
	return func(o *Options) {
		o.AdapterDSN = adapterDSN
	}
}

// WithAutoSave 设置是否自动保存策略
func WithAutoSave(autoSave bool) Option {
	return func(o *Options) {
		o.AutoSave = autoSave
	}
}

// WithAutoNotifyWatcher 设置是否自动通知观察者
func WithAutoNotifyWatcher(autoNotifyWatcher bool) Option {
	return func(o *Options) {
		o.AutoNotifyWatcher = autoNotifyWatcher
	}
}

// WithEnableLog 设置是否启用日志
func WithEnableLog(enableLog bool) Option {
	return func(o *Options) {
		o.EnableLog = enableLog
	}
}

// WithEnableWatcher 设置是否启用观察者
func WithEnableWatcher(enableWatcher bool) Option {
	return func(o *Options) {
		o.EnableWatcher = enableWatcher
	}
}

// WithWatcherType 设置观察者类型
func WithWatcherType(watcherType string) Option {
	return func(o *Options) {
		o.WatcherType = watcherType
	}
}

// WithWatcherOption 设置观察者选项
func WithWatcherOption(key string, value interface{}) Option {
	return func(o *Options) {
		if o.WatcherOptions == nil {
			o.WatcherOptions = make(map[string]interface{})
		}
		o.WatcherOptions[key] = value
	}
}

// WithEnableCache 设置是否启用缓存
func WithEnableCache(enableCache bool) Option {
	return func(o *Options) {
		o.EnableCache = enableCache
	}
}

// WithCacheType 设置缓存类型
func WithCacheType(cacheType string) Option {
	return func(o *Options) {
		o.CacheType = cacheType
	}
}

// WithCacheOption 设置缓存选项
func WithCacheOption(key string, value interface{}) Option {
	return func(o *Options) {
		if o.CacheOptions == nil {
			o.CacheOptions = make(map[string]interface{})
		}
		o.CacheOptions[key] = value
	}
}

// WithEnableRBAC 设置是否启用RBAC
func WithEnableRBAC(enableRBAC bool) Option {
	return func(o *Options) {
		o.EnableRBAC = enableRBAC
	}
}

// WithEnableABAC 设置是否启用ABAC
func WithEnableABAC(enableABAC bool) Option {
	return func(o *Options) {
		o.EnableABAC = enableABAC
	}
}

// WithEnableREBAC 设置是否启用ReBAC
func WithEnableREBAC(enableREBAC bool) Option {
	return func(o *Options) {
		o.EnableREBAC = enableREBAC
	}
}

// WithMode 设置授权模式
func WithMode(mode Mode) Option {
	return func(o *Options) {
		o.Mode = mode
	}
}

// WithRemoteURL 设置远程服务URL
func WithRemoteURL(remoteURL string) Option {
	return func(o *Options) {
		o.RemoteURL = remoteURL
	}
}

// WithProviderOption 设置提供者特定选项
func WithProviderOption(key string, value interface{}) Option {
	return func(o *Options) {
		if o.ProviderOptions == nil {
			o.ProviderOptions = make(map[string]interface{})
		}
		o.ProviderOptions.(map[string]interface{})[key] = value
	}
}
