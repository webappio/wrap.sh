package wrap

import (
	"github.com/creack/pty"
	"github.com/layer-devops/wrap.sh/src/protocol"
	"github.com/pkg/errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"
)

type terminal struct {
	close  func()
	closer sync.Once
	closed bool
	bash   *os.File
	stdin  io.Reader
	stdout io.Writer
}

/*
Starts a local bash shell for running commands sent
by Wrap Dashboard users, sends any output to the wrap.sh server.
*/
func (client *Client) startPty() {
	bash := exec.Command("bash")

	t := &terminal{}
	client.terminal = t

	// Prepare teardown function
	t.close = func() {
		t.closed = true
		err := bash.Process.Kill()
		if err != nil {
			panic(errors.Wrap(err, "close pty"))
		}
	}

	// Allocate a terminal for this channel
	bashf, err := pty.Start(bash)
	if err != nil {
		panic(errors.Wrap(err, "open pty"))
	}

	t.bash = bashf

	// send output to wrap.sh server
	var buf [1024]byte
	for {
		if t.closed {
			return
		}
		n, err := bashf.Read(buf[:])
		if err != nil {
			client.Log(errors.Wrap(err, "terminal output").Error())
			t.closer.Do(t.close)
			return
		}
		if n == 0 {
			continue
		}
		err = client.send(&protocol.MessageFromWrapClient{
			Spec: &protocol.MessageFromWrapClient_TerminalData{
				TerminalData: &protocol.TerminalData{
					Data: buf[:n],
				},
			}})
		if err != nil {
			panic(errors.Wrap(err, "send terminal data"))
		}
	}
}

func (client *Client) handleTerminalWrite(msg *protocol.TerminalData) error {
	if client.terminal == nil || client.terminal.closed {
		client.Log("write attempt for non-open terminal")
		return nil
	}
	d := msg.GetData()
	_, err := client.terminal.bash.Write(d)
	if err != nil {
		return err
	}
	return nil
}

// terminalSize stores the Height and Width of a terminal.
type terminalSize struct {
	Height uint16
	Width  uint16
	x      uint16 // unused
	y      uint16 // unused
}

func setTerminalSize(fd uintptr, w, h uint32) {
	ws := &terminalSize{Width: uint16(w), Height: uint16(h)}
	syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TIOCSWINSZ), uintptr(unsafe.Pointer(ws)))
}

func (client *Client) handleTerminalWidth(msg *protocol.TerminalWidth) error {
	if client.terminal == nil || client.terminal.closed {
		client.debugLog("resize attempt for non-open terminal")
		return nil
	}
	setTerminalSize(client.terminal.bash.Fd(), msg.GetNewWidth(), 40)
	return nil
}
