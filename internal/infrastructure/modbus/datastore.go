package modbus

import (
	"sync"

	"modbus_simulator/internal/domain/datastore"
	"modbus_simulator/internal/domain/protocol"
)

// ModbusDataStore はModbusプロトコル用のデータストア
type ModbusDataStore struct {
	mu             sync.RWMutex
	coils          []bool
	discreteInputs []bool
	holdingRegs    []uint16
	inputRegs      []uint16
}

// エリアID定数
const (
	AreaCoils           = "coils"
	AreaDiscreteInputs  = "discreteInputs"
	AreaHoldingRegs     = "holdingRegisters"
	AreaInputRegs       = "inputRegisters"
)

// NewModbusDataStore は新しいModbusDataStoreを作成する
func NewModbusDataStore(coilCount, discreteCount, holdingCount, inputCount int) *ModbusDataStore {
	return &ModbusDataStore{
		coils:          make([]bool, coilCount),
		discreteInputs: make([]bool, discreteCount),
		holdingRegs:    make([]uint16, holdingCount),
		inputRegs:      make([]uint16, inputCount),
	}
}

// GetAreas は利用可能なメモリエリアの一覧を返す
func (s *ModbusDataStore) GetAreas() []protocol.MemoryArea {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return []protocol.MemoryArea{
		{
			ID:          AreaCoils,
			DisplayName: "コイル (0x)",
			IsBit:       true,
			Size:        uint32(len(s.coils)),
			ReadOnly:    false,
		},
		{
			ID:          AreaDiscreteInputs,
			DisplayName: "ディスクリート入力 (1x)",
			IsBit:       true,
			Size:        uint32(len(s.discreteInputs)),
			ReadOnly:    false, // シミュレーターなので書き込み可能
		},
		{
			ID:          AreaHoldingRegs,
			DisplayName: "保持レジスタ (4x)",
			IsBit:       false,
			Size:        uint32(len(s.holdingRegs)),
			ReadOnly:    false,
		},
		{
			ID:          AreaInputRegs,
			DisplayName: "入力レジスタ (3x)",
			IsBit:       false,
			Size:        uint32(len(s.inputRegs)),
			ReadOnly:    false, // シミュレーターなので書き込み可能
		},
	}
}

// ReadBit はビット値を読み込む
func (s *ModbusDataStore) ReadBit(area string, address uint32) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	switch area {
	case AreaCoils:
		if int(address) >= len(s.coils) {
			return false, datastore.ErrAddressOutOfRange
		}
		return s.coils[address], nil
	case AreaDiscreteInputs:
		if int(address) >= len(s.discreteInputs) {
			return false, datastore.ErrAddressOutOfRange
		}
		return s.discreteInputs[address], nil
	default:
		return false, datastore.ErrAreaNotFound
	}
}

// WriteBit はビット値を書き込む
func (s *ModbusDataStore) WriteBit(area string, address uint32, value bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch area {
	case AreaCoils:
		if int(address) >= len(s.coils) {
			return datastore.ErrAddressOutOfRange
		}
		s.coils[address] = value
		return nil
	case AreaDiscreteInputs:
		if int(address) >= len(s.discreteInputs) {
			return datastore.ErrAddressOutOfRange
		}
		s.discreteInputs[address] = value
		return nil
	default:
		return datastore.ErrAreaNotFound
	}
}

// ReadBits は複数のビット値を読み込む
func (s *ModbusDataStore) ReadBits(area string, address uint32, count uint16) ([]bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	switch area {
	case AreaCoils:
		if int(address)+int(count) > len(s.coils) {
			return nil, datastore.ErrAddressOutOfRange
		}
		result := make([]bool, count)
		copy(result, s.coils[address:address+uint32(count)])
		return result, nil
	case AreaDiscreteInputs:
		if int(address)+int(count) > len(s.discreteInputs) {
			return nil, datastore.ErrAddressOutOfRange
		}
		result := make([]bool, count)
		copy(result, s.discreteInputs[address:address+uint32(count)])
		return result, nil
	default:
		return nil, datastore.ErrAreaNotFound
	}
}

// WriteBits は複数のビット値を書き込む
func (s *ModbusDataStore) WriteBits(area string, address uint32, values []bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch area {
	case AreaCoils:
		if int(address)+len(values) > len(s.coils) {
			return datastore.ErrAddressOutOfRange
		}
		copy(s.coils[address:], values)
		return nil
	case AreaDiscreteInputs:
		if int(address)+len(values) > len(s.discreteInputs) {
			return datastore.ErrAddressOutOfRange
		}
		copy(s.discreteInputs[address:], values)
		return nil
	default:
		return datastore.ErrAreaNotFound
	}
}

// ReadWord はワード値を読み込む
func (s *ModbusDataStore) ReadWord(area string, address uint32) (uint16, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	switch area {
	case AreaHoldingRegs:
		if int(address) >= len(s.holdingRegs) {
			return 0, datastore.ErrAddressOutOfRange
		}
		return s.holdingRegs[address], nil
	case AreaInputRegs:
		if int(address) >= len(s.inputRegs) {
			return 0, datastore.ErrAddressOutOfRange
		}
		return s.inputRegs[address], nil
	default:
		return 0, datastore.ErrAreaNotFound
	}
}

// WriteWord はワード値を書き込む
func (s *ModbusDataStore) WriteWord(area string, address uint32, value uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch area {
	case AreaHoldingRegs:
		if int(address) >= len(s.holdingRegs) {
			return datastore.ErrAddressOutOfRange
		}
		s.holdingRegs[address] = value
		return nil
	case AreaInputRegs:
		if int(address) >= len(s.inputRegs) {
			return datastore.ErrAddressOutOfRange
		}
		s.inputRegs[address] = value
		return nil
	default:
		return datastore.ErrAreaNotFound
	}
}

// ReadWords は複数のワード値を読み込む
func (s *ModbusDataStore) ReadWords(area string, address uint32, count uint16) ([]uint16, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	switch area {
	case AreaHoldingRegs:
		if int(address)+int(count) > len(s.holdingRegs) {
			return nil, datastore.ErrAddressOutOfRange
		}
		result := make([]uint16, count)
		copy(result, s.holdingRegs[address:address+uint32(count)])
		return result, nil
	case AreaInputRegs:
		if int(address)+int(count) > len(s.inputRegs) {
			return nil, datastore.ErrAddressOutOfRange
		}
		result := make([]uint16, count)
		copy(result, s.inputRegs[address:address+uint32(count)])
		return result, nil
	default:
		return nil, datastore.ErrAreaNotFound
	}
}

// WriteWords は複数のワード値を書き込む
func (s *ModbusDataStore) WriteWords(area string, address uint32, values []uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch area {
	case AreaHoldingRegs:
		if int(address)+len(values) > len(s.holdingRegs) {
			return datastore.ErrAddressOutOfRange
		}
		copy(s.holdingRegs[address:], values)
		return nil
	case AreaInputRegs:
		if int(address)+len(values) > len(s.inputRegs) {
			return datastore.ErrAddressOutOfRange
		}
		copy(s.inputRegs[address:], values)
		return nil
	default:
		return datastore.ErrAreaNotFound
	}
}

// Snapshot はデータストアのスナップショットを作成する
func (s *ModbusDataStore) Snapshot() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	coils := make([]bool, len(s.coils))
	copy(coils, s.coils)

	discreteInputs := make([]bool, len(s.discreteInputs))
	copy(discreteInputs, s.discreteInputs)

	holdingRegs := make([]uint16, len(s.holdingRegs))
	copy(holdingRegs, s.holdingRegs)

	inputRegs := make([]uint16, len(s.inputRegs))
	copy(inputRegs, s.inputRegs)

	return map[string]interface{}{
		AreaCoils:          coils,
		AreaDiscreteInputs: discreteInputs,
		AreaHoldingRegs:    holdingRegs,
		AreaInputRegs:      inputRegs,
	}
}

// Restore はスナップショットからデータを復元する
func (s *ModbusDataStore) Restore(data map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if coils, ok := data[AreaCoils]; ok {
		if bools, ok := coils.([]bool); ok {
			count := len(bools)
			if count > len(s.coils) {
				count = len(s.coils)
			}
			copy(s.coils, bools[:count])
		}
	}

	if discreteInputs, ok := data[AreaDiscreteInputs]; ok {
		if bools, ok := discreteInputs.([]bool); ok {
			count := len(bools)
			if count > len(s.discreteInputs) {
				count = len(s.discreteInputs)
			}
			copy(s.discreteInputs, bools[:count])
		}
	}

	if holdingRegs, ok := data[AreaHoldingRegs]; ok {
		if words, ok := holdingRegs.([]uint16); ok {
			count := len(words)
			if count > len(s.holdingRegs) {
				count = len(s.holdingRegs)
			}
			copy(s.holdingRegs, words[:count])
		}
	}

	if inputRegs, ok := data[AreaInputRegs]; ok {
		if words, ok := inputRegs.([]uint16); ok {
			count := len(words)
			if count > len(s.inputRegs) {
				count = len(s.inputRegs)
			}
			copy(s.inputRegs, words[:count])
		}
	}

	return nil
}

// ClearAll は全てのデータをクリアする
func (s *ModbusDataStore) ClearAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.coils {
		s.coils[i] = false
	}
	for i := range s.discreteInputs {
		s.discreteInputs[i] = false
	}
	for i := range s.holdingRegs {
		s.holdingRegs[i] = 0
	}
	for i := range s.inputRegs {
		s.inputRegs[i] = 0
	}
}

// === 旧RegisterStoreとの互換性のためのメソッド ===

// GetCoil はコイルの値を取得する
func (s *ModbusDataStore) GetCoil(address uint16) (bool, error) {
	return s.ReadBit(AreaCoils, uint32(address))
}

// SetCoil はコイルの値を設定する
func (s *ModbusDataStore) SetCoil(address uint16, value bool) error {
	return s.WriteBit(AreaCoils, uint32(address), value)
}

// GetCoils は複数のコイルの値を取得する
func (s *ModbusDataStore) GetCoils(address uint16, count uint16) ([]bool, error) {
	return s.ReadBits(AreaCoils, uint32(address), count)
}

// SetCoils は複数のコイルの値を設定する
func (s *ModbusDataStore) SetCoils(address uint16, values []bool) error {
	return s.WriteBits(AreaCoils, uint32(address), values)
}

// GetDiscreteInput はディスクリート入力の値を取得する
func (s *ModbusDataStore) GetDiscreteInput(address uint16) (bool, error) {
	return s.ReadBit(AreaDiscreteInputs, uint32(address))
}

// SetDiscreteInput はディスクリート入力の値を設定する
func (s *ModbusDataStore) SetDiscreteInput(address uint16, value bool) error {
	return s.WriteBit(AreaDiscreteInputs, uint32(address), value)
}

// GetDiscreteInputs は複数のディスクリート入力の値を取得する
func (s *ModbusDataStore) GetDiscreteInputs(address uint16, count uint16) ([]bool, error) {
	return s.ReadBits(AreaDiscreteInputs, uint32(address), count)
}

// GetHoldingRegister は保持レジスタの値を取得する
func (s *ModbusDataStore) GetHoldingRegister(address uint16) (uint16, error) {
	return s.ReadWord(AreaHoldingRegs, uint32(address))
}

// SetHoldingRegister は保持レジスタの値を設定する
func (s *ModbusDataStore) SetHoldingRegister(address uint16, value uint16) error {
	return s.WriteWord(AreaHoldingRegs, uint32(address), value)
}

// GetHoldingRegisters は複数の保持レジスタの値を取得する
func (s *ModbusDataStore) GetHoldingRegisters(address uint16, count uint16) ([]uint16, error) {
	return s.ReadWords(AreaHoldingRegs, uint32(address), count)
}

// SetHoldingRegisters は複数の保持レジスタの値を設定する
func (s *ModbusDataStore) SetHoldingRegisters(address uint16, values []uint16) error {
	return s.WriteWords(AreaHoldingRegs, uint32(address), values)
}

// GetInputRegister は入力レジスタの値を取得する
func (s *ModbusDataStore) GetInputRegister(address uint16) (uint16, error) {
	return s.ReadWord(AreaInputRegs, uint32(address))
}

// SetInputRegister は入力レジスタの値を設定する
func (s *ModbusDataStore) SetInputRegister(address uint16, value uint16) error {
	return s.WriteWord(AreaInputRegs, uint32(address), value)
}

// GetInputRegisters は複数の入力レジスタの値を取得する
func (s *ModbusDataStore) GetInputRegisters(address uint16, count uint16) ([]uint16, error) {
	return s.ReadWords(AreaInputRegs, uint32(address), count)
}

// GetAllCoils は全てのコイルを取得する
func (s *ModbusDataStore) GetAllCoils() []bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]bool, len(s.coils))
	copy(result, s.coils)
	return result
}

// GetAllDiscreteInputs は全てのディスクリート入力を取得する
func (s *ModbusDataStore) GetAllDiscreteInputs() []bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]bool, len(s.discreteInputs))
	copy(result, s.discreteInputs)
	return result
}

// GetAllHoldingRegisters は全ての保持レジスタを取得する
func (s *ModbusDataStore) GetAllHoldingRegisters() []uint16 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]uint16, len(s.holdingRegs))
	copy(result, s.holdingRegs)
	return result
}

// GetAllInputRegisters は全ての入力レジスタを取得する
func (s *ModbusDataStore) GetAllInputRegisters() []uint16 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]uint16, len(s.inputRegs))
	copy(result, s.inputRegs)
	return result
}

// SetAllCoils は全てのコイルを設定する
func (s *ModbusDataStore) SetAllCoils(values []bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := len(values)
	if count > len(s.coils) {
		count = len(s.coils)
	}
	copy(s.coils, values[:count])
}

// SetAllDiscreteInputs は全てのディスクリート入力を設定する
func (s *ModbusDataStore) SetAllDiscreteInputs(values []bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := len(values)
	if count > len(s.discreteInputs) {
		count = len(s.discreteInputs)
	}
	copy(s.discreteInputs, values[:count])
}

// SetAllHoldingRegisters は全ての保持レジスタを設定する
func (s *ModbusDataStore) SetAllHoldingRegisters(values []uint16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := len(values)
	if count > len(s.holdingRegs) {
		count = len(s.holdingRegs)
	}
	copy(s.holdingRegs, values[:count])
}

// SetAllInputRegisters は全ての入力レジスタを設定する
func (s *ModbusDataStore) SetAllInputRegisters(values []uint16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := len(values)
	if count > len(s.inputRegs) {
		count = len(s.inputRegs)
	}
	copy(s.inputRegs, values[:count])
}
