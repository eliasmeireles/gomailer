"""Publishes test messages to the mailer queue through the RabbitMQ management API.

Usage (inside the compose "publisher" service):  publish.py <message> [<message> ...]

Each <message> is /messages/<message>.json. $MAIL_ID gets a new UUID per publish; $MAIL_FROM, $MAIL_TO, $MAIL_CC, $MAIL_BCC,
$CALLBACK_SUCCESS_URL and $CALLBACK_FAILURE_URL in the template are replaced from the environment
(set by `make dev-publish FROM=... TO=... CC=... BCC=...`).
"""

import base64
import json
import os
import sys
import urllib.request
import uuid
from string import Template

API_URL = os.getenv("RABBITMQ_API_URL", "http://rabbitmq:15672")
QUEUE = os.getenv("RABBITMQ_QUEUE", "mailer-service")
CREDENTIALS = base64.b64encode(b"guest:guest").decode()


def render(name):
    with open(f"/messages/{name}.json", encoding="utf-8") as template:
        values = {
            "MAIL_ID": str(uuid.uuid4()),
            "MAIL_FROM": os.getenv("MAIL_FROM", "no-reply@exemplo.com.br"),
            "MAIL_TO": os.getenv("MAIL_TO", "maria@exemplo.com.br"),
            "MAIL_CC": os.getenv("MAIL_CC", ""),
            "MAIL_BCC": os.getenv("MAIL_BCC", ""),
            "CALLBACK_SUCCESS_URL": os.getenv("CALLBACK_SUCCESS_URL", "http://callback:9099/success"),
            "CALLBACK_FAILURE_URL": os.getenv("CALLBACK_FAILURE_URL", "http://callback:9099/failures"),
        }
        return Template(template.read()).substitute(values)


def publish(payload):
    request = urllib.request.Request(
        f"{API_URL}/api/exchanges/%2F/amq.default/publish",
        method="POST",
        data=json.dumps({
            "properties": {"content_type": "application/json"},
            "routing_key": QUEUE,
            "payload": payload,
            "payload_encoding": "string",
        }).encode(),
        headers={"Content-Type": "application/json", "Authorization": f"Basic {CREDENTIALS}"},
    )
    with urllib.request.urlopen(request) as response:
        return json.load(response).get("routed", False)


if __name__ == "__main__":
    names = sys.argv[1:] or ["success"]
    for name in names:
        message = render(name)
        subject = json.loads(message).get("subject")
        print(f"{name}: routed={publish(message)} subject={subject!r}")
