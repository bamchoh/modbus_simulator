package modbus

import (
	"modbus_simulator/internal/domain/register"

	"github.com/simonvetter/modbus"
)

// RegisterHandler はModbusリクエストを処理するハンドラ
type RegisterHandler struct {
	store   *register.RegisterStore
	slaveID uint8
}

// NewRegisterHandler は新しいRegisterHandlerを作成する
func NewRegisterHandler(store *register.RegisterStore, slaveID uint8) *RegisterHandler {
	return &RegisterHandler{
		store:   store,
		slaveID: slaveID,
	}
}

// HandleCoils はコイル読み取りを処理する (Function Code 1)
func (h *RegisterHandler) HandleCoils(req *modbus.CoilsRequest) ([]bool, error) {
	if req.UnitId != h.slaveID {
		return nil, modbus.ErrIllegalFunction
	}
	return h.store.GetCoils(req.Addr, req.Quantity)
}

// HandleDiscreteInputs はディスクリート入力読み取りを処理する (Function Code 2)
func (h *RegisterHandler) HandleDiscreteInputs(req *modbus.DiscreteInputsRequest) ([]bool, error) {
	if req.UnitId != h.slaveID {
		return nil, modbus.ErrIllegalFunction
	}
	return h.store.GetDiscreteInputs(req.Addr, req.Quantity)
}

// HandleHoldingRegisters は保持レジスタ読み取りを処理する (Function Code 3)
func (h *RegisterHandler) HandleHoldingRegisters(req *modbus.HoldingRegistersRequest) ([]uint16, error) {
	if req.UnitId != h.slaveID {
		return nil, modbus.ErrIllegalFunction
	}

	if req.IsWrite {
		// 書き込みリクエスト (Function Code 6, 16)
		if err := h.store.SetHoldingRegisters(req.Addr, req.Args); err != nil {
			return nil, modbus.ErrIllegalDataAddress
		}
		return req.Args, nil
	}

	// 読み取りリクエスト
	return h.store.GetHoldingRegisters(req.Addr, req.Quantity)
}

// HandleInputRegisters は入力レジスタ読み取りを処理する (Function Code 4)
func (h *RegisterHandler) HandleInputRegisters(req *modbus.InputRegistersRequest) ([]uint16, error) {
	if req.UnitId != h.slaveID {
		return nil, modbus.ErrIllegalFunction
	}
	return h.store.GetInputRegisters(req.Addr, req.Quantity)
}

// HandleWriteSingleCoil は単一コイル書き込みを処理する (Function Code 5)
func (h *RegisterHandler) HandleWriteSingleCoil(req *modbus.CoilsRequest) error {
	if req.UnitId != h.slaveID {
		return modbus.ErrIllegalFunction
	}
	if len(req.Args) == 0 {
		return modbus.ErrIllegalDataValue
	}
	return h.store.SetCoil(req.Addr, req.Args[0])
}

// HandleWriteMultipleCoils は複数コイル書き込みを処理する (Function Code 15)
func (h *RegisterHandler) HandleWriteMultipleCoils(req *modbus.CoilsRequest) error {
	if req.UnitId != h.slaveID {
		return modbus.ErrIllegalFunction
	}
	return h.store.SetCoils(req.Addr, req.Args)
}
