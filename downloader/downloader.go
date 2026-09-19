package downloader

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"soundflow/ytdlp"
)

var ErrNotFound = ytdlp.ErrNotFound

type Progress struct {
	Percent float64
	Status  string
	Done    bool
	Err     error
}

type ProgressFunc func(Progress)

var percentRe = regexp.MustCompile(`\[download\]\s+([\d.]+)%`)

var invalidFilenameChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	name = invalidFilenameChars.ReplaceAllString(name, "_")
	name = strings.TrimRight(name, ". ")
	if name == "" {
		name = "track"
	}
	const maxLen = 150
	if r := []rune(name); len(r) > maxLen {
		name = string(r[:maxLen])
	}
	return name
}

func Available() error {
	_, err := ytdlp.Path()
	return err
}

func DownloadAudio(ctx context.Context, rawURL, outputDir, format, title string, progress ProgressFunc) (string, error) {
	binPath, err := ytdlp.Path()
	if err != nil {
		return "", err
	}
	if rawURL = strings.TrimSpace(rawURL); rawURL == "" {
		return "", errors.New("downloader: empty URL")
	}
	if format == "" {
		format = "mp3"
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("downloader: creating output dir: %w", err)
	}

	knownTitle := strings.TrimSpace(title) != ""
	var safeName string
	var outputTemplate string
	if knownTitle {
		safeName = sanitizeFilename(title)
		outputTemplate = filepath.Join(outputDir, safeName+".%(ext)s")
	} else {
		outputTemplate = filepath.Join(outputDir, "%(title)s.%(ext)s")
	}

	args := []string{
		"-x",
		"--audio-format", format,
		"--no-playlist",
		"--newline",
		"--encoding", "utf-8",
		"-o", outputTemplate,
	}
	if !knownTitle {
		args = append(args, "--print", "after_move:%(filepath)j")
	}
	args = append(args, rawURL)

	cmd := exec.CommandContext(ctx, binPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("downloader: stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("downloader: stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("downloader: starting yt-dlp: %w", err)
	}

	var finalPath string
	var lastErrLine string

	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()

			if m := percentRe.FindStringSubmatch(line); m != nil {
				pct, _ := strconv.ParseFloat(m[1], 64)
				report(progress, Progress{Percent: pct, Status: line})
				continue
			}

			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, `"`) {
				var decoded string
				if err := json.Unmarshal([]byte(trimmed), &decoded); err == nil {
					finalPath = decoded
				}
			}
			report(progress, Progress{Percent: -1, Status: line})
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			lastErrLine = scanner.Text()
			report(progress, Progress{Percent: -1, Status: lastErrLine})
		}
	}()

	if err := cmd.Wait(); err != nil {
		report(progress, Progress{Done: true, Err: err})
		if lastErrLine != "" {
			return "", fmt.Errorf("downloader: yt-dlp failed: %s", lastErrLine)
		}
		return "", fmt.Errorf("downloader: yt-dlp failed: %w", err)
	}

	if finalPath == "" {
		if knownTitle {
			predicted := filepath.Join(outputDir, safeName+"."+format)
			if _, statErr := os.Stat(predicted); statErr == nil {
				finalPath = predicted
			} else if matches, _ := filepath.Glob(filepath.Join(outputDir, safeName+".*")); len(matches) > 0 {
				finalPath = matches[0]
			}
		}
	}

	if finalPath == "" {
		report(progress, Progress{Done: true, Err: errors.New("downloader: could not determine output file")})
		return "", errors.New("downloader: could not determine output file path")
	}

	report(progress, Progress{Percent: 100, Status: "done", Done: true})
	return finalPath, nil
}

func report(fn ProgressFunc, p Progress) {
	if fn != nil {
		fn(p)
	}
}
