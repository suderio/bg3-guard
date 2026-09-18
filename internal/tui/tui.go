package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/suderio/bg3-guard/internal/backup"
	"github.com/suderio/bg3-guard/internal/config"
	"github.com/suderio/bg3-guard/internal/process"
	"github.com/suderio/bg3-guard/internal/recover"
	"github.com/suderio/bg3-guard/internal/restore"
)

type ViewMode int

const (
	ViewMonitor ViewMode = iota
	ViewRestore
	ViewRecover
)

// Messages
type tickMsg time.Time
type backupCompleteMsg struct {
	count int
	err   error
}
type restoreCompleteMsg struct {
	res *restore.RestoreResult
	err error
}
type recoverCompleteMsg struct {
	res *recover.RecoveryResult
	err error
}
type logMsg string

// Model represents the Bubble Tea application state.
type Model struct {
	cfg            *config.Config
	configFilePath string
	currentView    ViewMode

	// Window dimensions
	width  int
	height int

	// Monitor state
	spinner          spinner.Model
	isBackingUp      bool
	secondsRemaining int
	lastBackupTime   time.Time
	gameRunning      bool
	gameProcName     string
	logs             []string
	maxLogs          int

	// Restore state
	allBackups      []backup.BackupEntry
	filteredBackups []backup.BackupEntry
	restoreCursor   int
	filterInput     textinput.Model
	isFiltering     bool
	showRestoreWarn bool
	selectedBackup  *backup.BackupEntry

	// Recover state
	allSaves           []string
	filteredSaves      []string
	recoverCursor      int
	recoverFilterInput textinput.Model
	isRecoverFiltering bool
	showRecoverWarn    bool
	selectedSave       string
	isRecovering       bool

	// Modal / Feedback status
	statusMessage string
	statusIsError bool
	statusTimer   int
}

// NewModel creates and initializes the TUI model.
func NewModel(cfg *config.Config, configFilePath string) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ColorPrimary)

	ti := textinput.New()
	ti.Placeholder = "Type to search backups..."
	ti.CharLimit = 50
	ti.Width = 35

	rti := textinput.New()
	rti.Placeholder = "e.g. *HonourMode*, *Custom*, *"
	rti.CharLimit = 50
	rti.Width = 35
	rti.SetValue(cfg.SaveFilter)

	return Model{
		cfg:                cfg,
		configFilePath:     configFilePath,
		currentView:        ViewMonitor,
		spinner:            s,
		secondsRemaining:   cfg.IntervalMinutes * 60,
		logs:               []string{fmt.Sprintf("[%s] Baldur's Gate 3 Guard started.", time.Now().Format("15:04:05"))},
		maxLogs:            15,
		filterInput:        ti,
		recoverFilterInput: rti,
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		m.spinner.Tick,
		tickCmd(),
		m.triggerBackupCmd(),
		m.refreshBackupsCmd(),
		m.refreshSavesCmd(),
	)
}

func (m Model) triggerBackupCmd() tea.Cmd {
	return func() tea.Msg {
		count, err := backup.RunBackupCycle(m.cfg, nil)
		return backupCompleteMsg{count: count, err: err}
	}
}

func (m Model) refreshBackupsCmd() tea.Cmd {
	return func() tea.Msg {
		backups, err := backup.GetBackups(m.cfg.DestinationPath)
		if err != nil {
			return logMsg(fmt.Sprintf("Failed to load backups: %v", err))
		}
		// Sort descending
		for i := 0; i < len(backups)-1; i++ {
			for j := i + 1; j < len(backups); j++ {
				if backups[i].Name < backups[j].Name {
					backups[i], backups[j] = backups[j], backups[i]
				}
			}
		}
		return backups
	}
}

func (m Model) refreshSavesCmd() tea.Cmd {
	return func() tea.Msg {
		entries, err := os.ReadDir(m.cfg.SourcePath)
		if err != nil {
			return nil
		}
		var saves []string
		for _, e := range entries {
			if e.IsDir() {
				// verify it has a .lsv file
				files, _ := os.ReadDir(filepath.Join(m.cfg.SourcePath, e.Name()))
				for _, f := range files {
					if strings.HasSuffix(strings.ToLower(f.Name()), ".lsv") {
						saves = append(saves, e.Name())
						break
					}
				}
			}
		}
		return saves
	}
}

func (m *Model) addLog(text string) {
	entry := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), text)
	m.logs = append(m.logs, entry)
	if len(m.logs) > m.maxLogs {
		m.logs = m.logs[len(m.logs)-m.maxLogs:]
	}
}

func (m *Model) updateFilter() {
	query := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	if query == "" {
		m.filteredBackups = m.allBackups
		return
	}
	var filtered []backup.BackupEntry
	for _, b := range m.allBackups {
		if strings.Contains(strings.ToLower(b.Name), query) ||
			strings.Contains(strings.ToLower(restore.FormatTimestamp(b.Name)), query) ||
			strings.Contains(strings.ToLower(b.GroupName), query) {
			filtered = append(filtered, b)
		}
	}
	m.filteredBackups = filtered
	if m.restoreCursor >= len(m.filteredBackups) {
		m.restoreCursor = 0
	}
}

func (m *Model) updateRecoverFilter() {
	pattern := strings.TrimSpace(m.recoverFilterInput.Value())
	if pattern == "" {
		pattern = m.cfg.SaveFilter
	}
	m.filteredSaves = backup.FilterSaveFolders(m.allSaves, pattern)
	if m.recoverCursor >= len(m.filteredSaves) {
		if len(m.filteredSaves) == 0 {
			m.recoverCursor = 0
		} else {
			m.recoverCursor = len(m.filteredSaves) - 1
		}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case tickMsg:
		// 1. Update countdown
		if m.secondsRemaining > 0 {
			m.secondsRemaining--
		} else {
			// Trigger auto backup
			m.secondsRemaining = m.cfg.IntervalMinutes * 60
			m.isBackingUp = true
			cmds = append(cmds, m.triggerBackupCmd())
		}

		// 2. Periodic game process check
		running, name, _ := process.IsGameRunning()
		m.gameRunning = running
		m.gameProcName = name

		// 3. Status timer decay
		if m.statusTimer > 0 {
			m.statusTimer--
			if m.statusTimer == 0 {
				m.statusMessage = ""
			}
		}

		cmds = append(cmds, tickCmd())

	case backupCompleteMsg:
		m.isBackingUp = false
		if msg.err != nil {
			m.addLog(fmt.Sprintf("Backup error: %v", msg.err))
		} else if msg.count > 0 {
			m.lastBackupTime = time.Now()
			m.addLog(fmt.Sprintf("Auto-backup created %d snapshot(s).", msg.count))
			cmds = append(cmds, m.refreshBackupsCmd())
		}

	case []backup.BackupEntry:
		m.allBackups = msg
		m.updateFilter()

	case []string:
		m.allSaves = msg
		m.updateRecoverFilter()

	case restoreCompleteMsg:
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("Restore failed: %v", msg.err)
			m.statusIsError = true
			m.addLog(m.statusMessage)
		} else {
			m.statusMessage = msg.res.Message
			m.statusIsError = false
			m.addLog(msg.res.Message)
		}
		m.statusTimer = 6
		m.showRestoreWarn = false
		m.selectedBackup = nil

	case recoverCompleteMsg:
		m.isRecovering = false
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("Recovery failed: %v", msg.err)
			m.statusIsError = true
			m.addLog(m.statusMessage)
		} else {
			m.statusMessage = msg.res.Message
			m.statusIsError = false
			m.addLog(msg.res.Message)
			m.addLog("IMPORTANT: Keep Steam Cloud OFF until you launch and re-save in game!")
		}
		m.statusTimer = 10
		m.showRecoverWarn = false
		m.selectedSave = ""
		cmds = append(cmds, m.refreshSavesCmd())

	case logMsg:
		m.addLog(string(msg))

	case tea.KeyMsg:
		// Modal / Warning active for Restore
		if m.showRestoreWarn {
			switch msg.String() {
			case "enter", "y", "Y":
				// Confirm restore
				m.showRestoreWarn = false
				if m.selectedBackup != nil {
					target := *m.selectedBackup
					cmds = append(cmds, func() tea.Msg {
						res, err := restore.PerformSafeRestore(m.cfg, target, true, nil)
						return restoreCompleteMsg{res: res, err: err}
					})
				}
			case "esc", "n", "N", "q":
				m.showRestoreWarn = false
				m.selectedBackup = nil
			}
			return m, tea.Batch(cmds...)
		}

		// Modal / Warning active for Recover
		if m.showRecoverWarn {
			switch msg.String() {
			case "enter", "y", "Y":
				m.showRecoverWarn = false
				m.isRecovering = true
				targetSave := m.selectedSave
				cmds = append(cmds, func() tea.Msg {
					res, err := recover.RecoverSave(m.cfg, targetSave, true, nil)
					return recoverCompleteMsg{res: res, err: err}
				})
			case "esc", "n", "N", "q":
				m.showRecoverWarn = false
				m.selectedSave = ""
			}
			return m, tea.Batch(cmds...)
		}

		// Filter input active in restore view
		if m.isFiltering {
			switch msg.String() {
			case "esc", "enter":
				m.isFiltering = false
				m.filterInput.Blur()
			default:
				var cmd tea.Cmd
				m.filterInput, cmd = m.filterInput.Update(msg)
				m.updateFilter()
				return m, cmd
			}
			return m, nil
		}

		// Filter input active in recover view
		if m.isRecoverFiltering {
			switch msg.String() {
			case "enter":
				m.isRecoverFiltering = false
				m.recoverFilterInput.Blur()
				newFilter := strings.TrimSpace(m.recoverFilterInput.Value())
				if newFilter == "" {
					newFilter = "*"
				}
				m.recoverFilterInput.SetValue(newFilter)
				m.cfg.SaveFilter = newFilter
				_ = config.SaveConfig(m.cfg, m.configFilePath)
				m.addLog(fmt.Sprintf("Save filter updated to '%s' (saved to config).", m.cfg.SaveFilter))
				m.updateRecoverFilter()
				return m, nil
			case "esc":
				m.isRecoverFiltering = false
				m.recoverFilterInput.Blur()
				m.recoverFilterInput.SetValue(m.cfg.SaveFilter)
				m.updateRecoverFilter()
				return m, nil
			default:
				var cmd tea.Cmd
				m.recoverFilterInput, cmd = m.recoverFilterInput.Update(msg)
				m.updateRecoverFilter()
				return m, cmd
			}
		}

		// Standard navigation
		switch msg.String() {
		case "ctrl+c", "q":
			// Retention cleanup before exit
			_, _ = backup.CleanupRetention(m.cfg.DestinationPath, m.cfg.RetentionLimit, nil)
			return m, tea.Quit

		case "tab":
			m.currentView = (m.currentView + 1) % 3
		case "1", "b":
			m.currentView = ViewMonitor
		case "2", "r":
			m.currentView = ViewRestore
			cmds = append(cmds, m.refreshBackupsCmd())
		case "3", "h":
			m.currentView = ViewRecover
			cmds = append(cmds, m.refreshSavesCmd())

		case " ":
			// Manual instant backup
			if !m.isBackingUp {
				m.isBackingUp = true
				m.addLog("Manual backup triggered.")
				cmds = append(cmds, m.triggerBackupCmd())
			}

		case "/":
			if m.currentView == ViewRestore {
				m.isFiltering = true
				m.filterInput.Focus()
				return m, textinput.Blink
			} else if m.currentView == ViewRecover {
				m.isRecoverFiltering = true
				m.recoverFilterInput.Focus()
				return m, textinput.Blink
			}

		case "f":
			if m.currentView == ViewRecover {
				m.isRecoverFiltering = true
				m.recoverFilterInput.Focus()
				return m, textinput.Blink
			}

		case "up", "k":
			if m.currentView == ViewRestore && m.restoreCursor > 0 {
				m.restoreCursor--
			} else if m.currentView == ViewRecover && m.recoverCursor > 0 {
				m.recoverCursor--
			}

		case "down", "j":
			if m.currentView == ViewRestore && m.restoreCursor < len(m.filteredBackups)-1 {
				m.restoreCursor++
			} else if m.currentView == ViewRecover && m.recoverCursor < len(m.filteredSaves)-1 {
				m.recoverCursor++
			}

		case "enter":
			if m.currentView == ViewRestore && len(m.filteredBackups) > 0 {
				m.selectedBackup = &m.filteredBackups[m.restoreCursor]
				m.showRestoreWarn = true
			} else if m.currentView == ViewRecover && len(m.filteredSaves) > 0 {
				m.selectedSave = m.filteredSaves[m.recoverCursor]
				m.showRecoverWarn = true
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	s := strings.Builder{}

	// 1. Header
	s.WriteString(m.renderHeader())
	s.WriteString("\n\n")

	// 2. Navigation Tabs
	s.WriteString(m.renderTabs())
	s.WriteString("\n\n")

	// 3. Main Content
	if m.showRestoreWarn {
		s.WriteString(m.renderRestoreWarningModal())
	} else if m.showRecoverWarn {
		s.WriteString(m.renderRecoverWarningModal())
	} else {
		switch m.currentView {
		case ViewMonitor:
			s.WriteString(m.renderMonitorView())
		case ViewRestore:
			s.WriteString(m.renderRestoreView())
		case ViewRecover:
			s.WriteString(m.renderRecoverView())
		}
	}

	// 4. Status Bar / Toast
	if m.statusMessage != "" {
		s.WriteString("\n")
		style := SuccessBoxStyle
		if m.statusIsError {
			style = WarningBoxStyle
		}
		s.WriteString(style.Render(m.statusMessage))
	}

	// 5. Footer / Help
	s.WriteString("\n\n")
	s.WriteString(m.renderFooter())

	return s.String()
}

func (m Model) renderHeader() string {
	title := TitleStyle.Render("🛡️  BALDUR'S GATE 3: GUARD")
	badge := BadgeStyle.Render("HONOUR GUARD ACTIVE")

	gameStatus := lipgloss.NewStyle().Foreground(ColorGray).Render("Game: Idle")
	if m.gameRunning {
		gameStatus = lipgloss.NewStyle().Bold(true).Foreground(ColorGreen).Render(fmt.Sprintf("● Game Running (%s)", m.gameProcName))
	}

	return lipgloss.JoinHorizontal(lipgloss.Center, title, badge, "   ", gameStatus)
}

func (m Model) renderTabs() string {
	tabs := []string{"[1/b] Backup Monitor", "[2/r] Restore Backups", "[3/h] Honour Mode Recover"}
	var rendered []string
	for i, t := range tabs {
		if ViewMode(i) == m.currentView {
			rendered = append(rendered, TabActiveStyle.Render(t))
		} else {
			rendered = append(rendered, TabInactiveStyle.Render(t))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

func (m Model) renderMonitorView() string {
	min := m.secondsRemaining / 60
	sec := m.secondsRemaining % 60
	countdownStr := fmt.Sprintf("%02dm %02ds", min, sec)

	backupSpinner := "●"
	if m.isBackingUp {
		backupSpinner = m.spinner.View()
	}

	configInfo := strings.Builder{}
	configInfo.WriteString(fmt.Sprintf("%s %s\n", HeaderLabel.Render("Config File:"), HeaderVal.Render(m.configFilePath)))
	configInfo.WriteString(fmt.Sprintf("%s %s\n", HeaderLabel.Render("Source Path:"), HeaderVal.Render(m.cfg.SourcePath)))
	configInfo.WriteString(fmt.Sprintf("%s %s\n", HeaderLabel.Render("Destination:"), HeaderVal.Render(m.cfg.DestinationPath)))
	configInfo.WriteString(fmt.Sprintf("%s %s\n", HeaderLabel.Render("Save Filter:"), HeaderVal.Render(m.cfg.SaveFilter)))
	configInfo.WriteString(fmt.Sprintf("%s %s min | %s %d saves\n",
		HeaderLabel.Render("Interval:"), HeaderVal.Render(fmt.Sprintf("%d", m.cfg.IntervalMinutes)),
		HeaderLabel.Render("Retention:"), m.cfg.RetentionLimit))

	statusBox := ActiveBoxStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Center,
			lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("%s Next Backup In: ", backupSpinner)),
			CountdownStyle.Render(countdownStr),
		),
		"",
		configInfo.String(),
	))

	logContent := strings.Builder{}
	logContent.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("Live Activity Log:") + "\n")
	if len(m.logs) == 0 {
		logContent.WriteString(HelpStyle.Render("No events recorded yet."))
	} else {
		for _, l := range m.logs {
			logContent.WriteString(l + "\n")
		}
	}
	logsBox := BoxStyle.Width(80).Render(logContent.String())

	return lipgloss.JoinVertical(lipgloss.Left, statusBox, "", logsBox)
}

func (m Model) renderRestoreView() string {
	s := strings.Builder{}

	// Filter line
	s.WriteString(lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.NewStyle().Bold(true).Render("Search/Filter: "),
		m.filterInput.View(),
		"  ",
		HelpStyle.Render("('/' to focus, 'esc' to unfocus)"),
	))
	s.WriteString("\n\n")

	if len(m.filteredBackups) == 0 {
		s.WriteString(BoxStyle.Render(HelpStyle.Render("No backups found in destination folder.")))
		return s.String()
	}

	content := strings.Builder{}
	content.WriteString(lipgloss.NewStyle().Bold(true).Render("Available Backups (Newest First):") + "\n\n")

	for i, b := range m.filteredBackups {
		cursor := "  "
		if i == m.restoreCursor {
			cursor = "👉"
		}
		formattedDate := restore.FormatTimestamp(b.Name)
		line := fmt.Sprintf("%s [%d] %s (%s)", cursor, i+1, formattedDate, b.Name)

		if i == m.restoreCursor {
			content.WriteString(ListItemSelected.Render(line) + "\n")
		} else {
			content.WriteString(ListItemNormal.Render(line) + "\n")
		}
	}

	s.WriteString(BoxStyle.Width(90).Render(content.String()))
	s.WriteString("\n" + HelpStyle.Render("Press [Enter] to restore selected snapshot. (A pre-restore backup of the current save is always made)"))

	return s.String()
}

func (m Model) renderRestoreWarningModal() string {
	s := strings.Builder{}
	warnHeader := lipgloss.NewStyle().Bold(true).Foreground(ColorRed).Render("⚠️  RESTORE CONFIRMATION & SAFETY CHECK")
	s.WriteString(warnHeader + "\n\n")

	if m.gameRunning {
		warnText := fmt.Sprintf(
			"DANGER: Baldur's Gate 3 is currently running (%s)!\n"+
				"Restoring save files while the game is active can cause save corruption or immediate crashes.\n"+
				"We strongly advise closing the game before restoring.", m.gameProcName)
		s.WriteString(WarningBoxStyle.Render(warnText))
		s.WriteString("\n\n")
	}

	target := m.selectedBackup
	s.WriteString(fmt.Sprintf("Target Save Campaign: %s\n", lipgloss.NewStyle().Bold(true).Render(target.GroupName)))
	s.WriteString(fmt.Sprintf("Selected Snapshot:    %s\n", restore.FormatTimestamp(target.Name)))
	s.WriteString("\n" + lipgloss.NewStyle().Foreground(ColorGreen).Render("✔ A safety backup of your CURRENT active save will be created before restoring.") + "\n")
	s.WriteString(lipgloss.NewStyle().Foreground(ColorGreen).Render("✔ Transactional rollback is enabled if copy fails.") + "\n\n")
	s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("Are you sure you want to restore? [Enter / y] Confirm  |  [Esc / n] Cancel"))

	return BoxStyle.Width(85).BorderForeground(ColorPrimary).Render(s.String())
}

func (m Model) renderRecoverView() string {
	s := strings.Builder{}

	// Cloud sync warning banner
	s.WriteString(WarningBoxStyle.Width(85).Render(
		lipgloss.NewStyle().Bold(true).Render("⚠️  IMPORTANT NOTICE FOR HONOUR MODE RECOVERY:\n") +
			"\n1. Ensure Baldur's Gate 3 is COMPLETELY CLOSED before recovering.\n" +
			"2. DISABLE Steam Cloud / Larian Cross-Save before launching with recovered save.\n" +
			"   (Cloud sync will overwrite your local restored save with the dead cloud state!)\n" +
			"3. Launch BG3, load the save, save once in-game, then re-enable cloud sync."))
	s.WriteString("\n\n")

	// Filter bar
	filterBar := strings.Builder{}
	if m.isRecoverFiltering {
		filterBar.WriteString(fmt.Sprintf("%s %s  %s",
			lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("Filter:"),
			m.recoverFilterInput.View(),
			HelpStyle.Render("[Enter] Apply & Save | [Esc] Cancel"),
		))
	} else {
		filterBar.WriteString(fmt.Sprintf("%s %s   %s   %s",
			HeaderLabel.Render("Save Filter:"),
			HeaderVal.Render(m.cfg.SaveFilter),
			HelpStyle.Render("[/] or [f] to change"),
			HelpStyle.Render(fmt.Sprintf("(Matching %d of %d saves)", len(m.filteredSaves), len(m.allSaves))),
		))
	}
	s.WriteString(BoxStyle.Width(85).Render(filterBar.String()))
	s.WriteString("\n\n")

	if len(m.allSaves) == 0 {
		s.WriteString(BoxStyle.Render(HelpStyle.Render("No BG3 campaign save folders found in " + m.cfg.SourcePath)))
		return s.String()
	}

	if len(m.filteredSaves) == 0 {
		emptyMsg := fmt.Sprintf("No save folders matching filter '%s' found (total saves: %d).\n\nPress [/] or [f] to change the filter (e.g. '*' to show all).", m.cfg.SaveFilter, len(m.allSaves))
		s.WriteString(BoxStyle.Width(85).Render(HelpStyle.Render(emptyMsg)))
		return s.String()
	}

	content := strings.Builder{}
	content.WriteString(lipgloss.NewStyle().Bold(true).Render("Select Save Campaign to Recover / Reactivate Honour Mode:") + "\n\n")

	for i, saveName := range m.filteredSaves {
		cursor := "  "
		if i == m.recoverCursor {
			cursor = "👉"
		}
		line := fmt.Sprintf("%s [%d] %s", cursor, i+1, saveName)
		if i == m.recoverCursor {
			content.WriteString(ListItemSelected.Render(line) + "\n")
		} else {
			content.WriteString(ListItemNormal.Render(line) + "\n")
		}
	}

	s.WriteString(BoxStyle.Width(85).Render(content.String()))
	s.WriteString("\n" + HelpStyle.Render("Press [Enter] to recover selected save. (A pre-recovery safeguard backup is always created)"))

	return s.String()
}

func (m Model) renderRecoverWarningModal() string {
	s := strings.Builder{}
	warnHeader := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("🔮  CONFIRM HONOUR MODE REACTIVATION")
	s.WriteString(warnHeader + "\n\n")

	if m.gameRunning {
		warnText := fmt.Sprintf(
			"STOP! Baldur's Gate 3 is currently running (%s)!\n"+
				"You MUST quit the game before recovering.", m.gameProcName)
		s.WriteString(WarningBoxStyle.Render(warnText))
		s.WriteString("\n\n")
	}

	s.WriteString(fmt.Sprintf("Target Campaign: %s\n\n", lipgloss.NewStyle().Bold(true).Render(m.selectedSave)))
	s.WriteString("Automated recovery actions:\n")
	s.WriteString("  1. Create pre-recovery safeguard backup: Custom-" + m.selectedSave + "-<timestamp>\n")
	s.WriteString("  2. Patch meta.lsf inside .lsv (Restore Honour RuleSet GUID & Difficulty)\n")
	s.WriteString("  3. Clean deactivated session from profile8.lsf (DisabledSingleSaveSessions)\n\n")
	s.WriteString(lipgloss.NewStyle().Foreground(ColorRed).Render("REMINDER: Disable Steam Cloud before loading game!") + "\n\n")
	s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("Proceed with recovery? [Enter / y] Yes  |  [Esc / n] Cancel"))

	return BoxStyle.Width(85).BorderForeground(ColorPrimary).Render(s.String())
}

func (m Model) renderFooter() string {
	switch m.currentView {
	case ViewRestore:
		return HelpStyle.Render("Keys: [Tab] Switch Tab | [↑/↓] Select Snapshot | [Enter] Restore | [/] Filter | [q] Quit")
	case ViewRecover:
		return HelpStyle.Render("Keys: [Tab] Switch Tab | [↑/↓] Select Campaign | [Enter] Recover | [/] or [f] Change Filter | [q] Quit")
	default:
		return HelpStyle.Render("Keys: [Tab] Switch Tab | [Space] Backup Now | [r] Restore | [h] Recover | [q] Quit")
	}
}
