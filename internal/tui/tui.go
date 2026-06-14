package tui

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
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
	Icons            IconMode
}

type step int

const (
	stepRelease step = iota
	stepFamilies
	stepDone
)

// Neon / synthwave palette. Colors are true-color hex; lipgloss + termenv
// downsample them to whatever the terminal actually supports, so they stay
// safe on 256-color and 16-color terminals too.
const (
	cPink    = "#FF5FAF"
	cMagenta = "#C75CFF"
	cViolet  = "#8A7CFF"
	cBlue    = "#5BA8FF"
	cCyan    = "#46E5E0"
	cMint    = "#5BF0B8"
	cText    = "#EDEDF7"
	cMuted   = "#A2A2BE"
	cFaint   = "#595972"
	cAmber   = "#FFC857"
	cGreen   = "#54E08A"
	cRed     = "#FF5C7A"
	cInk     = "#13131F"
	cPanelHi = "#2E2A57"
	cDescHi  = "#CFCBF2"
)

// brandRamp is the left-to-right gradient used for the wordmark, rules and the
// progress bar fill. See gradientText.
var brandRamp = []string{cPink, cMagenta, cViolet, cCyan, cMint}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(cText))
	bannerStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(cViolet)).
			Padding(1, 2)
	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(cMuted)).
			Italic(true)
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(cFaint))
	keyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(cCyan)).
			Bold(true)
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(cRed)).
			Bold(true)
	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(cGreen)).
			Bold(true)
	accentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(cCyan)).
			Bold(true)
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(cFaint)).
			Padding(1, 2)
	activePanelStyle = panelStyle.
				BorderForeground(lipgloss.Color(cCyan))
	sidePanelStyle = panelStyle.
			BorderForeground(lipgloss.Color(cViolet))
	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(cMuted)).
			Bold(true)
	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(cText))
	progressTrackStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(cFaint))
	spinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(cCyan))

	// badges are the small filled pills in the banner meta row.
	badgePkg = lipgloss.NewStyle().
			Foreground(lipgloss.Color(cInk)).
			Background(lipgloss.Color(cPink)).
			Bold(true).
			Padding(0, 1)
	badgeFont = lipgloss.NewStyle().
			Foreground(lipgloss.Color(cInk)).
			Background(lipgloss.Color(cCyan)).
			Bold(true).
			Padding(0, 1)
	badgeLaunch = lipgloss.NewStyle().
			Foreground(lipgloss.Color(cInk)).
			Background(lipgloss.Color(cMint)).
			Bold(true).
			Padding(0, 1)

	// breadcrumb styles for the release › families › install stepper.
	crumbActive = lipgloss.NewStyle().Foreground(lipgloss.Color(cCyan)).Bold(true)
	crumbDone   = lipgloss.NewStyle().Foreground(lipgloss.Color(cGreen))
	crumbTodo   = lipgloss.NewStyle().Foreground(lipgloss.Color(cFaint))
	crumbSep    = lipgloss.NewStyle().Foreground(lipgloss.Color(cFaint))
)

// gradientText paints s across the given color stops, interpolating per rune so
// a short word still shows the full sweep. ANSI styling is applied per rune; the
// underlying glyphs stay intact, so substring checks on the rendered string
// (single glyphs) still match.
func gradientText(s string, stops []string) string {
	runes := []rune(s)
	n := len(runes)
	if n == 0 || len(stops) == 0 {
		return s
	}
	if len(stops) == 1 || n == 1 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(stops[0])).Render(s)
	}

	var b strings.Builder
	for i, r := range runes {
		t := float64(i) / float64(n-1)
		color := lipgloss.Color(rampColor(stops, t))
		b.WriteString(lipgloss.NewStyle().Foreground(color).Render(string(r)))
	}
	return b.String()
}

// rampColor returns the hex color at position t in [0,1] along stops.
func rampColor(stops []string, t float64) string {
	switch {
	case t <= 0:
		return stops[0]
	case t >= 1:
		return stops[len(stops)-1]
	}
	seg := t * float64(len(stops)-1)
	i := int(seg)
	return lerpHex(stops[i], stops[i+1], seg-float64(i))
}

// lerpHex linearly interpolates between two "#RRGGBB" colors.
func lerpHex(a, b string, t float64) string {
	ar, ag, ab := hexRGB(a)
	br, bg, bb := hexRGB(b)
	lerp := func(x, y int) int { return x + int(float64(y-x)*t+0.5) }
	return fmt.Sprintf("#%02X%02X%02X", lerp(ar, br), lerp(ag, bg), lerp(ab, bb))
}

func hexRGB(h string) (int, int, int) {
	h = strings.TrimPrefix(h, "#")
	if len(h) != 6 {
		return 255, 255, 255
	}
	r, _ := strconv.ParseInt(h[0:2], 16, 0)
	g, _ := strconv.ParseInt(h[2:4], 16, 0)
	b, _ := strconv.ParseInt(h[4:6], 16, 0)
	return int(r), int(g), int(b)
}

// gradientRule draws a full-width horizontal rule in the brand gradient.
func gradientRule(width int) string {
	if width < 1 {
		return ""
	}
	return gradientText(strings.Repeat("─", width), brandRamp)
}

// spread places left and right on one line of the given width, pushing right to
// the far edge. It falls back to a single-space join when there is no room.
func spread(width int, left, right string) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return left + " " + right
	}
	return left + strings.Repeat(" ", gap) + right
}

const (
	IconAuto    IconMode = "auto"
	IconNerd    IconMode = "nerd"
	IconUnicode IconMode = "unicode"
	IconASCII   IconMode = "ascii"
)

type IconMode string

type iconSet struct {
	Mode       IconMode
	Title      string
	Package    string
	Release    string
	Font       string
	Folder     string
	Checked    string
	Unchecked  string
	Selected   string
	Ready      string
	Launch     string
	Toolbox    string
	Separator  string
	NerdFamily map[string]string
}

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
	return strings.Join([]string{i.title, i.description, i.value}, " ")
}

type model struct {
	step             step
	releases         []nerdfonts.Release
	releaseList      list.Model
	familyList       list.Model
	icons            iconSet
	selectedFamilies map[string]bool
	selectedRelease  nerdfonts.Release
	destination      string
	refreshFontCache bool
	cancelled        bool
	err              error
	width            int
	height           int
}

type loadReleasesMsg struct {
	releases []nerdfonts.Release
	err      error
}

type loadingModel struct {
	spinner spinner.Model
	load    func(context.Context) ([]nerdfonts.Release, error)
	ctx     context.Context
	message string
	state   *loadingState
}

type loadingState struct {
	releases []nerdfonts.Release
	err      error
	done     bool
}

func LoadReleases(
	ctx context.Context,
	load func(context.Context) ([]nerdfonts.Release, error),
	output io.Writer,
) ([]nerdfonts.Release, error) {
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = spinnerStyle

	programOptions := []tea.ProgramOption{
		tea.WithContext(ctx),
		tea.WithInput(nil),
		tea.WithoutSignalHandler(),
	}
	if output != nil {
		programOptions = append(programOptions, tea.WithOutput(output))
	}

	program := tea.NewProgram(loadingModel{
		spinner: s,
		load:    load,
		ctx:     ctx,
		message: "Loading Nerd Fonts releases",
		state:   &loadingState{},
	}, programOptions...)
	finalModel, err := program.Run()
	if err != nil {
		return nil, err
	}

	m, ok := finalModel.(loadingModel)
	if !ok {
		return nil, fmt.Errorf("unexpected loading model %T", finalModel)
	}
	if !m.state.done {
		return nil, fmt.Errorf("release loader exited before completion")
	}
	return m.state.releases, m.state.err
}

func (m loadingModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		releases, err := m.load(m.ctx)
		return loadReleasesMsg{releases: releases, err: err}
	})
}

func (m loadingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loadReleasesMsg:
		m.state.releases = msg.releases
		m.state.err = msg.err
		m.state.done = true
		if msg.err != nil {
			m.message = errorStyle.Render(msg.err.Error())
			return m, tea.Quit
		}
		m.message = successStyle.Render("✓ Releases loaded")
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func (m loadingModel) View() string {
	brand := gradientText("✦ nerdfont-install", brandRamp)
	return fmt.Sprintf("\n  %s\n  %s %s\n", brand, spinnerStyle.Render(m.spinner.View()), accentStyle.Render(m.message))
}

func Run(ctx context.Context, releases []nerdfonts.Release, opts Options) (Result, error) {
	if len(releases) == 0 {
		return Result{}, fmt.Errorf("no Nerd Fonts releases available")
	}

	destination := opts.Destination
	if strings.TrimSpace(destination) == "" {
		destination = "~/.local/share/fonts/NerdFonts"
	}

	m := newModel(releases, destination, opts.RefreshFontCache, opts.Icons)
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
	return m.result()
}

// result derives the install configuration from the final model state. It is
// separated from Run so the selection-to-config mapping (sorted families, and
// "finished with nothing selected" treated as a cancel) is unit-testable
// without driving a real terminal program.
func (m model) result() (Result, error) {
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

func newModel(releases []nerdfonts.Release, destination string, refreshFontCache bool, iconMode IconMode) model {
	icons := resolveIconSet(iconMode)
	items := make([]list.Item, 0, len(releases))
	for _, release := range releases {
		description := fmt.Sprintf("%s  %d font archives  %s  %s ready for terminals and editors",
			icons.Font,
			len(release.Families),
			icons.Separator,
			icons.Toolbox,
		)
		items = append(items, item{
			title:       icons.Release + " " + release.TagName,
			description: description,
			value:       release.TagName,
		})
	}

	delegate := newDelegate()
	releaseList := list.New(items, delegate, 0, 0)
	releaseList.Title = icons.Package + "  Select Nerd Fonts release"
	configureList(&releaseList, "release", "releases")

	m := model{
		step:             stepRelease,
		releases:         releases,
		releaseList:      releaseList,
		icons:            icons,
		selectedFamilies: map[string]bool{},
		destination:      destination,
		refreshFontCache: refreshFontCache,
		width:            96,
		height:           32,
	}
	listWidth, listHeight := m.listSize()
	m.releaseList = setListSize(m.releaseList, listWidth, listHeight)
	return m
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		listWidth, listHeight := m.listSize()
		m.releaseList = setListSize(m.releaseList, listWidth, listHeight)
		if m.step == stepFamilies {
			m.familyList = setListSize(m.familyList, listWidth, listHeight)
		}
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
		if release.TagName == selected.value {
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
	delegate := newDelegate()
	listWidth, listHeight := m.listSize()
	familyList := list.New(m.familyItems(), delegate, listWidth, listHeight)
	familyList.Title = m.icons.Title + "  Select font families"
	configureList(&familyList, "font", "fonts")
	return familyList
}

func configureList(model *list.Model, singular, plural string) {
	model.SetShowStatusBar(true)
	model.SetStatusBarItemName(singular, plural)
	model.SetFilteringEnabled(true)
	model.SetShowHelp(false)
	model.DisableQuitKeybindings()
	model.Styles.Title = model.Styles.Title.
		Foreground(lipgloss.Color(cInk)).
		Background(lipgloss.Color(cCyan)).
		Bold(true).
		Padding(0, 1)
	model.Styles.TitleBar = model.Styles.TitleBar.
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(lipgloss.Color(cFaint)).
		PaddingBottom(1)
	model.Styles.StatusBar = model.Styles.StatusBar.
		Foreground(lipgloss.Color(cMuted)).
		PaddingTop(1)
	model.Styles.StatusBarActiveFilter = model.Styles.StatusBarActiveFilter.
		Foreground(lipgloss.Color(cAmber)).
		Bold(true)
	model.Styles.StatusBarFilterCount = model.Styles.StatusBarFilterCount.
		Foreground(lipgloss.Color(cPink))
	model.Styles.PaginationStyle = helpStyle
	model.Styles.HelpStyle = helpStyle
}

func (m model) familyItems() []list.Item {
	items := make([]list.Item, 0, len(m.selectedRelease.Families))
	for _, family := range m.selectedRelease.Families {
		marker := m.icons.Unchecked
		if m.selectedFamilies[family] {
			marker = m.icons.Checked
		}
		items = append(items, item{
			title:       marker + "  " + m.iconForFamily(family) + "  " + family,
			description: fmt.Sprintf("%s %s  %s  %s", m.icons.Release, m.selectedRelease.TagName, m.icons.Separator, familyHint(family)),
			value:       family,
		})
	}
	return items
}

func newDelegate() list.DefaultDelegate {
	delegate := list.NewDefaultDelegate()
	delegate.SetSpacing(1)
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color(cText)).
		BorderForeground(lipgloss.Color(cPink)).
		Background(lipgloss.Color(cPanelHi)).
		Bold(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color(cDescHi)).
		BorderForeground(lipgloss.Color(cPink)).
		Background(lipgloss.Color(cPanelHi))
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.
		Foreground(lipgloss.Color(cText)).
		Bold(true)
	delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.Foreground(lipgloss.Color(cMuted))
	delegate.Styles.FilterMatch = delegate.Styles.FilterMatch.
		Foreground(lipgloss.Color(cAmber)).
		Underline(true).
		Bold(true)
	return delegate
}

func (m model) iconForFamily(family string) string {
	key := strings.ToLower(strings.ReplaceAll(family, " ", ""))
	if icon, ok := m.icons.NerdFamily[key]; ok {
		return icon
	}
	return m.icons.Font
}

func familyHint(family string) string {
	key := strings.ToLower(family)
	switch {
	case strings.Contains(key, "mono"):
		return "monospace favorite"
	case strings.Contains(key, "code"):
		return "coding ligatures"
	case strings.Contains(key, "symbol"):
		return "glyph toolkit"
	default:
		return "Nerd Font patched"
	}
}

// setListSize clamps to positive dimensions (bubbles' list.SetSize panics on a
// zero/negative size) and recovers as a belt-and-suspenders guard against any
// other size-related panic from the dependency, returning the unresized model
// rather than crashing the TUI on a pathological terminal size.
func setListSize(model list.Model, width, height int) (resized list.Model) {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	defer func() {
		if recover() != nil {
			resized = model
		}
	}()
	model.SetSize(width, height)
	return model
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
		return m.releaseView()
	case stepFamilies:
		return m.familiesView()
	case stepDone:
		return m.doneView()
	default:
		return ""
	}
}

func (m model) releaseView() string {
	header := m.banner("Choose a release", "Pick the Nerd Fonts tag to browse, then filter or confirm.")
	body := m.screenBody(m.releaseList.View(), m.releasePreview())
	return m.screen(header, body, help("enter", "choose release", "/", "filter", "esc/q", "quit"))
}

func (m model) familiesView() string {
	selected := fmt.Sprintf("%d/%d selected", m.selectedCount(), len(m.selectedRelease.Families))
	header := m.banner("Build your font set", selected+" for "+m.selectedRelease.TagName)
	body := m.screenBody(m.familyList.View(), m.familyPreview())
	return m.screen(
		header,
		body,
		help("space", "toggle", "a", "all/none", "enter", "install", "b/esc", "back", "/", "filter", "q", "quit"),
	)
}

func (m model) doneView() string {
	header := m.banner("Ready to install", fmt.Sprintf("%d families selected", m.selectedCount()))
	bodyWidth := m.bodyWidth() - 6
	body := activePanelStyle.Width(bodyWidth).Render(strings.Join([]string{
		successStyle.Render(m.icons.Ready + "  Selection locked in"),
		gradientRule(bodyWidth - 4),
		statLine(m.icons.Release, "Release", m.selectedRelease.TagName),
		statLine(m.icons.Folder, "Destination", m.destination),
		statLine(m.icons.Selected, "Families", fmt.Sprintf("%d", m.selectedCount())),
		"",
		gradientText(m.icons.Launch+"  Launching the installer…", brandRamp),
	}, "\n"))
	return m.screen(header, body, help("enter", "continue"))
}

func (m model) screen(header, body, footer string) string {
	return strings.Join([]string{header, body, footer}, "\n\n")
}

func (m model) screenBody(listView, preview string) string {
	listPanel := activePanelStyle.Width(m.listPanelWidth()).Render(listView)
	if !m.wideLayout() {
		return strings.Join([]string{listPanel, preview}, "\n")
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, listPanel, "  ", preview)
}

func (m model) banner(stepLabel, detail string) string {
	boxWidth := m.bodyWidth() - 6
	textWidth := boxWidth - 4 // account for the banner's horizontal padding

	wordmark := gradientText(m.logo()+"  nerdfont-install", brandRamp)
	header := wordmark
	if m.wideLayout() {
		header = spread(textWidth, wordmark, m.breadcrumb())
	}

	title := titleStyle.Render(stepLabel)
	meta := strings.Join([]string{
		badgePkg.Render(m.icons.Package + " CLI"),
		badgeFont.Render(m.icons.Font + " patched glyphs"),
		badgeLaunch.Render(m.icons.Launch + " terminal-ready"),
	}, " ")
	lines := []string{
		header,
		gradientRule(textWidth),
		title,
		subtitleStyle.Render(detail),
		meta,
	}
	return bannerStyle.Width(boxWidth).Render(strings.Join(lines, "\n"))
}

// breadcrumb renders the release › families › install stepper, highlighting the
// current step and marking completed ones.
func (m model) breadcrumb() string {
	labels := []string{"release", "families", "install"}
	current := int(m.step)
	parts := make([]string, len(labels))
	for i, label := range labels {
		switch {
		case i == current:
			parts[i] = crumbActive.Render("◉ " + label)
		case i < current:
			parts[i] = crumbDone.Render("✓ " + label)
		default:
			parts[i] = crumbTodo.Render("○ " + label)
		}
	}
	return strings.Join(parts, crumbSep.Render(" › "))
}

func (m model) logo() string {
	switch m.icons.Mode {
	case IconASCII:
		return "[NF]"
	default:
		return "✦ NF ✦"
	}
}

// panelTitle renders a side-panel heading with a brand-colored accent bar and a
// gradient label.
func panelTitle(label string) string {
	return accentStyle.Render("▌") + " " + gradientText(label, brandRamp)
}

func (m model) releasePreview() string {
	release := m.currentRelease()
	lines := []string{
		panelTitle(m.icons.Package + " Release cockpit"),
		"",
		statLine(m.icons.Release, "Current", release.TagName),
		statLine(m.icons.Font, "Archives", fmt.Sprintf("%d", len(release.Families))),
		statLine(m.icons.Toolbox, "Mode", string(m.icons.Mode)),
		"",
		subtitleStyle.Render("Use filtering to jump across releases. Press enter to open the selected archive catalog."),
	}
	return sidePanelStyle.Width(m.previewWidth()).Render(strings.Join(lines, "\n"))
}

func (m model) familyPreview() string {
	total := len(m.selectedRelease.Families)
	selected := m.selectedCount()
	lines := []string{
		panelTitle(m.icons.Title + " Install plan"),
		"",
		statLine(m.icons.Release, "Release", m.selectedRelease.TagName),
		statLine(m.icons.Folder, "Destination", m.destination),
		statLine(m.icons.Selected, "Selected", fmt.Sprintf("%d of %d", selected, total)),
		"",
		m.progressBar(selected, total),
		"",
		subtitleStyle.Render("Toggle families with space. Select all when bootstrapping a new terminal profile."),
	}
	return sidePanelStyle.Width(m.previewWidth()).Render(strings.Join(lines, "\n"))
}

func (m model) currentRelease() nerdfonts.Release {
	selected, ok := m.releaseList.SelectedItem().(item)
	if ok {
		for _, release := range m.releases {
			if release.TagName == selected.value {
				return release
			}
		}
	}
	return m.releases[0]
}

func (m model) progressBar(selected, total int) string {
	const cells = 24
	filled := 0
	if total > 0 {
		filled = selected * cells / total
	}
	if filled > cells {
		filled = cells
	}
	bar := gradientText(strings.Repeat("█", filled), brandRamp) +
		progressTrackStyle.Render(strings.Repeat("░", cells-filled))
	return bar + "  " + accentStyle.Render(fmt.Sprintf("%3d%%", percentage(selected, total)))
}

func percentage(selected, total int) int {
	if total == 0 {
		return 0
	}
	return selected * 100 / total
}

func statLine(icon, label, value string) string {
	return fmt.Sprintf(
		"%s  %s  %s",
		accentStyle.Render(icon),
		labelStyle.Width(12).Render(label),
		valueStyle.Render(value),
	)
}

func (m model) listSize() (int, int) {
	height := m.safeHeight() - 15
	if height < 8 {
		height = 8
	}
	if m.wideLayout() {
		return m.listPanelWidth() - 6, height
	}
	return m.bodyWidth() - 8, height
}

func (m model) wideLayout() bool {
	return m.safeWidth() >= 104
}

func (m model) bodyWidth() int {
	width := m.safeWidth()
	if width > 132 {
		return 132
	}
	return width
}

func (m model) listPanelWidth() int {
	if !m.wideLayout() {
		return m.bodyWidth() - 4
	}
	return m.bodyWidth() - m.previewWidth() - 8
}

func (m model) previewWidth() int {
	if !m.wideLayout() {
		return m.bodyWidth() - 4
	}
	return 34
}

func (m model) safeWidth() int {
	if m.width < 48 {
		return 48
	}
	return m.width
}

func (m model) safeHeight() int {
	if m.height < 24 {
		return 24
	}
	return m.height
}

func help(parts ...string) string {
	if len(parts)%2 != 0 {
		return helpStyle.Render(strings.Join(parts, " "))
	}

	segments := make([]string, 0, len(parts)/2)
	for i := 0; i+1 < len(parts); i += 2 {
		segments = append(segments, keyStyle.Render(parts[i])+helpStyle.Render(": "+parts[i+1]))
	}
	return strings.Join(segments, helpStyle.Render("  •  "))
}
