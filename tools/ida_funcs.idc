#include <idc.idc>

// 把資料庫裡所有函式的邊界匯出成純文字，供後續用程式分析。
//
//   tools/ida.sh script ida_funcs.idc md_mm2.i64
//
// 輸出 workplace/ida/out/funcs.txt，一行一個：起點<TAB>終點<TAB>名稱
//
// 為什麼要匯出而不是每次問 IDA：一次分析要對幾十個位址查「屬於哪支函式」，
// 每次都開一輪 IDA 太慢，而函式邊界是分析結果、一次匯出就能重複使用。
// headless 的 print 不進 stdout，一律 fopen 寫檔。

static main() {
    auto fh, ea, n;

    auto_wait();
    fh = fopen("/work/out/funcs.txt", "w");

    n = 0;
    ea = get_next_func(0);
    while (ea != BADADDR) {
        fprintf(fh, "%X\t%X\t%s\n", ea, get_func_attr(ea, FUNCATTR_END), get_func_name(ea));
        n = n + 1;
        ea = get_next_func(ea);
    }
    fclose(fh);

    fh = fopen("/work/out/funcs_count.txt", "w");
    fprintf(fh, "%d\n", n);
    fclose(fh);
    qexit(0);
}
