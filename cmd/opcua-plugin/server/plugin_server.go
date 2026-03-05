package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "modbus_simulator/pb/pluginpb"

	"modbus_simulator/cmd/opcua-plugin/internal/opcua"
	"modbus_simulator/internal/domain/protocol"
)

// PluginServer は OPC UA プラグインの gRPC サーバー実装
type PluginServer struct {
	pb.UnimplementedPluginServiceServer
	pb.UnimplementedDataStoreServiceServer

	mu      sync.Mutex
	factory *opcua.OpcuaServerFactory
	server  protocol.ProtocolServer
}

// NewPluginServer は PluginServer を作成する
func NewPluginServer() *PluginServer {
	return &PluginServer{
		factory: &opcua.OpcuaServerFactory{},
	}
}

// Register は gRPC サーバーにサービスを登録する
func (s *PluginServer) Register(srv *grpc.Server) {
	pb.RegisterPluginServiceServer(srv, s)
	pb.RegisterDataStoreServiceServer(srv, s)
}

// ===== PluginService =====

func (s *PluginServer) GetMetadata(ctx context.Context, _ *pb.Empty) (*pb.PluginMetadata, error) {
	caps := s.factory.GetProtocolCapabilities()
	return &pb.PluginMetadata{
		ProtocolType: string(s.factory.ProtocolType()),
		DisplayName:  s.factory.DisplayName(),
		Capabilities: &pb.ProtocolCapabilities{
			SupportsUnitId:         caps.SupportsUnitID,
			UnitIdMin:              int32(caps.UnitIDMin),
			UnitIdMax:              int32(caps.UnitIDMax),
			SupportsNodePublishing: caps.SupportsNodePublishing,
		},
	}, nil
}

func (s *PluginServer) GetConfigVariants(ctx context.Context, _ *pb.Empty) (*pb.GetConfigVariantsResponse, error) {
	variants := s.factory.ConfigVariants()
	pbVariants := make([]*pb.ConfigVariant, len(variants))
	for i, v := range variants {
		pbVariants[i] = &pb.ConfigVariant{Id: v.ID, DisplayName: v.DisplayName}
	}
	return &pb.GetConfigVariantsResponse{Variants: pbVariants}, nil
}

func (s *PluginServer) GetConfigFields(ctx context.Context, req *pb.GetConfigFieldsRequest) (*pb.GetConfigFieldsResponse, error) {
	fields := s.factory.GetConfigFields(req.VariantId)
	pbFields := make([]*pb.ConfigField, len(fields))
	for i, f := range fields {
		pbF := &pb.ConfigField{
			Name:     f.Name,
			Label:    f.Label,
			Type:     f.Type,
			Required: f.Required,
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
	config := s.factory.CreateConfigFromVariant(req.VariantId)
	settingsMap := s.factory.ConfigToMap(config)
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
	var settings map[string]interface{}
	if err := json.Unmarshal([]byte(req.SettingsJson), &settings); err != nil {
		return &pb.MapToConfigResponse{Error: "JSON パース失敗: " + err.Error()}, nil
	}
	config, err := s.factory.MapToConfig(req.VariantId, settings)
	if err != nil {
		return &pb.MapToConfigResponse{Error: err.Error()}, nil
	}
	settingsMap := s.factory.ConfigToMap(config)
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
	var settings map[string]interface{}
	if err := json.Unmarshal([]byte(req.SettingsJson), &settings); err != nil {
		return nil, err
	}
	config, err := s.factory.MapToConfig(req.VariantId, settings)
	if err != nil {
		return nil, err
	}
	m := s.factory.ConfigToMap(config)
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return &pb.ConfigToMapResponse{SettingsJson: string(b)}, nil
}

func (s *PluginServer) CreateAndStart(ctx context.Context, req *pb.CreateAndStartRequest) (*pb.Empty, error) {
	fmt.Fprintln(os.Stderr, "CreateAndStart called with VariantId:", req.VariantId)

	s.mu.Lock()
	defer s.mu.Unlock()

	// HostGrpcServer への gRPC 接続を確立して RemoteVariableStoreAccessor を注入
	if req.HostGrpcAddr != "" {
		conn, err := grpc.NewClient(req.HostGrpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, fmt.Errorf("HostGrpcServer への接続失敗: %w", err)
		}
		accessor := newRemoteVariableStoreAccessor(pb.NewVariableAccessorServiceClient(conn))
		s.factory.InjectVariableStore(accessor)
	}

	// 設定を復元
	variantID := req.VariantId
	var config protocol.ProtocolConfig
	if req.SettingsJson != "" {
		var settings map[string]interface{}
		if err := json.Unmarshal([]byte(req.SettingsJson), &settings); err != nil {
			return nil, fmt.Errorf("設定のパース失敗: %w", err)
		}
		var err error
		config, err = s.factory.MapToConfig(variantID, settings)
		if err != nil {
			return nil, fmt.Errorf("設定の変換失敗: %w", err)
		}
	} else {
		config = s.factory.CreateConfigFromVariant(variantID)
	}

	// DataStore を作成
	dataStore := s.factory.CreateDataStore()

	// サーバーを作成・起動
	srv, err := s.factory.CreateServer(config, dataStore)
	if err != nil {
		return nil, fmt.Errorf("サーバー作成失敗: %w", err)
	}
	s.server = srv

	// gRPC リクエストの ctx はメソッドが返った時点でキャンセルされる。
	// OPC UA サーバーの goroutine はサーバーが Stop() されるまで動き続ける必要があるため、
	// context.Background() を渡す。キャンセルは OpcuaServer.Stop() → s.cancel() が担う。
	if err := srv.Start(context.Background()); err != nil {
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
	s.mu.Lock()
	srv := s.server
	s.mu.Unlock()

	if srv != nil {
		if aware, ok := srv.(protocol.NodePublishingAware); ok {
			aware.OnNodePublishingUpdated()
		}
	}
	return &pb.Empty{}, nil
}

// ===== DataStoreService（OPC UA は DataStore を持たないため空実装）=====

func (s *PluginServer) GetAreas(ctx context.Context, _ *pb.Empty) (*pb.GetAreasResponse, error) {
	return &pb.GetAreasResponse{}, nil
}

func (s *PluginServer) Snapshot(ctx context.Context, _ *pb.Empty) (*pb.SnapshotResponse, error) {
	return &pb.SnapshotResponse{}, nil
}

func (s *PluginServer) Restore(ctx context.Context, _ *pb.RestoreRequest) (*pb.Empty, error) {
	return &pb.Empty{}, nil
}

func (s *PluginServer) ClearAll(ctx context.Context, _ *pb.Empty) (*pb.Empty, error) {
	return &pb.Empty{}, nil
}

func (s *PluginServer) SubscribeChanges(_ *pb.Empty, stream pb.DataStoreService_SubscribeChangesServer) error {
	// OPC UA は DataStore の変更通知を持たないため、クライアントが切断するまで待つ
	<-stream.Context().Done()
	return nil
}

// ===== RemoteVariableStoreAccessor =====
// HostGrpcServer の VariableAccessorService を gRPC 経由で呼び出す

type remoteVariableStoreAccessor struct {
	client pb.VariableAccessorServiceClient
}

func newRemoteVariableStoreAccessor(client pb.VariableAccessorServiceClient) *remoteVariableStoreAccessor {
	return &remoteVariableStoreAccessor{client: client}
}

func (a *remoteVariableStoreAccessor) GetEnabledNodePublishings(protocolType string) []protocol.NodePublishingInfo {
	resp, err := a.client.GetEnabledNodePublishings(context.Background(), &pb.GetNodePublishingsRequest{
		ProtocolType: protocolType,
	})
	if err != nil {
		return nil
	}
	result := make([]protocol.NodePublishingInfo, len(resp.Publishings))
	for i, p := range resp.Publishings {
		result[i] = protocol.NodePublishingInfo{
			VariableID:   p.VariableId,
			VariableName: p.VariableName,
			DataType:     p.DataType,
			AccessMode:   p.AccessMode,
		}
	}
	return result
}

func (a *remoteVariableStoreAccessor) ReadVariableValue(variableID string) (interface{}, error) {
	resp, err := a.client.ReadVariableValue(context.Background(), &pb.ReadVariableValueRequest{
		VariableId: variableID,
	})
	if err != nil {
		return nil, err
	}
	var value interface{}
	if err := json.Unmarshal([]byte(resp.ValueJson), &value); err != nil {
		return nil, err
	}
	return value, nil
}

func (a *remoteVariableStoreAccessor) WriteVariableValue(variableID string, value interface{}) error {
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = a.client.WriteVariableValue(context.Background(), &pb.WriteVariableValueRequest{
		VariableId: variableID,
		ValueJson:  string(valueJSON),
	})
	return err
}

func (a *remoteVariableStoreAccessor) GetStructFields(typeName string) []protocol.StructFieldInfo {
	resp, err := a.client.GetStructFields(context.Background(), &pb.GetStructFieldsRequest{
		TypeName: typeName,
	})
	if err != nil {
		return nil
	}
	result := make([]protocol.StructFieldInfo, len(resp.Fields))
	for i, f := range resp.Fields {
		result[i] = protocol.StructFieldInfo{
			Name:     f.Name,
			DataType: f.DataType,
		}
	}
	return result
}
