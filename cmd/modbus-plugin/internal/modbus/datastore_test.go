package modbus

import (
	"testing"

	"modbus_simulator/internal/domain/datastore"
)

func TestNewModbusDataStore(t *testing.T) {
	store := NewModbusDataStore(100, 50, 200, 150)
	if store == nil {
		t.Fatal("NewModbusDataStore returned nil")
	}

	if len(store.coils) != 100 {
		t.Errorf("expected 100 coils, got %d", len(store.coils))
	}
	if len(store.discreteInputs) != 50 {
		t.Errorf("expected 50 discrete inputs, got %d", len(store.discreteInputs))
	}
	if len(store.holdingRegs) != 200 {
		t.Errorf("expected 200 holding registers, got %d", len(store.holdingRegs))
	}
	if len(store.inputRegs) != 150 {
		t.Errorf("expected 150 input registers, got %d", len(store.inputRegs))
	}
}

func TestModbusDataStore_GetAreas(t *testing.T) {
	store := NewModbusDataStore(100, 50, 200, 150)
	areas := store.GetAreas()

	if len(areas) != 4 {
		t.Fatalf("expected 4 areas, got %d", len(areas))
	}

	// エリアの確認
	areaMap := make(map[string]bool)
	for _, area := range areas {
		areaMap[area.ID] = true
	}

	expectedAreas := []string{AreaCoils, AreaDiscreteInputs, AreaHoldingRegs, AreaInputRegs}
	for _, expected := range expectedAreas {
		if !areaMap[expected] {
			t.Errorf("expected area %s not found", expected)
		}
	}
}

func TestModbusDataStore_ReadWriteBit(t *testing.T) {
	store := NewModbusDataStore(100, 50, 200, 150)

	// コイルへの書き込みと読み取り
	err := store.WriteBit(AreaCoils, 10, true)
	if err != nil {
		t.Fatalf("WriteBit failed: %v", err)
	}

	val, err := store.ReadBit(AreaCoils, 10)
	if err != nil {
		t.Fatalf("ReadBit failed: %v", err)
	}
	if !val {
		t.Error("expected true, got false")
	}

	// ディスクリート入力への書き込みと読み取り
	err = store.WriteBit(AreaDiscreteInputs, 5, true)
	if err != nil {
		t.Fatalf("WriteBit failed: %v", err)
	}

	val, err = store.ReadBit(AreaDiscreteInputs, 5)
	if err != nil {
		t.Fatalf("ReadBit failed: %v", err)
	}
	if !val {
		t.Error("expected true, got false")
	}
}

func TestModbusDataStore_ReadWriteBit_OutOfRange(t *testing.T) {
	store := NewModbusDataStore(100, 50, 200, 150)

	// 範囲外アクセス
	_, err := store.ReadBit(AreaCoils, 100)
	if err != datastore.ErrAddressOutOfRange {
		t.Errorf("expected ErrAddressOutOfRange, got %v", err)
	}

	err = store.WriteBit(AreaCoils, 100, true)
	if err != datastore.ErrAddressOutOfRange {
		t.Errorf("expected ErrAddressOutOfRange, got %v", err)
	}
}

func TestModbusDataStore_ReadWriteBit_AreaNotFound(t *testing.T) {
	store := NewModbusDataStore(100, 50, 200, 150)

	// 存在しないエリア
	_, err := store.ReadBit("nonexistent", 0)
	if err != datastore.ErrAreaNotFound {
		t.Errorf("expected ErrAreaNotFound, got %v", err)
	}

	err = store.WriteBit("nonexistent", 0, true)
	if err != datastore.ErrAreaNotFound {
		t.Errorf("expected ErrAreaNotFound, got %v", err)
	}
}

func TestModbusDataStore_ReadWriteBits(t *testing.T) {
	store := NewModbusDataStore(100, 50, 200, 150)

	// 複数ビットの書き込み
	values := []bool{true, false, true, true, false}
	err := store.WriteBits(AreaCoils, 10, values)
	if err != nil {
		t.Fatalf("WriteBits failed: %v", err)
	}

	// 複数ビットの読み取り
	got, err := store.ReadBits(AreaCoils, 10, 5)
	if err != nil {
		t.Fatalf("ReadBits failed: %v", err)
	}

	for i, v := range values {
		if got[i] != v {
			t.Errorf("bit %d: expected %v, got %v", i, v, got[i])
		}
	}
}

func TestModbusDataStore_ReadWriteBits_OutOfRange(t *testing.T) {
	store := NewModbusDataStore(100, 50, 200, 150)

	// 範囲外アクセス（終了アドレスが範囲外）
	_, err := store.ReadBits(AreaCoils, 95, 10)
	if err != datastore.ErrAddressOutOfRange {
		t.Errorf("expected ErrAddressOutOfRange, got %v", err)
	}

	err = store.WriteBits(AreaCoils, 95, []bool{true, true, true, true, true, true})
	if err != datastore.ErrAddressOutOfRange {
		t.Errorf("expected ErrAddressOutOfRange, got %v", err)
	}
}

func TestModbusDataStore_ReadWriteWord(t *testing.T) {
	store := NewModbusDataStore(100, 50, 200, 150)

	// 保持レジスタへの書き込みと読み取り
	err := store.WriteWord(AreaHoldingRegs, 10, 0x1234)
	if err != nil {
		t.Fatalf("WriteWord failed: %v", err)
	}

	val, err := store.ReadWord(AreaHoldingRegs, 10)
	if err != nil {
		t.Fatalf("ReadWord failed: %v", err)
	}
	if val != 0x1234 {
		t.Errorf("expected 0x1234, got 0x%04x", val)
	}

	// 入力レジスタへの書き込みと読み取り
	err = store.WriteWord(AreaInputRegs, 5, 0xABCD)
	if err != nil {
		t.Fatalf("WriteWord failed: %v", err)
	}

	val, err = store.ReadWord(AreaInputRegs, 5)
	if err != nil {
		t.Fatalf("ReadWord failed: %v", err)
	}
	if val != 0xABCD {
		t.Errorf("expected 0xABCD, got 0x%04x", val)
	}
}

func TestModbusDataStore_ReadWriteWord_OutOfRange(t *testing.T) {
	store := NewModbusDataStore(100, 50, 200, 150)

	// 範囲外アクセス
	_, err := store.ReadWord(AreaHoldingRegs, 200)
	if err != datastore.ErrAddressOutOfRange {
		t.Errorf("expected ErrAddressOutOfRange, got %v", err)
	}

	err = store.WriteWord(AreaHoldingRegs, 200, 0x1234)
	if err != datastore.ErrAddressOutOfRange {
		t.Errorf("expected ErrAddressOutOfRange, got %v", err)
	}
}

func TestModbusDataStore_ReadWriteWord_AreaNotFound(t *testing.T) {
	store := NewModbusDataStore(100, 50, 200, 150)

	// 存在しないエリア
	_, err := store.ReadWord("nonexistent", 0)
	if err != datastore.ErrAreaNotFound {
		t.Errorf("expected ErrAreaNotFound, got %v", err)
	}

	err = store.WriteWord("nonexistent", 0, 0x1234)
	if err != datastore.ErrAreaNotFound {
		t.Errorf("expected ErrAreaNotFound, got %v", err)
	}
}

func TestModbusDataStore_ReadWriteWords(t *testing.T) {
	store := NewModbusDataStore(100, 50, 200, 150)

	// 複数ワードの書き込み
	values := []uint16{0x1111, 0x2222, 0x3333, 0x4444, 0x5555}
	err := store.WriteWords(AreaHoldingRegs, 10, values)
	if err != nil {
		t.Fatalf("WriteWords failed: %v", err)
	}

	// 複数ワードの読み取り
	got, err := store.ReadWords(AreaHoldingRegs, 10, 5)
	if err != nil {
		t.Fatalf("ReadWords failed: %v", err)
	}

	for i, v := range values {
		if got[i] != v {
			t.Errorf("word %d: expected 0x%04x, got 0x%04x", i, v, got[i])
		}
	}
}

func TestModbusDataStore_ReadWriteWords_OutOfRange(t *testing.T) {
	store := NewModbusDataStore(100, 50, 200, 150)

	// 範囲外アクセス
	_, err := store.ReadWords(AreaHoldingRegs, 195, 10)
	if err != datastore.ErrAddressOutOfRange {
		t.Errorf("expected ErrAddressOutOfRange, got %v", err)
	}

	err = store.WriteWords(AreaHoldingRegs, 195, []uint16{1, 2, 3, 4, 5, 6})
	if err != datastore.ErrAddressOutOfRange {
		t.Errorf("expected ErrAddressOutOfRange, got %v", err)
	}
}

func TestModbusDataStore_Snapshot(t *testing.T) {
	store := NewModbusDataStore(10, 10, 10, 10)

	// データを設定
	_ = store.WriteBit(AreaCoils, 0, true)
	_ = store.WriteBit(AreaDiscreteInputs, 1, true)
	_ = store.WriteWord(AreaHoldingRegs, 2, 0x1234)
	_ = store.WriteWord(AreaInputRegs, 3, 0x5678)

	// スナップショット取得
	snapshot := store.Snapshot()

	// スナップショットの確認
	coils, ok := snapshot[AreaCoils].([]bool)
	if !ok {
		t.Fatal("coils not found in snapshot")
	}
	if !coils[0] {
		t.Error("expected coil[0] to be true")
	}

	discreteInputs, ok := snapshot[AreaDiscreteInputs].([]bool)
	if !ok {
		t.Fatal("discreteInputs not found in snapshot")
	}
	if !discreteInputs[1] {
		t.Error("expected discreteInput[1] to be true")
	}

	holdingRegs, ok := snapshot[AreaHoldingRegs].([]uint16)
	if !ok {
		t.Fatal("holdingRegs not found in snapshot")
	}
	if holdingRegs[2] != 0x1234 {
		t.Errorf("expected 0x1234, got 0x%04x", holdingRegs[2])
	}

	inputRegs, ok := snapshot[AreaInputRegs].([]uint16)
	if !ok {
		t.Fatal("inputRegs not found in snapshot")
	}
	if inputRegs[3] != 0x5678 {
		t.Errorf("expected 0x5678, got 0x%04x", inputRegs[3])
	}
}

func TestModbusDataStore_Restore(t *testing.T) {
	store := NewModbusDataStore(10, 10, 10, 10)

	// 復元データを作成
	data := map[string]interface{}{
		AreaCoils:          []bool{true, false, true},
		AreaDiscreteInputs: []bool{false, true, false},
		AreaHoldingRegs:    []uint16{0x1111, 0x2222, 0x3333},
		AreaInputRegs:      []uint16{0x4444, 0x5555, 0x6666},
	}

	// 復元
	err := store.Restore(data)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// 確認
	val, _ := store.ReadBit(AreaCoils, 0)
	if !val {
		t.Error("expected coil[0] to be true")
	}

	val, _ = store.ReadBit(AreaDiscreteInputs, 1)
	if !val {
		t.Error("expected discreteInput[1] to be true")
	}

	word, _ := store.ReadWord(AreaHoldingRegs, 1)
	if word != 0x2222 {
		t.Errorf("expected 0x2222, got 0x%04x", word)
	}

	word, _ = store.ReadWord(AreaInputRegs, 2)
	if word != 0x6666 {
		t.Errorf("expected 0x6666, got 0x%04x", word)
	}
}

func TestModbusDataStore_ClearAll(t *testing.T) {
	store := NewModbusDataStore(10, 10, 10, 10)

	// データを設定
	_ = store.WriteBit(AreaCoils, 0, true)
	_ = store.WriteBit(AreaDiscreteInputs, 0, true)
	_ = store.WriteWord(AreaHoldingRegs, 0, 0x1234)
	_ = store.WriteWord(AreaInputRegs, 0, 0x5678)

	// クリア
	store.ClearAll()

	// 確認
	val, _ := store.ReadBit(AreaCoils, 0)
	if val {
		t.Error("expected coil[0] to be false after clear")
	}

	val, _ = store.ReadBit(AreaDiscreteInputs, 0)
	if val {
		t.Error("expected discreteInput[0] to be false after clear")
	}

	word, _ := store.ReadWord(AreaHoldingRegs, 0)
	if word != 0 {
		t.Errorf("expected 0, got 0x%04x after clear", word)
	}

	word, _ = store.ReadWord(AreaInputRegs, 0)
	if word != 0 {
		t.Errorf("expected 0, got 0x%04x after clear", word)
	}
}

func TestModbusDataStore_LegacyMethods(t *testing.T) {
	store := NewModbusDataStore(100, 50, 200, 150)

	// コイルのレガシーメソッド
	err := store.SetCoil(10, true)
	if err != nil {
		t.Fatalf("SetCoil failed: %v", err)
	}

	val, err := store.GetCoil(10)
	if err != nil {
		t.Fatalf("GetCoil failed: %v", err)
	}
	if !val {
		t.Error("expected true, got false")
	}

	// 保持レジスタのレガシーメソッド
	err = store.SetHoldingRegister(20, 0x1234)
	if err != nil {
		t.Fatalf("SetHoldingRegister failed: %v", err)
	}

	word, err := store.GetHoldingRegister(20)
	if err != nil {
		t.Fatalf("GetHoldingRegister failed: %v", err)
	}
	if word != 0x1234 {
		t.Errorf("expected 0x1234, got 0x%04x", word)
	}
}

func TestModbusDataStore_GetAll(t *testing.T) {
	store := NewModbusDataStore(10, 10, 10, 10)

	// データを設定
	_ = store.SetCoil(0, true)
	_ = store.SetCoil(2, true)
	_ = store.SetHoldingRegister(1, 0x1234)

	// 全データ取得
	coils := store.GetAllCoils()
	if len(coils) != 10 {
		t.Errorf("expected 10 coils, got %d", len(coils))
	}
	if !coils[0] || coils[1] || !coils[2] {
		t.Error("coil values mismatch")
	}

	holdingRegs := store.GetAllHoldingRegisters()
	if len(holdingRegs) != 10 {
		t.Errorf("expected 10 holding registers, got %d", len(holdingRegs))
	}
	if holdingRegs[1] != 0x1234 {
		t.Errorf("expected 0x1234, got 0x%04x", holdingRegs[1])
	}
}

func TestModbusDataStore_SetAll(t *testing.T) {
	store := NewModbusDataStore(10, 10, 10, 10)

	// 全コイルを設定
	coils := []bool{true, false, true, false, true}
	store.SetAllCoils(coils)

	// 確認
	for i, expected := range coils {
		val, _ := store.GetCoil(uint16(i))
		if val != expected {
			t.Errorf("coil[%d]: expected %v, got %v", i, expected, val)
		}
	}

	// サイズより大きいデータを設定（切り詰められるべき）
	largeCoils := make([]bool, 20)
	for i := range largeCoils {
		largeCoils[i] = true
	}
	store.SetAllCoils(largeCoils)

	// 確認（10個まで設定されているはず）
	for i := 0; i < 10; i++ {
		val, _ := store.GetCoil(uint16(i))
		if !val {
			t.Errorf("coil[%d]: expected true", i)
		}
	}
}
