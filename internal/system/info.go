package system

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// SystemInfo holds information about the current system environment
type SystemInfo struct {
	OS           string
	Architecture string
	Shell        string
	HasGNU       bool
	HasBSD       bool
	Capabilities map[string]string
}

// DetectSystemInfo gathers information about the current system
func DetectSystemInfo() *SystemInfo {
	info := &SystemInfo{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		Capabilities: make(map[string]string),
	}

	// Detect shell
	if shell := os.Getenv("SHELL"); shell != "" {
		info.Shell = shell
	}

	// Detect command variants
	info.detectCommandVariants()

	return info
}

// detectCommandVariants checks which command variants are available
func (si *SystemInfo) detectCommandVariants() {
	// Check if GNU coreutils are available (common on Linux)
	if output, err := exec.Command("ls", "--version").CombinedOutput(); err == nil {
		outputStr := strings.ToLower(string(output))
		if strings.Contains(outputStr, "gnu") {
			si.HasGNU = true
			si.Capabilities["ls"] = "GNU"
		}
	}

	// Check if BSD commands are available (macOS, FreeBSD)
	if output, err := exec.Command("stat", "-f", "%z", "/").CombinedOutput(); err == nil {
		if len(output) > 0 {
			si.HasBSD = true
			si.Capabilities["stat"] = "BSD"
		}
	}

	// Check stat command variant
	if si.Capabilities["stat"] == "" {
		if _, err := exec.Command("stat", "-c", "%s", "/").CombinedOutput(); err == nil {
			si.Capabilities["stat"] = "GNU"
		}
	}

	// Check du command variant
	if _, err := exec.Command("du", "-b", "/dev/null").CombinedOutput(); err == nil {
		si.Capabilities["du"] = "GNU"
	} else if _, err := exec.Command("du", "-h", "/dev/null").CombinedOutput(); err == nil {
		si.Capabilities["du"] = "BSD"
	}

	// Check find command variant
	if _, err := exec.Command("find", "/dev/null", "-printf", "%s").CombinedOutput(); err == nil {
		si.Capabilities["find"] = "GNU"
	} else {
		si.Capabilities["find"] = "BSD"
	}

	// Check if common tools are available
	commonTools := []string{"git", "curl", "wget", "docker", "kubectl", "npm", "python3", "go", "make"}
	for _, tool := range commonTools {
		if _, err := exec.LookPath(tool); err == nil {
			if version := getToolVersion(tool); version != "" {
				si.Capabilities[tool] = version
			} else {
				si.Capabilities[tool] = "available"
			}
		}
	}
}

// getToolVersion attempts to get the version of a tool
func getToolVersion(tool string) string {
	// Try common version flags
	versionFlags := []string{"--version", "-version", "-V", "version"}
	
	for _, flag := range versionFlags {
		if output, err := exec.Command(tool, flag).CombinedOutput(); err == nil {
			lines := strings.Split(string(output), "\n")
			if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
				// Return first line, truncated if too long
				version := strings.TrimSpace(lines[0])
				if len(version) > 50 {
					version = version[:50] + "..."
				}
				return version
			}
		}
	}
	
	return ""
}

// GetCommandSyntaxHints returns platform-specific command syntax hints
func (si *SystemInfo) GetCommandSyntaxHints() string {
	var hints strings.Builder
	
	hints.WriteString("SYSTEM ENVIRONMENT:\n")
	hints.WriteString(fmt.Sprintf("OS: %s, Architecture: %s\n", si.OS, si.Architecture))
	if si.Shell != "" {
		hints.WriteString(fmt.Sprintf("Shell: %s\n", si.Shell))
	}
	
	hints.WriteString("\nCOMMAND SYNTAX GUIDELINES:\n")
	
	// Stat command
	if si.Capabilities["stat"] == "BSD" {
		hints.WriteString("- Use 'stat -f %z file' for file size (BSD syntax)\n")
	} else if si.Capabilities["stat"] == "GNU" {
		hints.WriteString("- Use 'stat -c %s file' for file size (GNU syntax)\n")
	}
	
	// Du command
	if si.Capabilities["du"] == "BSD" {
		hints.WriteString("- Use 'du -h' for human-readable sizes (BSD syntax)\n")
	} else if si.Capabilities["du"] == "GNU" {
		hints.WriteString("- Use 'du -b' for bytes or 'du -h' for human-readable (GNU syntax)\n")
	}
	
	// Find command
	if si.Capabilities["find"] == "GNU" {
		hints.WriteString("- Use 'find ... -printf %s' for file sizes (GNU syntax)\n")
	} else {
		hints.WriteString("- Use 'find ... -exec stat ...' for file operations (BSD syntax)\n")
	}
	
	// List sorting
	if si.HasGNU {
		hints.WriteString("- Use 'ls --sort=size' or 'ls -S' for size sorting (GNU)\n")
	} else {
		hints.WriteString("- Use 'ls -lS' for size sorting (BSD)\n")
	}
	
	// Available tools
	if len(si.Capabilities) > 0 {
		hints.WriteString("\nAVAILABLE TOOLS:\n")
		for tool, version := range si.Capabilities {
			if tool != "stat" && tool != "du" && tool != "find" && tool != "ls" {
				if version == "available" {
					hints.WriteString(fmt.Sprintf("- %s: available\n", tool))
				} else {
					hints.WriteString(fmt.Sprintf("- %s: %s\n", tool, version))
				}
			}
		}
	}
	
	return hints.String()
}

// GetSystemDetectionCommands returns commands to detect system properties
func (si *SystemInfo) GetSystemDetectionCommands() []string {
	commands := []string{
		"uname -a",  // System information
	}
	
	// Add OS-specific detection commands
	switch si.OS {
	case "darwin":
		commands = append(commands, "sw_vers")  // macOS version
	case "linux":
		commands = append(commands, 
			"lsb_release -a 2>/dev/null || cat /etc/os-release | head -5",  // Linux distribution
		)
	case "windows":
		commands = append(commands, "ver")  // Windows version
	}
	
	return commands
}
