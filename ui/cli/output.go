package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

// printBody writes an API response. With --json (or rawText for exports) the
// bytes are emitted verbatim so callers get the untouched payload. Otherwise a
// trivial key/value view is printed; values are never transformed.
func printBody(w io.Writer, raw []byte, jsonOut, rawText bool) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil
	}

	if jsonOut || rawText {
		if _, err := w.Write(trimmed); err != nil {
			return err
		}
		_, err := fmt.Fprintln(w)
		return err
	}

	var value any
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	if err := dec.Decode(&value); err != nil {
		// Non-JSON payloads (e.g. markdown exports) are printed as-is.
		if _, werr := w.Write(raw); werr != nil {
			return werr
		}
		if !bytes.HasSuffix(raw, []byte("\n")) {
			_, err := fmt.Fprintln(w)
			return err
		}
		return nil
	}

	printValue(w, value, 0)
	return nil
}

func printValue(w io.Writer, value any, depth int) {
	switch v := value.(type) {
	case map[string]any:
		if len(v) == 0 {
			fmt.Fprintln(w, "{}")
			return
		}
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(w, "%s: %s\n", key, compactJSON(v[key]))
		}
	case []any:
		if len(v) == 0 {
			fmt.Fprintln(w, "(empty)")
			return
		}
		for i, item := range v {
			if _, ok := item.(map[string]any); ok {
				if i > 0 {
					fmt.Fprintln(w)
				}
				printValue(w, item, depth+1)
				continue
			}
			fmt.Fprintln(w, compactJSON(item))
		}
	default:
		fmt.Fprintln(w, compactJSON(v))
	}
}

func compactJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(data)
}
