package config

import "path/filepath"

const (
	// Global layout under NEOCLAW_HOME.
	ConfigFilePath = "config.toml"
	DataDirPath    = "data"
	PolicyDirPath  = "policy"
	LogsDirPath    = "logs"
	PIDFilePath    = "claw.pid"

	// Agent directory layout under NEOCLAW_HOME/data/agents/{agent}/.
	AgentsDirPath      = "agents"
	WorkspaceDirPath   = "workspace"
	MemoryDirPath      = "memory"
	DailyDirPath       = "daily"
	SessionsDirPath    = "sessions"
	CLISessionsDirPath = "cli"
	DefaultSessionPath = "default.jsonl"
	JobsFilePath       = "jobs.json"
	SoulFilePath       = "SOUL.md"
	UserFilePath       = "USER.md"
	MemoryFilePath     = "memory.tsv"

	AllowedDomainsFileName  = "allowed_domains.json"
	AllowedCommandsFileName = "allowed_commands.json"
	AllowedUsersFileName    = "allowed_users.json"
	CostsFileName           = "costs.tsv"
)

func homeConfigPath(home string) string {
	return filepath.Join(home, ConfigFilePath)
}

func defaultHomePath(home string) string {
	return filepath.Join(home, ".neoclaw")
}

func homeDataPath(home string) string {
	return filepath.Join(home, DataDirPath)
}

func (c *Config) ConfigPath() string {
	return homeConfigPath(c.HomeDir)
}

func (c *Config) DataDir() string {
	return homeDataPath(c.HomeDir)
}

func (c *Config) PolicyDir() string {
	return filepath.Join(c.DataDir(), PolicyDirPath)
}

func (c *Config) LogsDir() string {
	return filepath.Join(c.DataDir(), LogsDirPath)
}

func (c *Config) AllowedDomainsPath() string {
	return filepath.Join(c.PolicyDir(), AllowedDomainsFileName)
}

func (c *Config) AllowedCommandsPath() string {
	return filepath.Join(c.PolicyDir(), AllowedCommandsFileName)
}

func (c *Config) AllowedUsersPath() string {
	return filepath.Join(c.PolicyDir(), AllowedUsersFileName)
}

func (c *Config) CostsPath() string {
	return filepath.Join(c.LogsDir(), CostsFileName)
}

func (c *Config) PIDPath() string {
	return filepath.Join(c.DataDir(), PIDFilePath)
}

func (c *Config) AgentDir() string {
	return filepath.Join(c.DataDir(), AgentsDirPath, c.Agent)
}

func (c *Config) AgentsDir() string {
	return filepath.Join(c.DataDir(), AgentsDirPath)
}

func (c *Config) WorkspaceDir() string {
	return filepath.Join(c.AgentDir(), WorkspaceDirPath)
}

func (c *Config) MemoryDir() string {
	return filepath.Join(c.AgentDir(), MemoryDirPath)
}

func (c *Config) DailyLogsDir() string {
	return filepath.Join(c.MemoryDir(), DailyDirPath)
}

func (c *Config) SessionsDir() string {
	return filepath.Join(c.AgentDir(), SessionsDirPath)
}

func (c *Config) CLISessionDir() string {
	return filepath.Join(c.SessionsDir(), CLISessionsDirPath)
}

func (c *Config) CLIContextPath() string {
	return filepath.Join(c.CLISessionDir(), DefaultSessionPath)
}

func (c *Config) TelegramContextPath() string {
	return filepath.Join(c.SessionsDir(), "telegram", DefaultSessionPath)
}

func (c *Config) JobsPath() string {
	return filepath.Join(c.AgentDir(), JobsFilePath)
}

func (c *Config) SoulPath() string {
	return filepath.Join(c.AgentDir(), SoulFilePath)
}

func (c *Config) UserPath() string {
	return filepath.Join(c.AgentDir(), UserFilePath)
}

func (c *Config) MemoryPath() string {
	return filepath.Join(c.MemoryDir(), MemoryFilePath)
}
