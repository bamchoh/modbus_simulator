package application

import (
	"fmt"
	"sync"
	"time"

	"modbus_simulator/internal/domain/register"
	"modbus_simulator/internal/domain/script"
	"modbus_simulator/internal/domain/server"
	"modbus_simulator/internal/infrastructure/modbus"
	"modbus_simulator/internal/infrastructure/scripting"

	"github.com/google/uuid"
)

// PLCService はPLCシミュレーターのメインサービス
type PLCService struct {
	mu           sync.RWMutex
	store        *register.RegisterStore
	modbusServer *modbus.Server
	scriptEngine *scripting.ScriptEngine
	scripts      map[string]*script.Script
}

// NewPLCService は新しいPLCServiceを作成する
func NewPLCService() *PLCService {
	// デフォルトのレジスタストアを作成（各65536個）
	store := register.NewRegisterStore(65536, 65536, 65536, 65536)

	return &PLCService{
		store:        store,
		modbusServer: modbus.NewServer(server.DefaultTCPConfig(), store),
		scriptEngine: scripting.NewScriptEngine(store),
		scripts:      make(map[string]*script.Script),
	}
}

// === サーバー管理 ===

// StartServer はModbusサーバーを起動する
func (s *PLCService) StartServer() error {
	return s.modbusServer.Start()
}

// StopServer はModbusサーバーを停止する
func (s *PLCService) StopServer() error {
	return s.modbusServer.Stop()
}

// GetServerStatus はサーバーのステータスを返す
func (s *PLCService) GetServerStatus() string {
	return s.modbusServer.Status().String()
}

// GetServerConfig はサーバーの設定を返す
func (s *PLCService) GetServerConfig() *ServerConfigDTO {
	config := s.modbusServer.GetConfig()
	return &ServerConfigDTO{
		Type:       int(config.Type),
		TypeName:   config.Type.String(),
		TCPAddress: config.TCPAddress,
		TCPPort:    config.TCPPort,
		SerialPort: config.SerialPort,
		BaudRate:   config.BaudRate,
		DataBits:   config.DataBits,
		StopBits:   config.StopBits,
		Parity:     config.Parity,
	}
}

// UpdateServerConfig はサーバーの設定を更新する
func (s *PLCService) UpdateServerConfig(dto *ServerConfigDTO) error {
	config := &server.ServerConfig{
		Type:       server.ServerType(dto.Type),
		TCPAddress: dto.TCPAddress,
		TCPPort:    dto.TCPPort,
		SerialPort: dto.SerialPort,
		BaudRate:   dto.BaudRate,
		DataBits:   dto.DataBits,
		StopBits:   dto.StopBits,
		Parity:     dto.Parity,
	}
	return s.modbusServer.UpdateConfig(config)
}

// SetUnitIdEnabled は指定したUnitIdの応答を有効/無効にする
func (s *PLCService) SetUnitIdEnabled(unitId int, enabled bool) {
	s.modbusServer.SetUnitIdEnabled(uint8(unitId), enabled)
}

// IsUnitIdEnabled は指定したUnitIdが応答するかどうかを返す
func (s *PLCService) IsUnitIdEnabled(unitId int) bool {
	return s.modbusServer.IsUnitIdEnabled(uint8(unitId))
}

// GetDisabledUnitIDs は無効化されたUnitIDのリストを返す
func (s *PLCService) GetDisabledUnitIDs() []int {
	ids := s.modbusServer.GetDisabledUnitIDs()
	result := make([]int, len(ids))
	for i, id := range ids {
		result[i] = int(id)
	}
	return result
}

// SetDisabledUnitIDs は無効化するUnitIDのリストを設定する
func (s *PLCService) SetDisabledUnitIDs(ids []int) {
	uint8Ids := make([]uint8, len(ids))
	for i, id := range ids {
		uint8Ids[i] = uint8(id)
	}
	s.modbusServer.SetDisabledUnitIDs(uint8Ids)
}

// === レジスタ操作 ===

// GetCoils はコイルの値を取得する
func (s *PLCService) GetCoils(start, count int) []bool {
	vals, _ := s.store.GetCoils(uint16(start), uint16(count))
	return vals
}

// SetCoil はコイルの値を設定する
func (s *PLCService) SetCoil(address int, value bool) error {
	return s.store.SetCoil(uint16(address), value)
}

// GetDiscreteInputs はディスクリート入力の値を取得する
func (s *PLCService) GetDiscreteInputs(start, count int) []bool {
	vals, _ := s.store.GetDiscreteInputs(uint16(start), uint16(count))
	return vals
}

// SetDiscreteInput はディスクリート入力の値を設定する
func (s *PLCService) SetDiscreteInput(address int, value bool) error {
	return s.store.SetDiscreteInput(uint16(address), value)
}

// GetHoldingRegisters は保持レジスタの値を取得する
func (s *PLCService) GetHoldingRegisters(start, count int) []int {
	vals, _ := s.store.GetHoldingRegisters(uint16(start), uint16(count))
	result := make([]int, len(vals))
	for i, v := range vals {
		result[i] = int(v)
	}
	return result
}

// SetHoldingRegister は保持レジスタの値を設定する
func (s *PLCService) SetHoldingRegister(address int, value int) error {
	return s.store.SetHoldingRegister(uint16(address), uint16(value))
}

// GetInputRegisters は入力レジスタの値を取得する
func (s *PLCService) GetInputRegisters(start, count int) []int {
	vals, _ := s.store.GetInputRegisters(uint16(start), uint16(count))
	result := make([]int, len(vals))
	for i, v := range vals {
		result[i] = int(v)
	}
	return result
}

// SetInputRegister は入力レジスタの値を設定する
func (s *PLCService) SetInputRegister(address int, value int) error {
	return s.store.SetInputRegister(uint16(address), uint16(value))
}

// === スクリプト管理 ===

// CreateScript は新しいスクリプトを作成する
func (s *PLCService) CreateScript(name, code string, intervalMs int) (*ScriptDTO, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.New().String()
	sc := script.NewScript(id, name, code, time.Duration(intervalMs)*time.Millisecond)
	s.scripts[id] = sc

	return scriptToDTO(sc, false), nil
}

// UpdateScript はスクリプトを更新する
func (s *PLCService) UpdateScript(id, name, code string, intervalMs int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sc, ok := s.scripts[id]
	if !ok {
		return fmt.Errorf("script not found: %s", id)
	}

	// 実行中なら一旦停止
	wasRunning := s.scriptEngine.IsRunning(id)
	if wasRunning {
		s.scriptEngine.StopScript(id)
	}

	sc.Name = name
	sc.Code = code
	sc.Interval = time.Duration(intervalMs) * time.Millisecond

	// 実行中だった場合は再開
	if wasRunning {
		s.scriptEngine.StartScript(sc)
	}

	return nil
}

// DeleteScript はスクリプトを削除する
func (s *PLCService) DeleteScript(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.scripts[id]; !ok {
		return fmt.Errorf("script not found: %s", id)
	}

	s.scriptEngine.StopScript(id)
	delete(s.scripts, id)
	return nil
}

// GetScripts は全てのスクリプトを取得する
func (s *PLCService) GetScripts() []*ScriptDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*ScriptDTO, 0, len(s.scripts))
	for _, sc := range s.scripts {
		isRunning := s.scriptEngine.IsRunning(sc.ID)
		result = append(result, scriptToDTO(sc, isRunning))
	}
	return result
}

// GetScript は特定のスクリプトを取得する
func (s *PLCService) GetScript(id string) (*ScriptDTO, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sc, ok := s.scripts[id]
	if !ok {
		return nil, fmt.Errorf("script not found: %s", id)
	}

	isRunning := s.scriptEngine.IsRunning(id)
	return scriptToDTO(sc, isRunning), nil
}

// StartScript はスクリプトを開始する
func (s *PLCService) StartScript(id string) error {
	s.mu.RLock()
	sc, ok := s.scripts[id]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("script not found: %s", id)
	}

	return s.scriptEngine.StartScript(sc)
}

// StopScript はスクリプトを停止する
func (s *PLCService) StopScript(id string) error {
	return s.scriptEngine.StopScript(id)
}

// RunScriptOnce はスクリプトを1回だけ実行する
func (s *PLCService) RunScriptOnce(code string) (interface{}, error) {
	return s.scriptEngine.RunOnce(code)
}

// Shutdown はサービスをシャットダウンする
func (s *PLCService) Shutdown() {
	s.scriptEngine.StopAll()
	s.modbusServer.Stop()
}

// GetIntervalPresets は周期プリセットを取得する
func (s *PLCService) GetIntervalPresets() []IntervalPresetDTO {
	presets := script.IntervalPresets
	result := make([]IntervalPresetDTO, len(presets))
	for i, p := range presets {
		result[i] = IntervalPresetDTO{
			Label: p.Label,
			Ms:    int(p.Duration.Milliseconds()),
		}
	}
	return result
}

func scriptToDTO(sc *script.Script, isRunning bool) *ScriptDTO {
	return &ScriptDTO{
		ID:         sc.ID,
		Name:       sc.Name,
		Code:       sc.Code,
		IntervalMs: int(sc.Interval.Milliseconds()),
		IsRunning:  isRunning,
	}
}

// ExportProject はプロジェクト全体のデータをエクスポートする
func (s *PLCService) ExportProject() *ProjectDataDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// サーバー設定を取得
	serverConfig := s.GetServerConfig()

	// 無効化されたUnitIDを取得
	disabledUnitIDs := s.GetDisabledUnitIDs()

	// レジスタデータを取得
	coils := s.store.GetAllCoils()
	discreteInputs := s.store.GetAllDiscreteInputs()
	holdingRegs := s.store.GetAllHoldingRegisters()
	inputRegs := s.store.GetAllInputRegisters()

	// uint16 を int に変換
	holdingInts := make([]int, len(holdingRegs))
	for i, v := range holdingRegs {
		holdingInts[i] = int(v)
	}
	inputInts := make([]int, len(inputRegs))
	for i, v := range inputRegs {
		inputInts[i] = int(v)
	}

	// スクリプトを取得
	scripts := make([]*ScriptDTO, 0, len(s.scripts))
	for _, sc := range s.scripts {
		scripts = append(scripts, &ScriptDTO{
			ID:         sc.ID,
			Name:       sc.Name,
			Code:       sc.Code,
			IntervalMs: int(sc.Interval.Milliseconds()),
			IsRunning:  false, // エクスポート時は実行状態を保存しない
		})
	}

	return &ProjectDataDTO{
		Version:         1,
		ServerConfig:    serverConfig,
		DisabledUnitIDs: disabledUnitIDs,
		Registers: &RegisterDataDTO{
			Coils:            coils,
			DiscreteInputs:   discreteInputs,
			HoldingRegisters: holdingInts,
			InputRegisters:   inputInts,
		},
		Scripts: scripts,
	}
}

// ImportProject はプロジェクト全体のデータをインポートする
func (s *PLCService) ImportProject(data *ProjectDataDTO) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 実行中のスクリプトを全て停止
	s.scriptEngine.StopAll()

	// サーバー設定を更新（サーバーが停止中の場合のみ）
	if data.ServerConfig != nil {
		config := &server.ServerConfig{
			Type:       server.ServerType(data.ServerConfig.Type),
			TCPAddress: data.ServerConfig.TCPAddress,
			TCPPort:    data.ServerConfig.TCPPort,
			SerialPort: data.ServerConfig.SerialPort,
			BaudRate:   data.ServerConfig.BaudRate,
			DataBits:   data.ServerConfig.DataBits,
			StopBits:   data.ServerConfig.StopBits,
			Parity:     data.ServerConfig.Parity,
		}
		// サーバーが実行中でもエラーを無視（設定は次回起動時に反映）
		s.modbusServer.UpdateConfig(config)
	}

	// 無効化されたUnitIDを設定
	if data.DisabledUnitIDs != nil {
		uint8Ids := make([]uint8, len(data.DisabledUnitIDs))
		for i, id := range data.DisabledUnitIDs {
			uint8Ids[i] = uint8(id)
		}
		s.modbusServer.SetDisabledUnitIDs(uint8Ids)
	}

	// レジスタデータを設定
	if data.Registers != nil {
		if data.Registers.Coils != nil {
			s.store.SetAllCoils(data.Registers.Coils)
		}
		if data.Registers.DiscreteInputs != nil {
			s.store.SetAllDiscreteInputs(data.Registers.DiscreteInputs)
		}
		if data.Registers.HoldingRegisters != nil {
			uint16Vals := make([]uint16, len(data.Registers.HoldingRegisters))
			for i, v := range data.Registers.HoldingRegisters {
				uint16Vals[i] = uint16(v)
			}
			s.store.SetAllHoldingRegisters(uint16Vals)
		}
		if data.Registers.InputRegisters != nil {
			uint16Vals := make([]uint16, len(data.Registers.InputRegisters))
			for i, v := range data.Registers.InputRegisters {
				uint16Vals[i] = uint16(v)
			}
			s.store.SetAllInputRegisters(uint16Vals)
		}
	}

	// スクリプトを設定
	if data.Scripts != nil {
		// 既存のスクリプトをクリア
		s.scripts = make(map[string]*script.Script)

		for _, dto := range data.Scripts {
			sc := script.NewScript(
				dto.ID,
				dto.Name,
				dto.Code,
				time.Duration(dto.IntervalMs)*time.Millisecond,
			)
			s.scripts[dto.ID] = sc
		}
	}

	return nil
}
