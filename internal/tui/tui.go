package tui

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/w0rxbend/nerd-font-installer/internal/config"
	"github.com/w0rxbend/nerd-font-installer/internal/nerdfonts"
)

type Result struct {
	Config    config.Config
	Cancelled bool
}

type Options struct {
	Destination      string
	RefreshFontCache bool
	Output           io.Writer
}

type step int

const (
	stepRelease step = iota
	stepFamilies
	stepDone
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	helpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

type item struct {
	title       string
	description string
	value       string
}

func (i item) Title() string {
	return i.title
}

func (i item) Description() string {
	return i.description
}

func (i item) FilterValue() string {
	return i.value
}

type model struct {
	step             step
	releases         []nerdfonts.Release
	releaseList      list.Model
	familyList       list.Model
	selectedFamilies map[string]bool
	selectedRelease  nerdfonts.Release
	destination      string
	refreshFontCache bool
	cancelled        bool
	err              error
}

func Run(ctx context.Context, releases []nerdfonts.Release, opts Options) (Result, error) {
	if len(releases) == 0 {
		return Result{}, fmt.Errorf("no Nerd Fonts releases available")
	}

	destination := opts.Destination
	if strings.TrimSpace(destination) == "" {
		destination = "~/.local/share/fonts/NerdFonts"
	}

	m := newModel(releases, destination, opts.RefreshFontCache)
	programOptions := []tea.ProgramOption{
		tea.WithContext(ctx),
		tea.WithAltScreen(),
	}
	if opts.Output != nil {
		programOptions = append(programOptions, tea.WithOutput(opts.Output))
	}

	program := tea.NewProgram(m, programOptions...)
	finalModel, err := program.Run()
	if err != nil {
		return Result{}, err
	}

	m, ok := finalModel.(model)
	if !ok {
		return Result{}, fmt.Errorf("unexpected TUI model %T", finalModel)
	}
	if m.cancelled {
		return Result{Cancelled: true}, nil
	}
	if m.err != nil {
		return Result{}, m.err
	}

	families := make([]string, 0, len(m.selectedFamilies))
	for family, selected := range m.selectedFamilies {
		if selected {
			families = append(families, family)
		}
	}
	slices.Sort(families)
	if len(families) == 0 {
		return Result{Cancelled: true}, nil
	}

	return Result{
		Config: config.Config{
			Release:          m.selectedRelease.TagName,
			Destination:      m.destination,
			RefreshFontCache: m.refreshFontCache,
			Families:         families,
		},
	}, nil
}

func newModel(releases []nerdfonts.Release, destination string, refreshFontCache bool) model {
	items := make([]list.Item, 0, len(releases))
	for _, release := range releases {
		description := fmt.Sprintf("%d font archives", len(release.Families))
		items = append(items, item{
			title:       release.TagName,
			description: description,
			value:       release.TagName + " " + release.Name,
		})
	}

	delegate := list.NewDefaultDelegate()
	releaseList := list.New(items, delegate, 0, 0)
	releaseList.Title = "Select Nerd Fonts release"
	releaseList.SetShowStatusBar(false)
	releaseList.SetFilteringEnabled(true)

	return model{
		step:             stepRelease,
		releases:         releases,
		releaseList:      releaseList,
		selectedFamilies: map[string]bool{},
		destination:      destination,
		refreshFontCache: refreshFontCache,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.releaseList.SetSize(msg.Width, msg.Height-6)
		m.familyList.SetSize(msg.Width, msg.Height-8)
		return m, nil
	case tea.KeyMsg:
		return m.updateKey(msg)
	}

	var cmd tea.Cmd
	switch m.step {
	case stepRelease:
		m.releaseList, cmd = m.releaseList.Update(msg)
	case stepFamilies:
		m.familyList, cmd = m.familyList.Update(msg)
	}
	return m, cmd
}

func (m model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.cancelled = true
		return m, tea.Quit
	case "esc":
		if m.step == stepFamilies {
			m.step = stepRelease
			return m, nil
		}
		m.cancelled = true
		return m, tea.Quit
	}

	switch m.step {
	case stepRelease:
		return m.updateReleaseKey(msg)
	case stepFamilies:
		return m.updateFamilyKey(msg)
	default:
		return m, nil
	}
}

func (m model) updateReleaseKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() != "enter" {
		var cmd tea.Cmd
		m.releaseList, cmd = m.releaseList.Update(msg)
		return m, cmd
	}

	selected, ok := m.releaseList.SelectedItem().(item)
	if !ok {
		return m, nil
	}
	for _, release := range m.releases {
		if release.TagName == selected.title {
			m.selectedRelease = release
			m.step = stepFamilies
			m.selectedFamilies = map[string]bool{}
			m.familyList = m.newFamilyList()
			return m, nil
		}
	}

	m.err = fmt.Errorf("selected release %q was not found", selected.title)
	return m, tea.Quit
}

func (m model) updateFamilyKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "b":
		m.step = stepRelease
		return m, nil
	case " ":
		selected, ok := m.familyList.SelectedItem().(item)
		if !ok {
			return m, nil
		}
		m.selectedFamilies[selected.value] = !m.selectedFamilies[selected.value]
		m.familyList.SetItems(m.familyItems())
		return m, nil
	case "a":
		allSelected := m.selectedCount() == len(m.selectedRelease.Families)
		m.selectedFamilies = map[string]bool{}
		if !allSelected {
			for _, family := range m.selectedRelease.Families {
				m.selectedFamilies[family] = true
			}
		}
		m.familyList.SetItems(m.familyItems())
		return m, nil
	case "enter":
		if m.selectedCount() == 0 {
			return m, nil
		}
		m.step = stepDone
		return m, tea.Quit
	default:
		var cmd tea.Cmd
		m.familyList, cmd = m.familyList.Update(msg)
		return m, cmd
	}
}

func (m model) newFamilyList() list.Model {
	delegate := list.NewDefaultDelegate()
	familyList := list.New(m.familyItems(), delegate, m.releaseList.Width(), m.releaseList.Height())
	familyList.Title = "Select font families"
	familyList.SetShowStatusBar(false)
	familyList.SetFilteringEnabled(true)
	return familyList
}

func (m model) familyItems() []list.Item {
	items := make([]list.Item, 0, len(m.selectedRelease.Families))
	for _, family := range m.selectedRelease.Families {
		marker := "[ ]"
		if m.selectedFamilies[family] {
			marker = "[x]"
		}
		items = append(items, item{
			title:       marker + " " + family,
			description: m.selectedRelease.TagName,
			value:       family,
		})
	}
	return items
}

func (m model) selectedCount() int {
	count := 0
	for _, selected := range m.selectedFamilies {
		if selected {
			count++
		}
	}
	return count
}

func (m model) View() string {
	if m.err != nil {
		return errorStyle.Render(m.err.Error())
	}

	switch m.step {
	case stepRelease:
		return strings.Join([]string{
			titleStyle.Render("Nerd Font Installer"),
			"Choose the Nerd Fonts release to install from.",
			m.releaseList.View(),
			helpStyle.Render("enter: choose release  /: filter  q: quit"),
		}, "\n")
	case stepFamilies:
		summary := fmt.Sprintf(
			"Release %s -> %s (%d selected)",
			m.selectedRelease.TagName,
			m.destination,
			m.selectedCount(),
		)
		return strings.Join([]string{
			titleStyle.Render("Nerd Font Installer"),
			summary,
			m.familyList.View(),
			helpStyle.Render("space: toggle  a: all/none  enter: install  b/esc: back  /: filter  q: quit"),
		}, "\n")
	default:
		return ""
	}
}
