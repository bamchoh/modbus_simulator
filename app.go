package main

import (
	"context"

	"modbus_simulator/internal/application"
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
