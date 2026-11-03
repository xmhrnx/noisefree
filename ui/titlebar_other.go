//go:build !windows && !darwin

package ui

import "github.com/xmhrnx/giode/components/titlebar"

// Linux keeps the window manager's decorations, which resize reliably on
// every desktop.
const useCustomTitleBar = false

// Frameless asks for a window without OS decorations.
const Frameless = useCustomTitleBar

var titleBarVariant = titlebar.VariantWindows
