"""Fake Resend-compatible email API for the local stack (MAILER_ENV=resend-mock).

POST   /emails  -> Resend "send email": fails while failNext > 0 (with the configured status,
                   name and message, like a real Resend error), otherwise accepts and records it.
GET    /state   -> current behavior and counters.
PUT    /state   -> updates failNext, status, name and/or message (JSON body).
DELETE /state   -> resets behavior, counters and recorded emails.
"""

import json
import threading
import uuid
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

EXPECTED_TOKEN = "Bearer re_dev_mock"
MAX_EMAILS = 50
DEFAULT_STATE = {"failNext": 0, "status": 503, "name": "service_unavailable", "message": "Mock provider unavailable"}

lock = threading.Lock()
state = dict(DEFAULT_STATE, accepted=0, rejected=0, emails=[])


class MockHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        if self.path != "/emails":
            return self.reply(404, {"name": "not_found", "message": "Unknown endpoint"})
        if self.headers.get("Authorization") != EXPECTED_TOKEN:
            return self.reply(401, {"statusCode": 401, "name": "validation_error", "message": "API key is invalid"})

        payload = self.read_json()
        with lock:
            if state["failNext"] > 0:
                state["failNext"] -= 1
                state["rejected"] += 1
                status = state["status"]
                return self.reply(status, {"statusCode": status, "name": state["name"], "message": state["message"]})

            email_id = str(uuid.uuid4())
            state["accepted"] += 1
            state["emails"].insert(0, {
                "id": email_id,
                "receivedAt": datetime.now(timezone.utc).isoformat(),
                "from": payload.get("from"),
                "to": payload.get("to"),
                "cc": payload.get("cc"),
                "bcc": payload.get("bcc"),
                "subject": payload.get("subject"),
                "attachments": len(payload.get("attachments") or []),
            })
            del state["emails"][MAX_EMAILS:]
        self.reply(200, {"id": email_id})

    def do_GET(self):
        if self.path != "/state":
            return self.reply(404, {"error": "not found"})
        with lock:
            self.reply(200, state)

    def do_PUT(self):
        if self.path != "/state":
            return self.reply(404, {"error": "not found"})
        update = self.read_json()
        with lock:
            for key in DEFAULT_STATE:
                if key in update:
                    state[key] = type(DEFAULT_STATE[key])(update[key])
            self.reply(200, state)

    def do_DELETE(self):
        if self.path != "/state":
            return self.reply(404, {"error": "not found"})
        with lock:
            state.update(DEFAULT_STATE, accepted=0, rejected=0, emails=[])
            self.reply(200, state)

    def read_json(self):
        raw = self.rfile.read(int(self.headers.get("Content-Length", 0)))
        try:
            return json.loads(raw or b"{}")
        except json.JSONDecodeError:
            return {}

    def reply(self, status, body):
        payload = json.dumps(body).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def log_message(self, fmt, *args):
        print(f"{self.command} {self.path} -> {args[1] if len(args) > 1 else ''}", flush=True)


if __name__ == "__main__":
    print("mock email API listening on :9100", flush=True)
    ThreadingHTTPServer(("0.0.0.0", 9100), MockHandler).serve_forever()
