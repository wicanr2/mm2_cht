#include <idc.idc>

// 列出 Amiga 版 `mm2` 裡「檔名字串」的所有交叉參照。
//
//   tools/ida.sh script ida_amiga_names.idc amiga_mm2.i64
//
// 為什麼要走 IDA 而不是自己掃位元組：這支程式用 A4 當小資料模型的基底暫存器，
// 檔名幾乎都是 `(d16,A4)` 存取。A4 的實際值要靠載入器決定，自己掃位元組時
// **A4 猜錯就整批對不上**，而且錯得很安靜（掃出來的位址全部落在字串中間，
// 看起來像「有命中」）。IDA 的 xref 圖是分析結果，不是猜測。
//
// 輸出到 workplace/ida/out/names.txt。headless 的 print 不進 stdout。
// IDC 的 `auto` 宣告必須全部放在函式開頭。
static main() {
    auto fh, i, ea, x, n, names, addrs;

    auto_wait();

    // 字串表在 0x85C2..0x8716，以 NUL 分隔，順序就是遊戲用的順序。
    addrs = 0x85C2;

    fh = fopen("/work/out/names.txt", "w");

    ea = addrs;
    while (ea < 0x8716) {
        create_strlit(ea, BADADDR);
        fprintf(fh, "%04X  %-12s", ea, get_strlit_contents(ea, -1, 0));

        n = 0;
        x = get_first_dref_to(ea);
        while (x != BADADDR) {
            fprintf(fh, "  <- %06X(%s)", x, get_func_name(x));
            n = n + 1;
            x = get_next_dref_to(ea, x);
        }
        if (n == 0) {
            fprintf(fh, "  <- 無 xref");
        }
        fprintf(fh, "\n");

        ea = ea + strlen(get_strlit_contents(ea, -1, 0)) + 1;
    }

    fclose(fh);
    qexit(0);
}
