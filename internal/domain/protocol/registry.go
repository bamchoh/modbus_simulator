package protocol

import (
	"fmt"
	"sync"
)

// Registry はプロトコルファクトリーを管理するレジストリ
type Registry struct {
	mu        sync.RWMutex
	factories map[ProtocolType]ServerFactory
}

// NewRegistry は新しいレジストリを作成する
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[ProtocolType]ServerFactory),
	}
}

// Register はファクトリーを登録する
func (r *Registry) Register(factory ServerFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	pt := factory.ProtocolType()
	if _, exists := r.factories[pt]; exists {
		return fmt.Errorf("protocol already registered: %s", pt)
	}

	r.factories[pt] = factory
	return nil
}

// Get は指定したプロトコルのファクトリーを取得する
func (r *Registry) Get(protocolType ProtocolType) (ServerFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, ok := r.factories[protocolType]
	if !ok {
		return nil, fmt.Errorf("protocol not found: %s", protocolType)
	}

	return factory, nil
}

// GetAll は登録されている全てのファクトリーを返す
func (r *Registry) GetAll() []ServerFactory {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factories := make([]ServerFactory, 0, len(r.factories))
	for _, f := range r.factories {
		factories = append(factories, f)
	}
	return factories
}

// ProtocolTypes は登録されている全てのプロトコルタイプを返す
func (r *Registry) ProtocolTypes() []ProtocolType {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]ProtocolType, 0, len(r.factories))
	for pt := range r.factories {
		types = append(types, pt)
	}
	return types
}

// DefaultRegistry はデフォルトのグローバルレジストリ
var DefaultRegistry = NewRegistry()

// Register はデフォルトレジストリにファクトリーを登録する
func Register(factory ServerFactory) error {
	return DefaultRegistry.Register(factory)
}

// Get はデフォルトレジストリからファクトリーを取得する
func Get(protocolType ProtocolType) (ServerFactory, error) {
	return DefaultRegistry.Get(protocolType)
}

// GetAll はデフォルトレジストリの全ファクトリーを返す
func GetAll() []ServerFactory {
	return DefaultRegistry.GetAll()
}
