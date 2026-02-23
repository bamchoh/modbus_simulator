package scripting

import (
	"context"
	"fmt"
	"sync"
	"time"

	"modbus_simulator/internal/domain/script"
	"modbus_simulator/internal/domain/variable"

	"github.com/dop251/goja"
)

// ScriptEngine はJavaScriptスクリプトを実行するエンジン
type ScriptEngine struct {
	mu            sync.Mutex
	variableStore *variable.VariableStore
	scripts       map[string]*runningScript
}

type runningScript struct {
	script    *script.Script
	cancel    context.CancelFunc
	vm        *goja.Runtime
	lastError string
	errorAt   time.Time
}

// NewScriptEngine は新しいスクリプトエンジンを作成する
func NewScriptEngine(varStore *variable.VariableStore) *ScriptEngine {
	return &ScriptEngine{
		variableStore: varStore,
		scripts:       make(map[string]*runningScript),
	}
}

// createVM は新しいJavaScript VMを作成し、変数アクセス関数を登録する
func (e *ScriptEngine) createVM() *goja.Runtime {
	vm := goja.New()

	// コンソールオブジェクト
	console := vm.NewObject()
	console.Set("log", func(call goja.FunctionCall) goja.Value {
		args := make([]any, len(call.Arguments))
		for i, arg := range call.Arguments {
			args[i] = arg.Export()
		}
		fmt.Println(args...)
		return goja.Undefined()
	})
	vm.Set("console", console)

	// PLCオブジェクト - 変数アクセス用
	plc := vm.NewObject()

	if e.variableStore != nil {
		// readVariable(name) - 変数名で値を読む
		plc.Set("readVariable", func(name string) any {
			v, err := e.variableStore.GetVariableByName(name)
			if err != nil {
				return nil
			}
			// gojaがJavaScript numberとして扱えるようGoの標準型に変換
			return toJSCompatibleValue(v.Value)
		})

		// writeVariable(name, value) - 変数名で値を書く
		plc.Set("writeVariable", func(name string, value any) {
			e.variableStore.UpdateValueByName(name, value)
		})

		// readArrayElement(name, index) - 配列要素読み込み
		plc.Set("readArrayElement", func(name string, index int) any {
			v, err := e.variableStore.GetVariableByName(name)
			if err != nil {
				return nil
			}
			arr, ok := v.Value.([]any)
			if !ok || index < 0 || index >= len(arr) {
				return nil
			}
			return toJSCompatibleValue(arr[index])
		})

		// writeArrayElement(name, index, value) - 配列要素書き込み
		plc.Set("writeArrayElement", func(name string, index int, value any) {
			v, err := e.variableStore.GetVariableByName(name)
			if err != nil {
				return
			}
			arr, ok := v.Value.([]any)
			if !ok || index < 0 || index >= len(arr) {
				return
			}
			newArr := make([]any, len(arr))
			copy(newArr, arr)
			newArr[index] = value
			e.variableStore.UpdateValueByName(name, newArr)
		})

		// readStructField(name, fieldName) - 構造体フィールド読み込み
		plc.Set("readStructField", func(name string, fieldName string) any {
			v, err := e.variableStore.GetVariableByName(name)
			if err != nil {
				return nil
			}
			m, ok := v.Value.(map[string]any)
			if !ok {
				return nil
			}
			val, exists := m[fieldName]
			if !exists {
				return nil
			}
			return toJSCompatibleValue(val)
		})

		// writeStructField(name, fieldName, value) - 構造体フィールド書き込み
		plc.Set("writeStructField", func(name string, fieldName string, value any) {
			v, err := e.variableStore.GetVariableByName(name)
			if err != nil {
				return
			}
			m, ok := v.Value.(map[string]any)
			if !ok {
				return
			}
			newMap := make(map[string]any)
			for k, val := range m {
				newMap[k] = val
			}
			newMap[fieldName] = value
			e.variableStore.UpdateValueByName(name, newMap)
		})

		// getVariables() - 全変数名一覧を取得
		plc.Set("getVariables", func() []string {
			vars := e.variableStore.GetAllVariables()
			names := make([]string, len(vars))
			for i, v := range vars {
				names[i] = v.Name
			}
			return names
		})
	}

	// TIME/DATE型シンタックスシュガー（変数の読み書きをワンステップで）

	if e.variableStore != nil {
		// readTimeMs(name) -> ミリ秒(number)
		plc.Set("readTimeMs", func(name string) any {
			v, err := e.variableStore.GetVariableByName(name)
			if err != nil {
				return nil
			}
			s, ok := v.Value.(string)
			if !ok {
				return nil
			}
			ms, err := variable.ParseTIME(s)
			if err != nil {
				return nil
			}
			return int64(ms)
		})
		// writeTimeMs(name, ms)
		plc.Set("writeTimeMs", func(name string, ms int64) {
			e.variableStore.UpdateValueByName(name, variable.FormatTIME(int32(ms)))
		})

		// readDateSec(name) -> Unix秒(number)
		plc.Set("readDateSec", func(name string) any {
			v, err := e.variableStore.GetVariableByName(name)
			if err != nil {
				return nil
			}
			s, ok := v.Value.(string)
			if !ok {
				return nil
			}
			sec, err := variable.ParseDATE(s)
			if err != nil {
				return nil
			}
			return int64(sec)
		})
		// writeDateSec(name, sec)
		plc.Set("writeDateSec", func(name string, sec int64) {
			e.variableStore.UpdateValueByName(name, variable.FormatDATE(uint64(sec)))
		})

		// readTimeOfDayMs(name) -> ミリ秒(number)
		plc.Set("readTimeOfDayMs", func(name string) any {
			v, err := e.variableStore.GetVariableByName(name)
			if err != nil {
				return nil
			}
			s, ok := v.Value.(string)
			if !ok {
				return nil
			}
			ms, err := variable.ParseTIME_OF_DAY(s)
			if err != nil {
				return nil
			}
			return int64(ms)
		})
		// writeTimeOfDayMs(name, ms)
		plc.Set("writeTimeOfDayMs", func(name string, ms int64) {
			e.variableStore.UpdateValueByName(name, variable.FormatTIME_OF_DAY(uint32(ms)))
		})

		// readDateAndTimeSec(name) -> Unix秒(number)
		plc.Set("readDateAndTimeSec", func(name string) any {
			v, err := e.variableStore.GetVariableByName(name)
			if err != nil {
				return nil
			}
			s, ok := v.Value.(string)
			if !ok {
				return nil
			}
			sec, err := variable.ParseDATE_AND_TIME(s)
			if err != nil {
				return nil
			}
			return int64(sec)
		})
		// writeDateAndTimeSec(name, sec)
		plc.Set("writeDateAndTimeSec", func(name string, sec int64) {
			e.variableStore.UpdateValueByName(name, variable.FormatDATE_AND_TIME(uint64(sec)))
		})
	}

	// TIME/DATE型ユーティリティ（文字列⇔数値変換のみ）

	// parseTime("T#1h30m45s") -> ミリ秒(number)
	plc.Set("parseTime", func(s string) any {
		ms, err := variable.ParseTIME(s)
		if err != nil {
			return nil
		}
		return int64(ms)
	})
	// formatTime(ms) -> "T#1h30m45s"
	plc.Set("formatTime", func(ms int64) string {
		return variable.FormatTIME(int32(ms))
	})

	// parseDate("D#2024-01-01") -> Unix秒(number)
	plc.Set("parseDate", func(s string) any {
		sec, err := variable.ParseDATE(s)
		if err != nil {
			return nil
		}
		return int64(sec)
	})
	// formatDate(sec) -> "D#2024-01-01"
	plc.Set("formatDate", func(sec int64) string {
		return variable.FormatDATE(uint64(sec))
	})

	// parseTimeOfDay("TOD#12:30:15") -> ミリ秒(number)
	plc.Set("parseTimeOfDay", func(s string) any {
		ms, err := variable.ParseTIME_OF_DAY(s)
		if err != nil {
			return nil
		}
		return int64(ms)
	})
	// formatTimeOfDay(ms) -> "TOD#12:30:15"
	plc.Set("formatTimeOfDay", func(ms int64) string {
		return variable.FormatTIME_OF_DAY(uint32(ms))
	})

	// parseDateAndTime("DT#2024-01-01-12:30:15") -> Unix秒(number)
	plc.Set("parseDateAndTime", func(s string) any {
		sec, err := variable.ParseDATE_AND_TIME(s)
		if err != nil {
			return nil
		}
		return int64(sec)
	})
	// formatDateAndTime(sec) -> "DT#2024-01-01-12:30:15"
	plc.Set("formatDateAndTime", func(sec int64) string {
		return variable.FormatDATE_AND_TIME(uint64(sec))
	})

	vm.Set("plc", plc)

	return vm
}

// toJSCompatibleValue はGoの型をgojaが扱えるJavaScript互換の型に変換する
// int8/int16/int32 → int64, uint8/uint16/uint32 → int64, float32 → float64
func toJSCompatibleValue(value any) any {
	switch v := value.(type) {
	case int8:
		return int64(v)
	case int16:
		return int64(v)
	case int32:
		return int64(v)
	case uint8:
		return int64(v)
	case uint16:
		return int64(v)
	case uint32:
		return int64(v)
	case float32:
		return float64(v)
	default:
		return value
	}
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

	// スクリプトをIIFEでラップしてコンパイル（const/letの再宣言エラーを防止）
	wrappedCode := "(function(){\n" + s.Code + "\n})();"
	program, err := goja.Compile(s.Name, wrappedCode, false)
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
							errMsg := fmt.Sprintf("panic: %v", r)
							fmt.Printf("Script %s panicked: %v\n", s.Name, r)
							e.mu.Lock()
							if cur, ok := e.scripts[s.ID]; ok {
								cur.lastError = errMsg
								cur.errorAt = time.Now()
							}
							e.mu.Unlock()
						}
					}()
					_, runErr := vm.RunProgram(program)
					if runErr != nil {
						fmt.Printf("Script %s error: %v\n", s.Name, runErr)
						e.mu.Lock()
						if cur, ok := e.scripts[s.ID]; ok {
							cur.lastError = runErr.Error()
							cur.errorAt = time.Now()
						}
						e.mu.Unlock()
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

// GetLastError はスクリプトの最新エラー情報を返す
func (e *ScriptEngine) GetLastError(scriptID string) (string, time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()

	rs, ok := e.scripts[scriptID]
	if !ok {
		return "", time.Time{}
	}
	return rs.lastError, rs.errorAt
}

// ClearError はスクリプトのエラー情報をクリアする
func (e *ScriptEngine) ClearError(scriptID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if rs, ok := e.scripts[scriptID]; ok {
		rs.lastError = ""
		rs.errorAt = time.Time{}
	}
}

// RunOnce はスクリプトを1回だけ実行する（テスト用）
func (e *ScriptEngine) RunOnce(code string) (any, error) {
	vm := e.createVM()
	result, err := vm.RunString(code)
	if err != nil {
		return nil, err
	}
	return result.Export(), nil
}
