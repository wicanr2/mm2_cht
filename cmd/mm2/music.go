package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"

	gamemusic "github.com/wicanr2/mm2_cht/internal/music"
	"github.com/wicanr2/mm2_cht/internal/ui"
)

const musicSampleRate = 48000

// musicPlayer 只持有玩家本機音樂包的位元組。原版音檔不編入執行檔，
// manifest 與全部檔案必須先通過驗證，才會建立 audio.Context。
type musicPlayer struct {
	ctx     *audio.Context
	tracks  map[gamemusic.Role]localTrack
	current ui.MusicCue
	player  *audio.Player
	// once 是還在播的一次性音效。留著參照是必要的：不留的話播放器會被
	// 回收，聲音在一兩幀之內就斷掉。
	once []*audio.Player
}

type localTrack struct {
	ext  string
	data []byte
}

type decodedTrack interface {
	io.ReadSeeker
	Length() int64
}

// newMusicPlayer 原子載入整個音樂包。任何映射檔不存在、格式不支援或
// 無法解碼時，整包拒絕，不讓遊戲在不同場景間半套切換。
func newMusicPlayer(pack *gamemusic.Pack) (*musicPlayer, error) {
	if pack == nil || pack.Theme == gamemusic.ThemeOff {
		return nil, nil
	}
	tracks := make(map[gamemusic.Role]localTrack, len(pack.Tracks))
	for role, path := range pack.Tracks {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("讀取 %s 音樂 %q：%w", role, path, err)
		}
		ext := strings.ToLower(filepath.Ext(path))
		track := localTrack{ext: ext, data: b}
		stream, err := track.decode()
		if err != nil {
			return nil, fmt.Errorf("驗證 %s 音樂 %q：%w", role, path, err)
		}
		if stream.Length() <= 0 {
			return nil, fmt.Errorf("%s 音樂 %q 解碼後為空", role, path)
		}
		tracks[role] = track
	}
	return &musicPlayer{ctx: audio.NewContext(musicSampleRate), tracks: tracks}, nil
}

func (t localTrack) decode() (decodedTrack, error) {
	src := bytes.NewReader(t.data)
	switch t.ext {
	case ".wav":
		return wav.DecodeWithSampleRate(musicSampleRate, src)
	default:
		return nil, fmt.Errorf("不支援副檔名 %q（只接受 PCM .wav）", t.ext)
	}
}

// PlayOnce 播一次性音效（打贏、陣亡、撿到寶那些），播完就結束，
// **不影響正在播的背景音樂**。
//
// 包裡沒有這個角色就安靜跳過 —— 與 Sync 同一個原則：不拿別的曲子頂替。
func (m *musicPlayer) PlayOnce(cue ui.MusicCue) error {
	if m == nil || cue == "" {
		return nil
	}
	// 先收掉播完的，否則長時間遊玩會一直累積。
	live := m.once[:0]
	for _, p := range m.once {
		if p.IsPlaying() {
			live = append(live, p)
			continue
		}
		if err := p.Close(); err != nil {
			return err
		}
	}
	m.once = live

	track, ok := m.tracks[gamemusic.Role(cue)]
	if !ok {
		return nil
	}
	stream, err := track.decode()
	if err != nil {
		return err
	}
	p, err := m.ctx.NewPlayer(stream)
	if err != nil {
		return err
	}
	m.once = append(m.once, p)
	p.Play()
	return nil
}

// Sync 讓正常 UI 的語意角色驅動背景音樂。部分 MSX／Amiga／DOS 包若沒有
// 對應角色會明確靜音；不沿用上一場景的曲子冒充缺少的素材。
func (m *musicPlayer) Sync(cue ui.MusicCue) error {
	if m == nil || cue == m.current {
		return nil
	}
	if m.player != nil {
		if err := m.player.Close(); err != nil {
			return err
		}
		m.player = nil
	}
	m.current = cue
	track, ok := m.tracks[gamemusic.Role(cue)]
	if !ok || cue == ui.MusicCueUnknown {
		return nil
	}
	stream, err := track.decode()
	if err != nil {
		return err
	}
	loop := audio.NewInfiniteLoop(stream, stream.Length())
	p, err := m.ctx.NewPlayer(loop)
	if err != nil {
		return err
	}
	m.player = p
	p.Play()
	return nil
}
