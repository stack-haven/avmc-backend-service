#!/usr/bin/env bash
#
# db-backup.sh — 备份后端各服务对应的 MySQL 数据库（结构 + 数据）
#
# 设计目标：
#   1. 循环扫描 backend-service 下所有服务的 configs/config.yaml，
#      解析 data.database.source（Go DSN 格式），得到库名 / 账号 / 密码 / 地址。
#   2. 对每个库执行 mysqldump（优先走 docker exec 进 mysql 容器，本机有 mysqldump 也可）。
#   3. 导出文件写入 deploy/sql/<库名>_<YYYYMMDDHHMM>.sql（可选 gzip）。
#   4. 记录每个文件的备份时间，幂等可重复执行，失败不中断其余库。
#
# 用法：
#   ./scripts/db-backup.sh                 # 默认备份所有服务库
#   ./scripts/db-backup.sh --gzip          # 输出 .sql.gz
#   ./scripts/db-backup.sh --db platform_system,platform_ai   # 只备份指定库
#   ./scripts/db-backup.sh --dry-run       # 只打印将要执行的命令，不真正 dump
#
# 可用环境变量覆盖（默认值见下方「配置」区块）：
#   BACKEND_DIR      后端仓库根目录
#   OUTPUT_DIR       备份输出目录
#   MYSQL_CONTAINER  MySQL 所在的 docker 容器名（留空则自动探测）
#   MYSQLDUMP_BIN    本机 mysqldump 可执行文件路径（优先使用 docker exec）
#   EXTRA_DBS        额外要备份的库（逗号分隔，不在任何服务配置中但需要备份）
#   EXTRA_DBS_DSN    额外库的连接 DSN（缺省 root:root@tcp(127.0.0.1:3306)/<db>）
#   SKIP_SYSTEM_DBS  是否跳过 MySQL 系统库（1=跳过，0=不跳过）

set -uo pipefail

# ===== 配置（可用环境变量覆盖）=====
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="${BACKEND_DIR:-$(cd "$SCRIPT_DIR/.." && pwd)}"
OUTPUT_DIR="${OUTPUT_DIR:-$BACKEND_DIR/deploy/sql}"
MYSQL_CONTAINER="${MYSQL_CONTAINER:-}"
MYSQLDUMP_BIN="${MYSQLDUMP_BIN:-}"
EXTRA_DBS="${EXTRA_DBS:-}"
EXTRA_DBS_DSN="${EXTRA_DBS_DSN:-}"
SKIP_SYSTEM_DBS="${SKIP_SYSTEM_DBS:-1}"
TIMESTAMP="$(date +%Y%m%d%H%M)"

# 系统库（不需要备份）
SYSTEM_DBS="information_schema performance_schema mysql sys"

# ===== 日志 =====
log()  { printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"; }
warn() { printf '[%s] [WARN] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" >&2; }
err()  { printf '[%s] [ERROR] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" >&2; }

# ===== 参数解析 =====
GZIP=0
DRY_RUN=0
ONLY_DBS=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --gzip)      GZIP=1 ;;
    --dry-run)   DRY_RUN=1 ;;
    --db)        ONLY_DBS="${2:-}"; shift ;;
    --db=*)      ONLY_DBS="${1#--db=}" ;;
    -h|--help)
      sed -n '2,30p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0 ;;
    *) warn "未知参数: $1（已忽略）" ;;
  esac
  shift
done

# ===== 探测 MySQL 执行方式 =====
DOCKER_EXEC=""

detect_mysql() {
  if [[ -n "$MYSQL_CONTAINER" ]]; then
    if docker ps --format '{{.Names}}' 2>/dev/null | grep -qx "$MYSQL_CONTAINER"; then
      DOCKER_EXEC="docker exec $MYSQL_CONTAINER"
      log "使用 docker 容器: $MYSQL_CONTAINER"
      return 0
    fi
    err "指定的容器 $MYSQL_CONTAINER 未在运行"
    exit 1
  fi

  local container
  container="$(docker ps --format '{{.Names}}\t{{.Image}}' 2>/dev/null \
    | awk -F'\t' 'tolower($2) ~ /mysql/ {print $1; exit}')"
  if [[ -n "$container" ]]; then
    DOCKER_EXEC="docker exec $container"
    log "自动探测到 MySQL 容器: $container"
    return 0
  fi

  local local_dump local_client
  local_dump="${MYSQLDUMP_BIN:-$(command -v mysqldump || true)}"
  local_client="$(command -v mysql || true)"
  if [[ -n "$local_dump" && -n "$local_client" ]]; then
    DOCKER_EXEC=""
    log "使用本机 mysqldump/mysql"
    return 0
  fi

  err "未找到可用的 mysqldump：既无运行中的 mysql 容器，本机也未安装 mysqldump/mysql"
  exit 1
}

# 执行一条 SQL 查询，输出原始结果
mysql_query() {
  local sql="$1" extra="${2:-}"
  if [[ -n "$DOCKER_EXEC" ]]; then
    $DOCKER_EXEC sh -c "mysql $extra -e \"$sql\"" 2>/dev/null
  else
    mysql $extra -e "$sql" 2>/dev/null
  fi
}

# ===== DSN 解析 =====
# Go DSN 形如: user:pass@tcp(host:port)/dbname?params
parse_dsn() {
  local dsn="$1" user pass hostport host port db
  user="$(printf '%s' "$dsn" | sed -E 's/^([^:]*):.*/\1/')"
  pass="$(printf '%s' "$dsn" | sed -E 's/^[^:]*:([^@]*)@.*/\1/')"
  hostport="$(printf '%s' "$dsn" | sed -E 's/.*@tcp\(([^)]*)\).*/\1/')"
  host="${hostport%%:*}"
  port="${hostport##*:}"
  [[ "$host" == "$port" ]] && port="3306"
  db="$(printf '%s' "$dsn" | sed -E 's|.*/([^/?]*)(\?.*)?$|\1|')"
  printf '%s|%s|%s|%s|%s' "$user" "$pass" "$host" "$port" "$db"
}

# ===== 扫描服务配置 =====
# 输出每行: service_label<TAB>dsn
scan_services() {
  local cfg rel label dsn
  while IFS= read -r cfg; do
    dsn="$(grep -E '^[[:space:]]*source:[[:space:]]*' "$cfg" 2>/dev/null | grep '@tcp(' | head -1 | sed -E 's/^[[:space:]]*source:[[:space:]]*//')"
    [[ -z "$dsn" ]] && continue

    rel="${cfg#$BACKEND_DIR/app/}"
    rel="${rel%/configs/config.yaml}"
    rel="${rel%/service}"
    label="${rel//\//-}"
    [[ -z "$label" ]] && label="unknown"

    printf '%s\t%s\n' "$label" "$dsn"
  done < <(find "$BACKEND_DIR/app" -type f -path '*/configs/config.yaml' 2>/dev/null | sort)
}

# 判断库是否存在（用服务自己的 DSN 凭据）
db_exists() {
  local user="$1" pass="$2" host="$3" port="$4" db="$5" out
  local extra="-h${host} -P${port} -u${user} -p${pass}"
  out="$(mysql_query "SELECT SCHEMA_NAME FROM information_schema.schemata WHERE SCHEMA_NAME='${db}'" "$extra")"
  [[ -n "$out" ]]
}

# ===== 主流程 =====
main() {
  detect_mysql
  mkdir -p "$OUTPUT_DIR"

  local label dsn user pass host port db out_file dump_cmd size

  log "扫描后端服务数据库配置..."
  while IFS=$'\t' read -r label dsn; do
    IFS='|' read -r user pass host port db <<< "$(parse_dsn "$dsn")"

    # 显式 --db 白名单
    if [[ -n "$ONLY_DBS" ]]; then
      if ! grep -qw "$db" <<< "${ONLY_DBS//,/ }"; then
        log "跳过 ${db}（不在 --db 白名单）"
        continue
      fi
    fi
    # 系统库
    if [[ "$SKIP_SYSTEM_DBS" == "1" ]] && grep -qw "$db" <<< "$SYSTEM_DBS"; then
      log "跳过系统库 $db"
      continue
    fi
    # 去重
    if [[ " ${BACKED_UP:-} " == *" $db "* ]]; then
      log "跳过 ${db}（已备份过）"
      continue
    fi

    # 库不存在则跳过
    if ! db_exists "$user" "$pass" "$host" "$port" "$db"; then
      warn "库 $db 不存在（服务 ${label}），跳过"
      continue
    fi
    BACKED_UP="${BACKED_UP:-} $db"

    out_file="$OUTPUT_DIR/${db}_${TIMESTAMP}.sql"
    dump_cmd="mysqldump -h${host} -P${port} -u${user} -p${pass} --single-transaction --routines --triggers --add-drop-table ${db}"

    if [[ "$DRY_RUN" == "1" ]]; then
      log "[dry-run] 将备份 $label -> $db -> $out_file"
      continue
    fi

    log "备份 $label 库: ${db}（$user@$host:${port}）-> $out_file"
    if [[ -n "$DOCKER_EXEC" ]]; then
      $DOCKER_EXEC sh -c "$dump_cmd" > "$out_file" 2>/tmp/db-backup.err || {
        warn "备份失败: ${db}（$(tail -1 /tmp/db-backup.err 2>/dev/null)）"
        rm -f "$out_file"
        continue
      }
    else
      sh -c "$dump_cmd" > "$out_file" 2>/tmp/db-backup.err || {
        warn "备份失败: ${db}（$(tail -1 /tmp/db-backup.err 2>/dev/null)）"
        rm -f "$out_file"
        continue
      }
    fi

    if [[ "$GZIP" == "1" ]]; then
      gzip -f "$out_file" 2>/dev/null || true
      out_file="${out_file}.gz"
    fi

    size="$(du -h "$out_file" 2>/dev/null | cut -f1)"
    log "完成: ${out_file}（${size:-未知}）"
  done < <(scan_services)

  # 额外库（不在服务配置中，如独立的 casbin 策略库）
  if [[ -n "$EXTRA_DBS" ]]; then
    local extra_db extra_dsn
    for extra_db in ${EXTRA_DBS//,/ }; do
      [[ " ${BACKED_UP:-} " == *" $extra_db "* ]] && continue
      extra_dsn="${EXTRA_DBS_DSN:-root:root@tcp(127.0.0.1:3306)/${extra_db}}"
      IFS='|' read -r user pass host port db <<< "$(parse_dsn "$extra_dsn")"
      out_file="$OUTPUT_DIR/${extra_db}_${TIMESTAMP}.sql"

      if ! db_exists "$user" "$pass" "$host" "$port" "$extra_db"; then
        warn "额外库 $extra_db 不存在，跳过"
        continue
      fi

      if [[ "$DRY_RUN" == "1" ]]; then
        log "[dry-run] 将备份额外库 $extra_db -> $out_file"
        continue
      fi

      log "备份额外库: $extra_db -> $out_file"
      dump_cmd="mysqldump -h${host} -P${port} -u${user} -p${pass} --single-transaction --routines --triggers --add-drop-table ${extra_db}"
      if [[ -n "$DOCKER_EXEC" ]]; then
        $DOCKER_EXEC sh -c "$dump_cmd" > "$out_file" 2>/tmp/db-backup.err || {
          warn "备份失败: ${extra_db}（$(tail -1 /tmp/db-backup.err 2>/dev/null)）"
          rm -f "$out_file"
          continue
        }
      else
        sh -c "$dump_cmd" > "$out_file" 2>/tmp/db-backup.err || {
          warn "备份失败: ${extra_db}（$(tail -1 /tmp/db-backup.err 2>/dev/null)）"
          rm -f "$out_file"
          continue
        }
      fi
      if [[ "$GZIP" == "1" ]]; then
        gzip -f "$out_file" 2>/dev/null || true
        out_file="${out_file}.gz"
      fi
      log "完成: $out_file"
      BACKED_UP="${BACKED_UP:-} $extra_db"
    done
  fi

  log "全部备份完成，输出目录: $OUTPUT_DIR"
  ls -lh "$OUTPUT_DIR" | tail -n +2
}

main "$@"
