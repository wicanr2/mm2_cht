#include <idc.idc>

// 對一組位址印出「它屬於哪支函式」與「那支函式被誰呼叫」。
//
//   tools/ida.sh script ida_context.idc md_mm2.i64
//
// 位址清單寫在下面的 main() 裡（這是針對本專案音樂場景對照的一次性分析，
// 位址就是分析對象本身，寫在腳本裡才留得住「當初查了哪幾個」）。
//
// 為什麼不自己掃位元組找函式邊界：`link a6` 的位元組樣式在資料裡也會出現，
// 往回掃會掃到別的函式中間。IDA 的函式邊界是分析結果，而 xref 圖已經知道
// 誰呼叫誰 —— 自己解析指令文字只會重造一個比較差的版本。
//
// 三件踩過的事：headless 的 print 不進 stdout，一律 fopen 寫檔（不寫檔＝沒跑，
// 而 exit code 照樣是 0）；IDC 的 auto 宣告要全部放在函式開頭；
// 舊名（MinEA／GetFlags 這一批）在 IDA 9.4 的 IDC 會讓整支腳本編譯失敗。

static dump(fh, ea, tag) {
    auto f, fend, x, n, cnt;

    f = get_func_attr(ea, FUNCATTR_START);
    fend = get_func_attr(ea, FUNCATTR_END);

    fprintf(fh, "\n=== %s：呼叫點 0x%06X ===\n", tag, ea);
    if (f == BADADDR) {
        fprintf(fh, "  不在任何已辨識的函式裡（IDA 沒把這段當程式碼）\n");
        return;
    }
    fprintf(fh, "  所屬函式 %s  0x%06X..0x%06X（%d bytes）\n",
            get_func_name(f), f, fend, fend - f);

    cnt = 0;
    x = get_first_cref_to(f);
    while (x != BADADDR) {
        fprintf(fh, "    被 0x%06X 呼叫  (%s)  %s\n", x, get_func_name(x), GetDisasm(x));
        cnt = cnt + 1;
        x = get_next_cref_to(f, x);
    }
    if (cnt == 0) {
        fprintf(fh, "    沒有任何呼叫者（可能是進入點，或 IDA 沒解出來）\n");
    }
}

static main() {
    auto fh;

    auto_wait();
    fh = fopen("/work/out/context.txt", "w");

    // sub_B620（選曲分派）的 11 個呼叫端，括號內是傳進去的 case。
    dump(fh, 0x006F94, "case 7");
    dump(fh, 0x00706C, "case 10");
    dump(fh, 0x008962, "case 2");
    dump(fh, 0x00A51E, "case 10");
    dump(fh, 0x00EC30, "case 2");
    dump(fh, 0x00ECC4, "case 2");
    dump(fh, 0x010446, "case 0");
    dump(fh, 0x0108B4, "case 1");
    dump(fh, 0x010962, "case 0");
    dump(fh, 0x010FDA, "case 19");
    dump(fh, 0x0115DC, "case 9");

    // 選曲分派本身，以及設定 case 19 分支變數的那一處。
    dump(fh, 0x00B620, "sub_B620 選曲分派");
    dump(fh, 0x010F14, "寫 -$4C6(a5) 的地方");

    fclose(fh);
    qexit(0);
}
