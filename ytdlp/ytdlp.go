package ytdlp

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

var ErrNotFound = errors.New("yt-dlp: binary not found next to SoundFlow, in the working directory, or in PATH")

func Path() (string, error) {
	path, err := Resolve("yt-dlp")
	if err != nil {
		return "", ErrNotFound
	}
	return path, nil
}

func Resolve(name string) (string, error) {
	exeName := name
	if runtime.GOOS == "windows" {
		exeName += ".exe"
	}

	var candidates []string
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), exeName))
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, exeName))
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c, nil
		}
	}

	path, err := exec.LookPath(exeName)
	if err != nil && !errors.Is(err, exec.ErrDot) {
		return "", fmt.Errorf("%s: binary not found next to SoundFlow, in the working directory, or in PATH", name)
	}
	if path == "" {
		return "", fmt.Errorf("%s: binary not found next to SoundFlow, in the working directory, or in PATH", name)
	}
	return path, nil
}
