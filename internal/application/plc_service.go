package application

import (
	"context"
	"fmt"
	"sync"
	"time"

	"modbus_simulator/internal/domain/protocol"
	"modbus_simulator/internal/domain/script"
	"modbus_simulator/internal/infrastructure/scripting"

	"github.com/google/uuid"
)

// PLCService はPLCシミュレーターのメインサービス
type PLCService struct {
	mu       sync.RWMutex
	registry *protocol.Registry

	// アクティブなプロトコル（1つのみ）
	activeProtocol protocol.ProtocolType
	activeVariant  string
	factory        protocol.ServerFactory
	dataStore      protocol.DataStore
	server         protocol.ProtocolServer
	config         protocol.ProtocolConfig

	// スクリプト
	scriptEngine *scripting.ScriptEngine
	scripts      map[string]*script.Script
}

// NewPLCService は新しいPLCServiceを作成する
func NewPLCService() *PLCService {
	service := &PLCService{
		registry: protocol.DefaultRegistry,
		scripts:  make(map[string]*script.Script),
	}

	// デフォルトでModbus TCPを設定
	service.SetProtocol("modbus", "tcp")

	return service
}

// === サーバー管理 ===

// StartServer はサーバーを起動する
func (s *PLCService) StartServer() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server == nil {
		return fmt.Errorf("server not initialized")
	}
	return s.server.Start(context.Background())
}

// StopServer はサーバーを停止する
func (s *PLCService) StopServer() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server != nil {
		return s.server.Stop()
	}
	return nil
}

// GetServerStatus はサーバーのステータスを返す
func (s *PLCService) GetServerStatus() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.server != nil {
		return s.server.Status().String()
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

// GetActiveProtocol はアクティブなプロトコルタイプを返す
func (s *PLCService) GetActiveProtocol() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return string(s.activeProtocol)
}

// GetActiveVariant はアクティブなバリアントIDを返す
func (s *PLCService) GetActiveVariant() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeVariant
}

// SetProtocol はプロトコルを設定する
func (s *PLCService) SetProtocol(protocolType string, variantID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// サーバーが動作中なら停止
	if s.server != nil && s.server.Status() == protocol.StatusRunning {
		return fmt.Errorf("cannot change protocol while server is running")
	}

	// ファクトリーを取得
	factory, err := s.registry.Get(protocol.ProtocolType(protocolType))
	if err != nil {
		return err
	}

	// Factoryに全て任せる
	config := factory.CreateConfigFromVariant(variantID)
	dataStore := factory.CreateDataStore()
	server, err := factory.CreateServer(config, dataStore)
	if err != nil {
		return err
	}

	s.factory = factory
	s.activeProtocol = protocol.ProtocolType(protocolType)
	s.activeVariant = variantID
	s.config = config
	s.dataStore = dataStore
	s.server = server
	s.scriptEngine = scripting.NewScriptEngineWithDataStore(dataStore)

	return nil
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

// GetCurrentConfig は現在の設定を返す
func (s *PLCService) GetCurrentConfig() *ProtocolConfigDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.factory == nil || s.config == nil {
		return nil
	}

	return &ProtocolConfigDTO{
		ProtocolType: string(s.activeProtocol),
		Variant:      s.activeVariant,
		Settings:     s.factory.ConfigToMap(s.config),
	}
}

// UpdateConfig は設定を更新する
func (s *PLCService) UpdateConfig(dto *ProtocolConfigDTO) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server != nil && s.server.Status() == protocol.StatusRunning {
		return fmt.Errorf("cannot update config while server is running")
	}

	// プロトコルが変わる場合はSetProtocolを使う
	if dto.ProtocolType != string(s.activeProtocol) {
		s.mu.Unlock()
		err := s.SetProtocol(dto.ProtocolType, dto.Variant)
		s.mu.Lock()
		if err != nil {
			return err
		}
	}

	// バリアントが変わる場合
	if dto.Variant != s.activeVariant {
		config := s.factory.CreateConfigFromVariant(dto.Variant)
		server, err := s.factory.CreateServer(config, s.dataStore)
		if err != nil {
			return err
		}
		s.activeVariant = dto.Variant
		s.config = config
		s.server = server
	}

	// 設定を更新
	newConfig, err := s.factory.MapToConfig(s.activeVariant, dto.Settings)
	if err != nil {
		return err
	}

	if err := s.server.UpdateConfig(newConfig); err != nil {
		return err
	}

	s.config = newConfig
	return nil
}

// === UnitID設定 ===

// GetUnitIDSettings はUnitID設定を返す（プロトコルがサポートしない場合はnil）
func (s *PLCService) GetUnitIDSettings() *UnitIDSettingsDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.factory == nil {
		return nil
	}

	caps := s.factory.GetProtocolCapabilities()
	if !caps.SupportsUnitID {
		return nil
	}

	// サーバーからdisabled IDsを取得
	type unitIDSupporter interface {
		GetDisabledUnitIDs() []uint8
	}

	var disabledIDs []int
	if us, ok := s.server.(unitIDSupporter); ok {
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
func (s *PLCService) SetUnitIDEnabled(unitId int, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	type unitIDSupporter interface {
		SetUnitIdEnabled(unitId uint8, enabled bool)
	}

	if us, ok := s.server.(unitIDSupporter); ok {
		us.SetUnitIdEnabled(uint8(unitId), enabled)
		return nil
	}

	return fmt.Errorf("protocol does not support unit ID")
}

// GetDisabledUnitIDs は無効化されたUnitIDのリストを返す
func (s *PLCService) GetDisabledUnitIDs() []int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type unitIDSupporter interface {
		GetDisabledUnitIDs() []uint8
	}

	if us, ok := s.server.(unitIDSupporter); ok {
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
func (s *PLCService) SetDisabledUnitIDs(ids []int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	type unitIDSupporter interface {
		SetDisabledUnitIDs(ids []uint8)
	}

	if us, ok := s.server.(unitIDSupporter); ok {
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
func (s *PLCService) GetMemoryAreas() []MemoryAreaDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.dataStore == nil {
		return nil
	}

	areas := s.dataStore.GetAreas()
	result := make([]MemoryAreaDTO, len(areas))
	for i, area := range areas {
		result[i] = MemoryAreaDTO{
			ID:          area.ID,
			DisplayName: area.DisplayName,
			IsBit:       area.IsBit,
			Size:        int(area.Size),
			ReadOnly:    area.ReadOnly,
		}
	}
	return result
}

// ReadBits は指定エリアの複数ビット値を読み込む
func (s *PLCService) ReadBits(area string, address, count int) ([]bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.dataStore == nil {
		return nil, fmt.Errorf("data store not initialized")
	}
	return s.dataStore.ReadBits(area, uint32(address), uint16(count))
}

// WriteBit は指定エリアのビット値を書き込む
func (s *PLCService) WriteBit(area string, address int, value bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.dataStore == nil {
		return fmt.Errorf("data store not initialized")
	}
	return s.dataStore.WriteBit(area, uint32(address), value)
}

// ReadWords は指定エリアの複数ワード値を読み込む
func (s *PLCService) ReadWords(area string, address, count int) ([]int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.dataStore == nil {
		return nil, fmt.Errorf("data store not initialized")
	}

	vals, err := s.dataStore.ReadWords(area, uint32(address), uint16(count))
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
func (s *PLCService) WriteWord(area string, address int, value int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.dataStore == nil {
		return fmt.Errorf("data store not initialized")
	}
	return s.dataStore.WriteWord(area, uint32(address), uint16(value))
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
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.scriptEngine != nil {
		s.scriptEngine.StopAll()
	}
	if s.server != nil {
		s.server.Stop()
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

func scriptToDTO(sc *script.Script, isRunning bool) *ScriptDTO {
	return &ScriptDTO{
		ID:         sc.ID,
		Name:       sc.Name,
		Code:       sc.Code,
		IntervalMs: int(sc.Interval.Milliseconds()),
		IsRunning:  isRunning,
	}
}

// === プロジェクトExport/Import ===

// ExportProject はプロジェクト全体のデータをエクスポートする
func (s *PLCService) ExportProject() *ProjectDataDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 設定を取得
	var settings map[string]interface{}
	if s.factory != nil && s.config != nil {
		settings = s.factory.ConfigToMap(s.config)
	}

	// メモリスナップショットを取得
	var memorySnapshot map[string]interface{}
	if s.dataStore != nil {
		memorySnapshot = s.dataStore.Snapshot()
	}

	// UnitID設定を取得
	var unitIDSettings *UnitIDSettingsDTO
	if s.factory != nil {
		caps := s.factory.GetProtocolCapabilities()
		if caps.SupportsUnitID {
			type unitIDSupporter interface {
				GetDisabledUnitIDs() []uint8
			}
			if us, ok := s.server.(unitIDSupporter); ok {
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
		Version:        2, // 新バージョン
		ProtocolType:   string(s.activeProtocol),
		Variant:        s.activeVariant,
		Settings:       settings,
		MemorySnapshot: memorySnapshot,
		UnitIDSettings: unitIDSettings,
		Scripts:        scripts,
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

	// プロトコルを設定
	protocolType := data.ProtocolType
	variant := data.Variant

	// 古い形式の互換性対応
	if protocolType == "" {
		protocolType = "modbus"
		variant = "tcp"
	}

	// プロトコル変更（ロックを一時解除）
	s.mu.Unlock()
	if err := s.SetProtocol(protocolType, variant); err != nil {
		s.mu.Lock()
		return err
	}
	s.mu.Lock()

	// 設定を更新
	if data.Settings != nil && s.factory != nil {
		newConfig, err := s.factory.MapToConfig(variant, data.Settings)
		if err != nil {
			return err
		}
		if err := s.server.UpdateConfig(newConfig); err != nil {
			return err
		}
		s.config = newConfig
	}

	// メモリデータを復元
	if data.MemorySnapshot != nil && s.dataStore != nil {
		if err := s.dataStore.Restore(data.MemorySnapshot); err != nil {
			return err
		}
	}

	// UnitID設定を復元
	if data.UnitIDSettings != nil {
		type unitIDSupporter interface {
			SetDisabledUnitIDs(ids []uint8)
		}
		if us, ok := s.server.(unitIDSupporter); ok {
			uint8Ids := make([]uint8, len(data.UnitIDSettings.DisabledIDs))
			for i, id := range data.UnitIDSettings.DisabledIDs {
				uint8Ids[i] = uint8(id)
			}
			us.SetDisabledUnitIDs(uint8Ids)
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

	return nil
}
