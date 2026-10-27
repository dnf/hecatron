"""Downloads in installers and hooks retry transient failures (the curl
wrapper in languages/_common.sh and languages/_hooks.sh), and don't retry a
404, which is an answer."""

from __future__ import annotations

import http.server
import os
import shutil
import subprocess
import threading
import unittest

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


class _FlakyServer(http.server.ThreadingHTTPServer):
    """Answers /flaky with 503 twice, then 200; /missing with 404."""

    def __init__(self):
        super().__init__(("127.0.0.1", 0), _Handler)
        self.hits: dict[str, int] = {}


class _Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        hits = self.server.hits
        hits[self.path] = hits.get(self.path, 0) + 1
        if self.path == "/flaky" and hits[self.path] <= 2:
            self.send_response(503)
            self.end_headers()
        elif self.path == "/flaky":
            body = b"payload\n"
            self.send_response(200)
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
        else:
            self.send_response(404)
            self.end_headers()

    def log_message(self, *args):
        pass


@unittest.skipUnless(shutil.which("curl"), "needs curl")
class DownloadRetryTest(unittest.TestCase):
    def setUp(self):
        self.server = _FlakyServer()
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.base = f"http://127.0.0.1:{self.server.server_address[1]}"

    def tearDown(self):
        self.server.shutdown()
        self.server.server_close()

    def fetch(self, helpers: str, path: str) -> subprocess.CompletedProcess:
        return subprocess.run(["bash", "-c", f'source "{helpers}"; curl -fsS "{self.base}{path}"'],
                              capture_output=True, text=True, timeout=120)

    def check_helpers(self, helpers: str):
        result = self.fetch(helpers, "/flaky")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, "payload\n")
        self.assertEqual(self.server.hits["/flaky"], 3)
        result = self.fetch(helpers, "/missing")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.server.hits["/missing"], 1)

    def test_installers_retry_transient_failures_only(self):
        self.check_helpers(os.path.join(ROOT, "languages", "_common.sh"))

    def test_hooks_retry_transient_failures_only(self):
        self.check_helpers(os.path.join(ROOT, "languages", "_hooks.sh"))


if __name__ == "__main__":
    unittest.main()
