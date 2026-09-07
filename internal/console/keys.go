// SPDX-License-Identifier: MPL-2.0

package console

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
)

var quitKey = key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit console"))
var scrollKey = key.NewBinding(key.WithKeys("up", "down", "k", "j"), key.WithHelp("↑/↓", "scroll"))

type keyMap struct{}

func (keyMap) ShortHelp() []key.Binding  { return []key.Binding{scrollKey, quitKey} }
func (keyMap) FullHelp() [][]key.Binding { return [][]key.Binding{{scrollKey, quitKey}} }
func keyHelp() string                    { return help.New().View(keyMap{}) }
