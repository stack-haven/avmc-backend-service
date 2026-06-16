module backend-service

go 1.24.6

require (
	buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.36.5-20250307204501-0409229c3780.1
	entgo.io/ent v0.14.5
	github.com/BurntSushi/toml v1.6.0
	github.com/casbin/casbin/v2 v2.135.0
	github.com/envoyproxy/protoc-gen-validate v1.2.1
	github.com/glebarez/go-sqlite v1.20.3
	github.com/go-kratos/kratos/v2 v2.9.2
	github.com/go-sql-driver/mysql v1.9.3
	github.com/google/gnostic v0.7.1
	github.com/google/wire v0.7.0
	github.com/iancoleman/strcase v0.3.0
	github.com/nicksnyder/go-i18n/v2 v2.6.0
	github.com/stretchr/testify v1.11.0
	go.einride.tech/aip v0.80.0
	go.uber.org/automaxprocs v1.6.0
	golang.org/x/crypto v0.47.0
	golang.org/x/oauth2 v0.30.0
	google.golang.org/genproto/googleapis/api v0.0.0-20251213004720-97cd9d5aeac2
	google.golang.org/grpc v1.75.0
	google.golang.org/protobuf v1.36.10
)

require (
	cel.dev/expr v0.24.0 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/bmatcuk/doublestar/v4 v4.8.1 // indirect
	github.com/bufbuild/protovalidate-go v0.9.2 // indirect
	github.com/casbin/govaluate v1.4.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/glebarez/sqlite v1.7.0 // indirect
	github.com/go-jose/go-jose/v4 v4.1.1 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect
	github.com/golang-sql/sqlexp v0.1.0 // indirect
	github.com/google/cel-go v0.24.1 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.7.2 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/lufia/plan9stats v0.0.0-20230326075908-cb1d2100619a // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/microsoft/go-mssqldb v1.6.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20221212215047-62379fc7944b // indirect
	github.com/redis/go-redis/extra/rediscmd/v9 v9.8.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230126093431-47fa9a501578 // indirect
	github.com/shirou/gopsutil/v3 v3.23.6 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/stoewer/go-strcase v1.3.1 // indirect
	github.com/tklauser/go-sysconf v0.3.11 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/yusufpapurcu/wmi v1.2.3 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/exp v0.0.0-20250305212735-054e65f0b394 // indirect
	golang.org/x/tools v0.40.0 // indirect
	gorm.io/driver/mysql v1.5.7 // indirect
	gorm.io/driver/postgres v1.5.9 // indirect
	gorm.io/driver/sqlserver v1.5.3 // indirect
	gorm.io/gorm v1.25.12 // indirect
	gorm.io/plugin/dbresolver v1.5.3 // indirect
	modernc.org/libc v1.22.2 // indirect
	modernc.org/mathutil v1.5.0 // indirect
	modernc.org/memory v1.5.0 // indirect
	modernc.org/sqlite v1.20.3 // indirect
)

require (
	ariga.io/atlas v0.32.1-0.20250325101103-175b25e1c1b9 // indirect
	dario.cat/mergo v1.0.0 // indirect
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/apparentlymart/go-textseg/v15 v15.0.0 // indirect
	github.com/bmatcuk/doublestar v1.3.4 // indirect
	github.com/bradfitz/gomemcache v0.0.0-20250403215159-8d39553ac7cf
	github.com/bwmarrin/snowflake v0.3.0
	github.com/casbin/gorm-adapter/v3 v3.32.0
	github.com/coreos/go-oidc/v3 v3.14.1
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-kratos/aegis v0.2.0 // indirect
	github.com/go-kratos/aip-go/ents v0.0.0-20251213081434-74ffa1fc1588
	github.com/go-kratos/kratos/contrib/middleware/validate/v2 v2.0.0-20250731084034-f7f150c3f139
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/inflect v0.21.0 // indirect
	github.com/go-playground/form/v4 v4.2.1 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.2
	github.com/google/gnostic-models v0.7.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/handlers v1.5.2
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/websocket v1.5.3
	github.com/hashicorp/hcl/v2 v2.23.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/redis/go-redis/extra/redisotel/v9 v9.8.0
	github.com/redis/go-redis/v9 v9.8.0
	github.com/zclconf/go-cty v1.16.2 // indirect
	github.com/zclconf/go-cty-yaml v1.1.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/metric v1.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	golang.org/x/mod v0.31.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/text v0.33.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251124214823-79d6a2a48846 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
