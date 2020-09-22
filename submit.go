package journal

// #include <systemd/sd-journal.h>
// #include <stdlib.h>
import (
	"C"
)
import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// Submit submits a new entry to the journal
func Submit(p Priority, m string) error {
	return SubmitWithFields(p, m, Fields{})
}

// SubmitWithFields submits a new entry to the journal
// With optional fields
func SubmitWithFields(p Priority, m string, f Fields) error {

	if f == nil {
		f = Fields{}
	}

	// Add priority field if not already present
	if _, ok := f[FieldPriority]; !ok {
		f[FieldPriority] = strconv.Itoa(int(p))
	}

	// Add message field if not already present
	if _, ok := f[FieldMessage]; !ok {
		f[FieldMessage] = m
	}

	iov := make([]C.struct_iovec, len(f))

	i := 0
	for k, v := range f {

		if k == "" {
			return errors.New("Field name must not be empty")
		}
		if k[0] == '_' {
			return errors.New("Field name must not begin with the character '_'")
		}
		fnv := strings.ToUpper(k)
		if fnv != k {
			return errors.New("Field name must be upper-case")
		}

		fnv = fnv + "=" + v

		f := C.CString(fnv)
		defer C.free(unsafe.Pointer(f))

		iov[i].iov_len = C.ulong(len(fnv))
		iov[i].iov_base = unsafe.Pointer(f)
		i++
	}

	if ret := C.sd_journal_sendv((*C.struct_iovec)(unsafe.Pointer(&iov[0])), C.int(i)); ret < 0 {
		return fmt.Errorf("failed to get send entry to journal: %w", syscall.Errno(-ret))
	}

	return nil
}
