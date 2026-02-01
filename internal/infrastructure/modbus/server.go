package modbus

import (
	"fmt"
	"sync"

	"modbus_simulator/internal/domain/protocol"
	"modbus_simulator/internal/domain/register"
	"modbus_simulator/internal/domain/server"
	"modbus_simulator/internal/infrastructure/modbus/rtu"

	"github.com/simonvetter/modbus"
)

// ModbusHandler はModbusリクエストを処理するインターフェース
type ModbusHandler interface {
	SetUnitIdEnabled(unitId uint8, enabled bool)
	IsUnitIdEnabled(unitId uint8) bool
	GetDisabledUnitIDs() []uint8
	SetDisabledUnitIDs(ids []uint8)
}

// Server はModbusサーバーを管理する
type Server struct {
	mu             sync.Mutex
	config         *server.ServerConfig
	modbusConfig   *ModbusConfig
	store          *register.RegisterStore
	handler        *RegisterHandler
	dsHandler      *DataStoreHandler
	server         *modbus.ModbusServer
	rtuServer      *rtu.RTUServer
	asciiServer    *rtu.ASCIIServer
	status         server.ServerStatus
	lastErr        error
	useDataStore   bool
	eventEmitter   protocol.CommunicationEventEmitter
	sessionManager *protocol.SessionManager
}

// NewServer は新しいModbusサーバーを作成する
func NewServer(config *server.ServerConfig, store *register.RegisterStore) *Server {
	return &Server{
		config:       config,
		store:        store,
		handler:      NewRegisterHandler(store),
		status:       server.StatusStopped,
		useDataStore: false,
	}
}

// NewServerWithHandler はDataStoreHandlerを使用するサーバーを作成する
func NewServerWithHandler(config *ModbusConfig, handler *DataStoreHandler) *Server {
	// ModbusConfigからserver.ServerConfigへ変換
	serverType := server.ModbusTCP
	switch config.GetVariant() {
	case VariantRTU:
		serverType = server.ModbusRTU
	case VariantASCII:
		serverType = server.ModbusRTUASCII
	}

	serverConfig := &server.ServerConfig{
		Type:       serverType,
		TCPAddress: config.TCPAddress,
		TCPPort:    config.TCPPort,
		SerialPort: config.SerialPort,
		BaudRate:   config.BaudRate,
		DataBits:   config.DataBits,
		StopBits:   config.StopBits,
		Parity:     config.Parity,
	}

	return &Server{
		config:       serverConfig,
		modbusConfig: config,
		dsHandler:    handler,
		status:       server.StatusStopped,
		useDataStore: true,
	}
}

// Start はサーバーを起動する
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == server.StatusRunning {
		return fmt.Errorf("server is already running")
	}

	switch s.config.Type {
	case server.ModbusTCP:
		return s.startTCPServer()
	case server.ModbusRTU:
		return s.startRTUServer()
	case server.ModbusRTUASCII:
		return s.startASCIIServer()
	default:
		return fmt.Errorf("unknown server type: %v", s.config.Type)
	}
}

// startTCPServer はTCPサーバーを起動する（simonvetter/modbusを使用）
func (s *Server) startTCPServer() error {
	url := fmt.Sprintf("tcp://%s:%d", s.config.TCPAddress, s.config.TCPPort)

	// 使用するハンドラーを決定
	var handler modbus.RequestHandler
	if s.useDataStore && s.dsHandler != nil {
		reqHandler := NewDataStoreRequestHandler(s.dsHandler)
		reqHandler.SetEventEmitter(s.eventEmitter)
		reqHandler.SetSessionManager(s.sessionManager)
		handler = reqHandler
	} else {
		handler = s.handler
	}

	srv, err := modbus.NewServer(&modbus.ServerConfiguration{
		URL: url,
	}, handler)
	if err != nil {
		s.status = server.StatusError
		s.lastErr = err
		return fmt.Errorf("failed to create server: %w", err)
	}

	if err := srv.Start(); err != nil {
		s.status = server.StatusError
		s.lastErr = err
		return fmt.Errorf("failed to start server: %w", err)
	}

	s.server = srv
	s.status = server.StatusRunning
	s.lastErr = nil
	return nil
}

// startRTUServer はRTUサーバーを起動する（自作実装）
func (s *Server) startRTUServer() error {
	config := rtu.SerialConfig{
		Port:     s.config.SerialPort,
		BaudRate: s.config.BaudRate,
		DataBits: s.config.DataBits,
		StopBits: s.config.StopBits,
		Parity:   s.config.Parity,
	}

	var adapter rtu.RequestHandler
	if s.useDataStore && s.dsHandler != nil {
		rtuAdapter := NewRTUDataStoreAdapter(s.dsHandler)
		rtuAdapter.SetEventEmitter(s.eventEmitter)
		adapter = rtuAdapter
	} else {
		adapter = NewRTUHandlerAdapter(s.handler)
	}
	rtuSrv := rtu.NewRTUServer(config, adapter)

	if err := rtuSrv.Start(); err != nil {
		s.status = server.StatusError
		s.lastErr = err
		return fmt.Errorf("failed to start RTU server: %w", err)
	}

	s.rtuServer = rtuSrv
	s.status = server.StatusRunning
	s.lastErr = nil
	return nil
}

// startASCIIServer はRTU ASCIIサーバーを起動する（自作実装）
func (s *Server) startASCIIServer() error {
	config := rtu.SerialConfig{
		Port:     s.config.SerialPort,
		BaudRate: s.config.BaudRate,
		DataBits: s.config.DataBits,
		StopBits: s.config.StopBits,
		Parity:   s.config.Parity,
	}

	var adapter rtu.RequestHandler
	if s.useDataStore && s.dsHandler != nil {
		rtuAdapter := NewRTUDataStoreAdapter(s.dsHandler)
		rtuAdapter.SetEventEmitter(s.eventEmitter)
		adapter = rtuAdapter
	} else {
		adapter = NewRTUHandlerAdapter(s.handler)
	}
	asciiSrv := rtu.NewASCIIServer(config, adapter)

	if err := asciiSrv.Start(); err != nil {
		s.status = server.StatusError
		s.lastErr = err
		return fmt.Errorf("failed to start ASCII server: %w", err)
	}

	s.asciiServer = asciiSrv
	s.status = server.StatusRunning
	s.lastErr = nil
	return nil
}

// Stop はサーバーを停止する
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// ASCIIサーバーの停止
	if s.asciiServer != nil {
		if err := s.asciiServer.Stop(); err != nil {
			return fmt.Errorf("failed to stop ASCII server: %w", err)
		}
		s.asciiServer = nil
		s.status = server.StatusStopped
		return nil
	}

	// RTUサーバーの停止
	if s.rtuServer != nil {
		if err := s.rtuServer.Stop(); err != nil {
			return fmt.Errorf("failed to stop RTU server: %w", err)
		}
		s.rtuServer = nil
		s.status = server.StatusStopped
		return nil
	}

	// TCPサーバーの停止
	if s.server == nil {
		return nil
	}

	if err := s.server.Stop(); err != nil {
		return fmt.Errorf("failed to stop server: %w", err)
	}

	s.server = nil
	s.status = server.StatusStopped
	return nil
}

// Status はサーバーの状態を返す
func (s *Server) Status() server.ServerStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// LastError は最後のエラーを返す
func (s *Server) LastError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastErr
}

// UpdateConfig は設定を更新する（サーバーが停止中のみ）
func (s *Server) UpdateConfig(config *server.ServerConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == server.StatusRunning {
		return fmt.Errorf("cannot update config while server is running")
	}

	if s.useDataStore {
		// DataStoreHandlerを使用している場合
		s.config = config
	} else {
		// 現在の無効化UnitIDリストを保持
		disabledIDs := s.handler.GetDisabledUnitIDs()
		s.config = config
		s.handler = NewRegisterHandler(s.store)
		s.handler.SetDisabledUnitIDs(disabledIDs)
	}
	return nil
}

// GetConfig は現在の設定を返す
func (s *Server) GetConfig() *server.ServerConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config
}

// SetUnitIdEnabled は指定したUnitIdの応答を有効/無効にする
func (s *Server) SetUnitIdEnabled(unitId uint8, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.useDataStore && s.dsHandler != nil {
		s.dsHandler.SetUnitIdEnabled(unitId, enabled)
	} else if s.handler != nil {
		s.handler.SetUnitIdEnabled(unitId, enabled)
	}
}

// IsUnitIdEnabled は指定したUnitIdが応答するかどうかを返す
func (s *Server) IsUnitIdEnabled(unitId uint8) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.useDataStore && s.dsHandler != nil {
		return s.dsHandler.IsUnitIdEnabled(unitId)
	} else if s.handler != nil {
		return s.handler.IsUnitIdEnabled(unitId)
	}
	return true
}

// GetDisabledUnitIDs は無効化されたUnitIDのリストを返す
func (s *Server) GetDisabledUnitIDs() []uint8 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.useDataStore && s.dsHandler != nil {
		return s.dsHandler.GetDisabledUnitIDs()
	} else if s.handler != nil {
		return s.handler.GetDisabledUnitIDs()
	}
	return nil
}

// SetDisabledUnitIDs は無効化するUnitIDのリストを設定する
func (s *Server) SetDisabledUnitIDs(ids []uint8) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.useDataStore && s.dsHandler != nil {
		s.dsHandler.SetDisabledUnitIDs(ids)
	} else if s.handler != nil {
		s.handler.SetDisabledUnitIDs(ids)
	}
}

// SetEventEmitter はイベントエミッターを設定する
func (s *Server) SetEventEmitter(emitter protocol.CommunicationEventEmitter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventEmitter = emitter
}

// SetSessionManager はセッションマネージャーを設定する
func (s *Server) SetSessionManager(manager *protocol.SessionManager) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessionManager = manager
}
