package modbus

import (
	"fmt"
	"sync"

	"modbus_simulator/internal/domain/register"
	"modbus_simulator/internal/domain/server"

	"github.com/simonvetter/modbus"
)

// Server はModbusサーバーを管理する
type Server struct {
	mu       sync.Mutex
	config   *server.ServerConfig
	store    *register.RegisterStore
	handler  *RegisterHandler
	server   *modbus.ModbusServer
	status   server.ServerStatus
	lastErr  error
}

// NewServer は新しいModbusサーバーを作成する
func NewServer(config *server.ServerConfig, store *register.RegisterStore) *Server {
	return &Server{
		config:  config,
		store:   store,
		handler: NewRegisterHandler(store),
		status:  server.StatusStopped,
	}
}

// Start はサーバーを起動する
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == server.StatusRunning {
		return fmt.Errorf("server is already running")
	}

	var url string
	switch s.config.Type {
	case server.ModbusTCP:
		url = fmt.Sprintf("tcp://%s:%d", s.config.TCPAddress, s.config.TCPPort)
	case server.ModbusRTU:
		url = fmt.Sprintf("%s?baud=%d&data=%d&stop=%d&parity=%s",
			s.config.SerialPort, s.config.BaudRate, s.config.DataBits, s.config.StopBits, s.config.Parity)
	case server.ModbusRTUASCII:
		url = fmt.Sprintf("%s?baud=%d&data=%d&stop=%d&parity=%s",
			s.config.SerialPort, s.config.BaudRate, s.config.DataBits, s.config.StopBits, s.config.Parity)
	default:
		return fmt.Errorf("unknown server type: %v", s.config.Type)
	}

	srv, err := modbus.NewServer(&modbus.ServerConfiguration{
		URL: url,
	}, s.handler)
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

// Stop はサーバーを停止する
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

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

	// 現在の無効化UnitIDリストを保持
	disabledIDs := s.handler.GetDisabledUnitIDs()
	s.config = config
	s.handler = NewRegisterHandler(s.store)
	s.handler.SetDisabledUnitIDs(disabledIDs)
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
	s.handler.SetUnitIdEnabled(unitId, enabled)
}

// IsUnitIdEnabled は指定したUnitIdが応答するかどうかを返す
func (s *Server) IsUnitIdEnabled(unitId uint8) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.handler.IsUnitIdEnabled(unitId)
}

// GetDisabledUnitIDs は無効化されたUnitIDのリストを返す
func (s *Server) GetDisabledUnitIDs() []uint8 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.handler.GetDisabledUnitIDs()
}

// SetDisabledUnitIDs は無効化するUnitIDのリストを設定する
func (s *Server) SetDisabledUnitIDs(ids []uint8) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler.SetDisabledUnitIDs(ids)
}
