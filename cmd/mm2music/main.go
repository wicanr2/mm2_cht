// mm2music 將 MM2.EXE 尾部 DGROUP 的 PC speaker 曲目離線算成 WAV。
//
// 原版 `sub_157E0` 以 ds:5214 的十個指標讀取（音高索引、時值索引）對，
// 再查 ds:5144 的 Hz 表與 ds:51F4 的 PIT IRQ tick 表。TIMER.DRV
// 將 channel 0 設為 divisor 0x0400；每 tick = 1024/1193182 秒。輸出只應放在 gitignore
// 目錄；曲目資料仍來自玩家自備的原版 MM2.EXE。
package main

import (
	"crypto/sha256"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	dgroupBase    = 0x8630
	pitchTable    = 0x5144
	durationTable = 0x51F4
	tunePointers  = 0x5214
	sampleRate    = 48000
	pitClockHz    = 1193182
	pitDivisor    = 1024
	mm2EXESHA256  = "631facb658a39e0d438c451f8a43c9f6e2aeb774fc3843c1a9bac1e14bf8c4d4"
)

type note struct {
	hz    int
	ticks int
	// silenceSamples 是推廣片編排用的 remake 靜音，不是原版 tick 資料。
	silenceSamples int
}

func main() {
	exePath := flag.String("exe", "workplace/orig/MM2/MM2.EXE", "玩家自備 MM2.EXE")
	outPath := flag.String("out", "workplace/promo/music/mm2-original-pc-speaker.wav", "WAV 輸出")
	tunesArg := flag.String("tunes", "3,4,5,6,7,8", "依序播放的原版曲目索引（0–9）")
	seconds := flag.Int("seconds", 65, "循環曲目組合到至少這個秒數")
	flag.Parse()

	exe, err := os.ReadFile(*exePath)
	if err != nil {
		log.Fatal(err)
	}
	if err := verifyMM2EXE(exe); err != nil {
		log.Fatal(err)
	}
	tunes, err := parseTunes(*tunesArg)
	if err != nil {
		log.Fatal(err)
	}
	if *seconds <= 0 {
		log.Fatal("秒數必須大於 0")
	}
	if len(tunes) == 0 {
		log.Fatal("至少需要一個曲目")
	}
	var score []note
	for _, i := range tunes {
		ns, err := readTune(exe, i)
		if err != nil {
			log.Fatal(err)
		}
		score = append(score, ns...)
		// 曲目本來是獨立事件 cue；串成推廣片 medley 時保留一小段界線。
		score = append(score, note{silenceSamples: 250 * sampleRate / 1000})
		fmt.Printf("曲目 %d：%d 個音符，%.3f 秒\n", i, len(ns), duration(ns))
	}
	if len(score) == 0 {
		log.Fatal("沒有可輸出的曲目")
	}
	pcm := renderLoop(score, *seconds)
	if err := os.MkdirAll(dir(*outPath), 0o755); err != nil {
		log.Fatal(err)
	}
	if err := writeWAV(*outPath, pcm); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("寫入 %s：%.3f 秒，48 kHz mono PCM\n", *outPath,
		float64(len(pcm))/sampleRate)
}

func verifyMM2EXE(exe []byte) error {
	got := fmt.Sprintf("%x", sha256.Sum256(exe))
	if got != mm2EXESHA256 {
		return fmt.Errorf("MM2.EXE SHA-256 不符：得到 %s，需要 %s", got, mm2EXESHA256)
	}
	return nil
}

func parseTunes(s string) ([]int, error) {
	var out []int
	for _, p := range strings.Split(s, ",") {
		if strings.TrimSpace(p) == "" {
			return nil, fmt.Errorf("曲目索引不可為空")
		}
		i, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || i < 0 || i >= 10 {
			return nil, fmt.Errorf("曲目索引 %q 無效（應為 0–9）", p)
		}
		out = append(out, i)
	}
	return out, nil
}

func readTune(exe []byte, tune int) ([]note, error) {
	if tune < 0 || tune >= 10 {
		return nil, fmt.Errorf("曲目索引 %d 無效（應為 0–9）", tune)
	}
	ptr, err := word(exe, tunePointers+tune*2)
	if err != nil {
		return nil, err
	}
	pos := ptr
	var out []note
	for steps := 0; steps < 1024; steps++ {
		p := dgroupBase + pos
		if p < 0 || p+1 >= len(exe) {
			return nil, fmt.Errorf("曲目 %d 的 ds:%04X 超出 EXE", tune, pos)
		}
		if exe[p] == 0xFF {
			return out, nil
		}
		hz, err := word(exe, pitchTable+int(exe[p])*2)
		if err != nil {
			return nil, err
		}
		ticks, err := word(exe, durationTable+int(exe[p+1])*2)
		if err != nil {
			return nil, err
		}
		out = append(out, note{hz: hz, ticks: ticks})
		pos += 2
	}
	return nil, fmt.Errorf("曲目 %d 超過 1024 步仍無 0xFF", tune)
}

func word(exe []byte, ds int) (int, error) {
	p := dgroupBase + ds
	if p < 0 || p+2 > len(exe) {
		return 0, fmt.Errorf("DGROUP ds:%04X 超出 EXE", ds)
	}
	return int(binary.LittleEndian.Uint16(exe[p:])), nil
}

func duration(ns []note) float64 {
	ticks := 0
	for _, n := range ns {
		ticks += n.ticks
	}
	return float64(ticks*pitDivisor) / pitClockHz
}

func renderLoop(score []note, seconds int) []int16 {
	need := seconds * sampleRate
	out := make([]int16, 0, need+sampleRate)
	phase := 0.0
	tickRemainder := 0
	for len(out) < need {
		for _, n := range score {
			count := n.silenceSamples
			if n.silenceSamples == 0 {
				total := n.ticks*pitDivisor*sampleRate + tickRemainder
				count = total / pitClockHz
				tickRemainder = total % pitClockHz
			}
			for i := 0; i < count && len(out) < need; i++ {
				v := int16(0)
				if n.hz > 0 {
					phase += float64(n.hz) / sampleRate
					phase -= math.Floor(phase)
					if phase < 0.5 {
						v = 7200
					} else {
						v = -7200
					}
				}
				out = append(out, v)
			}
		}
	}
	return out
}

func writeWAV(path string, pcm []int16) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	dataSize := len(pcm) * 2
	h := make([]byte, 44)
	copy(h[0:], "RIFF")
	binary.LittleEndian.PutUint32(h[4:], uint32(36+dataSize))
	copy(h[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(h[16:], 16)
	binary.LittleEndian.PutUint16(h[20:], 1)
	binary.LittleEndian.PutUint16(h[22:], 1)
	binary.LittleEndian.PutUint32(h[24:], sampleRate)
	binary.LittleEndian.PutUint32(h[28:], sampleRate*2)
	binary.LittleEndian.PutUint16(h[32:], 2)
	binary.LittleEndian.PutUint16(h[34:], 16)
	copy(h[36:], "data")
	binary.LittleEndian.PutUint32(h[40:], uint32(dataSize))
	if _, err := f.Write(h); err != nil {
		return err
	}
	buf := make([]byte, 2*len(pcm))
	for i, v := range pcm {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(v))
	}
	_, err = f.Write(buf)
	return err
}

func dir(path string) string {
	return filepath.Dir(path)
}
