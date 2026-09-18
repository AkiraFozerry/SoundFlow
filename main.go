package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/widget"
)

func main() {
	a := app.New()
	w := a.NewWindow("SoundFlow")

	label := widget.NewLabel("Нормалды все vhjkvvgjcgробит")
	w.SetContent(label)

	w.Resize(fyne.NewSize(400, 300))
	w.ShowAndRun()
}