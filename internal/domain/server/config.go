package server

// ServerType はModbusサーバーの種類
type ServerType int

const (
	ModbusTCP ServerType = iota
	ModbusRTU
	ModbusRTUASCII
)

func (t ServerType) String() string {
	switch t {
	case ModbusTCP:
		return "Modbus TCP"
	case ModbusRTU:
		return "Modbus RTU"
	case ModbusRTUASCII:
		return "Modbus RTU ASCII"
	default:
		return "Unknown"
	}
}

// ServerConfig はModbusサーバーの設定
type ServerConfig struct {
	Type     ServerType
	SlaveID  uint8

	// TCP設定
	TCPAddress string
	TCPPort    int

	// RTU設定
	SerialPort   string
	BaudRate     int
	DataBits     int
	StopBits     int
	Parity       string // "N", "E", "O"
}

// DefaultTCPConfig はデフォルトのTCP設定を返す
func DefaultTCPConfig() *ServerConfig {
	return &ServerConfig{
		Type:       ModbusTCP,
		SlaveID:    1,
		TCPAddress: "0.0.0.0",
		TCPPort:    502,
	}
}

// DefaultRTUConfig はデフォルトのRTU設定を返す
func DefaultRTUConfig() *ServerConfig {
	return &ServerConfig{
		Type:       ModbusRTU,
		SlaveID:    1,
		SerialPort: "COM1",
		BaudRate:   9600,
		DataBits:   8,
		StopBits:   1,
		Parity:     "N",
	}
}

// ServerStatus はサーバーの状態
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
