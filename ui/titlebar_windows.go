package ui

import "github.com/xmhrnx/giode/components/titlebar"

// On Windows the window is frameless with Giode's title bar, which keeps
// native snapping and resize borders.
const useCustomTitleBar = true

// Frameless asks for a window without OS decorations.
const Frameless = useCustomTitleBar

var titleBarVariant = titlebar.VariantWindows
