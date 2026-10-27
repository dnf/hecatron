"""Run libexec/lib/audit.py's and outdated.py's report on each case read as
JSON from stdin, as their main() does, and print what each printed: the
Go port's parity tests (parity_test.go) compare it with internal/deps."""

import contextlib
import io
import json
import os
import sys

HOME = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
sys.path.insert(0, os.path.join(HOME, "libexec", "lib"))

import audit  # noqa: E402
import outdated  # noqa: E402


def run(case):
    out, err = io.StringIO(), io.StringIO()
    os.environ.pop("HECATRON_AUDIT_FAIL_ON", None)
    if case.get("failOn"):
        os.environ["HECATRON_AUDIT_FAIL_ON"] = case["failOn"]
    text = case["text"]
    uncaught = ""
    try:
        with contextlib.redirect_stdout(out):
            if case["kind"] == "audit":
                status = audit.report(case["format"], text, "lang", "comp", case.get("config", {}), case["today"])
            else:
                status = outdated.report(case["format"], text, "lang", "comp", case.get("direct") or None,
                                         None, bool(case.get("fail")))
    except (ValueError, KeyError, TypeError, AttributeError) as error:
        print(f"error: lang: couldn't read the {case['format']} report ({error}); its output was:", file=err)
        print(text[:4000], file=err)
        status = 1
    except Exception as error:  # noqa: BLE001 - main() would end in a traceback
        out = io.StringIO()
        uncaught = type(error).__name__
        status = 1
    return {"out": out.getvalue(), "err": err.getvalue(), "status": status, "uncaught": uncaught}


print(json.dumps([run(case) for case in json.load(sys.stdin)]))
