"""Delivery-callback receiver (success and failure) for the local stack.

POST <any path>  -> stores and prints the callback, answers CALLBACK_STATUS (default 204).
GET  /events     -> JSON list of received callbacks, newest first (used by the web app).
DELETE /events   -> clears the list.

Set CALLBACK_STATUS=500 to exercise the requeue path.
"""

import json
import os
import threading
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

STATUS = int(os.getenv("CALLBACK_STATUS", "204"))
MAX_EVENTS = 100

events = []
events_lock = threading.Lock()


class CallbackHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        raw = self.rfile.read(int(self.headers.get("Content-Length", 0)))
        try:
            body = json.loads(raw)
        except json.JSONDecodeError:
            body = raw.decode(errors="replace")

        event = {
            "receivedAt": datetime.now(timezone.utc).isoformat(),
            "path": self.path,
            "authorization": self.headers.get("Authorization"),
            "status": STATUS,
            "body": body,
        }
        with events_lock:
            events.insert(0, event)
            del events[MAX_EVENTS:]

        print(json.dumps(event, indent=2, ensure_ascii=False))
        self.send_response(STATUS)
        self.end_headers()

    def do_GET(self):
        if self.path != "/events":
            self.send_error(404)
            return
        with events_lock:
            payload = json.dumps(events).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def do_DELETE(self):
        if self.path != "/events":
            self.send_error(404)
            return
        with events_lock:
            events.clear()
        self.send_response(204)
        self.end_headers()

    def log_message(self, *args):
        pass


if __name__ == "__main__":
    print(f"callback server listening on :9099, answering {STATUS}")
    ThreadingHTTPServer(("0.0.0.0", 9099), CallbackHandler).serve_forever()
