package scripting

import (
	"testing"
	"time"

	"modbus_simulator/internal/domain/script"
	"modbus_simulator/internal/infrastructure/modbus"
)

func TestNewScriptEngineWithDataStore(t *testing.T) {
	store := modbus.NewModbusDataStore(100, 100, 100, 100)
	engine := NewScriptEngineWithDataStore(store)
	if engine == nil {
		t.Fatal("NewScriptEngineWithDataStore returned nil")
	}
	if engine.dataStore == nil {
		t.Fatal("dataStore is nil")
	}
}

func TestScriptEngine_RunOnce(t *testing.T) {
	store := modbus.NewModbusDataStore(100, 100, 100, 100)
	engine := NewScriptEngineWithDataStore(store)

	// 単純な計算
	result, err := engine.RunOnce("1 + 2")
	if err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}
	if result != int64(3) {
		t.Errorf("expected 3, got %v (%T)", result, result)
	}
}

func TestScriptEngine_RunOnce_WithConsole(t *testing.T) {
	store := modbus.NewModbusDataStore(100, 100, 100, 100)
	engine := NewScriptEngineWithDataStore(store)

	// console.logは正常に動作するべき
	_, err := engine.RunOnce(`console.log("test message")`)
	if err != nil {
		t.Fatalf("RunOnce with console.log failed: %v", err)
	}
}

func TestScriptEngine_RunOnce_SyntaxError(t *testing.T) {
	store := modbus.NewModbusDataStore(100, 100, 100, 100)
	engine := NewScriptEngineWithDataStore(store)

	// 構文エラー
	_, err := engine.RunOnce("invalid syntax {{{")
	if err == nil {
		t.Fatal("expected syntax error")
	}
}

func TestScriptEngine_RunOnce_ReadWriteWord(t *testing.T) {
	store := modbus.NewModbusDataStore(100, 100, 100, 100)
	engine := NewScriptEngineWithDataStore(store)

	// 汎用APIでワードを書き込む
	_, err := engine.RunOnce(`plc.writeWord("holdingRegisters", 10, 1234)`)
	if err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	// DataStoreで確認
	val, _ := store.ReadWord(modbus.AreaHoldingRegs, 10)
	if val != 1234 {
		t.Errorf("expected 1234, got %d", val)
	}

	// スクリプトで読み取り
	result, err := engine.RunOnce(`plc.readWord("holdingRegisters", 10)`)
	if err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}
	if result != int64(1234) {
		t.Errorf("expected 1234, got %v", result)
	}
}

func TestScriptEngine_RunOnce_ReadWriteBit(t *testing.T) {
	store := modbus.NewModbusDataStore(100, 100, 100, 100)
	engine := NewScriptEngineWithDataStore(store)

	// 汎用APIでビットを書き込む
	_, err := engine.RunOnce(`plc.writeBit("coils", 5, true)`)
	if err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	// DataStoreで確認
	val, _ := store.ReadBit(modbus.AreaCoils, 5)
	if !val {
		t.Error("expected true, got false")
	}

	// スクリプトで読み取り
	result, err := engine.RunOnce(`plc.readBit("coils", 5)`)
	if err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}
	if result != true {
		t.Errorf("expected true, got %v", result)
	}
}

func TestScriptEngine_RunOnce_ModbusCompatMethods(t *testing.T) {
	store := modbus.NewModbusDataStore(100, 100, 100, 100)
	engine := NewScriptEngineWithDataStore(store)

	// Modbus互換API: setCoil/getCoil
	_, err := engine.RunOnce(`plc.setCoil(10, true)`)
	if err != nil {
		t.Fatalf("setCoil failed: %v", err)
	}

	val, _ := store.GetCoil(10)
	if !val {
		t.Error("expected coil[10] to be true")
	}

	result, err := engine.RunOnce(`plc.getCoil(10)`)
	if err != nil {
		t.Fatalf("getCoil failed: %v", err)
	}
	if result != true {
		t.Errorf("expected true, got %v", result)
	}

	// Modbus互換API: setHoldingRegister/getHoldingRegister
	_, err = engine.RunOnce(`plc.setHoldingRegister(20, 5678)`)
	if err != nil {
		t.Fatalf("setHoldingRegister failed: %v", err)
	}

	word, _ := store.GetHoldingRegister(20)
	if word != 5678 {
		t.Errorf("expected 5678, got %d", word)
	}

	result, err = engine.RunOnce(`plc.getHoldingRegister(20)`)
	if err != nil {
		t.Fatalf("getHoldingRegister failed: %v", err)
	}
	if result != int64(5678) {
		t.Errorf("expected 5678, got %v", result)
	}
}

func TestScriptEngine_RunOnce_GetAreas(t *testing.T) {
	store := modbus.NewModbusDataStore(100, 100, 100, 100)
	engine := NewScriptEngineWithDataStore(store)

	result, err := engine.RunOnce(`plc.getAreas()`)
	if err != nil {
		t.Fatalf("getAreas failed: %v", err)
	}

	// gojaは[]map[string]interface{}を返す
	var areas []map[string]interface{}
	switch v := result.(type) {
	case []interface{}:
		areas = make([]map[string]interface{}, len(v))
		for i, item := range v {
			areas[i] = item.(map[string]interface{})
		}
	case []map[string]interface{}:
		areas = v
	default:
		t.Fatalf("unexpected type: %T", result)
	}

	if len(areas) != 4 {
		t.Errorf("expected 4 areas, got %d", len(areas))
	}
}

func TestScriptEngine_StartStopScript(t *testing.T) {
	store := modbus.NewModbusDataStore(100, 100, 100, 100)
	engine := NewScriptEngineWithDataStore(store)

	s := script.NewScript("test-1", "counter", `
		var val = plc.readWord("holdingRegisters", 0);
		plc.writeWord("holdingRegisters", 0, val + 1);
	`, 50*time.Millisecond)

	// スクリプト開始
	err := engine.StartScript(s)
	if err != nil {
		t.Fatalf("StartScript failed: %v", err)
	}

	// 実行中であることを確認
	if !engine.IsRunning("test-1") {
		t.Error("expected script to be running")
	}

	// 少し待ってカウンターが増加していることを確認
	time.Sleep(200 * time.Millisecond)

	val, _ := store.ReadWord(modbus.AreaHoldingRegs, 0)
	if val < 2 {
		t.Errorf("expected counter >= 2, got %d", val)
	}

	// スクリプト停止
	err = engine.StopScript("test-1")
	if err != nil {
		t.Fatalf("StopScript failed: %v", err)
	}

	// 停止していることを確認
	if engine.IsRunning("test-1") {
		t.Error("expected script to be stopped")
	}
}

func TestScriptEngine_StopScript_NotFound(t *testing.T) {
	store := modbus.NewModbusDataStore(100, 100, 100, 100)
	engine := NewScriptEngineWithDataStore(store)

	err := engine.StopScript("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent script")
	}
}

func TestScriptEngine_StopAll(t *testing.T) {
	store := modbus.NewModbusDataStore(100, 100, 100, 100)
	engine := NewScriptEngineWithDataStore(store)

	// 複数のスクリプトを開始
	s1 := script.NewScript("test-1", "script1", `1+1`, 100*time.Millisecond)
	s2 := script.NewScript("test-2", "script2", `2+2`, 100*time.Millisecond)

	_ = engine.StartScript(s1)
	_ = engine.StartScript(s2)

	// 実行中であることを確認
	if len(engine.GetRunningScripts()) != 2 {
		t.Errorf("expected 2 running scripts, got %d", len(engine.GetRunningScripts()))
	}

	// 全て停止
	engine.StopAll()

	// 停止していることを確認
	if len(engine.GetRunningScripts()) != 0 {
		t.Errorf("expected 0 running scripts, got %d", len(engine.GetRunningScripts()))
	}
}

func TestScriptEngine_GetRunningScripts(t *testing.T) {
	store := modbus.NewModbusDataStore(100, 100, 100, 100)
	engine := NewScriptEngineWithDataStore(store)

	// 初期状態
	if len(engine.GetRunningScripts()) != 0 {
		t.Error("expected no running scripts initially")
	}

	// スクリプトを開始
	s := script.NewScript("test-1", "script1", `1+1`, 100*time.Millisecond)
	_ = engine.StartScript(s)

	running := engine.GetRunningScripts()
	if len(running) != 1 {
		t.Errorf("expected 1 running script, got %d", len(running))
	}
	if running[0] != "test-1" {
		t.Errorf("expected 'test-1', got '%s'", running[0])
	}

	// 停止
	_ = engine.StopScript("test-1")

	if len(engine.GetRunningScripts()) != 0 {
		t.Error("expected no running scripts after stop")
	}
}

func TestScriptEngine_StartScript_ReplaceRunning(t *testing.T) {
	store := modbus.NewModbusDataStore(100, 100, 100, 100)
	engine := NewScriptEngineWithDataStore(store)

	s1 := script.NewScript("test-1", "script1", `plc.writeWord("holdingRegisters", 0, 111)`, 50*time.Millisecond)
	s2 := script.NewScript("test-1", "script1-updated", `plc.writeWord("holdingRegisters", 0, 222)`, 50*time.Millisecond)

	// 最初のスクリプト開始
	_ = engine.StartScript(s1)
	time.Sleep(100 * time.Millisecond)

	val, _ := store.ReadWord(modbus.AreaHoldingRegs, 0)
	if val != 111 {
		t.Errorf("expected 111, got %d", val)
	}

	// 同じIDで別のスクリプトを開始（置き換え）
	_ = engine.StartScript(s2)
	time.Sleep(100 * time.Millisecond)

	val, _ = store.ReadWord(modbus.AreaHoldingRegs, 0)
	if val != 222 {
		t.Errorf("expected 222, got %d", val)
	}

	// クリーンアップ
	engine.StopAll()
}

func TestScriptEngine_StartScript_CompileError(t *testing.T) {
	store := modbus.NewModbusDataStore(100, 100, 100, 100)
	engine := NewScriptEngineWithDataStore(store)

	s := script.NewScript("test-1", "invalid", `invalid syntax {{{`, 100*time.Millisecond)

	err := engine.StartScript(s)
	if err == nil {
		t.Fatal("expected compile error")
	}
}

func TestScriptEngine_IsRunning(t *testing.T) {
	store := modbus.NewModbusDataStore(100, 100, 100, 100)
	engine := NewScriptEngineWithDataStore(store)

	// 存在しないスクリプト
	if engine.IsRunning("nonexistent") {
		t.Error("expected false for nonexistent script")
	}

	// スクリプトを開始
	s := script.NewScript("test-1", "script1", `1+1`, 100*time.Millisecond)
	_ = engine.StartScript(s)

	if !engine.IsRunning("test-1") {
		t.Error("expected true for running script")
	}

	// 停止
	_ = engine.StopScript("test-1")

	if engine.IsRunning("test-1") {
		t.Error("expected false after stop")
	}
}
