#!/usr/bin/env python3
"""
New-API Relay Server (Python)
A lightweight API relay platform for LLM API aggregation.
Runs all the business enhancement endpoints designed for the Go backend.
"""

import json
import os
import sys
import time
import uuid
import hashlib
import sqlite3
import threading
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse, parse_qs
from datetime import datetime, timezone

# ── Config ──────────────────────────────────────────
PORT = int(os.environ.get("PORT", 3000))
START_TIME = int(time.time())
VERSION = "v0.0.0-enhanced"
DB_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "data", "new-api.db")
os.makedirs(os.path.dirname(DB_PATH), exist_ok=True)

# ── Metrics ─────────────────────────────────────────
metrics = {
    "total_requests": 0,
    "successful_requests": 0,
    "failed_requests": 0,
    "total_prompt_tokens": 0,
    "total_completion_tokens": 0,
    "rate_limited_requests": 0,
    "active_connections": 0,
    "total_latency_ms": 0,
    "channel_requests": {},
    "channel_errors": {},
}
metrics_lock = threading.Lock()

# ── Database ─────────────────────────────────────────
def get_db():
    db = sqlite3.connect(DB_PATH)
    db.row_factory = sqlite3.Row
    db.execute("PRAGMA journal_mode=WAL")
    db.execute("PRAGMA foreign_keys=ON")
    return db

def init_db():
    db = get_db()
    db.executescript("""
        CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            username TEXT UNIQUE NOT NULL,
            password TEXT NOT NULL,
            display_name TEXT DEFAULT '',
            role INTEGER DEFAULT 1,
            status INTEGER DEFAULT 1,
            email TEXT DEFAULT '',
            quota INTEGER DEFAULT 0,
            used_quota INTEGER DEFAULT 0,
            group_name TEXT DEFAULT 'default',
            created_at INTEGER DEFAULT 0
        );
        CREATE TABLE IF NOT EXISTS tokens (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER NOT NULL,
            key TEXT UNIQUE NOT NULL,
            status INTEGER DEFAULT 1,
            name TEXT DEFAULT '',
            remain_quota INTEGER DEFAULT 0,
            unlimited_quota INTEGER DEFAULT 0,
            expired_time INTEGER DEFAULT -1,
            allow_ips TEXT DEFAULT '',
            group_name TEXT DEFAULT '',
            scopes TEXT DEFAULT '["read","write"]',
            rate_limit_rpm INTEGER DEFAULT 0,
            rotation_key TEXT DEFAULT '',
            rotation_expires_at INTEGER DEFAULT 0,
            model_limits_enabled INTEGER DEFAULT 0,
            model_limits TEXT DEFAULT '',
            created_time INTEGER DEFAULT 0
        );
        CREATE TABLE IF NOT EXISTS workspaces (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL,
            slug TEXT UNIQUE NOT NULL,
            description TEXT DEFAULT '',
            logo TEXT DEFAULT '',
            owner_id INTEGER NOT NULL,
            status INTEGER DEFAULT 1,
            plan TEXT DEFAULT 'free',
            settings TEXT DEFAULT '{}',
            created_at INTEGER DEFAULT 0,
            updated_at INTEGER DEFAULT 0
        );
        CREATE TABLE IF NOT EXISTS workspace_members (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            workspace_id INTEGER NOT NULL,
            user_id INTEGER NOT NULL,
            role TEXT DEFAULT 'member',
            display_name TEXT DEFAULT '',
            joined_at INTEGER DEFAULT 0,
            UNIQUE(workspace_id, user_id)
        );
        CREATE TABLE IF NOT EXISTS workspace_invitations (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            workspace_id INTEGER NOT NULL,
            inviter_id INTEGER NOT NULL,
            email TEXT DEFAULT '',
            role TEXT DEFAULT 'member',
            token TEXT UNIQUE,
            status TEXT DEFAULT 'pending',
            expires_at INTEGER DEFAULT 0,
            created_at INTEGER DEFAULT 0
        );
        CREATE TABLE IF NOT EXISTS payment_gateways (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL,
            type TEXT NOT NULL,
            enabled INTEGER DEFAULT 1,
            is_default INTEGER DEFAULT 0,
            config TEXT DEFAULT '{}',
            sort_order INTEGER DEFAULT 0,
            created_at INTEGER DEFAULT 0,
            updated_at INTEGER DEFAULT 0
        );
        CREATE TABLE IF NOT EXISTS payment_transactions (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER NOT NULL,
            gateway_id INTEGER DEFAULT 0,
            gateway_type TEXT DEFAULT '',
            transaction_no TEXT UNIQUE,
            order_no TEXT DEFAULT '',
            amount REAL DEFAULT 0,
            currency TEXT DEFAULT 'USD',
            quota_amount INTEGER DEFAULT 0,
            description TEXT DEFAULT '',
            status TEXT DEFAULT 'pending',
            payment_method TEXT DEFAULT '',
            raw_notification TEXT DEFAULT '',
            completed_at INTEGER DEFAULT 0,
            created_at INTEGER DEFAULT 0,
            updated_at INTEGER DEFAULT 0
        );
        CREATE TABLE IF NOT EXISTS model_catalogs (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            model_name TEXT UNIQUE NOT NULL,
            display_name TEXT NOT NULL,
            provider TEXT DEFAULT '',
            description TEXT DEFAULT '',
            long_description TEXT DEFAULT '',
            icon TEXT DEFAULT '',
            category TEXT DEFAULT 'chat',
            tags TEXT DEFAULT '',
            pricing_info TEXT DEFAULT '{}',
            capabilities TEXT DEFAULT '{}',
            doc_url TEXT DEFAULT '',
            status INTEGER DEFAULT 1,
            featured INTEGER DEFAULT 0,
            sort_order INTEGER DEFAULT 0,
            created_at INTEGER DEFAULT 0,
            updated_at INTEGER DEFAULT 0
        );
        CREATE TABLE IF NOT EXISTS model_categories (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT UNIQUE NOT NULL,
            display_name TEXT NOT NULL,
            description TEXT DEFAULT '',
            icon TEXT DEFAULT '',
            sort_order INTEGER DEFAULT 0,
            created_at INTEGER DEFAULT 0
        );
        CREATE TABLE IF NOT EXISTS model_tags (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT UNIQUE NOT NULL,
            color TEXT DEFAULT '#6366f1',
            usage_count INTEGER DEFAULT 0,
            created_at INTEGER DEFAULT 0
        );
        CREATE TABLE IF NOT EXISTS logs (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER NOT NULL,
            token_id INTEGER DEFAULT 0,
            model_name TEXT DEFAULT '',
            quota INTEGER DEFAULT 0,
            prompt_tokens INTEGER DEFAULT 0,
            completion_tokens INTEGER DEFAULT 0,
            use_time INTEGER DEFAULT 0,
            is_stream INTEGER DEFAULT 0,
            channel_id INTEGER DEFAULT 0,
            created_at INTEGER DEFAULT 0,
            type INTEGER DEFAULT 2,
            channel_name TEXT DEFAULT '',
            username TEXT DEFAULT '',
            token_name TEXT DEFAULT '',
            content TEXT DEFAULT ''
        );
        CREATE TABLE IF NOT EXISTS audit_logs (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER DEFAULT 0,
            username TEXT DEFAULT '',
            action TEXT NOT NULL,
            resource TEXT DEFAULT '',
            resource_id INTEGER DEFAULT 0,
            detail TEXT DEFAULT '',
            ip_address TEXT DEFAULT '',
            user_agent TEXT DEFAULT '',
            created_at INTEGER DEFAULT 0
        );
        CREATE TABLE IF NOT EXISTS monitor_alert_rules (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL,
            description TEXT DEFAULT '',
            metric_type TEXT NOT NULL,
            condition TEXT NOT NULL,
            threshold REAL DEFAULT 0,
            duration INTEGER DEFAULT 300,
            enabled INTEGER DEFAULT 1,
            notify_channels TEXT DEFAULT '[]',
            last_triggered INTEGER DEFAULT 0,
            created_at INTEGER DEFAULT 0,
            updated_at INTEGER DEFAULT 0
        );
        CREATE TABLE IF NOT EXISTS monitor_alert_events (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            rule_id INTEGER NOT NULL,
            rule_name TEXT DEFAULT '',
            metric_type TEXT DEFAULT '',
            value REAL DEFAULT 0,
            threshold REAL DEFAULT 0,
            message TEXT DEFAULT '',
            acknowledged INTEGER DEFAULT 0,
            acknowledged_by INTEGER DEFAULT 0,
            created_at INTEGER DEFAULT 0
        );
    """)
    # Seed default data
    _seed(db)
    db.commit()
    db.close()

def _seed(db):
    # Check if root user exists
    cur = db.execute("SELECT id FROM users WHERE role = 100")
    if not cur.fetchone():
        pwd_hash = hashlib.sha256("123456".encode()).hexdigest()
        db.execute(
            "INSERT INTO users (username, password, display_name, role, status, quota, created_at) VALUES (?,?,?,?,?,?,?)",
            ("root", pwd_hash, "Root User", 100, 1, 100000000, int(time.time()))
        )
        print("[seed] Created root user (root / 123456)")

    # Seed model categories
    cats = db.execute("SELECT COUNT(*) FROM model_categories").fetchone()
    if cats[0] == 0:
        for cat in [
            ("chat", "Chat", "Conversational AI models"),
            ("image", "Image", "Image generation models"),
            ("audio", "Audio", "Audio/Speech models"),
            ("embedding", "Embedding", "Text embedding models"),
            ("video", "Video", "Video generation models"),
            ("rerank", "Rerank", "Reranking models"),
        ]:
            db.execute("INSERT INTO model_categories (name, display_name, description, created_at) VALUES (?,?,?,?)",
                       (cat[0], cat[1], cat[2], int(time.time())))

    # Seed model catalog
    models = db.execute("SELECT COUNT(*) FROM model_catalogs").fetchone()
    if models[0] == 0:
        for m in [
            ("gpt-4o", "GPT-4o", "OpenAI", "OpenAI flagship model", "chat", 1, 1),
            ("gpt-4o-mini", "GPT-4o Mini", "OpenAI", "Cost-efficient small model", "chat", 1, 2),
            ("claude-sonnet-4-20250514", "Claude Sonnet 4", "Anthropic", "Anthropic balanced model", "chat", 1, 3),
            ("claude-opus-4-20250514", "Claude Opus 4", "Anthropic", "Anthropic most powerful", "chat", 1, 4),
            ("gemini-2.5-flash", "Gemini 2.5 Flash", "Google", "Google fast model", "chat", 1, 5),
            ("deepseek-chat", "DeepSeek Chat", "DeepSeek", "DeepSeek chat model", "chat", 1, 6),
            ("qwen-max", "Qwen Max", "Alibaba", "Alibaba most capable", "chat", 1, 7),
            ("dall-e-3", "DALL-E 3", "OpenAI", "Image generation", "image", 0, 8),
            ("whisper-1", "Whisper", "OpenAI", "Speech-to-text", "audio", 0, 9),
            ("text-embedding-3-large", "Embedding 3 Large", "OpenAI", "Text embeddings", "embedding", 0, 10),
        ]:
            db.execute(
                "INSERT INTO model_catalogs (model_name, display_name, provider, description, category, featured, sort_order, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?)",
                (m[0], m[1], m[2], m[3], m[4], m[5], m[6], int(time.time()), int(time.time()))
            )

    # Seed payment gateways
    gws = db.execute("SELECT COUNT(*) FROM payment_gateways").fetchone()
    if gws[0] == 0:
        for gw in [
            ("Stripe", "stripe", 1, 1, '{"api_key":"sk_test_...","webhook_secret":"whsec_..."}', 1),
            ("Alipay", "alipay", 1, 0, '{"app_id":"...","private_key":"..."}', 2),
            ("WeChat Pay", "wechat_pay", 0, 0, '{"app_id":"...","mch_id":"..."}', 3),
            ("PayPal", "paypal", 1, 0, '{"client_id":"...","secret":"..."}', 4),
        ]:
            db.execute(
                "INSERT INTO payment_gateways (name, type, enabled, is_default, config, sort_order, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?)",
                (gw[0], gw[1], gw[2], gw[3], gw[4], gw[5], int(time.time()), int(time.time()))
            )

    # Seed default test token for root user
    tokens = db.execute("SELECT COUNT(*) FROM tokens").fetchone()
    if tokens[0] == 0:
        user = db.execute("SELECT id FROM users WHERE role = 100").fetchone()
        if user:
            now = int(time.time())
            db.execute(
                "INSERT INTO tokens (user_id, key, name, status, remain_quota, unlimited_quota, created_time, expired_time, scopes) VALUES (?,?,?,?,?,?,?,?,?)",
                (user["id"], "sk-test-token-1", "Default Key", 1, 100000, 1, now, -1, '["read","write"]')
            )
            print("[seed] Created default test token: sk-test-token-1")

# ── Auth ────────────────────────────────────────────
def authenticate_token(handler):
    """Validate API token from Authorization header"""
    auth = handler.headers.get("Authorization", "")
    key = auth.replace("Bearer ", "").replace("bearer ", "").strip()

    if not key:
        # Check x-api-key header
        key = handler.headers.get("x-api-key", "")
        if not key:
            return None, "Token not provided"

    # Keep original key for lookup (DB stores full key with sk- prefix)
    db = get_db()
    token = db.execute("SELECT * FROM tokens WHERE key = ?", (key,)).fetchone()
    if not token:
        # Try without sk- prefix
        if key.startswith("sk-"):
            clean_key = key[3:]
            token = db.execute("SELECT * FROM tokens WHERE key = ?", (clean_key,)).fetchone()
    if not token:
        # Check rotation key
        now = int(time.time())
        token = db.execute(
            "SELECT * FROM tokens WHERE rotation_key = ? AND rotation_expires_at > ?",
            (key, now)
        ).fetchone()
    db.close()

    if not token:
        return None, "Invalid token"
    if token["status"] != 1:
        return None, "Token disabled/expired"

    # Get user
    db = get_db()
    user = db.execute("SELECT * FROM users WHERE id = ?", (token["user_id"],)).fetchone()
    db.close()

    if not user or user["status"] != 1:
        return None, "User banned or not found"

    return {"token": dict(token), "user": dict(user)}, None

def authenticate_session(handler):
    """Session-based auth (simplified for demo)"""
    # In production, use JWT or session cookies
    # For demo: check X-User-Id header
    user_id = handler.headers.get("X-User-Id", "")
    if not user_id:
        return None, "Not logged in"

    db = get_db()
    user = db.execute("SELECT * FROM users WHERE id = ?", (int(user_id),)).fetchone()
    db.close()

    if not user or user["status"] != 1:
        return None, "User not found or disabled"

    return {"user": dict(user)}, None

# ── HTTP Handler ─────────────────────────────────────
class RelayHandler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        pass  # suppress default logging

    def _send_json(self, data, status=200):
        body = json.dumps(data, ensure_ascii=False, default=str).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-User-Id")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _send_text(self, text, status=200, content_type="text/plain"):
        body = text.encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _read_body(self):
        length = int(self.headers.get("Content-Length", 0))
        if length > 0:
            return json.loads(self.rfile.read(length))
        return {}

    def _record_metric(self, success=True, prompt_tokens=0, completion_tokens=0, channel_id=0, latency_ms=0):
        with metrics_lock:
            metrics["total_requests"] += 1
            if success:
                metrics["successful_requests"] += 1
            else:
                metrics["failed_requests"] += 1
            metrics["total_prompt_tokens"] += prompt_tokens
            metrics["total_completion_tokens"] += completion_tokens
            if latency_ms > 0:
                if metrics["total_requests"] > 1:
                    metrics["avg_latency_ms"] = (metrics["avg_latency_ms"] * (metrics["total_requests"] - 1) + latency_ms) / metrics["total_requests"]
                else:
                    metrics["avg_latency_ms"] = float(latency_ms)

    def do_OPTIONS(self):
        self._send_json({})

    # ── Routing ──────────────────────────────────
    def do_GET(self):
        self._route("GET")

    def do_POST(self):
        self._route("POST")

    def do_PUT(self):
        self._route("PUT")

    def do_DELETE(self):
        self._route("DELETE")

    def _route(self, method):
        path = urlparse(self.path).path.rstrip("/")
        qs = parse_qs(urlparse(self.path).query)

        routes = {
            # Health & Metrics
            ("GET", "/api/status"): self.handle_status,
            ("GET", "/api/health"): self.handle_health,
            ("GET", "/api/metrics"): self.handle_metrics,

            # Auth (for testing/development)
            ("POST", "/api/auth/login"): self.handle_login,

            # Model Catalog (Public)
            ("GET", "/api/model-catalog"): self.handle_model_catalog,
            ("GET", "/api/model-catalog/featured"): self.handle_featured_models,
            ("GET", "/api/model-catalog/search"): self.handle_search_models,
            ("GET", "/api/model-catalog/categories"): self.handle_model_categories,
            ("GET", "/api/model-catalog/tags"): self.handle_model_tags,

            # Payment
            ("GET", "/api/payment/methods"): self.require_auth(self.handle_payment_methods, "session"),
            ("POST", "/api/payment/create"): self.require_auth(self.handle_payment_create, "session"),

            # Workspace
            ("GET", "/api/workspaces"): self.require_auth(self.handle_list_workspaces, "session"),
            ("POST", "/api/workspaces"): self.require_auth(self.handle_create_workspace, "session"),

            # Token Enhanced
            ("GET", "/api/token/usage-summary"): self.require_auth(self.handle_token_usage_summary, "session"),

            # Monitoring (Admin)
            ("GET", "/api/admin/audit-logs"): self.require_auth(self.handle_audit_logs, "session", min_role=10),
            ("GET", "/api/admin/monitor/metrics"): self.require_auth(self.handle_monitor_metrics, "session", min_role=10),
            ("GET", "/api/admin/monitor/alerts/rules"): self.require_auth(self.handle_alert_rules, "session", min_role=10),
            ("GET", "/api/admin/monitor/alerts/events"): self.require_auth(self.handle_alert_events, "session", min_role=10),

            # ── Relay Endpoints (Token Auth) ──────────────────
            ("POST", "/v1/chat/completions"): self.require_auth(self.handle_chat_completions, "token"),
            ("GET", "/v1/models"): self.require_auth(self.handle_list_models, "token"),
        }

        # Handle model catalog detail: /api/model-catalog/{id}
        if method == "GET" and path.startswith("/api/model-catalog/") and len(path.split("/")) == 4:
            model_id = path.split("/")[-1]
            if model_id.isdigit():
                self.handle_model_detail(int(model_id))
                return

        handler = routes.get((method, path))
        if handler:
            handler()
        else:
            self._record_metric(success=False)
            self._send_json({"success": False, "message": f"Not found: {method} {path}"}, 404)

    def require_auth(self, handler, auth_type="token", min_role=1):
        def wrapper():
            if auth_type == "token":
                result, err = authenticate_token(self)
            else:
                result, err = authenticate_session(self)

            if err:
                self._send_json({"success": False, "message": err}, 401)
                return

            user = result["user"]
            token = result.get("token")

            if user["role"] < min_role:
                self._send_json({"success": False, "message": "Insufficient privileges"}, 403)
                return

            # Store auth context
            self.user = user
            self.token = token
            handler()
        return wrapper

    # ── Health & Metrics ─────────────────────────────
    def handle_status(self):
        db = get_db()
        users = db.execute("SELECT COUNT(*) FROM users").fetchone()[0]
        tokens = db.execute("SELECT COUNT(*) FROM tokens").fetchone()[0]
        db.close()
        self._send_json({
            "success": True,
            "data": {
                "version": VERSION,
                "uptime": int(time.time()) - START_TIME,
                "users": users,
                "tokens": tokens,
                "start_time": START_TIME,
            }
        })

    def handle_health(self):
        self._send_json({
            "status": "ok",
            "version": VERSION,
            "uptime": int(time.time()) - START_TIME,
        })

    def handle_metrics(self):
        with metrics_lock:
            m = dict(metrics)
            m["uptime"] = int(time.time()) - START_TIME

        lines = [
            "# HELP newapi_uptime_seconds Gateway uptime in seconds",
            "# TYPE newapi_uptime_seconds gauge",
            f"newapi_uptime_seconds {m['uptime']}",
            "# HELP newapi_requests_total Total requests",
            "# TYPE newapi_requests_total counter",
            f"newapi_requests_total {m['total_requests']}",
            "# HELP newapi_requests_successful_total Successful requests",
            "# TYPE newapi_requests_successful_total counter",
            f"newapi_requests_successful_total {m['successful_requests']}",
            "# HELP newapi_requests_failed_total Failed requests",
            "# TYPE newapi_requests_failed_total counter",
            f"newapi_requests_failed_total {m['failed_requests']}",
            "# HELP newapi_prompt_tokens_total Total prompt tokens",
            "# TYPE newapi_prompt_tokens_total counter",
            f"newapi_prompt_tokens_total {m['total_prompt_tokens']}",
            "# HELP newapi_completion_tokens_total Total completion tokens",
            "# TYPE newapi_completion_tokens_total counter",
            f"newapi_completion_tokens_total {m['total_completion_tokens']}",
            "# HELP newapi_active_connections Active connections",
            "# TYPE newapi_active_connections gauge",
            f"newapi_active_connections {m['active_connections']}",
        ]
        self._send_text("\n".join(lines) + "\n")

    # ── Model Catalog ─────────────────────────────────
    def handle_model_catalog(self):
        db = get_db()
        category = parse_qs(urlparse(self.path).query).get("category", [None])[0]
        if category:
            rows = db.execute("SELECT * FROM model_catalogs WHERE status=1 AND category=? ORDER BY featured DESC, sort_order ASC", (category,)).fetchall()
        else:
            rows = db.execute("SELECT * FROM model_catalogs WHERE status=1 ORDER BY featured DESC, sort_order ASC").fetchall()
        db.close()
        self._send_json({"success": True, "data": [dict(r) for r in rows], "total": len(rows)})

    def handle_featured_models(self):
        db = get_db()
        rows = db.execute("SELECT * FROM model_catalogs WHERE featured=1 AND status=1 ORDER BY sort_order LIMIT 20").fetchall()
        db.close()
        self._send_json({"success": True, "data": [dict(r) for r in rows]})

    def handle_model_detail(self, model_id):
        db = get_db()
        row = db.execute("SELECT * FROM model_catalogs WHERE id = ?", (model_id,)).fetchone()
        db.close()
        if not row:
            self._send_json({"success": False, "message": "model not found"}, 404)
            return
        data = dict(row)
        data["pricing"] = json.loads(data.get("pricing_info", "{}"))
        data["capabilities"] = json.loads(data.get("capabilities", "{}"))
        self._send_json({"success": True, "data": {"model": data, "pricing": data["pricing"], "capabilities": data["capabilities"]}})

    def handle_search_models(self):
        q = parse_qs(urlparse(self.path).query).get("q", [""])[0]
        if not q:
            self._send_json({"success": False, "message": "search query required"}, 400)
            return
        db = get_db()
        pattern = f"%{q}%"
        rows = db.execute(
            "SELECT * FROM model_catalogs WHERE status=1 AND (model_name LIKE ? OR display_name LIKE ? OR description LIKE ? OR provider LIKE ?) ORDER BY featured DESC",
            (pattern, pattern, pattern, pattern)
        ).fetchall()
        db.close()
        self._send_json({"success": True, "data": [dict(r) for r in rows]})

    def handle_model_categories(self):
        db = get_db()
        rows = db.execute("SELECT * FROM model_categories ORDER BY sort_order ASC").fetchall()
        db.close()
        self._send_json({"success": True, "data": [dict(r) for r in rows]})

    def handle_model_tags(self):
        db = get_db()
        rows = db.execute("SELECT * FROM model_tags ORDER BY usage_count DESC").fetchall()
        db.close()
        self._send_json({"success": True, "data": [dict(r) for r in rows]})

    # ── Payment ───────────────────────────────────────
    def handle_payment_methods(self):
        db = get_db()
        rows = db.execute("SELECT id, name, type, is_default FROM payment_gateways WHERE enabled=1 ORDER BY sort_order").fetchall()
        db.close()
        self._send_json({"success": True, "data": [dict(r) for r in rows]})

    # ── Workspace ─────────────────────────────────────
    def handle_list_workspaces(self):
        db = get_db()
        rows = db.execute(
            "SELECT w.* FROM workspaces w JOIN workspace_members wm ON wm.workspace_id = w.id WHERE wm.user_id = ?",
            (self.user["id"],)
        ).fetchall()
        db.close()
        self._send_json({"success": True, "data": [dict(r) for r in rows]})

    # ── Audit Logs (Admin) ────────────────────────────
    def handle_audit_logs(self):
        page = int(parse_qs(urlparse(self.path).query).get("page", ["1"])[0])
        limit = 20
        offset = (page - 1) * limit
        db = get_db()
        total = db.execute("SELECT COUNT(*) FROM audit_logs").fetchone()[0]
        rows = db.execute("SELECT * FROM audit_logs ORDER BY id DESC LIMIT ? OFFSET ?", (limit, offset)).fetchall()
        db.close()
        self._send_json({"success": True, "data": [dict(r) for r in rows], "total": total})

    # ── Monitor Metrics (Admin) ───────────────────────
    def handle_monitor_metrics(self):
        with metrics_lock:
            self._send_json({
                "success": True,
                "data": {
                    "total_requests": metrics["total_requests"],
                    "successful_requests": metrics["successful_requests"],
                    "failed_requests": metrics["failed_requests"],
                    "total_prompt_tokens": metrics["total_prompt_tokens"],
                    "total_completion_tokens": metrics["total_completion_tokens"],
                    "rate_limited": metrics["rate_limited_requests"],
                    "active_connections": metrics["active_connections"],
                    "uptime_seconds": int(time.time()) - START_TIME,
                }
            })

    def handle_alert_rules(self):
        db = get_db()
        rows = db.execute("SELECT * FROM monitor_alert_rules ORDER BY id DESC").fetchall()
        db.close()
        self._send_json({"success": True, "data": [dict(r) for r in rows]})

    def handle_alert_events(self):
        db = get_db()
        rows = db.execute("SELECT * FROM monitor_alert_events ORDER BY id DESC LIMIT 50").fetchall()
        db.close()
        self._send_json({"success": True, "data": [dict(r) for r in rows]})

    # ── Auth ──────────────────────────────────────────
    def handle_login(self):
        data = self._read_body()
        username = data.get("username", "")
        password = data.get("password", "")
        import hashlib
        pwd_hash = hashlib.sha256(password.encode()).hexdigest()
        db = get_db()
        user = db.execute("SELECT * FROM users WHERE username = ? AND password = ?",
                         (username, pwd_hash)).fetchone()
        db.close()
        if not user:
            self._send_json({"success": False, "message": "Invalid credentials"}, 401)
            return
        # Set session cookie
        import uuid
        session_id = str(uuid.uuid4())
        self._send_json({
            "success": True,
            "data": {
                "id": user["id"],
                "username": user["username"],
                "role": user["role"],
                "session_id": session_id
            }
        })

    # ── Payment ───────────────────────────────────────
    def handle_payment_create(self):
        db = get_db()
        db.execute(
            "INSERT INTO payment_transactions (user_id, gateway_type, amount, currency, description, status, created_at) VALUES (?,?,?,?,?,?,?)",
            (self.user["id"], "stripe", 10.0, "USD", "Test payment", "pending", int(time.time()))
        )
        db.commit()
        db.close()
        self._send_json({"success": True, "message": "Payment order created"})

    # ── Workspace ─────────────────────────────────────
    def handle_create_workspace(self):
        data = self._read_body()
        name = data.get("name", "My Workspace")
        slug = data.get("slug", f"ws-{int(time.time())}")
        db = get_db()
        cursor = db.execute(
            "INSERT INTO workspaces (name, slug, description, owner_id, status, plan, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?)",
            (name, slug, "", self.user["id"], 1, "free", int(time.time()), int(time.time()))
        )
        ws_id = cursor.lastrowid
        db.execute("INSERT INTO workspace_members (workspace_id, user_id, role, joined_at) VALUES (?,?,?,?)",
                   (ws_id, self.user["id"], "owner", int(time.time())))
        db.commit()
        db.close()
        self._send_json({"success": True, "data": {"id": ws_id, "name": name, "slug": slug}})

    # ── Token Enhanced ────────────────────────────────
    def handle_token_usage_summary(self):
        db = get_db()
        rows = db.execute(
            "SELECT t.id as token_id, t.name as token_name, COALESCE(SUM(l.quota),0) as quota, COUNT(l.id) as requests FROM tokens t LEFT JOIN logs l ON l.token_id = t.id WHERE t.user_id = ? GROUP BY t.id",
            (self.user["id"],)
        ).fetchall()
        db.close()
        self._send_json({"success": True, "data": [dict(r) for r in rows]})

    # ── Relay: Chat Completions ───────────────────────
    def handle_chat_completions(self):
        data = self._read_body()
        model = data.get("model", "gpt-4o")
        messages = data.get("messages", [])
        stream = data.get("stream", False)

        # Simulate token usage
        prompt_tokens = len(json.dumps(messages)) // 4
        completion_tokens = 100

        self._record_metric(
            success=True,
            prompt_tokens=prompt_tokens,
            completion_tokens=completion_tokens,
            channel_id=self.token.get("id", 1) if self.token else 1,
            latency_ms=50
        )

        # Log to DB
        db = get_db()
        db.execute(
            "INSERT INTO logs (user_id, token_id, model_name, quota, prompt_tokens, completion_tokens, use_time, is_stream, channel_id, created_at, type) VALUES (?,?,?,?,?,?,?,?,?,?,?)",
            (self.user["id"], self.token["id"] if self.token else 1, model, 10,
             prompt_tokens, completion_tokens, 50, int(stream), 1, int(time.time()), 2)
        )
        db.commit()
        db.close()

        response = {
            "id": "chatcmpl-" + str(int(time.time())),
            "object": "chat.completion",
            "created": int(time.time()),
            "model": model,
            "choices": [{
                "index": 0,
                "message": {
                    "role": "assistant",
                    "content": f"[Relay Demo] This is a simulated response for model '{model}'. Your request was successfully processed through the API relay. (Prompt: {prompt_tokens} tokens, Completion: {completion_tokens} tokens)"
                },
                "finish_reason": "stop"
            }],
            "usage": {
                "prompt_tokens": prompt_tokens,
                "completion_tokens": completion_tokens,
                "total_tokens": prompt_tokens + completion_tokens
            }
        }
        self._send_json(response)

    # ── Relay: List Models ────────────────────────────
    def handle_list_models(self):
        db = get_db()
        rows = db.execute("SELECT * FROM model_catalogs WHERE status = 1 ORDER BY sort_order").fetchall()
        db.close()
        models = []
        for r in rows:
            models.append({
                "id": r["model_name"],
                "object": "model",
                "created": r["created_at"],
                "owned_by": r["provider"]
            })
        self._send_json({"object": "list", "data": models})


# ── Server ────────────────────────────────────────────
class ThreadedHTTPServer(HTTPServer):
    allow_reuse_address = True
    daemon_threads = True

def main():
    print(f"""
╔══════════════════════════════════════════════════════╗
║           New-API Relay Server (Enhanced)            ║
╠══════════════════════════════════════════════════════╣
║  Port:     {PORT:<41}║
║  Version:  {VERSION:<41}║
║  DB:       SQLite ({DB_PATH})║
╠══════════════════════════════════════════════════════╣
║  Endpoints:                                         ║
║    GET  /api/status          System status           ║
║    GET  /api/health          Health check            ║
║    GET  /api/metrics         Prometheus metrics      ║
║    GET  /api/model-catalog   Model marketplace       ║
║    GET  /api/payment/methods Payment methods         ║
║    GET  /api/workspaces      User workspaces         ║
║    GET  /api/admin/audit-logs   Audit log (admin)    ║
║    GET  /api/admin/monitor/metrics  Monitor metrics  ║
╚══════════════════════════════════════════════════════╝
""")

    print("[init] Initializing database...")
    init_db()
    print(f"[init] Database ready at {DB_PATH}")

    server = ThreadedHTTPServer(("0.0.0.0", PORT), RelayHandler)
    print(f"[serve] Listening on http://0.0.0.0:{PORT}")
    print(f"[serve] Try: curl http://localhost:{PORT}/api/status")
    print()

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\n[shutdown] Server stopped.")
        server.shutdown()

if __name__ == "__main__":
    main()
