package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"sync"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	focusIndex        int
	topPanes          []pane
	outputPane        viewport.Model
	outputPaneFocused bool
	mu                sync.Mutex
	conn9001          net.Conn
	conn9002          net.Conn
	width             int
	height            int
}

type pane struct {
	title   string
	items   list.Model
	isMenu  bool
	focused bool
}

const (
	port9001 = "localhost:9001"
	port9002 = "localhost:9002"
)

var (
	borderStyle        = lipgloss.NewStyle().Border(lipgloss.NormalBorder())
	focusedBorderStyle = borderStyle.Copy().BorderForeground(lipgloss.Color("205"))
)

type menuItem struct {
	title, desc string
	selected    bool
}

func (m menuItem) Title() string       { return m.title }
func (m menuItem) Description() string { return m.desc } // Optional description
func (m menuItem) FilterValue() string { return m.title }

// Custom delegate
type customDelegate struct {
	styles  list.DefaultItemStyles
	focused bool
}

func (d customDelegate) Height() int  { return 1 }
func (d customDelegate) Spacing() int { return 0 }
func (d customDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

func (d customDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	// Assuming the list model has a method Items() that returns all items and an Index() method to get the current index
	pageItems := m.Items()
	for i, item := range pageItems {
		menuItem, ok := item.(menuItem)
		if !ok {
			continue // Skip items that are not menuItems
		}

		selector := "[ ]"
		if menuItem.selected {
			selector = "[x]"
		}

		if m.Title == "Commands" {
			selector = ""
		}
		var cursor string
		if i == m.Index() && d.focused { // Compare loop index with model's current index
			cursor = ">" // Show cursor when focused and selected
		} else {
			cursor = " " // No cursor when not focused
		}

		str := fmt.Sprintf("%s %s %s", cursor, selector, menuItem.Title())
		fmt.Fprintln(w, str)
	}
}

// Initialize the panes
func initialModel() model {
	pane1ItemList := []string{"Option A", "Option B", "Option C"}
	pane2ItemList := []string{"Option X", "Option Y", "Option Z"}
	commands := []string{"Cmd 1", "Cmd 2", "Cmd 3", "Exit"}
	pane4Itemlist := []string{"Opt 1", "Opt 2", "Opt 3"}

	panes := []pane{
		{"Pane 1", createList("Pane 1", pane1ItemList, 10, true), true, true},
		{"Pane 2", createList("Pane 2", pane2ItemList, 10, false), true, false},
		{"Commands", createList("Commands", commands, 10, false), false, false},
		{"Pane 4", createList("Pane 4", pane4Itemlist, 10, false), true, false},
	}

	output := viewport.New(100, 20)
	output.SetContent("Welcome to the TUI!")

	return model{topPanes: panes, outputPane: output, focusIndex: 0}
}

func createList(title string, items []string, height int, focused bool) list.Model {
	itemList := make([]list.Item, len(items))
	for i, item := range items {
		itemList[i] = menuItem{title: item}
	}

	// Ensure the list height is sufficient to display all items
	listHeight := len(items)
	if listHeight > height {
		listHeight = height
	}

	delegate := customDelegate{focused: focused}
	l := list.New(itemList, delegate, 20, listHeight) // Use dynamic height
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)

	return l
}

// Update the delegates when focus changes
func (m *model) updatePaneDelegates() {
	for i := range m.topPanes {
		m.topPanes[i].items.SetDelegate(customDelegate{focused: i == m.focusIndex})
	}
}

// BubbleTea's Init function
func (m model) Init() tea.Cmd {
	// Initialize default dimensions
	m.width = 80  // Default terminal width
	m.height = 24 // Default terminal height
	for i := range m.topPanes {
		m.topPanes[i].items.SetWidth(m.width/4 - 2)
	}
	m.outputPane.Width = m.width - 2
	m.outputPane.Height = m.height / 2

	go m.connectToServer()
	return nil
}

func (m *model) connectToServer() {
	var err error
	m.conn9001, err = net.Dial("tcp", port9001)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to port 9001: %v\n", err)
	}
	m.conn9002, err = net.Dial("tcp", port9002)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to port 9002: %v\n", err)
		return
	}
	go m.listenToServer()
}
func (m *model) listenToServer() {
	reader := bufio.NewReader(m.conn9002)
	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading from port 9002: %v\n", err)
			return
		}
		m.mu.Lock()
		m.outputPane.SetContent(m.outputPane.View() + message)
		m.mu.Unlock()
	}
}

func (m *model) addToOutputPane(txt string) {
	m.mu.Lock()
	m.outputPane.SetContent(m.outputPane.View() + txt)
	m.mu.Unlock()
}

// Update the model based on messages
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Divide the top row into four equal-width panes
		// paneWidth := m.width / 4
		paneHeight := (m.height / 2) - 2 // Consistent height for all panes
		for i := range m.topPanes {
			// Adjust height dynamically based on content
			numItems := len(m.topPanes[i].items.Items())
			displayHeight := paneHeight
			if numItems < paneHeight {
				displayHeight = numItems
			}
			m.topPanes[i].items.SetHeight(displayHeight)
		} // Output pane spans the full width and the remaining height
		m.outputPane.Width = m.width - 2
		m.outputPane.Height = m.height / 2
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			previousFocusIndex := m.focusIndex
			m.focusIndex = (m.focusIndex + 1) % (len(m.topPanes) + 1) // Include output pane

			// Update focus states
			if previousFocusIndex < len(m.topPanes) {
				m.topPanes[previousFocusIndex].focused = false
			} else {
				m.outputPaneFocused = false
			}
			if m.focusIndex < len(m.topPanes) {
				m.topPanes[m.focusIndex].focused = true
			} else {
				m.outputPaneFocused = true
			}

			// Update the delegates
			m.updatePaneDelegates()
		case " ":
			if m.focusIndex < len(m.topPanes) && m.topPanes[m.focusIndex].isMenu {
				index := m.topPanes[m.focusIndex].items.Index()
				items := m.topPanes[m.focusIndex].items.Items()
				item := items[index].(menuItem)                // Get the menuItem
				item.selected = !item.selected                 // Toggle selection
				items[index] = item                            // Update the item in the slice
				m.topPanes[m.focusIndex].items.SetItems(items) // Apply the changes
			}
			var cmd tea.Cmd
			m.topPanes[m.focusIndex].items, cmd = m.topPanes[m.focusIndex].items.Update(msg)
			return m, cmd
		case "up", "down":
			// Arrow keys are automatically handled by list.Model when focused
			if m.focusIndex < len(m.topPanes) {
				var cmd tea.Cmd
				m.topPanes[m.focusIndex].items, cmd = m.topPanes[m.focusIndex].items.Update(msg)
				return m, cmd
			} else if m.outputPaneFocused {
				var cmd tea.Cmd
				m.outputPane, cmd = m.outputPane.Update(msg)
				return m, cmd
			}
		case "enter":
			if m.focusIndex < len(m.topPanes) && !m.topPanes[m.focusIndex].isMenu {
				index := m.topPanes[m.focusIndex].items.Index()
				command := m.topPanes[m.focusIndex].items.Items()[index].FilterValue()

				if command == "Exit" {
					return m, tea.Quit
				}

				if m.conn9001 != nil {
					_, err := m.conn9001.Write([]byte(command + "\n"))
					if err != nil {
						m.outputPane.SetContent(m.outputPane.View() + "\nError sending command: " + err.Error())
					}
				}
			}
		case "q":
			return m, tea.Quit
		}
	}

	// Update the focused component
	if m.focusIndex < len(m.topPanes) {
		var cmd tea.Cmd
		m.topPanes[m.focusIndex].items, cmd = m.topPanes[m.focusIndex].items.Update(msg)
		return m, cmd
	} else {
		var cmd tea.Cmd
		m.outputPane, cmd = m.outputPane.Update(msg)
		return m, cmd
	}
}

// View renders the TUI
func (m model) View() string {
	var topPanes []string
	paneHeight := (m.height / 2) - 2 // Consistent height for all panes

	for _, pane := range m.topPanes {
		// Adjust the styling based on focus
		style := borderStyle
		if pane.focused {
			style = focusedBorderStyle
		}

		// Render each pane with the calculated height and width
		topPanes = append(topPanes, style.Width(m.width/4-2).Height(paneHeight).Render(pane.items.View()))
	}

	// Combine the top panes into a horizontal layout
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, topPanes...)

	// Render the output pane with focus styling
	var output string
	if m.outputPaneFocused {
		output = focusedBorderStyle.Width(m.width - 2).Height(m.height / 2).Render(m.outputPane.View())
	} else {
		output = borderStyle.Width(m.width - 2).Height(m.height / 2).Render(m.outputPane.View())
	}

	// Combine the top row and output pane
	return lipgloss.JoinVertical(lipgloss.Left, topRow, output)
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting app: %v\n", err)
		os.Exit(1)
	}
}
