package main

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"soundflow/downloader"
	"soundflow/player"
	"soundflow/providers"
)

var searchProviders = map[string]providers.Provider{
	"YouTube":    providers.YouTube{},
	"SoundCloud": providers.SoundCloud{},
}

func main() {
	a := app.New()
	w := a.NewWindow("SoundFlow")

	streamPlayer := &player.Player{}

	if err := downloader.Available(); err != nil {
		dialog.ShowError(fmt.Errorf("yt-dlp не найден в PATH — установи его, чтобы искать и скачивать треки"), w)
	}

	status := widget.NewLabel("Готов к загрузке")
	progress := widget.NewProgressBar()
	progress.Hide()

	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Название трека или исполнителя...")

	providerSelect := widget.NewSelect([]string{"YouTube", "SoundCloud"}, nil)
	providerSelect.SetSelected("YouTube")

	var downloadedTracks []string
	downloadedList := widget.NewList(
		func() int { return len(downloadedTracks) },
		func() fyne.CanvasObject { return widget.NewLabel("track") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(filepath.Base(downloadedTracks[i]))
		},
	)
	downloadedList.OnSelected = func(i widget.ListItemID) {
		playLocalFile(downloadedTracks[i])
		downloadedList.UnselectAll()
	}

	downloadAndSave := func(url string) {
		progress.Show()
		progress.SetValue(0)
		status.SetText("Загрузка...")

		go func() {
			ctx := context.Background()
			path, err := downloader.DownloadAudio(ctx, url, "downloads", "mp3", func(p downloader.Progress) {
				fyne.Do(func() {
					if p.Percent >= 0 {
						progress.SetValue(p.Percent / 100)
					}
					status.SetText(p.Status)
				})
			})
			if err != nil {
				fyne.Do(func() { status.SetText("Ошибка: " + err.Error()) })
				return
			}

			fyne.Do(func() {
				downloadedTracks = append(downloadedTracks, path)
				downloadedList.Refresh()
				status.SetText("Готово: " + filepath.Base(path))
				progress.SetValue(1)
			})
		}()
	}

	playStream := func(t providers.Track) {
		status.SetText("Стримлю: " + t.Title)
		go func() {
			if err := streamPlayer.Play(t.URL, func(s string) {
				fyne.Do(func() { status.SetText(s) })
			}); err != nil {
				fyne.Do(func() { status.SetText("Ошибка стрима: " + err.Error()) })
			}
		}()
	}

	var results []providers.Track
	resultsList := widget.NewList(
		func() int { return len(results) },
		func() fyne.CanvasObject {
			label := widget.NewLabel("result")
			playBtn := widget.NewButtonWithIcon("", theme.MediaPlayIcon(), nil)
			downloadBtn := widget.NewButtonWithIcon("", theme.DownloadIcon(), nil)
			buttons := container.NewHBox(playBtn, downloadBtn)
			return container.NewHBox(label, buttons)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			row := o.(*fyne.Container)
			label := row.Objects[0].(*widget.Label)
			buttons := row.Objects[1].(*fyne.Container)
			playBtn := buttons.Objects[0].(*widget.Button)
			downloadBtn := buttons.Objects[1].(*widget.Button)

			t := results[i]
			label.SetText(fmt.Sprintf("[%s] %s — %s (%s)",
				t.Provider, t.Title, t.Uploader, formatDuration(t.Duration)))
			playBtn.OnTapped = func() { playStream(t) }
			downloadBtn.OnTapped = func() { downloadAndSave(t.URL) }
		},
	)

	doSearch := func() {
		query := searchEntry.Text
		if query == "" {
			return
		}
		p, ok := searchProviders[providerSelect.Selected]
		if !ok {
			return
		}
		status.SetText("Ищу «" + query + "» на " + p.Name() + "...")
		results = nil
		resultsList.Refresh()

		go func() {
			ctx := context.Background()
			found, err := p.Search(ctx, query, 10)
			if err != nil {
				fyne.Do(func() { status.SetText("Ошибка поиска: " + err.Error()) })
				return
			}
			fyne.Do(func() {
				results = found
				resultsList.Refresh()
				status.SetText(fmt.Sprintf("Найдено: %d", len(results)))
			})
		}()
	}
	searchEntry.OnSubmitted = func(string) { doSearch() }
	searchBtn := widget.NewButton("Искать", doSearch)
	stopBtn := widget.NewButtonWithIcon("Стоп", theme.MediaStopIcon(), func() {
		streamPlayer.Stop()
		status.SetText("Остановлено")
	})

	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder("...или вставь прямую ссылку")
	pasteDownloadBtn := widget.NewButton("Скачать по ссылке", func() {
		if urlEntry.Text != "" {
			downloadAndSave(urlEntry.Text)
		}
	})

	searchBar := container.NewBorder(nil, nil, providerSelect, searchBtn, searchEntry)
	pasteBar := container.NewBorder(nil, nil, nil, pasteDownloadBtn, urlEntry)

	controls := container.NewVBox(
		searchBar,
		pasteBar,
		container.NewHBox(stopBtn, progress),
		status,
	)

	resultsBox := container.NewBorder(widget.NewLabel("Результаты поиска:"), nil, nil, nil, resultsList)
	downloadedBox := container.NewBorder(widget.NewLabel("Скачано:"), nil, nil, nil, downloadedList)
	lists := container.NewVSplit(resultsBox, downloadedBox)
	lists.Offset = 0.55

	content := container.NewBorder(controls, nil, nil, nil, lists)

	w.SetContent(content)
	w.Resize(fyne.NewSize(560, 560))
	w.ShowAndRun()
}

func formatDuration(seconds int) string {
	if seconds <= 0 {
		return "0:00"
	}
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

func playLocalFile(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	_ = cmd.Start()
}
