#!/usr/bin/env python3
"""
import_japanese_blogs.py
Generates 100 Japanese blog articles using the Claude API and imports them
into the realworld app running on EC2 via the realworld API.

Usage: python import_japanese_blogs.py
Requires: pip install anthropic requests
"""

import os
import openai
import requests
import json
import time
import sys

BACKEND_URL = f"http://{os.environ.get('EC2_HOST', 'localhost')}:8080/api"

# 8 seed users created by the seed service (password: password123)
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

# 10 topic batches × 10 articles = 100 articles total
TOPIC_BATCHES = [
    ["日本の伝統的なお祭り", "茶道の精神と作法", "着物の着付けと歴史", "神社仏閣の参拝マナー", "生け花の基本と美しさ",
     "歌舞伎の世界入門", "日本の四季と自然の美しさ", "武道の精神：柔道と空手", "日本庭園のデザイン哲学", "俳句の作り方と楽しみ方"],

    ["東京のグルメスポット特集", "本格ラーメンの作り方", "寿司職人の技と歴史", "家庭料理：肉じゃがレシピ", "和菓子の種類と季節感",
     "日本酒の選び方と楽しみ方", "居酒屋文化と飲み会マナー", "おにぎりの具材ベスト10", "鍋料理の種類と楽しみ方", "弁当文化と詰め方のコツ"],

    ["日本のテクノロジー最前線", "AIと日本社会の変化", "スマートシティ東京の未来", "ロボット技術の発展と課題", "電気自動車と日本の製造業",
     "宇宙開発と日本のJAXA", "半導体産業の現状と展望", "5G通信と生活の変化", "フィンテックと日本の金融", "ゲーム産業の歴史と現在"],

    ["京都観光完全ガイド", "北海道の自然と食の魅力", "沖縄の文化と海の美しさ", "富士山登山の準備と注意点", "日光の歴史と観光スポット",
     "奈良の鹿と世界遺産めぐり", "広島と平和の祈り", "鎌倉の寺社と海岸線", "金沢：加賀百万石の城下町", "大阪グルメと笑いの文化"],

    ["日本のアニメ産業の歴史", "人気漫画家の創作秘話", "コスプレ文化と海外での広がり", "声優という職業の魅力と苦労", "アニメ聖地巡礼の楽しみ方",
     "漫画の描き方入門", "ゲームキャラクターデザインの基礎", "ライトノベルの書き方と市場", "VTuberとバーチャル文化の未来", "日本のインディーゲーム開発者"],

    ["日本のビジネスマナーと礼儀", "転職市場の最新トレンド", "フリーランスとして生きる方法", "スタートアップ起業の基礎知識", "在宅勤務で生産性を上げる方法",
     "投資初心者のための株式入門", "副業で稼ぐための戦略", "管理職になるためのスキルアップ", "女性活躍推進と職場環境の変化", "終身雇用制度の崩壊と新しい働き方"],

    ["日本の環境問題と取り組み", "再生可能エネルギーの現状", "プラスチックゴミ削減への挑戦", "都市の緑化プロジェクト", "里山保全と生物多様性",
     "海洋汚染と漁業への影響", "気候変動と農業の未来", "エコツーリズムの可能性", "節電・省エネの生活習慣", "地方創生と持続可能な社会"],

    ["マインドフルネス瞑想の実践法", "日本の健康長寿の秘訣", "ジョギングを習慣にするコツ", "ヨガと日本文化の融合", "温泉・銭湯の効能と楽しみ方",
     "食事制限なしで痩せる方法", "睡眠の質を上げる生活習慣", "メンタルヘルスとセルフケア", "登山初心者のための装備ガイド", "武術から学ぶ集中力の鍛え方"],

    ["江戸時代の庶民生活", "明治維新と近代化の歴史", "第二次世界大戦後の日本の復興", "源氏物語と平安文学の世界", "戦国武将の生涯と戦略",
     "縄文時代の日本列島", "日本神話：古事記の世界", "江戸文化：浮世絵と俳諧", "幕末の志士たちの夢と現実", "昭和の高度経済成長期"],

    ["一人暮らしの節約術", "断捨離と整理整頓の極意", "猫との暮らし方と健康効果", "観葉植物で部屋を飾るコツ", "DIYインテリアで部屋をおしゃれに",
     "読書習慣をつける10のコツ", "映画鑑賞をより深く楽しむ方法", "料理初心者が最初に学ぶこと", "日記を続けるための工夫", "趣味でつながる地域コミュニティ"],
]


def login_users():
    """Login all seed users and return {email: token} dict."""
    tokens = {}
    for user in USERS:
        try:
            resp = requests.post(
                f"{BACKEND_URL}/users/login",
                json={"user": {"email": user["email"], "password": user["password"]}},
                timeout=10,
            )
            resp.raise_for_status()
            token = resp.json()["user"]["token"]
            tokens[user["email"]] = token
            print(f"  ✓ {user['email']}")
        except Exception as e:
            print(f"  ✗ {user['email']}: {e}")
    return tokens


def generate_batch(client, topics):
    """Ask GPT-4o to generate 10 Japanese blog articles for the given topics."""
    topic_list = "\n".join(f"{i+1}. {t}" for i, t in enumerate(topics))

    prompt = f"""あなたはプロの日本語ブロガーです。以下の10つのトピックについて、それぞれ日本語のブログ記事を書いてください。

トピック一覧:
{topic_list}

各記事について以下のJSON形式で出力してください:
- title: 魅力的なタイトル（日本語）
- description: 記事の概要（日本語、1〜2文）
- body: 本文（日本語、3〜4段落、各段落100〜200文字程度）
- tags: 関連タグ（2〜3個、日本語または英語）

出力はJSONの配列のみにしてください。マークダウンのコードブロックは使わないこと。
形式例:
[
  {{
    "title": "...",
    "description": "...",
    "body": "...",
    "tags": ["タグ1", "タグ2"]
  }},
  ...
]"""

    response = client.chat.completions.create(
        model="gpt-4o",
        messages=[{"role": "user", "content": prompt}],
        max_tokens=8000,
    )

    text = response.choices[0].message.content.strip()
    # Strip markdown code fences if present
    if text.startswith("```"):
        lines = text.splitlines()
        text = "\n".join(lines[1:-1] if lines[-1].strip() == "```" else lines[1:])
    return json.loads(text)


def post_article(token, article):
    """POST one article to the realworld API. Returns the created article or None."""
    try:
        resp = requests.post(
            f"{BACKEND_URL}/articles/",
            json={
                "article": {
                    "title":       article.get("title", "無題"),
                    "description": article.get("description", ""),
                    "body":        article.get("body", ""),
                    "tagList":     article.get("tags", []),
                }
            },
            headers={"Authorization": f"Token {token}"},
            timeout=15,
        )
        if resp.status_code == 201:
            return resp.json()["article"]
        print(f"    API error {resp.status_code}: {resp.text[:120]}")
        return None
    except Exception as e:
        print(f"    Request error: {e}")
        return None


def main():
    client = openai.OpenAI()  # reads OPENAI_API_KEY from environment

    # ── Step 1: login ────────────────────────────────────────────────────────
    print("=== Logging in users ===")
    tokens = login_users()
    if not tokens:
        print("No tokens — check backend URL or user credentials.")
        sys.exit(1)
    token_list = list(tokens.values())

    # ── Step 2: generate + import 10 batches × 10 articles ──────────────────
    total_imported = 0
    total_articles = 0

    for batch_idx, topics in enumerate(TOPIC_BATCHES):
        batch_num = batch_idx + 1
        print(f"\n=== Batch {batch_num}/10 — generating {len(topics)} articles with Claude ===")

        try:
            articles = generate_batch(client, topics)
        except Exception as e:
            print(f"  Generation failed: {e}")
            continue

        print(f"  Generated {len(articles)} articles — importing...")

        for i, article in enumerate(articles):
            total_articles += 1
            # Round-robin through users
            token = token_list[(total_articles - 1) % len(token_list)]
            result = post_article(token, article)
            if result:
                total_imported += 1
                slug = result.get("slug", "")
                title = result.get("title", "")[:50]
                print(f"  [{total_imported:3d}/100] {title}")
            else:
                print(f"  [{total_articles:3d}/100] FAILED: {article.get('title', '')[:50]}")

        # Brief pause between batches to be polite to the API
        if batch_idx < len(TOPIC_BATCHES) - 1:
            time.sleep(1)

    # ── Summary ──────────────────────────────────────────────────────────────
    print(f"\n=== Done: {total_imported}/{total_articles} articles imported ===")
    print(f"  Frontend: http://{os.environ.get('EC2_HOST', 'localhost')}:3001")


if __name__ == "__main__":
    main()
