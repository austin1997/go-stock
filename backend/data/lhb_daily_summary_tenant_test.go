package data

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"go-stock/backend/models"
	"go-stock/backend/tenant"

	"github.com/go-resty/resty/v2"
)

const lhbSummaryFixtureBranch = "隔离测试证券测试路证券营业部"

type lhbSummaryFixtureTransport struct {
	mu       sync.Mutex
	requests map[uint]int
	before   func(*http.Request)
	remote   string
}

func (f *lhbSummaryFixtureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	f.requests[tenant.UserID()]++
	f.mu.Unlock()
	if f.before != nil {
		f.before(req)
	}
	var body string
	switch req.URL.Query().Get("reportName") {
	case "RPT_DAILYBILLBOARD_DETAILSNEW":
		body = `{"result":{"data":[{"SECURITY_CODE":"600001","SECURITY_NAME_ABBR":"测试股票","CHANGE_RATE":5,"CLOSE_PRICE":10}]}}`
	case "RPT_BILLBOARD_DAILYDETAILSBUY", "RPT_BILLBOARD_DAILYDETAILSSELL":
		body = fmt.Sprintf(`{"result":{"data":[{"OPERATEDEPT_NAME":%q,"BUY":120,"SELL":30,"NET":90}]}}`, lhbSummaryFixtureBranch)
	default:
		if req.URL.String() != "https://seats.fixture/seats.json" || f.remote == "" {
			return nil, fmt.Errorf("unexpected fixture request (network disabled): %s", req.URL)
		}
		body = f.remote
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

func (f *lhbSummaryFixtureTransport) count(id uint) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests[id]
}

// All tests are serial: isolate the shared client and in-memory caches, never
// allow a real HTTP request or lazy loading of a real user's seat file.
func setupLhbSummaryFixture(t *testing.T) *lhbSummaryFixtureTransport {
	t.Helper()
	oldClient := SharedHTTPClient
	fixture := &lhbSummaryFixtureTransport{requests: map[uint]int{}}
	SharedHTTPClient = resty.NewWithClient(&http.Client{Transport: fixture}).SetRetryCount(0)
	hotMoneySeatsMu.Lock()
	oldIndex, oldLoaded := hotMoneySeatIndexByTenant, hotMoneySeatsLoaded
	hotMoneySeatIndexByTenant = map[uint][]hotMoneyIndexEntry{}
	hotMoneySeatsLoaded = map[uint]bool{}
	hotMoneySeatsMu.Unlock()
	lhbDailySummaryMu.Lock()
	oldCache := maps.Clone(lhbDailySummaryCache)
	oldGeneration := lhbDailySummaryGeneration
	lhbDailySummaryGeneration = map[uint]uint64{}
	clear(lhbDailySummaryCache)
	lhbDailySummaryMu.Unlock()
	t.Cleanup(func() {
		SharedHTTPClient = oldClient
		hotMoneySeatsMu.Lock()
		hotMoneySeatIndexByTenant, hotMoneySeatsLoaded = oldIndex, oldLoaded
		hotMoneySeatsMu.Unlock()
		lhbDailySummaryMu.Lock()
		lhbDailySummaryGeneration = oldGeneration
		clear(lhbDailySummaryCache)
		for key, value := range oldCache {
			lhbDailySummaryCache[key] = value
		}
		lhbDailySummaryMu.Unlock()
	})
	setLhbSummaryFixtureSeats(nil, "桌面私有游资")
	return fixture
}

func withLhbSummaryTenant(rt *tenant.Runtime, fn func()) {
	if rt != nil {
		tenant.Bind(rt)
		defer tenant.Unbind()
	}
	fn()
}

func setLhbSummaryFixtureSeats(rt *tenant.Runtime, name string) {
	withLhbSummaryTenant(rt, func() {
		storeHotMoneySeatIndex([]hotMoneyIndexEntry{{
			branch: lhbSummaryFixtureBranch, category: "游资", name: name,
			tier: name + "梯队", style: name + "风格", risk: name + "风险",
		}})
	})
}

func getLhbSummaryForTenant(rt *tenant.Runtime, date string) (summary *models.LhbDailySummary) {
	withLhbSummaryTenant(rt, func() { summary = NewLhbSeatApi().GetLhbDailySummary(date) })
	return summary
}

func assertLhbSummaryProfile(t *testing.T, summary *models.LhbDailySummary, name string) {
	t.Helper()
	if summary == nil || summary.StockCount != 1 || len(summary.HotMoneyActivities) != 1 {
		t.Fatalf("want one stock and one activity for %q, got %+v", name, summary)
	}
	act := summary.HotMoneyActivities[0]
	if act.HotMoneyName != name || act.Tier != name+"梯队" || act.Style != name+"风格" || act.RiskLevel != name+"风险" {
		t.Fatalf("tenant classification leaked: want profile %q, got %+v", name, act)
	}
	if act.TotalBuy != 120 || act.TotalSell != 30 || len(act.Stocks) != 1 || act.Stocks[0].Net != 90 {
		t.Fatalf("unexpected aggregation: %+v", act)
	}
}

func TestLhbDailySummaryTenantCacheIsolation(t *testing.T) {
	fixture := setupLhbSummaryFixture(t)
	tenants := []struct {
		rt   *tenant.Runtime
		name string
	}{
		{&tenant.Runtime{UserID: 402, Root: t.TempDir()}, "甲私有游资"},
		{&tenant.Runtime{UserID: 403, Root: t.TempDir()}, "乙私有游资"},
		{nil, "桌面私有游资"},
	}
	for _, tc := range tenants {
		setLhbSummaryFixtureSeats(tc.rt, tc.name)
	}
	for _, date := range []string{"2026-09-02", "2026-09-03"} {
		for _, tc := range tenants {
			summary := getLhbSummaryForTenant(tc.rt, date)
			assertLhbSummaryProfile(t, summary, tc.name)
			if summary.Date != date {
				t.Fatalf("want date %s, got %s", date, summary.Date)
			}
			if cached := getLhbSummaryForTenant(tc.rt, date); cached != summary {
				t.Fatal("same tenant and date should reuse the cached summary")
			}
		}
	}
	for _, id := range []uint{402, 403, 0} {
		if got := fixture.count(id); got != 6 {
			t.Fatalf("tenant %d: got %d fixture requests, want 6 (two dates, cache hits fetch nothing)", id, got)
		}
	}
}

func TestLhbDailySummaryTenantReloadInvalidatesOnlyOwner(t *testing.T) {
	for _, operation := range []string{"save", "reset", "refresh"} {
		t.Run(operation, func(t *testing.T) {
			fixture := setupLhbSummaryFixture(t)
			a := &tenant.Runtime{UserID: 404, Root: t.TempDir()}
			b := &tenant.Runtime{UserID: 405, Root: t.TempDir()}
			setLhbSummaryFixtureSeats(a, "甲旧游资")
			setLhbSummaryFixtureSeats(b, "乙私有游资")
			dates := []string{"2026-09-04", "2026-09-05"}
			for _, date := range dates {
				assertLhbSummaryProfile(t, getLhbSummaryForTenant(a, date), "甲旧游资")
			}
			other := getLhbSummaryForTenant(b, dates[0])
			desktop := getLhbSummaryForTenant(nil, dates[0])
			updated := &HotMoneySeatFile{HotMoneyList: []HotMoneySeat{{
				Name: "甲新游资", Tier: "甲新游资梯队", Style: "甲新游资风格", Risk: "甲新游资风险",
				Seats: []HotMoneySeatBranch{{Branch: lhbSummaryFixtureBranch}},
			}}}
			raw, err := json.Marshal(updated)
			if err != nil {
				t.Fatal(err)
			}
			fixture.remote = string(raw)
			withLhbSummaryTenant(a, func() {
				var err error
				switch operation {
				case "save":
					err = SaveHotMoneySeats(updated)
				case "reset":
					err = ResetHotMoneySeats()
				case "refresh":
					err = RefreshHotMoneySeats("https://seats.fixture/seats.json")
				}
				if err != nil {
					t.Fatal(err)
				}
			})
			for _, date := range dates {
				summary := getLhbSummaryForTenant(a, date)
				if operation == "reset" {
					if summary.StockCount != 1 || len(summary.HotMoneyActivities) != 0 {
						t.Fatalf("reset must discard private classification on every cached date: %+v", summary)
					}
				} else {
					assertLhbSummaryProfile(t, summary, "甲新游资")
				}
			}
			if got := getLhbSummaryForTenant(b, dates[0]); got != other || fixture.count(b.UserID) != 3 {
				t.Fatal("another tenant's cached summary was invalidated")
			}
			if got := getLhbSummaryForTenant(nil, dates[0]); got != desktop || fixture.count(0) != 3 {
				t.Fatal("desktop cached summary was invalidated by a tenant update")
			}
		})
	}
}

func TestLhbDailySummaryTenantReloadRejectsInflightResult(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(fmt.Sprintf("new_result_cached=%t", replace), func(t *testing.T) {
			fixture := setupLhbSummaryFixture(t)
			rt := &tenant.Runtime{UserID: 406, Root: t.TempDir()}
			setLhbSummaryFixtureSeats(rt, "旧游资")
			blocked, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			var sells atomic.Int32
			fixture.before = func(req *http.Request) {
				if req.URL.Query().Get("reportName") == "RPT_BILLBOARD_DAILYDETAILSSELL" && sells.Add(1) == 1 {
					// The BUY response has already been classified using the old index.
					close(blocked)
					<-release
				}
			}
			finished := make(chan *models.LhbDailySummary, 1)
			go func() { finished <- getLhbSummaryForTenant(rt, "2026-09-06") }()
			select {
			case <-blocked:
			case <-time.After(5 * time.Second):
				t.Fatal("old summary did not reach the classification barrier")
			}
			setLhbSummaryFixtureSeats(rt, "新游资")
			var fresh *models.LhbDailySummary
			if replace {
				fresh = getLhbSummaryForTenant(rt, "2026-09-06")
				assertLhbSummaryProfile(t, fresh, "新游资")
			}
			unblock()
			select {
			case old := <-finished:
				if len(old.HotMoneyActivities) != 2 {
					t.Fatalf("fixture must overlap old BUY and new SELL classifications: %+v", old)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("old summary did not finish")
			}
			got := getLhbSummaryForTenant(rt, "2026-09-06")
			assertLhbSummaryProfile(t, got, "新游资")
			if replace && got != fresh {
				t.Fatal("stale in-flight result replaced the fresh cached summary")
			}
		})
	}
}

func TestLhbDailySummaryTenantOldExpiryCannotEvictReplacement(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixture := setupLhbSummaryFixture(t)
		defer func() {
			synctest.Wait()
			time.Sleep(10 * time.Minute)
			synctest.Wait() // Drain expiry goroutines before the bubble exits.
		}()
		rt := &tenant.Runtime{UserID: 407, Root: t.TempDir()}
		setLhbSummaryFixtureSeats(rt, "旧游资")
		getLhbSummaryForTenant(rt, "2026-09-07")
		synctest.Wait() // The old expiry goroutine is sleeping before advancing time.
		time.Sleep(5 * time.Minute)
		setLhbSummaryFixtureSeats(rt, "新游资")
		fresh := getLhbSummaryForTenant(rt, "2026-09-07")
		assertLhbSummaryProfile(t, fresh, "新游资")
		synctest.Wait()
		time.Sleep(5 * time.Minute)
		synctest.Wait() // Old timer fires; the replacement is only five minutes old.
		if got := getLhbSummaryForTenant(rt, "2026-09-07"); got != fresh || fixture.count(rt.UserID) != 6 {
			t.Fatal("old expiry evicted the replacement before its own ten-minute TTL")
		}
		time.Sleep(5 * time.Minute)
		synctest.Wait()
		if got := getLhbSummaryForTenant(rt, "2026-09-07"); got == fresh || fixture.count(rt.UserID) != 9 {
			t.Fatal("replacement did not expire after its own ten-minute TTL")
		}
	})
}

func TestLhbDailySummaryTenantHTTPTimeoutIsRequestScoped(t *testing.T) {
	for _, path := range []string{"fetchLhbSeatList", "RefreshHotMoneySeats", "fetchLhbBillboardStocks"} {
		t.Run(path, func(t *testing.T) {
			fixture := setupLhbSummaryFixture(t)
			rt := &tenant.Runtime{UserID: 408, Root: t.TempDir()}
			setLhbSummaryFixtureSeats(rt, "测试游资")
			fixture.remote = `{"hot_money_list":[{"name":"测试游资","seats":[{"branch":"测试营业部"}]}]}`
			client := SharedHTTPClient.GetClient()
			client.Timeout = time.Minute
			started := time.Now()
			var requestContext context.Context
			fixture.before = func(req *http.Request) {
				requestContext = req.Context()
				if got := client.Timeout; got != time.Minute {
					t.Errorf("shared HTTP timeout mutated during request: got %v, want 1m", got)
				}
				deadline, ok := requestContext.Deadline()
				if !ok || deadline.Before(started.Add(15*time.Second)) || deadline.After(time.Now().Add(15*time.Second)) {
					t.Errorf("request deadline must be 15s from request creation: got %v (present=%t)", deadline, ok)
				}
			}
			withLhbSummaryTenant(rt, func() {
				switch path {
				case "fetchLhbSeatList":
					seats, _ := fetchLhbSeatList("600001", "2026-09-08", "RPT_BILLBOARD_DAILYDETAILSBUY", "BUY")
					if len(seats) != 1 {
						t.Fatalf("want one fixture seat, got %d", len(seats))
					}
				case "RefreshHotMoneySeats":
					if err := RefreshHotMoneySeats("https://seats.fixture/seats.json"); err != nil {
						t.Fatal(err)
					}
				case "fetchLhbBillboardStocks":
					if stocks := fetchLhbBillboardStocks("2026-09-08"); len(stocks) != 1 {
						t.Fatalf("want one fixture stock, got %d", len(stocks))
					}
				}
			})
			if got := fixture.count(rt.UserID); got != 1 || requestContext == nil {
				t.Fatalf("want one request through the shared fixture, got %d", got)
			}
			if got := client.Timeout; got != time.Minute {
				t.Errorf("shared HTTP timeout mutated after request: got %v, want 1m", got)
			}
			if err := requestContext.Err(); err != context.Canceled {
				t.Errorf("request context not canceled on return: got %v", err)
			}
		})
	}
}

func TestLhbDailySummaryTenantWorkers(t *testing.T) {
	fixture := setupLhbSummaryFixture(t)
	rt := &tenant.Runtime{UserID: 401, Root: t.TempDir()}
	setLhbSummaryFixtureSeats(rt, "甲私有游资")
	summary := getLhbSummaryForTenant(rt, "2026-09-01")
	assertLhbSummaryProfile(t, summary, "甲私有游资")
	if got := fixture.count(rt.UserID); got != 3 {
		t.Fatalf("billboard and both seat workers must keep tenant binding: got %d requests, want 3", got)
	}
	if got := fixture.count(0); got != 0 {
		t.Fatalf("tenant workers made %d requests in desktop scope", got)
	}
}
