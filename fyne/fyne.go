package main

import (
	"io"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

func main() {
	app := app.New()
	w := app.NewWindow("文本查看器")

	// 将 TextGrid 改为 TextArea
	textArea := widget.NewMultiLineEntry()
	textArea.Wrapping = fyne.TextWrapWord // 启用自动换行

	// 创建固定大小的打开文件按钮
	openButton := widget.NewButton("打开文件", func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if reader == nil {
				return
			}
			defer reader.Close()

			data, err := io.ReadAll(reader)
			if err != nil {
				dialog.ShowError(err, w)
				return
			}

			textArea.SetText(string(data))
		}, w)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".txt"}))
		fd.Show()
	})
	openButton.Resize(fyne.NewSize(100, 40))

	// 创建按钮容器，使用spacer来保持按钮大小固定
	buttonContainer := container.NewHBox(openButton, layout.NewSpacer())

	// 使用 Border 布局，让文本区域能够填充剩余空间
	content := container.NewBorder(buttonContainer, nil, nil, nil, textArea)

	w.SetContent(content)
	w.Resize(fyne.NewSize(800, 600))
	w.ShowAndRun()
}
