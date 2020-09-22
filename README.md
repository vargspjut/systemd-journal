# systemd-journal

systemd-journal provide go-bindings to systemd's journal logging facility available on most modern Linux systems. Supported features include filtered reading, writing with custom fields and log tail following. Bindings are accomplished using [cgo](https://golang.org/cmd/cgo/) to call the [sdjournal C API](https://www.freedesktop.org/software/systemd/man/sd-journal.html) directly.


**NOTE**: Although systemd-journal itself is thread-safe, that is not enough to satisfy the requirements of the sdjournal API. Besides when writing to the journal, all calls made against the sdjournal must be made on the very same thread used when a journal instance was created. You may however create multiple instances on different threads.

## Getting started
To access the journal, create a journal instance by calling *journal.Open*. On the returned instance you may seek, filter and read log entries.

```golang
package main

import (
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

### Filtering and matching
While reading the journal you may apply *Match* objects to influence what entries that will be returned from the journal. You can apply any number of matches. A match can be combined with logical AND and OR directives to create complex filtering. To clear all filters, call *FlushMatches*.

```golang
// Code left out for brevity

// Only return entries from the sshd and
// gdm units OR empty messages
m := journal.NewMatch().
    Match(journal.FieldUnit, "sshd.service").
    Match(journal.FieldUnit, "gdm.service").
    Or().
    Match(journa.FieldMessage, "")

// Add match. jour is a journal instance.
if err := jour.AddMatch(m); err != nil {
    wlog.Fatal(err)
}
```
NOTE: sytemd-journal exposes all offically defined fields as *journal.Field[name].

### Following
To start following the journal from the **current** position, call *Follow*. Provide a callback to receive new journal entries in a thread safe manner. Call the returned *FollowStop* function to stop following.

**NOTE 1**: If the current cursor doesn't point to an log entry, Follow will automatically seek to the end of the journal and start following from there.

**NOTE 2**: Follow runs in a go-routine. Since one single journal instance must not be called from multiple threads, a new instance will be created with the very same configuration and state as the parent instance.

```golang
// Code left out for brevity

// Seek to the very end of the journal.
// Next is implicity called by Follow.
if err := jour.SeekTail(); err != nil {
    wlog.Fatal(err)
}

// Start following from the end of the journal. 
// Provide a callback to receive new entries. 
// Save the returned stop function and call it to 
// stop following the journal from this instance.
stop, err := jour.Follow(func(entry *journal.Entry, err error) {
    if err != nil {
        // ErrFollowStopped is a controlled error
        // intended to signal to the handler about
        // an explicit call to stop following.
        if err == journal.ErrFollowStopped {
            wlog.Info("Journal following stopped")
            return
        }

        wlog.Fatal(err)
    }

    wlog.Infof("\n%s", entry)
})

if err != nil {
    wlog.Fatal(err)
}

defer stop()

// To keep process from exiting
fmt.Println("Press Ctrl-C to exit")
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit
```

### Writing to the journal
To write to the journal, use the package-exported functions *Submit* or *SubmitWithFields*. The latter lets you specify custom fields when writing to the journal.

**NOTE 1** As an exception to most sdjournal API calls, writing to the journal is natively thread-safe and may be called from any thread without explicit locking.

**NOTE 2** If *journal.FieldPriority* or *journal.FieldMessage* is part of fields when calling SubmitWithFields, arguments priority and message will be ignored. 

**NOTE 3** All field names must be all upper-case and may not contain any preceeding '_' characters. An error will be returned if encountered.

```golang
// Code left out for brevity

// Write a simple log entry with logging
// priority Informal
journal.Submit(journal.PriorityInfo, "A message")

// Write a log entry with custom fields and
// priority Error.
journal.SubmitWithFields(journal.PriorityError, "An error message",
    Fields{
        "CUSTOM": "custom value",
        journal.FieldErrNo: "-2",
    },
)
```

## Documentation
Besides this README.md document code documentation can be generated by running the built-in tool go doc.

```bash
cd [...]/systemd-journal
go doc -all systemd-journal
```