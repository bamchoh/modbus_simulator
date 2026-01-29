package datastore

// MemoryArea はメモリエリアの定義
type MemoryArea struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	IsBit       bool   `json:"isBit"`
	Size        uint32 `json:"size"`
	ReadOnly    bool   `json:"readOnly"`
}

// DataStore はデータストアの共通インターフェース
type DataStore interface {
	// GetAreas は利用可能なメモリエリアの一覧を返す
	GetAreas() []MemoryArea

	// ビット操作
	ReadBit(area string, address uint32) (bool, error)
	WriteBit(area string, address uint32, value bool) error
	ReadBits(area string, address uint32, count uint16) ([]bool, error)
	WriteBits(area string, address uint32, values []bool) error

	// ワード操作（16ビット）
	ReadWord(area string, address uint32) (uint16, error)
	WriteWord(area string, address uint32, value uint16) error
	ReadWords(area string, address uint32, count uint16) ([]uint16, error)
	WriteWords(area string, address uint32, values []uint16) error

	// スナップショット/復元
	Snapshot() map[string]interface{}
	Restore(data map[string]interface{}) error

	// クリア
	ClearAll()
}

// AreaInfo はメモリエリアの情報を取得するヘルパー
type AreaInfo interface {
	// GetAreaByID は指定IDのエリアを取得する
	GetAreaByID(id string) (*MemoryArea, bool)
	// GetBitAreas はビットエリアのみ取得する
	GetBitAreas() []MemoryArea
	// GetWordAreas はワードエリアのみ取得する
	GetWordAreas() []MemoryArea
}
