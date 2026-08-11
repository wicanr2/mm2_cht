#include <idc.idc>

// 把裸 68000 映像從指定位址開始強制當成程式碼分析，再輸出整份 .asm。
//
//   tools/ida.sh script ida_m68k_code.idc amiga_mm2.i64 0
//
// 為什麼需要這一支：IDA 載入裸 68000 檔時**預設把開頭當成例外向量表**
// （`initial_sp`／`initial_pc`／`bus_error`…），於是前 1 KB 被標成資料，
// 自動分析找不到進入點，整份檔案只解出零星幾段 —— 症狀是 `.asm` 有五萬行
// 卻一條 `(d16,A4)` 都沒有。
//
// Amiga 的 `mm2` 不是這樣：它是被 `playmm2` 載入後直接跳進去的裸程式碼，
// offset 0 就是第一條指令（`lea ($8622,A4),A0`）。
//
// 輸出到 workplace/ida/out/code.asm。headless 的 print 不進 stdout，
// 一律 fopen 寫檔 —— 不寫檔＝沒跑，而 exit code 照樣是 0。
//
// IDC 的 `auto` 宣告必須全部放在函式開頭，寫在迴圈裡是編譯失敗
// （exit 0 加零輸出，看起來像跑了但沒找到東西）。
static main() {
    auto fh, ea, n, r;

    auto_wait();

    ea = xtol(ARGV[1]);

    // 先把向量表那一段的既有標記清掉，否則 create_insn 會被擋。
    del_items(ea, DELIT_EXPAND, 0x400);
    auto_wait();

    r = create_insn(ea);
    add_func(ea, BADADDR);
    auto_wait();

    // 讓自動分析把能追到的都追完。
    n = 0;
    while (n < 3) {
        auto_wait();
        n = n + 1;
    }

    fh = fopen("/work/out/code.txt", "w");
    fprintf(fh, "create_insn(0x%X) = %d\n", ea, r);
    fprintf(fh, "func_name = %s\n", get_func_name(ea));
    fprintf(fh, "first insn = %s\n", GetDisasm(ea));
    fclose(fh);

    fh = fopen("/work/out/code.asm", "w");
    gen_file(OFILE_ASM, fh, 0, BADADDR, 0);
    fclose(fh);
    qexit(0);
}
