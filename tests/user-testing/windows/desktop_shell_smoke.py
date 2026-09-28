from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

from pywinauto import Application


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Verify the visible Reasonix Windows desktop shell through UI Automation."
    )
    parser.add_argument("--exe", required=True, type=Path, help="Path to kwu-desktop-dev.exe")
    parser.add_argument("--title", default="云智企办", help="Expected window title")
    parser.add_argument("--timeout", default=20, type=int, help="Attach timeout in seconds")
    parser.add_argument(
        "--exercise-window",
        action="store_true",
        help="Minimize and restore the window before reporting success",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    executable = args.exe.resolve()
    if not executable.is_file():
        raise FileNotFoundError(f"Desktop executable does not exist: {executable}")

    app = Application(backend="uia").connect(path=str(executable), timeout=args.timeout)
    window = app.top_window()
    window.wait("exists visible enabled ready", timeout=args.timeout)

    if args.exercise_window:
        window.minimize()
        time.sleep(0.5)
        window.restore()
        window.set_focus()
        window.wait("visible enabled ready", timeout=args.timeout)

    rectangle = window.rectangle()
    actual_title = window.window_text()
    evidence = {
        "executable": str(executable),
        "processId": app.process,
        "handle": window.handle,
        "title": actual_title,
        "visible": window.is_visible(),
        "enabled": window.is_enabled(),
        "respondingRectangle": {
            "left": rectangle.left,
            "top": rectangle.top,
            "right": rectangle.right,
            "bottom": rectangle.bottom,
            "width": rectangle.width(),
            "height": rectangle.height(),
        },
        "windowExercise": args.exercise_window,
    }

    if actual_title != args.title:
        raise AssertionError(f"Expected window title {args.title!r}, got {actual_title!r}")
    if not evidence["visible"] or not evidence["enabled"]:
        raise AssertionError(f"Desktop window is not ready: {evidence}")
    if rectangle.width() < 640 or rectangle.height() < 480:
        raise AssertionError(f"Desktop window is unexpectedly small: {evidence}")

    print(json.dumps(evidence, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
