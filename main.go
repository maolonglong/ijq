package main

import (
	"cmp"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

var _marginTop1 = lipgloss.NewStyle().MarginTop(1)

type keyMap struct {
	quit          key.Binding
	focusNextPane key.Binding
	eval          key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
		focusNextPane: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "focus next pane"),
		),
		eval: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "eval"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.quit, k.eval, k.focusNextPane}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.quit, k.eval, k.focusNextPane}}
}

type model struct {
	help          help.Model
	content       string
	result        string
	viewport      viewport.Model
	keys          keyMap
	textinput     textinput.Model
	ready         bool
	focusViewport bool
}

func newModel(content string) model {
	ti := textinput.New()
	ti.Focus()
	ti.Placeholder = "jq filter"

	return model{
		content:   content,
		result:    jq(content, "."),
		keys:      defaultKeyMap(),
		textinput: ti,
		help:      help.New(),
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		inputHeight := lipgloss.Height(m.textinput.View())
		helpHeight := lipgloss.Height(_marginTop1.Render(m.help.View(m.keys)))
		margin := inputHeight + helpHeight
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-margin)
			m.viewport.HighPerformanceRendering = false
			m.viewport.SetContent(m.result)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - margin
		}

		m.textinput.Width = msg.Width
		m.help.Width = msg.Width

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "tab":
			if !m.focusViewport {
				m.textinput.Blur()
				m.keys.eval.SetEnabled(false)
			} else {
				m.textinput.Focus()
				m.keys.eval.SetEnabled(true)
			}
			m.focusViewport = !m.focusViewport
		case "enter":
			if !m.focusViewport {
				out := jq(m.content, m.jqFilter())
				m.result = out
				m.viewport.SetContent(out)
				m.viewport.GotoTop()
			}
		default:
			if !m.focusViewport {
				m.textinput, cmd = m.textinput.Update(msg)
			} else {
				m.viewport, cmd = m.viewport.Update(msg)
			}
		}

	default:
	}

	return m, cmd
}

func (m model) View() string {
	var sb strings.Builder
	sb.WriteString(m.textinput.View())
	sb.WriteByte('\n')
	sb.WriteString(m.viewport.View())
	sb.WriteByte('\n')
	sb.WriteString(_marginTop1.Render(m.help.View(m.keys)))
	return sb.String()
}

func (m model) jqFilter() string {
	return cmp.Or(strings.TrimSpace(m.textinput.Value()), ".")
}

func jq(content, filter string) string {
	cmd := exec.Command("jq", "--color-output", cmp.Or(filter, "."))
	cmd.Stdin = strings.NewReader(content)
	var sb strings.Builder
	cmd.Stdout = &sb
	cmd.Stderr = &sb
	_ = cmd.Run()
	return sb.String()
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [file...]\n", os.Args[0])
}

func getContent() (string, error) {
	var (
		sb  strings.Builder
		err error
	)
	if flag.NArg() == 0 {
		_, err = io.Copy(&sb, os.Stdin)
	} else {
		for _, name := range flag.Args() {
			if err = readFile(&sb, name); err != nil {
				break
			}
		}
	}
	if err != nil {
		return "", err
	}
	return sb.String(), nil
}

func readFile(dst io.Writer, name string) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(dst, f)
	return err
}

func main() {
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()

	_, err := exec.LookPath("jq")
	if err != nil {
		log.Fatal("'jq: command not found")
	}

	content, err := getContent()
	if err != nil {
		log.Fatal(err)
	}

	lipgloss.SetColorProfile(termenv.NewOutput(os.Stderr).Profile)
	p := tea.NewProgram(
		newModel(content),
		tea.WithOutput(os.Stderr),
		tea.WithAltScreen(),
	)

	tm, err := p.Run()
	if err != nil {
		log.Fatal(err)
	}

	m := tm.(model)
	fmt.Println(m.jqFilter())
}
