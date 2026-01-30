package application

// === プロトコルスキーマDTO ===

// ProtocolSchemaDTO はプロトコル設定スキーマ
type ProtocolSchemaDTO struct {
	ProtocolType string          `json:"protocolType"`
	DisplayName  string          `json:"displayName"`
	Variants     []VariantDTO    `json:"variants"`
	Capabilities CapabilitiesDTO `json:"capabilities"`
}

// VariantDTO はバリアント情報
type VariantDTO struct {
	ID          string     `json:"id"`
	DisplayName string     `json:"displayName"`
	Fields      []FieldDTO `json:"fields"`
}

// FieldDTO は設定フィールド情報
type FieldDTO struct {
	Name      string        `json:"name"`
	Label     string        `json:"label"`
	Type      string        `json:"type"`
	Required  bool          `json:"required"`
	Default   interface{}   `json:"default"`
	Options   []OptionDTO   `json:"options,omitempty"`
	Min       *int          `json:"min,omitempty"`
	Max       *int          `json:"max,omitempty"`
	ShowWhen  *ConditionDTO `json:"showWhen,omitempty"`
}

// OptionDTO はセレクトフィールドのオプション
type OptionDTO struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// ConditionDTO はフィールドの表示条件
type ConditionDTO struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

// CapabilitiesDTO はプロトコルの機能情報
type CapabilitiesDTO struct {
	SupportsUnitID bool `json:"supportsUnitId"`
	UnitIDMin      int  `json:"unitIdMin,omitempty"`
	UnitIDMax      int  `json:"unitIdMax,omitempty"`
}

// === 汎用プロトコル設定DTO ===

// ProtocolConfigDTO は汎用プロトコル設定のDTO
type ProtocolConfigDTO struct {
	ProtocolType string                 `json:"protocolType"`
	Variant      string                 `json:"variant"`
	Settings     map[string]interface{} `json:"settings"`
}

// === プロトコル情報DTO ===

// ProtocolInfoDTO はプロトコル情報のDTO
type ProtocolInfoDTO struct {
	Type        string             `json:"type"`
	DisplayName string             `json:"displayName"`
	Variants    []ConfigVariantDTO `json:"variants"`
}

// ConfigVariantDTO は設定バリアント情報のDTO
type ConfigVariantDTO struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

// === メモリ操作DTO ===

// MemoryAreaDTO はメモリエリア情報のDTO
type MemoryAreaDTO struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	IsBit       bool   `json:"isBit"`
	Size        int    `json:"size"`
	ReadOnly    bool   `json:"readOnly"`
}

// === UnitID設定DTO ===

// UnitIDSettingsDTO はUnitID設定のDTO
type UnitIDSettingsDTO struct {
	Min         int   `json:"min"`
	Max         int   `json:"max"`
	DisabledIDs []int `json:"disabledIds"`
}

// === スクリプトDTO ===

// ScriptDTO はスクリプトのDTO
type ScriptDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Code       string `json:"code"`
	IntervalMs int    `json:"intervalMs"`
	IsRunning  bool   `json:"isRunning"`
}

// IntervalPresetDTO は周期プリセットのDTO
type IntervalPresetDTO struct {
	Label string `json:"label"`
	Ms    int    `json:"ms"`
}

// === モニタリングDTO ===

// MonitoringItemDTO はモニタリング項目のDTO
type MonitoringItemDTO struct {
	ID            string `json:"id"`
	Order         int    `json:"order"`
	MemoryArea    string `json:"memoryArea"`
	Address       int    `json:"address"`
	BitWidth      int    `json:"bitWidth"`
	Endianness    string `json:"endianness"`
	DisplayFormat string `json:"displayFormat"`
}

// MonitoringConfigDTO はモニタリング設定全体のDTO
type MonitoringConfigDTO struct {
	Version int                  `json:"version"`
	Items   []*MonitoringItemDTO `json:"items"`
}

// === プロジェクトエクスポート/インポートDTO ===

// ProjectDataDTO はプロジェクト全体のエクスポート/インポート用DTO
type ProjectDataDTO struct {
	Version         int                    `json:"version"`
	ProtocolType    string                 `json:"protocolType"`
	Variant         string                 `json:"variant"`
	Settings        map[string]interface{} `json:"settings"`
	MemorySnapshot  map[string]interface{} `json:"memorySnapshot"`
	UnitIDSettings  *UnitIDSettingsDTO     `json:"unitIdSettings,omitempty"`
	Scripts         []*ScriptDTO           `json:"scripts"`
	MonitoringItems []*MonitoringItemDTO   `json:"monitoringItems,omitempty"`
}
