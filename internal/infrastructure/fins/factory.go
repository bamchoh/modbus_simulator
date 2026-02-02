package fins

import (
	"fmt"

	"modbus_simulator/internal/domain/protocol"
)

// FINSServerFactory はFINSサーバーのファクトリー
type FINSServerFactory struct{}

// NewFINSServerFactory は新しいFINSServerFactoryを作成する
func NewFINSServerFactory() *FINSServerFactory {
	return &FINSServerFactory{}
}

// ProtocolType はファクトリーが作成するプロトコルの種類を返す
func (f *FINSServerFactory) ProtocolType() protocol.ProtocolType {
	return protocol.ProtocolFINS
}

// DisplayName はプロトコルの表示名を返す
func (f *FINSServerFactory) DisplayName() string {
	return "OMRON FINS"
}

// CreateServer はサーバーを作成する
func (f *FINSServerFactory) CreateServer(config protocol.ProtocolConfig, store protocol.DataStore) (protocol.ProtocolServer, error) {
	finsConfig, ok := config.(*FINSConfig)
	if !ok {
		return nil, fmt.Errorf("invalid config type: expected FINSConfig")
	}

	finsStore, ok := store.(*FINSDataStore)
	if !ok {
		return nil, fmt.Errorf("invalid store type: expected FINSDataStore")
	}

	switch finsConfig.Variant() {
	case "udp":
		return NewFINSUDPServer(finsConfig, finsStore), nil
	default:
		return NewFINSServer(finsConfig, finsStore), nil
	}
}

// CreateDataStore はプロトコル用のデータストアを作成する
func (f *FINSServerFactory) CreateDataStore() protocol.DataStore {
	return NewFINSDataStore()
}

// DefaultConfig はデフォルト設定を返す
func (f *FINSServerFactory) DefaultConfig() protocol.ProtocolConfig {
	return DefaultFINSConfig()
}

// ConfigVariants は利用可能な設定バリアントを返す
func (f *FINSServerFactory) ConfigVariants() []protocol.ConfigVariant {
	return []protocol.ConfigVariant{
		{ID: "tcp", DisplayName: "FINS/TCP"},
		{ID: "udp", DisplayName: "FINS/UDP"},
	}
}

// CreateConfigFromVariant は指定バリアントの設定を作成する
func (f *FINSServerFactory) CreateConfigFromVariant(variantID string) protocol.ProtocolConfig {
	switch variantID {
	case "tcp":
		return DefaultFINSConfig()
	case "udp":
		return DefaultFINSUDPConfig()
	default:
		return DefaultFINSConfig()
	}
}

// GetConfigFields は指定バリアントの設定フィールドを返す
func (f *FINSServerFactory) GetConfigFields(variantID string) []protocol.ConfigField {
	return []protocol.ConfigField{
		{Name: "address", Label: "アドレス", Type: "text", Required: true, Default: "0.0.0.0"},
		{Name: "port", Label: "ポート", Type: "number", Required: true, Default: 9600, Min: intPtr(1), Max: intPtr(65535)},
		{Name: "nodeAddress", Label: "ノードアドレス", Type: "number", Required: true, Default: 1, Min: intPtr(0), Max: intPtr(255)},
		{Name: "networkId", Label: "ネットワークID", Type: "number", Required: true, Default: 0, Min: intPtr(0), Max: intPtr(255)},
	}
}

// GetProtocolCapabilities はプロトコルの機能情報を返す
func (f *FINSServerFactory) GetProtocolCapabilities() protocol.ProtocolCapabilities {
	return protocol.ProtocolCapabilities{
		SupportsUnitID: false,
	}
}

// ConfigToMap は設定をmapに変換する
func (f *FINSServerFactory) ConfigToMap(config protocol.ProtocolConfig) map[string]interface{} {
	fc, ok := config.(*FINSConfig)
	if !ok {
		return nil
	}
	return map[string]interface{}{
		"variant":     fc.Variant(),
		"address":     fc.Address,
		"port":        fc.Port,
		"nodeAddress": int(fc.NodeAddress),
		"networkId":   int(fc.NetworkID),
	}
}

// MapToConfig はmapから設定を作成する
func (f *FINSServerFactory) MapToConfig(variantID string, settings map[string]interface{}) (protocol.ProtocolConfig, error) {
	config := f.CreateConfigFromVariant(variantID).(*FINSConfig)

	if v, ok := settings["address"].(string); ok {
		config.Address = v
	}
	if v, ok := settings["port"].(float64); ok {
		config.Port = int(v)
	} else if v, ok := settings["port"].(int); ok {
		config.Port = v
	}
	if v, ok := settings["nodeAddress"].(float64); ok {
		config.NodeAddress = byte(v)
	} else if v, ok := settings["nodeAddress"].(int); ok {
		config.NodeAddress = byte(v)
	}
	if v, ok := settings["networkId"].(float64); ok {
		config.NetworkID = byte(v)
	} else if v, ok := settings["networkId"].(int); ok {
		config.NetworkID = byte(v)
	}

	return config, nil
}

func intPtr(i int) *int {
	return &i
}

// FINSConfig はFINSサーバーの設定
type FINSConfig struct {
	TransportType string `json:"variant"`
	Address       string `json:"address"`
	Port          int    `json:"port"`
	NodeAddress   byte   `json:"nodeAddress"`
	NetworkID     byte   `json:"networkId"`
}

// ProtocolType はプロトコルの種類を返す
func (c *FINSConfig) ProtocolType() protocol.ProtocolType {
	return protocol.ProtocolFINS
}

// Variant はバリアント名を返す
func (c *FINSConfig) Variant() string {
	if c.TransportType != "" {
		return c.TransportType
	}
	return "tcp"
}

// Validate は設定を検証する
func (c *FINSConfig) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}
	return nil
}

// Clone は設定のコピーを作成する
func (c *FINSConfig) Clone() protocol.ProtocolConfig {
	return &FINSConfig{
		TransportType: c.TransportType,
		Address:       c.Address,
		Port:          c.Port,
		NodeAddress:   c.NodeAddress,
		NetworkID:     c.NetworkID,
	}
}

// DefaultFINSConfig はデフォルトのFINS/TCP設定を返す
func DefaultFINSConfig() *FINSConfig {
	return &FINSConfig{
		TransportType: "tcp",
		Address:       "0.0.0.0",
		Port:          9600,
		NodeAddress:   1,
		NetworkID:     0,
	}
}

// DefaultFINSUDPConfig はデフォルトのFINS/UDP設定を返す
func DefaultFINSUDPConfig() *FINSConfig {
	return &FINSConfig{
		TransportType: "udp",
		Address:       "0.0.0.0",
		Port:          9600,
		NodeAddress:   1,
		NetworkID:     0,
	}
}

// protocol.DataStoreインターフェースを満たすことを確認
var _ protocol.DataStore = (*FINSDataStore)(nil)

func init() {
	// FINSファクトリーをデフォルトレジストリに登録
	protocol.Register(NewFINSServerFactory())
}
