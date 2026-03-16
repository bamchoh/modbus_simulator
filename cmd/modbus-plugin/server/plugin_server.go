package server

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"google.golang.org/grpc"

	pb "modbus_simulator/pb/pluginpb"

	"modbus_simulator/internal/domain/protocol"
	"modbus_simulator/cmd/modbus-plugin/internal/modbus"
)

// PluginServer は Modbus プラグインの gRPC サーバー実装
// PluginService と DataStoreService を同一の gRPC サーバーで提供する
type PluginServer struct {
	pb.UnimplementedPluginServiceServer
	pb.UnimplementedDataStoreServiceServer

	mu           sync.Mutex
	protocolType string // "modbus-tcp", "modbus-rtu", "modbus-ascii"
	factory      protocol.ServerFactory
	store        *modbus.ModbusDataStore
	server       protocol.ProtocolServer

	// SubscribeChanges ストリームの購読者チャンネル
	subsMu      sync.RWMutex
	subscribers []chan *pb.DataChange

	// ホストからの書き込み中フラグ（循環通知防止）
	hostWriting bool
}

// NewPluginServer は PluginServer を作成する。
// protocolType は "modbus-tcp", "modbus-rtu", "modbus-ascii" のいずれかを指定する。
func NewPluginServer(protocolType string) *PluginServer {
	var factory protocol.ServerFactory
	switch protocolType {
	case "modbus-rtu":
		factory = modbus.NewModbusRTUServerFactory()
	case "modbus-ascii":
		factory = modbus.NewModbusASCIIServerFactory()
	default:
		factory = modbus.NewModbusTCPServerFactory()
	}
	return &PluginServer{
		protocolType: protocolType,
		factory:      factory,
		store:        modbus.NewModbusDataStore(65536, 65536, 65536, 65536),
	}
}

// Register は gRPC サーバーにサービスを登録する
func (s *PluginServer) Register(srv *grpc.Server) {
	pb.RegisterPluginServiceServer(srv, s)
	pb.RegisterDataStoreServiceServer(srv, s)
}

// ===== PluginService =====

func (s *PluginServer) GetMetadata(ctx context.Context, _ *pb.Empty) (*pb.PluginMetadata, error) {
	return &pb.PluginMetadata{
		ProtocolType: s.protocolType,
		DisplayName:  s.factory.DisplayName(),
		Capabilities: &pb.ProtocolCapabilities{
			SupportsUnitId:         true,
			UnitIdMin:              1,
			UnitIdMax:              247,
			SupportsNodePublishing: false,
		},
	}, nil
}

func (s *PluginServer) GetConfigVariants(ctx context.Context, _ *pb.Empty) (*pb.GetConfigVariantsResponse, error) {
	// バリアントなし（プロトコルタイプが固定されているため）
	return &pb.GetConfigVariantsResponse{Variants: nil}, nil
}

func (s *PluginServer) GetConfigFields(ctx context.Context, req *pb.GetConfigFieldsRequest) (*pb.GetConfigFieldsResponse, error) {
	factory := s.factory
	fields := factory.GetConfigFields(req.VariantId)
	pbFields := make([]*pb.ConfigField, len(fields))
	for i, f := range fields {
		pbF := &pb.ConfigField{
			Name:        f.Name,
			Label:       f.Label,
			Description: f.Description,
			Type:        f.Type,
			Required:    f.Required,
		}
		if f.Default != nil {
			if b, err := json.Marshal(f.Default); err == nil {
				pbF.DefaultJson = string(b)
			}
		}
		for _, o := range f.Options {
			pbF.Options = append(pbF.Options, &pb.FieldOption{Value: o.Value, Label: o.Label})
		}
		if f.Min != nil {
			pbF.HasMin = true
			pbF.Min = int32(*f.Min)
		}
		if f.Max != nil {
			pbF.HasMax = true
			pbF.Max = int32(*f.Max)
		}
		if f.Condition != nil {
			pbF.Condition = &pb.FieldCondition{Field: f.Condition.Field, Value: f.Condition.Value}
		}
		pbFields[i] = pbF
	}
	return &pb.GetConfigFieldsResponse{Fields: pbFields}, nil
}

func (s *PluginServer) GetDefaultConfig(ctx context.Context, req *pb.GetDefaultConfigRequest) (*pb.ConfigDataResponse, error) {
	factory := s.factory
	config := factory.CreateConfigFromVariant(req.VariantId)
	settingsMap := factory.ConfigToMap(config)
	settingsJSON, err := json.Marshal(settingsMap)
	if err != nil {
		return nil, err
	}
	return &pb.ConfigDataResponse{
		VariantId:    req.VariantId,
		SettingsJson: string(settingsJSON),
	}, nil
}

func (s *PluginServer) MapToConfig(ctx context.Context, req *pb.MapToConfigRequest) (*pb.MapToConfigResponse, error) {
	factory := s.factory
	var settings map[string]interface{}
	if err := json.Unmarshal([]byte(req.SettingsJson), &settings); err != nil {
		return &pb.MapToConfigResponse{Error: "JSON パース失敗: " + err.Error()}, nil
	}
	config, err := factory.MapToConfig(req.VariantId, settings)
	if err != nil {
		return &pb.MapToConfigResponse{Error: err.Error()}, nil
	}
	settingsMap := factory.ConfigToMap(config)
	settingsJSON, err := json.Marshal(settingsMap)
	if err != nil {
		return &pb.MapToConfigResponse{Error: err.Error()}, nil
	}
	return &pb.MapToConfigResponse{
		VariantId:    req.VariantId,
		SettingsJson: string(settingsJSON),
	}, nil
}

func (s *PluginServer) ConfigToMap(ctx context.Context, req *pb.ConfigToMapRequest) (*pb.ConfigToMapResponse, error) {
	factory := s.factory
	var settings map[string]interface{}
	if err := json.Unmarshal([]byte(req.SettingsJson), &settings); err != nil {
		return nil, err
	}
	config, err := factory.MapToConfig(req.VariantId, settings)
	if err != nil {
		return nil, err
	}
	m := factory.ConfigToMap(config)
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return &pb.ConfigToMapResponse{SettingsJson: string(b)}, nil
}

func (s *PluginServer) CreateAndStart(ctx context.Context, req *pb.CreateAndStartRequest) (*pb.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	variantID := req.VariantId
	factory := s.factory

	// 設定を復元
	var settings map[string]interface{}
	if req.SettingsJson != "" {
		if err := json.Unmarshal([]byte(req.SettingsJson), &settings); err != nil {
			return nil, fmt.Errorf("設定のパース失敗: %w", err)
		}
	}

	var config protocol.ProtocolConfig
	if len(settings) > 0 {
		var err error
		config, err = factory.MapToConfig(variantID, settings)
		if err != nil {
			return nil, fmt.Errorf("設定の変換失敗: %w", err)
		}
	} else {
		config = factory.CreateConfigFromVariant(variantID)
	}

	// DataStore を作成
	innerStore := factory.CreateDataStore()
	modbusStore, ok := innerStore.(*modbus.ModbusDataStore)
	if !ok {
		return nil, fmt.Errorf("DataStore の型が不正: %T", innerStore)
	}
	s.store = modbusStore

	// 変更フックを設定（Modbus クライアントの書き込みを SubscribeChanges ストリームに転送）
	s.store.SetChangeHook(s.onDataChange)

	// サーバーを作成・起動
	srv, err := factory.CreateServer(config, innerStore)
	if err != nil {
		return nil, fmt.Errorf("サーバー作成失敗: %w", err)
	}
	s.server = srv
	s.factory = factory

	if err := srv.Start(ctx); err != nil {
		return nil, fmt.Errorf("サーバー起動失敗: %w", err)
	}

	return &pb.Empty{}, nil
}

func (s *PluginServer) Stop(ctx context.Context, _ *pb.Empty) (*pb.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server != nil {
		s.server.Stop()
	}
	return &pb.Empty{}, nil
}

func (s *PluginServer) GetStatus(ctx context.Context, _ *pb.Empty) (*pb.StatusResponse, error) {
	s.mu.Lock()
	srv := s.server
	s.mu.Unlock()

	if srv == nil {
		return &pb.StatusResponse{Status: "Stopped"}, nil
	}
	switch srv.Status() {
	case protocol.StatusRunning:
		return &pb.StatusResponse{Status: "Running"}, nil
	case protocol.StatusStopped:
		return &pb.StatusResponse{Status: "Stopped"}, nil
	default:
		return &pb.StatusResponse{Status: "Error"}, nil
	}
}

func (s *PluginServer) UpdateConfig(ctx context.Context, req *pb.UpdateConfigRequest) (*pb.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server == nil {
		return nil, fmt.Errorf("サーバーが未起動")
	}

	var settings map[string]interface{}
	if err := json.Unmarshal([]byte(req.SettingsJson), &settings); err != nil {
		return nil, err
	}
	config, err := s.factory.MapToConfig(req.VariantId, settings)
	if err != nil {
		return nil, err
	}
	if err := s.server.UpdateConfig(config); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

func (s *PluginServer) OnNodePublishingUpdated(ctx context.Context, _ *pb.Empty) (*pb.Empty, error) {
	// Modbus は NodePublishing をサポートしないため何もしない
	return &pb.Empty{}, nil
}

// UnitID サポート

func (s *PluginServer) GetUnitIDSettings(ctx context.Context, _ *pb.Empty) (*pb.UnitIDSettingsResponse, error) {
	s.mu.Lock()
	srv := s.server
	s.mu.Unlock()

	type unitIDGetter interface {
		GetDisabledUnitIDs() []uint8
	}
	if srv != nil {
		if u, ok := srv.(unitIDGetter); ok {
			ids := u.GetDisabledUnitIDs()
			int32IDs := make([]int32, len(ids))
			for i, id := range ids {
				int32IDs[i] = int32(id)
			}
			return &pb.UnitIDSettingsResponse{DisabledIds: int32IDs}, nil
		}
	}
	return &pb.UnitIDSettingsResponse{}, nil
}

func (s *PluginServer) SetUnitIDEnabled(ctx context.Context, req *pb.SetUnitIDEnabledRequest) (*pb.Empty, error) {
	s.mu.Lock()
	srv := s.server
	s.mu.Unlock()

	type unitIDSetter interface {
		SetUnitIdEnabled(unitId uint8, enabled bool)
	}
	if srv != nil {
		if u, ok := srv.(unitIDSetter); ok {
			u.SetUnitIdEnabled(uint8(req.UnitId), req.Enabled)
		}
	}
	return &pb.Empty{}, nil
}

func (s *PluginServer) SetDisabledUnitIDs(ctx context.Context, req *pb.SetDisabledUnitIDsRequest) (*pb.Empty, error) {
	s.mu.Lock()
	srv := s.server
	s.mu.Unlock()

	type unitIDSetter interface {
		SetDisabledUnitIDs(ids []uint8)
	}
	if srv != nil {
		if u, ok := srv.(unitIDSetter); ok {
			ids := make([]uint8, len(req.Ids))
			for i, id := range req.Ids {
				ids[i] = uint8(id)
			}
			u.SetDisabledUnitIDs(ids)
		}
	}
	return &pb.Empty{}, nil
}

// ===== DataStoreService =====

func (s *PluginServer) GetAreas(ctx context.Context, _ *pb.Empty) (*pb.GetAreasResponse, error) {
	if s.store == nil {
		return &pb.GetAreasResponse{}, nil
	}
	areas := s.store.GetAreas()
	pbAreas := make([]*pb.MemoryArea, len(areas))
	for i, a := range areas {
		pbAreas[i] = &pb.MemoryArea{
			Id:             a.ID,
			DisplayName:    a.DisplayName,
			IsBit:          a.IsBit,
			Size:           a.Size,
			ReadOnly:       a.ReadOnly,
			ByteAddressing: a.ByteAddressing,
			OneOrigin:      a.OneOrigin,
		}
	}
	return &pb.GetAreasResponse{Areas: pbAreas}, nil
}

func (s *PluginServer) ReadBit(ctx context.Context, req *pb.ReadBitRequest) (*pb.ReadBitResponse, error) {
	if s.store == nil {
		return nil, fmt.Errorf("DataStore 未初期化")
	}
	v, err := s.store.ReadBit(req.Area, req.Address)
	if err != nil {
		return nil, err
	}
	return &pb.ReadBitResponse{Value: v}, nil
}

func (s *PluginServer) WriteBit(ctx context.Context, req *pb.WriteBitRequest) (*pb.Empty, error) {
	if s.store == nil {
		return nil, fmt.Errorf("DataStore 未初期化")
	}
	// ホストからの書き込みフラグを立てて循環通知を防止
	s.setHostWriting(true)
	err := s.store.WriteBit(req.Area, req.Address, req.Value)
	s.setHostWriting(false)
	return &pb.Empty{}, err
}

func (s *PluginServer) ReadBits(ctx context.Context, req *pb.ReadBitsRequest) (*pb.ReadBitsResponse, error) {
	if s.store == nil {
		return nil, fmt.Errorf("DataStore 未初期化")
	}
	vals, err := s.store.ReadBits(req.Area, req.Address, uint16(req.Count))
	if err != nil {
		return nil, err
	}
	return &pb.ReadBitsResponse{Values: vals}, nil
}

func (s *PluginServer) WriteBits(ctx context.Context, req *pb.WriteBitsRequest) (*pb.Empty, error) {
	if s.store == nil {
		return nil, fmt.Errorf("DataStore 未初期化")
	}
	s.setHostWriting(true)
	err := s.store.WriteBits(req.Area, req.Address, req.Values)
	s.setHostWriting(false)
	return &pb.Empty{}, err
}

func (s *PluginServer) ReadWord(ctx context.Context, req *pb.ReadWordRequest) (*pb.ReadWordResponse, error) {
	if s.store == nil {
		return nil, fmt.Errorf("DataStore 未初期化")
	}
	v, err := s.store.ReadWord(req.Area, req.Address)
	if err != nil {
		return nil, err
	}
	return &pb.ReadWordResponse{Value: uint32(v)}, nil
}

func (s *PluginServer) WriteWord(ctx context.Context, req *pb.WriteWordRequest) (*pb.Empty, error) {
	if s.store == nil {
		return nil, fmt.Errorf("DataStore 未初期化")
	}
	s.setHostWriting(true)
	err := s.store.WriteWord(req.Area, req.Address, uint16(req.Value))
	s.setHostWriting(false)
	return &pb.Empty{}, err
}

func (s *PluginServer) ReadWords(ctx context.Context, req *pb.ReadWordsRequest) (*pb.ReadWordsResponse, error) {
	if s.store == nil {
		return nil, fmt.Errorf("DataStore 未初期化")
	}
	vals, err := s.store.ReadWords(req.Area, req.Address, uint16(req.Count))
	if err != nil {
		return nil, err
	}
	uint32Vals := make([]uint32, len(vals))
	for i, v := range vals {
		uint32Vals[i] = uint32(v)
	}
	return &pb.ReadWordsResponse{Values: uint32Vals}, nil
}

func (s *PluginServer) WriteWords(ctx context.Context, req *pb.WriteWordsRequest) (*pb.Empty, error) {
	if s.store == nil {
		return nil, fmt.Errorf("DataStore 未初期化")
	}
	vals := make([]uint16, len(req.Values))
	for i, v := range req.Values {
		vals[i] = uint16(v)
	}
	s.setHostWriting(true)
	err := s.store.WriteWords(req.Area, req.Address, vals)
	s.setHostWriting(false)
	return &pb.Empty{}, err
}

func (s *PluginServer) Snapshot(ctx context.Context, _ *pb.Empty) (*pb.SnapshotResponse, error) {
	if s.store == nil {
		return &pb.SnapshotResponse{}, nil
	}
	snap := s.store.Snapshot()
	b, err := json.Marshal(snap)
	if err != nil {
		return nil, err
	}
	return &pb.SnapshotResponse{SnapshotJson: b}, nil
}

func (s *PluginServer) Restore(ctx context.Context, req *pb.RestoreRequest) (*pb.Empty, error) {
	if s.store == nil {
		return &pb.Empty{}, nil
	}
	var snap map[string]interface{}
	if err := json.Unmarshal(req.SnapshotJson, &snap); err != nil {
		return nil, err
	}
	s.store.Restore(snap)
	return &pb.Empty{}, nil
}

func (s *PluginServer) ClearAll(ctx context.Context, _ *pb.Empty) (*pb.Empty, error) {
	if s.store == nil {
		return &pb.Empty{}, nil
	}
	s.store.ClearAll()
	return &pb.Empty{}, nil
}

// SubscribeChanges は Modbus クライアントが書き込んだ変更をストリームで送信する
func (s *PluginServer) SubscribeChanges(_ *pb.Empty, stream pb.DataStoreService_SubscribeChangesServer) error {
	ch := make(chan *pb.DataChange, 64)

	s.subsMu.Lock()
	s.subscribers = append(s.subscribers, ch)
	s.subsMu.Unlock()

	defer func() {
		s.subsMu.Lock()
		for i, sub := range s.subscribers {
			if sub == ch {
				s.subscribers = append(s.subscribers[:i], s.subscribers[i+1:]...)
				break
			}
		}
		s.subsMu.Unlock()
		close(ch)
	}()

	for {
		select {
		case change, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(change); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return nil
		}
	}
}

// onDataChange は ModbusDataStore の変更フックから呼ばれる
func (s *PluginServer) onDataChange(area string, address uint32, values []uint16, isBit bool, bitValues []bool) {
	// ホストからの書き込み中は通知しない（循環防止）
	if s.isHostWriting() {
		return
	}

	change := &pb.DataChange{
		Area:    area,
		Address: address,
		IsBit:   isBit,
	}
	if isBit {
		change.BitValues = bitValues
	} else {
		uint32Vals := make([]uint32, len(values))
		for i, v := range values {
			uint32Vals[i] = uint32(v)
		}
		change.Values = uint32Vals
	}

	s.subsMu.RLock()
	subs := make([]chan *pb.DataChange, len(s.subscribers))
	copy(subs, s.subscribers)
	s.subsMu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- change:
		default:
			// チャンネルが詰まっている場合はスキップ
		}
	}
}

func (s *PluginServer) setHostWriting(v bool) {
	s.mu.Lock()
	s.hostWriting = v
	s.mu.Unlock()
}

func (s *PluginServer) isHostWriting() bool {
	s.mu.Lock()
	v := s.hostWriting
	s.mu.Unlock()
	return v
}

