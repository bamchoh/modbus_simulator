package modbus

import (
	"context"
	"errors"
	"fmt"

	"modbus_simulator/internal/domain/protocol"
)

// ErrUnitIdDisabled はUnitIDが無効化されている場合のエラー
var ErrUnitIdDisabled = errors.New("unit ID is disabled")

// ModbusServerFactory はModbusサーバーのファクトリー
type ModbusServerFactory struct{}

// NewModbusServerFactory は新しいModbusServerFactoryを作成する
func NewModbusServerFactory() *ModbusServerFactory {
	return &ModbusServerFactory{}
}

// ProtocolType はファクトリーが作成するプロトコルの種類を返す
func (f *ModbusServerFactory) ProtocolType() protocol.ProtocolType {
	return protocol.ProtocolModbus
}

// DisplayName はプロトコルの表示名を返す
func (f *ModbusServerFactory) DisplayName() string {
	return "Modbus"
}

// CreateServer はサーバーを作成する
func (f *ModbusServerFactory) CreateServer(config protocol.ProtocolConfig, store protocol.DataStore) (protocol.ProtocolServer, error) {
	modbusConfig, ok := config.(*ModbusConfig)
	if !ok {
		return nil, fmt.Errorf("invalid config type: expected ModbusConfig")
	}

	modbusStore, ok := store.(*ModbusDataStore)
	if !ok {
		return nil, fmt.Errorf("invalid store type: expected ModbusDataStore")
	}

	return NewModbusServer(modbusConfig, modbusStore), nil
}

// CreateDataStore はプロトコル用のデータストアを作成する
func (f *ModbusServerFactory) CreateDataStore() protocol.DataStore {
	return NewModbusDataStore(65536, 65536, 65536, 65536)
}

// DefaultConfig はデフォルト設定を返す
func (f *ModbusServerFactory) DefaultConfig() protocol.ProtocolConfig {
	return DefaultTCPConfig()
}

// ConfigVariants は利用可能な設定バリアントを返す
func (f *ModbusServerFactory) ConfigVariants() []protocol.ConfigVariant {
	return []protocol.ConfigVariant{
		{ID: "tcp", DisplayName: "Modbus TCP"},
		{ID: "rtu", DisplayName: "Modbus RTU"},
		{ID: "ascii", DisplayName: "Modbus RTU ASCII"},
	}
}

// CreateConfigFromVariant は指定バリアントの設定を作成する
func (f *ModbusServerFactory) CreateConfigFromVariant(variantID string) protocol.ProtocolConfig {
	switch variantID {
	case "tcp":
		return DefaultTCPConfig()
	case "rtu":
		return DefaultRTUConfig()
	case "ascii":
		return DefaultASCIIConfig()
	default:
		return DefaultTCPConfig()
	}
}

// GetConfigFields は指定バリアントの設定フィールドを返す
func (f *ModbusServerFactory) GetConfigFields(variantID string) []protocol.ConfigField {
	switch variantID {
	case "tcp":
		return []protocol.ConfigField{
			{Name: "tcpAddress", Label: "アドレス", Type: "text", Required: true, Default: "0.0.0.0"},
			{Name: "tcpPort", Label: "ポート", Type: "number", Required: true, Default: 502, Min: intPtr(1), Max: intPtr(65535)},
		}
	case "rtu", "ascii":
		defaultBaud := 115200
		defaultDataBits := 8
		defaultParity := "N"
		if variantID == "ascii" {
			defaultBaud = 9600
			defaultDataBits = 7
			defaultParity = "E"
		}
		return []protocol.ConfigField{
			{Name: "serialPort", Label: "シリアルポート", Type: "serialport", Required: true, Default: "COM1"},
			{Name: "baudRate", Label: "ボーレート", Type: "select", Required: true, Default: defaultBaud, Options: []protocol.FieldOption{
				{Value: "9600", Label: "9600"},
				{Value: "19200", Label: "19200"},
				{Value: "38400", Label: "38400"},
				{Value: "57600", Label: "57600"},
				{Value: "115200", Label: "115200"},
			}},
			{Name: "dataBits", Label: "データビット", Type: "select", Required: true, Default: defaultDataBits, Options: []protocol.FieldOption{
				{Value: "7", Label: "7"},
				{Value: "8", Label: "8"},
			}},
			{Name: "stopBits", Label: "ストップビット", Type: "select", Required: true, Default: 1, Options: []protocol.FieldOption{
				{Value: "1", Label: "1"},
				{Value: "2", Label: "2"},
			}},
			{Name: "parity", Label: "パリティ", Type: "select", Required: true, Default: defaultParity, Options: []protocol.FieldOption{
				{Value: "N", Label: "None"},
				{Value: "E", Label: "Even"},
				{Value: "O", Label: "Odd"},
			}},
		}
	}
	return nil
}

// GetProtocolCapabilities はプロトコルの機能情報を返す
func (f *ModbusServerFactory) GetProtocolCapabilities() protocol.ProtocolCapabilities {
	return protocol.ProtocolCapabilities{
		SupportsUnitID: true,
		UnitIDMin:      1,
		UnitIDMax:      247,
	}
}

// ConfigToMap は設定をmapに変換する
func (f *ModbusServerFactory) ConfigToMap(config protocol.ProtocolConfig) map[string]interface{} {
	mc, ok := config.(*ModbusConfig)
	if !ok {
		return nil
	}
	result := make(map[string]interface{})
	switch mc.variant {
	case VariantTCP:
		result["tcpAddress"] = mc.TCPAddress
		result["tcpPort"] = mc.TCPPort
	case VariantRTU, VariantASCII:
		result["serialPort"] = mc.SerialPort
		result["baudRate"] = mc.BaudRate
		result["dataBits"] = mc.DataBits
		result["stopBits"] = mc.StopBits
		result["parity"] = mc.Parity
	}
	return result
}

// MapToConfig はmapから設定を作成する
func (f *ModbusServerFactory) MapToConfig(variantID string, settings map[string]interface{}) (protocol.ProtocolConfig, error) {
	config := f.CreateConfigFromVariant(variantID).(*ModbusConfig)

	switch variantID {
	case "tcp":
		if v, ok := settings["tcpAddress"].(string); ok {
			config.TCPAddress = v
		}
		if v, ok := settings["tcpPort"].(float64); ok {
			config.TCPPort = int(v)
		} else if v, ok := settings["tcpPort"].(int); ok {
			config.TCPPort = v
		}
	case "rtu", "ascii":
		if v, ok := settings["serialPort"].(string); ok {
			config.SerialPort = v
		}
		if v, ok := settings["baudRate"].(float64); ok {
			config.BaudRate = int(v)
		} else if v, ok := settings["baudRate"].(int); ok {
			config.BaudRate = v
		}
		if v, ok := settings["dataBits"].(float64); ok {
			config.DataBits = int(v)
		} else if v, ok := settings["dataBits"].(int); ok {
			config.DataBits = v
		}
		if v, ok := settings["stopBits"].(float64); ok {
			config.StopBits = int(v)
		} else if v, ok := settings["stopBits"].(int); ok {
			config.StopBits = v
		}
		if v, ok := settings["parity"].(string); ok {
			config.Parity = v
		}
	}

	return config, nil
}

func intPtr(i int) *int {
	return &i
}

// ModbusVariant はModbusのバリアント
type ModbusVariant string

const (
	VariantTCP   ModbusVariant = "tcp"
	VariantRTU   ModbusVariant = "rtu"
	VariantASCII ModbusVariant = "ascii"
)

// ModbusConfig はModbusサーバーの設定
type ModbusConfig struct {
	variant ModbusVariant

	// TCP設定
	TCPAddress string `json:"tcpAddress"`
	TCPPort    int    `json:"tcpPort"`

	// RTU設定
	SerialPort string `json:"serialPort"`
	BaudRate   int    `json:"baudRate"`
	DataBits   int    `json:"dataBits"`
	StopBits   int    `json:"stopBits"`
	Parity     string `json:"parity"`
}

// ProtocolType はプロトコルの種類を返す
func (c *ModbusConfig) ProtocolType() protocol.ProtocolType {
	return protocol.ProtocolModbus
}

// Variant はバリアント名を返す
func (c *ModbusConfig) Variant() string {
	return string(c.variant)
}

// Validate は設定を検証する
func (c *ModbusConfig) Validate() error {
	switch c.variant {
	case VariantTCP:
		if c.TCPPort < 1 || c.TCPPort > 65535 {
			return fmt.Errorf("invalid TCP port: %d", c.TCPPort)
		}
	case VariantRTU, VariantASCII:
		if c.SerialPort == "" {
			return fmt.Errorf("serial port is required")
		}
		if c.BaudRate <= 0 {
			return fmt.Errorf("invalid baud rate: %d", c.BaudRate)
		}
	default:
		return fmt.Errorf("unknown variant: %s", c.variant)
	}
	return nil
}

// Clone は設定のコピーを作成する
func (c *ModbusConfig) Clone() protocol.ProtocolConfig {
	return &ModbusConfig{
		variant:    c.variant,
		TCPAddress: c.TCPAddress,
		TCPPort:    c.TCPPort,
		SerialPort: c.SerialPort,
		BaudRate:   c.BaudRate,
		DataBits:   c.DataBits,
		StopBits:   c.StopBits,
		Parity:     c.Parity,
	}
}

// GetVariant はバリアントを返す
func (c *ModbusConfig) GetVariant() ModbusVariant {
	return c.variant
}

// DefaultTCPConfig はデフォルトのTCP設定を返す
func DefaultTCPConfig() *ModbusConfig {
	return &ModbusConfig{
		variant:    VariantTCP,
		TCPAddress: "0.0.0.0",
		TCPPort:    502,
	}
}

// DefaultRTUConfig はデフォルトのRTU設定を返す
func DefaultRTUConfig() *ModbusConfig {
	return &ModbusConfig{
		variant:    VariantRTU,
		SerialPort: "COM1",
		BaudRate:   115200,
		DataBits:   8,
		StopBits:   1,
		Parity:     "N",
	}
}

// DefaultASCIIConfig はデフォルトのASCII設定を返す
func DefaultASCIIConfig() *ModbusConfig {
	return &ModbusConfig{
		variant:    VariantASCII,
		SerialPort: "COM1",
		BaudRate:   9600,
		DataBits:   7,
		StopBits:   1,
		Parity:     "E",
	}
}

// ModbusServer はModbusプロトコルサーバー
type ModbusServer struct {
	config         *ModbusConfig
	store          *ModbusDataStore
	handler        *DataStoreHandler
	innerServer    *Server
	status         protocol.ServerStatus
	eventEmitter   protocol.CommunicationEventEmitter
	sessionManager *protocol.SessionManager
}

// NewModbusServer は新しいModbusServerを作成する
func NewModbusServer(config *ModbusConfig, store *ModbusDataStore) *ModbusServer {
	return &ModbusServer{
		config:  config,
		store:   store,
		handler: NewDataStoreHandler(store),
		status:  protocol.StatusStopped,
	}
}

// Start はサーバーを起動する
func (s *ModbusServer) Start(ctx context.Context) error {
	if s.status == protocol.StatusRunning {
		return fmt.Errorf("server is already running")
	}

	// 内部サーバーを作成
	s.innerServer = NewServerWithHandler(s.config, s.handler)

	// イベントエミッターとセッションマネージャーを設定
	if s.eventEmitter != nil {
		s.innerServer.SetEventEmitter(s.eventEmitter)
	}
	if s.sessionManager != nil {
		s.innerServer.SetSessionManager(s.sessionManager)
	}

	if err := s.innerServer.Start(); err != nil {
		s.status = protocol.StatusError
		return err
	}

	s.status = protocol.StatusRunning
	return nil
}

// Stop はサーバーを停止する
func (s *ModbusServer) Stop() error {
	if s.innerServer != nil {
		if err := s.innerServer.Stop(); err != nil {
			return err
		}
		s.innerServer = nil
	}
	s.status = protocol.StatusStopped
	return nil
}

// Status はサーバーの状態を返す
func (s *ModbusServer) Status() protocol.ServerStatus {
	return s.status
}

// ProtocolType はプロトコルの種類を返す
func (s *ModbusServer) ProtocolType() protocol.ProtocolType {
	return protocol.ProtocolModbus
}

// Config は現在の設定を返す
func (s *ModbusServer) Config() protocol.ProtocolConfig {
	return s.config
}

// UpdateConfig は設定を更新する
func (s *ModbusServer) UpdateConfig(config protocol.ProtocolConfig) error {
	if s.status == protocol.StatusRunning {
		return fmt.Errorf("cannot update config while server is running")
	}

	modbusConfig, ok := config.(*ModbusConfig)
	if !ok {
		return fmt.Errorf("invalid config type: expected ModbusConfig")
	}

	// ハンドラーの無効化UnitIDリストを保持
	disabledIDs := s.handler.GetDisabledUnitIDs()
	s.config = modbusConfig
	s.handler = NewDataStoreHandler(s.store)
	s.handler.SetDisabledUnitIDs(disabledIDs)
	return nil
}

// SetUnitIdEnabled は指定したUnitIdの応答を有効/無効にする
func (s *ModbusServer) SetUnitIdEnabled(unitId uint8, enabled bool) {
	s.handler.SetUnitIdEnabled(unitId, enabled)
}

// IsUnitIdEnabled は指定したUnitIdが応答するかどうかを返す
func (s *ModbusServer) IsUnitIdEnabled(unitId uint8) bool {
	return s.handler.IsUnitIdEnabled(unitId)
}

// GetDisabledUnitIDs は無効化されたUnitIDのリストを返す
func (s *ModbusServer) GetDisabledUnitIDs() []uint8 {
	return s.handler.GetDisabledUnitIDs()
}

// SetDisabledUnitIDs は無効化するUnitIDのリストを設定する
func (s *ModbusServer) SetDisabledUnitIDs(ids []uint8) {
	s.handler.SetDisabledUnitIDs(ids)
}

// SetEventEmitter はイベントエミッターを設定する
func (s *ModbusServer) SetEventEmitter(emitter protocol.CommunicationEventEmitter) {
	s.eventEmitter = emitter
	if s.innerServer != nil {
		s.innerServer.SetEventEmitter(emitter)
	}
}

// SetSessionManager はセッションマネージャーを設定する
func (s *ModbusServer) SetSessionManager(manager *protocol.SessionManager) {
	s.sessionManager = manager
	if s.innerServer != nil {
		s.innerServer.SetSessionManager(manager)
	}
}

// DataStoreHandler はDataStoreを使用するModbusハンドラー
type DataStoreHandler struct {
	store           *ModbusDataStore
	disabledUnitIDs map[uint8]bool
}

// NewDataStoreHandler は新しいDataStoreHandlerを作成する
func NewDataStoreHandler(store *ModbusDataStore) *DataStoreHandler {
	return &DataStoreHandler{
		store:           store,
		disabledUnitIDs: make(map[uint8]bool),
	}
}

// SetUnitIdEnabled sets whether a unit ID responds
func (h *DataStoreHandler) SetUnitIdEnabled(unitId uint8, enabled bool) {
	if enabled {
		delete(h.disabledUnitIDs, unitId)
	} else {
		h.disabledUnitIDs[unitId] = true
	}
}

// IsUnitIdEnabled checks if a unit ID responds
func (h *DataStoreHandler) IsUnitIdEnabled(unitId uint8) bool {
	return !h.disabledUnitIDs[unitId]
}

// GetDisabledUnitIDs returns the list of disabled unit IDs
func (h *DataStoreHandler) GetDisabledUnitIDs() []uint8 {
	ids := make([]uint8, 0, len(h.disabledUnitIDs))
	for id := range h.disabledUnitIDs {
		ids = append(ids, id)
	}
	return ids
}

// SetDisabledUnitIDs sets the list of disabled unit IDs
func (h *DataStoreHandler) SetDisabledUnitIDs(ids []uint8) {
	h.disabledUnitIDs = make(map[uint8]bool)
	for _, id := range ids {
		h.disabledUnitIDs[id] = true
	}
}

// DataStore のインターフェースを満たすことを確認
var _ protocol.DataStore = (*ModbusDataStore)(nil)

func init() {
	// Modbusファクトリーをデフォルトレジストリに登録
	protocol.Register(NewModbusServerFactory())
}
