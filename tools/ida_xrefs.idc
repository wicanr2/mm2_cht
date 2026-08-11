#include <idc.idc>

// 列出某個位址的所有交叉參照（呼叫者），連同呼叫者所屬的函式。
//
//   tools/ida.sh script ida_xrefs.idc amiga_mm2_ok.i64 254FC
//
// 為什麼不自己掃位元組：`jsr`／`bsr` 的位元組樣式在資料裡也會出現，
// 逐位址掃會掃出一堆落在指令中間的假命中（踩過：把 `moveq #0,d0`
// 認成 `jsr`，然後往整個錯的函式追）。IDA 的 xref 是分析結果，
// 已經知道哪裡是指令邊界。
//
// 輸出到 workplace/ida/out/xrefs.txt。headless 的 print 不進 stdout。
static main() {
    auto fh, ea, x, n;

    auto_wait();
    ea = xtol(ARGV[1]);

    fh = fopen("/work/out/xrefs.txt", "w");
    fprintf(fh, "; xrefs → 0x%X (%s)\n", ea, get_func_name(ea));

    n = 0;
    x = get_first_cref_to(ea);
    while (x != BADADDR) {
        fprintf(fh, "%06X  %-24s  %s\n", x, get_func_name(x), GetDisasm(x));
        n = n + 1;
        x = get_next_cref_to(ea, x);
    }

    x = get_first_dref_to(ea);
    while (x != BADADDR) {
        fprintf(fh, "%06X  (資料) %-18s %s\n", x, get_func_name(x), GetDisasm(x));
        n = n + 1;
        x = get_next_dref_to(ea, x);
    }

    fprintf(fh, "; 共 %d 筆\n", n);
    fclose(fh);
    qexit(0);
}
