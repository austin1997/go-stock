package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/philippgille/chromem-go"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/tenant"
)

func TestSearchAllKnowledgeTenantIsolation(t *testing.T) {
	// Keep both legacy and tenant stores populated: an unbound worker must not
	// silently return legacy documents with the same KB name.
	legacyRoot := t.TempDir()
	t.Setenv("GO_STOCK_ROOT_DIR", legacyRoot)

	settingsDB, err := db.Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := settingsDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := settingsDB.AutoMigrate(&data.Settings{}, &data.AIConfig{}); err != nil {
		t.Fatal(err)
	}
	if err := settingsDB.Create(&data.Settings{BrowserPath: "/unused"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := settingsDB.Create(&data.AIConfig{
		Name: "test embedding", ApiKey: "test", BaseUrl: "http://127.0.0.1:1",
		ModelType: "embedding", ModelName: "test-model",
	}).Error; err != nil {
		t.Fatal(err)
	}
	originalDB := db.Dao
	db.Dao = settingsDB
	t.Cleanup(func() { db.Dao = originalDB })

	kbMetaMu.Lock()
	originalMeta := kbMetaByKey
	kbMetaByKey = make(map[string]*kbMetaBucket)
	kbMetaMu.Unlock()
	longTermMemoryMu.Lock()
	originalStores := longTermMemoryByKey
	longTermMemoryByKey = make(map[string]*ltmHolder)
	longTermMemoryMu.Unlock()
	t.Cleanup(func() {
		kbMetaMu.Lock()
		kbMetaByKey = originalMeta
		kbMetaMu.Unlock()
		longTermMemoryMu.Lock()
		longTermMemoryByKey = originalStores
		longTermMemoryMu.Unlock()
	})

	scopes := []struct {
		name string
		root string
		uid  uint
	}{
		{name: "desktop", root: legacyRoot},
		{name: "tenant-1", root: t.TempDir(), uid: 101},
		{name: "tenant-2", root: t.TempDir(), uid: 202},
	}
	for _, scope := range scopes {
		vectorDB := chromem.NewDB()
		// Only embedding generation is substituted; real collections, cosine
		// search, metadata filters and the unified search workers are exercised.
		embed := func(context.Context, string) ([]float32, error) {
			return []float32{1, 0}, nil
		}
		ltm, err := vectorDB.CreateCollection(longTermMemoryCollectionName, nil, embed)
		if err != nil {
			t.Fatal(err)
		}
		userKey := CurrentUserKey("")
		if scope.uid != 0 {
			userKey = fmt.Sprintf("web:%d", scope.uid)
		}
		for _, doc := range []chromem.Document{
			{ID: "own-memory", Content: scope.name + " memory", Embedding: []float32{1, 0},
				Metadata: map[string]string{"user": userKey, "question": scope.name + " question"}},
			{ID: "other-memory", Content: "wrong user memory", Embedding: []float32{1, 0},
				Metadata: map[string]string{"user": "web:999", "question": "other user question"}},
		} {
			if err := ltm.AddDocument(context.Background(), doc); err != nil {
				t.Fatal(err)
			}
		}
		meta := &kbMetaBucket{data: make(map[string]*KnowledgeBaseInfo)}
		for _, name := range []string{"research", "reports"} {
			coll, err := vectorDB.CreateCollection(kbCollectionName(name), nil, embed)
			if err != nil {
				t.Fatal(err)
			}
			if err := coll.AddDocument(context.Background(), chromem.Document{
				ID: name, Content: scope.name + " " + name, Embedding: []float32{1, 0},
				Metadata: map[string]string{"source": name + ".txt"},
			}); err != nil {
				t.Fatal(err)
			}
			meta.data[name] = &KnowledgeBaseInfo{Name: name, DocumentCount: 1}
		}
		kbMetaByKey[scope.root] = meta
		longTermMemoryByKey[scope.root] = &ltmHolder{db: vectorDB, coll: ltm}
	}

	for _, scope := range scopes {
		t.Run(scope.name, func(t *testing.T) {
			if scope.uid != 0 {
				tenant.Bind(&tenant.Runtime{UserID: scope.uid, Root: scope.root, DB: settingsDB})
				defer tenant.Unbind()
			}
			hits, err := SearchAllKnowledge(context.Background(), "same query", 10)
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]string{
				UnifiedSourceKB + "/research": scope.name + " research",
				UnifiedSourceKB + "/reports":  scope.name + " reports",
				UnifiedSourceLTM + "/历史经验":    scope.name + " memory",
			}
			if len(hits) != len(want) {
				t.Errorf("got %d hits, want %d: %+v", len(hits), len(want), hits)
			}
			for _, hit := range hits {
				key := hit.SourceType + "/" + hit.KBName
				content, ok := want[key]
				if !ok || hit.Content != content {
					t.Errorf("%s content = %q, want %q (tenant %d)", key, hit.Content, content, scope.uid)
				}
				delete(want, key)
			}
			for key := range want {
				t.Errorf("missing result from %s", key)
			}
		})
	}
}
