package cmdb

import (
	"context"
	"os"
	"testing"
)

func TestCompositeSecretProviderWithMockUpstream(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mock_upstream_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	fileUpstream, err := NewFileUpstreamVault(tmpDir, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Test upload to mock upstream
	ctx := context.Background()
	uuid1, err := fileUpstream.Upload(ctx, UpstreamCipherItem{
		Ref:      "credentials/xasc/test-1",
		Name:     "test-1",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("file upstream upload failed: %v", err)
	}
	if uuid1 == "" {
		t.Fatal("expected non-empty uuid from mock upstream")
	}

	// Verify reading from mock upstream using returned UUID
	res, err := fileUpstream.Read(ctx, uuid1)
	if err != nil {
		t.Fatalf("file upstream read failed: %v", err)
	}
	if string(res.Value) != "password123" {
		t.Fatalf("expected password123, got %s", string(res.Value))
	}
}
