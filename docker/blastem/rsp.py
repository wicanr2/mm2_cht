#!/usr/bin/env python3
"""BlastEm GDB remote stub 的客戶端。

`blastem ROM -D` 直接在 stdio 上講 GDB remote serial protocol，不需要終端機。
（原生除錯器 `-d` 靠 `termhelper` 另開終端機視窗，headless 容器裡會**靜默地
不進除錯器** —— stdin 餵命令零回應，管道與 pty 都一樣。）

四個會靜默失敗的地方，全部寫在對應的方法上：

- 第一次 `cont()` 可能要等十秒以上（開機到第一個中斷點），逾時設太短會誤判成
  「中斷點沒命中」。
- 放行要送**合法封包** `$c#63`，寫裸的 `c` 位元組會被 stub 忽略，
  模擬器就一直停著。
- stub **不回應 raw `0x03`** 非同步中斷，送了會永遠等不到回覆。
  所以要在放行之前把該讀的都讀完。
- 停在中斷點時模擬器不處理視窗事件，`xdotool` 的按鍵送不進去。
"""

from __future__ import annotations

import subprocess


class Rsp:
    """夠用的 GDB remote serial protocol 客戶端。"""

    def __init__(self, proc: subprocess.Popen):
        self.p = proc

    # --- 底層 ---

    def _read_exact(self, n: int) -> bytes:
        out = b""
        while len(out) < n:
            b = self.p.stdout.read(n - len(out))
            if not b:
                raise SystemExit("模擬器提早結束（stdout 關閉）")
            out += b
        return out

    def send(self, data: bytes) -> None:
        """送一個封包但不等回覆。放行（`c`）用這個。"""
        ck = b"%02x" % (sum(data) & 0xFF)
        self.p.stdin.write(b"$" + data + b"#" + ck)
        self.p.stdin.flush()

    def recv(self) -> bytes:
        """讀一個封包。非封包位元組（ack、模擬器印的訊息）一律略過。"""
        while True:
            if self._read_exact(1) == b"$":
                break
        body = b""
        while True:
            c = self._read_exact(1)
            if c == b"#":
                break
            body += c
        self._read_exact(2)  # checksum
        self.p.stdin.write(b"+")
        self.p.stdin.flush()
        return body

    def cmd(self, data: bytes) -> bytes:
        self.send(data)
        return self.recv()

    # --- 常用操作 ---

    def status(self) -> bytes:
        return self.cmd(b"?")

    def regs(self) -> list[int]:
        """68000 的暫存器：d0-d7、a0-a7、SR、PC。"""
        r = self.cmd(b"g")
        return [int(r[i : i + 8], 16) for i in range(0, len(r), 8)]

    def pc(self) -> int:
        return self.regs()[-1]

    def read_mem(self, addr: int, size: int) -> bytes:
        r = self.cmd(f"m{addr:x},{size}".encode())
        return bytes.fromhex(r.decode())

    def read_u32(self, addr: int) -> int:
        return int.from_bytes(self.read_mem(addr, 4), "big")

    def write_u32(self, addr: int, value: int) -> bytes:
        return self.cmd(f"M{addr:x},4:{value:08x}".encode())

    def read_cstr(self, addr: int, limit: int = 96) -> str:
        b = self.read_mem(addr, limit)
        return b.split(b"\x00")[0].decode("latin1", "replace")

    def add_break(self, addr: int) -> bytes:
        return self.cmd(f"Z0,{addr:x},2".encode())

    def del_break(self, addr: int) -> bytes:
        return self.cmd(f"z0,{addr:x},2".encode())

    def cont(self) -> bytes:
        """續跑並**等**下一次停下。第一次可能要等十秒以上。"""
        self.send(b"c")
        return self.recv()

    def release(self) -> None:
        """放行但不等 —— 要讓模擬器自由跑（例如送按鍵）時用。

        必須是合法封包；寫裸的 `c` 位元組會被 stub 忽略。
        """
        self.send(b"c")
