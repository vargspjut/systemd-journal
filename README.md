# systemd-journal

systemd-journal provide go-bindings to systemd's journal logging facility available on most modern Linux systems. Supported features include filtered reading, writing with custom fields and log tail following. Bindings are accomplished using [cgo](https://golang.org/cmd/cgo/) to call the [sdjournal C API](https://www.freedesktop.org/software/systemd/man/sd-journal.html) directly.


NOTE: Although systemd-journal itself is thread-safe, that is not enough to satisfy the requirements of the sdjournal API. Besides when writing to the journal, all calls made against the sdjournal must be made on the very same thread used when a journal instance was created. You may however create multiple instances on different threads.

## Getting started
To access the journal, create a journal instance by calling *journal.Open*. On the returned instance you may seek, filter and read log entries.

```golang
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	journal "github.com/vargspjut/systemd-journal"

	"github.com/vargspjut/wlog"
)

func main() {
    // Open a journal instance
    jour, err := journal.Open()
    if err != nil {
        wlog.Fatal(err)
    }

    // Close instance on exit
    defer jour.Close()

    // Seek to the very beginning of the journal
    if err := jour.SeekHead(); err != nil {
        wlog.Fatal(err)
    }
    
    // After each seek operation is paramount
    // to call Next, Previous or similar to move
    // the cursor to point to an actual entry
    if _, err := jour.Next(); err != nil {
        wlog.Fatal(err)
    }

    // Read full entry
    entry, err := jour.ReadEntry()
    if err != nil {
        wlog.Fatal(err)
    }

    // Write entry to std-out.
    // NOTE: entry supports fmt.Stringer
    // producing a JSON-representation.
    wlog.Infof("\n%s", entry)
}
```