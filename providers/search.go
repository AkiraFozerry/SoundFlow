package providers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"soundflow/ytdlp"
)

type Track struct {
	Title    string
	Uploader string
	Duration int
	URL      string
	Provider string
}

type Provider interface {
	Name() string
	Search(ctx context.Context, query string, limit int) ([]Track, error)
}

type rawResult struct {
	Title      string  `json:"title"`
	Uploader   string  `json:"uploader"`
	Channel    string  `json:"channel"`
	Duration   float64 `json:"duration"`
	WebpageURL string  `json:"webpage_url"`
}

func searchViaYtDlp(ctx context.Context, prefix, providerName, query string, limit int) ([]Track, error) {
	if limit <= 0 {
		limit = 10
	}
	term := fmt.Sprintf("%s%d:%s", prefix, limit, query)

	binPath, err := ytdlp.Path()
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, binPath,
		"--dump-json",
		"--no-playlist",
		"--skip-download",
		"--ignore-errors",
		"--encoding", "utf-8",
		term,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("providers: stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("providers: starting yt-dlp: %w", err)
	}

	var tracks []Track
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 256*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var r rawResult
		if err := json.Unmarshal(line, &r); err != nil {
			continue
		}
		uploader := r.Uploader
		if uploader == "" {
			uploader = r.Channel
		}
		tracks = append(tracks, Track{
			Title:    r.Title,
			Uploader: uploader,
			Duration: int(r.Duration),
			URL:      r.WebpageURL,
			Provider: providerName,
		})
	}

	waitErr := cmd.Wait()
	if waitErr != nil && len(tracks) == 0 {
		return nil, fmt.Errorf("providers: %s search failed: %w", providerName, waitErr)
	}

	return tracks, nil
}
