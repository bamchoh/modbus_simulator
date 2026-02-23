package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"modbus_simulator/internal/domain/protocol"
	"modbus_simulator/internal/domain/script"
	"modbus_simulator/internal/domain/variable"
	"modbus_simulator/internal/infrastructure/adapter"
	"modbus_simulator/internal/infrastructure/scripting"

	"github.com/google/uuid"
)

// serverInstance はサーバーインスタンスの内部表現
type serverInstance struct {
	protocolType protocol.ProtocolType
	variant      string
	factory      protocol.ServerFactory
	dataStore    protocol.DataStore
	config       protocol.ProtocolConfig
	server       protocol.ProtocolServer
}

// PLCService はPLCシミュレーターのメインサービス
type PLCService struct {
	mu       sync.RWMutex
	registry *protocol.Registry

	// 中央変数ストア
	variableStore *variable.VariableStore

	// 複数サーバーインスタンス（protocolType → instance）
	servers map[protocol.ProtocolType]*serverInstance

	// スクリプト
	scriptEngine *scripting.ScriptEngine
	scripts      map[string]*script.Script

	// モニタリング
	monitoringItems map[string]*MonitoringItemDTO

	// 通信イベント
	eventEmitter   protocol.CommunicationEventEmitter
	sessionManager *protocol.SessionManager
}

// NewPLCService は新しいPLCServiceを作成する
func NewPLCService() *PLCService {
	varStore := variable.NewVariableStore()

	service := &PLCService{
		registry:        protocol.DefaultRegistry,
		variableStore:   varStore,
		servers:         make(map[protocol.ProtocolType]*serverInstance),
		scriptEngine:    scripting.NewScriptEngine(varStore),
		scripts:         make(map[string]*script.Script),
		monitoringItems: make(map[string]*MonitoringItemDTO),
	}

	// 変数設定を読み込み
	_ = service.LoadVariablesConfig()

	// 変数変更時に自動保存
	varStore.SetOnChange(func() {
		go service.saveVariablesConfigInternal()
	})

	// デフォルトでModbus TCPを追加
	_ = service.AddServer("modbus", "tcp")

	// モニタリング設定を読み込み
	_ = service.LoadMonitoringConfig()

	return service
}

// === サーバーインスタンス管理 ===

// GetServerInstances はサーバーインスタンス一覧を返す
func (s *PLCService) GetServerInstances() []ServerInstanceDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ServerInstanceDTO, 0, len(s.servers))
	for _, inst := range s.servers {
		status := "Stopped"
		if inst.server != nil {
			status = inst.server.Status().String()
		}
		result = append(result, ServerInstanceDTO{
			ProtocolType: string(inst.protocolType),
			DisplayName:  inst.factory.DisplayName(),
			Variant:      inst.variant,
			Status:       status,
		})
	}

	// プロトコルタイプ順にソート（安定した順序）
	sort.Slice(result, func(i, j int) bool {
		return result[i].ProtocolType < result[j].ProtocolType
	})

	return result
}

// AddServer は新しいサーバーインスタンスを追加する
func (s *PLCService) AddServer(protocolType string, variantID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pt := protocol.ProtocolType(protocolType)

	// 既に存在する場合はエラー
	if _, exists := s.servers[pt]; exists {
		return fmt.Errorf("server already exists for protocol: %s", protocolType)
	}

	// ファクトリーを取得
	factory, err := s.registry.Get(pt)
	if err != nil {
		return err
	}

	// サーバーインスタンスを作成
	config := factory.CreateConfigFromVariant(variantID)
	innerDataStore := factory.CreateDataStore()
	dataStore := adapter.NewVariableBackedDataStore(innerDataStore, s.variableStore, protocolType)

	server, err := factory.CreateServer(config, dataStore)
	if err != nil {
		return err
	}

	inst := &serverInstance{
		protocolType: pt,
		variant:      variantID,
		factory:      factory,
		dataStore:    dataStore,
		config:       config,
		server:       server,
	}

	s.servers[pt] = inst

	// イベントエミッターをサーバーに設定
	if s.eventEmitter != nil {
		s.setEmitterToServerInstance(inst)
	}

	return nil
}

// RemoveServer はサーバーインスタンスを削除する
func (s *PLCService) RemoveServer(protocolType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pt := protocol.ProtocolType(protocolType)
	inst, exists := s.servers[pt]
	if !exists {
		return fmt.Errorf("server not found for protocol: %s", protocolType)
	}

	// 実行中なら停止
	if inst.server != nil && inst.server.Status() == protocol.StatusRunning {
		if err := inst.server.Stop(); err != nil {
			return fmt.Errorf("failed to stop server: %w", err)
		}
	}

	// アダプターを解除
	if ds, ok := inst.dataStore.(*adapter.VariableBackedDataStore); ok {
		ds.Detach()
	}

	delete(s.servers, pt)
	return nil
}

// getServerInstance は指定プロトコルのサーバーインスタンスを取得する（ロック済み前提）
func (s *PLCService) getServerInstance(protocolType string) (*serverInstance, error) {
	pt := protocol.ProtocolType(protocolType)
	inst, exists := s.servers[pt]
	if !exists {
		return nil, fmt.Errorf("server not found for protocol: %s", protocolType)
	}
	return inst, nil
}

// === サーバー管理 ===

// StartServer はサーバーを起動する
func (s *PLCService) StartServer(protocolType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	inst, err := s.getServerInstance(protocolType)
	if err != nil {
		return err
	}
	if inst.server == nil {
		return fmt.Errorf("server not initialized")
	}
	return inst.server.Start(context.Background())
}

// StopServer はサーバーを停止する
func (s *PLCService) StopServer(protocolType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	inst, err := s.getServerInstance(protocolType)
	if err != nil {
		return err
	}
	if inst.server != nil {
		return inst.server.Stop()
	}
	return nil
}

// GetServerStatus はサーバーのステータスを返す
func (s *PLCService) GetServerStatus(protocolType string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	inst, err := s.getServerInstance(protocolType)
	if err != nil {
		return "Stopped"
	}
	if inst.server != nil {
		return inst.server.Status().String()
	}
	return "Stopped"
}

// === プロトコル管理API ===

// GetAvailableProtocols は利用可能なプロトコル一覧を返す
func (s *PLCService) GetAvailableProtocols() []ProtocolInfoDTO {
	factories := s.registry.GetAll()
	result := make([]ProtocolInfoDTO, len(factories))
	for i, factory := range factories {
		variants := factory.ConfigVariants()
		variantDTOs := make([]ConfigVariantDTO, len(variants))
		for j, v := range variants {
			variantDTOs[j] = ConfigVariantDTO{
				ID:          v.ID,
				DisplayName: v.DisplayName,
			}
		}
		result[i] = ProtocolInfoDTO{
			Type:        string(factory.ProtocolType()),
			DisplayName: factory.DisplayName(),
			Variants:    variantDTOs,
		}
	}
	return result
}

// GetProtocolSchema はプロトコルスキーマを返す
func (s *PLCService) GetProtocolSchema(protocolType string) (*ProtocolSchemaDTO, error) {
	factory, err := s.registry.Get(protocol.ProtocolType(protocolType))
	if err != nil {
		return nil, err
	}

	variants := factory.ConfigVariants()
	variantDTOs := make([]VariantDTO, len(variants))
	for i, v := range variants {
		fields := factory.GetConfigFields(v.ID)
		fieldDTOs := make([]FieldDTO, len(fields))
		for j, f := range fields {
			fieldDTOs[j] = FieldDTO{
				Name:     f.Name,
				Label:    f.Label,
				Type:     f.Type,
				Required: f.Required,
				Default:  f.Default,
				Min:      f.Min,
				Max:      f.Max,
			}
			if f.Options != nil {
				fieldDTOs[j].Options = make([]OptionDTO, len(f.Options))
				for k, o := range f.Options {
					fieldDTOs[j].Options[k] = OptionDTO{Value: o.Value, Label: o.Label}
				}
			}
			if f.Condition != nil {
				fieldDTOs[j].ShowWhen = &ConditionDTO{Field: f.Condition.Field, Value: f.Condition.Value}
			}
		}
		variantDTOs[i] = VariantDTO{
			ID:          v.ID,
			DisplayName: v.DisplayName,
			Fields:      fieldDTOs,
		}
	}

	caps := factory.GetProtocolCapabilities()
	return &ProtocolSchemaDTO{
		ProtocolType: string(factory.ProtocolType()),
		DisplayName:  factory.DisplayName(),
		Variants:     variantDTOs,
		Capabilities: CapabilitiesDTO{
			SupportsUnitID: caps.SupportsUnitID,
			UnitIDMin:      caps.UnitIDMin,
			UnitIDMax:      caps.UnitIDMax,
		},
	}, nil
}

// GetServerConfig は指定サーバーの現在の設定を返す
func (s *PLCService) GetServerConfig(protocolType string) *ServerConfigDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()

	inst, err := s.getServerInstance(protocolType)
	if err != nil {
		return nil
	}

	return &ServerConfigDTO{
		ProtocolType: protocolType,
		Variant:      inst.variant,
		Settings:     inst.factory.ConfigToMap(inst.config),
	}
}

// UpdateServerConfig はサーバーの設定を更新する
func (s *PLCService) UpdateServerConfig(dto *ServerConfigDTO) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	inst, err := s.getServerInstance(dto.ProtocolType)
	if err != nil {
		return err
	}

	if inst.server != nil && inst.server.Status() == protocol.StatusRunning {
		return fmt.Errorf("cannot update config while server is running")
	}

	// バリアントが変わる場合は新しいサーバーを作成
	if dto.Variant != inst.variant {
		config := inst.factory.CreateConfigFromVariant(dto.Variant)
		server, err := inst.factory.CreateServer(config, inst.dataStore)
		if err != nil {
			return err
		}
		inst.variant = dto.Variant
		inst.config = config
		inst.server = server

		if s.eventEmitter != nil {
			s.setEmitterToServerInstance(inst)
		}
	}

	// 設定を更新
	newConfig, err := inst.factory.MapToConfig(inst.variant, dto.Settings)
	if err != nil {
		return err
	}

	if err := inst.server.UpdateConfig(newConfig); err != nil {
		return err
	}

	inst.config = newConfig
	return nil
}

// === UnitID設定 ===

// GetUnitIDSettings はUnitID設定を返す
func (s *PLCService) GetUnitIDSettings(protocolType string) *UnitIDSettingsDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()

	inst, err := s.getServerInstance(protocolType)
	if err != nil {
		return nil
	}

	caps := inst.factory.GetProtocolCapabilities()
	if !caps.SupportsUnitID {
		return nil
	}

	type unitIDSupporter interface {
		GetDisabledUnitIDs() []uint8
	}

	var disabledIDs []int
	if us, ok := inst.server.(unitIDSupporter); ok {
		ids := us.GetDisabledUnitIDs()
		disabledIDs = make([]int, len(ids))
		for i, id := range ids {
			disabledIDs[i] = int(id)
		}
	}

	return &UnitIDSettingsDTO{
		Min:         caps.UnitIDMin,
		Max:         caps.UnitIDMax,
		DisabledIDs: disabledIDs,
	}
}

// SetUnitIDEnabled は指定したUnitIdの応答を有効/無効にする
func (s *PLCService) SetUnitIDEnabled(protocolType string, unitId int, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	inst, err := s.getServerInstance(protocolType)
	if err != nil {
		return err
	}

	type unitIDSupporter interface {
		SetUnitIdEnabled(unitId uint8, enabled bool)
	}

	if us, ok := inst.server.(unitIDSupporter); ok {
		us.SetUnitIdEnabled(uint8(unitId), enabled)
		return nil
	}

	return fmt.Errorf("protocol does not support unit ID")
}

// GetDisabledUnitIDs は無効化されたUnitIDのリストを返す
func (s *PLCService) GetDisabledUnitIDs(protocolType string) []int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	inst, err := s.getServerInstance(protocolType)
	if err != nil {
		return nil
	}

	type unitIDSupporter interface {
		GetDisabledUnitIDs() []uint8
	}

	if us, ok := inst.server.(unitIDSupporter); ok {
		ids := us.GetDisabledUnitIDs()
		result := make([]int, len(ids))
		for i, id := range ids {
			result[i] = int(id)
		}
		return result
	}
	return nil
}

// SetDisabledUnitIDs は無効化するUnitIDのリストを設定する
func (s *PLCService) SetDisabledUnitIDs(protocolType string, ids []int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	inst, err := s.getServerInstance(protocolType)
	if err != nil {
		return err
	}

	type unitIDSupporter interface {
		SetDisabledUnitIDs(ids []uint8)
	}

	if us, ok := inst.server.(unitIDSupporter); ok {
		uint8Ids := make([]uint8, len(ids))
		for i, id := range ids {
			uint8Ids[i] = uint8(id)
		}
		us.SetDisabledUnitIDs(uint8Ids)
		return nil
	}

	return fmt.Errorf("protocol does not support unit ID")
}

// === 汎用メモリ操作API ===

// GetMemoryAreas は利用可能なメモリエリアの一覧を返す
func (s *PLCService) GetMemoryAreas(protocolType string) []MemoryAreaDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()

	inst, err := s.getServerInstance(protocolType)
	if err != nil {
		return nil
	}

	areas := inst.dataStore.GetAreas()
	result := make([]MemoryAreaDTO, len(areas))
	for i, area := range areas {
		result[i] = MemoryAreaDTO{
			ID:             area.ID,
			DisplayName:    area.DisplayName,
			IsBit:          area.IsBit,
			Size:           int(area.Size),
			ReadOnly:       area.ReadOnly,
			ByteAddressing: area.ByteAddressing,
			OneOrigin:      area.OneOrigin,
		}
	}
	return result
}

// ReadBits は指定エリアの複数ビット値を読み込む
func (s *PLCService) ReadBits(protocolType, area string, address, count int) ([]bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	inst, err := s.getServerInstance(protocolType)
	if err != nil {
		return nil, err
	}
	return inst.dataStore.ReadBits(area, uint32(address), uint16(count))
}

// WriteBit は指定エリアのビット値を書き込む
func (s *PLCService) WriteBit(protocolType, area string, address int, value bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	inst, err := s.getServerInstance(protocolType)
	if err != nil {
		return err
	}
	return inst.dataStore.WriteBit(area, uint32(address), value)
}

// ReadWords は指定エリアの複数ワード値を読み込む
func (s *PLCService) ReadWords(protocolType, area string, address, count int) ([]int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	inst, err := s.getServerInstance(protocolType)
	if err != nil {
		return nil, err
	}

	vals, err := inst.dataStore.ReadWords(area, uint32(address), uint16(count))
	if err != nil {
		return nil, err
	}
	result := make([]int, len(vals))
	for i, v := range vals {
		result[i] = int(v)
	}
	return result, nil
}

// WriteWord は指定エリアのワード値を書き込む
func (s *PLCService) WriteWord(protocolType, area string, address int, value int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	inst, err := s.getServerInstance(protocolType)
	if err != nil {
		return err
	}
	return inst.dataStore.WriteWord(area, uint32(address), uint16(value))
}

// === スクリプト管理 ===

// CreateScript は新しいスクリプトを作成する
func (s *PLCService) CreateScript(name, code string, intervalMs int) (*ScriptDTO, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.New().String()
	sc := script.NewScript(id, name, code, time.Duration(intervalMs)*time.Millisecond)
	s.scripts[id] = sc

	return scriptToDTO(sc, false, "", 0), nil
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
		var lastError string
		var errorAtMs int64
		if isRunning {
			errMsg, errAt := s.scriptEngine.GetLastError(sc.ID)
			lastError = errMsg
			if !errAt.IsZero() {
				errorAtMs = errAt.UnixMilli()
			}
		}
		result = append(result, scriptToDTO(sc, isRunning, lastError, errorAtMs))
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
	var lastError string
	var errorAtMs int64
	if isRunning {
		errMsg, errAt := s.scriptEngine.GetLastError(id)
		lastError = errMsg
		if !errAt.IsZero() {
			errorAtMs = errAt.UnixMilli()
		}
	}
	return scriptToDTO(sc, isRunning, lastError, errorAtMs), nil
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

// ClearScriptError はスクリプトのエラー情報をクリアする
func (s *PLCService) ClearScriptError(id string) {
	s.scriptEngine.ClearError(id)
}

// GetConsoleLogs はコンソールログの一覧を返す
func (s *PLCService) GetConsoleLogs() []ConsoleLogDTO {
	entries := s.scriptEngine.GetConsoleLogs()
	result := make([]ConsoleLogDTO, len(entries))
	for i, e := range entries {
		result[i] = ConsoleLogDTO{
			ScriptID:   e.ScriptID,
			ScriptName: e.ScriptName,
			Message:    e.Message,
			At:         e.At.UnixMilli(),
		}
	}
	return result
}

// ClearConsoleLogs はコンソールログをクリアする
func (s *PLCService) ClearConsoleLogs() {
	s.scriptEngine.ClearConsoleLogs()
}

// Shutdown はサービスをシャットダウンする
func (s *PLCService) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.scriptEngine != nil {
		s.scriptEngine.StopAll()
	}
	for _, inst := range s.servers {
		if inst.server != nil {
			inst.server.Stop()
		}
	}
	if s.sessionManager != nil {
		s.sessionManager.Stop()
	}
}

// SetEventEmitter はイベントエミッターを設定する
func (s *PLCService) SetEventEmitter(emitter protocol.CommunicationEventEmitter) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.eventEmitter = emitter

	// セッションマネージャーを作成
	s.sessionManager = protocol.NewSessionManager(5*time.Second, emitter)
	s.sessionManager.Start()

	// 全サーバーにエミッターを設定
	for _, inst := range s.servers {
		s.setEmitterToServerInstance(inst)
	}
}

// GetEventEmitter はイベントエミッターを返す
func (s *PLCService) GetEventEmitter() protocol.CommunicationEventEmitter {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.eventEmitter
}

// GetSessionManager はセッションマネージャーを返す
func (s *PLCService) GetSessionManager() *protocol.SessionManager {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessionManager
}

// setEmitterToServerInstance はサーバーインスタンスにイベントエミッターを設定する（ロック済み前提）
func (s *PLCService) setEmitterToServerInstance(inst *serverInstance) {
	type eventAware interface {
		SetEventEmitter(emitter protocol.CommunicationEventEmitter)
	}
	type sessionAware interface {
		SetSessionManager(manager *protocol.SessionManager)
	}

	if ea, ok := inst.server.(eventAware); ok {
		ea.SetEventEmitter(s.eventEmitter)
	}
	if sa, ok := inst.server.(sessionAware); ok {
		sa.SetSessionManager(s.sessionManager)
	}
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

func scriptToDTO(sc *script.Script, isRunning bool, lastError string, errorAtMs int64) *ScriptDTO {
	return &ScriptDTO{
		ID:         sc.ID,
		Name:       sc.Name,
		Code:       sc.Code,
		IntervalMs: int(sc.Interval.Milliseconds()),
		IsRunning:  isRunning,
		LastError:  lastError,
		ErrorAt:    errorAtMs,
	}
}

// === プロジェクトExport/Import ===

// ExportProject はプロジェクト全体のデータをエクスポートする
func (s *PLCService) ExportProject() *ProjectDataDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 全サーバーのスナップショットを取得
	servers := make([]ServerSnapshotDTO, 0, len(s.servers))
	for _, inst := range s.servers {
		var settings map[string]interface{}
		if inst.config != nil {
			settings = inst.factory.ConfigToMap(inst.config)
		}

		var memorySnapshot map[string]interface{}
		if inst.dataStore != nil {
			memorySnapshot = inst.dataStore.Snapshot()
		}

		var unitIDSettings *UnitIDSettingsDTO
		caps := inst.factory.GetProtocolCapabilities()
		if caps.SupportsUnitID {
			type unitIDSupporter interface {
				GetDisabledUnitIDs() []uint8
			}
			if us, ok := inst.server.(unitIDSupporter); ok {
				ids := us.GetDisabledUnitIDs()
				disabledIDs := make([]int, len(ids))
				for i, id := range ids {
					disabledIDs[i] = int(id)
				}
				unitIDSettings = &UnitIDSettingsDTO{
					Min:         caps.UnitIDMin,
					Max:         caps.UnitIDMax,
					DisabledIDs: disabledIDs,
				}
			}
		}

		servers = append(servers, ServerSnapshotDTO{
			ProtocolType:   string(inst.protocolType),
			Variant:        inst.variant,
			Settings:       settings,
			MemorySnapshot: memorySnapshot,
			UnitIDSettings: unitIDSettings,
		})
	}

	// スクリプトを取得
	scripts := make([]*ScriptDTO, 0, len(s.scripts))
	for _, sc := range s.scripts {
		scripts = append(scripts, &ScriptDTO{
			ID:         sc.ID,
			Name:       sc.Name,
			Code:       sc.Code,
			IntervalMs: int(sc.Interval.Milliseconds()),
			IsRunning:  false,
		})
	}

	// モニタリング項目を取得
	monitoringItems := make([]*MonitoringItemDTO, 0, len(s.monitoringItems))
	for _, item := range s.monitoringItems {
		monitoringItems = append(monitoringItems, item)
	}

	// 構造体型定義を取得
	allStructTypes := s.variableStore.GetAllStructTypes()
	structTypeDTOs := make([]StructTypeDTO, len(allStructTypes))
	for i, st := range allStructTypes {
		structTypeDTOs[i] = structTypeToDTO(st)
	}

	return &ProjectDataDTO{
		Version:         3,
		Servers:         servers,
		Scripts:         scripts,
		MonitoringItems: monitoringItems,
		StructTypes:     structTypeDTOs,
	}
}

// ImportProject はプロジェクト全体のデータをインポートする
func (s *PLCService) ImportProject(data *ProjectDataDTO) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 実行中のスクリプトを全て停止
	if s.scriptEngine != nil {
		s.scriptEngine.StopAll()
	}

	// 全サーバーを停止・削除
	for _, inst := range s.servers {
		if inst.server != nil {
			inst.server.Stop()
		}
		if ds, ok := inst.dataStore.(*adapter.VariableBackedDataStore); ok {
			ds.Detach()
		}
	}
	s.servers = make(map[protocol.ProtocolType]*serverInstance)

	// 構造体型を復元（変数より先に復元する必要がある）
	if data.StructTypes != nil {
		for _, stDTO := range data.StructTypes {
			fields := make([]variable.StructField, len(stDTO.Fields))
			for i, f := range stDTO.Fields {
				fields[i] = variable.StructField{
					Name:     f.Name,
					DataType: variable.DataType(f.DataType),
				}
			}
			st, err := variable.NewStructTypeDef(stDTO.Name, fields, s.variableStore)
			if err != nil {
				continue
			}
			s.variableStore.RegisterStructType(st)
		}
	}

	if data.Version >= 3 && len(data.Servers) > 0 {
		// Version 3以降: Serversフィールドを使用
		for _, snap := range data.Servers {
			s.mu.Unlock()
			err := s.AddServer(snap.ProtocolType, snap.Variant)
			s.mu.Lock()
			if err != nil {
				return err
			}

			inst := s.servers[protocol.ProtocolType(snap.ProtocolType)]

			// 設定を更新
			if snap.Settings != nil && inst.factory != nil {
				newConfig, err := inst.factory.MapToConfig(snap.Variant, snap.Settings)
				if err == nil {
					if err := inst.server.UpdateConfig(newConfig); err == nil {
						inst.config = newConfig
					}
				}
			}

			// メモリデータを復元
			if snap.MemorySnapshot != nil && inst.dataStore != nil {
				inst.dataStore.Restore(snap.MemorySnapshot)
			}

			// UnitID設定を復元
			if snap.UnitIDSettings != nil {
				type unitIDSupporter interface {
					SetDisabledUnitIDs(ids []uint8)
				}
				if us, ok := inst.server.(unitIDSupporter); ok {
					uint8Ids := make([]uint8, len(snap.UnitIDSettings.DisabledIDs))
					for i, id := range snap.UnitIDSettings.DisabledIDs {
						uint8Ids[i] = uint8(id)
					}
					us.SetDisabledUnitIDs(uint8Ids)
				}
			}
		}
	} else {
		// Version <= 2 後方互換
		protocolType := data.ProtocolType
		variant := data.Variant
		if protocolType == "" {
			protocolType = "modbus"
			variant = "tcp"
		}

		s.mu.Unlock()
		err := s.AddServer(protocolType, variant)
		s.mu.Lock()
		if err != nil {
			return err
		}

		inst := s.servers[protocol.ProtocolType(protocolType)]

		if data.Settings != nil && inst.factory != nil {
			newConfig, err := inst.factory.MapToConfig(variant, data.Settings)
			if err == nil {
				if err := inst.server.UpdateConfig(newConfig); err == nil {
					inst.config = newConfig
				}
			}
		}

		if data.MemorySnapshot != nil && inst.dataStore != nil {
			inst.dataStore.Restore(data.MemorySnapshot)
		}

		if data.UnitIDSettings != nil {
			type unitIDSupporter interface {
				SetDisabledUnitIDs(ids []uint8)
			}
			if us, ok := inst.server.(unitIDSupporter); ok {
				uint8Ids := make([]uint8, len(data.UnitIDSettings.DisabledIDs))
				for i, id := range data.UnitIDSettings.DisabledIDs {
					uint8Ids[i] = uint8(id)
				}
				us.SetDisabledUnitIDs(uint8Ids)
			}
		}
	}

	// スクリプトを設定
	if data.Scripts != nil {
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

	// モニタリング項目を設定
	if data.MonitoringItems != nil {
		s.monitoringItems = make(map[string]*MonitoringItemDTO)
		for _, item := range data.MonitoringItems {
			s.monitoringItems[item.ID] = item
		}
	}

	return nil
}

// === モニタリング管理 ===

// GetMonitoringItems はモニタリング項目一覧をOrder順で返す
func (s *PLCService) GetMonitoringItems() []*MonitoringItemDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*MonitoringItemDTO, 0, len(s.monitoringItems))
	for _, item := range s.monitoringItems {
		result = append(result, item)
	}

	// Order順にソート
	sort.Slice(result, func(i, j int) bool {
		return result[i].Order < result[j].Order
	})

	return result
}

// getNextOrder は次のOrder値を返す（ロック済み前提）
func (s *PLCService) getNextOrder() int {
	maxOrder := 0
	for _, item := range s.monitoringItems {
		if item.Order > maxOrder {
			maxOrder = item.Order
		}
	}
	return maxOrder + 1
}

// AddMonitoringItem はモニタリング項目を追加する
func (s *PLCService) AddMonitoringItem(item *MonitoringItemDTO) (*MonitoringItemDTO, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// IDを生成
	item.ID = uuid.New().String()
	// Orderを設定
	item.Order = s.getNextOrder()
	s.monitoringItems[item.ID] = item

	// 自動保存
	go s.saveMonitoringConfigInternal()

	return item, nil
}

// MoveMonitoringItem はモニタリング項目を移動する（fromIndex → toIndex）
func (s *PLCService) MoveMonitoringItem(id string, direction string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 現在の項目を取得
	target, ok := s.monitoringItems[id]
	if !ok {
		return fmt.Errorf("monitoring item not found: %s", id)
	}

	// 全項目をOrder順にソート
	items := make([]*MonitoringItemDTO, 0, len(s.monitoringItems))
	for _, item := range s.monitoringItems {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Order < items[j].Order
	})

	// 現在のインデックスを探す
	currentIndex := -1
	for i, item := range items {
		if item.ID == id {
			currentIndex = i
			break
		}
	}

	if currentIndex == -1 {
		return fmt.Errorf("item not found in sorted list")
	}

	// 移動先インデックスを計算
	var swapIndex int
	if direction == "up" {
		if currentIndex == 0 {
			return nil // すでに先頭
		}
		swapIndex = currentIndex - 1
	} else if direction == "down" {
		if currentIndex == len(items)-1 {
			return nil // すでに末尾
		}
		swapIndex = currentIndex + 1
	} else {
		return fmt.Errorf("invalid direction: %s", direction)
	}

	// Orderを入れ替え
	target.Order, items[swapIndex].Order = items[swapIndex].Order, target.Order

	// 自動保存
	go s.saveMonitoringConfigInternal()

	return nil
}

// ReorderMonitoringItem はモニタリング項目を指定したインデックスに移動する
func (s *PLCService) ReorderMonitoringItem(id string, newIndex int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 項目の存在確認
	if _, ok := s.monitoringItems[id]; !ok {
		return fmt.Errorf("monitoring item not found: %s", id)
	}

	// 全項目をOrder順にソート
	items := make([]*MonitoringItemDTO, 0, len(s.monitoringItems))
	for _, item := range s.monitoringItems {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Order < items[j].Order
	})

	// 現在のインデックスを探す
	currentIndex := -1
	for i, item := range items {
		if item.ID == id {
			currentIndex = i
			break
		}
	}

	if currentIndex == -1 {
		return fmt.Errorf("item not found in sorted list")
	}

	// 範囲チェック
	if newIndex < 0 {
		newIndex = 0
	}
	if newIndex >= len(items) {
		newIndex = len(items) - 1
	}

	// 同じ位置なら何もしない
	if currentIndex == newIndex {
		return nil
	}

	// 項目を新しい位置に移動（配列操作）
	item := items[currentIndex]
	// 元の位置から削除
	items = append(items[:currentIndex], items[currentIndex+1:]...)
	// 新しい位置に挿入
	items = append(items[:newIndex], append([]*MonitoringItemDTO{item}, items[newIndex:]...)...)

	// Orderを再割り当て
	for i, item := range items {
		item.Order = i
	}

	// 自動保存
	go s.saveMonitoringConfigInternal()

	return nil
}

// UpdateMonitoringItem はモニタリング項目を更新する
func (s *PLCService) UpdateMonitoringItem(item *MonitoringItemDTO) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.monitoringItems[item.ID]; !ok {
		return fmt.Errorf("monitoring item not found: %s", item.ID)
	}

	s.monitoringItems[item.ID] = item

	// 自動保存
	go s.saveMonitoringConfigInternal()

	return nil
}

// DeleteMonitoringItem はモニタリング項目を削除する
func (s *PLCService) DeleteMonitoringItem(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.monitoringItems[id]; !ok {
		return fmt.Errorf("monitoring item not found: %s", id)
	}

	delete(s.monitoringItems, id)

	// 自動保存
	go s.saveMonitoringConfigInternal()

	return nil
}

// ClearMonitoringItems は全モニタリング項目を削除する
func (s *PLCService) ClearMonitoringItems() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.monitoringItems = make(map[string]*MonitoringItemDTO)

	// 自動保存
	go s.saveMonitoringConfigInternal()
}

// getMonitoringConfigPath はモニタリング設定ファイルのパスを返す
func getMonitoringConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(configDir, "PLCSimulator")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	return filepath.Join(dir, "monitoring_config.json"), nil
}

// SaveMonitoringConfig はモニタリング設定をファイルに保存する
func (s *PLCService) SaveMonitoringConfig() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.saveMonitoringConfigInternal()
}

// saveMonitoringConfigInternal は内部保存処理（ロック不要）
func (s *PLCService) saveMonitoringConfigInternal() error {
	configPath, err := getMonitoringConfigPath()
	if err != nil {
		return err
	}

	items := make([]*MonitoringItemDTO, 0, len(s.monitoringItems))
	for _, item := range s.monitoringItems {
		items = append(items, item)
	}

	config := &MonitoringConfigDTO{
		Version: 1,
		Items:   items,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// LoadMonitoringConfig はモニタリング設定をファイルから読み込む
func (s *PLCService) LoadMonitoringConfig() error {
	configPath, err := getMonitoringConfigPath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // ファイルがなければ無視
		}
		return err
	}

	var config MonitoringConfigDTO
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// デフォルトサーバーのプロトコルタイプを取得（後方互換用）
	defaultProtocol := "modbus"
	for pt := range s.servers {
		defaultProtocol = string(pt)
		break
	}

	s.monitoringItems = make(map[string]*MonitoringItemDTO)
	for _, item := range config.Items {
		// ProtocolType が空の場合はデフォルトを設定
		if item.ProtocolType == "" {
			item.ProtocolType = defaultProtocol
		}
		s.monitoringItems[item.ID] = item
	}

	return nil
}

// === OPC UA変数管理 ===

// IsOPCUAProtocol はOPC UAプロトコルがアクティブかどうかを返す
func (s *PLCService) IsOPCUAProtocol() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.servers[protocol.ProtocolOPCUA]
	return exists
}

// GetOPCUAVariables はOPC UA変数一覧を返す
func (s *PLCService) GetOPCUAVariables() []*OPCUAVariableDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()

	inst, exists := s.servers[protocol.ProtocolOPCUA]
	if !exists {
		return nil
	}

	type variableStore interface {
		GetAllVariablesDTOs() []*OPCUAVariableDTO
	}

	vs, ok := inst.dataStore.(variableStore)
	if !ok {
		return nil
	}

	return vs.GetAllVariablesDTOs()
}

// GetOPCUADataTypes はOPC UAのデータ型一覧を返す
func (s *PLCService) GetOPCUADataTypes() *OPCUADataTypesDTO {
	return &OPCUADataTypesDTO{
		Types: []OPCUADataTypeDTO{
			{ID: "BOOL", DisplayName: "BOOL", Description: "ブール値 (true/false)"},
			{ID: "SINT", DisplayName: "SINT", Description: "符号付き8ビット整数 (-128 ~ 127)"},
			{ID: "INT", DisplayName: "INT", Description: "符号付き16ビット整数 (-32768 ~ 32767)"},
			{ID: "DINT", DisplayName: "DINT", Description: "符号付き32ビット整数"},
			{ID: "USINT", DisplayName: "USINT", Description: "符号なし8ビット整数 (0 ~ 255)"},
			{ID: "UINT", DisplayName: "UINT", Description: "符号なし16ビット整数 (0 ~ 65535)"},
			{ID: "UDINT", DisplayName: "UDINT", Description: "符号なし32ビット整数"},
			{ID: "REAL", DisplayName: "REAL", Description: "32ビット浮動小数点"},
			{ID: "LREAL", DisplayName: "LREAL", Description: "64ビット浮動小数点"},
			{ID: "STRING", DisplayName: "STRING", Description: "文字列"},
		},
	}
}

// CreateOPCUAVariable はOPC UA変数を作成する
func (s *PLCService) CreateOPCUAVariable(name, dataType string, value interface{}) (*OPCUAVariableDTO, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	inst, exists := s.servers[protocol.ProtocolOPCUA]
	if !exists {
		return nil, fmt.Errorf("OPC UA protocol is not active")
	}

	type variableCreator interface {
		CreateVariable(name, dataType string, value interface{}) (*OPCUAVariableDTO, error)
	}

	vc, ok := inst.dataStore.(variableCreator)
	if !ok {
		return nil, fmt.Errorf("datastore does not support variable creation")
	}

	return vc.CreateVariable(name, dataType, value)
}

// UpdateOPCUAVariable はOPC UA変数を更新する
func (s *PLCService) UpdateOPCUAVariable(name string, value interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	inst, exists := s.servers[protocol.ProtocolOPCUA]
	if !exists {
		return fmt.Errorf("OPC UA protocol is not active")
	}

	type variableUpdater interface {
		UpdateVariable(name string, value interface{}) error
	}

	vu, ok := inst.dataStore.(variableUpdater)
	if !ok {
		return fmt.Errorf("datastore does not support variable update")
	}

	return vu.UpdateVariable(name, value)
}

// DeleteOPCUAVariable はOPC UA変数を削除する
func (s *PLCService) DeleteOPCUAVariable(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	inst, exists := s.servers[protocol.ProtocolOPCUA]
	if !exists {
		return fmt.Errorf("OPC UA protocol is not active")
	}

	type variableDeleter interface {
		DeleteVariableByName(name string) error
	}

	vd, ok := inst.dataStore.(variableDeleter)
	if !ok {
		return fmt.Errorf("datastore does not support variable deletion")
	}

	return vd.DeleteVariableByName(name)
}

// === 変数管理API ===

// GetVariableStore は変数ストアを返す
func (s *PLCService) GetVariableStore() *variable.VariableStore {
	return s.variableStore
}

// GetVariables はすべての変数を返す
func (s *PLCService) GetVariables() []*VariableDTO {
	vars := s.variableStore.GetAllVariables()
	result := make([]*VariableDTO, len(vars))
	for i, v := range vars {
		result[i] = s.variableToDTO(v)
	}
	// 名前順でソート
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// CreateVariable は新しい変数を作成する
func (s *PLCService) CreateVariable(name, dataType string, initialValue interface{}) (*VariableDTO, error) {
	dt := variable.DataType(dataType)
	v, err := s.variableStore.CreateVariable(name, dt, initialValue)
	if err != nil {
		return nil, err
	}
	return s.variableToDTO(v), nil
}

// UpdateVariableValue は変数の値を更新する
func (s *PLCService) UpdateVariableValue(id string, value interface{}) error {
	return s.variableStore.UpdateValue(id, value)
}

// DeleteVariable は変数を削除する
func (s *PLCService) DeleteVariable(id string) error {
	return s.variableStore.DeleteVariable(id)
}

// GetVariableMappings は変数のマッピングを返す
func (s *PLCService) GetVariableMappings(id string) ([]ProtocolMappingDTO, error) {
	mappings := s.variableStore.GetMappings(id)
	result := make([]ProtocolMappingDTO, len(mappings))
	for i, m := range mappings {
		result[i] = ProtocolMappingDTO{
			ProtocolType: m.ProtocolType,
			MemoryArea:   m.MemoryArea,
			Address:      int(m.Address),
			Endianness:   m.Endianness,
		}
	}
	return result, nil
}

// UpdateVariableMappings は変数のマッピングを更新する
func (s *PLCService) UpdateVariableMappings(id string, mappingDTOs []ProtocolMappingDTO) error {
	mappings := make([]variable.ProtocolMapping, len(mappingDTOs))
	for i, dto := range mappingDTOs {
		mappings[i] = variable.ProtocolMapping{
			ProtocolType: dto.ProtocolType,
			MemoryArea:   dto.MemoryArea,
			Address:      uint32(dto.Address),
			Endianness:   dto.Endianness,
		}
	}
	return s.variableStore.SetMappings(id, mappings)
}

// GetDataTypes はデータ型一覧を返す
func (s *PLCService) GetDataTypes() *DataTypesDTO {
	types := variable.AllDataTypes()
	result := make([]DataTypeInfoDTO, 0, len(types))
	for _, dt := range types {
		// STRING は STRING[n] としてUIで動的生成するため除外
		if dt == variable.TypeSTRING {
			continue
		}
		result = append(result, DataTypeInfoDTO{
			ID:          string(dt),
			DisplayName: string(dt),
			Description: dataTypeDescription(dt),
			WordCount:   dt.WordCount(),
		})
	}

	// 構造体型情報を含める
	structTypes := s.variableStore.GetAllStructTypes()
	structDTOs := make([]StructTypeDTO, len(structTypes))
	for i, st := range structTypes {
		structDTOs[i] = structTypeToDTO(st)
	}

	return &DataTypesDTO{Types: result, StructTypes: structDTOs}
}

// === 構造体型管理 ===

// RegisterStructType は構造体型を登録する
func (s *PLCService) RegisterStructType(dto StructTypeDTO) (*StructTypeDTO, error) {
	fields := make([]variable.StructField, len(dto.Fields))
	for i, f := range dto.Fields {
		fields[i] = variable.StructField{
			Name:     f.Name,
			DataType: variable.DataType(f.DataType),
		}
	}

	st, err := variable.NewStructTypeDef(dto.Name, fields, s.variableStore)
	if err != nil {
		return nil, err
	}

	if err := s.variableStore.RegisterStructType(st); err != nil {
		return nil, err
	}

	result := structTypeToDTO(st)
	return &result, nil
}

// GetStructTypes は全構造体型を返す
func (s *PLCService) GetStructTypes() []StructTypeDTO {
	structTypes := s.variableStore.GetAllStructTypes()
	result := make([]StructTypeDTO, len(structTypes))
	for i, st := range structTypes {
		result[i] = structTypeToDTO(st)
	}
	return result
}

// DeleteStructType は構造体型を削除する
func (s *PLCService) DeleteStructType(name string) error {
	return s.variableStore.DeleteStructType(name)
}

// structTypeToDTO は構造体型をDTOに変換する
func structTypeToDTO(st *variable.StructTypeDef) StructTypeDTO {
	fields := make([]StructFieldDTO, len(st.Fields))
	for i, f := range st.Fields {
		fields[i] = StructFieldDTO{
			Name:     f.Name,
			DataType: string(f.DataType),
			Offset:   f.Offset,
		}
	}
	return StructTypeDTO{
		Name:      st.Name,
		Fields:    fields,
		WordCount: st.WordCount,
	}
}

// variableToDTO は変数をDTOに変換する
func (s *PLCService) variableToDTO(v *variable.Variable) *VariableDTO {
	mappings := s.variableStore.GetMappings(v.ID)
	mappingDTOs := make([]ProtocolMappingDTO, len(mappings))
	for i, m := range mappings {
		mappingDTOs[i] = ProtocolMappingDTO{
			ProtocolType: m.ProtocolType,
			MemoryArea:   m.MemoryArea,
			Address:      int(m.Address),
			Endianness:   m.Endianness,
		}
	}
	return &VariableDTO{
		ID:       v.ID,
		Name:     v.Name,
		DataType: string(v.DataType),
		Value:    v.Value,
		Mappings: mappingDTOs,
	}
}

// dataTypeDescription はデータ型の説明を返す
func dataTypeDescription(dt variable.DataType) string {
	switch dt {
	case variable.TypeBOOL:
		return "ブール値 (1ビット)"
	case variable.TypeSINT:
		return "符号付き8ビット整数"
	case variable.TypeINT:
		return "符号付き16ビット整数"
	case variable.TypeDINT:
		return "符号付き32ビット整数"
	case variable.TypeUSINT:
		return "符号なし8ビット整数"
	case variable.TypeUINT:
		return "符号なし16ビット整数"
	case variable.TypeUDINT:
		return "符号なし32ビット整数"
	case variable.TypeREAL:
		return "32ビット浮動小数点"
	case variable.TypeLREAL:
		return "64ビット浮動小数点"
	case variable.TypeSTRING:
		return "文字列"
	case variable.TypeTIME:
		return "経過時間"
	case variable.TypeDATE:
		return "日付"
	case variable.TypeTIME_OF_DAY:
		return "時刻"
	case variable.TypeDATE_AND_TIME:
		return "日時"
	default:
		return ""
	}
}

// === 変数設定の永続化 ===

// LoadVariablesConfig は変数設定をファイルから読み込む
func (s *PLCService) LoadVariablesConfig() error {
	configPath := s.getVariablesConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil // ファイルがなければ無視
	}

	var snapshot map[string]interface{}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return fmt.Errorf("failed to parse variables config: %w", err)
	}

	return s.variableStore.Restore(snapshot)
}

// saveVariablesConfigInternal は変数設定をファイルに保存する（内部用）
func (s *PLCService) saveVariablesConfigInternal() {
	configPath := s.getVariablesConfigPath()
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("Failed to create config directory: %v\n", err)
		return
	}

	snapshot := s.variableStore.Snapshot()
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal variables config: %v\n", err)
		return
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		fmt.Printf("Failed to write variables config: %v\n", err)
	}
}

// getVariablesConfigPath は変数設定ファイルのパスを返す
func (s *PLCService) getVariablesConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = "."
	}
	return filepath.Join(configDir, "PLCSimulator", "variables_config.json")
}
