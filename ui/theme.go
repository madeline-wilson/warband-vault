package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type vaultTheme struct {
	base fyne.Theme
}

func newVaultTheme() fyne.Theme {
	return vaultTheme{base: theme.DefaultTheme()}
}

func (t vaultTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 0x0c, G: 0x10, B: 0x12, A: 0xff}
	case theme.ColorNameHeaderBackground, theme.ColorNameMenuBackground:
		return color.NRGBA{R: 0x17, G: 0x18, B: 0x16, A: 0xff}
	case theme.ColorNameButton:
		return color.NRGBA{R: 0x20, G: 0x25, B: 0x28, A: 0xff}
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 0x13, G: 0x18, B: 0x1b, A: 0xff}
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return color.NRGBA{R: 0x8f, G: 0x6b, B: 0x2d, A: 0xff}
	case theme.ColorNameForeground:
		return color.NRGBA{R: 0xe8, G: 0xdc, B: 0xc3, A: 0xff}
	case theme.ColorNamePlaceHolder, theme.ColorNameDisabled:
		return color.NRGBA{R: 0x8d, G: 0x86, B: 0x76, A: 0xff}
	case theme.ColorNamePrimary, theme.ColorNameHyperlink:
		return color.NRGBA{R: 0x00, G: 0xf0, B: 0xff, A: 0xff}
	case theme.ColorNameForegroundOnPrimary:
		return color.NRGBA{R: 0x08, G: 0x0c, B: 0x0f, A: 0xff}
	case theme.ColorNameFocus:
		return color.NRGBA{R: 0x00, G: 0xf0, B: 0xff, A: 0x66}
	case theme.ColorNameSelection:
		return color.NRGBA{R: 0x00, G: 0xb7, B: 0xc7, A: 0x55}
	case theme.ColorNameHover:
		return color.NRGBA{R: 0xbd, G: 0x2f, B: 0xff, A: 0x24}
	case theme.ColorNamePressed:
		return color.NRGBA{R: 0xbd, G: 0x2f, B: 0xff, A: 0x4c}
	case theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 0x12, G: 0x13, B: 0x11, A: 0xff}
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x9a}
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 0x00, G: 0xf0, B: 0xff, A: 0xbb}
	case theme.ColorNameScrollBarBackground:
		return color.NRGBA{R: 0x16, G: 0x17, B: 0x16, A: 0xff}
	case theme.ColorNameSuccess:
		return color.NRGBA{R: 0x72, G: 0xff, B: 0x98, A: 0xff}
	case theme.ColorNameWarning:
		return color.NRGBA{R: 0xd7, G: 0x9d, B: 0x3d, A: 0xff}
	case theme.ColorNameError:
		return color.NRGBA{R: 0xff, G: 0x4f, B: 0x6d, A: 0xff}
	}
	return t.base.Color(name, variant)
}

func (t vaultTheme) Font(style fyne.TextStyle) fyne.Resource {
	return t.base.Font(style)
}

func (t vaultTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t vaultTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInnerPadding:
		return 9
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 2
	case theme.SizeNameSeparatorThickness:
		return 1.5
	case theme.SizeNameText:
		return 14
	case theme.SizeNameHeadingText:
		return 22
	case theme.SizeNameSubHeadingText:
		return 17
	}
	return t.base.Size(name)
}
