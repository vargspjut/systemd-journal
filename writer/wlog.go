// +build linux

package writer

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	journal "github.com/vargspjut/systemd-journal"
)

// WlogWriter allows the wlog logging package to act as a
// front-end for the journal.
// NOTE: The wlog instance writing to WlogWriter must
// be configured to use structured JSON logging. In practice
// the wlog.JSONFormatter must be used as formatter.
type WlogWriter struct {
	io.Writer
}

func (ww WlogWriter) Write(b []byte) (int, error) {

	m := make(map[string]interface{})
	if err := json.Unmarshal(b, &m); err != nil {
		return 0, err
	}

	if len(m) == 0 {
		return 0, errors.New("invalid structured log entry from wlog")
	}

	var (
		fields = journal.Fields{}
		msg    string
		prio   journal.Priority
	)

	for k, v := range m {
		if k == "" {
			continue
		}

		// Deduce type from interface{}
		val := ww.valueAsString(v)

		switch k {
		case "message", "@m":
			msg = val
		case "level", "@l":
			prio = ww.journalPriority(val)
		case "timestamp", "@t":
			// Journal writes timestamp implicitly
			continue
		default:
			if k[0] == '@' {
				continue
			}
			// Add custom field. The journal requires field
			// names to be all uppercase
			fields[strings.ToUpper(k)] = val
		}
	}

	if err := journal.SubmitWithFields(prio, msg, fields); err != nil {
		return 0, err
	}

	return len(b), nil
}

func (ww WlogWriter) valueAsString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case time.Time:
		return t.Format(time.RFC3339)
	case []byte:
		return base64.StdEncoding.EncodeToString(t)
	default:
		return ""
	}
}

func (ww WlogWriter) journalPriority(level string) journal.Priority {

	var p journal.Priority

	switch level {
	case "Debug":
		p = journal.PriorityDebug
	case "Warning":
		p = journal.PriorityWarning
	case "Error":
		p = journal.PriorityError
	case "Fatal":
		p = journal.PriorityCritical
	default:
		p = journal.PriorityInfo
	}

	return p
}
