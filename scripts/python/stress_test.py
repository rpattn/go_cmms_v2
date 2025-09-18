"""Stress test script for the Go CMMS API.

This script exercises the authentication, table management, row insertion,
and search endpoints with configurable concurrency.  It intentionally creates
separate organisations per synthetic user so that each one can provision their
own tables, columns, and data without interfering with the others.

Usage example::

    python scripts/stress_test.py --base-url http://localhost:8080 \
        --users 5 --tables-per-user 3 --rows-per-table 2000 --search-requests 200

Requirements:
    pip install httpx
"""

from __future__ import annotations

import argparse
import asyncio
import json
import math
import random
import statistics
import string
import sys
import time
from collections import Counter, defaultdict
from dataclasses import dataclass, field
from typing import Any, Dict, Iterable, List, Optional

try:
    import httpx
except ImportError as exc:  # pragma: no cover - dependency guard
    raise SystemExit("Install the httpx package to run this script: pip install httpx") from exc


# ----------------------------- Metrics helpers ------------------------------


@dataclass
class MetricEntry:
    """Single request metric."""

    duration_ms: float
    status_code: Optional[int]
    ok: bool
    extra: Dict[str, Any] = field(default_factory=dict)


class MetricsRecorder:
    """Collects latency and status metrics grouped by stage name."""

    def __init__(self) -> None:
        self._data: Dict[str, List[MetricEntry]] = defaultdict(list)

    def record(
        self,
        stage: str,
        start: float,
        status_code: Optional[int],
        ok: bool,
        **extra: Any,
    ) -> None:
        duration_ms = (time.perf_counter() - start) * 1000
        self._data[stage].append(MetricEntry(duration_ms, status_code, ok, extra))

    def summary(self) -> Dict[str, Dict[str, Any]]:
        summary: Dict[str, Dict[str, Any]] = {}
        for stage, entries in self._data.items():
            durations = [m.duration_ms for m in entries]
            statuses = Counter(m.status_code for m in entries)
            success = sum(1 for m in entries if m.ok)
            failure = len(entries) - success
            stats: Dict[str, Any] = {
                "requests": len(entries),
                "success": success,
                "failure": failure,
                "status_counts": dict(statuses),
            }
            if durations:
                sorted_durations = sorted(durations)
                stats.update(
                    {
                        "avg_ms": statistics.fmean(durations),
                        "min_ms": sorted_durations[0],
                        "max_ms": sorted_durations[-1],
                        "p50_ms": percentile(sorted_durations, 0.50),
                        "p95_ms": percentile(sorted_durations, 0.95),
                    }
                )
            summary[stage] = stats
        return summary

    def to_json(self) -> str:
        return json.dumps(self.summary(), indent=2, sort_keys=True)


def percentile(sorted_values: List[float], pct: float) -> float:
    """Return the percentile for an already sorted list."""

    if not sorted_values:
        return math.nan
    k = (len(sorted_values) - 1) * pct
    f = math.floor(k)
    c = math.ceil(k)
    if f == c:
        return sorted_values[int(k)]
    d0 = sorted_values[f] * (c - k)
    d1 = sorted_values[c] * (k - f)
    return d0 + d1


# --------------------------- Synthetic data builders ------------------------


STATUSES = ["open", "in_progress", "blocked", "complete"]
CATEGORIES = ["electrical", "mechanical", "safety", "general"]


def random_string(length: int = 8) -> str:
    alphabet = string.ascii_lowercase + string.digits
    return "".join(random.choice(alphabet) for _ in range(length))


def build_row_payload(row_idx: int) -> Dict[str, Any]:
    status = random.choice(STATUSES)
    category = random.choice(CATEGORIES)
    cost = round(random.uniform(10, 5000), 2)
    completed = status == "complete" and random.random() < 0.8
    payload: Dict[str, Any] = {
        "title": f"Work item {row_idx} - {status}",
        "status": status,
        "category": category,
        "cost": cost,
        "completed": completed,
    }
    print(payload)
    return payload


def build_search_payload() -> Dict[str, Any]:
    filters = []
    if random.random() < 0.7:
        filters.append({"field": "status", "operation": "eq", "value": random.choice(STATUSES)})
    if random.random() < 0.4:
        filters.append({"field": "category", "operation": "eq", "value": random.choice(CATEGORIES)})
    if random.random() < 0.3:
        filters.append({"field": "completed", "operation": "eq", "value": random.choice([True, False])})

    sort_field = random.choice(["created_at", "title", "status", "category"])
    direction = random.choice(["asc", "desc"])
    return {
        "pageNum": random.randint(0, 4),
        "pageSize": random.choice([10, 25, 50, 100]),
        "filterFields": filters,
        "sortField": sort_field,
        "direction": direction,
    }


# ----------------------------- Domain dataclasses ----------------------------


@dataclass
class TableConfig:
    name: str
    slug: str


@dataclass
class UserContext:
    email: str
    username: str
    org_slug: str
    client: httpx.AsyncClient
    tables: List[TableConfig] = field(default_factory=list)


# ---------------------------- Core stress routines -------------------------


async def signup_user(base_url: str, idx: int, metrics: MetricsRecorder) -> UserContext:
    email = f"stress{idx}@example.com"
    username = f"stress{idx}"
    org_slug = f"org-{idx}-{random_string(6)}"
    client = httpx.AsyncClient(base_url=base_url, timeout=30.0)

    payload = {
        "email": email,
        "username": username,
        "name": f"Stress Tester {idx}",
        "password": f"Passw0rd!{idx}",
        "org_slug": org_slug,
    }

    start = time.perf_counter()
    try:
        resp = await client.post("/auth/signup", json=payload)
    except Exception as exc:  # pragma: no cover - network failure path
        metrics.record("signup", start, None, False, error=str(exc))
        raise

    metrics.record("signup", start, resp.status_code, resp.is_success)
    if resp.status_code not in (200, 201):
        body = await resp.aread()
        raise RuntimeError(f"Signup failed for {email}: {resp.status_code} {body.decode()}"
                           )

    return UserContext(email=email, username=username, org_slug=org_slug, client=client)


async def create_table(user: UserContext, table_idx: int, metrics: MetricsRecorder) -> TableConfig:
    table_name = f"work_items_{table_idx}_{random_string(4)}"
    start = time.perf_counter()
    resp = await user.client.post("/tables/", json={"name": table_name})
    metrics.record("create_table", start, resp.status_code, resp.is_success)
    resp.raise_for_status()
    data = resp.json()
    table = data.get("table", {})
    slug = table.get("slug")
    if not slug:
        raise RuntimeError(f"Table slug missing in response: {data}")
    return TableConfig(name=table_name, slug=slug)


async def add_columns(user: UserContext, table: TableConfig, metrics: MetricsRecorder) -> None:
    column_payloads = [
        {"name": "title", "type": "text", "required": True, "indexed": True},
        {
            "name": "status",
            "type": "enum",
            "required": True,
            "indexed": True,
            "enum_values": STATUSES,
        },
        {
            "name": "category",
            "type": "enum",
            "required": False,
            "indexed": True,
            "enum_values": CATEGORIES,
        },
        {
            "name": "cost",
            "type": "float",
            "required": False,
            "indexed": False,
        },
        {
            "name": "completed",
            "type": "bool",
            "required": False,
            "indexed": False,
        },
    ]

    for payload in column_payloads:
        start = time.perf_counter()
        resp = await user.client.post(f"/tables/{table.slug}/columns", json=payload)
        metrics.record("add_column", start, resp.status_code, resp.is_success)
        resp.raise_for_status()


import re
DEADLOCK_RE = re.compile(r"(40P01|deadlock|serialize|serialization|timeout|insert failed)", re.IGNORECASE)

async def insert_rows(
    user: UserContext,
    table: TableConfig,
    rows_per_table: int,
    row_concurrency: int,
    metrics: MetricsRecorder,
) -> None:
    sem = asyncio.Semaphore(row_concurrency)
    max_retries = 3

    async def one_post(row_idx: int) -> None:
        payload = build_row_payload(row_idx)
        # Optional: make retries idempotent if your server supports it
        headers = {"Idempotency-Key": f"{table.slug}:{row_idx}"}

        attempt = 0
        while True:
            attempt += 1
            async with sem:
                start = time.perf_counter()
                try:
                    resp = await user.client.post(
                        f"/tables/{table.slug}/rows",
                        json=payload,
                        headers=headers,
                    )
                except Exception as exc:
                    # network / transport error – retryable
                    metrics.record("add_row", start, None, False, error=str(exc), attempt=attempt)
                    if attempt <= max_retries:
                        await asyncio.sleep(random.uniform(0.025, 0.075))
                        continue
                    raise

                ok = resp.is_success
                metrics.record("add_row", start, resp.status_code, ok, attempt=attempt)

                if ok:
                    return

                body = (await resp.aread()).decode(errors="ignore")

                # classify retryable
                retryable_http = resp.status_code in (409, 425, 429, 500, 502, 503, 504)
                retryable_body = DEADLOCK_RE.search(body) is not None

                # Some servers incorrectly return 400 for transient DB errors. Only retry 400 if the body matches.
                retryable = retryable_http or (resp.status_code == 400 and retryable_body)

                await asyncio.sleep(random.uniform(0, 0.02))

                if retryable and attempt <= max_retries:
                    await asyncio.sleep(random.uniform(0.025, 0.075))
                    continue

                raise RuntimeError(
                    f"Insert failed after {attempt} attempt(s) "
                    f"({resp.status_code}): {body[:300]}"
                )

    # Gather but DO NOT fail-fast: collect exceptions and report at the end
    results = await asyncio.gather(*(one_post(i) for i in range(rows_per_table)), return_exceptions=True)

    failures = [e for e in results if isinstance(e, Exception)]
    if failures:
        # Surface a concise summary while keeping successful inserts
        counts = Counter(type(e).__name__ for e in failures)
        examples = "\n".join(f"  - {type(e).__name__}: {str(e)[:160]}" for e in failures[:5])
        raise RuntimeError(
            f"{len(failures)} row insert(s) ultimately failed.\nBy type: {dict(counts)}\nExamples:\n{examples}"
        )


async def run_searches(
    user: UserContext,
    table: TableConfig,
    request_count: int,
    concurrency: int,
    metrics: MetricsRecorder,
) -> None:
    sem = asyncio.Semaphore(concurrency)

    async def worker(_: int) -> None:
        payload = build_search_payload()
        async with sem:
            start = time.perf_counter()
            resp = await user.client.post(f"/tables/{table.slug}/search", json=payload)
            ok = resp.is_success
            metrics.record("search", start, resp.status_code, ok, pageSize=payload.get("pageSize"))
            if not ok:
                body = await resp.aread()
                raise RuntimeError(
                    f"Search failed ({resp.status_code}): {body.decode(errors='ignore')}"
                )
            _ = resp.json()

    await asyncio.gather(*(worker(i) for i in range(request_count)))


async def stress_user(
    base_url: str,
    user_idx: int,
    tables_per_user: int,
    rows_per_table: int,
    row_concurrency: int,
    search_requests: int,
    search_concurrency: int,
    metrics: MetricsRecorder,
) -> None:
    user = await signup_user(base_url, user_idx, metrics)
    try:
        for t_idx in range(tables_per_user):
            table = await create_table(user, t_idx, metrics)
            user.tables.append(table)
            await add_columns(user, table, metrics)
            await insert_rows(user, table, rows_per_table, row_concurrency, metrics)
            await run_searches(user, table, search_requests, search_concurrency, metrics)
    finally:
        await user.client.aclose()


# ------------------------------ CLI plumbing --------------------------------


def parse_args(argv: Optional[Iterable[str]] = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Stress test the Go CMMS API server")
    parser.add_argument("--base-url", default="http://127.0.0.1:8080", help="Server base URL")
    parser.add_argument("--users", type=int, default=3, help="Number of synthetic users to create")
    parser.add_argument(
        "--tables-per-user", type=int, default=2, help="Tables created by each user"
    )
    parser.add_argument(
        "--rows-per-table", type=int, default=2000, help="Rows inserted into each table"
    )
    parser.add_argument(
        "--row-concurrency", type=int, default=5, help="Concurrent row insertions per table"
    )
    parser.add_argument(
        "--search-requests",
        type=int,
        default=200,
        help="Number of search requests per table",
    )
    parser.add_argument(
        "--search-concurrency",
        type=int,
        default=50,
        help="Concurrent search requests per table",
    )
    parser.add_argument(
        "--output",
        type=str,
        default="",
        help="Optional path to write the aggregated metrics as JSON",
    )
    parser.add_argument(
        "--seed",
        type=int,
        default=None,
        help="Seed for the random number generator to reproduce runs",
    )
    return parser.parse_args(argv)


async def async_main(args: argparse.Namespace) -> MetricsRecorder:
    if args.seed is not None:
        random.seed(args.seed)

    metrics = MetricsRecorder()
    tasks = [
        stress_user(
            base_url=args.base_url,
            user_idx=i,
            tables_per_user=args.tables_per_user,
            rows_per_table=args.rows_per_table,
            row_concurrency=args.row_concurrency,
            search_requests=args.search_requests,
            search_concurrency=args.search_concurrency,
            metrics=metrics,
        )
        for i in range(args.users)
    ]
    await asyncio.gather(*tasks)
    return metrics


def main(argv: Optional[Iterable[str]] = None) -> None:
    args = parse_args(argv)

    try:
        metrics = asyncio.run(async_main(args))
    except KeyboardInterrupt:  # pragma: no cover - manual interruption
        print("Interrupted", file=sys.stderr)
        return

    summary = metrics.summary()
    print("\n=== Stress Test Summary ===")
    for stage, stats in summary.items():
        print(f"\nStage: {stage}")
        for key, value in stats.items():
            if isinstance(value, float):
                print(f"  {key}: {value:.2f}")
            else:
                print(f"  {key}: {value}")

    if args.output:
        with open(args.output, "w", encoding="utf-8") as fh:
            fh.write(metrics.to_json())
        print(f"\nMetrics written to {args.output}")


if __name__ == "__main__":
    main()