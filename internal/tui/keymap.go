package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines the keybindings for the application.
// It helps in managing and displaying help information.
type KeyMap struct {
	Up              key.Binding
	Down            key.Binding
	Tab             key.Binding
	ShiftTab        key.Binding
	Enter           key.Binding // Context-dependent help
	Esc             key.Binding // Context-dependent help
	Quit            key.Binding
	Help            key.Binding
	NewCollection   key.Binding
	Restart         key.Binding
	SwitchContext   key.Binding
	ToggleDark      key.Binding
	ToggleDebug     key.Binding
	ToggleLog       key.Binding
	CopyLogs        key.Binding
	ToggleMcpConfig key.Binding
	// Add other bindings as needed, e.g., for input mode
}

// DefaultKeyMap returns a KeyMap with default bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("↑/k", "navigate up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("↓/j", "navigate down"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next panel"),
		),
		ShiftTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "previous panel"),
		),
		Enter: key.NewBinding( // Generic, help text might be too vague for root keymap
			key.WithKeys("enter"),
			key.WithHelp("enter", "select/confirm"),
		),
		Esc: key.NewBinding( // Generic, help text might be context specific
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel/back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q/ctrl+c", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("h"),
			key.WithHelp("h", "toggle help"),
		),
		NewCollection: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new connection"),
		),
		Restart: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "restart forwarder"),
		),
		SwitchContext: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "switch k8s context"),
		),
		ToggleDark: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "toggle dark/light mode"),
		),
		ToggleDebug: key.NewBinding(
			key.WithKeys("z"),
			key.WithHelp("z", "toggle debug info"),
		),
		CopyLogs: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "copy logs"),
		),
		ToggleLog: key.NewBinding(
			key.WithKeys("L"),
			key.WithHelp("L", "toggle log overlay"),
		),
		ToggleMcpConfig: key.NewBinding(
			key.WithKeys("C"),
			key.WithHelp("C", "show MCP config"),
		),
	}
}

// FullHelp returns bindings for the main help view.
// It's a slice of slices, where each inner slice is a column in the help view.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Tab, k.ShiftTab},                                             // Navigation column
		{k.NewCollection, k.Restart, k.SwitchContext, k.CopyLogs},                     // Operations column
		{k.Help, k.ToggleLog, k.ToggleMcpConfig, k.ToggleDark, k.ToggleDebug, k.Quit}, // UI/General column
	}
}

// ShortHelp returns a minimal set of bindings, often used for a status bar.
func (k KeyMap) ShortHelp() []key.Binding {
	// Define which keys are essential enough for a very short help line.
	// This might be context-dependent. For a global default:
	return []key.Binding{k.Help, k.Quit}
}

// InputModeHelp returns bindings specific to when in text input mode.
// This is an example if you want different help text for input mode.
func (k KeyMap) InputModeHelp() [][]key.Binding {
	return [][]key.Binding{
		{key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "submit")),
			key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "submit")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel input")),
			key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "autocomplete"))},
	}
}
