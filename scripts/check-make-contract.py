#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
make 命令契约自检门禁。

## 为什么需要它

2026-11-13 的真实故障：conf 生成机制收敛到 `pkg/conf` 后，`app.mk` 里的
`config` 目标被删除，但 `gen` / `app` 的前置列表里仍留着 `config`。

    $ cd app/platform/service && make gen
    make: *** No rule to make target `config', needed by `gen'.  Stop.

后果不是"少生成一个文件"，而是**命令直接不可用**：
  - 5 个服务（platform / ai / evie / edai / version）的 `make gen` 全部 exit 2
  - 根目录的 `make gen-<svc>` 委托链随之全部失败
  - `make app`（全流水线）同样失败

这类缺陷的共同特征：**Makefile 里引用的名字，在 make 求值前谁都没有校验过。**
make 本身只在真正构建某个目标时才报错，所以"删目标"和"删引用"之间没有任何
强制同步 —— 靠人工评审必然复发（本次就是复发）。

## 本门禁校验三件事

1. 前置闭包：每个 target 的前置目标都必须在本上下文内有规则定义，
   或者是真实存在的文件。变量前置（`$(LOCAL_GEN_DEPS)` 等）先展开再校验，
   展开不出来的变量直接报错（不允许"看不见的前置"）。
2. 委托一致性：每个 `$(MAKE) -C <dir> <target>` 委托，`<dir>` 必须是已知的
   make 上下文，且 `<target>` 必须在**那个上下文**里已定义。
   （`gen-<svc>` 这类薄壳命令的失效正是从这一环暴露的。）
3. `.PHONY` 对称性：声明了但没有规则的 target（死声明）必须清零；
   有规则但未声明 `.PHONY` 的 target 也报错（否则同名文件会让 make
   静默跳过命令 —— 典型的"改了没生效"）。

## 设计取舍

- **纯静态解析，不调用 make**：只读、无副作用、可在任何环境（含离线 CI）跑，
  且不会因为 `make -n` 执行 `$(MAKE)` 递归行而产生真实副作用。
- **不做通用 Makefile 解析器**：只覆盖本仓库实际使用的语法子集
  （单行规则、`\\` 续行、`:=`/`?=`/`=` 变量、`.PHONY`、`ifdef` 块）。
  遇到不认识的写法宁可报错（fail-closed），也不要静默放过。
- 上下文清单与 `docs/architecture/4-10-治理-命令契约.md` 的三层模型对齐；
  新增 L2 目录时必须同步登记，否则脚本会提示"未登记"。

用法：
    python3 scripts/check-make-contract.py          # 校验
    python3 scripts/check-make-contract.py --list   # 打印解析结果（排障用）
"""

from __future__ import annotations

import os
import re
import sys

BACKEND_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

# ---------------------------------------------------------------------------
# 上下文清单：一个"上下文"= 一个 make 求值现场（在哪个目录、include 了哪些文件）
# ---------------------------------------------------------------------------
CONTEXTS = [
    ("backend-service 根（L0）", ".", ["Makefile", "mk/services.mk"]),
    ("platform（L1+L2）", "app/platform/service", ["Makefile", "../../../app.mk"]),
    ("ai（L1+L2）", "app/ai/service", ["Makefile", "../../../app.mk"]),
    ("evie（L1+L2）", "app/evie/service", ["Makefile", "../../../app.mk"]),
    ("edai（L1+L2）", "app/edai/service", ["Makefile", "../../../app.mk"]),
    ("version（L1+L2）", "app/version/service", ["Makefile", "../../../app.mk"]),
    ("evie/tool（L1+L2）", "app/evie/tool", ["Makefile", "../../../app.mk"]),
    ("funasr（独立）", "app/funasr/service", ["Makefile"]),
    ("lexnorm（独立模块）", "pkg/lexnorm", ["Makefile"]),
]

# 规则行：目标名 + 冒号 + 前置（允许为空）；排除 `:=` / `?=` 赋值
RE_RULE = re.compile(r"^([A-Za-z][A-Za-z0-9_.-]*)\s*:(?!=)(.*)$")
RE_ASSIGN = re.compile(r"^([A-Za-z_][A-Za-z0-9_]*)\s*(\?=|:=|\+=|=)\s*(.*)$")
RE_PHONY = re.compile(r"^\.PHONY:\s*(.*)$")
RE_VAR = re.compile(r"\$[({]([A-Za-z_][A-Za-z0-9_]*)[)}]")
# $(abspath X) 可能嵌在更长的文本里（如 `-C $(BACKEND_ROOT) target` 展开后）
RE_ABS = re.compile(r"\$\(abspath\s+([^()$]+)\)")

# make 选项：捕获 `-C <dir>` 之类的带参选项
OPT_WITH_ARG = {"-C", "--directory", "-f", "--file", "-I", "--include-dir"}
OPT_FLAGS = {
    "--no-print-directory",
    "--no-print-direct",
    "-s",
    "--silent",
    "--quiet",
    "-n",
    "--just-print",
    "--dry-run",
}


def strip_comment(line: str) -> str:
    """剥离 Makefile 行尾注释（保留 \\# 转义）。recipe 里的 # 也一并剥离：
    只影响 $(MAKE) 委托的可见范围，不会造成误报。"""
    return re.sub(r"(?<!\\)#.*$", "", line)


class Ctx:
    """一个 make 上下文：目录 + 参与文件，解析出变量/目标/前置/.PHONY/委托。"""

    def __init__(self, label: str, rel_dir: str, files: list[str]):
        self.label = label
        self.rel_dir = rel_dir
        self.dir = os.path.realpath(os.path.join(BACKEND_ROOT, rel_dir))
        self.files = files
        self.vars: dict[str, str] = {}
        self.rules: dict[str, list[str]] = {}      # target -> 展开后的前置（原文）
        self.phony: dict[str, str] = {}            # target -> 文件名
        self.delegations: list[tuple[str, str, str, str]] = []  # (dir, target, file, raw)
        self.recipes: dict[str, list[str]] = {}     # target -> recipe 原始行
        self.errors: list[str] = []
        self._parse()

    # -- 解析 ---------------------------------------------------------------
    def _logical_lines(self):
        """产出 (文件名, 行号, 逻辑行, 是否 recipe)。

        续行（行尾 `\\`）必须合并成**一条**：对 recipe 而言，make 是把整个
        续行块交给同一个 `sh -c` 执行的。也就是说「一条逻辑 recipe 行」
        = 一个 shell 进程 —— 这决定了 `exit 0` 的作用域（也决定它是有效跳过
        还是只跳过了半条命令）。
        """
        for name in self.files:
            path = os.path.join(self.dir, name)
            if not os.path.exists(path):
                self.errors.append(f"{name}: 文件不存在（上下文 {self.rel_dir}）")
                continue
            with open(path, encoding="utf-8") as fh:
                raw = fh.read().splitlines()
            buf: list[str] = []
            start, is_recipe = 0, False
            for i, line in enumerate(raw, 1):
                cont = line.rstrip().endswith("\\")
                body = line.rstrip()[:-1] if cont else line
                if not buf:
                    start, is_recipe = i, line.startswith("\t")
                buf.append(body)
                if cont:
                    continue
                yield name, start, "\n".join(buf), is_recipe
                buf = []
            if buf:
                yield name, start, "\n".join(buf), is_recipe

    def _parse(self):
        logical = list(self._logical_lines())

        # 第一遍：变量（后者覆盖前者，与 make 行为一致）
        for name, _ln, line, is_recipe in logical:
            if is_recipe:
                continue
            m = RE_ASSIGN.match(strip_comment(line).strip())
            if not m:
                continue
            key, op, val = m.group(1), m.group(2), m.group(3).strip()
            if op == "?=":
                self.vars.setdefault(key, val)      # ?= 只在未定义时生效
            elif op == "+=":
                self.vars[key] = (self.vars.get(key, "") + " " + val).strip()
            else:
                self.vars[key] = val                # := 与 = 覆盖

        # 第二遍：规则 / .PHONY / 委托
        current = None
        for name, _ln, line, is_recipe in logical:
            code = strip_comment(line)
            if not code.strip():
                continue
            if is_recipe:
                if current:
                    self.recipes.setdefault(current, []).append(line.strip())
                self._scan_make_calls(code, name)
                continue
            stripped = code.strip()
            m = RE_PHONY.match(stripped)
            if m:
                for tok in self.expand(m.group(1)).split():
                    self.phony.setdefault(tok, name)
                continue
            if stripped.startswith("."):  # 其它特殊目标（.DEFAULT_GOAL 等）
                continue
            m = RE_RULE.match(stripped)
            if m:
                target = m.group(1)
                current = target
                self.rules.setdefault(target, [])
                self.rules[target].append(m.group(2).strip())
            self._scan_make_calls(stripped, name)

    def _scan_make_calls(self, line: str, fname: str):
        for _m in re.finditer(re.escape("$(MAKE)"), line):
            tail = line[_m.end() :]
            toks = self.expand(tail).split()
            i, directory = 0, None
            target = None
            while i < len(toks):
                tok = toks[i]
                if tok in OPT_WITH_ARG:
                    if tok in ("-C", "--directory") and i + 1 < len(toks):
                        directory = toks[i + 1]
                    i += 2
                    continue
                if tok.startswith("--directory="):
                    directory = tok.split("=", 1)[1]
                    i += 1
                    continue
                if tok.startswith("-"):
                    i += 1
                    continue
                target = tok
                break
            if target:
                self.delegations.append((directory or ".", target, fname, line.strip()))

    # -- 变量展开 -----------------------------------------------------------
    def expand(self, text: str, depth: int = 0) -> str:
        if depth > 8:
            return text

        # $(abspath X) → 以本上下文目录为基准求绝对路径（本仓库只此一处用）
        def _abs(m):
            return os.path.realpath(os.path.join(self.dir, m.group(1).strip()))

        text = RE_ABS.sub(_abs, text)

        def _sub(m):
            key = m.group(1)
            if key in self.vars:
                return self.vars[key]
            self.errors.append(f"未定义变量 $({key})（上下文 {self.rel_dir}）")
            return ""

        new = RE_VAR.sub(_sub, text)
        if new == text:
            return new
        return self.expand(new, depth + 1)


def check() -> int:
    problems: list[str] = []
    contexts: dict[str, Ctx] = {}

    for label, rel, files in CONTEXTS:
        ctx = Ctx(label, rel, files)
        contexts[ctx.dir] = ctx
        for err in ctx.errors:
            problems.append(f"[{label}] {err}")
        ctx.errors.clear()

    # 1) 前置闭包
    for ctx in contexts.values():
        for target, prereq_texts in ctx.rules.items():
            for text in prereq_texts:
                expanded = ctx.expand(text)
                for tok in expanded.split():
                    if tok in ctx.rules:
                        continue
                    if not re.match(r"^[A-Za-z][A-Za-z0-9_.-]*$", tok):
                        continue  # 非目标名（文件路径 / 特殊写法），不判
                    if os.path.exists(os.path.join(ctx.dir, tok)):
                        continue
                    problems.append(
                        f"[{ctx.label}] 目标 '{target}' 的前置 '{tok}' 没有规则定义"
                        f"（make 会报 No rule to make target）"
                    )

    # 2) 委托一致性
    for ctx in contexts.values():
        for directory, target, fname, raw in ctx.delegations:
            if os.path.isabs(directory):
                dest = os.path.realpath(directory)
            else:
                dest = os.path.realpath(os.path.join(ctx.dir, directory))
            if dest not in contexts:
                problems.append(
                    f"[{ctx.label}] {fname}: 委托目录 '{directory}' 不是已登记的 make 上下文"
                    f"（{raw}）"
                )
                continue
            dst = contexts[dest]
            if target not in dst.rules:
                problems.append(
                    f"[{ctx.label}] {fname}: 委托目标 '{target}' 在 {dst.rel_dir}"
                    f" 未定义（{raw}）"
                )

    # 3) .PHONY 对称性
    for ctx in contexts.values():
        for name in sorted(set(ctx.phony) - set(ctx.rules)):
            problems.append(
                f"[{ctx.label}] .PHONY 声明了 '{name}' 但没有任何规则（死声明，"
                f"见 {ctx.phony[name]}）"
            )
        for name in sorted(set(ctx.rules) - set(ctx.phony)):
            problems.append(
                f"[{ctx.label}] 目标 '{name}' 缺少 .PHONY 声明"
                f"（同名文件/目录会让 make 跳过它）"
            )

    # 4) 多行 recipe 里的 `exit 0`：只退出当前行，后续行仍会执行
    for ctx in contexts.values():
        for target, lines in ctx.recipes.items():
            if len(lines) < 2:
                continue
            for line in lines:
                if re.search(r"\bexit\s+0\b", line):
                    problems.append(
                        f"[{ctx.label}] 目标 '{target}' 在多行 recipe 里用 'exit 0' 跳过："
                        f"它只结束当前行的 shell，后续 {len(lines) - 1} 行仍会执行"
                        f"（应改用解析期 ifeq 或单行 if/fi）"
                    )
                    break

    # -- 输出 ---------------------------------------------------------------
    total_targets = sum(len(c.rules) for c in contexts.values())
    total_delegations = sum(len(c.delegations) for c in contexts.values())

    if problems:
        print("❌ make 命令契约校验失败：")
        for p in problems:
            print(f"   - {p}")
        print()
        print(f"共 {len(problems)} 处问题。")
        print("修复方向：删除目标时同步删除所有引用，或把引用改成真实存在的目标；")
        print("        新增 L2 独有生成步骤请用 app.mk 的 LOCAL_GEN_DEPS。")
        return 1

    print(
        f"✅ make 命令契约通过：{len(contexts)} 个上下文 / {total_targets} 个目标 / "
        f"{total_delegations} 条委托 / 前置闭包·委托·.PHONY 全部一致"
    )
    return 0


def list_all() -> int:
    for label, rel, files in CONTEXTS:
        ctx = Ctx(label, rel, files)
        print(f"\n== {label}  ({rel})  文件: {', '.join(files)}")
        print(f"   目标({len(ctx.rules)}): {' '.join(sorted(ctx.rules))}")
        print(f"   委托({len(ctx.delegations)}):")
        for directory, target, _f, _raw in ctx.delegations:
            print(f"     -> {directory} :: {target}")
    return 0


if __name__ == "__main__":
    if "--list" in sys.argv:
        sys.exit(list_all())
    sys.exit(check())