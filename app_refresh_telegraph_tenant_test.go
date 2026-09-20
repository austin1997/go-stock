//go:build goweb

package main

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/tenant"

	"github.com/go-resty/resty/v2"
	"gorm.io/gorm"
)

type refreshTelegraphTransport struct {
	mu       sync.Mutex
	requests map[string]int
	clsDone  chan struct{}
	sinaDone chan struct{}
	clsOnce  sync.Once
	sinaOnce sync.Once
}

func (f *refreshTelegraphTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	f.requests[req.URL.Host]++
	f.mu.Unlock()

	var body string
	switch req.URL.Host {
	case "www.cls.cn":
		if req.URL.Query().Get("name") == "telegraph" {
			body = `{"errno":1}`
		} else {
			body = `{"errno":0,"data":{"roll_data":[{"ctime":1700000001,"title":"tenant-cls","content":"tenant cls content","level":"C"}]}}`
		}
	case "zhibo.sina.com.cn":
		<-f.clsDone
		body = `try{callback({"result":{"data":{"feed":{"list":[{"rich_text":"【tenant-sina】tenant sina content","create_time":"2024-01-02 03:04:05","tag":[]}]}}}});}catch(e){};`
	case "news-mediator.tradingview.com":
		<-f.sinaDone
		body = `{"items":[{"id":"tenant-tv-id","title":"tenant-tv","published":1700000002}]}`
	case "news-headlines.tradingview.com":
		body = `{"shortDescription":"tenant tv content"}`
	default:
		return nil, fmt.Errorf("unexpected request: %s", req.URL.String())
	}
	header := make(http.Header)
	header.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

type refreshTelegraphWrite struct {
	source string
	err    error
}

func TestReFleshTelegraphListWorkersKeepTenantDatabase(t *testing.T) {
	for _, shellHasTable := range []bool{false, true} {
		t.Run(fmt.Sprintf("shell_table=%v", shellHasTable), func(t *testing.T) {
			dir := t.TempDir()
			oldDB := db.Dao
			db.InitTenantShell(filepath.Join(dir, "shell.db"))
			t.Cleanup(func() {
				if sqlDB, err := db.Dao.DB(); err == nil {
					_ = sqlDB.Close()
				}
				db.Dao = oldDB
			})

			tenantDB, err := db.Open(filepath.Join(dir, "tenant.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if sqlDB, dbErr := tenantDB.DB(); dbErr == nil {
					_ = sqlDB.Close()
				}
			})
			if err := tenantDB.AutoMigrate(&models.Telegraph{}); err != nil {
				t.Fatal(err)
			}
			if shellHasTable {
				if err := db.Dao.AutoMigrate(&models.Telegraph{}); err != nil {
					t.Fatal(err)
				}
			}

			oldClient := data.SharedHTTPClient
			fixture := &refreshTelegraphTransport{
				requests: map[string]int{},
				clsDone:  make(chan struct{}),
				sinaDone: make(chan struct{}),
			}
			data.SharedHTTPClient = resty.NewWithClient(&http.Client{Transport: fixture}).SetRetryCount(0)
			t.Cleanup(func() { data.SharedHTTPClient = oldClient })

			writes := make(chan refreshTelegraphWrite, 3)
			callbackName := "test:refresh-telegraph-tenant"
			if err := db.Dao.Callback().Create().After("gorm:create").Register(callbackName, func(tx *gorm.DB) {
				if tx.Statement != nil && tx.Statement.Table == "telegraph_list" {
					source := ""
					switch value := tx.Statement.Dest.(type) {
					case *models.Telegraph:
						source = value.Source
					case **models.Telegraph:
						if value != nil && *value != nil {
							source = (*value).Source
						}
					}
					writes <- refreshTelegraphWrite{source: source, err: tx.Error}
					switch source {
					case "财联社电报":
						fixture.clsOnce.Do(func() { close(fixture.clsDone) })
					case "新浪财经":
						fixture.sinaOnce.Do(func() { close(fixture.sinaDone) })
					}
				}
			}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Dao.Callback().Create().Remove(callbackName) })

			rt := &tenant.Runtime{UserID: 77, DB: tenantDB, Root: filepath.Join(dir, "tenant")}
			tenant.Bind(rt)
			data.InitAnalyzeSentiment()
			_ = data.AnalyzeSentiment("tenant warmup")
			tenant.Unbind()
			returned := make(chan struct{})
			go func() {
				defer close(returned)
				tenant.Bind(rt)
				defer tenant.Unbind()
				_ = (&App{}).ReFleshTelegraphList("")
			}()
			select {
			case <-returned:
			case <-time.After(2 * time.Second):
				t.Fatal("ReFleshTelegraphList did not return asynchronously")
			}

			seen := make([]refreshTelegraphWrite, 0, 3)
			for len(seen) < 3 {
				select {
				case write := <-writes:
					seen = append(seen, write)
				case <-time.After(5 * time.Second):
					t.Fatalf("timed out waiting for worker writes; saw=%v", seen)
				}
			}

			for _, write := range seen {
				if write.err != nil {
					t.Fatalf("tenant write failed: %+v", write)
				}
			}
			var tenantRows []models.Telegraph
			if err := tenantDB.Order("source").Find(&tenantRows).Error; err != nil {
				t.Fatal(err)
			}
			if len(tenantRows) != 3 {
				t.Fatalf("tenant database got %d rows, want 3: %+v", len(tenantRows), tenantRows)
			}
			if shellHasTable {
				var shellCount int64
				if err := db.Dao.Model(&models.Telegraph{}).Count(&shellCount).Error; err != nil {
					t.Fatal(err)
				}
				if shellCount != 0 {
					t.Fatalf("shell database leaked %d tenant rows", shellCount)
				}
			}
		})
	}
}
