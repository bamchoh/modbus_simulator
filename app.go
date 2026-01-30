package main

import (
	"context"
	"encoding/json"
	"os"
	"sort"

	"modbus_simulator/internal/application"

	// プロトコル実装をレジストリに登録するためのブランクインポート
	_ "modbus_simulator/internal/infrastructure/fins"
	_ "modbus_simulator/internal/infrastructure/modbus"

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
}

// shutdown is called when the app closes
func (a *App) shutdown(ctx context.Context) {
	a.plcService.Shutdown()
}

// === サーバー管理 ===

// StartServer はサーバーを起動する
func (a *App) StartServer() error {
	return a.plcService.StartServer()
}

// StopServer はサーバーを停止する
func (a *App) StopServer() error {
	return a.plcService.StopServer()
}

// GetServerStatus はサーバーのステータスを返す
func (a *App) GetServerStatus() string {
	return a.plcService.GetServerStatus()
}

// === プロトコル管理API ===

// GetAvailableProtocols は利用可能なプロトコル一覧を返す
func (a *App) GetAvailableProtocols() []application.ProtocolInfoDTO {
	return a.plcService.GetAvailableProtocols()
}

// GetActiveProtocol はアクティブなプロトコルタイプを返す
func (a *App) GetActiveProtocol() string {
	return a.plcService.GetActiveProtocol()
}

// GetActiveVariant はアクティブなバリアントIDを返す
func (a *App) GetActiveVariant() string {
	return a.plcService.GetActiveVariant()
}

// SetProtocol はプロトコルを設定する
func (a *App) SetProtocol(protocolType string, variantID string) error {
	return a.plcService.SetProtocol(protocolType, variantID)
}

// GetProtocolSchema はプロトコルスキーマを返す
func (a *App) GetProtocolSchema(protocolType string) (*application.ProtocolSchemaDTO, error) {
	return a.plcService.GetProtocolSchema(protocolType)
}

// GetCurrentConfig は現在の設定を返す
func (a *App) GetCurrentConfig() *application.ProtocolConfigDTO {
	return a.plcService.GetCurrentConfig()
}

// UpdateConfig は設定を更新する
func (a *App) UpdateConfig(dto *application.ProtocolConfigDTO) error {
	return a.plcService.UpdateConfig(dto)
}

// === UnitID設定API ===

// GetUnitIDSettings はUnitID設定を返す（プロトコルがサポートしない場合はnil）
func (a *App) GetUnitIDSettings() *application.UnitIDSettingsDTO {
	return a.plcService.GetUnitIDSettings()
}

// SetUnitIDEnabled は指定したUnitIdの応答を有効/無効にする
func (a *App) SetUnitIDEnabled(unitId int, enabled bool) error {
	return a.plcService.SetUnitIDEnabled(unitId, enabled)
}

// GetDisabledUnitIDs は無効化されたUnitIDのリストを返す
func (a *App) GetDisabledUnitIDs() []int {
	return a.plcService.GetDisabledUnitIDs()
}

// SetDisabledUnitIDs は無効化するUnitIDのリストを設定する
func (a *App) SetDisabledUnitIDs(ids []int) error {
	return a.plcService.SetDisabledUnitIDs(ids)
}

// === 汎用メモリ操作API ===

// GetMemoryAreas は利用可能なメモリエリアの一覧を返す
func (a *App) GetMemoryAreas() []application.MemoryAreaDTO {
	return a.plcService.GetMemoryAreas()
}

// ReadBits は指定エリアの複数ビット値を読み込む
func (a *App) ReadBits(area string, address, count int) ([]bool, error) {
	return a.plcService.ReadBits(area, address, count)
}

// WriteBit は指定エリアのビット値を書き込む
func (a *App) WriteBit(area string, address int, value bool) error {
	return a.plcService.WriteBit(area, address, value)
}

// ReadWords は指定エリアの複数ワード値を読み込む
func (a *App) ReadWords(area string, address, count int) ([]int, error) {
	return a.plcService.ReadWords(area, address, count)
}

// WriteWord は指定エリアのワード値を書き込む
func (a *App) WriteWord(area string, address int, value int) error {
	return a.plcService.WriteWord(area, address, value)
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
