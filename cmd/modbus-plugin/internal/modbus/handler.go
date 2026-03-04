package modbus

import (
	"modbus_simulator/internal/domain/register"

	"github.com/simonvetter/modbus"
)

// RegisterHandler はModbusリクエストを処理するハンドラ
type RegisterHandler struct {
	store           *register.RegisterStore
	disabledUnitIDs map[uint8]bool // 応答しないUnitIDのセット
}

// NewRegisterHandler は新しいRegisterHandlerを作成する
func NewRegisterHandler(store *register.RegisterStore) *RegisterHandler {
	return &RegisterHandler{
		store:           store,
		disabledUnitIDs: make(map[uint8]bool),
	}
}

// SetUnitIdEnabled は指定したUnitIdの応答を有効/無効にする
func (h *RegisterHandler) SetUnitIdEnabled(unitId uint8, enabled bool) {
	if enabled {
		delete(h.disabledUnitIDs, unitId)
	} else {
		h.disabledUnitIDs[unitId] = true
	}
}

// IsUnitIdEnabled は指定したUnitIdが応答するかどうかを返す
func (h *RegisterHandler) IsUnitIdEnabled(unitId uint8) bool {
	return !h.disabledUnitIDs[unitId]
}

// GetDisabledUnitIDs は無効化されたUnitIDのリストを返す
func (h *RegisterHandler) GetDisabledUnitIDs() []uint8 {
	result := make([]uint8, 0, len(h.disabledUnitIDs))
	for id := range h.disabledUnitIDs {
		result = append(result, id)
	}
	return result
}

// SetDisabledUnitIDs は無効化するUnitIDのリストを設定する
func (h *RegisterHandler) SetDisabledUnitIDs(ids []uint8) {
	h.disabledUnitIDs = make(map[uint8]bool)
	for _, id := range ids {
		h.disabledUnitIDs[id] = true
	}
}

// isUnitIdAllowed は指定したUnitIdがリクエストに応答すべきかを判定する
func (h *RegisterHandler) isUnitIdAllowed(unitId uint8) bool {
	// disabledUnitIDsに含まれるUnitIdには応答しない
	return !h.disabledUnitIDs[unitId]
}

// HandleCoils はコイル読み取りを処理する (Function Code 1)
func (h *RegisterHandler) HandleCoils(req *modbus.CoilsRequest) ([]bool, error) {
	if !h.isUnitIdAllowed(req.UnitId) {
		return nil, modbus.ErrIllegalFunction
	}
	return h.store.GetCoils(req.Addr, req.Quantity)
}

// HandleDiscreteInputs はディスクリート入力読み取りを処理する (Function Code 2)
func (h *RegisterHandler) HandleDiscreteInputs(req *modbus.DiscreteInputsRequest) ([]bool, error) {
	if !h.isUnitIdAllowed(req.UnitId) {
		return nil, modbus.ErrIllegalFunction
	}
	return h.store.GetDiscreteInputs(req.Addr, req.Quantity)
}

// HandleHoldingRegisters は保持レジスタ読み取りを処理する (Function Code 3)
func (h *RegisterHandler) HandleHoldingRegisters(req *modbus.HoldingRegistersRequest) ([]uint16, error) {
	if !h.isUnitIdAllowed(req.UnitId) {
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
	if !h.isUnitIdAllowed(req.UnitId) {
		return nil, modbus.ErrIllegalFunction
	}
	return h.store.GetInputRegisters(req.Addr, req.Quantity)
}

// HandleWriteSingleCoil は単一コイル書き込みを処理する (Function Code 5)
func (h *RegisterHandler) HandleWriteSingleCoil(req *modbus.CoilsRequest) error {
	if !h.isUnitIdAllowed(req.UnitId) {
		return modbus.ErrIllegalFunction
	}
	if len(req.Args) == 0 {
		return modbus.ErrIllegalDataValue
	}
	return h.store.SetCoil(req.Addr, req.Args[0])
}

// HandleWriteMultipleCoils は複数コイル書き込みを処理する (Function Code 15)
func (h *RegisterHandler) HandleWriteMultipleCoils(req *modbus.CoilsRequest) error {
	if !h.isUnitIdAllowed(req.UnitId) {
		return modbus.ErrIllegalFunction
	}
	return h.store.SetCoils(req.Addr, req.Args)
}
