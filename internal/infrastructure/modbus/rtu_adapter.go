package modbus

import (
	"modbus_simulator/internal/infrastructure/modbus/rtu"
)

// RTUHandlerAdapter はRegisterHandlerをrtu.RequestHandlerに適合させるアダプター
type RTUHandlerAdapter struct {
	handler *RegisterHandler
}

// NewRTUHandlerAdapter は新しいRTUHandlerAdapterを作成する
func NewRTUHandlerAdapter(handler *RegisterHandler) *RTUHandlerAdapter {
	return &RTUHandlerAdapter{handler: handler}
}

// HandleReadCoils はコイル読み取りを処理する (FC 01)
func (a *RTUHandlerAdapter) HandleReadCoils(unitID byte, address, quantity uint16) ([]bool, error) {
	if !a.handler.isUnitIdAllowed(unitID) {
		return nil, rtu.ErrIllegalFunction
	}
	values, err := a.handler.store.GetCoils(address, quantity)
	if err != nil {
		return nil, rtu.ErrIllegalDataAddress
	}
	return values, nil
}

// HandleReadDiscreteInputs はディスクリート入力読み取りを処理する (FC 02)
func (a *RTUHandlerAdapter) HandleReadDiscreteInputs(unitID byte, address, quantity uint16) ([]bool, error) {
	if !a.handler.isUnitIdAllowed(unitID) {
		return nil, rtu.ErrIllegalFunction
	}
	values, err := a.handler.store.GetDiscreteInputs(address, quantity)
	if err != nil {
		return nil, rtu.ErrIllegalDataAddress
	}
	return values, nil
}

// HandleReadHoldingRegisters は保持レジスタ読み取りを処理する (FC 03)
func (a *RTUHandlerAdapter) HandleReadHoldingRegisters(unitID byte, address, quantity uint16) ([]uint16, error) {
	if !a.handler.isUnitIdAllowed(unitID) {
		return nil, rtu.ErrIllegalFunction
	}
	values, err := a.handler.store.GetHoldingRegisters(address, quantity)
	if err != nil {
		return nil, rtu.ErrIllegalDataAddress
	}
	return values, nil
}

// HandleReadInputRegisters は入力レジスタ読み取りを処理する (FC 04)
func (a *RTUHandlerAdapter) HandleReadInputRegisters(unitID byte, address, quantity uint16) ([]uint16, error) {
	if !a.handler.isUnitIdAllowed(unitID) {
		return nil, rtu.ErrIllegalFunction
	}
	values, err := a.handler.store.GetInputRegisters(address, quantity)
	if err != nil {
		return nil, rtu.ErrIllegalDataAddress
	}
	return values, nil
}

// HandleWriteSingleCoil は単一コイル書き込みを処理する (FC 05)
func (a *RTUHandlerAdapter) HandleWriteSingleCoil(unitID byte, address uint16, value bool) error {
	if !a.handler.isUnitIdAllowed(unitID) {
		return rtu.ErrIllegalFunction
	}
	if err := a.handler.store.SetCoil(address, value); err != nil {
		return rtu.ErrIllegalDataAddress
	}
	return nil
}

// HandleWriteSingleRegister は単一レジスタ書き込みを処理する (FC 06)
func (a *RTUHandlerAdapter) HandleWriteSingleRegister(unitID byte, address, value uint16) error {
	if !a.handler.isUnitIdAllowed(unitID) {
		return rtu.ErrIllegalFunction
	}
	if err := a.handler.store.SetHoldingRegister(address, value); err != nil {
		return rtu.ErrIllegalDataAddress
	}
	return nil
}

// HandleWriteMultipleCoils は複数コイル書き込みを処理する (FC 15)
func (a *RTUHandlerAdapter) HandleWriteMultipleCoils(unitID byte, address uint16, values []bool) error {
	if !a.handler.isUnitIdAllowed(unitID) {
		return rtu.ErrIllegalFunction
	}
	if err := a.handler.store.SetCoils(address, values); err != nil {
		return rtu.ErrIllegalDataAddress
	}
	return nil
}

// HandleWriteMultipleRegisters は複数レジスタ書き込みを処理する (FC 16)
func (a *RTUHandlerAdapter) HandleWriteMultipleRegisters(unitID byte, address uint16, values []uint16) error {
	if !a.handler.isUnitIdAllowed(unitID) {
		return rtu.ErrIllegalFunction
	}
	if err := a.handler.store.SetHoldingRegisters(address, values); err != nil {
		return rtu.ErrIllegalDataAddress
	}
	return nil
}

// IsUnitIDEnabled は指定したUnitIDが応答するかどうかを返す
func (a *RTUHandlerAdapter) IsUnitIDEnabled(unitID byte) bool {
	return a.handler.isUnitIdAllowed(unitID)
}
