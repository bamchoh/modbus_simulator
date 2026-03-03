package plugin

import (
	"encoding/json"
	"fmt"

	"google.golang.org/grpc"

	pb "modbus_simulator/pb/pluginpb"

	"modbus_simulator/internal/domain/protocol"
)

// RemoteServerFactory は gRPC クライアントを通じてプラグインプロセスの ServerFactory を実装する
type RemoteServerFactory struct {
	client   pb.PluginServiceClient
	conn     *grpc.ClientConn
	metadata *pb.PluginMetadata
}

// NewRemoteServerFactory は RemoteServerFactory を作成する。
// metadata は起動時に一度だけ取得しキャッシュする。
func NewRemoteServerFactory(conn *grpc.ClientConn, metadata *pb.PluginMetadata) *RemoteServerFactory {
	return &RemoteServerFactory{
		client:   pb.NewPluginServiceClient(conn),
		conn:     conn,
		metadata: metadata,
	}
}

func (f *RemoteServerFactory) ProtocolType() protocol.ProtocolType {
	return protocol.ProtocolType(f.metadata.ProtocolType)
}

func (f *RemoteServerFactory) DisplayName() string {
	return f.metadata.DisplayName
}

func (f *RemoteServerFactory) CreateServer(config protocol.ProtocolConfig, store protocol.DataStore) (protocol.ProtocolServer, error) {
	// プラグインモデルでは CreateServer と Start を分離しない。
	// ここではサーバーインスタンスを表す RemoteProtocolServer を返す。
	// 実際の起動は Start() 呼び出し時に CreateAndStart RPC を送る。
	return NewRemoteProtocolServer(f.client, f.conn, config), nil
}

func (f *RemoteServerFactory) CreateDataStore() protocol.DataStore {
	return NewRemoteDataStore(pb.NewDataStoreServiceClient(f.conn))
}

func (f *RemoteServerFactory) DefaultConfig() protocol.ProtocolConfig {
	variants := f.ConfigVariants()
	if len(variants) == 0 {
		return &remoteProtocolConfig{protocolType: f.ProtocolType(), variantID: ""}
	}
	return f.CreateConfigFromVariant(variants[0].ID)
}

func (f *RemoteServerFactory) ConfigVariants() []protocol.ConfigVariant {
	resp, err := f.client.GetConfigVariants(backgroundCtx(), &pb.Empty{})
	if err != nil {
		return nil
	}
	variants := make([]protocol.ConfigVariant, len(resp.Variants))
	for i, v := range resp.Variants {
		variants[i] = protocol.ConfigVariant{
			ID:          v.Id,
			DisplayName: v.DisplayName,
		}
	}
	return variants
}

func (f *RemoteServerFactory) CreateConfigFromVariant(variantID string) protocol.ProtocolConfig {
	resp, err := f.client.GetDefaultConfig(backgroundCtx(), &pb.GetDefaultConfigRequest{VariantId: variantID})
	if err != nil {
		return &remoteProtocolConfig{protocolType: f.ProtocolType(), variantID: variantID, settingsJSON: "{}"}
	}
	return &remoteProtocolConfig{
		protocolType: f.ProtocolType(),
		variantID:    resp.VariantId,
		settingsJSON: resp.SettingsJson,
	}
}

func (f *RemoteServerFactory) GetConfigFields(variantID string) []protocol.ConfigField {
	resp, err := f.client.GetConfigFields(backgroundCtx(), &pb.GetConfigFieldsRequest{VariantId: variantID})
	if err != nil {
		return nil
	}
	fields := make([]protocol.ConfigField, len(resp.Fields))
	for i, pbf := range resp.Fields {
		fields[i] = pbConfigFieldToProtocol(pbf)
	}
	return fields
}

func (f *RemoteServerFactory) GetProtocolCapabilities() protocol.ProtocolCapabilities {
	caps := f.metadata.Capabilities
	if caps == nil {
		return protocol.ProtocolCapabilities{}
	}
	return protocol.ProtocolCapabilities{
		SupportsUnitID:         caps.SupportsUnitId,
		UnitIDMin:              int(caps.UnitIdMin),
		UnitIDMax:              int(caps.UnitIdMax),
		SupportsNodePublishing: caps.SupportsNodePublishing,
	}
}

func (f *RemoteServerFactory) ConfigToMap(config protocol.ProtocolConfig) map[string]interface{} {
	rc, ok := config.(*remoteProtocolConfig)
	if !ok {
		return map[string]interface{}{}
	}
	resp, err := f.client.ConfigToMap(backgroundCtx(), &pb.ConfigToMapRequest{
		VariantId:    rc.variantID,
		SettingsJson: rc.settingsJSON,
	})
	if err != nil {
		return map[string]interface{}{}
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(resp.SettingsJson), &result); err != nil {
		return map[string]interface{}{}
	}
	return result
}

func (f *RemoteServerFactory) MapToConfig(variantID string, settings map[string]interface{}) (protocol.ProtocolConfig, error) {
	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return nil, fmt.Errorf("設定の JSON 変換に失敗: %w", err)
	}
	resp, err := f.client.MapToConfig(backgroundCtx(), &pb.MapToConfigRequest{
		VariantId:    variantID,
		SettingsJson: string(settingsJSON),
	})
	if err != nil {
		return nil, fmt.Errorf("MapToConfig RPC 失敗: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("設定検証エラー: %s", resp.Error)
	}
	return &remoteProtocolConfig{
		protocolType: f.ProtocolType(),
		variantID:    resp.VariantId,
		settingsJSON: resp.SettingsJson,
	}, nil
}

// pbConfigFieldToProtocol は protobuf の ConfigField を domain の ConfigField に変換する
func pbConfigFieldToProtocol(pbf *pb.ConfigField) protocol.ConfigField {
	field := protocol.ConfigField{
		Name:     pbf.Name,
		Label:    pbf.Label,
		Type:     pbf.Type,
		Required: pbf.Required,
		Options:  make([]protocol.FieldOption, len(pbf.Options)),
	}
	// デフォルト値を JSON デシリアライズ
	if pbf.DefaultJson != "" {
		var def interface{}
		if err := json.Unmarshal([]byte(pbf.DefaultJson), &def); err == nil {
			field.Default = def
		}
	}
	// オプション
	for j, opt := range pbf.Options {
		field.Options[j] = protocol.FieldOption{Value: opt.Value, Label: opt.Label}
	}
	// Min/Max
	if pbf.HasMin {
		v := int(pbf.Min)
		field.Min = &v
	}
	if pbf.HasMax {
		v := int(pbf.Max)
		field.Max = &v
	}
	// 表示条件
	if pbf.Condition != nil {
		field.Condition = &protocol.FieldCondition{
			Field: pbf.Condition.Field,
			Value: pbf.Condition.Value,
		}
	}
	return field
}
