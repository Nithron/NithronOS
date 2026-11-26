package config

import "os"

// AgentSocket is nos-agent's unix socket. NOS_AGENT_SOCKET overrides it
// (development and tests run a stand-in agent elsewhere).
func (c Config) AgentSocket() string {
	if p := os.Getenv("NOS_AGENT_SOCKET"); p != "" {
		return p
	}
	return "/run/nos-agent.sock"
}
