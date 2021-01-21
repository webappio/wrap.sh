package wrap

import (
	"bytes"
	"github.com/gorilla/websocket"
	"github.com/layer-devops/wrap.sh/src/protocol"
	"github.com/pkg/errors"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"time"
)

type Client struct {
	TestCommand       string
	Token             string
	WebsocketLocation string
	DashboardURL      string
	UUID              string
	LogDebug          bool
	TimeoutMinutes    int
	NumRetries        int
	wasAccessed       bool
	ExitCode          int

	/*
		Privacy settings.
		Telemetry fields set in this map
		are redacted before anything is sent.
	*/
	ExcludedTelemetryFields map[string]bool

	// TCP
	connMapWriteMutex sync.Mutex
	connections       map[uint32]*tunnelTcpConn

	// Websocket
	ws           *websocket.Conn
	recvBuf      bytes.Buffer
	closed       bool
	closingMutex sync.Mutex
	closedChan   chan struct{}
	wsWriteMutex sync.Mutex

	// tty
	terminal *terminal
}

func (client *Client) debugLog(format string, args ...interface{}) {
	if client.LogDebug {
		log.Printf("[debug] "+format+"\n", args...)
	}
}

func (client *Client) Log(format string, args ...interface{}) {
	log.Printf("[wrap.sh] "+format+"\n", args...)
}

/* runs the provided test command and returns whether it succeeded. */
func (client *Client) runTestCommand() (bool, error) {
	testCmd := exec.Command("bash", "-c", client.TestCommand)
	testCmd.Env = os.Environ()
	stderrPipe, err := testCmd.StderrPipe()
	if err != nil {
		return false, errors.Wrap(err, "stderr pipe")
	}
	defer stderrPipe.Close()
	stdoutPipe, err := testCmd.StdoutPipe()
	if err != nil {
		return false, errors.Wrap(err, "stdout pipe")
	}
	defer stdoutPipe.Close()
	go io.Copy(os.Stdout, stdoutPipe)
	go io.Copy(os.Stderr, stderrPipe)
	client.Log("Running \"%v\"", client.TestCommand)
	err = testCmd.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			// non-0 return code
			client.ExitCode = exitError.ExitCode()
			client.Log("The command had a non-zero exit code: %v", exitError.ExitCode())
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (client *Client) runTestCommandWithRetries() (bool, error) {
	// wrap with bash to allow for pipes and such
	for attempt := 0; attempt <= client.NumRetries; attempt++ {
		if client.NumRetries > 0 && attempt > 0 {
			client.Log("Retrying command (%v/%v)...", attempt, client.NumRetries)
		}
		succeeded, err := client.runTestCommand()
		if err != nil {
			return false, err
		}
		if succeeded {
			return true, nil
		}
	}
	return false, nil
}

func (client *Client) Run() {
	if client.TestCommand == "" {
		client.Log("No command was specified, shutting down.")
		return
	}
	commandSucceeded, err := client.runTestCommandWithRetries()
	if err != nil {
		log.Fatalf(errors.Wrap(err, "run test command").Error())
		return
	}
	if commandSucceeded {
		return
	}
	err = client.connectToServer()
	if err != nil {
		panic(errors.Wrap(err, "could not connect"))
	}
	go client.startPty()
	go client.listenServer()
	go client.timeout()
	interrupt := make(chan os.Signal, 1)
	client.closedChan = make(chan struct{}, 1)
	signal.Notify(interrupt, os.Interrupt)
	select {
	case <-interrupt:
		client.close()
	case <-client.closedChan:
		return
	}
}

func (client *Client) timeout() {
	// Exit if timeout disabled
	if client.TimeoutMinutes < 1 {
		return
	}
	client.Log("Timing out if not accessed within %v minute(s).", client.TimeoutMinutes)
	ticker := time.NewTicker(time.Minute * 1)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			client.TimeoutMinutes -= 1
			if client.wasAccessed {
				client.Log("client was accessed, timeout cancelled.")
				return
			}
			if client.TimeoutMinutes == 0 {
				client.Log("Closing connection due to timeout...")
				client.close()
				return
			}
		}
	}
}

func (client *Client) close() {
	client.closingMutex.Lock()
	defer client.closingMutex.Unlock()
	if client.closed {
		return
	}
	client.closed = true
	client.debugLog("closing...")
	// close terminal
	client.terminal.closer.Do(client.terminal.close)
	client.debugLog("closed bash tty")
	// close all open connections
	for _, rc := range client.connections {
		rc.Close()
	}
	client.debugLog("closed tcp connections")
	_ = client.ws.Close()
	client.debugLog("closed websocket connection")
	// stop waiting for interrupt
	close(client.closedChan)
}

func (client *Client) HandleMessage(message *protocol.MessageToWrapClient) error {
	// some messages are sent by specific listeners on the server side (e.g. file read)
	listenerId := message.GetListenerId()
	// TCP
	if write := message.GetTcpWriteCall(); write != nil {
		client.wasAccessed = true
		return client.handleTcpWriteCall(write)
	}
	if read := message.GetTcpReadCall(); read != nil {
		client.wasAccessed = true
		return client.handleTcpReadCall(read)
	}
	if dial := message.GetTcpDialCall(); dial != nil {
		client.wasAccessed = true
		return client.handleTcpDialCall(dial)
	}
	// Terminal Pty
	if termWrite := message.GetTerminalWrite(); termWrite != nil {
		client.wasAccessed = true
		return client.handleTerminalWrite(termWrite)
	}
	if termWidth := message.GetTerminalWidth(); termWidth != nil {
		client.wasAccessed = true
		return client.handleTerminalWidth(termWidth)
	}
	// File browser
	if fileRead := message.GetFileRead(); fileRead != nil {
		client.wasAccessed = true
		return client.handleFileRead(fileRead, listenerId)
	}
	if fileReadDir := message.GetFileReadDir(); fileReadDir != nil {
		client.wasAccessed = true
		return client.handleFileReadDir(fileReadDir, listenerId)
	}
	// response to our Hello message
	if helloResponse := message.GetHelloResponse(); helloResponse != nil {
		return client.handleHelloResponse(helloResponse)
	}
	if err := message.GetError(); err != "" {
		return errors.New(err)
	}
	// Dashboard user or the wrap.sh server requests that this client immediately shuts down.
	if message.GetClose() {
		client.Log("Shut-down requested, closing connection...")
		client.close()
	}
	return errors.New("unexpected message type from wrap.sh server")
}
