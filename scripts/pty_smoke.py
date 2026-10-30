#!/usr/bin/env python3
"""Exercise the real Linux terminal application using only Python's stdlib."""

import errno
import fcntl
import json
import os
from pathlib import Path
import pty
import select
import signal
import struct
import sys
import tempfile
import termios
import time


class Terminal:
    def __init__(self, executable, state_dir):
        self.output = bytearray()
        self.exited = False
        self.pid, self.fd = pty.fork()
        if self.pid == 0:
            fcntl.ioctl(0, termios.TIOCSWINSZ, struct.pack("HHHH", 24, 80, 0, 0))
            env = dict(os.environ, XDG_STATE_HOME=str(state_dir), TERM="xterm-256color", COLORTERM="truecolor")
            env.pop("NO_COLOR", None)
            os.execve(str(executable), [str(executable)], env)

    def read(self):
        ready, _, _ = select.select([self.fd], [], [], 0.03)
        if ready:
            try:
                self.output.extend(os.read(self.fd, 65536))
            except OSError as error:
                if error.errno != errno.EIO:
                    raise

    def expect(self, text, timeout=20):
        target = text.encode()
        until = time.monotonic() + timeout
        while target not in self.output:
            if time.monotonic() > until:
                raise AssertionError(f"Missing {text!r}; terminal tail: {self.output[-2000:]!r}")
            self.read()

    def send(self, keys):
        os.write(self.fd, keys.encode())

    def resize(self, columns, rows):
        fcntl.ioctl(self.fd, termios.TIOCSWINSZ, struct.pack("HHHH", rows, columns, 0, 0))
        os.kill(self.pid, signal.SIGWINCH)

    def finish(self):
        until = time.monotonic() + 10
        while True:
            pid, status = os.waitpid(self.pid, os.WNOHANG)
            if pid:
                self.exited = True
                assert os.waitstatus_to_exitcode(status) == 0, self.output[-2000:]
                attrs = termios.tcgetattr(self.fd)
                assert attrs[3] & termios.ICANON, "terminal left in raw mode"
                assert attrs[3] & termios.ECHO, "terminal echo was not restored"
                return
            if time.monotonic() > until:
                raise AssertionError("Game did not finish saving and quit")
            self.read()

    def close(self):
        if not self.exited:
            os.kill(self.pid, signal.SIGKILL)
            os.waitpid(self.pid, 0)
        os.close(self.fd)


def wait_save(path, predicate):
    until = time.monotonic() + 10
    while time.monotonic() < until:
        if path.exists():
            data = json.loads(path.read_text())
            if predicate(data):
                return data
        time.sleep(0.02)
    raise AssertionError("Expected game state was not saved")


def main():
    executable = Path(sys.argv[1] if len(sys.argv) > 1 else "./sudoku").resolve()
    with tempfile.TemporaryDirectory(prefix="sudoku-pty-") as directory:
        path = Path(directory) / "sudoku-term" / "game.json"
        terminal = Terminal(executable, directory)
        try:
            terminal.expect("New game")
            terminal.send("\r")
            terminal.expect("Choose your challenge")
            terminal.send("\r")
            terminal.expect("ANSWER")
            saved = wait_save(path, lambda data: True)
            original_givens = saved["givens"]
            cursor = saved["cursor"]
            correct = saved["solution"][cursor]
            wrong = correct % 9 + 1
            terminal.send(f"n{correct}n{wrong}")
            saved = wait_save(path, lambda data: data["board"][cursor]["value"] == wrong)
            assert saved["board"][cursor]["notes"] == 0
            assert len(saved["history"]) == 2
            terminal.send("q")
            terminal.finish()
        finally:
            terminal.close()

        terminal = Terminal(executable, directory)
        try:
            terminal.expect("Resume game")
            terminal.send("\r")
            terminal.expect("ANSWER")
            terminal.send("u")
            saved = wait_save(path, lambda data: data["board"][cursor]["value"] == 0)
            assert saved["board"][cursor]["notes"] == 1 << (correct - 1)
            terminal.send("\x12")  # Ctrl+R
            wait_save(path, lambda data: data["board"][cursor]["value"] == wrong)
            terminal.send(f"{correct}i")
            # The renderer may reuse the A already present in ANSWER.
            terminal.expect("WAY FORWARD")
            prior_history = len(wait_save(path, lambda data: data["board"][cursor]["value"] == correct)["history"])
            terminal.send("\r")
            saved = wait_save(path, lambda data: len(data["history"]) > prior_history)
            assert saved["givens"] == original_givens
            terminal.resize(100, 45)
            terminal.expect("═════")
            terminal.resize(60, 20)
            terminal.expect("Resize to at least")
            terminal.resize(80, 24)
            time.sleep(0.1)
            terminal.send("\x1b")
            terminal.expect("Take your time")
            terminal.send("q")
            terminal.finish()
        finally:
            terminal.close()

        terminal = Terminal(executable, directory)
        try:
            terminal.expect("Resume game")
            terminal.send("\r")
            terminal.expect("ANSWER")
            os.kill(terminal.pid, signal.SIGTERM)
            terminal.finish()
            final = json.loads(path.read_text())
            assert final["board"] == saved["board"]
            assert final["history"] == saved["history"]
        finally:
            terminal.close()

    print("PTY smoke passed: new game, notes, wrong answer, resume, undo/redo, hint, resize, quit, SIGTERM, terminal restoration")


if __name__ == "__main__":
    main()
