package scripting

import (
	"context"
	"fmt"
	"sync"
	"time"

	"modbus_simulator/internal/domain/register"
	"modbus_simulator/internal/domain/script"

	"github.com/dop251/goja"
)

// ScriptEngine はJavaScriptスクリプトを実行するエンジン
type ScriptEngine struct {
	mu      sync.Mutex
	store   *register.RegisterStore
	scripts map[string]*runningScript
}

type runningScript struct {
	script *script.Script
	cancel context.CancelFunc
	vm     *goja.Runtime
}

// NewScriptEngine は新しいスクリプトエンジンを作成する
func NewScriptEngine(store *register.RegisterStore) *ScriptEngine {
	return &ScriptEngine{
		store:   store,
		scripts: make(map[string]*runningScript),
	}
}

// createVM は新しいJavaScript VMを作成し、レジスタアクセス関数を登録する
func (e *ScriptEngine) createVM() *goja.Runtime {
	vm := goja.New()

	// コンソールオブジェクト
	console := vm.NewObject()
	console.Set("log", func(call goja.FunctionCall) goja.Value {
		args := make([]interface{}, len(call.Arguments))
		for i, arg := range call.Arguments {
			args[i] = arg.Export()
		}
		fmt.Println(args...)
		return goja.Undefined()
	})
	vm.Set("console", console)

	// PLCオブジェクト - レジスタアクセス用
	plc := vm.NewObject()

	// コイル操作
	plc.Set("getCoil", func(address int) bool {
		val, _ := e.store.GetCoil(uint16(address))
		return val
	})
	plc.Set("setCoil", func(address int, value bool) {
		e.store.SetCoil(uint16(address), value)
	})

	// ディスクリート入力操作
	plc.Set("getDiscreteInput", func(address int) bool {
		val, _ := e.store.GetDiscreteInput(uint16(address))
		return val
	})
	plc.Set("setDiscreteInput", func(address int, value bool) {
		e.store.SetDiscreteInput(uint16(address), value)
	})

	// 保持レジスタ操作
	plc.Set("getHoldingRegister", func(address int) int {
		val, _ := e.store.GetHoldingRegister(uint16(address))
		return int(val)
	})
	plc.Set("setHoldingRegister", func(address int, value int) {
		e.store.SetHoldingRegister(uint16(address), uint16(value))
	})

	// 入力レジスタ操作
	plc.Set("getInputRegister", func(address int) int {
		val, _ := e.store.GetInputRegister(uint16(address))
		return int(val)
	})
	plc.Set("setInputRegister", func(address int, value int) {
		e.store.SetInputRegister(uint16(address), uint16(value))
	})

	vm.Set("plc", plc)

	return vm
}

// StartScript はスクリプトを開始する
func (e *ScriptEngine) StartScript(s *script.Script) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 既に実行中の場合は停止
	if existing, ok := e.scripts[s.ID]; ok {
		existing.cancel()
		delete(e.scripts, s.ID)
	}

	vm := e.createVM()

	// スクリプトをコンパイル
	program, err := goja.Compile(s.Name, s.Code, false)
	if err != nil {
		return fmt.Errorf("failed to compile script: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	rs := &runningScript{
		script: s,
		cancel: cancel,
		vm:     vm,
	}
	e.scripts[s.ID] = rs

	// 周期実行ゴルーチン
	go func() {
		ticker := time.NewTicker(s.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				func() {
					defer func() {
						if r := recover(); r != nil {
							fmt.Printf("Script %s panicked: %v\n", s.Name, r)
						}
					}()
					_, err := vm.RunProgram(program)
					if err != nil {
						fmt.Printf("Script %s error: %v\n", s.Name, err)
					}
				}()
			}
		}
	}()

	return nil
}

// StopScript はスクリプトを停止する
func (e *ScriptEngine) StopScript(scriptID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	rs, ok := e.scripts[scriptID]
	if !ok {
		return fmt.Errorf("script not found: %s", scriptID)
	}

	rs.cancel()
	delete(e.scripts, scriptID)
	return nil
}

// StopAll は全てのスクリプトを停止する
func (e *ScriptEngine) StopAll() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for id, rs := range e.scripts {
		rs.cancel()
		delete(e.scripts, id)
	}
}

// IsRunning はスクリプトが実行中かどうかを返す
func (e *ScriptEngine) IsRunning(scriptID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	_, ok := e.scripts[scriptID]
	return ok
}

// GetRunningScripts は実行中のスクリプトIDのリストを返す
func (e *ScriptEngine) GetRunningScripts() []string {
	e.mu.Lock()
	defer e.mu.Unlock()

	ids := make([]string, 0, len(e.scripts))
	for id := range e.scripts {
		ids = append(ids, id)
	}
	return ids
}

// RunOnce はスクリプトを1回だけ実行する（テスト用）
func (e *ScriptEngine) RunOnce(code string) (interface{}, error) {
	vm := e.createVM()
	result, err := vm.RunString(code)
	if err != nil {
		return nil, err
	}
	return result.Export(), nil
}
