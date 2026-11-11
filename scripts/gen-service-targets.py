#!/usr/bin/env python3
"""gen-service-targets.py —— 从 services.yaml 生成 mk/services.mk。

原则：服务元数据只有一个事实源（services.yaml），根 Makefile 的服务命令是
**薄壳委托**（`$(MAKE) -C <path> <target>`），不复制 L1 app.mk 的实现，
从而避免"两套实现漂移"。

用法：
    python3 scripts/gen-service-targets.py            # 生成 mk/services.mk
    python3 scripts/gen-service-targets.py --check    # CI 校验生成物无漂移
"""

from __future__ import annotations

import sys
from pathlib import Path

try:
    import yaml
except ImportError:  # pragma: no cover
    sys.exit("需要 PyYAML：python3 -m pip install pyyaml")

ROOT = Path(__file__).resolve().parents[1]
SRC = ROOT / "services.yaml"
OUT = ROOT / "mk" / "services.mk"

HEADER = """\
# =============================================================================
# 本文件由 scripts/gen-service-targets.py 自动生成，禁止手工修改。
# 数据源：backend-service/services.yaml
# 重新生成：make gen-servicetargets
# 校验漂移：make verify-servicetargets
# =============================================================================
#
# 这些是 L0（根 Makefile）的**委托命令**：只做 $(MAKE) -C <service> <target>，
# 真正的实现在 L1 backend-service/app.mk 与服务级 Makefile 中。
# 目的：让"在根目录就能操作任意服务"，同时不产生第二份实现。
"""


def load() -> dict:
    if not SRC.exists():
        sys.exit(f"缺少 {SRC.relative_to(ROOT)}")
    data = yaml.safe_load(SRC.read_text(encoding="utf-8")) or {}
    if not data:
        sys.exit("services.yaml 为空")
    return data


def render(data: dict) -> str:
    out: list[str] = [HEADER]

    # ── 服务清单 ──────────────────────────────────────────────────────────
    names = list(data.keys())
    out.append(".PHONY: services")
    out.append("# 列出所有后端服务及其端口")
    out.append("services:")
    out.append('	@printf "%-12s %-32s %-6s %-6s %-6s %-10s %s\\n" \\')
    out.append('		"SERVICE" "PATH" "HTTP" "GRPC" "SSE" "STATUS" "DB"')
    for name, cfg in data.items():
        db = cfg.get("db") or {}
        db_s = db.get("name") or db.get("driver") or "-"
        out.append(
            '	@printf "%-12s %-32s %-6s %-6s %-6s %-10s %s\\n" \\\n'
            f'\t\t"{name}" "{cfg["path"]}" "{cfg.get("http","-")}" '
            f'"{cfg.get("grpc","-")}" "{cfg.get("sse","-")}" '
            f'"{cfg.get("status","-")}" "{db_s}"'
        )
    out.append("")

    # ── 通用委托：serve / gen / build / test / stop ────────────────────────
    common = [
        ("serve", "run_target", "run", "启动服务（前台）"),
        ("gen", None, "gen", "全量生成该服务的代码"),
        ("build", None, "build", "构建该服务二进制"),
        ("test", None, "test", "运行该服务单元测试"),
        ("stop", "kill_target", "kill", "停止该服务并释放端口"),
    ]
    for verb, override_key, default_target, desc in common:
        out.append(f"# ── {verb}-<service>：{desc} ──")
        for name, cfg in data.items():
            target = cfg.get(override_key) or default_target
            out.append(f".PHONY: {verb}-{name}")
            out.append(f"# {desc}：{name}")
            out.append(f"{verb}-{name}:")
            out.append(
                f'\t@$(MAKE) --no-print-directory -C {cfg["path"]} {target}'
            )
            out.append("")

    # ── cmds 派生：migrate / seed(mock) / policy ──────────────────────────
    cmd_map = [
        ("migrate", "migrate", "执行数据库迁移"),
        ("seed", "mock", "写入 mock / seed 数据"),
        ("policy", "policy", "初始化 Casbin 策略"),
    ]
    for verb, cmd, desc in cmd_map:
        has_any = any(cmd in (cfg.get("cmds") or []) for cfg in data.values())
        out.append(f"# ── {verb}-<service>：{desc}（仅 cmds 含 {cmd} 的服务）──")
        if not has_any:
            out.append(f"# （当前没有服务声明 {cmd}）")
        for name, cfg in data.items():
            if cmd not in (cfg.get("cmds") or []):
                continue
            out.append(f".PHONY: {verb}-{name}")
            out.append(f"# {desc}：{name}")
            out.append(f"{verb}-{name}:")
            out.append(
                f'\t@$(MAKE) --no-print-directory -C {cfg["path"]} {cmd}'
            )
            out.append("")

    out.append("# ── 运行时辅助 ──")
    out.append(".PHONY: kill-all")
    out.append("# 停止全部服务并释放各自端口")
    out.append("kill-all:")
    for name in names:
        out.append(f"\t@$(MAKE) --no-print-directory stop-{name} >/dev/null 2>&1 || true")
    out.append('\t@echo "✅ 已尝试停止全部服务"')
    out.append("")

    return "\n".join(out)


def validate(data: dict) -> None:
    """契约自检：services.yaml 声明的每个服务都必须有 Makefile。

    没有 Makefile 的服务会让根目录的 serve-<svc>/gen-<svc> 等委托命令报
    "No rule to make target"（edai 曾长期如此）。
    """
    missing = [
        (name, cfg["path"])
        for name, cfg in data.items()
        if not (ROOT / cfg["path"] / "Makefile").exists()
    ]
    if missing:
        for name, path in missing:
            print(f"❌ 服务 {name} 缺少 Makefile：{path}/Makefile")
        sys.exit("services.yaml 与仓库不一致：请补齐 Makefile 或在 services.yaml 中移除该服务")


def main() -> int:
    data = load()
    validate(data)
    content = render(data)
    OUT.parent.mkdir(parents=True, exist_ok=True)

    if "--check" in sys.argv:
        current = OUT.read_text(encoding="utf-8") if OUT.exists() else ""
        if current != content:
            print("❌ mk/services.mk 与 services.yaml 不一致，请运行 make gen-servicetargets")
            return 1
        print("✅ mk/services.mk 与 services.yaml 一致")
        return 0

    OUT.write_text(content, encoding="utf-8")
    print(f"✅ 已生成 {OUT.relative_to(ROOT)}（{len(data)} 个服务）")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
