package player

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"os/exec"

	"soundflow/ytdlp"
)

type Player struct {
	mu     sync.Mutex
	cancel context.CancelFunc
}

type StatusFunc func(string)

func (p *Player) Play(rawURL string, onStatus StatusFunc) error {
	p.Stop()

	ytPath, err := ytdlp.Resolve("yt-dlp")
	if err != nil {
		return err
	}
	ffPath, err := ytdlp.Resolve("ffplay")
	if err != nil {
		return fmt.Errorf("ffplay не найден рядом с SoundFlow, в текущей папке или в PATH: %w", err)
	}
	if rawURL = strings.TrimSpace(rawURL); rawURL == "" {
		return fmt.Errorf("player: empty URL")
	}

	ctx, cancel := context.WithCancel(context.Background())

	ytCmd := exec.CommandContext(ctx, ytPath,
		"-f", "bestaudio",
		"--no-playlist",
		"-q",
		"-o", "-",
		rawURL,
	)
	ffCmd := exec.CommandContext(ctx, ffPath,
		"-nodisp",
		"-autoexit",
		"-loglevel", "quiet",
		"-i", "pipe:0",
	)

	stdout, err := ytCmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("player: yt-dlp stdout pipe: %w", err)
	}
	ffCmd.Stdin = stdout

	if err := ytCmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("player: starting yt-dlp: %w", err)
	}
	if err := ffCmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("player: starting ffplay: %w", err)
	}

	p.mu.Lock()
	p.cancel = cancel
	p.mu.Unlock()

	report(onStatus, "Воспроизведение...")

	go func() { _ = ytCmd.Wait() }()
	go func() {
		_ = ffCmd.Wait()
		report(onStatus, "Остановлено")
	}()

	return nil
}

func (p *Player) Stop() {
	p.mu.Lock()
	cancel := p.cancel
	p.cancel = nil
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func report(fn StatusFunc, s string) {
	if fn != nil {
		fn(s)
	}
}
