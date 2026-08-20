# 攻略／說明書網站的產生器環境。
#
# 為什麼另開一個 image：`mm2-go` 與 `mm2-pil` 裡都沒有 markdown 轉換器，
# 而網站要的是「markdown → HTML」這一步。裝在這裡，產生器就不必自己
# 手寫一個 markdown 剖析器 —— 手寫的那種會在表格、巢狀清單與行內
# 標記上安靜地出錯，而錯法是「少一行」，看起來像原稿就沒寫。
FROM python:3.12-slim
RUN pip install --no-cache-dir 'markdown==3.7' 'pygments==2.18.0'
WORKDIR /src
