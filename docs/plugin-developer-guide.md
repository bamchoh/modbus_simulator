# プロトコルプラグイン開発者ガイド

## 概要

PLCシミュレーターのプロトコルプラグインは、独立した実行ファイルとして動作し、gRPC でホストアプリと通信します。

```
ホストアプリ (Wails)
  │
  ├─ PluginProcessManager  ← プラグイン管理
  │    ├─ アプリ起動時: plugin.json を走査してマニフェストを読み込む（プロセスは起動しない）
  │    └─ サーバー追加時: exe をオンデマンドで起動
  │                サーバー削除時: exe を停止
  │
  └─ gRPC クライアント
       ├─ PluginService     ← ファクトリー情報・サーバーライフサイクル
       └─ DataStoreService  ← メモリ読み書き（レジスタベースの場合）

プラグイン exe (独立プロセス)
  └─ gRPC サーバー
       ├─ PluginService     ← 実装必須
       └─ DataStoreService  ← 実装必須（レジスタなしの場合は空実装）
```

> **プロセスライフサイクル**: プラグインプロセスはアプリ起動時には起動しません。ユーザーが UI でサーバーを追加したタイミングで起動し、サーバーを削除したタイミングで停止します。同一プロトコルのサーバーを再追加すると、新しいプロセスが起動します。

---

## ファイル構成

新しいプラグイン `myplugin` を作成する場合：

```
cmd/
└── myplugin-plugin/
    ├── main.go                 # エントリーポイント（定型文）
    ├── internal/
    │   └── myplugin/          # プロトコル実装本体（Go internal/ 制約でプラグイン専用）
    │       ├── factory.go
    │       ├── server.go
    │       └── datastore.go
    └── server/
        └── plugin_server.go   # gRPC サーバー実装（メインの実装対象）

plugins/
└── myplugin-plugin/
    ├── myplugin-plugin.exe    # task plugins でビルド
    └── plugin.json            # マニフェスト
```

> **配置方針**: プロトコル実装は `cmd/<plugin>/internal/` に置きます。Go の `internal/` パッケージ制約により、ホストアプリが誤ってプロトコル固有コードに依存することをコンパイル時に防止できます。

---

## Step 1: plugin.json を作成する

`plugins/myplugin-plugin/plugin.json`:

```json
{
  "name": "MyProtocol Plugin",
  "entrypoint": "myplugin-plugin.exe",
  "version": "0.0.1",
  "author": "Your Name",
  "description": "MyProtocol implementation",
  "protocol_type": "myplugin",
  "display_name": "My Protocol",
  "variants": [
    {"id": "tcp", "display_name": "MyProtocol TCP"},
    {"id": "serial", "display_name": "MyProtocol Serial"}
  ],
  "capabilities": {
    "supports_unit_id": false,
    "supports_node_publishing": false
  }
}
```

| フィールド | 必須 | 説明 |
|-----------|------|------|
| `name` | ○ | プラグイン名（内部用） |
| `entrypoint` | ○ | 実行ファイル名（plugin.json と同じディレクトリ） |
| `version` | ○ | バージョン文字列 |
| `protocol_type` | ○ | プロトコルタイプ（ホストの識別キー。既存と重複不可） |
| `display_name` | ○ | UI 表示名 |
| `variants` | ○ | バリアント一覧（接続方式の違い）。1種類のみなら空配列 `[]` |
| `capabilities.supports_unit_id` | - | UnitID 対応（省略時 false） |
| `capabilities.unit_id_min` | - | UnitID の最小値（`supports_unit_id` が true の場合） |
| `capabilities.unit_id_max` | - | UnitID の最大値 |
| `capabilities.supports_node_publishing` | - | ノードベース公開対応（OPC UA 等。省略時 false） |
| `author` | - | 作者（省略可） |
| `description` | - | 説明（省略可） |

> **重要**: `protocol_type` は既存の `"modbus-tcp"`, `"modbus-rtu"`, `"modbus-ascii"`, `"opcua"` と衝突しない値を使ってください。ホストがサーバーを識別するキーです。

> **`plugin.json` と gRPC の整合性**: `protocol_type` / `display_name` / `variants` / `capabilities` はホストが **プロセスを起動せずに** 読み取るため、`GetMetadata()` / `GetConfigVariants()` の返す値と一致させてください。

---

## Step 2: main.go を作成する

`cmd/myplugin-plugin/main.go` は定型文です。内容を変える必要はありません：

```go
package main

import (
    "fmt"
    "net"
    "os"
    "os/signal"
    "syscall"

    "google.golang.org/grpc"

    "modbus_simulator/cmd/myplugin-plugin/server"
)

func main() {
    // ランダムな空きポートで gRPC サーバーを起動
    lis, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil {
        fmt.Fprintf(os.Stderr, "[ERROR] gRPC リスナー起動失敗: %v\n", err)
        os.Exit(1)
    }
    port := lis.Addr().(*net.TCPAddr).Port

    grpcServer := grpc.NewServer()
    pluginSrv := server.NewPluginServer()
    pluginSrv.Register(grpcServer)

    go func() {
        if err := grpcServer.Serve(lis); err != nil {
            fmt.Fprintf(os.Stderr, "[ERROR] gRPC サーバーエラー: %v\n", err)
        }
    }()

    // ★ この行が必須: ホストがポート番号を読み取る
    fmt.Printf("GRPC_PORT=%d\n", port)

    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
    <-sigCh

    grpcServer.GracefulStop()
}
```

> **重要**: `fmt.Printf("GRPC_PORT=%d\n", port)` は必ず stdout に出力してください。ホストはこの行を読み取って gRPC 接続先を特定します。出力前にブロックすると10秒でタイムアウトします。

---

## Step 3: PluginServer を実装する

`cmd/myplugin-plugin/server/plugin_server.go` がメインの実装対象です。

### プロトコルの分類

実装パターンはプロトコルの性質によって2種類あります：

| 分類 | 例 | DataStore | `SupportsNodePublishing` |
|------|----|-----------|--------------------------|
| **レジスタ型** | Modbus, FINS, SLMP | 実装する | `false` |
| **変数型** | OPC UA, MQTT | 空実装でよい | `true` |

---

### レジスタ型プラグインの実装（Modbus 参考）

```go
package server

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"

    "google.golang.org/grpc"

    pb "modbus_simulator/pb/pluginpb"
    "modbus_simulator/internal/domain/protocol"
    "modbus_simulator/cmd/myplugin-plugin/internal/myplugin"
)

type PluginServer struct {
    pb.UnimplementedPluginServiceServer
    pb.UnimplementedDataStoreServiceServer

    mu      sync.Mutex
    factory protocol.ServerFactory
    store   *myplugin.MyDataStore
    server  protocol.ProtocolServer

    subsMu      sync.RWMutex
    subscribers []chan *pb.DataChange
    hostWriting bool
}

func NewPluginServer() *PluginServer {
    return &PluginServer{}
}

func (s *PluginServer) Register(srv *grpc.Server) {
    pb.RegisterPluginServiceServer(srv, s)
    pb.RegisterDataStoreServiceServer(srv, s)
}
```

#### GetMetadata: プロトコル情報を返す

```go
func (s *PluginServer) GetMetadata(ctx context.Context, _ *pb.Empty) (*pb.PluginMetadata, error) {
    return &pb.PluginMetadata{
        ProtocolType: "myplugin",           // ★ ProtocolType（一意である必要あり）
        DisplayName:  "My Protocol",
        Capabilities: &pb.ProtocolCapabilities{
            SupportsUnitId:         false,
            SupportsNodePublishing: false,  // レジスタ型は false
        },
    }, nil
}
```

`ProtocolType` はホストが各サーバーを識別するキーです。`plugin.json` の `protocol_type` と一致させてください。

#### GetConfigVariants: バリアント一覧を返す

バリアントとは、同一プロトコルの接続方式の違いです（例: TCP と RTU）。バリアントが1種類のみなら空スライスを返しても構いません。

```go
func (s *PluginServer) GetConfigVariants(ctx context.Context, _ *pb.Empty) (*pb.GetConfigVariantsResponse, error) {
    factory := myplugin.NewServerFactory()
    variants := factory.ConfigVariants()
    pbVariants := make([]*pb.ConfigVariant, len(variants))
    for i, v := range variants {
        pbVariants[i] = &pb.ConfigVariant{Id: v.ID, DisplayName: v.DisplayName}
    }
    return &pb.GetConfigVariantsResponse{Variants: pbVariants}, nil
}
```

#### GetConfigFields: UI フォームのスキーマを返す

```go
func (s *PluginServer) GetConfigFields(ctx context.Context, req *pb.GetConfigFieldsRequest) (*pb.GetConfigFieldsResponse, error) {
    factory := myplugin.NewServerFactory()
    fields := factory.GetConfigFields(req.VariantId)
    pbFields := make([]*pb.ConfigField, len(fields))
    for i, f := range fields {
        pbF := &pb.ConfigField{
            Name:     f.Name,
            Label:    f.Label,
            Type:     f.Type,      // "text", "number", "select", "checkbox"
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
```

#### CreateAndStart: サーバーを起動する

```go
func (s *PluginServer) CreateAndStart(ctx context.Context, req *pb.CreateAndStartRequest) (*pb.Empty, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    factory := myplugin.NewServerFactory()

    // 設定を復元
    var config protocol.ProtocolConfig
    if req.SettingsJson != "" {
        var settings map[string]interface{}
        if err := json.Unmarshal([]byte(req.SettingsJson), &settings); err != nil {
            return nil, fmt.Errorf("設定のパース失敗: %w", err)
        }
        var err error
        config, err = factory.MapToConfig(req.VariantId, settings)
        if err != nil {
            return nil, fmt.Errorf("設定の変換失敗: %w", err)
        }
    } else {
        config = factory.CreateConfigFromVariant(req.VariantId)
    }

    // DataStore を作成して変更フックを設定
    innerStore := factory.CreateDataStore()
    myStore := innerStore.(*myplugin.MyDataStore)
    s.store = myStore
    s.store.SetChangeHook(s.onDataChange)  // ★ クライアント書き込みをホストに通知

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
```

#### Stop / GetStatus / UpdateConfig

```go
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

    if s.server == nil || s.factory == nil {
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
    return &pb.Empty{}, s.server.UpdateConfig(config)
}

// OnNodePublishingUpdated: レジスタ型は何もしなくてよい
func (s *PluginServer) OnNodePublishingUpdated(ctx context.Context, _ *pb.Empty) (*pb.Empty, error) {
    return &pb.Empty{}, nil
}
```

#### DataStoreService の実装

```go
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
    // ★ ホストからの書き込みフラグを立てて循環通知を防止
    s.setHostWriting(true)
    err := s.store.WriteWord(req.Area, req.Address, uint16(req.Value))
    s.setHostWriting(false)
    return &pb.Empty{}, err
}

// ... ReadBit, WriteBit, ReadBits, WriteBits, ReadWords, WriteWords, Snapshot, Restore, ClearAll も同様に実装
```

> **重要**: ホスト（`WriteWord` 等）からの書き込みには必ず `hostWriting` フラグを立ててください。これを怠ると、ホスト書き込み → 変更通知 → ホスト書き込み の無限ループが発生します。

#### SubscribeChanges: クライアント書き込み通知ストリーム

```go
// SubscribeChanges はプロトコルクライアントが書き込んだ変更をストリームで送信する
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

// onDataChange は DataStore の変更フックから呼ばれる（プロトコルクライアントの書き込み時のみ）
func (s *PluginServer) onDataChange(area string, address uint32, values []uint16, isBit bool, bitValues []bool) {
    if s.isHostWriting() {
        return  // ホストからの書き込みは通知しない
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
```

---

### 変数型プラグインの実装（OPC UA 参考）

`SupportsNodePublishing: true` のプロトコル（ノードベース）は DataStore を持たないため、DataStoreService は空実装で構いません。代わりに、ホストの VariableStore に gRPC 経由でアクセスします。

#### CreateAndStart での VariableStoreAccessor 接続

```go
func (s *PluginServer) CreateAndStart(ctx context.Context, req *pb.CreateAndStartRequest) (*pb.Empty, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    // ★ ホストの VariableAccessorService に接続して accessor を注入
    if req.HostGrpcAddr != "" {
        conn, err := grpc.NewClient(req.HostGrpcAddr,
            grpc.WithTransportCredentials(insecure.NewCredentials()))
        if err != nil {
            return nil, fmt.Errorf("HostGrpcServer への接続失敗: %w", err)
        }
        accessor := newRemoteVariableStoreAccessor(pb.NewVariableAccessorServiceClient(conn))
        s.factory.InjectVariableStore(accessor)
    }

    // ... 以降は通常のサーバー作成・起動
}
```

#### OnNodePublishingUpdated: ノード再構築通知を処理する

```go
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
```

#### DataStoreService の空実装

```go
func (s *PluginServer) GetAreas(_ context.Context, _ *pb.Empty) (*pb.GetAreasResponse, error) {
    return &pb.GetAreasResponse{}, nil
}
func (s *PluginServer) Snapshot(_ context.Context, _ *pb.Empty) (*pb.SnapshotResponse, error) {
    return &pb.SnapshotResponse{}, nil
}
func (s *PluginServer) Restore(_ context.Context, _ *pb.RestoreRequest) (*pb.Empty, error) {
    return &pb.Empty{}, nil
}
func (s *PluginServer) ClearAll(_ context.Context, _ *pb.Empty) (*pb.Empty, error) {
    return &pb.Empty{}, nil
}
func (s *PluginServer) SubscribeChanges(_ *pb.Empty, stream pb.DataStoreService_SubscribeChangesServer) error {
    // 変更通知なし。クライアントが切断するまで待つだけ
    <-stream.Context().Done()
    return nil
}
```

#### RemoteVariableStoreAccessor の実装

ホストの変数ストアに gRPC 経由でアクセスするアダプター：

```go
type remoteVariableStoreAccessor struct {
    client pb.VariableAccessorServiceClient
}

func newRemoteVariableStoreAccessor(client pb.VariableAccessorServiceClient) *remoteVariableStoreAccessor {
    return &remoteVariableStoreAccessor{client: client}
}

func (a *remoteVariableStoreAccessor) GetEnabledNodePublishings(protocolType string) []protocol.NodePublishingInfo {
    resp, err := a.client.GetEnabledNodePublishings(context.Background(),
        &pb.GetNodePublishingsRequest{ProtocolType: protocolType})
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
    resp, err := a.client.ReadVariableValue(context.Background(),
        &pb.ReadVariableValueRequest{VariableId: variableID})
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
    _, err = a.client.WriteVariableValue(context.Background(),
        &pb.WriteVariableValueRequest{VariableId: variableID, ValueJson: string(valueJSON)})
    return err
}

func (a *remoteVariableStoreAccessor) GetStructFields(typeName string) []protocol.StructFieldInfo {
    resp, err := a.client.GetStructFields(context.Background(),
        &pb.GetStructFieldsRequest{TypeName: typeName})
    if err != nil {
        return nil
    }
    result := make([]protocol.StructFieldInfo, len(resp.Fields))
    for i, f := range resp.Fields {
        result[i] = protocol.StructFieldInfo{Name: f.Name, DataType: f.DataType}
    }
    return result
}
```

---

## Step 4: Taskfile.yaml にビルドタスクを追加する

`Taskfile.yaml` の `plugins` タスクに追記します：

```yaml
tasks:
  plugins:
    cmds:
      # ... 既存の modbus-plugin, opcua-plugin ...
      - powershell -Command "mkdir -p {{.PLUGINS_DIR}}/myplugin-plugin"
      - go build -o {{.PLUGINS_DIR}}/myplugin-plugin/myplugin-plugin.exe ./cmd/myplugin-plugin
      - |
        printf '{\n  "name": "MyProtocol Plugin",\n  "entrypoint": "myplugin-plugin.exe",\n  "version": "0.0.1",\n  "protocol_type": "myplugin",\n  "display_name": "My Protocol",\n  "variants": [],\n  "capabilities": {}\n}\n' > {{.PLUGINS_DIR}}/myplugin-plugin/plugin.json
```

その後ビルドします：

```bash
task plugins   # プラグインバイナリをビルド
task dev       # 開発サーバー起動（plugins が先に実行される）
```

---

## gRPC メッセージリファレンス

### PluginService RPC 一覧

| RPC | 説明 | 実装必須 |
|-----|------|---------|
| `GetMetadata` | プロトコル種別・表示名・機能情報を返す | ○ |
| `GetConfigVariants` | バリアント一覧を返す | ○ |
| `GetConfigFields` | UI フォームのスキーマを返す | ○ |
| `GetDefaultConfig` | デフォルト設定 JSON を返す | ○ |
| `MapToConfig` | JSON → 設定オブジェクト変換（バリデーション含む） | ○ |
| `ConfigToMap` | 設定オブジェクト → JSON 変換 | ○ |
| `CreateAndStart` | サーバーを作成して起動する | ○ |
| `Stop` | サーバーを停止する | ○ |
| `GetStatus` | `"Stopped"` / `"Running"` / `"Error"` を返す | ○ |
| `UpdateConfig` | サーバー設定を更新する（停止中のみ） | ○ |
| `OnNodePublishingUpdated` | 変数公開設定変更を通知（変数型のみ有効） | ○（空実装可） |
| `GetUnitIDSettings` | 無効 UnitID 一覧を返す（UnitID 対応のみ） | △ |
| `SetUnitIDEnabled` | 特定 UnitID の有効/無効を切り替える | △ |
| `SetDisabledUnitIDs` | 無効 UnitID を一括設定する | △ |

### DataStoreService RPC 一覧

| RPC | 説明 | レジスタ型 | 変数型 |
|-----|------|-----------|--------|
| `GetAreas` | メモリエリア定義を返す | 実装 | 空実装 |
| `ReadBit` / `WriteBit` | ビット単位読み書き | 実装 | 空実装 |
| `ReadBits` / `WriteBits` | ビット一括読み書き | 実装 | 空実装 |
| `ReadWord` / `WriteWord` | ワード単位読み書き | 実装 | 空実装 |
| `ReadWords` / `WriteWords` | ワード一括読み書き | 実装 | 空実装 |
| `Snapshot` | Export 用スナップショット取得 | 実装 | 空実装 |
| `Restore` | Import 用スナップショット復元 | 実装 | 空実装 |
| `ClearAll` | 全メモリを初期化 | 実装 | 空実装 |
| `SubscribeChanges` | クライアント書き込み変更通知ストリーム | 実装 | 空実装（待機のみ） |

### MemoryArea フィールド

```protobuf
message MemoryArea {
  string id = 1;             // 内部識別子（API で使用。例: "holdingRegisters"）
  string display_name = 2;   // UI 表示名（例: "保持レジスタ (4x)"）
  bool is_bit = 3;           // true: ビットエリア / false: ワードエリア
  uint32 size = 4;           // エリアサイズ（ビット数またはワード数）
  bool read_only = 5;        // true: 読み取り専用
  bool byte_addressing = 6;  // true: バイト単位アドレッシング
  bool one_origin = 7;       // true: UI 表示を 1 オリジン（内部は常に 0 ベース）
}
```

### ConfigField フィールド

```protobuf
message ConfigField {
  string name = 1;           // フィールド名（設定 map のキー）
  string label = 2;          // UI ラベル
  string type = 3;           // "text" / "number" / "select" / "checkbox"
  bool required = 4;         // 必須フラグ
  string default_json = 5;   // デフォルト値（JSON 文字列）
  repeated FieldOption options = 6;   // type="select" の選択肢
  bool has_min = 7;          // min 値が有効かどうか
  int32 min = 8;             // type="number" の最小値
  bool has_max = 9;          // max 値が有効かどうか
  int32 max = 10;            // type="number" の最大値
  FieldCondition condition = 11;  // 表示条件（別フィールドの値で制御）
}
```

---

## よくある実装ミス

### ❌ GRPC_PORT の出力忘れ

```go
// NG: fmt.Printf("GRPC_PORT=%d\n", port) が抜けている
// → ホストが10秒待ってタイムアウトし、プラグインが起動しない
```

### ❌ hostWriting フラグなしで WriteWord を実装する

```go
// NG:
func (s *PluginServer) WriteWord(...) {
    s.store.WriteWord(req.Area, req.Address, uint16(req.Value))
    // → 変更フックが発火 → SubscribeChanges に通知 → ホストが再書き込み → 無限ループ
}

// OK:
func (s *PluginServer) WriteWord(...) {
    s.setHostWriting(true)
    s.store.WriteWord(req.Area, req.Address, uint16(req.Value))
    s.setHostWriting(false)
}
```

### ❌ UnimplementedXxxServer の埋め込み忘れ

```go
// NG: 未実装 RPC を呼ばれると panic
type PluginServer struct{}

// OK: 未実装 RPC はデフォルトで Unimplemented エラーを返す
type PluginServer struct {
    pb.UnimplementedPluginServiceServer
    pb.UnimplementedDataStoreServiceServer
}
```

### ❌ MapToConfig でエラーを gRPC エラーとして返す

```go
// NG: バリデーションエラーを gRPC エラーとして返すと、ホストが設定保存できない
config, err := factory.MapToConfig(...)
if err != nil {
    return nil, err  // gRPC エラーになる
}

// OK: MapToConfigResponse.Error フィールドに詰めて返す
config, err := factory.MapToConfig(...)
if err != nil {
    return &pb.MapToConfigResponse{Error: err.Error()}, nil
}
```

---

## proto 定義の場所

`proto/plugin.proto` が唯一の定義ファイルです。変更後は以下で再生成してください：

```bash
task proto
```

生成先: `pb/pluginpb/` （コミット対象）

> **注意**: `pb/pluginpb/` のファイルは手動で編集しないでください。`task proto` で常に上書きされます。
