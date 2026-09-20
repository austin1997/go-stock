package agent

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/tenant"

	"github.com/go-resty/resty/v2"
	"gorm.io/gorm"
)

type reportCopyRoundTripper func(*http.Request) (*http.Response, error)

func (f reportCopyRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestReportCopiesStayInTenantDatabase(t *testing.T) {
	// These entry points have no injectable LLM. A missing AI configuration makes
	// ChatWithContext emit a deterministic, non-empty initialization-error message.
	// The report APIs currently treat that as report content, exercising their real
	// successful-save/copy paths without an LLM, production hooks, or live services.
	// Only the unrelated market-data HTTP boundary is replaced with an empty fixture.
	oldHTTP := data.SharedHTTPClient
	data.SharedHTTPClient = resty.NewWithClient(&http.Client{
		Transport: reportCopyRoundTripper(func(r *http.Request) (*http.Response, error) {
			if r.URL.Host != "datacenter-web.eastmoney.com" || r.URL.Query().Get("reportName") != "RPT_DAILYBILLBOARD_DETAILSNEW" {
				return nil, fmt.Errorf("unexpected report-test HTTP request: %s", r.URL)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"result":{"data":[]}}`)),
				Request:    r,
			}, nil
		}),
	})
	t.Cleanup(func() { data.SharedHTTPClient = oldHTTP })

	for _, shellHasTable := range []bool{false, true} {
		t.Run(fmt.Sprintf("shell_has_report_table=%t", shellHasTable), func(t *testing.T) {
			dir := t.TempDir()
			oldDAO := db.Dao
			db.InitTenantShell(filepath.Join(dir, ".web_shell.db"))
			shell := db.Dao
			shellSQL, err := shell.DB()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				db.Dao = oldDAO
				_ = shellSQL.Close()
			})
			if shellHasTable {
				if err := shell.AutoMigrate(&models.AIResponseResult{}); err != nil {
					t.Fatal(err)
				}
			}

			runtimes := make([]*tenant.Runtime, 0, 2)
			for _, name := range []string{"alice", "bob"} {
				tenantDB, err := db.Open(filepath.Join(dir, name+".db"))
				if err != nil {
					t.Fatal(err)
				}
				sqlDB, err := tenantDB.DB()
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = sqlDB.Close() })
				if err := tenantDB.AutoMigrate(
					&data.Settings{}, &data.AIConfig{}, &models.AIResponseResult{},
					&models.DailyReview{}, &models.MorningStrategy{}, &models.MarketStatistic{},
					&data.TradingRecord{}, &models.GlobalStockIndex{}, &data.FollowedStock{},
				); err != nil {
					t.Fatal(err)
				}
				if err := tenantDB.Create(&data.Settings{BrowserPath: "unused-test-browser"}).Error; err != nil {
					t.Fatal(err)
				}
				runtimes = append(runtimes, &tenant.Runtime{UserID: uint(len(runtimes) + 1), DB: tenantDB, Root: filepath.Join(dir, name)})
			}

			type copyResult struct {
				err error
				rt  *tenant.Runtime
			}
			copied := make(chan copyResult, 1)
			if err := shell.Callback().Create().After("gorm:create").Register("test:report_copy_complete", func(tx *gorm.DB) {
				if _, ok := tx.Statement.Dest.(*models.AIResponseResult); ok {
					copied <- copyResult{err: tx.Error, rt: tenant.Current()}
				}
			}); err != nil {
				t.Fatal(err)
			}

			for _, rt := range runtimes {
				for _, title := range []string{"每日复盘", "盘前策略"} {
					t.Run(fmt.Sprintf("user%d/%s", rt.UserID, title), func(t *testing.T) {
						const date = "2000-01-05" // Never refresh today's remote market snapshot.
						missingConfigID := 987654 + int(rt.UserID)
						var content string
						func() {
							tenant.Bind(rt)
							defer tenant.Unbind()
							// nil skips UI events; DB routing must inherit the goroutine binding.
							if title == "每日复盘" {
								report, err := NewDailyReviewApi().GenerateDailyReview(nil, date, missingConfigID, 0, false, "", "manual")
								if err != nil {
									t.Fatal(err)
								}
								content = report.Content
								var saved models.DailyReview
								if err := rt.DB.First(&saved, report.ID).Error; err != nil || saved.Content != content || saved.Status != "success" {
									t.Fatalf("primary daily review was not saved: %+v, err=%v", saved, err)
								}
							} else {
								report, err := NewMorningStrategyApi().GenerateMorningStrategy(nil, date, missingConfigID, 0, false, "", "manual")
								if err != nil {
									t.Fatal(err)
								}
								content = report.Content
								var saved models.MorningStrategy
								if err := rt.DB.First(&saved, report.ID).Error; err != nil || saved.Content != content || saved.Status != "success" {
									t.Fatalf("primary morning strategy was not saved: %+v, err=%v", saved, err)
								}
							}
						}()
						if content == "" || !strings.Contains(content, fmt.Sprint(missingConfigID)) {
							t.Fatalf("did not reach expected deterministic report-content path: %q", content)
						}

						select {
						case result := <-copied:
							if result.rt != rt || result.err != nil {
								t.Errorf("report copy lost tenant runtime: got=%v want_user=%d save_error=%v", result.rt, rt.UserID, result.err)
							}
						case <-time.After(5 * time.Second):
							t.Fatal("report copy did not finish")
						}

						var rows []models.AIResponseResult
						if err := rt.DB.Where("stock_code = ?", title).Find(&rows).Error; err != nil {
							t.Fatal(err)
						}
						if len(rows) != 1 {
							t.Fatalf("tenant %d has %d %s copies, want 1", rt.UserID, len(rows), title)
						}
						if row := rows[0]; row.StockName != title || row.Content != content || row.ChatId != "" || !strings.Contains(row.Question, date) {
							t.Errorf("copy changed report fields: %+v", row)
						}
					})
				}
			}

			// Reading the concrete databases avoids relying on the routing under test.
			for _, rt := range runtimes {
				var count int64
				if err := rt.DB.Model(&models.AIResponseResult{}).Count(&count).Error; err != nil || count != 2 {
					t.Errorf("tenant %d report count=%d, want 2; err=%v", rt.UserID, count, err)
				}
			}
			if shellHasTable {
				var count int64
				if err := shell.Model(&models.AIResponseResult{}).Count(&count).Error; err != nil || count != 0 {
					t.Errorf("shell received %d private reports, want 0; err=%v", count, err)
				}
			}
		})
	}
}
