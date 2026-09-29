package permission

import "strings"

type Mode string

const (
	ModePlan    Mode = "plan"
	ModeDefault Mode = "default"
	ModeAuto    Mode = "auto"
)

var readOnlyTools = map[string]bool{
	"read": true,
	"grep": true,
	"glob": true,
}

var deniedBashSubstrings = []string{
	"rm -rf /",
	"rm -rf ~",
	"rm -rf *",
	"mkfs",
	"dd if=",
	":(){ :|:& };:",
	"git push --force",
	"git push -f",
	"> /dev/sda",
}

// Decision describes what should happen before a tool actually runs.
type Decision struct {
	Denied       bool
	DenyReason   string
	NeedsConfirm bool
}

func Check(mode Mode, toolName string, rawArgs string) Decision {
	if toolName == "bash" {
		lower := strings.ToLower(rawArgs)
		for _, bad := range deniedBashSubstrings {
			if strings.Contains(lower, strings.ToLower(bad)) {
				return Decision{Denied: true, DenyReason: "command matches deny-list pattern: " + bad}
			}
		}
	}

	if mode == ModePlan && !readOnlyTools[toolName] {
		return Decision{Denied: true, DenyReason: "plan mode only allows read-only tools (read, grep, glob)"}
	}

	if mode == ModeAuto || readOnlyTools[toolName] {
		return Decision{}
	}

	return Decision{NeedsConfirm: true}
}
