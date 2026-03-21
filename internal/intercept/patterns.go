package intercept

import (
	"regexp"
	"strings"
)

// ansiEscape matches ANSI escape sequences (colors, cursor movement, etc.).
var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// promptPatterns are the regexes used to detect CLI confirmation prompts.
// Each pattern captures the Y/N indicator to determine options and default.
var promptPatterns = []*promptPattern{
	// (Y/n), (y/N), (y/n), (Y/N)
	{
		re:   regexp.MustCompile(`\(([YyNn])/([YyNn])\)\s*:?\s*$`),
		kind: "yes_no",
	},
	// [Y/n], [y/N], [y/n], [Y/N]
	{
		re:   regexp.MustCompile(`\[([YyNn])/([YyNn])\]\s*:?\s*$`),
		kind: "yes_no",
	},
	// (yes/no), [yes/no] — word variants
	{
		re:   regexp.MustCompile(`[\[\(](yes|no)/(yes|no)[\]\)]\s*:?\s*$`),
		kind: "yes_no_word",
	},
}

type promptPattern struct {
	re   *regexp.Regexp
	kind string // "yes_no" or "yes_no_word"
}

// StripANSI removes ANSI escape sequences from a byte slice.
func StripANSI(b []byte) []byte {
	return ansiEscape.ReplaceAll(b, nil)
}

// matchLine checks a single line (ANSI-stripped) against all patterns.
// Returns a Prompt if a match is found, nil otherwise.
func matchLine(line string) *Prompt {
	for _, p := range promptPatterns {
		loc := p.re.FindStringSubmatchIndex(line)
		if loc == nil {
			continue
		}

		// Extract the prompt text (everything before the match).
		text := line[:loc[0]]
		// Trim leading "? " from inquirer-style prompts.
		if len(text) > 2 && text[0] == '?' && text[1] == ' ' {
			text = text[2:]
		}
		text = strings.TrimSpace(text)

		switch p.kind {
		case "yes_no":
			first := line[loc[2]:loc[3]]  // first letter
			second := line[loc[4]:loc[5]] // second letter
			return yesNoPrompt(text, first, second)
		case "yes_no_word":
			first := line[loc[2]:loc[3]]  // "yes" or "no"
			second := line[loc[4]:loc[5]] // "yes" or "no"
			return yesNoWordPrompt(text, first, second)
		}
	}
	return nil
}

func yesNoPrompt(text, first, second string) *Prompt {
	options := []string{"Yes", "No"}
	defaultOpt := ""

	// Uppercase letter indicates default.
	if first == "Y" {
		defaultOpt = "Yes"
	} else if second == "N" {
		// Only set default No if first wasn't uppercase Y.
		defaultOpt = "No"
	} else if first == "N" {
		defaultOpt = "No"
	} else if second == "Y" {
		defaultOpt = "Yes"
	}

	return &Prompt{Text: text, Options: options, Default: defaultOpt}
}

func yesNoWordPrompt(text, first, second string) *Prompt {
	options := []string{"Yes", "No"}
	defaultOpt := ""
	// Word variant: "yes"/"no" — no default indication from case.
	_ = first
	_ = second
	return &Prompt{Text: text, Options: options, Default: defaultOpt}
}
