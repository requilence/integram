package multipartstreamer

import (
	"bytes"
	"io"
	"io/ioutil"
	"mime/multipart"
	"os"
	"path/filepath"
	"testing"
)

func TestMultipartFile(t *testing.T) {
	path, _ := os.Getwd()
	file := filepath.Join(path, "multipartstreamer.go")
	stat, _ := os.Stat(file)

	ms := New()
	err := ms.WriteFields(map[string]string{"a": "b"})
	if err != nil {
		t.Fatalf("Error writing fields: %s", err)
	}

	err = ms.WriteFile("file", file)
	if err != nil {
		t.Fatalf("Error writing file: %s", err)
	}

	diff := ms.Len() - stat.Size()
	if diff != 363 {
		t.Error("Unexpected multipart size")
	}

	data, err := ioutil.ReadAll(ms.GetReader())
	if err != nil {
		t.Fatalf("Error reading multipart data: %s", err)
	}

	buf := bytes.NewBuffer(data)
	reader := multipart.NewReader(buf, ms.Boundary())

	part, err := reader.NextPart()
	if err != nil {
		t.Fatalf("Expected form field: %s", err)
	}

	if str := part.FileName(); str != "" {
		t.Errorf("Unexpected filename: %s", str)
	}

	if str := part.FormName(); str != "a" {
		t.Errorf("Unexpected form name: %s", str)
	}

	if by, _ := ioutil.ReadAll(part); string(by) != "b" {
		t.Errorf("Unexpected form value: %s", string(by))
	}

	part, err = reader.NextPart()
	if err != nil {
		t.Fatalf("Expected file field: %s", err)
	}

	if str := part.FileName(); str != "multipartstreamer.go" {
		t.Errorf("Unexpected filename: %s", str)
	}

	if str := part.FormName(); str != "file" {
		t.Errorf("Unexpected form name: %s", str)
	}

	src, _ := ioutil.ReadFile(file)
	if by, _ := ioutil.ReadAll(part); string(by) != string(src) {
		t.Errorf("Unexpected file value")
	}

	part, err = reader.NextPart()
	if err != io.EOF {
		t.Errorf("Unexpected 3rd part: %s", part)
	}
}

func TestMultipartReader(t *testing.T) {
	ms := New()

	err := ms.WriteReader("file", "code/bass", 3, bytes.NewBufferString("ABC"))
	if err != nil {
		t.Fatalf("Error writing reader: %s", err)
	}

	if size := ms.Len(); size != 244 {
		t.Errorf("Unexpected multipart size: %d", size)
	}

	data, err := ioutil.ReadAll(ms.GetReader())
	if err != nil {
		t.Fatalf("Error reading multipart data: %s", err)
	}

	buf := bytes.NewBuffer(data)
	reader := multipart.NewReader(buf, ms.Boundary())

	part, err := reader.NextPart()
	if err != nil {
		t.Fatalf("Expected file field: %s", err)
	}

	if str := part.FileName(); str != "code/bass" {
		t.Errorf("Unexpected filename: %s", str)
	}

	if str := part.FormName(); str != "file" {
		t.Errorf("Unexpected form name: %s", str)
	}

	if by, _ := ioutil.ReadAll(part); string(by) != "ABC" {
		t.Errorf("Unexpected file value")
	}

	part, err = reader.NextPart()
	if err != io.EOF {
		t.Errorf("Unexpected 2nd part: %s", part)
	}
}
