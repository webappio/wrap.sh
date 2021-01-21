package wrapper

import (
	"crypto/tls"
	"fmt"
	"github.com/layer-devops/wrap.sh/src/protocol"
	"github.com/pkg/errors"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

func httpHelloMessage(host string) []byte {
	return []byte(fmt.Sprintf("GET / HTTP/1.1\nHost: %v", host))
}
func tlsHelloMessage(host string) []byte {
	return []byte(fmt.Sprintf(`GET / TLS/1.0
Host: %v`, host))
}
func tlsUpgradeMessage(host string) []byte {
	return []byte(fmt.Sprintf(`OPTIONS * HTTP/1.1
Host: %v
Upgrade: TLS/1.0
Connection: Upgrade`, host))
}

// scans all tcp ports for http servers
func discoverServices() ([]*protocol.Service, error) {
	services := []*protocol.Service{}
	servicesLock := sync.Mutex{}
	appendService := func(address string) {
		servicesLock.Lock()
		defer servicesLock.Unlock()
		services = append(services, &protocol.Service{
			Address: address,
		})
	}
	progressLock := sync.Mutex{}
	pg := 0
	endPg := 65535
	barsize := 40
	done := make(chan struct{})
	progress := func(amt int) {
		progressLock.Lock()
		defer progressLock.Unlock()
		pg += amt
		progF := float32(pg) / float32(endPg)
		percent := int(progF * 100)
		dots := "\r"
		b := int(progF * float32(barsize))
		for i := 0; i < barsize; i++ {
			if i <= b {
				dots += "="
			} else {
				dots += "."
			}
		}
		dots += fmt.Sprintf(" %v%%", percent)
		fmt.Print(dots)
		if pg == endPg {
			close(done)
		}
	}
	for startPort := 1; startPort <= 60001; startPort += 10000 {
		endPort := startPort + 10000
		if endPort > 65536 {
			endPort = 65536
		}
		go func(sp int, ep int) {
			dialer := &net.Dialer{
				Timeout: time.Millisecond * 50,
			}
			amt := 0
			signalProgress := func() {
				amt++
				if amt >= 100 {
					progress(amt)
					amt = 0
				}
			}
			for port := sp; port < ep; port++ {
				addr := fmt.Sprintf("localhost:%v", port)
				ok := false
				tlsOk := false
				for _, tlsEnabled := range []bool{true, false} {
					buf := make([]byte, 4)
					conn, err := dialer.Dial("tcp", addr)
					if err != nil {
						break
					}
					closeConn := func() {
						_ = conn.Close()
					}
					_ = conn.SetReadDeadline(time.Now().Add(time.Millisecond * 50))
					useConn := conn
					payload := httpHelloMessage(addr)
					if tlsEnabled {
						useConn = tls.Client(conn, &tls.Config{
							InsecureSkipVerify: true,
							ServerName:         addr,
						})
						payload = tlsHelloMessage(addr)
					}
					_, err = useConn.Write(payload)
					if err != nil {
						closeConn()
						continue
					}
					_, err = io.ReadFull(useConn, buf)
					if err != nil {
						closeConn()
						continue
					}
					if string(buf) != "HTTP" {
						closeConn()
						continue
					}
					tlsOk = tlsEnabled
					ok = true
					closeConn()
					break
				}
				if !ok {
					signalProgress()
					continue
				}
				if tlsOk {
					appendService("https://" + addr)
				} else {
					appendService("http://" + addr)
				}
				signalProgress()
			}
			if amt > 0 {
				progress(amt)
			}
		}(startPort, endPort)
	}
	select {
	case <-done:
		log.Println("")
	}
	return services, nil
}

func (client *Client) sendHello() error {
	client.Log("Starting up a debug server...")
	msg := &protocol.Hello{}
	// knowing the working directory is handy for the file browser
	workingDirectory, err := os.Getwd()
	if err != nil {
		workingDirectory = "/"
	}
	msg.WorkingDirectory = workingDirectory
	client.debugLog("Getting pipeline info...")
	errs := populatePipelineInfo(msg)
	if len(errs) > 0 {
		for _, err := range errs {
			client.debugLog(errors.Wrap(err, "get pipeline info").Error())
		}
	}
	// exclude certain telemetry fields before phoning home,
	// based on provided settings
	client.debugLog("Excluding fields: %v", client.ExcludedTelemetryFields)
	redactPipelineInfo(msg, client.ExcludedTelemetryFields)
	if client.LogDebug {
		client.debugLog("** Sending the following metadata **")
		client.debugLog("  CI Provider: %v", msg.CiProvider)
		client.debugLog("  Slug: %v", msg.Slug)
		client.debugLog("  Commit hash: %v", msg.CommitHash)
		client.debugLog("  Branch name: %v", msg.BranchName)
		client.debugLog("  Pull request: %v", msg.PullRequest)
		client.debugLog("  Tag: %v", msg.Tag)
		client.debugLog("  Job Id: %v", msg.JobId)
		client.debugLog("  Build URL: %v", msg.BuildUrl)
		client.debugLog("  Build ID: %v", msg.BuildId)
		client.debugLog("  Author name: %v", msg.AuthorName)
		client.debugLog("  Author email: %v", msg.AuthorEmail)
		client.debugLog("  Author email domain: %v", msg.AuthorEmailDomain)
		client.debugLog("  Author avatar: %v", msg.AuthorAvatar)
	}
	client.debugLog("Discovering services...")
	services, err := discoverServices()
	if err != nil {
		client.debugLog(errors.Wrap(err, "find services").Error())
	} else if client.LogDebug {
		for _, service := range services {
			client.debugLog("found service %v", service.Address)
		}
	}
	msg.Service = services
	return client.send(&protocol.MessageFromWrapperClient{
		Spec: &protocol.MessageFromWrapperClient_Hello{
			Hello: msg,
		},
	})
}

func (client *Client) handleHelloResponse(msg *protocol.HelloResponse) error {
	dashboardUrl := msg.GetDashboardUrl()
	if dashboardUrl == "" {
		return errors.New("unable to request debug server")
	}
	client.DashboardURL = dashboardUrl
	client.Log("Debug server ready! Explore and make changes at:")
	client.Log(client.DashboardURL)
	return nil
}
