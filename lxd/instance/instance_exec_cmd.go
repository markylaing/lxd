package instance

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// Cmd represents a local or remote command being run.
type Cmd interface {
	Wait() (int, error)
	PID() int
	Signal(s unix.Signal) error
	WindowResize(fd, winchWidth, winchHeight int) error
}

func NewMockCmd(stdin *os.File, stdout *os.File, stderr *os.File) Cmd {
	ch := make(chan struct{})

	go func() {
		_, _ = fmt.Fprint(stdout, "Hello, World!\n")
		time.Sleep(time.Second)
		close(ch)
	}()

	return &mockCmd{
		done: ch,
	}
}

type mockCmd struct {
	done chan struct{}
}

func (m *mockCmd) Wait() (int, error) {
	<-m.done
	return 0, nil
}

func (m *mockCmd) PID() int {
	return 1
}

func (mockCmd) Signal(s unix.Signal) error {
	return nil
}

func (mockCmd) WindowResize(fd, winchWidth, winchHeight int) error {
	return nil
}
