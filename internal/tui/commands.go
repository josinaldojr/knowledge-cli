package tui

type InputType string

const (
	InputTypeText        InputType = "text"
	InputTypeTextArea    InputType = "textarea"
	InputTypeSelect      InputType = "select"
	InputTypeMultiSelect InputType = "multiselect"
	InputTypeBoolean     InputType = "boolean"
)

type ArgDefinition struct {
	Key          string     // command arg/flag, e.g. "--session" or "query"
	IsFlag       bool       // if true, it's prefixed, e.g. --session <value>. if false, it's positional.
	Label        string     // label for display
	Description  string     // hint/help text
	Type         InputType  // UI control type
	Required     bool       // is it required?
	Choices      []string   // hardcoded choices if Type is select
	ChoicesSrc   string     // dynamic source, e.g., "apps", "sessions"
	DefaultValue string     // default text or boolean
}

type Command struct {
	Name          string          // CLI name of command, e.g. "session start"
	DisplayName   string          // Display name, e.g. "Start Session"
	Description   string          // Short help text
	Group         string          // Category group (e.g., "Session", "Vault", "Workspace")
	Args          []ArgDefinition // Arguments and flags
	IsLongRunning bool            // if true, command runs as a background process/daemon
}

func GetCommands() []Command {
	return []Command{
		// --- WORKSPACE & APPS ---
		{
			Name:        "init",
			DisplayName: "Initialize AI Harness",
			Description: "Initialize standard workspace and templates (creates .kv/config.yaml)",
			Group:       "Workspace & Apps",
			Args: []ArgDefinition{
				{
					Key:         "--vault",
					IsFlag:      true,
					Label:       "Vault Path",
					Description: "Path to default Knowledge Vault (optional)",
					Type:        InputTypeText,
					Required:    false,
				},
			},
		},
		{
			Name:        "workspace init",
			DisplayName: "Initialize Workspace",
			Description: "Initialize workspace configuration (creates kv-workspace.yaml)",
			Group:       "Workspace & Apps",
			Args: []ArgDefinition{
				{
					Key:         "--vault",
					IsFlag:      true,
					Label:       "Vault Path",
					Description: "Path to default Knowledge Vault (optional)",
					Type:        InputTypeText,
					Required:    false,
				},
			},
		},
		{
			Name:        "workspace show",
			DisplayName: "Show Workspace Status",
			Description: "Display current workspace registered applications and details",
			Group:       "Workspace & Apps",
		},
		{
			Name:        "workspace scan",
			DisplayName: "Scan Apps",
			Description: "Automatically scan directories for apps and save to workspace config",
			Group:       "Workspace & Apps",
		},
		{
			Name:        "app list",
			DisplayName: "List Apps",
			Description: "List all registered applications in this workspace",
			Group:       "Workspace & Apps",
		},
		{
			Name:        "app add",
			DisplayName: "Add Application",
			Description: "Register a new application in the workspace configuration",
			Group:       "Workspace & Apps",
			Args: []ArgDefinition{
				{
					Key:         "--id",
					IsFlag:      true,
					Label:       "App ID",
					Description: "Unique application identifier (e.g. backend-api)",
					Type:        InputTypeText,
					Required:    true,
				},
				{
					Key:         "--name",
					IsFlag:      true,
					Label:       "App Name",
					Description: "Friendly display name",
					Type:        InputTypeText,
					Required:    true,
				},
				{
					Key:         "--path",
					IsFlag:      true,
					Label:       "App Directory Path",
					Description: "Relative or absolute path to the app directory",
					Type:        InputTypeText,
					Required:    true,
				},
				{
					Key:         "--type",
					IsFlag:      true,
					Label:       "App Type",
					Description: "Type of app, e.g. service, library, webapp",
					Type:        InputTypeText,
					Required:    true,
				},
				{
					Key:         "--stack",
					IsFlag:      true,
					Label:       "Tech Stack",
					Description: "Underlying stack, e.g. go, typescript, python",
					Type:        InputTypeText,
					Required:    true,
				},
			},
		},

		// --- VAULT OPERATIONS ---
		{
			Name:        "vault attach",
			DisplayName: "Attach Knowledge Vault",
			Description: "Attach an external Knowledge Vault to this workspace",
			Group:       "Knowledge Vault",
			Args: []ArgDefinition{
				{
					Key:         "path",
					IsFlag:      false,
					Label:       "Vault Directory Path",
					Description: "Path to the Knowledge Vault",
					Type:        InputTypeText,
					Required:    true,
				},
			},
		},
		{
			Name:        "vault init",
			DisplayName: "Create New Vault",
			Description: "Initialize a new empty Knowledge Vault structure",
			Group:       "Knowledge Vault",
			Args: []ArgDefinition{
				{
					Key:         "path",
					IsFlag:      false,
					Label:       "New Vault Path",
					Description: "Target directory where vault will be created",
					Type:        InputTypeText,
					Required:    true,
				},
			},
		},
		{
			Name:        "vault path",
			DisplayName: "Show Vault Path",
			Description: "Discover and display the active attached Knowledge Vault path",
			Group:       "Knowledge Vault",
		},
		{
			Name:        "vault doctor",
			DisplayName: "Validate Vault Health",
			Description: "Diagnose and validate the active vault's health and format",
			Group:       "Knowledge Vault",
		},
		{
			Name:        "find",
			DisplayName: "Search Vault Files",
			Description: "Query/search documents inside the active Knowledge Vault",
			Group:       "Knowledge Vault",
			Args: []ArgDefinition{
				{
					Key:         "query",
					IsFlag:      false,
					Label:       "Search Query",
					Description: "Keywords to search (e.g. database setup)",
					Type:        InputTypeText,
					Required:    true,
				},
			},
		},

		// --- SESSIONS ---
		{
			Name:        "session init",
			DisplayName: "Init Session Manually",
			Description: "Initialize a development session with explicit parameters",
			Group:       "Session Management",
			Args: []ArgDefinition{
				{
					Key:         "--id",
					IsFlag:      true,
					Label:       "Session ID",
					Description: "Custom session ID (optional)",
					Type:        InputTypeText,
					Required:    false,
				},
				{
					Key:         "--goal",
					IsFlag:      true,
					Label:       "Session Goal",
					Description: "Objective description of what needs to be built",
					Type:        InputTypeTextArea,
					Required:    true,
				},
				{
					Key:         "--apps",
					IsFlag:      true,
					Label:       "Select Apps",
					Description: "Select applications to register in the session",
					Type:        InputTypeMultiSelect,
					ChoicesSrc:  "apps",
					Required:    false,
				},
				{
					Key:         "--vault",
					IsFlag:      true,
					Label:       "Vault Sources",
					Description: "Comma-separated paths to vault source files",
					Type:        InputTypeText,
					Required:    false,
				},
				{
					Key:         "--writable",
					IsFlag:      true,
					Label:       "Writable Paths",
					Description: "Comma-separated paths the agent can modify",
					Type:        InputTypeText,
					Required:    false,
				},
			},
		},
		{
			Name:        "session start",
			DisplayName: "Start Session",
			Description: "Initialize and start a multi-app session interactively",
			Group:       "Session Management",
			Args: []ArgDefinition{
				{
					Key:         "--goal",
					IsFlag:      true,
					Label:       "Objective / Goal",
					Description: "Instruction / Objective of the session",
					Type:        InputTypeTextArea,
					Required:    true,
				},
				{
					Key:         "--apps",
					IsFlag:      true,
					Label:       "Select Workspace Apps",
					Description: "Select one or more workspace apps to include",
					Type:        InputTypeMultiSelect,
					ChoicesSrc:  "apps",
					Required:    true,
				},
			},
		},
		{
			Name:        "run",
			DisplayName: "Run Session Agent",
			Description: "Execute agent instructions defined inside a session",
			Group:       "Session Management",
			Args: []ArgDefinition{
				{
					Key:         "--session",
					IsFlag:      true,
					Label:       "Session ID",
					Description: "Select session to run",
					Type:        InputTypeSelect,
					ChoicesSrc:  "sessions",
					Required:    true,
				},
				{
					Key:         "--dry-run",
					IsFlag:      true,
					Label:       "Dry Run",
					Description: "Simulate run without launching agent execution",
					Type:        InputTypeBoolean,
					Required:    false,
					DefaultValue: "false",
				},
			},
		},
		{
			Name:        "session validate",
			DisplayName: "Validate Session Contract",
			Description: "Validate structured boundaries and policies of a session contract",
			Group:       "Session Management",
			Args: []ArgDefinition{
				{
					Key:         "--session",
					IsFlag:      true,
					Label:       "Session ID",
					Description: "Select session to validate",
					Type:        InputTypeSelect,
					ChoicesSrc:  "sessions",
					Required:    true,
				},
			},
		},
		{
			Name:        "session diff",
			DisplayName: "Show Session Diff",
			Description: "Compare files changed inside the boundaries of a session",
			Group:       "Session Management",
			Args: []ArgDefinition{
				{
					Key:         "--session",
					IsFlag:      true,
					Label:       "Session ID",
					Description: "Select session to diff",
					Type:        InputTypeSelect,
					ChoicesSrc:  "sessions",
					Required:    true,
				},
				{
					Key:         "--include-untracked",
					IsFlag:      true,
					Label:       "Include Untracked",
					Description: "Include untracked files in the Git diff",
					Type:        InputTypeBoolean,
					Required:    false,
					DefaultValue: "false",
				},
			},
		},
		{
			Name:        "session report",
			DisplayName: "Generate Session Report",
			Description: "Generate a markdown summary and audit log report for the session",
			Group:       "Session Management",
			Args: []ArgDefinition{
				{
					Key:         "--session",
					IsFlag:      true,
					Label:       "Session ID",
					Description: "Select session to generate report for",
					Type:        InputTypeSelect,
					ChoicesSrc:  "sessions",
					Required:    true,
				},
			},
		},

		// --- QUALITY & BOUNDARIES ---
		{
			Name:        "boundary validate",
			DisplayName: "Validate Boundary Violations",
			Description: "Check file changes against session boundary configurations",
			Group:       "Quality & Boundaries",
			Args: []ArgDefinition{
				{
					Key:         "--session",
					IsFlag:      true,
					Label:       "Session ID",
					Description: "Select session to check",
					Type:        InputTypeSelect,
					ChoicesSrc:  "sessions",
					Required:    true,
				},
				{
					Key:         "--include-untracked",
					IsFlag:      true,
					Label:       "Include Untracked",
					Description: "Include untracked files in checks",
					Type:        InputTypeBoolean,
					Required:    false,
					DefaultValue: "false",
				},
			},
		},
		{
			Name:        "quality run",
			DisplayName: "Run Quality Gates",
			Description: "Execute session test commands and compile verification results",
			Group:       "Quality & Boundaries",
			Args: []ArgDefinition{
				{
					Key:         "--session",
					IsFlag:      true,
					Label:       "Session ID",
					Description: "Select session to run gates for",
					Type:        InputTypeSelect,
					ChoicesSrc:  "sessions",
					Required:    true,
				},
			},
		},
		{
			Name:        "diff summarize",
			DisplayName: "Summarize Diff",
			Description: "Summarize session code changes using LLM tools",
			Group:       "Quality & Boundaries",
			Args: []ArgDefinition{
				{
					Key:         "--session",
					IsFlag:      true,
					Label:       "Session ID",
					Description: "Select session to summarize",
					Type:        InputTypeSelect,
					ChoicesSrc:  "sessions",
					Required:    true,
				},
			},
		},

		// --- TASKS & WORKFLOWS ---
		{
			Name:        "workflow new",
			DisplayName: "Create Workflow",
			Description: "Create a new versionable workflow directory structure",
			Group:       "Tasks & Workflows",
			Args: []ArgDefinition{
				{
					Key:         "slug",
					IsFlag:      false,
					Label:       "Workflow Slug",
					Description: "Identifier for the workflow directory (e.g. database-migration)",
					Type:        InputTypeText,
					Required:    true,
				},
			},
		},
		{
			Name:        "task enrich",
			DisplayName: "Enrich Task",
			Description: "Gather workspace information, rules, and pack task context",
			Group:       "Tasks & Workflows",
			Args: []ArgDefinition{
				{
					Key:         "workflow-slug",
					IsFlag:      false,
					Label:       "Workflow Slug",
					Description: "Target workflow folder containing task definitions",
					Type:        InputTypeText,
					Required:    true,
				},
				{
					Key:         "task-id",
					IsFlag:      false,
					Label:       "Task ID",
					Description: "ID of task to enrich",
					Type:        InputTypeText,
					Required:    true,
				},
			},
		},
		{
			Name:        "task run",
			DisplayName: "Run Task",
			Description: "Execute a defined task utilizing the OpenCode runner",
			Group:       "Tasks & Workflows",
			Args: []ArgDefinition{
				{
					Key:         "workflow-slug",
					IsFlag:      false,
					Label:       "Workflow Slug",
					Description: "Workflow folder containing task definitions",
					Type:        InputTypeText,
					Required:    true,
				},
				{
					Key:         "task-id",
					IsFlag:      false,
					Label:       "Task ID",
					Description: "ID of task to execute",
					Type:        InputTypeText,
					Required:    true,
				},
				{
					Key:         "--runner",
					IsFlag:      true,
					Label:       "Runner Type",
					Description: "Runner adapter to use",
					Type:        InputTypeSelect,
					Choices:     []string{"opencode"},
					Required:    false,
					DefaultValue: "opencode",
				},
			},
		},
		{
			Name:        "context build",
			DisplayName: "Build Context",
			Description: "Compile session context or task context pack",
			Group:       "Tasks & Workflows",
			Args: []ArgDefinition{
				{
					Key:         "--session",
					IsFlag:      true,
					Label:       "Session ID",
					Description: "Session ID (leave blank if building task context instead)",
					Type:        InputTypeText, // Can be session select optionally
					Required:    false,
				},
				{
					Key:         "workflow-slug",
					IsFlag:      false,
					Label:       "Workflow Slug (Task Only)",
					Description: "Required only for task context building",
					Type:        InputTypeText,
					Required:    false,
				},
				{
					Key:         "task-id",
					IsFlag:      false,
					Label:       "Task ID (Task Only)",
					Description: "Required only for task context building",
					Type:        InputTypeText,
					Required:    false,
				},
			},
		},

		// --- LLM WIKI ---
		{
			Name:        "wiki compile",
			DisplayName: "Compile Wiki Notes",
			Description: "Scan inbox and structure notes using LLM into canonical topics",
			Group:       "LLM Wiki",
		},
		{
			Name:        "wiki link",
			DisplayName: "Auto Link Wiki Pages",
			Description: "Scan and build relative cross-links across canonical wiki files",
			Group:       "LLM Wiki",
		},
		{
			Name:        "wiki ask",
			DisplayName: "Ask LLM Wiki",
			Description: "Ask a natural language query against the local knowledge base",
			Group:       "LLM Wiki",
			Args: []ArgDefinition{
				{
					Key:         "query",
					IsFlag:      false,
					Label:       "Question",
					Description: "Your question for the wiki (e.g. How does auth work?)",
					Type:        InputTypeText,
					Required:    true,
				},
			},
		},
		{
			Name:          "wiki serve",
			DisplayName:   "Serve Web Wiki",
			Description:   "Launch the premium web wiki client server locally",
			Group:         "LLM Wiki",
			IsLongRunning: true,
			Args: []ArgDefinition{
				{
					Key:         "--port",
					IsFlag:      true,
					Label:       "Server Port",
					Description: "Local web interface port",
					Type:        InputTypeText,
					Required:    false,
					DefaultValue: "8080",
				},
			},
		},

		// --- OPENCODE ---
		{
			Name:        "opencode install",
			DisplayName: "Install OpenCode Integration",
			Description: "Install templates and script helpers for OpenCode integration",
			Group:       "OpenCode Runner",
		},
		{
			Name:        "opencode doctor",
			DisplayName: "Verify Workspace Integration",
			Description: "Diagnose workspace directories and OpenCode environment settings",
			Group:       "OpenCode Runner",
		},
	}
}
