#!/usr/bin/env python3
"""
workload.py  —  read/write workload against the realworld API
Run on the EC2 machine:
    python3 workload.py                  # default 4 hours
    python3 workload.py --duration 60    # quick 1-minute smoke test
"""

import requests
import random
import time
import threading
import argparse
from datetime import datetime, timedelta

API = "http://localhost:8080/api"

USERS = [
    {"email": "alice@example.com",   "password": "password123"},
    {"email": "bobby@example.com",   "password": "password123"},
    {"email": "charlie@example.com", "password": "password123"},
    {"email": "diana@example.com",   "password": "password123"},
    {"email": "evan@example.com",    "password": "password123"},
    {"email": "fiona@example.com",   "password": "password123"},
    {"email": "george@example.com",  "password": "password123"},
    {"email": "helen@example.com",   "password": "password123"},
]

WRITE_TITLES = [
    "今日の出来事について", "最近の気づき", "週末の旅行記",
    "技術トレンド考察", "日常の小さな幸せ", "新しいチャレンジ",
    "読書感想文", "料理の実験", "運動の記録", "仕事の振り返り",
    "My thoughts today", "Tech notes", "Weekend recap",
    "Random observations", "Learning log", "Project update",
    "Quick review", "Daily reflection", "New experiment", "Progress report",
]

WRITE_BODIES = [
    "今日は色々と考えさせられることがありました。毎日の小さな積み重ねが大切だと改めて感じています。これからも継続して努力していきたいと思います。",
    "新しいことを学ぶのは大変ですが、その分達成感も大きいです。今日学んだことを整理して、明日の自分に活かしていきます。",
    "今週も充実した日々を過ごすことができました。周りの人たちに感謝しながら、引き続き頑張っていきたいと思います。",
    "This week I explored several interesting topics that changed my perspective. The key insight was how small details compound over time into significant outcomes.",
    "After reflecting on recent events, I believe consistent action matters more than perfect planning. Progress over perfection is the guiding principle.",
    "Technology continues to evolve rapidly, and staying current requires daily learning. Today's session was particularly rewarding in terms of new insights gained.",
]

WRITE_TAGS = [
    ["日記", "日常"], ["技術", "学習"], ["旅行", "体験"],
    ["読書", "感想"], ["料理", "レシピ"], ["運動", "健康"],
    ["life", "daily"], ["tech", "learning"], ["travel", "experience"],
    ["thoughts", "reflection"], ["coding", "dev"], ["food", "recipe"],
]

# Shared stats — protected by stats_lock
stats = {
    "writes_ok":  0,
    "writes_err": 0,
    "reads_ok":   0,
    "reads_err":  0,
}
stats_lock = threading.Lock()


# ── helpers ──────────────────────────────────────────────────────────────────

def login(email, password):
    try:
        r = requests.post(
            f"{API}/users/login",
            json={"user": {"email": email, "password": password}},
            timeout=10,
        )
        if r.status_code == 200:
            return r.json()["user"]["token"]
    except Exception:
        pass
    return None


def get_article_slugs():
    """Fetch a random page of articles and return their slugs."""
    offset = random.randint(0, 90)
    try:
        r = requests.get(f"{API}/articles?limit=20&offset={offset}", timeout=10)
        if r.status_code == 200:
            return [a["slug"] for a in r.json().get("articles", [])]
    except Exception:
        pass
    return []


# ── workers ──────────────────────────────────────────────────────────────────

def write_worker(token, stop_event):
    """Continuously POST new articles until stop_event is set."""
    session = requests.Session()
    session.headers["Authorization"] = f"Token {token}"

    while not stop_event.is_set():
        title = random.choice(WRITE_TITLES) + f" #{random.randint(1000, 9999)}"
        body  = random.choice(WRITE_BODIES)
        tags  = random.choice(WRITE_TAGS)
        try:
            r = session.post(
                f"{API}/articles/",
                json={"article": {
                    "title": title,
                    "description": body[:60],
                    "body": body,
                    "tagList": tags,
                }},
                timeout=15,
            )
            with stats_lock:
                if r.status_code == 201:
                    stats["writes_ok"] += 1
                else:
                    stats["writes_err"] += 1
        except Exception:
            with stats_lock:
                stats["writes_err"] += 1

        time.sleep(random.uniform(0.5, 2.0))   # ~0.5–2 s between writes per thread


def read_worker(stop_event):
    """Continuously GET article lists and individual articles until stop_event."""
    session = requests.Session()

    while not stop_event.is_set():
        try:
            # 1. list articles (simulates homepage / feed)
            r = session.get(
                f"{API}/articles?limit=20&offset={random.randint(0,100)}",
                timeout=10,
            )
            if r.status_code == 200:
                with stats_lock:
                    stats["reads_ok"] += 1
                articles = r.json().get("articles", [])

                # 2. read one random article from the list
                if articles:
                    slug = random.choice(articles)["slug"]
                    r2 = session.get(f"{API}/articles/{slug}", timeout=10)
                    with stats_lock:
                        if r2.status_code == 200:
                            stats["reads_ok"] += 1
                        else:
                            stats["reads_err"] += 1
            else:
                with stats_lock:
                    stats["reads_err"] += 1
        except Exception:
            with stats_lock:
                stats["reads_err"] += 1

        time.sleep(random.uniform(0.1, 0.5))   # reads are faster/more frequent


# ── reporter ─────────────────────────────────────────────────────────────────

def reporter(stop_event, duration_s, start_time):
    """Print a stats line every 60 seconds."""
    prev = {"writes_ok": 0, "reads_ok": 0}
    interval = 60
    deadline = start_time + timedelta(seconds=duration_s)

    while not stop_event.is_set():
        time.sleep(interval)
        now = datetime.now()
        elapsed = (now - start_time).total_seconds()
        remaining = max(0, (deadline - now).total_seconds())

        with stats_lock:
            snap = dict(stats)

        w_delta = snap["writes_ok"]  - prev["writes_ok"]
        r_delta = snap["reads_ok"]   - prev["reads_ok"]
        prev["writes_ok"]  = snap["writes_ok"]
        prev["reads_ok"]   = snap["reads_ok"]

        print(
            f"[{now.strftime('%H:%M:%S')}] "
            f"elapsed {int(elapsed//60):3d}m  "
            f"writes {snap['writes_ok']:6d} (+{w_delta:4d}/min  err={snap['writes_err']})  "
            f"reads  {snap['reads_ok']:6d} (+{r_delta:4d}/min  err={snap['reads_err']})  "
            f"remaining {int(remaining//60)}m",
            flush=True,
        )


# ── main ─────────────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--duration", type=int, default=4*3600,
                        help="workload duration in seconds (default: 14400 = 4h)")
    parser.add_argument("--writers", type=int, default=4,
                        help="number of write threads (default: 4)")
    parser.add_argument("--readers", type=int, default=8,
                        help="number of read threads (default: 8)")
    args = parser.parse_args()

    print(f"=== Workload starting — duration {args.duration}s "
          f"({args.duration/3600:.1f}h), "
          f"{args.writers} writers, {args.readers} readers ===")

    # Login users and distribute tokens across write workers
    print("Logging in users...")
    tokens = []
    for u in USERS:
        t = login(u["email"], u["password"])
        if t:
            tokens.append(t)
            print(f"  ✓ {u['email']}")
        else:
            print(f"  ✗ {u['email']} (login failed)")

    if not tokens:
        print("No tokens — check that the backend is running.")
        return

    stop_event = threading.Event()
    start_time = datetime.now()
    threads = []

    # Write threads — round-robin tokens
    for i in range(args.writers):
        t = threading.Thread(
            target=write_worker,
            args=(tokens[i % len(tokens)], stop_event),
            daemon=True,
        )
        t.start()
        threads.append(t)

    # Read threads
    for _ in range(args.readers):
        t = threading.Thread(target=read_worker, args=(stop_event,), daemon=True)
        t.start()
        threads.append(t)

    # Reporter thread
    rep = threading.Thread(
        target=reporter,
        args=(stop_event, args.duration, start_time),
        daemon=True,
    )
    rep.start()

    print(f"Workload running. Ctrl-C to stop early. Auto-stop at "
          f"{(start_time + timedelta(seconds=args.duration)).strftime('%H:%M:%S')}.\n")

    try:
        time.sleep(args.duration)
    except KeyboardInterrupt:
        print("\nInterrupted by user.")

    stop_event.set()
    time.sleep(2)   # let threads finish current request

    elapsed = (datetime.now() - start_time).total_seconds()
    with stats_lock:
        snap = dict(stats)

    print(f"\n=== Workload finished after {int(elapsed)}s ===")
    print(f"  Writes   OK={snap['writes_ok']}  ERR={snap['writes_err']}")
    print(f"  Reads    OK={snap['reads_ok']}   ERR={snap['reads_err']}")
    total_ops = snap['writes_ok'] + snap['reads_ok']
    print(f"  Total ops: {total_ops}  avg TPS: {total_ops/elapsed:.1f}")


if __name__ == "__main__":
    main()
