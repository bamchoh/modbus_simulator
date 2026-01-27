package application

// ServerConfigDTO はサーバー設定のDTO
type ServerConfigDTO struct {
	Type       int    `json:"type"`
	TypeName   string `json:"typeName"`
	TCPAddress string `json:"tcpAddress"`
	TCPPort    int    `json:"tcpPort"`
	SerialPort string `json:"serialPort"`
	BaudRate   int    `json:"baudRate"`
	DataBits   int    `json:"dataBits"`
	StopBits   int    `json:"stopBits"`
	Parity     string `json:"parity"`
}

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

// RegisterValueDTO はレジスタ値のDTO
type RegisterValueDTO struct {
	Address int  `json:"address"`
	Value   int  `json:"value"`
	Bool    bool `json:"bool,omitempty"`
}
