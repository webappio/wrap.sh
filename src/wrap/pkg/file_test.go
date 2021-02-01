package wrap

import (
	"github.com/layer-devops/wrap.sh/src/protocol"
	"io/ioutil"
	"os"
	"testing"
)

type tempFile struct {
	Path string
	File *os.File
}

func (t *tempFile) Remove() {
	t.File.Close()
	os.Remove(t.Path)
}

func makeTempFile(t *testing.T, contents string) *tempFile {
	f, err := ioutil.TempFile("", "Wrap.TestFileRead")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(contents)
	if err != nil {
		t.Fatal(err)
	}
	return &tempFile{
		Path: f.Name(),
		File: f,
	}
}

func TestFileRead(t *testing.T) {
	c := newBlankTestClient()
	var tempFileContents = "abc"
	var tempFileMimeType = "text/plain; charset=us-ascii"
	f := makeTempFile(t, tempFileContents)
	result, err := c.readFile(&protocol.FileRead{
		Path: f.Path,
	}, int64(len(tempFileContents)+100))
	f.Remove()
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("File read result was nil")
	}
	assertEqual(t, "Path", f.Path, result.Path)
	assertEqual(t, "MimeType", tempFileMimeType, result.MimeType)
	assertEqual(t, "Error", "", result.Error)
	assertNotNil(t, "Data", result.Data)
	assertEqual(t, "Data", tempFileContents, string(result.Data))
}

func TestFileReadTooBig(t *testing.T) {
	c := newBlankTestClient()
	var tempFileContents = "abc"
	f := makeTempFile(t, tempFileContents)
	result, err := c.readFile(&protocol.FileRead{
		Path: f.Path,
	}, int64(len(tempFileContents)-1))
	f.Remove()
	assertEqual(t, "err", fileTooBigError, err)
	assertNil(t, "result", result)
}
