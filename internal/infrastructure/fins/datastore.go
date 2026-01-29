package fins

import (
	"sync"

	"modbus_simulator/internal/domain/datastore"
	"modbus_simulator/internal/domain/protocol"
)

// FINSDataStore はFINSプロトコル用のデータストア
type FINSDataStore struct {
	mu     sync.RWMutex
	areas  map[string][]uint16 // エリアID -> ワードデータ
	config map[string]MemoryAreaInfo
}

// エリアID定数
const (
	AreaIDCIO = "CIO"
	AreaIDWR  = "WR"
	AreaIDHR  = "HR"
	AreaIDAR  = "AR"
	AreaIDDM  = "DM"
	AreaIDTIM = "TIM"
	AreaIDCNT = "CNT"
)

// デフォルトメモリサイズ
const (
	DefaultCIOSize = 6144  // CIO: 6144 words
	DefaultWRSize  = 512   // WR: 512 words
	DefaultHRSize  = 1536  // HR: 1536 words
	DefaultARSize  = 960   // AR: 960 words
	DefaultDMSize  = 32768 // DM: 32768 words
	DefaultTIMSize = 4096  // TIM: 4096 words
	DefaultCNTSize = 4096  // CNT: 4096 words
)

// NewFINSDataStore は新しいFINSDataStoreを作成する
func NewFINSDataStore() *FINSDataStore {
	return NewFINSDataStoreWithSize(
		DefaultCIOSize,
		DefaultWRSize,
		DefaultHRSize,
		DefaultARSize,
		DefaultDMSize,
		DefaultTIMSize,
		DefaultCNTSize,
	)
}

// NewFINSDataStoreWithSize は指定サイズのFINSDataStoreを作成する
func NewFINSDataStoreWithSize(cioSize, wrSize, hrSize, arSize, dmSize, timSize, cntSize int) *FINSDataStore {
	store := &FINSDataStore{
		areas:  make(map[string][]uint16),
		config: make(map[string]MemoryAreaInfo),
	}

	// エリアを初期化
	store.areas[AreaIDCIO] = make([]uint16, cioSize)
	store.areas[AreaIDWR] = make([]uint16, wrSize)
	store.areas[AreaIDHR] = make([]uint16, hrSize)
	store.areas[AreaIDAR] = make([]uint16, arSize)
	store.areas[AreaIDDM] = make([]uint16, dmSize)
	store.areas[AreaIDTIM] = make([]uint16, timSize)
	store.areas[AreaIDCNT] = make([]uint16, cntSize)

	// エリア設定を保存
	store.config[AreaIDCIO] = MemoryAreaInfo{Code: AreaCIO, ID: AreaIDCIO, DisplayName: "CIO Area", Size: uint32(cioSize), IsBit: false, ReadOnly: false}
	store.config[AreaIDWR] = MemoryAreaInfo{Code: AreaWR, ID: AreaIDWR, DisplayName: "Work Area", Size: uint32(wrSize), IsBit: false, ReadOnly: false}
	store.config[AreaIDHR] = MemoryAreaInfo{Code: AreaHR, ID: AreaIDHR, DisplayName: "Holding Area", Size: uint32(hrSize), IsBit: false, ReadOnly: false}
	store.config[AreaIDAR] = MemoryAreaInfo{Code: AreaAR, ID: AreaIDAR, DisplayName: "Auxiliary Area", Size: uint32(arSize), IsBit: false, ReadOnly: false}
	store.config[AreaIDDM] = MemoryAreaInfo{Code: AreaDM, ID: AreaIDDM, DisplayName: "Data Memory", Size: uint32(dmSize), IsBit: false, ReadOnly: false}
	store.config[AreaIDTIM] = MemoryAreaInfo{Code: AreaTIM, ID: AreaIDTIM, DisplayName: "Timer", Size: uint32(timSize), IsBit: false, ReadOnly: false}
	store.config[AreaIDCNT] = MemoryAreaInfo{Code: AreaCNT, ID: AreaIDCNT, DisplayName: "Counter", Size: uint32(cntSize), IsBit: false, ReadOnly: false}

	return store
}

// GetAreas は利用可能なメモリエリアの一覧を返す
func (s *FINSDataStore) GetAreas() []protocol.MemoryArea {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 固定順序で返す
	order := []string{AreaIDCIO, AreaIDWR, AreaIDHR, AreaIDAR, AreaIDDM, AreaIDTIM, AreaIDCNT}
	result := make([]protocol.MemoryArea, 0, len(order))

	for _, id := range order {
		if info, ok := s.config[id]; ok {
			result = append(result, protocol.MemoryArea{
				ID:          info.ID,
				DisplayName: info.DisplayName,
				IsBit:       info.IsBit,
				Size:        info.Size,
				ReadOnly:    info.ReadOnly,
			})
		}
	}

	return result
}

// ReadBit はビット値を読み込む
func (s *FINSDataStore) ReadBit(area string, address uint32) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, ok := s.areas[area]
	if !ok {
		return false, datastore.ErrAreaNotFound
	}

	// ワードアドレスとビット位置を計算
	wordAddr := address / 16
	bitPos := address % 16

	if int(wordAddr) >= len(data) {
		return false, datastore.ErrAddressOutOfRange
	}

	return (data[wordAddr] & (1 << bitPos)) != 0, nil
}

// WriteBit はビット値を書き込む
func (s *FINSDataStore) WriteBit(area string, address uint32, value bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, ok := s.areas[area]
	if !ok {
		return datastore.ErrAreaNotFound
	}

	// ワードアドレスとビット位置を計算
	wordAddr := address / 16
	bitPos := address % 16

	if int(wordAddr) >= len(data) {
		return datastore.ErrAddressOutOfRange
	}

	if value {
		data[wordAddr] |= (1 << bitPos)
	} else {
		data[wordAddr] &^= (1 << bitPos)
	}

	return nil
}

// ReadBits は複数のビット値を読み込む
func (s *FINSDataStore) ReadBits(area string, address uint32, count uint16) ([]bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, ok := s.areas[area]
	if !ok {
		return nil, datastore.ErrAreaNotFound
	}

	result := make([]bool, count)
	for i := uint16(0); i < count; i++ {
		bitAddr := address + uint32(i)
		wordAddr := bitAddr / 16
		bitPos := bitAddr % 16

		if int(wordAddr) >= len(data) {
			return nil, datastore.ErrAddressOutOfRange
		}

		result[i] = (data[wordAddr] & (1 << bitPos)) != 0
	}

	return result, nil
}

// WriteBits は複数のビット値を書き込む
func (s *FINSDataStore) WriteBits(area string, address uint32, values []bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, ok := s.areas[area]
	if !ok {
		return datastore.ErrAreaNotFound
	}

	for i, value := range values {
		bitAddr := address + uint32(i)
		wordAddr := bitAddr / 16
		bitPos := bitAddr % 16

		if int(wordAddr) >= len(data) {
			return datastore.ErrAddressOutOfRange
		}

		if value {
			data[wordAddr] |= (1 << bitPos)
		} else {
			data[wordAddr] &^= (1 << bitPos)
		}
	}

	return nil
}

// ReadWord はワード値を読み込む
func (s *FINSDataStore) ReadWord(area string, address uint32) (uint16, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, ok := s.areas[area]
	if !ok {
		return 0, datastore.ErrAreaNotFound
	}

	if int(address) >= len(data) {
		return 0, datastore.ErrAddressOutOfRange
	}

	return data[address], nil
}

// WriteWord はワード値を書き込む
func (s *FINSDataStore) WriteWord(area string, address uint32, value uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, ok := s.areas[area]
	if !ok {
		return datastore.ErrAreaNotFound
	}

	if int(address) >= len(data) {
		return datastore.ErrAddressOutOfRange
	}

	data[address] = value
	return nil
}

// ReadWords は複数のワード値を読み込む
func (s *FINSDataStore) ReadWords(area string, address uint32, count uint16) ([]uint16, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, ok := s.areas[area]
	if !ok {
		return nil, datastore.ErrAreaNotFound
	}

	if int(address)+int(count) > len(data) {
		return nil, datastore.ErrAddressOutOfRange
	}

	result := make([]uint16, count)
	copy(result, data[address:address+uint32(count)])
	return result, nil
}

// WriteWords は複数のワード値を書き込む
func (s *FINSDataStore) WriteWords(area string, address uint32, values []uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, ok := s.areas[area]
	if !ok {
		return datastore.ErrAreaNotFound
	}

	if int(address)+len(values) > len(data) {
		return datastore.ErrAddressOutOfRange
	}

	copy(data[address:], values)
	return nil
}

// Snapshot はデータストアのスナップショットを作成する
func (s *FINSDataStore) Snapshot() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]interface{})
	for id, data := range s.areas {
		copied := make([]uint16, len(data))
		copy(copied, data)
		result[id] = copied
	}

	return result
}

// Restore はスナップショットからデータを復元する
func (s *FINSDataStore) Restore(data map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, values := range data {
		if words, ok := values.([]uint16); ok {
			if existing, exists := s.areas[id]; exists {
				count := len(words)
				if count > len(existing) {
					count = len(existing)
				}
				copy(existing, words[:count])
			}
		}
	}

	return nil
}

// ClearAll は全てのデータをクリアする
func (s *FINSDataStore) ClearAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, data := range s.areas {
		for i := range data {
			data[i] = 0
		}
	}
}

// ReadWordsByCode はエリアコードを使ってワードを読み込む
func (s *FINSDataStore) ReadWordsByCode(code MemoryAreaCode, address uint32, count uint16) ([]uint16, error) {
	areaID := AreaCodeToID(code)
	if areaID == "" {
		return nil, datastore.ErrAreaNotFound
	}
	return s.ReadWords(areaID, address, count)
}

// WriteWordsByCode はエリアコードを使ってワードを書き込む
func (s *FINSDataStore) WriteWordsByCode(code MemoryAreaCode, address uint32, values []uint16) error {
	areaID := AreaCodeToID(code)
	if areaID == "" {
		return datastore.ErrAreaNotFound
	}
	return s.WriteWords(areaID, address, values)
}

// GetAllWords は指定エリアの全ワードを取得する
func (s *FINSDataStore) GetAllWords(area string) ([]uint16, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, ok := s.areas[area]
	if !ok {
		return nil, datastore.ErrAreaNotFound
	}

	result := make([]uint16, len(data))
	copy(result, data)
	return result, nil
}

// SetAllWords は指定エリアの全ワードを設定する
func (s *FINSDataStore) SetAllWords(area string, values []uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, ok := s.areas[area]
	if !ok {
		return datastore.ErrAreaNotFound
	}

	count := len(values)
	if count > len(data) {
		count = len(data)
	}
	copy(data, values[:count])
	return nil
}

// protocol.DataStore インターフェースを満たすことを確認
var _ protocol.DataStore = (*FINSDataStore)(nil)
