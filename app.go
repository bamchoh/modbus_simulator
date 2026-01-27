package main

import (
	"context"
	"encoding/json"
	"os"

	"modbus_simulator/internal/application"

	"github.com/wailsapp/wails/v2/pkg/runtime"
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

// StartServer はModbusサーバーを起動する
func (a *App) StartServer() error {
	return a.plcService.StartServer()
}

// StopServer はModbusサーバーを停止する
func (a *App) StopServer() error {
	return a.plcService.StopServer()
}

// GetServerStatus はサーバーのステータスを返す
func (a *App) GetServerStatus() string {
	return a.plcService.GetServerStatus()
}

// GetServerConfig はサーバーの設定を返す
func (a *App) GetServerConfig() *application.ServerConfigDTO {
	return a.plcService.GetServerConfig()
}

// UpdateServerConfig はサーバーの設定を更新する
func (a *App) UpdateServerConfig(dto *application.ServerConfigDTO) error {
	return a.plcService.UpdateServerConfig(dto)
}

// SetUnitIdEnabled は指定したUnitIdの応答を有効/無効にする
func (a *App) SetUnitIdEnabled(unitId int, enabled bool) {
	a.plcService.SetUnitIdEnabled(unitId, enabled)
}

// IsUnitIdEnabled は指定したUnitIdが応答するかどうかを返す
func (a *App) IsUnitIdEnabled(unitId int) bool {
	return a.plcService.IsUnitIdEnabled(unitId)
}

// GetDisabledUnitIDs は無効化されたUnitIDのリストを返す
func (a *App) GetDisabledUnitIDs() []int {
	return a.plcService.GetDisabledUnitIDs()
}

// SetDisabledUnitIDs は無効化するUnitIDのリストを設定する
func (a *App) SetDisabledUnitIDs(ids []int) {
	a.plcService.SetDisabledUnitIDs(ids)
}

// === レジスタ操作 ===

// GetCoils はコイルの値を取得する
func (a *App) GetCoils(start, count int) []bool {
	return a.plcService.GetCoils(start, count)
}

// SetCoil はコイルの値を設定する
func (a *App) SetCoil(address int, value bool) error {
	return a.plcService.SetCoil(address, value)
}

// GetDiscreteInputs はディスクリート入力の値を取得する
func (a *App) GetDiscreteInputs(start, count int) []bool {
	return a.plcService.GetDiscreteInputs(start, count)
}

// SetDiscreteInput はディスクリート入力の値を設定する
func (a *App) SetDiscreteInput(address int, value bool) error {
	return a.plcService.SetDiscreteInput(address, value)
}

// GetHoldingRegisters は保持レジスタの値を取得する
func (a *App) GetHoldingRegisters(start, count int) []int {
	return a.plcService.GetHoldingRegisters(start, count)
}

// SetHoldingRegister は保持レジスタの値を設定する
func (a *App) SetHoldingRegister(address int, value int) error {
	return a.plcService.SetHoldingRegister(address, value)
}

// GetInputRegisters は入力レジスタの値を取得する
func (a *App) GetInputRegisters(start, count int) []int {
	return a.plcService.GetInputRegisters(start, count)
}

// SetInputRegister は入力レジスタの値を設定する
func (a *App) SetInputRegister(address int, value int) error {
	return a.plcService.SetInputRegister(address, value)
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
		Title: "プロジェクトをエクスポート",
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
