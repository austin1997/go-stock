//go:build goweb

package main

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-stock/backend/webauth"
)

func TestUploadAdmissionRejectsBeforeReadingBody(t *testing.T) {
	uploadDir := t.TempDir()
	entered := make(chan struct{}, maxConcurrentUploads)
	release := make(chan struct{})
	var quotaCalls atomic.Int32

	s := newUploadTestServer(uploadDir)
	s.enforceUploadQuota = func(string, int64) error {
		if quotaCalls.Add(1) <= maxConcurrentUploads {
			entered <- struct{}{}
			<-release
		}
		return nil
	}
	server := httptest.NewServer(uploadTestHandler(s, 7))
	defer server.Close()

	body, contentType := uploadMultipart(t, "small.txt", bytes.Repeat([]byte("x"), 1024))
	client := &http.Client{Transport: &http.Transport{ExpectContinueTimeout: 5 * time.Second}}

	var wg sync.WaitGroup
	errs := make(chan error, maxConcurrentUploads)
	for i := 0; i < maxConcurrentUploads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := postUpload(client, server.URL, contentType, bytes.NewReader(body), int64(len(body)), false)
			if err == nil {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					err = errors.New(resp.Status)
				}
			}
			errs <- err
		}()
	}
	for i := 0; i < maxConcurrentUploads; i++ {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatal("uploads did not reach the quota seam")
		}
	}

	probe := &countingReader{r: bytes.NewReader(body)}
	resp, err := postUpload(client, server.URL, contentType, probe, int64(len(body)), true)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusTooManyRequests)
	}
	if got := probe.n.Load(); got != 0 {
		t.Fatalf("rejected request body read %d bytes before admission", got)
	}

	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	// Completed requests must release admission slots.
	resp, err = postUpload(client, server.URL, contentType, bytes.NewReader(body), int64(len(body)), false)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("post-release status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestUploadSpillsLargePartAndRemovesMultipartTempFiles(t *testing.T) {
	multipartTemp := t.TempDir()
	t.Setenv("TMPDIR", multipartTemp)
	uploadDir := t.TempDir()
	parsed := make(chan struct{})
	release := make(chan struct{})

	s := newUploadTestServer(uploadDir)
	s.enforceUploadQuota = func(string, int64) error {
		close(parsed)
		<-release
		return errors.New("quota fixture")
	}
	server := httptest.NewServer(uploadTestHandler(s, 11))
	defer server.Close()

	payload := bytes.Repeat([]byte("z"), uploadMultipartMemoryBytes+(64<<10))
	body, contentType := uploadMultipart(t, "large.bin", payload)
	result := make(chan *http.Response, 1)
	errCh := make(chan error, 1)
	go func() {
		resp, err := postUpload(http.DefaultClient, server.URL, contentType, bytes.NewReader(body), int64(len(body)), false)
		result <- resp
		errCh <- err
	}()

	select {
	case <-parsed:
	case <-time.After(5 * time.Second):
		t.Fatal("upload did not finish multipart parsing")
	}
	entries, err := os.ReadDir(multipartTemp)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("large multipart part remained in memory instead of spilling to a temp file")
	}
	var spilled bool
	for _, entry := range entries {
		info, statErr := entry.Info()
		if statErr != nil {
			t.Fatal(statErr)
		}
		if info.Mode().IsRegular() && info.Size() >= int64(len(payload)) {
			spilled = true
		}
	}
	if !spilled {
		t.Fatalf("no spilled multipart file of at least %d bytes found", len(payload))
	}

	close(release)
	resp := <-result
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusInsufficientStorage {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInsufficientStorage)
	}
	entries, err = os.ReadDir(multipartTemp)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("multipart temp files not removed after error: %v", entryNames(entries))
	}
}

func newUploadTestServer(uploadDir string) *webServer {
	return &webServer{
		uploadSlots: make(chan struct{}, maxConcurrentUploads),
		uploadDir: func(uint) string {
			return uploadDir
		},
		enforceUploadQuota: webauth.EnforceTmpQuota,
	}
}

func uploadTestHandler(s *webServer, userID uint) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := &webauth.User{ID: userID, Username: "upload-test"}
		s.handleUpload(w, r.WithContext(webauth.WithUser(r.Context(), u)))
	})
}

func uploadMultipart(t *testing.T, filename string, data []byte) ([]byte, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes(), writer.FormDataContentType()
}

func postUpload(client *http.Client, baseURL, contentType string, body io.Reader, size int64, expectContinue bool) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/upload", body)
	if err != nil {
		return nil, err
	}
	req.ContentLength = size
	req.Header.Set("Content-Type", contentType)
	if expectContinue {
		req.Header.Set("Expect", "100-continue")
	}
	return client.Do(req)
}

type countingReader struct {
	r io.Reader
	n atomic.Int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.n.Add(int64(n))
	return n, err
}

func entryNames(entries []os.DirEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, filepath.Base(entry.Name()))
	}
	return names
}
