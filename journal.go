package journal

//// #cgo pkg-config: --cflags --libs libsystemd
// #cgo LDFLAGS: -lsystemd
// #include <systemd/sd-journal.h>
// #include <stdlib.h>
// #include <syslog.h>
import (
	"C"
)
import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// Predefined field names
const (
	FieldMessage                 = "MESSAGE"
	FieldMessageID               = "MESSAGE_ID"
	FieldPriority                = "PRIORITY"
	FieldCodeFile                = "CODE_FILE"
	FieldCodeLine                = "CODE_LINE"
	FieldCodeFunc                = "CODE_FUNC"
	FieldErrNo                   = "ERRNO"
	FieldInvocationID            = "INVOCATION_ID"
	FieldUserInvocationID        = "USER_INVOCATION_ID"
	FieldSyslogFacility          = "SYSLOG_FACILITY"
	FieldSyslogIdentifier        = "SYSLOG_INDENTIFIER"
	FieldSyslogPID               = "SYSLOG_PID"
	FieldSyslogTimestamp         = "SYSLOG_TIMESTAMP"
	FieldSyslogRaw               = "SYSLOG_RAW"
	FieldDocumentation           = "DOCUMENTATION"
	FieldPID                     = "_PID"
	FieldUID                     = "_UID"
	FieldGID                     = "_GID"
	FieldComm                    = "_COMM"
	FieldExe                     = "_EXE"
	FieldCmdLine                 = "_CMDLINE"
	FieldCapEffective            = "_CAP_EFFECTIVE"
	FieldAuditSession            = "_AUDIT_SESSION"
	FieldAuditLoginUID           = "_AUDIT_LOGINUID"
	FieldCGroup                  = "_SYSTEMD_CGROUP"
	FieldSession                 = "_SYSTEMD_SESSION"
	FieldUnit                    = "_SYSTEMD_UNIT"
	FieldUserUnit                = "_SYSTEMD_USER_UNIT"
	FieldOwnerUID                = "_SYSTEMD_OWNER_UID"
	FieldSlice                   = "_SYSTEMD_SLICE"
	FieldSELinuxContext          = "_SELINUX_CONTEXT"
	FieldSourceRealtimeTimestamp = "_SOURCE_REALTIME_TIMESTAMP"
	FieldBootID                  = "_BOOT_ID"
	FieldMachineID               = "_MACHINE_ID"
	FieldHostname                = "_HOSTNAME"
	FieldTransport               = "_TRANSPORT"
	FieldCursor                  = "__CURSOR"
	FieldRealtimeTimestamp       = "__REALTIME_TIMESTAMP"
	FieldMonotonicTimestamp      = "__MONOTONIC_TIMESTAMP"
)

var (
	// ErrTailStopped is sent to handler if tail is externally stopped.
	ErrTailStopped = errors.New("journal: tail stopped")
)

// WakeupEvent represents the outcome of a wait operation
type WakeupEvent int

const (
	// NoOperation indicates no operation during a wakeup event
	NoOperation WakeupEvent = iota
	// Append indicates that new entries was appended to the journal
	Append
	// Invalidate indicates that entries was added, removed or changed
	Invalidate
)

// Journal implements read access to systemd journal
type Journal struct {
	sdJournal *C.struct_sd_journal
	matches   []*Match
	mutex     sync.Mutex
}

// Fields is a map containing fields of an entry
type Fields map[string]string

// Entry contains all fields and meta-data for journal entry
type Entry struct {
	Fields    `json:"fields"`
	Cursor    string        `json:"cursor"`
	Timestamp time.Time     `json:"timestamp"`
	Elapsed   time.Duration `json:"elapsed"`
}

func (e *Entry) String() string {
	//data, err := json.Marshal(e)
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return ""
	}

	return string(data)
}

// Open creates a new journal instance
func Open() (*Journal, error) {

	sdJournal := new(C.struct_sd_journal)
	ret := int(C.sd_journal_open(&sdJournal, C.SD_JOURNAL_LOCAL_ONLY))
	if ret != 0 {
		return nil, fmt.Errorf("failed to open journal: %w", syscall.Errno(-ret))
	}

	j := Journal{sdJournal: sdJournal}

	return &j, nil
}

// Close closes the journal
func (j *Journal) Close() {
	j.mutex.Lock()
	C.sd_journal_close(j.sdJournal)
	j.mutex.Unlock()
}

// Next moves cursor to the next entry
func (j *Journal) Next() (int, error) {

	j.mutex.Lock()
	ret := C.sd_journal_next(j.sdJournal)
	j.mutex.Unlock()

	if ret < 0 {
		return 0, fmt.Errorf("failed to move to next entry: %w", syscall.Errno(-ret))
	}

	return int(ret), nil
}

// Previous moves cursor to the previous entry
func (j *Journal) Previous() (int, error) {

	j.mutex.Lock()
	ret := C.sd_journal_previous(j.sdJournal)
	j.mutex.Unlock()

	if ret < 0 {
		return 0, fmt.Errorf("failed to move to prevoius entry: %w", syscall.Errno(-ret))
	}

	return int(ret), nil
}

// Skip moves cursor n positions in any direction.
// Provide a positive value to move forward and a
// negative value to move back. Skip returns the
// number of positions moved or 0 if EOF is reached
func (j *Journal) Skip(n int64) (int64, error) {
	if n == 0 {
		return 0, nil
	}

	var ret C.int

	j.mutex.Lock()
	if n > 0 {
		ret = C.sd_journal_next_skip(j.sdJournal, C.uint64_t(n))
	} else {
		ret = C.sd_journal_previous_skip(j.sdJournal, C.uint64_t(n*-1))
	}
	j.mutex.Unlock()

	if ret < 0 {
		return 0, fmt.Errorf("failed to skip entries: %w", syscall.Errno(-ret))
	}

	return int64(ret), nil
}

// SeekHead moves cursor to the first entry
// NOTE: This call must be followed by a call to Next (or a similar call)
// before any data can be read
func (j *Journal) SeekHead() error {
	j.mutex.Lock()
	defer j.mutex.Unlock()
	if ret := C.sd_journal_seek_head(j.sdJournal); ret < 0 {
		return fmt.Errorf("failed seek head: %w", syscall.Errno(-ret))
	}

	return nil
}

// SeekTail moves the cursor to the last entry.
// NOTE: This call must be followed by a call to Next (or a similar call)
// before any data can be read
func (j *Journal) SeekTail() error {
	j.mutex.Lock()
	defer j.mutex.Unlock()
	if ret := C.sd_journal_seek_tail(j.sdJournal); ret < 0 {
		return fmt.Errorf("failed seek tail: %w", syscall.Errno(-ret))
	}

	return nil
}

// SeekTimestamp moves the cursor to the entry with the specified timestamp
// NOTE: This call must be followed by a call to Next (or a similar call)
// before any data can be read
func (j *Journal) SeekTimestamp(timestamp time.Time) error {

	usec := timestamp.UnixNano() / int64(time.Microsecond)

	j.mutex.Lock()
	defer j.mutex.Unlock()
	if ret := C.sd_journal_seek_realtime_usec(j.sdJournal, C.uint64_t(usec)); ret < 0 {
		return fmt.Errorf("failed seek to timestamp %v: %w", timestamp, syscall.Errno(-ret))
	}

	return nil
}

// SeekCursor moves cursor to specified cursor.
// NOTE: This call must be followed by a call to Next (or a similar call)
// before any data can be read
func (j *Journal) SeekCursor(cursor string) error {

	c := C.CString(cursor)
	defer C.free(unsafe.Pointer(c))

	j.mutex.Lock()
	ret := int(C.sd_journal_seek_cursor(j.sdJournal, c))
	j.mutex.Unlock()

	if ret != 0 {
		return syscall.Errno(-ret)
	}

	return nil
}

// Cursor returns the current cursor position
func (j *Journal) Cursor() (string, error) {
	var cursor *C.char

	j.mutex.Lock()
	defer j.mutex.Unlock()

	if ret := C.sd_journal_get_cursor(j.sdJournal, &cursor); ret < 0 {
		return "", fmt.Errorf("failed to read cursor: %w", syscall.Errno(-ret))
	}

	defer C.free(unsafe.Pointer(cursor))

	return C.GoString(cursor), nil
}

// TestCursor tests if the current position in the journal
// matches the specified cursor
func (j *Journal) TestCursor(cursor string) (bool, error) {

	c := C.CString(cursor)
	defer C.free(unsafe.Pointer(c))

	j.mutex.Lock()
	ret := C.sd_journal_test_cursor(j.sdJournal, c)
	j.mutex.Unlock()

	if ret < 0 {
		return false, fmt.Errorf("failed to test cursor: %w", syscall.Errno(-ret))
	}

	return ret > 0, nil
}

// SetDataThreshold sets the data field size threshold for data returned by
// GetData. Set to 0 to disable threshold and return all data.
func (j *Journal) SetDataThreshold(threshold uint64) error {

	j.mutex.Lock()
	defer j.mutex.Unlock()

	if ret := C.sd_journal_set_data_threshold(j.sdJournal, C.size_t(threshold)); ret < 0 {
		return fmt.Errorf("failed to set data threshold: %w", syscall.Errno(-ret))
	}

	return nil
}

// Field returns the content of a field at current position
func (j *Journal) Field(name string) (string, error) {

	var l C.size_t
	var d unsafe.Pointer

	f := C.CString(name)
	defer C.free(unsafe.Pointer(f))

	j.mutex.Lock()
	defer j.mutex.Unlock()

	if ret := C.sd_journal_get_data(j.sdJournal, f, &d, &l); ret < 0 {
		return "", fmt.Errorf("failed to get field '%s': %w", name, syscall.Errno(-ret))
	}

	data := C.GoStringN((*C.char)(d), C.int(l))

	return strings.TrimPrefix(data, name+"="), nil
}

// ReadEntry reads a full entry from current cursor position
func (j *Journal) ReadEntry() (*Entry, error) {

	entry := &Entry{
		Fields: Fields{},
	}

	var timestampUsec C.uint64_t
	var bootID C.sd_id128_t

	j.mutex.Lock()
	defer j.mutex.Unlock()

	// Timestamp
	if ret := C.sd_journal_get_realtime_usec(j.sdJournal, &timestampUsec); ret < 0 {
		return nil, fmt.Errorf("failed to get realtime timestamp: %w", syscall.Errno(-ret))
	}

	entry.Timestamp = time.Unix(0, int64(timestampUsec)*int64(time.Microsecond))

	// Elapsed
	if ret := C.sd_journal_get_monotonic_usec(j.sdJournal, &timestampUsec, &bootID); ret < 0 {
		return nil, fmt.Errorf("failed to get monotonic timestamp: %w", syscall.Errno(-ret))
	}

	entry.Elapsed = time.Duration(int64(timestampUsec))

	// Cursor
	var cursor *C.char
	if ret := C.sd_journal_get_cursor(j.sdJournal, &cursor); ret < 0 {
		return nil, fmt.Errorf("failed to read cursor: %w", syscall.Errno(-ret))
	}
	entry.Cursor = C.GoString(cursor)
	C.free(unsafe.Pointer(cursor))

	var (
		d   unsafe.Pointer
		l   C.size_t
		ret C.int
	)

	C.sd_journal_restart_data(j.sdJournal)

	for {
		if ret = C.sd_journal_enumerate_data(j.sdJournal, &d, &l); ret == 0 {
			break
		} else if ret < 0 {
			return nil, fmt.Errorf("failed to read message field: %w", syscall.Errno(-ret))
		}

		msg := C.GoStringN((*C.char)(d), C.int(l))
		kv := strings.SplitN(msg, "=", 2)
		if len(kv) < 2 {
			return nil, fmt.Errorf("failed to parse field")
		}

		entry.Fields[kv[0]] = kv[1]
	}

	return entry, nil
}

// Usage returns the journal disk space usage.
func (j *Journal) Usage() (uint64, error) {

	var usage C.uint64_t

	j.mutex.Lock()
	defer j.mutex.Unlock()

	if ret := C.sd_journal_get_usage(j.sdJournal, &usage); ret < 0 {
		return 0, fmt.Errorf("failed to get disk space usage: %w", syscall.Errno(-ret))
	}

	return uint64(usage), nil
}

// Wait will synchronously wait for the journal get changed. If
// -1 is passed as timeout, Wait will infinitely.
func (j *Journal) Wait(timeout time.Duration) (WakeupEvent, error) {

	var t uint64

	if timeout == -1 {
		t = math.MaxUint64 // No timeout
	} else {
		t = uint64(timeout / time.Microsecond)
	}

	j.mutex.Lock()
	ret := C.sd_journal_wait(j.sdJournal, C.uint64_t(t))
	j.mutex.Unlock()

	if ret < 0 {
		return NoOperation, fmt.Errorf("failed to wait for journal change: %w", syscall.Errno(-ret))
	}

	var event WakeupEvent

	switch ret {
	case C.SD_JOURNAL_NOP:
		event = NoOperation
	case C.SD_JOURNAL_APPEND:
		event = Append
	case C.SD_JOURNAL_INVALIDATE:
		event = Invalidate
	}

	return event, nil
}

// FlushMatches removes all matches, disjunctions and conjunctions
// from the journal instance.
func (j *Journal) FlushMatches() {
	j.mutex.Lock()
	C.sd_journal_flush_matches(j.sdJournal)
	j.mutex.Unlock()
}

// AddMatch adds a match expression to the journal instance
func (j *Journal) AddMatch(m *Match) error {

	if m == nil || len(m.expr) == 0 {
		return errors.New("no match expression to add")
	}

	for _, expr := range m.expr {
		var ret C.int

		j.mutex.Lock()

		switch expr.op {
		case matchOpField:
			for _, v := range expr.values {
				match := expr.field + "=" + v
				m := C.CString(match)
				ret = C.sd_journal_add_match(j.sdJournal, unsafe.Pointer(m), C.size_t(len(match)))
				C.free(unsafe.Pointer(m))
			}
		case matchOpAnd:
			ret = C.sd_journal_add_conjunction(j.sdJournal)
		case matchOpOr:
			ret = C.sd_journal_add_disjunction(j.sdJournal)
		}

		j.mutex.Unlock()

		if ret < 0 {
			return fmt.Errorf("failed to add match: %w", syscall.Errno(-ret))
		}
	}

	// Save match. Might be needed later if Tail is called that
	// will clone this instance
	j.matches = append(j.matches, m)

	return nil
}

// Catalog reads the message catalog entry pointed to by current cursor
func (j *Journal) Catalog() (string, error) {

	var c *C.char

	j.mutex.Lock()
	defer j.mutex.Unlock()

	if ret := C.sd_journal_get_catalog(j.sdJournal, &c); ret < 0 {
		return "", fmt.Errorf("failed to read catalog entry: %w", syscall.Errno(-ret))
	}

	defer C.free(unsafe.Pointer(c))

	return C.GoString(c), nil
}

// UniqueValues returns all unique values for a given field.
func (j *Journal) UniqueValues(field string) ([]string, error) {

	var result []string

	j.mutex.Lock()
	defer j.mutex.Unlock()

	f := C.CString(field)
	defer C.free(unsafe.Pointer(f))

	if ret := C.sd_journal_query_unique(j.sdJournal, f); ret < 0 {
		return nil, fmt.Errorf("failed to query journal: %w", syscall.Errno(-ret))
	}

	var d unsafe.Pointer
	var l C.size_t

	C.sd_journal_restart_unique(j.sdJournal)

	for {

		ret := C.sd_journal_enumerate_unique(j.sdJournal, &d, &l)
		if ret == 0 {
			break
		} else if ret < 0 {
			return nil, fmt.Errorf("failed to read field: %w", syscall.Errno(-ret))
		}

		result = append(result,
			strings.TrimPrefix(C.GoStringN((*C.char)(d), C.int(l)), field+"="))
	}

	return result, nil
}
