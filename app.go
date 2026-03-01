package main

import (
	"context"
	"encoding/json"
	"os"
	"sort"

	"modbus_simulator/internal/application"
	"modbus_simulator/internal/domain/protocol"

	// プロトコル実装をレジストリに登録するためのブランクインポート
	_ "modbus_simulator/internal/infrastructure/modbus"
	_ "modbus_simulator/internal/infrastructure/opcua"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go.bug.st/serial"
)

// App struct
type App struct {
	ctx        context.Context
	plcService *application.PLCService
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		plcService: application.NewPLCService(),
	}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// イベントエミッターを設定
	emitter := protocol.NewWailsEventEmitter(ctx)
	a.plcService.SetEventEmitter(emitter)
}

// shutdown is called when the app closes
func (a *App) shutdown(ctx context.Context) {
	a.plcService.Shutdown()
}

// === サーバーインスタンス管理 ===

// GetServerInstances はサーバーインスタンス一覧を返す
func (a *App) GetServerInstances() []application.ServerInstanceDTO {
	return a.plcService.GetServerInstances()
}

// AddServer は新しいサーバーインスタンスを追加する
func (a *App) AddServer(protocolType string, variantID string) error {
	return a.plcService.AddServer(protocolType, variantID)
}

// RemoveServer はサーバーインスタンスを削除する
func (a *App) RemoveServer(protocolType string) error {
	return a.plcService.RemoveServer(protocolType)
}

// === サーバー管理 ===

// StartServer はサーバーを起動する
func (a *App) StartServer(protocolType string) error {
	return a.plcService.StartServer(protocolType)
}

// StopServer はサーバーを停止する
func (a *App) StopServer(protocolType string) error {
	return a.plcService.StopServer(protocolType)
}

// GetServerStatus はサーバーのステータスを返す
func (a *App) GetServerStatus(protocolType string) string {
	return a.plcService.GetServerStatus(protocolType)
}

// === プロトコル管理API ===

// GetAvailableProtocols は利用可能なプロトコル一覧を返す
func (a *App) GetAvailableProtocols() []application.ProtocolInfoDTO {
	return a.plcService.GetAvailableProtocols()
}

// GetProtocolSchema はプロトコルスキーマを返す
func (a *App) GetProtocolSchema(protocolType string) (*application.ProtocolSchemaDTO, error) {
	return a.plcService.GetProtocolSchema(protocolType)
}

// GetServerConfig は指定サーバーの現在の設定を返す
func (a *App) GetServerConfig(protocolType string) *application.ServerConfigDTO {
	return a.plcService.GetServerConfig(protocolType)
}

// UpdateServerConfig はサーバーの設定を更新する
func (a *App) UpdateServerConfig(dto *application.ServerConfigDTO) error {
	return a.plcService.UpdateServerConfig(dto)
}

// === UnitID設定API ===

// GetUnitIDSettings はUnitID設定を返す
func (a *App) GetUnitIDSettings(protocolType string) *application.UnitIDSettingsDTO {
	return a.plcService.GetUnitIDSettings(protocolType)
}

// SetUnitIDEnabled は指定したUnitIdの応答を有効/無効にする
func (a *App) SetUnitIDEnabled(protocolType string, unitId int, enabled bool) error {
	return a.plcService.SetUnitIDEnabled(protocolType, unitId, enabled)
}

// GetDisabledUnitIDs は無効化されたUnitIDのリストを返す
func (a *App) GetDisabledUnitIDs(protocolType string) []int {
	return a.plcService.GetDisabledUnitIDs(protocolType)
}

// SetDisabledUnitIDs は無効化するUnitIDのリストを設定する
func (a *App) SetDisabledUnitIDs(protocolType string, ids []int) error {
	return a.plcService.SetDisabledUnitIDs(protocolType, ids)
}

// === 汎用メモリ操作API ===

// GetMemoryAreas は利用可能なメモリエリアの一覧を返す
func (a *App) GetMemoryAreas(protocolType string) []application.MemoryAreaDTO {
	return a.plcService.GetMemoryAreas(protocolType)
}

// ReadBits は指定エリアの複数ビット値を読み込む
func (a *App) ReadBits(protocolType, area string, address, count int) ([]bool, error) {
	return a.plcService.ReadBits(protocolType, area, address, count)
}

// WriteBit は指定エリアのビット値を書き込む
func (a *App) WriteBit(protocolType, area string, address int, value bool) error {
	return a.plcService.WriteBit(protocolType, area, address, value)
}

// ReadWords は指定エリアの複数ワード値を読み込む
func (a *App) ReadWords(protocolType, area string, address, count int) ([]int, error) {
	return a.plcService.ReadWords(protocolType, area, address, count)
}

// WriteWord は指定エリアのワード値を書き込む
func (a *App) WriteWord(protocolType, area string, address int, value int) error {
	return a.plcService.WriteWord(protocolType, area, address, value)
}

// === スクリプト管理 ===

// CreateScript は新しいスクリプトを作成する
func (a *App) CreateScript(name, code string, intervalMs int) (*application.ScriptDTO, error) {
	return a.plcService.CreateScript(name, code, intervalMs)
}

// UpdateScript はスクリプトを更新する
func (a *App) UpdateScript(id, name, code string, intervalMs int) error {
	return a.plcService.UpdateScript(id, name, code, intervalMs)
}

// DeleteScript はスクリプトを削除する
func (a *App) DeleteScript(id string) error {
	return a.plcService.DeleteScript(id)
}

// GetScripts は全てのスクリプトを取得する
func (a *App) GetScripts() []*application.ScriptDTO {
	return a.plcService.GetScripts()
}

// GetScript は特定のスクリプトを取得する
func (a *App) GetScript(id string) (*application.ScriptDTO, error) {
	return a.plcService.GetScript(id)
}

// StartScript はスクリプトを開始する
func (a *App) StartScript(id string) error {
	return a.plcService.StartScript(id)
}

// StopScript はスクリプトを停止する
func (a *App) StopScript(id string) error {
	return a.plcService.StopScript(id)
}

// RunScriptOnce はスクリプトを1回だけ実行する
func (a *App) RunScriptOnce(code string) (interface{}, error) {
	return a.plcService.RunScriptOnce(code)
}

// ClearScriptError はスクリプトのエラー情報をクリアする
func (a *App) ClearScriptError(id string) {
	a.plcService.ClearScriptError(id)
}

// GetConsoleLogs はスクリプトのコンソールログを返す
func (a *App) GetConsoleLogs() []application.ConsoleLogDTO {
	return a.plcService.GetConsoleLogs()
}

// ClearConsoleLogs はコンソールログをクリアする
func (a *App) ClearConsoleLogs() {
	a.plcService.ClearConsoleLogs()
}

// GetIntervalPresets は周期プリセットを取得する
func (a *App) GetIntervalPresets() []application.IntervalPresetDTO {
	return a.plcService.GetIntervalPresets()
}

// === プロジェクト管理 ===

// ExportProject はプロジェクトをファイルにエクスポートする
func (a *App) ExportProject() error {
	// ファイル保存ダイアログを表示
	filepath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "プロジェクトをエクスポート",
		DefaultFilename: "project.json",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON Files (*.json)", Pattern: "*.json"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return err
	}
	if filepath == "" {
		return nil // キャンセルされた
	}

	// プロジェクトデータを取得
	data := a.plcService.ExportProject()

	// JSONに変換
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	// ファイルに書き込み
	return os.WriteFile(filepath, jsonData, 0644)
}

// ImportProject はファイルからプロジェクトをインポートする
func (a *App) ImportProject() error {
	// ファイル選択ダイアログを表示
	filepath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "プロジェクトをインポート",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON Files (*.json)", Pattern: "*.json"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return err
	}
	if filepath == "" {
		return nil // キャンセルされた
	}

	// ファイルを読み込み
	jsonData, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}

	// JSONをパース
	var data application.ProjectDataDTO
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return err
	}

	// プロジェクトをインポート
	return a.plcService.ImportProject(&data)
}

// === モニタリング管理 ===

// GetMonitoringItems はモニタリング項目一覧を返す
func (a *App) GetMonitoringItems() []*application.MonitoringItemDTO {
	return a.plcService.GetMonitoringItems()
}

// AddMonitoringItem はモニタリング項目を追加する
func (a *App) AddMonitoringItem(item *application.MonitoringItemDTO) (*application.MonitoringItemDTO, error) {
	return a.plcService.AddMonitoringItem(item)
}

// UpdateMonitoringItem はモニタリング項目を更新する
func (a *App) UpdateMonitoringItem(item *application.MonitoringItemDTO) error {
	return a.plcService.UpdateMonitoringItem(item)
}

// DeleteMonitoringItem はモニタリング項目を削除する
func (a *App) DeleteMonitoringItem(id string) error {
	return a.plcService.DeleteMonitoringItem(id)
}

// MoveMonitoringItem はモニタリング項目を移動する
func (a *App) MoveMonitoringItem(id string, direction string) error {
	return a.plcService.MoveMonitoringItem(id, direction)
}

// ReorderMonitoringItem はモニタリング項目を指定したインデックスに移動する
func (a *App) ReorderMonitoringItem(id string, newIndex int) error {
	return a.plcService.ReorderMonitoringItem(id, newIndex)
}

// ClearMonitoringItems は全モニタリング項目を削除する
func (a *App) ClearMonitoringItems() {
	a.plcService.ClearMonitoringItems()
}

// === シリアルポート ===

// GetSerialPorts はシステムで利用可能なシリアルポートの一覧を返す
func (a *App) GetSerialPorts() []string {
	ports, err := serial.GetPortsList()
	if err != nil {
		return []string{}
	}
	sort.Strings(ports)
	return ports
}

// === 変数管理 ===

// GetVariables はすべての変数を返す
func (a *App) GetVariables() []*application.VariableDTO {
	return a.plcService.GetVariables()
}

// CreateVariable は新しい変数を作成する
func (a *App) CreateVariable(name, dataType string, value interface{}) (*application.VariableDTO, error) {
	return a.plcService.CreateVariable(name, dataType, value)
}

// UpdateVariableValue は変数の値を更新する
func (a *App) UpdateVariableValue(id string, value interface{}) error {
	return a.plcService.UpdateVariableValue(id, value)
}

// DeleteVariable は変数を削除する
func (a *App) DeleteVariable(id string) error {
	return a.plcService.DeleteVariable(id)
}

// UpdateVariable は変数の名前とデータタイプを更新する
func (a *App) UpdateVariable(id, name, dataType string) (*application.VariableDTO, error) {
	return a.plcService.UpdateVariable(id, name, dataType)
}

// GetDataTypes はデータ型一覧を返す
func (a *App) GetDataTypes() *application.DataTypesDTO {
	return a.plcService.GetDataTypes()
}

// GetVariableMappings は変数のマッピングを返す
func (a *App) GetVariableMappings(id string) ([]application.ProtocolMappingDTO, error) {
	return a.plcService.GetVariableMappings(id)
}

// UpdateVariableMappings は変数のマッピングを更新する
func (a *App) UpdateVariableMappings(id string, mappings []application.ProtocolMappingDTO) error {
	return a.plcService.UpdateVariableMappings(id, mappings)
}

// UpdateVariableNodePublishing は変数のプロトコル公開設定を更新する
func (a *App) UpdateVariableNodePublishing(variableID, protocolType string, dto *application.NodePublishingDTO) error {
	return a.plcService.UpdateVariableNodePublishing(variableID, protocolType, dto)
}

// === 構造体型管理 ===

// RegisterStructType は構造体型を登録する
func (a *App) RegisterStructType(dto application.StructTypeDTO) (*application.StructTypeDTO, error) {
	return a.plcService.RegisterStructType(dto)
}

// GetStructTypes は全構造体型を返す
func (a *App) GetStructTypes() []application.StructTypeDTO {
	return a.plcService.GetStructTypes()
}

// DeleteStructType は構造体型を削除する
func (a *App) DeleteStructType(name string) error {
	return a.plcService.DeleteStructType(name)
}

