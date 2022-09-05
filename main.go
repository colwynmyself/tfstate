package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	boxer "github.com/treilik/bubbleboxer"
)

// You generally won't need this unless you're processing stuff with
// complicated ANSI escape sequences. Turn it on if you notice flickering.
//
// Also keep in mind that high performance rendering only works for programs
// that use the full size of the terminal. We're enabling that below with
// tea.EnterAltScreen().
const useHighPerformanceRenderer = false

var (
	titleStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Right = "├"
		return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1)
	}()

	infoStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Left = "┤"
		return titleStyle.Copy().BorderStyle(b)
	}()
)

// ---------------------
// Boxer Model Interface
// ---------------------
type boxerModel struct {
	tui boxer.Boxer
}

func (m boxerModel) Init() tea.Cmd {
	return nil
}

func scrollState(m boxerModel, direction string) (boxerModel, error) {
	// TODO: Make this more efficient, it's pretty bad right now
	mStates := []stateModel{}
	for _, mCheck := range m.tui.ModelMap {
		if mState, ok := mCheck.(stateModel); ok {
			mStates = append(mStates, mState)
		}
	}
	sort.SliceStable(mStates, func(i, j int) bool {
		return mStates[i].index < mStates[j].index
	})

	for i, mState := range mStates {
		if mState.active {
			nextIndex := i
			if direction == "right" {
				nextIndex = i + 1
			} else if direction == "left" {
				nextIndex = i - 1
			} else {
				log.Panicf("%s is an invalid direction", direction)
			}

			maxIndex := len(mStates) - 1
			if nextIndex < 0 {
				if direction == "right" {
					// This is a weird case that should never happen
					nextIndex = 0
				} else {
					nextIndex = maxIndex
				}
			}
			if nextIndex >= len(mStates) {
				if direction == "right" {
					nextIndex = 0
				} else {
					// This is a weird case that should never happen
					nextIndex = maxIndex
				}
			}
			nextState := mStates[nextIndex]
			nextState.active = true
			mState.active = false

			// Point at the new object on the ModelMap to update our state. I should really figure out if I can do this
			// with references and change the mState object in place.
			m.tui.ModelMap[nextState.statefile] = nextState
			m.tui.ModelMap[mState.statefile] = mState

			break
		}
	}

	return m, nil
}

func (m boxerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	for name, childModel := range m.tui.ModelMap {
		if mState, ok := childModel.(stateModel); ok {
			if mState.active {
				updatedChildModel, _ := mState.Update(msg)
				m.tui.ModelMap[name] = updatedChildModel
			}
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "right", "left":
			m, err := scrollState(m, msg.String())
			if err != nil {
				// TODO: Do something better here
				panic("Error scolling states!")
			}

			return m, nil
		}
	case tea.WindowSizeMsg:
		m.tui.UpdateSize(msg)
	}

	return m, nil
}
func (m boxerModel) View() string {
	return m.tui.View()
}

type stringer string

func (s stringer) String() string {
	return string(s)
}

// satisfy the tea.Model interface
func (s stringer) Init() tea.Cmd                           { return nil }
func (s stringer) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return s, nil }
func (s stringer) View() string                            { return s.String() }

// ---------------------
// State Model Interface
// ---------------------
type stateModel struct {
	index     int
	content   string
	statefile string
	active    bool
	ready     bool
	viewport  viewport.Model
}

func (m stateModel) Init() tea.Cmd {
	return nil
}

func (m stateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.headerView())
		footerHeight := lipgloss.Height(m.footerView())
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			// Since this program is using the full size of the viewport we
			// need to wait until we've received the window dimensions before
			// we can initialize the viewport. The initial dimensions come in
			// quickly, though asynchronously, which is why we wait for them
			// here.
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.viewport.HighPerformanceRendering = useHighPerformanceRenderer
			m.viewport.SetContent(m.content)
			m.ready = true

			// This is only necessary for high performance rendering, which in
			// most cases you won't need.
			//
			// Render the viewport one line below the header.
			m.viewport.YPosition = headerHeight + 1
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}

		if useHighPerformanceRenderer {
			// Render (or re-render) the whole viewport. Necessary both to
			// initialize the viewport and when the window is resized.
			//
			// This is needed for high-performance rendering only.
			cmds = append(cmds, viewport.Sync(m.viewport))
		}
	}

	// Handle keyboard and mouse events in the viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m stateModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}
	return fmt.Sprintf("%s\n%s\n%s", m.headerView(), m.viewport.View(), m.footerView())
}

func (m stateModel) headerView() string {
	titleText := m.statefile
	if m.active {
		titleText += " (ACTIVE)"
	}
	title := titleStyle.Render(titleText)
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(title)))

	style := lipgloss.NewStyle()
	// TODO: Make this prettier
	if m.active {
		style = style.
			Bold(true).
			Foreground(lipgloss.Color("#000")).
			Background(lipgloss.Color("#EEE"))
	}

	return style.Render(lipgloss.JoinHorizontal(lipgloss.Center, title, line))
}

func (m stateModel) footerView() string {
	info := infoStyle.Render(fmt.Sprintf("%3.f%%", m.viewport.ScrollPercent()*100))
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(info)))
	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
}

// ---------------
// Everything else
// ---------------
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func getStateStringForFile(statefile string) (string, error) {
	stateBytes, err := exec.Command("terraform", "show", statefile).Output()
	if err != nil {
		// Maybe I'll deal with this one day
		return "", err
	}
	return string(stateBytes), nil
}

func main() {
	// I'm using a slice here because this will eventually be dynamic when we pull an arbitrary number of versions from
	// remote state.
	statefiles := []string{
		"terraform.tfstate",
		"terraform.tfstate.backup",
	}

	mStates := []stateModel{}
	for i, statefile := range statefiles {
		stateString, err := getStateStringForFile(statefile)
		if err != nil {
			log.Printf("Error reading statefile %s: %s\n", statefile, err)
			os.Exit(1)
		}
		mState := stateModel{
			index:     i,
			content:   stateString,
			statefile: statefile,
			active:    i == 0,
		}
		mStates = append(mStates, mState)
	}

	// layout-tree defintion
	mBoxer := boxerModel{tui: boxer.Boxer{}}
	boxerNodes := []boxer.Node{}
	// Probably combine this with the loop above to save some cycles, this is just explicit for now since I'm learning how
	// to write good golang.
	for _, mState := range mStates {
		boxerNodes = append(boxerNodes, mBoxer.tui.CreateLeaf(mState.statefile, mState))
	}

	mBoxer.tui.LayoutTree = boxer.Node{
		// orientation
		VerticalStacked: true,
		// spacing
		SizeFunc: func(_ boxer.Node, widthOrHeight int) []int {
			return []int{
				// since this node is vertical stacked return the height partioning since the width stays for all children fixed
				widthOrHeight - 1,
				1,
				// make also sure that the amount of the returned ints match the amount of children:
				// in this case two, but in more complex cases read the amount of the chilren from the len(boxer.Node.Children)
			}
		},
		Children: []boxer.Node{
			{
				Children: boxerNodes,
			},
			mBoxer.tui.CreateLeaf("lower", stringer("use q or ctrl+c to quit")),
		},
	}
	p := tea.NewProgram(
		mBoxer,
		tea.WithAltScreen(),       // use the full size of the terminal in its "alternate screen buffer"
		tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
	)
	p.EnterAltScreen()
	if err := p.Start(); err != nil {
		fmt.Println(err)
	}
	p.ExitAltScreen()
}
