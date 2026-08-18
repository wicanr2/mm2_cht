#!/usr/bin/env python3
"""把一個目錄封成 zip，**明確打開 UTF-8 旗標**並保留 Unix 權限。

為什麼不是 `zip -r`：三個要求 Info-ZIP 的預設值都不給。

1. **UTF-8 旗標（general purpose bit 11）**。zip 只在檔名超出 CP437 時才打開它，
   全 ASCII 的檔名不會標。沒標的話，解壓端只能用系統預設編碼去猜 ——
   在繁中 Windows 上就是 CP950，往後只要包裡出現一個中文檔名就會變亂碼。
   這裡對**每一筆**都打開，包括全 ASCII 的那些。
2. **Unix 權限**。macOS 的 `.app` 解開之後 `Contents/MacOS/` 底下那支必須還是
   可執行檔，權限掉了就變成「打不開」。權限放在 external_attr 的高 16 位元。
3. **可重現**。時間戳固定、走訪順序排序，同一份輸入永遠得到同一個位元組序列。

用法：pack_zip.py <來源目錄> <輸出 zip> [壓縮層級]
壓進去的最上層是來源目錄本身的名字。
"""
import os
import stat
import sys
import zipfile

UTF8 = 0x800
# zip 的時間戳沒有 1980 以前，固定用它的下界。
FIXED = (1980, 1, 1, 0, 0, 0)


def entries(root: str):
    """由淺到深、同層依名字排序，走訪出 (絕對路徑, 壓縮內路徑)。"""
    base = os.path.dirname(os.path.abspath(root))
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames.sort()
        for name in sorted(dirnames + filenames):
            full = os.path.join(dirpath, name)
            yield full, os.path.relpath(full, base)


def add(zf: zipfile.ZipFile, full: str, arc: str, level: int) -> None:
    st = os.lstat(full)
    is_link = stat.S_ISLNK(st.st_mode)
    is_dir = stat.S_ISDIR(st.st_mode)
    info = zipfile.ZipInfo(arc + ("/" if is_dir else ""), FIXED)
    info.create_system = 3  # Unix，否則 external_attr 的權限不會被採信
    info.external_attr = (st.st_mode & 0xFFFF) << 16
    if is_dir:
        info.external_attr |= 0x10  # MS-DOS 的目錄旗標，Windows 端看這個
    if is_link:
        zf.writestr(info, os.readlink(full))
        return
    if is_dir:
        zf.writestr(info, b"")
        return
    info.compress_type = zipfile.ZIP_DEFLATED
    with open(full, "rb") as f:
        data = f.read()
    zf.writestr(info, data, compresslevel=level)


def force_utf8(path: str) -> int:
    """事後把每一筆的 UTF-8 旗標點亮。

    **不能在寫入時設。** `ZipFile._open_to_write` 會把 `flag_bits` 歸零再自己填，
    寫進去的旗標一定被蓋掉（自己的驗收就是這樣抓到的）。所以改成寫完再回頭
    改位元組：以中央目錄為準走一遍，順手把它指到的區域檔頭一起改，
    兩處都要 —— 只改中央目錄的話，逐筆串流解壓的工具還是看不到旗標。
    """
    with open(path, "r+b") as f:
        b = bytearray(f.read())
        eocd = b.rfind(b"PK\x05\x06")
        if eocd < 0:
            raise SystemExit("找不到 EOCD")
        count = int.from_bytes(b[eocd + 10:eocd + 12], "little")
        off = int.from_bytes(b[eocd + 16:eocd + 20], "little")
        for _ in range(count):
            if bytes(b[off:off + 4]) != b"PK\x01\x02":
                raise SystemExit("中央目錄格式不符")
            flags = int.from_bytes(b[off + 8:off + 10], "little") | UTF8
            b[off + 8:off + 10] = flags.to_bytes(2, "little")
            local = int.from_bytes(b[off + 42:off + 46], "little")
            if bytes(b[local:local + 4]) != b"PK\x03\x04":
                raise SystemExit("區域檔頭格式不符")
            lf = int.from_bytes(b[local + 6:local + 8], "little") | UTF8
            b[local + 6:local + 8] = lf.to_bytes(2, "little")
            n = int.from_bytes(b[off + 28:off + 30], "little")
            m = int.from_bytes(b[off + 30:off + 32], "little")
            k = int.from_bytes(b[off + 32:off + 34], "little")
            off += 46 + n + m + k
        f.seek(0)
        f.write(b)
    return count


def main() -> None:
    root, out = sys.argv[1].rstrip("/"), sys.argv[2]
    level = int(sys.argv[3]) if len(sys.argv) > 3 else 9
    os.makedirs(os.path.dirname(os.path.abspath(out)), exist_ok=True)
    with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED) as zf:
        for full, arc in entries(root):
            add(zf, full, arc, level)
    force_utf8(out)
    # 驗收自己剛寫的東西：每一筆都要有 UTF-8 旗標，否則當場失敗。
    with zipfile.ZipFile(out) as zf:
        bad = [i.filename for i in zf.infolist() if not i.flag_bits & UTF8]
        if bad:
            raise SystemExit(f"有 {len(bad)} 筆沒有 UTF-8 旗標：{bad[:3]}")
        n = len(zf.infolist())
    print(f"[zip] {out}（{n} 筆，全部標了 UTF-8）")


if __name__ == "__main__":
    main()
