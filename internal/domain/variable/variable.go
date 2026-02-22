package variable

// Variable は中央変数ストアの変数エンティティ
type Variable struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	DataType DataType    `json:"dataType"`
	Value    interface{} `json:"value"`
}

// ProtocolMapping は変数の1プロトコルへのアドレスマッピング
type ProtocolMapping struct {
	ProtocolType string `json:"protocolType"` // "modbus", "fins" 等
	MemoryArea   string `json:"memoryArea"`   // "holdingRegisters", "DM" 等
	Address      uint32 `json:"address"`
	Endianness   string `json:"endianness"` // "big" / "little"
}

// Clone は変数のコピーを作成する
func (v *Variable) Clone() *Variable {
	return &Variable{
		ID:       v.ID,
		Name:     v.Name,
		DataType: v.DataType,
		Value:    v.Value,
	}
}
