// +build linux

package journal

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"syscall"
	"time"
)

const (
	// Timeout waiting for new journal entries
	waitTimeout = time.Duration(300 * time.Millisecond)
)

// FollowHandler is the callback that will receive journal entries.
// If an error occurs during processing, entry will be nil and
// err populated with the error encountered. An error is
// unrecoverable and no more entries will be received after this.
type FollowHandler func(entry *Entry, err error)

// FollowStop stops following the journal when called
type FollowStop func()

// Follow starts reading entries from the current cursor position
// and then starts tracking changes at the end of the journal and
// calls the provided function for each entry read.
// Use the returned func to stop processing.
// NOTE: Since the journal API does NOT allow multiple threads
// to access the same instance, even with locking, a new instance
// is created with same configuration as the parent instance.
func (j *Journal) Follow(h FollowHandler) (FollowStop, error) {

	if h != nil && reflect.ValueOf(h).IsNil() {
		return nil, errors.New("a follow handler must be provided")
	}

	eof := false

	cursor, err := j.Cursor()
	if err != nil {
		if errors.Is(err, syscall.EADDRNOTAVAIL) {
			// Position does not point to an entry. Decide EOF since this
			// is a follow method. Seek to tail, move one back and
			// retry reading the cursor.
			err = j.SeekTail()
			if err == nil {
				_, err = j.Previous()
				if err == nil {
					cursor, err = j.Cursor()
				}
			}

			if err != nil {
				return nil, err
			}

			eof = true
		}
	}

	done := make(chan bool, 1)
	once := sync.Once{}

	go followJournal(h, done, cursor, j.matches, eof)

	return func() {
		once.Do(func() {
			done <- true
		})
	}, nil
}

func followJournal(h FollowHandler, done <-chan bool, cursor string, matches []*Match, eof bool) {

	jour, err := Open()
	if err != nil {
		h(nil, err)
		return
	}

	defer jour.Close()

	for _, m := range matches {
		if err := jour.AddMatch(m); err != nil {
			h(nil, fmt.Errorf("failed to add match: %w", err))
			return
		}
	}

	if err := jour.SeekCursor(cursor); err != nil {
		h(nil, fmt.Errorf("failed to seek to cursor: %w", err))
		return
	}

	// If EOF, move to next position and let loop enter wait mode.
	if eof {
		if _, err := jour.Next(); err != nil {
			h(nil, fmt.Errorf("failed move cursor to next position: %w", err))
			return
		}
	}

exit:
	for {
		ret, err := jour.Next()
		if err != nil {
			h(nil, fmt.Errorf("failed to move cursor to next entry: %w", err))
			break exit
		}

		if ret == 0 {
			for {
				select {
				case <-done:
					h(nil, ErrFollowStopped)
					break exit
				default:
				}

				wue, err := jour.Wait(waitTimeout)
				if err != nil {
					h(nil, fmt.Errorf("failed to wait for new entries: %w", err))
					break exit
				}

				if wue == NoOperation {
					continue
				} else {
					// Break out of inner for loop to read entries
					break
				}
			}
		} else {
			select {
			case <-done:
				h(nil, ErrFollowStopped)
				break exit
			default:
				e, err := jour.ReadEntry()
				if err != nil {
					h(nil, fmt.Errorf("failed to read entry: %w", err))
					break exit
				}

				h(e, nil)
			}
		}
	}

}
