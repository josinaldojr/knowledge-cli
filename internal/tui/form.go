package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type FormField struct {
	Arg          ArgDefinition
	TextInput    textinput.Model
	TextArea     textarea.Model
	SelectIndex  int
	MultiChoices []string
	MultiChecked []bool
	MultiHover   int // highlighted option in select/multiselect
	BoolValue    bool
}

type Form struct {
	Command      Command
	Fields       []FormField
	ActiveIndex  int // 0 to len(Fields), where len(Fields) is the Submit button
	SubmitActive bool
}

func NewForm(cmd Command, workspaceApps []string, existingSessions []string, workspaceWorkflows []string, workspaceTasks []string, opencodeModels []string) *Form {
	var fields []FormField

	for _, arg := range cmd.Args {
		ff := FormField{Arg: arg}

		// Resolve choices source
		var choices []string
		if arg.ChoicesSrc == "apps" {
			choices = workspaceApps
		} else if arg.ChoicesSrc == "sessions" {
			choices = existingSessions
		} else if arg.ChoicesSrc == "workflows" {
			choices = workspaceWorkflows
		} else if arg.ChoicesSrc == "tasks" {
			choices = workspaceTasks
		} else if arg.ChoicesSrc == "models" {
			choices = opencodeModels
		} else {
			choices = arg.Choices
		}

		switch arg.Type {
		case InputTypeText:
			ti := textinput.New()
			ti.Placeholder = arg.Description
			if arg.DefaultValue != "" {
				ti.SetValue(arg.DefaultValue)
			}
			ff.TextInput = ti

		case InputTypeTextArea:
			ta := textarea.New()
			ta.Placeholder = arg.Description
			if arg.DefaultValue != "" {
				ta.SetValue(arg.DefaultValue)
			}
			ta.SetWidth(60)
			ta.SetHeight(4)
			ff.TextArea = ta

		case InputTypeSelect:
			ff.MultiChoices = choices
			ff.SelectIndex = 0
			// Find default if set
			if arg.DefaultValue != "" {
				for idx, ch := range choices {
					if ch == arg.DefaultValue {
						ff.SelectIndex = idx
						break
					}
				}
			}

		case InputTypeMultiSelect:
			ff.MultiChoices = choices
			ff.MultiChecked = make([]bool, len(choices))
			// Optionally check all or match defaults
			if arg.DefaultValue != "" {
				defaults := strings.Split(arg.DefaultValue, ",")
				for _, d := range defaults {
					d = strings.TrimSpace(d)
					for idx, ch := range choices {
						if ch == d {
							ff.MultiChecked[idx] = true
						}
					}
				}
			}

		case InputTypeBoolean:
			ff.BoolValue = arg.DefaultValue == "true"
		}

		fields = append(fields, ff)
	}

	f := &Form{
		Command:     cmd,
		Fields:      fields,
		ActiveIndex: 0,
	}

	// Focus the first field
	f.focusActive()
	return f
}

func (f *Form) focusActive() {
	f.SubmitActive = false
	for idx := range f.Fields {
		f.Fields[idx].TextInput.Blur()
		f.Fields[idx].TextArea.Blur()
	}

	if len(f.Fields) == 0 {
		f.SubmitActive = true
		return
	}

	if f.ActiveIndex == len(f.Fields) {
		f.SubmitActive = true
		return
	}

	field := &f.Fields[f.ActiveIndex]
	if field.Arg.Type == InputTypeText {
		field.TextInput.Focus()
	} else if field.Arg.Type == InputTypeTextArea {
		field.TextArea.Focus()
	}
}

func (f *Form) UpdateWidth(width int) {
	for idx := range f.Fields {
		field := &f.Fields[idx]
		if field.Arg.Type == InputTypeText {
			field.TextInput.Width = width - 12
		} else if field.Arg.Type == InputTypeTextArea {
			field.TextArea.SetWidth(width - 12)
		}
	}
}

func (f *Form) Update(msg tea.Msg) (*Form, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			if len(f.Fields) > 0 {
				f.ActiveIndex = (f.ActiveIndex + 1) % (len(f.Fields) + 1)
				f.focusActive()
			}
			return f, nil

		case "shift+tab":
			if len(f.Fields) > 0 {
				f.ActiveIndex = (f.ActiveIndex - 1 + len(f.Fields) + 1) % (len(f.Fields) + 1)
				f.focusActive()
			}
			return f, nil

		case "enter":
			if f.SubmitActive {
				// Form submitted
				return f, func() tea.Msg { return FormSubmitMsg{} }
			}
			if len(f.Fields) > 0 && f.ActiveIndex < len(f.Fields) {
				field := &f.Fields[f.ActiveIndex]
				if field.Arg.Type == InputTypeMultiSelect {
					if field.MultiHover >= 0 && field.MultiHover < len(field.MultiChoices) {
						field.MultiChecked[field.MultiHover] = !field.MultiChecked[field.MultiHover]
					}
					return f, nil
				}
				if field.Arg.Type == InputTypeTextArea {
					// Pass enter to the textarea to insert a newline
					var cmd tea.Cmd
					field.TextArea, cmd = field.TextArea.Update(msg)
					return f, cmd
				}
			}
			// If not submit, Tab forward
			if len(f.Fields) > 0 {
				f.ActiveIndex = (f.ActiveIndex + 1) % (len(f.Fields) + 1)
				f.focusActive()
			}
			return f, nil
		}

		// Pass input handling to the focused field
		if !f.SubmitActive && len(f.Fields) > 0 {
			field := &f.Fields[f.ActiveIndex]
			switch field.Arg.Type {
			case InputTypeText:
				var cmd tea.Cmd
				field.TextInput, cmd = field.TextInput.Update(msg)
				cmds = append(cmds, cmd)

			case InputTypeTextArea:
				var cmd tea.Cmd
				field.TextArea, cmd = field.TextArea.Update(msg)
				cmds = append(cmds, cmd)

			case InputTypeSelect:
				switch msg.String() {
				case "up", "k":
					if field.MultiHover > 0 {
						field.MultiHover--
						field.SelectIndex = field.MultiHover
					}
				case "down", "j":
					if field.MultiHover < len(field.MultiChoices)-1 {
						field.MultiHover++
						field.SelectIndex = field.MultiHover
					}
				}

			case InputTypeMultiSelect:
				switch msg.String() {
				case "up", "k":
					if field.MultiHover > 0 {
						field.MultiHover--
					}
				case "down", "j":
					if field.MultiHover < len(field.MultiChoices)-1 {
						field.MultiHover++
					}
				case " ":
					if field.MultiHover >= 0 && field.MultiHover < len(field.MultiChoices) {
						field.MultiChecked[field.MultiHover] = !field.MultiChecked[field.MultiHover]
					}
				}

			case InputTypeBoolean:
				if msg.String() == " " || msg.String() == "t" || msg.String() == "f" || msg.String() == "y" || msg.String() == "n" {
					field.BoolValue = !field.BoolValue
				}
			}
		}
	}

	return f, tea.Batch(cmds...)
}

type FormSubmitMsg struct{}

func (f *Form) GetValues() map[string]string {
	values := make(map[string]string)
	for _, field := range f.Fields {
		switch field.Arg.Type {
		case InputTypeText:
			values[field.Arg.Key] = field.TextInput.Value()
		case InputTypeTextArea:
			values[field.Arg.Key] = field.TextArea.Value()
		case InputTypeSelect:
			if len(field.MultiChoices) > 0 && field.SelectIndex >= 0 && field.SelectIndex < len(field.MultiChoices) {
				values[field.Arg.Key] = field.MultiChoices[field.SelectIndex]
			} else {
				values[field.Arg.Key] = ""
			}
		case InputTypeMultiSelect:
			var checked []string
			for idx, ch := range field.MultiChoices {
				if field.MultiChecked[idx] {
					checked = append(checked, ch)
				}
			}
			values[field.Arg.Key] = strings.Join(checked, ",")
		case InputTypeBoolean:
			if field.BoolValue {
				values[field.Arg.Key] = "true"
			} else {
				values[field.Arg.Key] = "false"
			}
		}
	}
	return values
}

// Render the form view
func (f *Form) View(themeTheme lipgloss.Style) string {
	var s strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4")).MarginBottom(1)
	s.WriteString(titleStyle.Render(fmt.Sprintf("Configure: %s", f.Command.DisplayName)) + "\n")
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8B8B97")).MarginBottom(1)
	s.WriteString(descStyle.Render(f.Command.Description) + "\n\n")

	for idx, field := range f.Fields {
		isActive := idx == f.ActiveIndex

		var fieldLabelStyle lipgloss.Style
		if isActive {
			fieldLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
		} else {
			fieldLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E3E3E6"))
		}

		switch field.Arg.Type {
		case InputTypeText:
			if isActive {
				s.WriteString(fieldLabelStyle.Render(field.Arg.Label))
				if field.Arg.Required {
					s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4A4A")).Render(" *"))
				}
				s.WriteString(lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("#8B8B97")).Render("  (Press TAB to go to next field)") + "\n")
				s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Render("▶ ") + field.TextInput.View() + "\n")
			} else {
				val := field.TextInput.Value()
				if val == "" {
					val = "(empty)"
				}
				s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#E3E3E6")).Render("  " + field.Arg.Label + ": ") + lipgloss.NewStyle().Foreground(lipgloss.Color("#8B8B97")).Render(val) + "\n")
			}

		case InputTypeTextArea:
			if isActive {
				s.WriteString(fieldLabelStyle.Render(field.Arg.Label))
				if field.Arg.Required {
					s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4A4A")).Render(" *"))
				}
				s.WriteString(lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("#8B8B97")).Render("  (Press TAB to go to next field)") + "\n")
				s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Render("▶ ") + "\n" + field.TextArea.View() + "\n")
			} else {
				val := field.TextArea.Value()
				if val == "" {
					val = "(empty)"
				} else {
					val = strings.ReplaceAll(val, "\n", " ")
					if len(val) > 40 {
						val = val[:37] + "..."
					}
				}
				s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#E3E3E6")).Render("  " + field.Arg.Label + ": ") + lipgloss.NewStyle().Foreground(lipgloss.Color("#8B8B97")).Render(val) + "\n")
			}

		case InputTypeSelect:
			if isActive {
				s.WriteString(fieldLabelStyle.Render(field.Arg.Label))
				if field.Arg.Required {
					s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4A4A")).Render(" *"))
				}
				s.WriteString(lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("#8B8B97")).Render("  (Press UP/DOWN arrows to choose, ENTER/TAB to go to next field)") + "\n")
				if field.Arg.Description != "" {
					s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#8B8B97")).Render("  " + field.Arg.Description) + "\n")
				}
				if len(field.MultiChoices) == 0 {
					s.WriteString(lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("#FF4A4A")).Render("  No options available.") + "\n")
				} else {
					maxVisible := 5
					start := 0
					end := len(field.MultiChoices)
					if end > maxVisible {
						start = field.MultiHover - maxVisible/2
						if start < 0 {
							start = 0
						}
						end = start + maxVisible
						if end > len(field.MultiChoices) {
							end = len(field.MultiChoices)
							start = end - maxVisible
						}
					}

					if start > 0 {
						s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#8B8B97")).Render("    ▲ ...") + "\n")
					}
					for cIdx := start; cIdx < end; cIdx++ {
						choice := field.MultiChoices[cIdx]
						isHovered := cIdx == field.MultiHover
						isSelected := cIdx == field.SelectIndex

						var line string
						if isSelected {
							line = fmt.Sprintf("● %s", choice)
						} else {
							line = fmt.Sprintf("○ %s", choice)
						}

						if isHovered {
							s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Bold(true).Render("  ▶ "+line) + "\n")
						} else {
							s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#E3E3E6")).Render("    "+line) + "\n")
						}
					}
					if end < len(field.MultiChoices) {
						s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#8B8B97")).Render("    ▼ ...") + "\n")
					}
				}
			} else {
				selectedVal := "None"
				if field.SelectIndex >= 0 && field.SelectIndex < len(field.MultiChoices) {
					selectedVal = field.MultiChoices[field.SelectIndex]
				}
				s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#E3E3E6")).Render("  " + field.Arg.Label + ": ") + lipgloss.NewStyle().Foreground(lipgloss.Color("#8B8B97")).Render(selectedVal) + "\n")
			}

		case InputTypeMultiSelect:
			if isActive {
				s.WriteString(fieldLabelStyle.Render(field.Arg.Label))
				if field.Arg.Required {
					s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4A4A")).Render(" *"))
				}
				s.WriteString(lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("#8B8B97")).Render("  (Press SPACE/ENTER to select/toggle, TAB to go to next field)") + "\n")
				if field.Arg.Description != "" {
					s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#8B8B97")).Render("  " + field.Arg.Description) + "\n")
				}
				if len(field.MultiChoices) == 0 {
					s.WriteString(lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("#FF4A4A")).Render("  No options available.") + "\n")
				} else {
					maxVisible := 5
					start := 0
					end := len(field.MultiChoices)
					if end > maxVisible {
						start = field.MultiHover - maxVisible/2
						if start < 0 {
							start = 0
						}
						end = start + maxVisible
						if end > len(field.MultiChoices) {
							end = len(field.MultiChoices)
							start = end - maxVisible
						}
					}

					if start > 0 {
						s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#8B8B97")).Render("    ▲ ...") + "\n")
					}
					for cIdx := start; cIdx < end; cIdx++ {
						choice := field.MultiChoices[cIdx]
						isHovered := cIdx == field.MultiHover
						isChecked := field.MultiChecked[cIdx]

						var box string
						if isChecked {
							box = "[✓]"
						} else {
							box = "[ ]"
						}

						line := fmt.Sprintf("%s %s", box, choice)
						if isHovered {
							s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Bold(true).Render("  ▶ "+line) + "\n")
						} else {
							s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#E3E3E6")).Render("    "+line) + "\n")
						}
					}
					if end < len(field.MultiChoices) {
						s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#8B8B97")).Render("    ▼ ...") + "\n")
					}
				}
			} else {
				var checked []string
				for idx, ch := range field.MultiChoices {
					if field.MultiChecked[idx] {
						checked = append(checked, ch)
					}
				}
				valStr := "None"
				if len(checked) > 0 {
					valStr = strings.Join(checked, ", ")
				}
				s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#E3E3E6")).Render("  " + field.Arg.Label + ": ") + lipgloss.NewStyle().Foreground(lipgloss.Color("#8B8B97")).Render(valStr) + "\n")
			}

		case InputTypeBoolean:
			var valStr string
			if field.BoolValue {
				valStr = "YES"
			} else {
				valStr = "NO"
			}

			if isActive {
				s.WriteString(fieldLabelStyle.Render(field.Arg.Label))
				if field.Arg.Required {
					s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4A4A")).Render(" *"))
				}
				s.WriteString(lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("#8B8B97")).Render("  (Press SPACE to toggle, ENTER/TAB to go to next field)") + "\n")
				s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Bold(true).Render("  ▶ [ "+valStr+" ]") + "\n")
			} else {
				s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#E3E3E6")).Render("  " + field.Arg.Label + ": ") + lipgloss.NewStyle().Foreground(lipgloss.Color("#8B8B97")).Render(valStr) + "\n")
			}
		}

		s.WriteString("\n")
	}

	// Submit Button
	buttonStyle := lipgloss.NewStyle().
		Padding(0, 3).
		Border(lipgloss.NormalBorder()).
		MarginTop(1)

	if f.SubmitActive {
		buttonStyle = buttonStyle.
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7D56F4")).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Bold(true)
		s.WriteString(buttonStyle.Render("EXECUTE COMMAND [ENTER]") + "\n")
	} else {
		buttonStyle = buttonStyle.
			Foreground(lipgloss.Color("#8B8B97")).
			BorderForeground(lipgloss.Color("#3B3B42"))
		s.WriteString(buttonStyle.Render("EXECUTE COMMAND") + "\n")
	}

	return s.String()
}
