#include <idc.idc>

// 把一段位址的反組譯寫成純文字。
//
//   tools/ida.sh script ida_func.idc 1MENU2.img.i64 18624 300
//
// 參數是**十六進位的起始位址**與**要走幾個位元組**（十進位）。
// 輸出到 workplace/ida/out/func_<起始>.txt。
//
// 為什麼要有這一支：`.i64` 與整份 `.asm` 都 gitignore，所以「當初怎麼看到
// 那段碼」必須留在可以重跑的腳本裡，不能只留結論。
//
// 三件已經踩過的事（別再踩）：
//   - headless 的 print 不進 stdout，一律 fopen 寫檔。不寫檔＝沒跑，
//     而 exit code 照樣是 0。
//   - 舊名（MakeCode／MinEA／GetFlags）在 IDA 9.4 的 IDC 會讓整支腳本
//     編譯失敗，症狀是 exit 1 加零輸出。只用 create_insn／GetDisasm 這一批。
//   - 位址沒被分析成指令時 GetDisasm 會回資料，先 create_insn 再讀。
//   - IDC 的 `auto` 宣告必須全部放在函式開頭。寫在迴圈裡同樣是編譯失敗，
//     一樣是 exit 0 加零輸出，看起來像「跑了但沒找到東西」。
static main() {
    auto fh, ea, end, n, sz;

    auto_wait();

    ea = xtol(ARGV[1]);
    n = xtol(ARGV[2]);
    if (n <= 0) {
        n = 0x100;
    }
    end = ea + n;

    fh = fopen("/work/out/func.txt", "w");
    fprintf(fh, "; 0x%X..0x%X\n", ea, end);

    while (ea < end) {
        create_insn(ea);
        sz = get_item_size(ea);
        if (sz < 1) {
            sz = 1;
        }
        fprintf(fh, "%05X  %s\n", ea, GetDisasm(ea));
        ea = ea + sz;
    }
    fclose(fh);
    qexit(0);
}
