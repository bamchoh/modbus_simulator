package fins

import (
	"testing"

	"modbus_simulator/internal/domain/datastore"
)

func TestNewFINSDataStore(t *testing.T) {
	store := NewFINSDataStore()
	if store == nil {
		t.Fatal("NewFINSDataStore returned nil")
	}

	// デフォルトサイズの確認
	if len(store.areas[AreaIDCIO]) != DefaultCIOSize {
		t.Errorf("expected CIO size %d, got %d", DefaultCIOSize, len(store.areas[AreaIDCIO]))
	}
	if len(store.areas[AreaIDDM]) != DefaultDMSize {
		t.Errorf("expected DM size %d, got %d", DefaultDMSize, len(store.areas[AreaIDDM]))
	}
}

func TestNewFINSDataStoreWithSize(t *testing.T) {
	store := NewFINSDataStoreWithSize(100, 50, 200, 150, 300, 60, 70)
	if store == nil {
		t.Fatal("NewFINSDataStoreWithSize returned nil")
	}

	if len(store.areas[AreaIDCIO]) != 100 {
		t.Errorf("expected CIO size 100, got %d", len(store.areas[AreaIDCIO]))
	}
	if len(store.areas[AreaIDWR]) != 50 {
		t.Errorf("expected WR size 50, got %d", len(store.areas[AreaIDWR]))
	}
	if len(store.areas[AreaIDHR]) != 200 {
		t.Errorf("expected HR size 200, got %d", len(store.areas[AreaIDHR]))
	}
	if len(store.areas[AreaIDAR]) != 150 {
		t.Errorf("expected AR size 150, got %d", len(store.areas[AreaIDAR]))
	}
	if len(store.areas[AreaIDDM]) != 300 {
		t.Errorf("expected DM size 300, got %d", len(store.areas[AreaIDDM]))
	}
	if len(store.areas[AreaIDTIM]) != 60 {
		t.Errorf("expected TIM size 60, got %d", len(store.areas[AreaIDTIM]))
	}
	if len(store.areas[AreaIDCNT]) != 70 {
		t.Errorf("expected CNT size 70, got %d", len(store.areas[AreaIDCNT]))
	}
}

func TestFINSDataStore_GetAreas(t *testing.T) {
	store := NewFINSDataStore()
	areas := store.GetAreas()

	if len(areas) != 7 {
		t.Fatalf("expected 7 areas, got %d", len(areas))
	}

	// 順序の確認（CIO, WR, HR, AR, DM, TIM, CNT）
	expectedOrder := []string{AreaIDCIO, AreaIDWR, AreaIDHR, AreaIDAR, AreaIDDM, AreaIDTIM, AreaIDCNT}
	for i, expected := range expectedOrder {
		if areas[i].ID != expected {
			t.Errorf("area[%d]: expected %s, got %s", i, expected, areas[i].ID)
		}
	}
}

func TestFINSDataStore_ReadWriteWord(t *testing.T) {
	store := NewFINSDataStoreWithSize(100, 100, 100, 100, 100, 100, 100)

	testCases := []struct {
		area    string
		address uint32
		value   uint16
	}{
		{AreaIDCIO, 10, 0x1234},
		{AreaIDWR, 20, 0x5678},
		{AreaIDHR, 30, 0xABCD},
		{AreaIDAR, 40, 0xEF01},
		{AreaIDDM, 50, 0x2345},
		{AreaIDTIM, 60, 0x6789},
		{AreaIDCNT, 70, 0xCDEF},
	}

	for _, tc := range testCases {
		t.Run(tc.area, func(t *testing.T) {
			err := store.WriteWord(tc.area, tc.address, tc.value)
			if err != nil {
				t.Fatalf("WriteWord failed: %v", err)
			}

			val, err := store.ReadWord(tc.area, tc.address)
			if err != nil {
				t.Fatalf("ReadWord failed: %v", err)
			}
			if val != tc.value {
				t.Errorf("expected 0x%04x, got 0x%04x", tc.value, val)
			}
		})
	}
}

func TestFINSDataStore_ReadWriteWord_OutOfRange(t *testing.T) {
	store := NewFINSDataStoreWithSize(100, 100, 100, 100, 100, 100, 100)

	_, err := store.ReadWord(AreaIDDM, 100)
	if err != datastore.ErrAddressOutOfRange {
		t.Errorf("expected ErrAddressOutOfRange, got %v", err)
	}

	err = store.WriteWord(AreaIDDM, 100, 0x1234)
	if err != datastore.ErrAddressOutOfRange {
		t.Errorf("expected ErrAddressOutOfRange, got %v", err)
	}
}

func TestFINSDataStore_ReadWriteWord_AreaNotFound(t *testing.T) {
	store := NewFINSDataStore()

	_, err := store.ReadWord("nonexistent", 0)
	if err != datastore.ErrAreaNotFound {
		t.Errorf("expected ErrAreaNotFound, got %v", err)
	}

	err = store.WriteWord("nonexistent", 0, 0x1234)
	if err != datastore.ErrAreaNotFound {
		t.Errorf("expected ErrAreaNotFound, got %v", err)
	}
}

func TestFINSDataStore_ReadWriteWords(t *testing.T) {
	store := NewFINSDataStoreWithSize(100, 100, 100, 100, 100, 100, 100)

	values := []uint16{0x1111, 0x2222, 0x3333, 0x4444, 0x5555}
	err := store.WriteWords(AreaIDDM, 10, values)
	if err != nil {
		t.Fatalf("WriteWords failed: %v", err)
	}

	got, err := store.ReadWords(AreaIDDM, 10, 5)
	if err != nil {
		t.Fatalf("ReadWords failed: %v", err)
	}

	for i, v := range values {
		if got[i] != v {
			t.Errorf("word[%d]: expected 0x%04x, got 0x%04x", i, v, got[i])
		}
	}
}

func TestFINSDataStore_ReadWriteWords_OutOfRange(t *testing.T) {
	store := NewFINSDataStoreWithSize(100, 100, 100, 100, 100, 100, 100)

	_, err := store.ReadWords(AreaIDDM, 95, 10)
	if err != datastore.ErrAddressOutOfRange {
		t.Errorf("expected ErrAddressOutOfRange, got %v", err)
	}

	err = store.WriteWords(AreaIDDM, 95, []uint16{1, 2, 3, 4, 5, 6})
	if err != datastore.ErrAddressOutOfRange {
		t.Errorf("expected ErrAddressOutOfRange, got %v", err)
	}
}

func TestFINSDataStore_ReadWriteBit(t *testing.T) {
	store := NewFINSDataStoreWithSize(100, 100, 100, 100, 100, 100, 100)

	// ビット0を設定（ワード0のビット0）
	err := store.WriteBit(AreaIDDM, 0, true)
	if err != nil {
		t.Fatalf("WriteBit failed: %v", err)
	}

	val, err := store.ReadBit(AreaIDDM, 0)
	if err != nil {
		t.Fatalf("ReadBit failed: %v", err)
	}
	if !val {
		t.Error("expected true, got false")
	}

	// ビット17を設定（ワード1のビット1）
	err = store.WriteBit(AreaIDDM, 17, true)
	if err != nil {
		t.Fatalf("WriteBit failed: %v", err)
	}

	val, err = store.ReadBit(AreaIDDM, 17)
	if err != nil {
		t.Fatalf("ReadBit failed: %v", err)
	}
	if !val {
		t.Error("expected true, got false")
	}

	// ワード値の確認
	word, _ := store.ReadWord(AreaIDDM, 1)
	if word != 0x0002 { // ビット1が設定されている
		t.Errorf("expected 0x0002, got 0x%04x", word)
	}
}

func TestFINSDataStore_ReadWriteBit_OutOfRange(t *testing.T) {
	store := NewFINSDataStoreWithSize(10, 10, 10, 10, 10, 10, 10)

	// 10ワード = 160ビット、160以上は範囲外
	_, err := store.ReadBit(AreaIDDM, 160)
	if err != datastore.ErrAddressOutOfRange {
		t.Errorf("expected ErrAddressOutOfRange, got %v", err)
	}

	err = store.WriteBit(AreaIDDM, 160, true)
	if err != datastore.ErrAddressOutOfRange {
		t.Errorf("expected ErrAddressOutOfRange, got %v", err)
	}
}

func TestFINSDataStore_ReadWriteBits(t *testing.T) {
	store := NewFINSDataStoreWithSize(100, 100, 100, 100, 100, 100, 100)

	// 複数ビットの書き込み
	values := []bool{true, false, true, true, false}
	err := store.WriteBits(AreaIDDM, 10, values)
	if err != nil {
		t.Fatalf("WriteBits failed: %v", err)
	}

	// 複数ビットの読み取り
	got, err := store.ReadBits(AreaIDDM, 10, 5)
	if err != nil {
		t.Fatalf("ReadBits failed: %v", err)
	}

	for i, v := range values {
		if got[i] != v {
			t.Errorf("bit[%d]: expected %v, got %v", i, v, got[i])
		}
	}
}

func TestFINSDataStore_Snapshot(t *testing.T) {
	store := NewFINSDataStoreWithSize(10, 10, 10, 10, 10, 10, 10)

	// データを設定
	_ = store.WriteWord(AreaIDCIO, 0, 0x1234)
	_ = store.WriteWord(AreaIDDM, 5, 0x5678)

	// スナップショット取得
	snapshot := store.Snapshot()

	// スナップショットの確認
	cio, ok := snapshot[AreaIDCIO].([]uint16)
	if !ok {
		t.Fatal("CIO not found in snapshot")
	}
	if cio[0] != 0x1234 {
		t.Errorf("expected 0x1234, got 0x%04x", cio[0])
	}

	dm, ok := snapshot[AreaIDDM].([]uint16)
	if !ok {
		t.Fatal("DM not found in snapshot")
	}
	if dm[5] != 0x5678 {
		t.Errorf("expected 0x5678, got 0x%04x", dm[5])
	}
}

func TestFINSDataStore_Restore(t *testing.T) {
	store := NewFINSDataStoreWithSize(10, 10, 10, 10, 10, 10, 10)

	// 復元データを作成
	data := map[string]interface{}{
		AreaIDCIO: []uint16{0x1111, 0x2222, 0x3333},
		AreaIDDM:  []uint16{0x4444, 0x5555, 0x6666},
	}

	// 復元
	err := store.Restore(data)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// 確認
	word, _ := store.ReadWord(AreaIDCIO, 0)
	if word != 0x1111 {
		t.Errorf("expected 0x1111, got 0x%04x", word)
	}

	word, _ = store.ReadWord(AreaIDDM, 2)
	if word != 0x6666 {
		t.Errorf("expected 0x6666, got 0x%04x", word)
	}
}

func TestFINSDataStore_ClearAll(t *testing.T) {
	store := NewFINSDataStoreWithSize(10, 10, 10, 10, 10, 10, 10)

	// データを設定
	_ = store.WriteWord(AreaIDCIO, 0, 0x1234)
	_ = store.WriteWord(AreaIDDM, 0, 0x5678)

	// クリア
	store.ClearAll()

	// 確認
	word, _ := store.ReadWord(AreaIDCIO, 0)
	if word != 0 {
		t.Errorf("expected 0, got 0x%04x after clear", word)
	}

	word, _ = store.ReadWord(AreaIDDM, 0)
	if word != 0 {
		t.Errorf("expected 0, got 0x%04x after clear", word)
	}
}

func TestFINSDataStore_GetAllWords(t *testing.T) {
	store := NewFINSDataStoreWithSize(10, 10, 10, 10, 10, 10, 10)

	// データを設定
	_ = store.WriteWord(AreaIDDM, 0, 0x1111)
	_ = store.WriteWord(AreaIDDM, 1, 0x2222)

	// 全データ取得
	words, err := store.GetAllWords(AreaIDDM)
	if err != nil {
		t.Fatalf("GetAllWords failed: %v", err)
	}

	if len(words) != 10 {
		t.Errorf("expected 10 words, got %d", len(words))
	}
	if words[0] != 0x1111 {
		t.Errorf("expected 0x1111, got 0x%04x", words[0])
	}
	if words[1] != 0x2222 {
		t.Errorf("expected 0x2222, got 0x%04x", words[1])
	}
}

func TestFINSDataStore_GetAllWords_AreaNotFound(t *testing.T) {
	store := NewFINSDataStore()

	_, err := store.GetAllWords("nonexistent")
	if err != datastore.ErrAreaNotFound {
		t.Errorf("expected ErrAreaNotFound, got %v", err)
	}
}

func TestFINSDataStore_SetAllWords(t *testing.T) {
	store := NewFINSDataStoreWithSize(10, 10, 10, 10, 10, 10, 10)

	// 全ワードを設定
	words := []uint16{0x1111, 0x2222, 0x3333, 0x4444, 0x5555}
	err := store.SetAllWords(AreaIDDM, words)
	if err != nil {
		t.Fatalf("SetAllWords failed: %v", err)
	}

	// 確認
	for i, expected := range words {
		val, _ := store.ReadWord(AreaIDDM, uint32(i))
		if val != expected {
			t.Errorf("word[%d]: expected 0x%04x, got 0x%04x", i, expected, val)
		}
	}

	// サイズより大きいデータを設定（切り詰められるべき）
	largeWords := make([]uint16, 20)
	for i := range largeWords {
		largeWords[i] = 0xFFFF
	}
	err = store.SetAllWords(AreaIDDM, largeWords)
	if err != nil {
		t.Fatalf("SetAllWords failed: %v", err)
	}

	// 確認（10ワードまで設定されているはず）
	for i := 0; i < 10; i++ {
		val, _ := store.ReadWord(AreaIDDM, uint32(i))
		if val != 0xFFFF {
			t.Errorf("word[%d]: expected 0xFFFF, got 0x%04x", i, val)
		}
	}
}

func TestFINSDataStore_SetAllWords_AreaNotFound(t *testing.T) {
	store := NewFINSDataStore()

	err := store.SetAllWords("nonexistent", []uint16{1, 2, 3})
	if err != datastore.ErrAreaNotFound {
		t.Errorf("expected ErrAreaNotFound, got %v", err)
	}
}

// FINSDataStoreがDataStoreインターフェースを実装していることを確認
func TestFINSDataStore_ImplementsDataStore(t *testing.T) {
	store := NewFINSDataStore()

	// GetAreas
	areas := store.GetAreas()
	if len(areas) == 0 {
		t.Error("GetAreas returned empty")
	}

	// ReadBit/WriteBit
	_ = store.WriteBit(AreaIDDM, 0, true)
	_, _ = store.ReadBit(AreaIDDM, 0)

	// ReadBits/WriteBits
	_ = store.WriteBits(AreaIDDM, 0, []bool{true, false})
	_, _ = store.ReadBits(AreaIDDM, 0, 2)

	// ReadWord/WriteWord
	_ = store.WriteWord(AreaIDDM, 0, 0x1234)
	_, _ = store.ReadWord(AreaIDDM, 0)

	// ReadWords/WriteWords
	_ = store.WriteWords(AreaIDDM, 0, []uint16{0x1234, 0x5678})
	_, _ = store.ReadWords(AreaIDDM, 0, 2)

	// Snapshot/Restore
	snapshot := store.Snapshot()
	_ = store.Restore(snapshot)

	// ClearAll
	store.ClearAll()
}
