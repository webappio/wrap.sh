package wrap

import (
	"github.com/layer-devops/wrap.sh/src/protocol"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const maxFileReadSize = 50 * 1024 * 1024

var fileTooBigError = errors.New("file too big")

func (client *Client) readFile(msg *protocol.FileRead, maxFileSize int64) (*protocol.FileReadResult, error) {
	info, err := os.Stat(msg.GetPath())
	if err != nil {
		return nil, errors.Wrap(err, "stat")
	}
	if info.Size() > maxFileSize {
		return nil, fileTooBigError
	}
	content, err := ioutil.ReadFile(msg.GetPath())
	if err != nil {
		return nil, errors.Wrap(err, "file read")
	}
	cmd := exec.Command("file", "-i", msg.GetPath())
	mimeType := ""
	output, err := cmd.CombinedOutput()
	if err == nil {
		mimeType = strings.TrimSpace(strings.TrimPrefix(string(output), msg.GetPath()+":"))
	}
	return &protocol.FileReadResult{
		Data:     content,
		Path:     msg.GetPath(),
		MimeType: mimeType,
	}, nil
}

func (client *Client) handleFileRead(msg *protocol.FileRead, listenerId uint32) error {
	fileReadResult, err := client.readFile(msg, maxFileReadSize)
	if err != nil {
		err = client.send(&protocol.MessageFromWrapClient{
			Spec: &protocol.MessageFromWrapClient_FileReadResult{
				FileReadResult: &protocol.FileReadResult{
					Error: err.Error(),
				},
			},
			ListenerId: listenerId,
		})
		if err != nil {
			panic(errors.Wrap(err, "send file-read result"))
		}
		return nil
	}
	err = client.send(&protocol.MessageFromWrapClient{
		Spec: &protocol.MessageFromWrapClient_FileReadResult{
			FileReadResult: fileReadResult,
		},
		ListenerId: listenerId,
	})
	if err != nil {
		panic(errors.Wrap(err, "send file-read result"))
	}
	return nil
}

func (client *Client) readFileDir(msg *protocol.FileReadDir) (*protocol.FileReadDirResult, error) {
	result := &protocol.FileReadDirResult{
		Entry: []*protocol.DirEntry{},
		Path:  msg.GetPath(),
	}
	err := filepath.Walk(msg.GetPath(), func(path string, info os.FileInfo, err error) error {
		if path == msg.GetPath() {
			return nil
		}
		entry := &protocol.DirEntry{
			Name:    info.Name(),
			IsDir:   info.IsDir(),
			IsEmpty: true,
			Path:    path,
		}
		cmd := exec.Command("file", "-i", path)
		output, err := cmd.CombinedOutput()
		if err == nil {
			entry.MimeType = strings.TrimSpace(strings.TrimPrefix(string(output), path+":"))
		} else {
			// might be best to fail silently
		}
		result.Entry = append(result.Entry, entry)
		if info.IsDir() {
			//TODO: HACK
			lsCmd := exec.Command("ls", "-al", path)
			output, err := lsCmd.CombinedOutput()
			if err == nil {
				entry.IsEmpty = len(strings.Split(string(output), "\n")) < 5
			} else {
				// e.g. permission error
				// might be best to fail silently
			}
			return filepath.SkipDir
		}
		return nil
	})
	return result, err
}

func (client *Client) handleFileReadDir(msg *protocol.FileReadDir, listenerId uint32) error {
	fileReadDirResult, err := client.readFileDir(msg)
	if err != nil {
		err = client.send(&protocol.MessageFromWrapClient{
			Spec: &protocol.MessageFromWrapClient_FileReadDirResult{
				FileReadDirResult: &protocol.FileReadDirResult{
					Error: err.Error(),
				},
			},
			ListenerId: listenerId,
		})
		if err != nil {
			panic(errors.Wrap(err, "send file-read-dir result"))
		}
		return nil
	}
	err = client.send(&protocol.MessageFromWrapClient{
		Spec: &protocol.MessageFromWrapClient_FileReadDirResult{
			FileReadDirResult: fileReadDirResult,
		},
		ListenerId: listenerId,
	})
	if err != nil {
		panic(errors.Wrap(err, "send file-read-dir result"))
	}
	return nil
}
