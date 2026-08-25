#!/usr/bin/env python3
"""Evie 词库中心 + 文本增强稳定性测试（多轮调用 API 验证功能稳定）。"""

import json
import random
import sys
import time
from urllib.request import Request, urlopen

BASE_PLATFORM = "http://localhost:8000"
BASE_EVIE = "http://localhost:8100"
USERNAME = "vben"
PASSWORD = "Admin@123456"
TENANT_ID = 1

ROUNDS = 10
CORRECT_CASES = [
    "给小田申请金种子奖励",
    "呃我想给那个技术部小田申请奖励",
    "今天功课了一个技术难点",
    "申请200个种籽",
    "金种子",
    "给小田申请奖励",
    "标准词1",
    "标准词60",
    "别名1-1-1",
]


def http(method, url, token=None, body=None, timeout=15):
    data = json.dumps(body).encode() if body is not None else None
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    req = Request(url, data=data, headers=headers, method=method)
    start = time.time()
    try:
        with urlopen(req, timeout=timeout) as resp:
            status = resp.status
            payload = resp.read().decode()
        return status, json.loads(payload) if payload else {}, time.time() - start
    except Exception as e:
        # 尽量读取错误体
        try:
            status = e.code
            payload = e.read().decode()
            return status, json.loads(payload) if payload else {}, time.time() - start
        except Exception:
            return 0, {"error": str(e)}, time.time() - start


def login():
    status, data, _ = http("POST", f"{BASE_PLATFORM}/platform/v1/auth/login-username",
                           body={"username": USERNAME, "password": PASSWORD, "tenant_id": TENANT_ID})
    if status != 200:
        raise RuntimeError(f"login failed: {status} {data}")
    return data["accessToken"]


def login2():
    status, data, _ = http("POST", f"{BASE_PLATFORM}/platform/v1/auth/login-username",
                           body={"username": "tenant2", "password": PASSWORD, "tenant_id": 2})
    if status != 200:
        raise RuntimeError(f"tenant2 login failed: {status} {data}")
    return data["accessToken"]


def get_list(token, path, params=None):
    qs = "&".join(f"{k}={v}" for k, v in (params or {}).items())
    url = f"{BASE_EVIE}{path}" + (f"?{qs}" if qs else "")
    return http("GET", url, token)


def correct(token, text):
    return http("POST", f"{BASE_EVIE}/evie/v1/correction:correct", token, {"text": text})


def main():
    print(f"=== Evie 稳定性测试：{ROUNDS} 轮 ===")
    token = login()
    print(f"登录成功 tenant={TENANT_ID}\n")

    # 预取词库/词条用于分页测试
    _, dict_resp, _ = get_list(token, "/evie/v1/dictionaries", {"pageSize": 5})
    dictionaries = dict_resp.get("dictionaries", [])
    dict_ids = [d["id"] for d in dictionaries]
    print(f"预取词库 {len(dict_ids)} 个: {dict_ids}")

    entry_ids = []
    if dict_ids:
        _, entry_resp, _ = get_list(token, f"/evie/v1/dictionaries/{dict_ids[0]}/entries", {"pageSize": 5})
        entry_ids = [e["id"] for e in entry_resp.get("entries", [])]
    print(f"预取词条 {len(entry_ids)} 个: {entry_ids}\n")

    results = []
    for rnd in range(1, ROUNDS + 1):
        print(f"--- 第 {rnd} 轮 ---")
        round_results = {}

        # 1. 词库列表（多页）
        for page in [1, 2, 3]:
            status, data, dt = get_list(token, "/evie/v1/dictionaries",
                                        {"pageSize": 20, "pageToken": str((page - 1) * 20)})
            round_results[f"dict_page{page}"] = (status, len(data.get("dictionaries", [])), data.get("total", 0), dt)

        # 2. 词条列表（选一个词库，翻 2 页）
        if dict_ids:
            for page in [1, 2]:
                status, data, dt = get_list(token, f"/evie/v1/dictionaries/{dict_ids[0]}/entries",
                                            {"pageSize": 20, "pageToken": str((page - 1) * 20)})
                round_results[f"entry_page{page}"] = (status, len(data.get("entries", [])), data.get("total", 0), dt)

        # 3. 关系列表（选一个词条）
        if entry_ids:
            status, data, dt = get_list(token, f"/evie/v1/entries/{entry_ids[0]}/relations", {"pageSize": 20})
            round_results["relation_list"] = (status, len(data.get("relations", [])), data.get("total", 0), dt)

        # 4. 分类/版本/冲突/策略/场景/日志列表
        status, data, dt = get_list(token, "/evie/v1/dictionary-categories", {"pageSize": 20})
        round_results["category_list"] = (status, len(data.get("categories", [])), data.get("total", 0), dt)
        if dict_ids:
            status, data, dt = get_list(token, f"/evie/v1/dictionaries/{dict_ids[0]}/versions", {"pageSize": 20})
            round_results["version_list"] = (status, len(data.get("versions", [])), data.get("total", 0), dt)
        status, data, dt = get_list(token, "/evie/v1/dictionary-conflicts", {"pageSize": 20})
        round_results["conflict_list"] = (status, len(data.get("conflicts", [])), data.get("total", 0), dt)
        status, data, dt = get_list(token, "/evie/v1/enhancement-policies", {"pageSize": 20})
        round_results["policy_list"] = (status, len(data.get("policies", [])), data.get("total", 0), dt)
        status, data, dt = get_list(token, "/evie/v1/enhancement-profiles", {"pageSize": 20})
        round_results["profile_list"] = (status, len(data.get("profiles", [])), data.get("total", 0), dt)
        status, data, dt = get_list(token, "/evie/v1/enhancement-logs", {"pageSize": 20})
        round_results["log_list"] = (status, len(data.get("logs", [])), data.get("total", 0), dt)

        # 5. Correct 文本增强（多组输入）
        for i, text in enumerate(CORRECT_CASES):
            status, data, dt = correct(token, text)
            round_results[f"correct_{i}"] = (status, data.get("correctedText", ""), data.get("changes", []), dt)

        # 5.1 大分页：词条翻 5 页（每页 20）
        if dict_ids:
            for page in [3, 4, 5]:
                status, data, dt = get_list(token, f"/evie/v1/dictionaries/{dict_ids[0]}/entries",
                                            {"pageSize": 20, "pageToken": str((page - 1) * 20)})
                round_results[f"entry_deep_page{page}"] = (status, len(data.get("entries", [])), data.get("total", 0), dt)

        # 6. 随机小规模 CRUD（创建词条→查询→更新→删除）
        if dict_ids:
            rand_text = f"稳定性测试词条-{rnd}-{random.randint(1000, 9999)}"
            status, data, dt = http("POST", f"{BASE_EVIE}/evie/v1/dictionaries/{dict_ids[0]}/entries",
                                    token, {"standardText": rand_text, "entryType": "WORD", "category": "TERM"})
            round_results["create_entry"] = (status, data.get("id"), dt)
            if status == 200 and data.get("id"):
                new_id = data["id"]
                status, data, dt = http("PUT", f"{BASE_EVIE}/evie/v1/entries/{new_id}", token,
                                        {"standardText": rand_text, "entryType": "WORD", "category": "TERM", "priority": 99})
                round_results["update_entry"] = (status, data.get("priority"), dt)
                status, data, dt = http("DELETE", f"{BASE_EVIE}/evie/v1/entries/{new_id}", token)
                round_results["delete_entry"] = (status, dt)

        # 7. 租户 2 隔离测试（每轮随机做 1 次）
        if rnd % 2 == 0:
            token2 = login2()
            status, data, dt = get_list(token2, "/evie/v1/dictionaries", {"pageSize": 5})
            round_results["tenant2_dict"] = (status, len(data.get("dictionaries", [])), data.get("total", 0), dt)
            status, data, dt = correct(token2, "给小田申请奖励")
            round_results["tenant2_correct"] = (status, data.get("correctedText", ""), dt)

        # 汇总
        failed = [k for k, v in round_results.items() if v[0] != 200]
        total_time = sum(v[-1] for v in round_results.values())
        results.append((rnd, round_results, failed, total_time))
        print(f"  接口数: {len(round_results)}, 失败: {len(failed)} {failed if failed else '无'}, 总耗时: {total_time:.2f}s")

        # 随机延迟模拟真实使用
        time.sleep(random.uniform(0.3, 0.8))

    # 最终汇总
    print("\n=== 汇总 ===")
    all_failures = []
    for rnd, round_results, failed, total_time in results:
        status_counts = {}
        for k, v in round_results.items():
            status_counts[v[0]] = status_counts.get(v[0], 0) + 1
        print(f"第 {rnd} 轮: {len(round_results)} 请求, 状态码分布 {status_counts}, 耗时 {total_time:.2f}s")
        all_failures.extend((rnd, k, v) for k, v in round_results.items() if v[0] != 200)

    if all_failures:
        print(f"\n❌ 发现 {len(all_failures)} 个失败请求:")
        for rnd, k, v in all_failures[:20]:
            print(f"  第{rnd}轮 {k}: status={v[0]} resp={str(v[1])[:200]}")
        sys.exit(1)
    print("\n✅ 全部接口多轮调用成功，稳定性测试通过")


if __name__ == "__main__":
    main()
