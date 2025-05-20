//go:build msgsample
// +build msgsample

package tui

import (
	"encoding/json"
	"os"
	"os/signal"
	"reflect"
	"sync"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
)

// The code in this file is only compiled when you build with
//   go build -tags msgsample
// It samples the frequency of message types that hit the Update loop and
// dumps the counts into msg_sample.json when the program receives SIGINT
// (Ctrl-C) or exits normally via m.quitting.

var (
    msgSampleCounts sync.Map // map[string]*atomic.Int64
    onceDump        sync.Once
)

func recordMsgSample(msg tea.Msg) {
    t := reflect.TypeOf(msg)
    var key string
    if t == nil {
        key = "<nil>"
    } else {
        key = t.String()
    }
    v, _ := msgSampleCounts.LoadOrStore(key, new(atomic.Int64))
    v.(*atomic.Int64).Add(1)
}

func init() {
    // Make sure we dump on Ctrl+C.
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt)
    go func() {
        <-c
        dumpMsgSample()
        os.Exit(0)
    }()
}

// dumpMsgSample writes the collected counts to msg_sample.json in the current
// working directory.
func dumpMsgSample() {
    onceDump.Do(func() {
        out := make(map[string]int64)
        msgSampleCounts.Range(func(k, v interface{}) bool {
            out[k.(string)] = v.(*atomic.Int64).Load()
            return true
        })
        b, err := json.MarshalIndent(out, "", "  ")
        if err != nil {
            return
        }
        _ = os.WriteFile("msg_sample.json", b, 0644)
    })
}

// finalizeMsgSampling flushes counts to disk. It can be called from regular
// shutdown paths (e.g. key 'q').
func finalizeMsgSampling() {
    dumpMsgSample()
} 