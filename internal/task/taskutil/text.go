package taskutil

import (
	"bufio"
	"strings"
)

const maxScanTokenSize = 1024 * 1024

func HasExactLine(output, line string) (bool, error) {
	found := false
	if err := ScanLines(output, func(text string) {
		if text == line {
			found = true
		}
	}); err != nil {
		return false, err
	}
	return found, nil
}

func LineSet(output string) (map[string]struct{}, error) {
	set := make(map[string]struct{})
	if err := ScanLines(output, func(text string) {
		set[text] = struct{}{}
	}); err != nil {
		return nil, err
	}
	return set, nil
}

// ParseKeyValueSettings reads "key value" lines, ignoring blank lines and # comments.
func ParseKeyValueSettings(output string) (map[string]string, error) {
	settings := make(map[string]string)
	if err := ScanLines(output, func(line string) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			return
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			return
		}
		settings[strings.ToLower(fields[0])] = strings.ToLower(fields[1])
	}); err != nil {
		return nil, err
	}
	return settings, nil
}

func ScanLines(output string, fn func(string)) error {
	scanner := bufio.NewScanner(strings.NewReader(output))
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxScanTokenSize)
	for scanner.Scan() {
		fn(scanner.Text())
	}
	return scanner.Err()
}
