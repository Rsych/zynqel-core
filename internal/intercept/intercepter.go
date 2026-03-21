package intercept

// maxLineBuffer limits the accumulated line buffer to prevent unbounded growth.
const maxLineBuffer = 1024

// Prompt represents a detected CLI confirmation prompt.
type Prompt struct {
	Text    string   `json:"text"`              // Prompt text (e.g. "Allow Claude to edit file.go?")
	Options []string `json:"options"`           // Available options (e.g. ["Yes", "No"])
	Default string   `json:"default,omitempty"` // Default option (e.g. "Yes")
}

// Intercepter scans PTY output for CLI confirmation prompts.
// It accumulates partial lines across chunks to handle prompts
// that arrive in multiple reads.
type Intercepter struct {
	buf []byte
}

// New creates an Intercepter.
func New() *Intercepter {
	return &Intercepter{}
}

// Scan processes a chunk of PTY output and returns any detected prompts.
// Most calls return nil (no prompt detected). The intercepter buffers
// incomplete lines across calls to handle split chunks.
func (i *Intercepter) Scan(chunk []byte) []Prompt {
	var prompts []Prompt

	for _, b := range chunk {
		if b == '\n' || b == '\r' {
			// Complete line — scan it.
			if p := i.scanBuffer(); p != nil {
				prompts = append(prompts, *p)
			}
			i.buf = i.buf[:0]
		} else {
			i.buf = append(i.buf, b)
			// Prevent unbounded growth.
			if len(i.buf) > maxLineBuffer {
				i.buf = i.buf[len(i.buf)-maxLineBuffer:]
			}
		}
	}

	// Also scan the current incomplete line — prompts often don't
	// end with a newline (they wait for user input).
	if len(i.buf) > 0 {
		if p := i.scanBuffer(); p != nil {
			prompts = append(prompts, *p)
			// Clear buffer after detection to avoid re-detecting on next chunk.
			i.buf = i.buf[:0]
		}
	}

	return prompts
}

// scanBuffer strips ANSI codes and matches the current buffer against patterns.
func (i *Intercepter) scanBuffer() *Prompt {
	cleaned := StripANSI(i.buf)
	if len(cleaned) == 0 {
		return nil
	}
	return matchLine(string(cleaned))
}
