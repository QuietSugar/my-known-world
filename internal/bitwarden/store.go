package bitwarden

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

// Store 管理解密后的 Bitwarden / Vaultwarden 条目与内存多维索引
type Store struct {
	config Config
	client *Client

	mu       sync.RWMutex
	items    map[string]Item // by UUID
	byName   map[string]Item // by normalized name/path
	lastSync time.Time
}

// NewStore 构造 Store
func NewStore(cfg Config) *Store {
	return &Store{
		config: cfg,
		client: NewClient(cfg),
		items:  make(map[string]Item),
		byName: make(map[string]Item),
	}
}

// Sync 从远端拉取并解密条目，刷新内存索引
func (s *Store) Sync(ctx context.Context) error {
	if !s.config.Enabled() {
		return errors.New("bitwarden provider is not configured")
	}
	items, _, err := s.client.Fetch(s.config.MasterPassword)
	if err != nil {
		return err
	}

	byId := make(map[string]Item, len(items))
	byName := make(map[string]Item, len(items)*2)

	for _, it := range items {
		byId[it.ID] = it
		normName := normalizeKey(it.Name)
		byName[normName] = it
		if slash := strings.LastIndex(it.Name, "/"); slash >= 0 {
			short := normalizeKey(it.Name[slash+1:])
			if _, exists := byName[short]; !exists {
				byName[short] = it
			}
		}
	}

	s.mu.Lock()
	s.items = byId
	s.byName = byName
	s.lastSync = time.Now()
	s.mu.Unlock()

	return nil
}

// SetItems 直接注入条目（用于预热缓存或离线测试）
func (s *Store) SetItems(items []Item) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = make(map[string]Item, len(items))
	s.byName = make(map[string]Item, len(items)*2)
	for _, it := range items {
		s.items[it.ID] = it
		norm := normalizeKey(it.Name)
		s.byName[norm] = it
		if slash := strings.LastIndex(it.Name, "/"); slash >= 0 {
			short := normalizeKey(it.Name[slash+1:])
			if _, exists := s.byName[short]; !exists {
				s.byName[short] = it
			}
		}
	}
	s.lastSync = time.Now()
}

// Lookup 按 ID、路径或名称查找解密条目
func (s *Store) Lookup(ref string) (Item, bool) {
	cleanRef := strings.TrimSpace(ref)
	cleanRef = strings.TrimPrefix(cleanRef, "vaultwarden:")
	cleanRef = strings.TrimPrefix(cleanRef, "bitwarden:")

	s.mu.RLock()
	defer s.mu.RUnlock()

	if it, ok := s.items[cleanRef]; ok {
		return it, true
	}
	norm := normalizeKey(cleanRef)
	if it, ok := s.byName[norm]; ok {
		return it, true
	}
	if slash := strings.LastIndex(cleanRef, "/"); slash >= 0 {
		short := normalizeKey(cleanRef[slash+1:])
		if it, ok := s.byName[short]; ok {
			return it, true
		}
	}
	return Item{}, false
}

// Items 返回当前所有条目的快照
func (s *Store) Items() []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]Item, 0, len(s.items))
	for _, it := range s.items {
		res = append(res, it)
	}
	return res
}

func normalizeKey(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = strings.TrimPrefix(s, "vaultwarden:")
	s = strings.TrimPrefix(s, "bitwarden:")
	return s
}
