package manifest

import (
	"fmt"
	"strings"
)

var shellMetacharacters = []string{
	"|", ";", "&&", "||", ">", "<", "$(", "`", "&",
}

var networkCommands = []string{
	"curl ", "wget ", "git clone ", "npm install ", "pip install ", "cargo install ",
}

func ValidateArgs(args string) error {
	if args == "" {
		return nil
	}

	argsLower := strings.ToLower(args)

	for _, mc := range shellMetacharacters {
		if strings.Contains(argsLower, mc) {
			return fmt.Errorf("args contain shell metacharacter %q", mc)
		}
	}

	for _, nc := range networkCommands {
		if strings.Contains(argsLower, nc) {
			return fmt.Errorf("args contain network command %q", nc)
		}
	}

	return nil
}
