package opcua

import "modbus_simulator/internal/domain/protocol"

// OpcuaDataStore は OPC UA サーバー用のスタブ DataStore。
// OPC UA はメモリエリアベースのアクセスを持たないため、空スライスを返す。
type OpcuaDataStore struct{}

func newOpcuaDataStore() *OpcuaDataStore {
	return &OpcuaDataStore{}
}

func (d *OpcuaDataStore) GetAreas() []protocol.MemoryArea        { return nil }
func (d *OpcuaDataStore) ReadBit(area string, address uint32) (bool, error) {
	return false, nil
}
func (d *OpcuaDataStore) WriteBit(area string, address uint32, value bool) error { return nil }
func (d *OpcuaDataStore) ReadBits(area string, address uint32, count uint16) ([]bool, error) {
	return nil, nil
}
func (d *OpcuaDataStore) WriteBits(area string, address uint32, values []bool) error { return nil }
func (d *OpcuaDataStore) ReadWord(area string, address uint32) (uint16, error) {
	return 0, nil
}
func (d *OpcuaDataStore) WriteWord(area string, address uint32, value uint16) error { return nil }
func (d *OpcuaDataStore) ReadWords(area string, address uint32, count uint16) ([]uint16, error) {
	return nil, nil
}
func (d *OpcuaDataStore) WriteWords(area string, address uint32, values []uint16) error { return nil }
func (d *OpcuaDataStore) Snapshot() map[string]interface{}                              { return nil }
func (d *OpcuaDataStore) Restore(data map[string]interface{}) error                    { return nil }
func (d *OpcuaDataStore) ClearAll()                                                    {}
