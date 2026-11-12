package ui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all keybindings. Using the Bubbles key package lets the help
// view be generated automatically.
type KeyMap struct {
	Up        key.Binding
	Down      key.Binding
	PrevDay   key.Binding
	NextDay   key.Binding
	PrevWeek  key.Binding
	NextWeek  key.Binding
	NextPanel key.Binding
	Panel1    key.Binding
	Panel2    key.Binding
	Panel3    key.Binding
	Add       key.Binding
	Edit      key.Binding
	Delete    key.Binding
	Account   key.Binding
	Search    key.Binding
	Sort      key.Binding
	Pin       key.Binding
	Refresh   key.Binding
	Help      key.Binding
	Quit      key.Binding
}

// DefaultKeyMap returns the standard bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:        key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("j/k", "navigate")),
		Down:      key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/k", "navigate")),
		PrevDay:   key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("h/l", "prev/next day")),
		NextDay:   key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("h/l", "prev/next day")),
		PrevWeek:  key.NewBinding(key.WithKeys("H"), key.WithHelp("H/L", "prev/next week")),
		NextWeek:  key.NewBinding(key.WithKeys("L"), key.WithHelp("H/L", "prev/next week")),
		NextPanel: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch panel")),
		Panel1:    key.NewBinding(key.WithKeys("1"), key.WithHelp("1/2/3", "jump to panel")),
		Panel2:    key.NewBinding(key.WithKeys("2")),
		Panel3:    key.NewBinding(key.WithKeys("3")),
		Add:       key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add worklog")),
		Edit:      key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		Delete:    key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		Account:   key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "change account")),
		Search:    key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search issues")),
		Sort:      key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sort issues")),
		Pin:       key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "pin/unpin")),
		Refresh:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

// ShortHelp implements help.KeyMap.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.NextPanel, k.Add, k.Edit, k.Delete, k.Account, k.Search, k.Refresh, k.Help, k.Quit}
}

// FullHelp implements help.KeyMap.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PrevDay, k.NextDay, k.PrevWeek, k.NextWeek},
		{k.NextPanel, k.Panel1, k.Search, k.Sort, k.Pin, k.Refresh},
		{k.Add, k.Edit, k.Delete, k.Account},
		{k.Help, k.Quit},
	}
}
