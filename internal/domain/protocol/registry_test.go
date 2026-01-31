package protocol

import (
	"context"
	"testing"
)

// mockServerFactory はテスト用のモックファクトリー
type mockServerFactory struct {
	protocolType ProtocolType
	displayName  string
}

func (f *mockServerFactory) ProtocolType() ProtocolType {
	return f.protocolType
}

func (f *mockServerFactory) DisplayName() string {
	return f.displayName
}

func (f *mockServerFactory) CreateServer(config ProtocolConfig, store DataStore) (ProtocolServer, error) {
	return nil, nil
}

func (f *mockServerFactory) CreateDataStore() DataStore {
	return nil
}

func (f *mockServerFactory) DefaultConfig() ProtocolConfig {
	return nil
}

func (f *mockServerFactory) ConfigVariants() []ConfigVariant {
	return nil
}

func (f *mockServerFactory) CreateConfigFromVariant(variantID string) ProtocolConfig {
	return nil
}

func (f *mockServerFactory) GetConfigFields(variantID string) []ConfigField {
	return nil
}

func (f *mockServerFactory) GetProtocolCapabilities() ProtocolCapabilities {
	return ProtocolCapabilities{}
}

func (f *mockServerFactory) ConfigToMap(config ProtocolConfig) map[string]interface{} {
	return nil
}

func (f *mockServerFactory) MapToConfig(variantID string, settings map[string]interface{}) (ProtocolConfig, error) {
	return nil, nil
}

// mockProtocolConfig はテスト用のモック設定
type mockProtocolConfig struct {
	protocolType ProtocolType
	variant      string
}

func (c *mockProtocolConfig) ProtocolType() ProtocolType {
	return c.protocolType
}

func (c *mockProtocolConfig) Variant() string {
	return c.variant
}

func (c *mockProtocolConfig) Validate() error {
	return nil
}

func (c *mockProtocolConfig) Clone() ProtocolConfig {
	return &mockProtocolConfig{
		protocolType: c.protocolType,
		variant:      c.variant,
	}
}

// mockProtocolServer はテスト用のモックサーバー
type mockProtocolServer struct {
	protocolType ProtocolType
	status       ServerStatus
	config       ProtocolConfig
}

func (s *mockProtocolServer) Start(ctx context.Context) error {
	s.status = StatusRunning
	return nil
}

func (s *mockProtocolServer) Stop() error {
	s.status = StatusStopped
	return nil
}

func (s *mockProtocolServer) Status() ServerStatus {
	return s.status
}

func (s *mockProtocolServer) ProtocolType() ProtocolType {
	return s.protocolType
}

func (s *mockProtocolServer) Config() ProtocolConfig {
	return s.config
}

func (s *mockProtocolServer) UpdateConfig(config ProtocolConfig) error {
	s.config = config
	return nil
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if r.factories == nil {
		t.Fatal("factories map is nil")
	}
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()
	factory := &mockServerFactory{
		protocolType: "test",
		displayName:  "Test Protocol",
	}

	// 正常な登録
	err := r.Register(factory)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// 重複登録はエラー
	err = r.Register(factory)
	if err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()
	factory := &mockServerFactory{
		protocolType: "test",
		displayName:  "Test Protocol",
	}

	_ = r.Register(factory)

	// 存在するプロトコルの取得
	got, err := r.Get("test")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.ProtocolType() != "test" {
		t.Errorf("expected protocol type 'test', got '%s'", got.ProtocolType())
	}

	// 存在しないプロトコルの取得
	_, err = r.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent protocol")
	}
}

func TestRegistry_GetAll(t *testing.T) {
	r := NewRegistry()

	// 空のレジストリ
	factories := r.GetAll()
	if len(factories) != 0 {
		t.Errorf("expected 0 factories, got %d", len(factories))
	}

	// ファクトリーを登録
	factory1 := &mockServerFactory{protocolType: "test1", displayName: "Test1"}
	factory2 := &mockServerFactory{protocolType: "test2", displayName: "Test2"}
	_ = r.Register(factory1)
	_ = r.Register(factory2)

	factories = r.GetAll()
	if len(factories) != 2 {
		t.Errorf("expected 2 factories, got %d", len(factories))
	}
}

func TestRegistry_ProtocolTypes(t *testing.T) {
	r := NewRegistry()

	// 空のレジストリ
	types := r.ProtocolTypes()
	if len(types) != 0 {
		t.Errorf("expected 0 types, got %d", len(types))
	}

	// ファクトリーを登録
	factory := &mockServerFactory{protocolType: "test", displayName: "Test"}
	_ = r.Register(factory)

	types = r.ProtocolTypes()
	if len(types) != 1 {
		t.Errorf("expected 1 type, got %d", len(types))
	}
	if types[0] != "test" {
		t.Errorf("expected 'test', got '%s'", types[0])
	}
}

func TestServerStatus_String(t *testing.T) {
	tests := []struct {
		status   ServerStatus
		expected string
	}{
		{StatusStopped, "Stopped"},
		{StatusRunning, "Running"},
		{StatusError, "Error"},
		{ServerStatus(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.status.String(); got != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, got)
			}
		})
	}
}
