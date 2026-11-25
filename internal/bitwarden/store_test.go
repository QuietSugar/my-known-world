package bitwarden

import (
	"testing"
)

func TestStoreLookup(t *testing.T) {
	s := NewStore(Config{})
	s.SetItems([]Item{
		{
			ID:       "uuid-xasc-mysql",
			Name:     "xasc/test-biz-mysql",
			Username: "deployer",
			Password: "MySecretPassword123!",
		},
		{
			ID:       "uuid-bastion",
			Name:     "cred-bastion-prod",
			Username: "admin",
			Password: "BastionSecurePass888",
		},
	})

	// 1. 按 UUID 查
	item, ok := s.Lookup("uuid-xasc-mysql")
	if !ok || item.Password != "MySecretPassword123!" {
		t.Fatalf("lookup by UUID failed")
	}

	// 2. 按带 vaultwarden: 前缀的完整路径查
	item, ok = s.Lookup("vaultwarden:xasc/test-biz-mysql")
	if !ok || item.Password != "MySecretPassword123!" {
		t.Fatalf("lookup by vaultwarden:path failed")
	}

	// 3. 按短名称查
	item, ok = s.Lookup("test-biz-mysql")
	if !ok || item.Password != "MySecretPassword123!" {
		t.Fatalf("lookup by short name failed")
	}

	// 4. 不存在的凭据
	_, ok = s.Lookup("vaultwarden:non-existent")
	if ok {
		t.Fatalf("expected false for non-existent credential")
	}
}
