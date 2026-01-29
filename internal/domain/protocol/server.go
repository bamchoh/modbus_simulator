package protocol

import (
	"context"
)

// ProtocolType はプロトコルの種類を表す
type ProtocolType string

const (
	ProtocolModbus ProtocolType = "modbus"
	ProtocolFINS   ProtocolType = "fins"
	// 将来追加予定:
	// ProtocolMC      ProtocolType = "mc"
	// ProtocolOPCUA   ProtocolType = "opcua"
)

// ServerStatus はサーバーの状態を表す
type ServerStatus int

const (
	StatusStopped ServerStatus = iota
	StatusRunning
	StatusError
)

func (s ServerStatus) String() string {
	switch s {
	case StatusStopped:
		return "Stopped"
	case StatusRunning:
		return "Running"
	case StatusError:
		return "Error"
	default:
		return "Unknown"
	}
}

// ProtocolConfig はプロトコル設定の共通インターフェース
type ProtocolConfig interface {
	// ProtocolType はプロトコルの種類を返す
	ProtocolType() ProtocolType
	// Variant はバリアント名を返す（例: "tcp", "rtu"）
	Variant() string
	// Validate は設定を検証する
	Validate() error
	// Clone は設定のコピーを作成する
	Clone() ProtocolConfig
}

// ProtocolServer はプロトコルサーバーの共通インターフェース
type ProtocolServer interface {
	// Start はサーバーを起動する
	Start(ctx context.Context) error
	// Stop はサーバーを停止する
	Stop() error
	// Status はサーバーの状態を返す
	Status() ServerStatus
	// ProtocolType はプロトコルの種類を返す
	ProtocolType() ProtocolType
	// Config は現在の設定を返す
	Config() ProtocolConfig
	// UpdateConfig は設定を更新する（停止中のみ）
	UpdateConfig(config ProtocolConfig) error
}

// ConfigVariant は設定バリアントの情報
type ConfigVariant struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

// ServerFactory はプロトコルサーバーを作成するファクトリーインターフェース
type ServerFactory interface {
	// ProtocolType はファクトリーが作成するプロトコルの種類を返す
	ProtocolType() ProtocolType
	// DisplayName はプロトコルの表示名を返す
	DisplayName() string
	// CreateServer はサーバーを作成する
	CreateServer(config ProtocolConfig, store DataStore) (ProtocolServer, error)
	// CreateDataStore はプロトコル用のデータストアを作成する
	CreateDataStore() DataStore
	// DefaultConfig はデフォルト設定を返す
	DefaultConfig() ProtocolConfig
	// ConfigVariants は利用可能な設定バリアントを返す
	ConfigVariants() []ConfigVariant
	// CreateConfigFromVariant は指定バリアントの設定を作成する
	CreateConfigFromVariant(variantID string) ProtocolConfig
	// GetConfigFields は指定バリアントの設定フィールドを返す
	GetConfigFields(variantID string) []ConfigField
	// GetProtocolCapabilities はプロトコルの機能情報を返す
	GetProtocolCapabilities() ProtocolCapabilities
	// ConfigToMap は設定をmapに変換する
	ConfigToMap(config ProtocolConfig) map[string]interface{}
	// MapToConfig はmapから設定を作成する
	MapToConfig(variantID string, settings map[string]interface{}) (ProtocolConfig, error)
}

// DataStore はプロトコルサーバーファクトリーで使用するインターフェース
// 実際の定義は datastore パッケージにある
type DataStore interface {
	GetAreas() []MemoryArea
	ReadBit(area string, address uint32) (bool, error)
	WriteBit(area string, address uint32, value bool) error
	ReadBits(area string, address uint32, count uint16) ([]bool, error)
	WriteBits(area string, address uint32, values []bool) error
	ReadWord(area string, address uint32) (uint16, error)
	WriteWord(area string, address uint32, value uint16) error
	ReadWords(area string, address uint32, count uint16) ([]uint16, error)
	WriteWords(area string, address uint32, values []uint16) error
	Snapshot() map[string]interface{}
	Restore(data map[string]interface{}) error
	ClearAll()
}

// MemoryArea はメモリエリアの定義
type MemoryArea struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	IsBit       bool   `json:"isBit"`
	Size        uint32 `json:"size"`
	ReadOnly    bool   `json:"readOnly"`
}

// ConfigField は設定フィールドの定義
type ConfigField struct {
	Name      string          `json:"name"`
	Label     string          `json:"label"`
	Type      string          `json:"type"` // "text", "number", "select"
	Required  bool            `json:"required"`
	Default   interface{}     `json:"default"`
	Options   []FieldOption   `json:"options,omitempty"`
	Min       *int            `json:"min,omitempty"`
	Max       *int            `json:"max,omitempty"`
	Condition *FieldCondition `json:"condition,omitempty"` // 表示条件
}

// FieldOption はセレクトフィールドのオプション
type FieldOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// FieldCondition はフィールドの表示条件
type FieldCondition struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

// ProtocolCapabilities はプロトコルの機能情報
type ProtocolCapabilities struct {
	SupportsUnitID bool `json:"supportsUnitId"`
	UnitIDMin      int  `json:"unitIdMin,omitempty"`
	UnitIDMax      int  `json:"unitIdMax,omitempty"`
}
