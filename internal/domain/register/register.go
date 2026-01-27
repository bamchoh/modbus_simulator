package register

import "sync"

// RegisterType はModbusレジスタの種類を表す
type RegisterType int

const (
	Coil RegisterType = iota
	DiscreteInput
	HoldingRegister
	InputRegister
)

func (t RegisterType) String() string {
	switch t {
	case Coil:
		return "Coil"
	case DiscreteInput:
		return "DiscreteInput"
	case HoldingRegister:
		return "HoldingRegister"
	case InputRegister:
		return "InputRegister"
	default:
		return "Unknown"
	}
}

// RegisterStore はPLCのレジスタを管理する
type RegisterStore struct {
	mu              sync.RWMutex
	coils           []bool
	discreteInputs  []bool
	holdingRegs     []uint16
	inputRegs       []uint16
}

// NewRegisterStore は新しいRegisterStoreを作成する
func NewRegisterStore(coilCount, discreteCount, holdingCount, inputCount int) *RegisterStore {
	return &RegisterStore{
		coils:          make([]bool, coilCount),
		discreteInputs: make([]bool, discreteCount),
		holdingRegs:    make([]uint16, holdingCount),
		inputRegs:      make([]uint16, inputCount),
	}
}

// GetCoil はコイルの値を取得する
func (r *RegisterStore) GetCoil(address uint16) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if int(address) >= len(r.coils) {
		return false, ErrAddressOutOfRange
	}
	return r.coils[address], nil
}

// SetCoil はコイルの値を設定する
func (r *RegisterStore) SetCoil(address uint16, value bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if int(address) >= len(r.coils) {
		return ErrAddressOutOfRange
	}
	r.coils[address] = value
	return nil
}

// GetCoils は複数のコイルの値を取得する
func (r *RegisterStore) GetCoils(address uint16, count uint16) ([]bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if int(address)+int(count) > len(r.coils) {
		return nil, ErrAddressOutOfRange
	}
	result := make([]bool, count)
	copy(result, r.coils[address:address+count])
	return result, nil
}

// SetCoils は複数のコイルの値を設定する
func (r *RegisterStore) SetCoils(address uint16, values []bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if int(address)+len(values) > len(r.coils) {
		return ErrAddressOutOfRange
	}
	copy(r.coils[address:], values)
	return nil
}

// GetDiscreteInput はディスクリート入力の値を取得する
func (r *RegisterStore) GetDiscreteInput(address uint16) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if int(address) >= len(r.discreteInputs) {
		return false, ErrAddressOutOfRange
	}
	return r.discreteInputs[address], nil
}

// SetDiscreteInput はディスクリート入力の値を設定する
func (r *RegisterStore) SetDiscreteInput(address uint16, value bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if int(address) >= len(r.discreteInputs) {
		return ErrAddressOutOfRange
	}
	r.discreteInputs[address] = value
	return nil
}

// GetDiscreteInputs は複数のディスクリート入力の値を取得する
func (r *RegisterStore) GetDiscreteInputs(address uint16, count uint16) ([]bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if int(address)+int(count) > len(r.discreteInputs) {
		return nil, ErrAddressOutOfRange
	}
	result := make([]bool, count)
	copy(result, r.discreteInputs[address:address+count])
	return result, nil
}

// GetHoldingRegister は保持レジスタの値を取得する
func (r *RegisterStore) GetHoldingRegister(address uint16) (uint16, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if int(address) >= len(r.holdingRegs) {
		return 0, ErrAddressOutOfRange
	}
	return r.holdingRegs[address], nil
}

// SetHoldingRegister は保持レジスタの値を設定する
func (r *RegisterStore) SetHoldingRegister(address uint16, value uint16) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if int(address) >= len(r.holdingRegs) {
		return ErrAddressOutOfRange
	}
	r.holdingRegs[address] = value
	return nil
}

// GetHoldingRegisters は複数の保持レジスタの値を取得する
func (r *RegisterStore) GetHoldingRegisters(address uint16, count uint16) ([]uint16, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if int(address)+int(count) > len(r.holdingRegs) {
		return nil, ErrAddressOutOfRange
	}
	result := make([]uint16, count)
	copy(result, r.holdingRegs[address:address+count])
	return result, nil
}

// SetHoldingRegisters は複数の保持レジスタの値を設定する
func (r *RegisterStore) SetHoldingRegisters(address uint16, values []uint16) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if int(address)+len(values) > len(r.holdingRegs) {
		return ErrAddressOutOfRange
	}
	copy(r.holdingRegs[address:], values)
	return nil
}

// GetInputRegister は入力レジスタの値を取得する
func (r *RegisterStore) GetInputRegister(address uint16) (uint16, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if int(address) >= len(r.inputRegs) {
		return 0, ErrAddressOutOfRange
	}
	return r.inputRegs[address], nil
}

// SetInputRegister は入力レジスタの値を設定する
func (r *RegisterStore) SetInputRegister(address uint16, value uint16) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if int(address) >= len(r.inputRegs) {
		return ErrAddressOutOfRange
	}
	r.inputRegs[address] = value
	return nil
}

// GetInputRegisters は複数の入力レジスタの値を取得する
func (r *RegisterStore) GetInputRegisters(address uint16, count uint16) ([]uint16, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if int(address)+int(count) > len(r.inputRegs) {
		return nil, ErrAddressOutOfRange
	}
	result := make([]uint16, count)
	copy(result, r.inputRegs[address:address+count])
	return result, nil
}

// GetAllCoils は全てのコイルを取得する
func (r *RegisterStore) GetAllCoils() []bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]bool, len(r.coils))
	copy(result, r.coils)
	return result
}

// GetAllDiscreteInputs は全てのディスクリート入力を取得する
func (r *RegisterStore) GetAllDiscreteInputs() []bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]bool, len(r.discreteInputs))
	copy(result, r.discreteInputs)
	return result
}

// GetAllHoldingRegisters は全ての保持レジスタを取得する
func (r *RegisterStore) GetAllHoldingRegisters() []uint16 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]uint16, len(r.holdingRegs))
	copy(result, r.holdingRegs)
	return result
}

// GetAllInputRegisters は全ての入力レジスタを取得する
func (r *RegisterStore) GetAllInputRegisters() []uint16 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]uint16, len(r.inputRegs))
	copy(result, r.inputRegs)
	return result
}

// SetAllCoils は全てのコイルを設定する
func (r *RegisterStore) SetAllCoils(values []bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := len(values)
	if count > len(r.coils) {
		count = len(r.coils)
	}
	copy(r.coils, values[:count])
}

// SetAllDiscreteInputs は全てのディスクリート入力を設定する
func (r *RegisterStore) SetAllDiscreteInputs(values []bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := len(values)
	if count > len(r.discreteInputs) {
		count = len(r.discreteInputs)
	}
	copy(r.discreteInputs, values[:count])
}

// SetAllHoldingRegisters は全ての保持レジスタを設定する
func (r *RegisterStore) SetAllHoldingRegisters(values []uint16) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := len(values)
	if count > len(r.holdingRegs) {
		count = len(r.holdingRegs)
	}
	copy(r.holdingRegs, values[:count])
}

// SetAllInputRegisters は全ての入力レジスタを設定する
func (r *RegisterStore) SetAllInputRegisters(values []uint16) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := len(values)
	if count > len(r.inputRegs) {
		count = len(r.inputRegs)
	}
	copy(r.inputRegs, values[:count])
}

// ClearAll は全てのレジスタをクリアする
func (r *RegisterStore) ClearAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.coils {
		r.coils[i] = false
	}
	for i := range r.discreteInputs {
		r.discreteInputs[i] = false
	}
	for i := range r.holdingRegs {
		r.holdingRegs[i] = 0
	}
	for i := range r.inputRegs {
		r.inputRegs[i] = 0
	}
}
