package opcua

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gopcua/opcua/id"
	goserver "github.com/gopcua/opcua/server"
	"github.com/gopcua/opcua/server/attrs"
	"github.com/gopcua/opcua/ua"

	"modbus_simulator/internal/domain/protocol"
)

// ===== 設定 =====

// OpcuaConfig は OPC UA サーバーの設定
type OpcuaConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

func defaultOpcuaConfig() *OpcuaConfig {
	return &OpcuaConfig{Host: "0.0.0.0", Port: 4840}
}

func (c *OpcuaConfig) ProtocolType() protocol.ProtocolType { return "opcua" }
func (c *OpcuaConfig) Variant() string                     { return "opcua" }

func (c *OpcuaConfig) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("host is required")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

func (c *OpcuaConfig) Clone() protocol.ProtocolConfig {
	cp := *c
	return &cp
}

// ===== サーバー =====

// OpcuaServer は OPC UA サーバーの実装
type OpcuaServer struct {
	mu       sync.Mutex
	config   *OpcuaConfig
	accessor protocol.VariableStoreAccessor
	srv      *goserver.Server
	ns       *PLCNameSpace
	cancel   context.CancelFunc
	status   protocol.ServerStatus
}

// NodePublishingAware インターフェース確認
var _ protocol.NodePublishingAware = (*OpcuaServer)(nil)


func newOpcuaServer(config *OpcuaConfig, accessor protocol.VariableStoreAccessor) *OpcuaServer {
	return &OpcuaServer{
		config:   config,
		accessor: accessor,
		status:   protocol.StatusStopped,
	}
}

func (s *OpcuaServer) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == protocol.StatusRunning {
		return fmt.Errorf("OPC UA server is already running")
	}

	srv := goserver.New(
		goserver.EndPoint(s.config.Host, s.config.Port),
		goserver.EnableSecurity("None", ua.MessageSecurityModeNone),
		goserver.EnableAuthMode(ua.UserTokenTypeAnonymous),
	)

	ns := newPLCNameSpace(srv, s.accessor, "opcua")
	srv.AddNamespace(ns)

	// ns=0 の ObjectsFolder に PLCVariables フォルダへの Organizes 参照を追加する。
	// これにより OPC UA クライアントが標準ブラウズパスで変数を発見できる。
	if ns0, err := srv.Namespace(0); err == nil {
		if n0, ok := ns0.(*goserver.NodeNameSpace); ok {
			if objFolder := n0.Objects(); objFolder != nil {
				objFolder.AddRef(ns.Objects(), goserver.RefTypeIDOrganizes, true)
			}
		}
	}

	srvCtx, cancel := context.WithCancel(ctx)
	if err := srv.Start(srvCtx); err != nil {
		cancel()
		s.status = protocol.StatusError
		return fmt.Errorf("OPC UA server failed to start: %w", err)
	}

	// サブスクリプション向けに変数値変化を定期通知するゴルーチンを起動
	go ns.pollChanges(srvCtx)

	s.srv = srv
	s.ns = ns
	s.cancel = cancel
	s.status = protocol.StatusRunning
	return nil
}

func (s *OpcuaServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status != protocol.StatusRunning {
		return nil
	}

	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	if s.srv != nil {
		_ = s.srv.Close()
		s.srv = nil
	}
	s.ns = nil
	s.status = protocol.StatusStopped
	return nil
}

func (s *OpcuaServer) Status() protocol.ServerStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *OpcuaServer) ProtocolType() protocol.ProtocolType {
	return "opcua"
}

func (s *OpcuaServer) Config() protocol.ProtocolConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config
}

func (s *OpcuaServer) UpdateConfig(config protocol.ProtocolConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == protocol.StatusRunning {
		return fmt.Errorf("cannot update config while server is running")
	}

	cfg, ok := config.(*OpcuaConfig)
	if !ok {
		return fmt.Errorf("invalid config type")
	}
	s.config = cfg
	return nil
}

// OnNodePublishingUpdated はノード公開設定変更通知（NodePublishingAware 実装）
func (s *OpcuaServer) OnNodePublishingUpdated() {
	s.mu.Lock()
	ns := s.ns
	srv := s.srv
	s.mu.Unlock()

	if ns != nil && srv != nil {
		ns.rebuildNodes(srv)
	}
}

// ===== PLCNameSpace =====

// plcVarInfo は公開中の変数の情報
type plcVarInfo struct {
	name       string
	accessMode string // "read" | "write" | "readwrite"
	dataType   string
	isArray    bool
	arraySize  int32
	elemType   string // 配列の場合の要素型（"INT" など）
}

// pathSegment はノードパスの1要素を表す（構造体フィールドまたは配列インデックス）
type pathSegment struct {
	kind  string // "field" または "index"
	field string // kind=="field" のときのフィールド名
	index int    // kind=="index" のときの配列インデックス
}

// PLCNameSpace はアクセサーから動的に値を読み書きするカスタム OPC UA NameSpace
type PLCNameSpace struct {
	mu           sync.RWMutex
	nsID         uint16
	protocolType string
	accessor     protocol.VariableStoreAccessor
	srv          *goserver.Server // ChangeNotification 呼び出し用

	// variableID → plcVarInfo
	vars map[string]*plcVarInfo
	// pollChanges で通知する全 NodeID（子ノードを含む）
	allNodeIDs []string
}

// goserver.NameSpace インターフェース実装確認
var _ goserver.NameSpace = (*PLCNameSpace)(nil)

func newPLCNameSpace(srv *goserver.Server, accessor protocol.VariableStoreAccessor, protocolType string) *PLCNameSpace {
	ns := &PLCNameSpace{
		protocolType: protocolType,
		accessor:     accessor,
		srv:          srv,
		vars:         make(map[string]*plcVarInfo),
	}
	// accessor が nil でなければ初期データを取得
	if accessor != nil {
		ns.loadFromAccessor()
	}
	return ns
}

// loadFromAccessor は accessor から公開中変数リストを読み込む（ロック外で呼ぶこと）
func (ns *PLCNameSpace) loadFromAccessor() {
	if ns.accessor == nil {
		return
	}
	infos := ns.accessor.GetEnabledNodePublishings(ns.protocolType)
	newVars := make(map[string]*plcVarInfo, len(infos))
	for _, info := range infos {
		v := &plcVarInfo{
			name:       info.VariableName,
			accessMode: info.AccessMode,
			dataType:   info.DataType,
		}
		v.elemType, v.arraySize, v.isArray = parseArrayType(info.DataType)
		newVars[info.VariableID] = v
	}
	// 全 NodeID（子ノードを含む）を事前計算して pollChanges で使用
	var allIDs []string
	for varID, v := range newVars {
		allIDs = append(allIDs, ns.collectAllNodeIDs(varID, v.dataType, varID)...)
	}
	ns.mu.Lock()
	ns.vars = newVars
	ns.allNodeIDs = allIDs
	ns.mu.Unlock()
}

// collectAllNodeIDs は指定ノード（nodeStr）とその子ノード全ての NodeID 文字列を返す
func (ns *PLCNameSpace) collectAllNodeIDs(varID, dataType, nodeStr string) []string {
	result := []string{nodeStr}
	elemType, lower, size, isArr := parseArrayTypeFull(dataType)
	if isArr {
		for i := 0; i < int(size); i++ {
			child := fmt.Sprintf("%s[%d]", nodeStr, int(lower)+i)
			result = append(result, ns.collectAllNodeIDs(varID, elemType, child)...)
		}
		return result
	}
	if isStructDataType(dataType) && ns.accessor != nil {
		for _, f := range ns.accessor.GetStructFields(dataType) {
			child := nodeStr + "." + f.Name
			result = append(result, ns.collectAllNodeIDs(varID, f.DataType, child)...)
		}
	}
	return result
}

// rebuildNodes は公開変数リストを再構築して OPC UA クライアントに変更通知する
func (ns *PLCNameSpace) rebuildNodes(srv *goserver.Server) {
	ns.loadFromAccessor()
	// ObjectsFolder への変更通知（クライアントが再ブラウズするよう促す）
	srv.ChangeNotification(ua.NewNumericNodeID(ns.nsID, uint32(id.ObjectsFolder)))
}

// pollChanges は公開中の全変数（子ノード含む）に対して定期的に ChangeNotification を呼び出す。
// これにより OPC UA クライアントのサブスクリプションが最新値を受け取れる。
func (ns *PLCNameSpace) pollChanges(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ns.mu.RLock()
			nodeIDs := make([]string, len(ns.allNodeIDs))
			copy(nodeIDs, ns.allNodeIDs)
			ns.mu.RUnlock()
			for _, nid := range nodeIDs {
				ns.srv.ChangeNotification(ua.NewStringNodeID(ns.nsID, nid))
			}
		}
	}
}

// --- NameSpace インターフェース実装 ---

func (ns *PLCNameSpace) Name() string   { return "urn:modbus-simulator:plc" }
func (ns *PLCNameSpace) ID() uint16     { return ns.nsID }
func (ns *PLCNameSpace) SetID(i uint16) { ns.nsID = i }

func (ns *PLCNameSpace) AddNode(n *goserver.Node) *goserver.Node { return n }

// Node はノードID に対応するノードを返す。
// ns=0 の Browse() が td.DataType() を呼ぶ際にパニックしないよう、
// ObjectsFolder と変数ノードに対して有効なノードを返す。
func (ns *PLCNameSpace) Node(nodeID *ua.NodeID) *goserver.Node {
	if nodeID == nil {
		return nil
	}
	// ObjectsFolder (i=85) → PLCVariables フォルダノードを返す
	if nodeID.IntID() == uint32(id.ObjectsFolder) {
		return ns.Objects()
	}
	// 文字列 NodeID
	varID := nodeID.StringID()
	if varID == "" {
		return nil
	}
	// DataType ノード (_dt_ プレフィックス) → 構造体型の DataType ノードを返す
	if strings.HasPrefix(varID, "_dt_") {
		typeName := strings.TrimPrefix(varID, "_dt_")
		return goserver.NewNode(nodeID,
			map[ua.AttributeID]*ua.DataValue{
				ua.AttributeIDNodeClass:   goserver.DataValueFromValue(int32(ua.NodeClassDataType)),
				ua.AttributeIDBrowseName:  goserver.DataValueFromValue(attrs.BrowseName(typeName)),
				ua.AttributeIDDisplayName: goserver.DataValueFromValue(attrs.DisplayName(typeName, "")),
			}, nil, nil)
	}
	// 変数ノードまたは子ノード
	varID2, path2 := parseNodePath(varID)
	ns.mu.RLock()
	info, ok := ns.vars[varID2]
	ns.mu.RUnlock()
	if !ok {
		return nil
	}
	currentType := info.dataType
	displayName := info.name
	if len(path2) > 0 {
		currentType = ns.followTypeForPath(info.dataType, path2)
		if currentType == "" {
			return nil
		}
		displayName = pathDisplayName(path2)
	}
	dtNodeID := ns.dataTypeNodeID(currentType)
	typedef := toExpandedNodeID(dtNodeID)
	return goserver.NewNode(nodeID,
		map[ua.AttributeID]*ua.DataValue{
			ua.AttributeIDDataType:    goserver.DataValueFromValue(typedef),
			ua.AttributeIDNodeClass:   goserver.DataValueFromValue(int32(ua.NodeClassVariable)),
			ua.AttributeIDBrowseName:  goserver.DataValueFromValue(attrs.BrowseName(displayName)),
			ua.AttributeIDDisplayName: goserver.DataValueFromValue(attrs.DisplayName(displayName, "")),
		}, nil, nil)
}

func (ns *PLCNameSpace) Objects() *goserver.Node {
	oid := ua.NewNumericNodeID(ns.nsID, uint32(id.ObjectsFolder))
	typedef := ua.NewNumericExpandedNodeID(0, uint32(id.FolderType))
	return goserver.NewNode(
		oid,
		map[ua.AttributeID]*ua.DataValue{
			ua.AttributeIDNodeClass:     goserver.DataValueFromValue(int32(ua.NodeClassObject)),
			ua.AttributeIDBrowseName:    goserver.DataValueFromValue(attrs.BrowseName("PLCVariables")),
			ua.AttributeIDDisplayName:   goserver.DataValueFromValue(attrs.DisplayName("PLCVariables", "")),
			ua.AttributeIDDataType:      goserver.DataValueFromValue(typedef),
			ua.AttributeIDEventNotifier: goserver.DataValueFromValue(int16(0)),
		},
		[]*ua.ReferenceDescription{},
		nil,
	)
}

func (ns *PLCNameSpace) Root() *goserver.Node {
	return goserver.NewNode(
		ua.NewNumericNodeID(ns.nsID, uint32(id.RootFolder)),
		map[ua.AttributeID]*ua.DataValue{
			ua.AttributeIDNodeClass:   goserver.DataValueFromValue(int32(ua.NodeClassObject)),
			ua.AttributeIDBrowseName:  goserver.DataValueFromValue(attrs.BrowseName("Root")),
			ua.AttributeIDDisplayName: goserver.DataValueFromValue(attrs.DisplayName("Root", "")),
		},
		[]*ua.ReferenceDescription{},
		nil,
	)
}

func (ns *PLCNameSpace) Browse(bd *ua.BrowseDescription) *ua.BrowseResult {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	nodeIntID := bd.NodeID.IntID()
	nodeStrID := bd.NodeID.StringID()

	if nodeIntID == uint32(id.RootFolder) {
		expoid := ua.NewNumericExpandedNodeID(ns.nsID, uint32(id.ObjectsFolder))
		return &ua.BrowseResult{
			StatusCode: ua.StatusGood,
			References: []*ua.ReferenceDescription{{
				ReferenceTypeID: ua.NewNumericNodeID(ns.nsID, uint32(id.ObjectsFolder)),
				NodeID:          expoid,
				BrowseName:      &ua.QualifiedName{NamespaceIndex: ns.nsID, Name: "Objects"},
				DisplayName:     &ua.LocalizedText{EncodingMask: ua.LocalizedTextText, Text: "Objects"},
				TypeDefinition:  expoid,
			}},
		}
	}

	if nodeIntID == uint32(id.ObjectsFolder) {
		// ObjectsFolder: 公開中変数を子ノードとして返す
		varTypedef := ua.NewNumericExpandedNodeID(0, uint32(id.BaseDataVariableType))
		refs := make([]*ua.ReferenceDescription, 0, len(ns.vars))
		for varID, info := range ns.vars {
			expnewid := ua.NewStringExpandedNodeID(ns.nsID, varID)
			refs = append(refs, &ua.ReferenceDescription{
				ReferenceTypeID: ua.NewNumericNodeID(0, uint32(id.HasComponent)),
				IsForward:       true,
				NodeID:          expnewid,
				BrowseName:      &ua.QualifiedName{NamespaceIndex: ns.nsID, Name: info.name},
				DisplayName:     &ua.LocalizedText{EncodingMask: ua.LocalizedTextText, Text: info.name},
				NodeClass:       ua.NodeClassVariable,
				TypeDefinition:  varTypedef,
			})
		}
		return &ua.BrowseResult{StatusCode: ua.StatusGood, References: refs}
	}

	// DataType ノード・空文字列はブラウズ結果なし
	if nodeStrID == "" || strings.HasPrefix(nodeStrID, "_dt_") {
		return &ua.BrowseResult{StatusCode: ua.StatusGood, References: []*ua.ReferenceDescription{}}
	}

	// 変数ノードまたは子ノード: 子を列挙する
	varID, path := parseNodePath(nodeStrID)
	info, ok := ns.vars[varID]
	if !ok {
		return &ua.BrowseResult{StatusCode: ua.StatusGood, References: []*ua.ReferenceDescription{}}
	}
	currentType := ns.followTypeForPath(info.dataType, path)
	refs := ns.browseChildRefs(nodeStrID, currentType)
	return &ua.BrowseResult{StatusCode: ua.StatusGood, References: refs}
}

// browseChildRefs は親ノードの型に応じて子ノードの ReferenceDescription を生成する
func (ns *PLCNameSpace) browseChildRefs(parentNodeStr, parentType string) []*ua.ReferenceDescription {
	varTypedef := ua.NewNumericExpandedNodeID(0, uint32(id.BaseDataVariableType))

	_, lower, size, isArr := parseArrayTypeFull(parentType)
	if isArr {
		refs := make([]*ua.ReferenceDescription, int(size))
		for i := 0; i < int(size); i++ {
			idx := int(lower) + i
			childStr := fmt.Sprintf("%s[%d]", parentNodeStr, idx)
			name := fmt.Sprintf("[%d]", idx)
			refs[i] = &ua.ReferenceDescription{
				ReferenceTypeID: ua.NewNumericNodeID(0, uint32(id.HasComponent)),
				IsForward:       true,
				NodeID:          ua.NewStringExpandedNodeID(ns.nsID, childStr),
				BrowseName:      &ua.QualifiedName{NamespaceIndex: ns.nsID, Name: name},
				DisplayName:     &ua.LocalizedText{EncodingMask: ua.LocalizedTextText, Text: name},
				NodeClass:       ua.NodeClassVariable,
				TypeDefinition:  varTypedef,
			}
		}
		return refs
	}

	if isStructDataType(parentType) && ns.accessor != nil {
		fields := ns.accessor.GetStructFields(parentType)
		refs := make([]*ua.ReferenceDescription, len(fields))
		for i, f := range fields {
			childStr := parentNodeStr + "." + f.Name
			refs[i] = &ua.ReferenceDescription{
				ReferenceTypeID: ua.NewNumericNodeID(0, uint32(id.HasComponent)),
				IsForward:       true,
				NodeID:          ua.NewStringExpandedNodeID(ns.nsID, childStr),
				BrowseName:      &ua.QualifiedName{NamespaceIndex: ns.nsID, Name: f.Name},
				DisplayName:     &ua.LocalizedText{EncodingMask: ua.LocalizedTextText, Text: f.Name},
				NodeClass:       ua.NodeClassVariable,
				TypeDefinition:  varTypedef,
			}
		}
		return refs
	}

	return []*ua.ReferenceDescription{}
}

func (ns *PLCNameSpace) Attribute(n *ua.NodeID, a ua.AttributeID) *ua.DataValue {
	// 数値 NodeID は ObjectsFolder の属性
	if n.IntID() != 0 {
		if n.IntID() == uint32(id.ObjectsFolder) {
			attrval, err := ns.Objects().Attribute(a)
			if err != nil {
				return errDV(ua.StatusBadAttributeIDInvalid)
			}
			return attrval.Value
		}
		return errDV(ua.StatusBadNodeIDInvalid)
	}

	varID := n.StringID()

	// DataType ノード (_dt_ プレフィックス) の属性要求
	if strings.HasPrefix(varID, "_dt_") {
		typeName := strings.TrimPrefix(varID, "_dt_")
		dv := &ua.DataValue{
			EncodingMask:    ua.DataValueServerTimestamp | ua.DataValueStatusCode,
			ServerTimestamp: time.Now(),
			Status:          ua.StatusBad,
		}
		switch a {
		case ua.AttributeIDNodeClass:
			dv.Status = ua.StatusOK
			dv.EncodingMask |= ua.DataValueValue
			dv.Value = ua.MustVariant(int32(ua.NodeClassDataType))
		case ua.AttributeIDBrowseName:
			dv.Status = ua.StatusOK
			dv.EncodingMask |= ua.DataValueValue
			dv.Value = ua.MustVariant(attrs.BrowseName(typeName))
		case ua.AttributeIDDisplayName:
			dv.Status = ua.StatusOK
			dv.EncodingMask |= ua.DataValueValue
			dv.Value = ua.MustVariant(attrs.DisplayName(typeName, ""))
		case ua.AttributeIDNodeID:
			dv.Status = ua.StatusOK
			dv.EncodingMask |= ua.DataValueValue
			dv.Value = ua.MustVariant(n)
		default:
			return dv
		}
		return dv
	}

	// 変数ノードまたは子ノード: varID + パスに分解して処理
	rootVarID, path := parseNodePath(varID)

	ns.mu.RLock()
	info, ok := ns.vars[rootVarID]
	ns.mu.RUnlock()

	if !ok {
		return errDV(ua.StatusBadNodeIDUnknown)
	}

	// 現在ノードのデータ型・表示名を決定（子ノードの場合はパスをたどる）
	currentType := info.dataType
	displayName := info.name
	if len(path) > 0 {
		currentType = ns.followTypeForPath(info.dataType, path)
		if currentType == "" {
			return errDV(ua.StatusBadNodeIDUnknown)
		}
		displayName = pathDisplayName(path)
	}
	_, currentArraySize, currentIsArray := parseArrayType(currentType)

	dv := &ua.DataValue{
		EncodingMask:    ua.DataValueServerTimestamp | ua.DataValueStatusCode,
		ServerTimestamp: time.Now(),
		Status:          ua.StatusBad,
	}

	switch a {
	case ua.AttributeIDNodeID:
		dv.Status = ua.StatusOK
		dv.EncodingMask |= ua.DataValueValue
		dv.Value = ua.MustVariant(n)

	case ua.AttributeIDValue:
		if info.accessMode == "write" && len(path) == 0 {
			return errDV(ua.StatusBadNotReadable)
		}
		if ns.accessor == nil {
			return errDV(ua.StatusBadInternalError)
		}
		val, err := ns.accessor.ReadVariableValue(rootVarID)
		if err != nil {
			return errDV(ua.StatusBadInternalError)
		}
		if len(path) > 0 {
			child, ok := ns.navigateValueExternal(val, path, info.dataType)
			if !ok {
				return errDV(ua.StatusBadInternalError)
			}
			val = child
		}
		opcVal := toOpcuaValue(val, currentType)
		dv.Status = ua.StatusOK
		dv.EncodingMask |= ua.DataValueValue
		dv.Value = ua.MustVariant(opcVal)

	case ua.AttributeIDDescription:
		dv.Status = ua.StatusOK
		dv.EncodingMask |= ua.DataValueValue
		dv.Value = ua.MustVariant(&ua.LocalizedText{EncodingMask: ua.LocalizedTextText, Text: ""})

	case ua.AttributeIDBrowseName:
		dv.Status = ua.StatusOK
		dv.EncodingMask |= ua.DataValueValue
		dv.Value = ua.MustVariant(attrs.BrowseName(displayName))

	case ua.AttributeIDDisplayName:
		dv.Status = ua.StatusOK
		dv.EncodingMask |= ua.DataValueValue
		dv.Value = ua.MustVariant(attrs.DisplayName(displayName, ""))

	case ua.AttributeIDAccessLevel:
		dv.Status = ua.StatusOK
		dv.EncodingMask |= ua.DataValueValue
		var level byte
		switch info.accessMode {
		case "read":
			level = byte(ua.AccessLevelTypeCurrentRead)
		case "write":
			level = byte(ua.AccessLevelTypeCurrentWrite)
		default: // "readwrite"
			level = byte(ua.AccessLevelTypeCurrentRead | ua.AccessLevelTypeCurrentWrite)
		}
		dv.Value = ua.MustVariant(level)

	case ua.AttributeIDNodeClass:
		dv.Status = ua.StatusOK
		dv.EncodingMask |= ua.DataValueValue
		dv.Value = ua.MustVariant(int32(ua.NodeClassVariable))

	case ua.AttributeIDEventNotifier:
		dv.Status = ua.StatusOK
		dv.EncodingMask |= ua.DataValueValue
		dv.Value = ua.MustVariant(int16(0))

	case ua.AttributeIDDataType:
		dv.Status = ua.StatusOK
		dv.EncodingMask |= ua.DataValueValue
		dv.Value = ua.MustVariant(ns.dataTypeNodeID(currentType))

	case ua.AttributeIDValueRank:
		dv.Status = ua.StatusOK
		dv.EncodingMask |= ua.DataValueValue
		if currentIsArray {
			dv.Value = ua.MustVariant(int32(1)) // OneDimensionalArray
		} else {
			dv.Value = ua.MustVariant(int32(-1)) // Scalar
		}

	case ua.AttributeIDArrayDimensions:
		dv.Status = ua.StatusOK
		dv.EncodingMask |= ua.DataValueValue
		if currentIsArray {
			dv.Value = ua.MustVariant([]uint32{uint32(currentArraySize)})
		} else {
			dv.Value = ua.MustVariant([]uint32{})
		}

	default:
		return dv
	}
	return dv
}

func (ns *PLCNameSpace) SetAttribute(node *ua.NodeID, attr ua.AttributeID, val *ua.DataValue) ua.StatusCode {
	if attr != ua.AttributeIDValue {
		return ua.StatusBadAttributeIDInvalid
	}

	nodeStr := node.StringID()
	rootVarID, path := parseNodePath(nodeStr)

	ns.mu.RLock()
	info, ok := ns.vars[rootVarID]
	ns.mu.RUnlock()

	if !ok {
		return ua.StatusBadNodeIDUnknown
	}
	if info.accessMode == "read" {
		return ua.StatusBadNotWritable
	}
	if ns.accessor == nil {
		return ua.StatusBadInternalError
	}

	goVal := fromOpcuaValue(val.Value.Value())

	if len(path) == 0 {
		// ルート変数への書き込み（既存動作）
		if isStructDataType(info.dataType) {
			if s, ok := goVal.(string); ok {
				var m map[string]interface{}
				if err := json.Unmarshal([]byte(s), &m); err == nil {
					goVal = m
				}
			}
		}
	} else {
		// 子ノードへの書き込み: ホスト側でアトミックに field を更新
		currentType := ns.followTypeForPath(info.dataType, path)
		if currentType == "" {
			return ua.StatusBadNodeIDUnknown
		}
		// 子ノードが構造体型の場合: JSON 文字列を map に変換
		if isStructDataType(currentType) {
			if s, ok := goVal.(string); ok {
				var m map[string]interface{}
				if err := json.Unmarshal([]byte(s), &m); err == nil {
					goVal = m
				}
			}
		}
		fieldPath := pathSegmentsToString(path)
		if err := ns.accessor.WriteVariableFieldValue(rootVarID, fieldPath, goVal); err != nil {
			return ua.StatusBadTypeMismatch
		}
		// 書き込み後にサブスクライバーへ即時通知（親ノードも通知）
		ns.srv.ChangeNotification(ua.NewStringNodeID(ns.nsID, rootVarID))
		ns.srv.ChangeNotification(node)
		return ua.StatusOK
	}

	if err := ns.accessor.WriteVariableValue(rootVarID, goVal); err != nil {
		return ua.StatusBadTypeMismatch
	}
	// 書き込み後にサブスクライバーへ即時通知（親ノードも通知）
	ns.srv.ChangeNotification(ua.NewStringNodeID(ns.nsID, rootVarID))
	ns.srv.ChangeNotification(node)
	return ua.StatusOK
}

// ===== 型変換ヘルパー =====

// anyToInt64 はどの数値型からも int64 に変換する（msgpack と JSON の両方に対応）
func anyToInt64(e interface{}) int64 {
	switch v := e.(type) {
	case int64:
		return v
	case uint64:
		return int64(v)
	case int32:
		return int64(v)
	case uint32:
		return int64(v)
	case int16:
		return int64(v)
	case uint16:
		return int64(v)
	case int8:
		return int64(v)
	case uint8:
		return int64(v)
	case int:
		return int64(v)
	case uint:
		return int64(v)
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	}
	return 0
}

// anyToUint64 はどの数値型からも uint64 に変換する（msgpack と JSON の両方に対応）
func anyToUint64(e interface{}) uint64 {
	switch v := e.(type) {
	case uint64:
		return v
	case int64:
		return uint64(v)
	case uint32:
		return uint64(v)
	case int32:
		return uint64(v)
	case uint16:
		return uint64(v)
	case int16:
		return uint64(v)
	case uint8:
		return uint64(v)
	case int8:
		return uint64(v)
	case uint:
		return uint64(v)
	case int:
		return uint64(v)
	case float64:
		return uint64(v)
	case float32:
		return uint64(v)
	}
	return 0
}

// anyToFloat64 はどの数値型からも float64 に変換する（msgpack と JSON の両方に対応）
func anyToFloat64(e interface{}) float64 {
	switch v := e.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int64:
		return float64(v)
	case uint64:
		return float64(v)
	case int32:
		return float64(v)
	case uint32:
		return float64(v)
	case int16:
		return float64(v)
	case uint16:
		return float64(v)
	case int8:
		return float64(v)
	case uint8:
		return float64(v)
	case int:
		return float64(v)
	case uint:
		return float64(v)
	}
	return 0
}

// toOpcuaValue は Go の PLC 値を OPC UA が扱える型に変換する
func toOpcuaValue(val interface{}, dataType string) interface{} {
	// 配列型の場合は型付きスライスに変換
	elemType, arraySize, isArray := parseArrayType(dataType)
	if isArray {
		return convertToOpcuaArray(val, elemType, int(arraySize))
	}

	// ULINT/LINT は dataType を優先して判定（msgpack デコード後の型多様性に対応）
	switch dataType {
	case "ULINT":
		return anyToUint64(val) // OPC UA UInt64 として返す
	case "LINT":
		return anyToInt64(val) // OPC UA Int64 として返す
	}

	switch v := val.(type) {
	case bool, int8, int16, int32, uint8, uint16, uint32, float32, float64, string:
		return v
	case int64:
		return v
	case uint64:
		return v
	default:
		// 構造体等は JSON 文字列として返す
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}

// convertToOpcuaArray は PLC 配列値を OPC UA の型付きスライスに変換する
func convertToOpcuaArray(val interface{}, elemType string, size int) interface{} {
	// []interface{} に正規化（msgpack デコード後は既に []interface{}）
	var elems []interface{}
	if s, ok := val.([]interface{}); ok {
		elems = s
	}
	// サイズが足りない場合はゼロ値で補填
	for len(elems) < size {
		elems = append(elems, nil)
	}

	toF64 := anyToFloat64
	toBool := func(e interface{}) bool {
		if e == nil {
			return false
		}
		if b, ok := e.(bool); ok {
			return b
		}
		return anyToFloat64(e) != 0
	}

	switch elemType {
	case "BOOL":
		arr := make([]bool, size)
		for i := 0; i < size; i++ {
			arr[i] = toBool(elems[i])
		}
		return arr
	case "SINT":
		arr := make([]int8, size)
		for i := 0; i < size; i++ {
			arr[i] = int8(toF64(elems[i]))
		}
		return arr
	case "INT":
		arr := make([]int16, size)
		for i := 0; i < size; i++ {
			arr[i] = int16(toF64(elems[i]))
		}
		return arr
	case "DINT":
		arr := make([]int32, size)
		for i := 0; i < size; i++ {
			arr[i] = int32(toF64(elems[i]))
		}
		return arr
	case "LINT":
		arr := make([]int64, size)
		for i := 0; i < size; i++ {
			arr[i] = anyToInt64(elems[i])
		}
		return arr
	case "USINT":
		arr := make([]uint8, size)
		for i := 0; i < size; i++ {
			arr[i] = uint8(toF64(elems[i]))
		}
		return arr
	case "UINT":
		arr := make([]uint16, size)
		for i := 0; i < size; i++ {
			arr[i] = uint16(toF64(elems[i]))
		}
		return arr
	case "UDINT":
		arr := make([]uint32, size)
		for i := 0; i < size; i++ {
			arr[i] = uint32(toF64(elems[i]))
		}
		return arr
	case "ULINT":
		arr := make([]uint64, size)
		for i := 0; i < size; i++ {
			arr[i] = anyToUint64(elems[i])
		}
		return arr
	case "REAL":
		arr := make([]float32, size)
		for i := 0; i < size; i++ {
			arr[i] = float32(toF64(elems[i]))
		}
		return arr
	case "LREAL":
		arr := make([]float64, size)
		for i := 0; i < size; i++ {
			arr[i] = toF64(elems[i])
		}
		return arr
	default:
		// 構造体等: 各要素を JSON 文字列にエンコードする
		arr := make([]string, size)
		for i := 0; i < size; i++ {
			if elems[i] != nil {
				b, err := json.Marshal(elems[i])
				if err != nil {
					arr[i] = fmt.Sprintf("%v", elems[i])
				} else {
					arr[i] = string(b)
				}
			}
		}
		return arr
	}
}

// ===== パス解析・ナビゲーション =====

// parseNodePath は "varID.field[0].sub" を varID とパスセグメント列に分解する
// UUID は "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" 形式で '.' '[' を含まないため安全に分離できる
func parseNodePath(s string) (varID string, path []pathSegment) {
	for i, c := range s {
		if c == '.' || c == '[' {
			varID = s[:i]
			path = parseSegments(s[i:])
			return
		}
	}
	varID = s
	return
}

// parseSegments は ".field" や "[0]" の連続をパースする
func parseSegments(s string) []pathSegment {
	var segs []pathSegment
	for len(s) > 0 {
		switch s[0] {
		case '.':
			s = s[1:]
			end := strings.IndexAny(s, ".[")
			var name string
			if end < 0 {
				name, s = s, ""
			} else {
				name, s = s[:end], s[end:]
			}
			if name != "" {
				segs = append(segs, pathSegment{kind: "field", field: name})
			}
		case '[':
			end := strings.Index(s, "]")
			if end < 0 {
				s = ""
				break
			}
			idx, _ := strconv.Atoi(s[1:end])
			segs = append(segs, pathSegment{kind: "index", index: idx})
			s = s[end+1:]
		default:
			s = ""
		}
	}
	return segs
}

// followTypeForPath はルート型からパスをたどって末端の型文字列を返す
func (ns *PLCNameSpace) followTypeForPath(rootType string, path []pathSegment) string {
	current := rootType
	for _, seg := range path {
		switch seg.kind {
		case "field":
			if !isStructDataType(current) || ns.accessor == nil {
				return ""
			}
			found := false
			for _, f := range ns.accessor.GetStructFields(current) {
				if f.Name == seg.field {
					current = f.DataType
					found = true
					break
				}
			}
			if !found {
				return ""
			}
		case "index":
			elemType, _, isArr := parseArrayType(current)
			if !isArr {
				return ""
			}
			current = elemType
		}
	}
	return current
}

// navigateValueExternal は外部インデックス（下限ベース）のパスで値ツリーをたどり末端値を返す。
// 配列インデックスを型情報から取得した下限で補正して内部スライスにアクセスする。
func (ns *PLCNameSpace) navigateValueExternal(value interface{}, path []pathSegment, dataType string) (interface{}, bool) {
	current := dataType
	for _, seg := range path {
		switch seg.kind {
		case "field":
			m, ok := value.(map[string]interface{})
			if !ok {
				return nil, false
			}
			v, ok := m[seg.field]
			if !ok {
				return nil, false
			}
			// 構造体フィールドの型を追跡
			if ns.accessor != nil {
				for _, f := range ns.accessor.GetStructFields(current) {
					if f.Name == seg.field {
						current = f.DataType
						break
					}
				}
			}
			value = v
		case "index":
			arr, ok := value.([]interface{})
			if !ok {
				return nil, false
			}
			_, lower, _, isArr := parseArrayTypeFull(current)
			internalIdx := seg.index
			if isArr {
				elemType, _, _, _ := parseArrayTypeFull(current)
				internalIdx = seg.index - int(lower)
				current = elemType
			}
			if internalIdx < 0 || internalIdx >= len(arr) {
				return nil, false
			}
			value = arr[internalIdx]
		}
	}
	return value, true
}

// pathSegmentsToString は pathSegment スライスを 0ベースのパス文字列に変換する
// 例: [{field:"motor"}, {index:0}, {field:"speed"}] → "motor[0].speed"
func pathSegmentsToString(path []pathSegment) string {
	var sb strings.Builder
	for _, seg := range path {
		if seg.kind == "field" {
			if sb.Len() > 0 {
				sb.WriteByte('.')
			}
			sb.WriteString(seg.field)
		} else {
			sb.WriteString(fmt.Sprintf("[%d]", seg.index))
		}
	}
	return sb.String()
}

// pathDisplayName はパスの末端セグメントの表示名を返す
func pathDisplayName(path []pathSegment) string {
	if len(path) == 0 {
		return ""
	}
	seg := path[len(path)-1]
	if seg.kind == "field" {
		return seg.field
	}
	return fmt.Sprintf("[%d]", seg.index)
}

// parseArrayTypeFull は配列型文字列を解析して要素型・下限・サイズ・配列フラグを返す
//
// IEC 61131-3 形式:
//   - "ARRAY[2..9] OF INT"       → ("INT", 2, 8, true)
//   - "ARRAY[0..2, 0..4] OF INT" → ("ARRAY[0..4] OF INT", 0, 3, true)
//
// 後方互換（旧形式）:
//   - "ARRAY[INT;10]"            → ("INT", 0, 10, true)
func parseArrayTypeFull(dataType string) (elemType string, lower, size int32, isArray bool) {
	if !strings.HasPrefix(dataType, "ARRAY[") {
		return "", 0, 0, false
	}
	// IEC 61131-3 形式: "] OF " を含む
	if ofIdx := strings.Index(dataType, "] OF "); ofIdx >= 0 {
		dimsStr := dataType[6:ofIdx]
		elemStr := dataType[ofIdx+5:]
		dimParts := strings.Split(dimsStr, ",")
		firstDim := strings.TrimSpace(dimParts[0])
		parts := strings.SplitN(firstDim, "..", 2)
		if len(parts) != 2 {
			return "", 0, 0, false
		}
		lo, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		up, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err1 != nil || err2 != nil {
			return "", 0, 0, false
		}
		n := up - lo + 1
		if n <= 0 {
			return "", 0, 0, false
		}
		if len(dimParts) > 1 {
			trimmed := make([]string, len(dimParts)-1)
			for i, d := range dimParts[1:] {
				trimmed[i] = strings.TrimSpace(d)
			}
			elemStr = fmt.Sprintf("ARRAY[%s] OF %s", strings.Join(trimmed, ", "), elemStr)
		}
		return elemStr, int32(lo), int32(n), true
	}
	// 旧形式（後方互換）: "ARRAY[ElementType;Size]"
	if !strings.HasSuffix(dataType, "]") {
		return "", 0, 0, false
	}
	inner := dataType[len("ARRAY[") : len(dataType)-1]
	idx := strings.LastIndex(inner, ";")
	if idx < 0 {
		return "", 0, 0, false
	}
	et := strings.TrimSpace(inner[:idx])
	sizeStr := strings.TrimSpace(inner[idx+1:])
	n, err := strconv.Atoi(sizeStr)
	if err != nil || n <= 0 {
		return "", 0, 0, false
	}
	return et, 0, int32(n), true
}

// parseArrayType は配列型文字列を解析して要素型・サイズ・配列フラグを返す（下限は無視）
func parseArrayType(dataType string) (elemType string, size int32, isArray bool) {
	et, _, n, ok := parseArrayTypeFull(dataType)
	return et, n, ok
}

// fromOpcuaValue は OPC UA から受け取った値を Go の値に変換する
func fromOpcuaValue(val interface{}) interface{} {
	// gopcua は配列を型付きスライス（[]int16 等）で渡すため、
	// VariableStore.UpdateValue が期待する []interface{} に変換する
	switch v := val.(type) {
	case []bool:
		arr := make([]interface{}, len(v))
		for i, e := range v {
			arr[i] = e
		}
		return arr
	case []int8:
		arr := make([]interface{}, len(v))
		for i, e := range v {
			arr[i] = e
		}
		return arr
	case []int16:
		arr := make([]interface{}, len(v))
		for i, e := range v {
			arr[i] = e
		}
		return arr
	case []int32:
		arr := make([]interface{}, len(v))
		for i, e := range v {
			arr[i] = e
		}
		return arr
	case []int64:
		arr := make([]interface{}, len(v))
		for i, e := range v {
			arr[i] = e
		}
		return arr
	case []uint64:
		arr := make([]interface{}, len(v))
		for i, e := range v {
			arr[i] = e
		}
		return arr
	case []uint8:
		arr := make([]interface{}, len(v))
		for i, e := range v {
			arr[i] = e
		}
		return arr
	case []uint16:
		arr := make([]interface{}, len(v))
		for i, e := range v {
			arr[i] = e
		}
		return arr
	case []uint32:
		arr := make([]interface{}, len(v))
		for i, e := range v {
			arr[i] = e
		}
		return arr
	case []float32:
		arr := make([]interface{}, len(v))
		for i, e := range v {
			arr[i] = e
		}
		return arr
	case []float64:
		arr := make([]interface{}, len(v))
		for i, e := range v {
			arr[i] = e
		}
		return arr
	case []string:
		arr := make([]interface{}, len(v))
		for i, e := range v {
			arr[i] = e
		}
		return arr
	}
	// スカラー型はそのまま返す
	return val
}

// isStructDataType は PLC データ型文字列が構造体型かどうかを判定する
func isStructDataType(dataType string) bool {
	if dataType == "" {
		return false
	}
	_, _, isArr := parseArrayType(dataType)
	if isArr {
		return false
	}
	switch dataType {
	case "BOOL", "SINT", "INT", "DINT", "LINT", "USINT", "UINT", "UDINT", "ULINT", "REAL", "LREAL",
		"STRING", "TIME", "DATE", "TIME_OF_DAY", "DATE_AND_TIME":
		return false
	}
	// STRING[n] 型
	if strings.HasPrefix(dataType, "STRING[") && strings.HasSuffix(dataType, "]") {
		return false
	}
	return true
}

// toExpandedNodeID は NodeID を ExpandedNodeID に変換する
func toExpandedNodeID(nodeID *ua.NodeID) *ua.ExpandedNodeID {
	if s := nodeID.StringID(); s != "" {
		return ua.NewStringExpandedNodeID(nodeID.Namespace(), s)
	}
	return ua.NewNumericExpandedNodeID(nodeID.Namespace(), nodeID.IntID())
}

// dataTypeNodeID は PLC データ型に対応する OPC UA データ型 NodeID を返す
// 構造体型は ns=X;s=_dt_<型名> のカスタム NodeID を返す
func (ns *PLCNameSpace) dataTypeNodeID(dataType string) *ua.NodeID {
	// 配列型の場合は要素型の NodeID を返す（OPC UA 仕様: DataType は要素型を指す）
	elemType, _, isArray := parseArrayType(dataType)
	if isArray {
		return ns.dataTypeNodeID(elemType)
	}

	// OPC UA 標準データ型 NodeID（ns=0）
	// https://reference.opcfoundation.org/v104/Core/docs/Part6/5.1.2/
	switch dataType {
	case "BOOL":
		return ua.NewNumericNodeID(0, 1) // Boolean
	case "SINT":
		return ua.NewNumericNodeID(0, 2) // SByte
	case "INT":
		return ua.NewNumericNodeID(0, 4) // Int16
	case "DINT":
		return ua.NewNumericNodeID(0, 6) // Int32
	case "LINT":
		return ua.NewNumericNodeID(0, 8) // Int64
	case "USINT":
		return ua.NewNumericNodeID(0, 3) // Byte
	case "UINT":
		return ua.NewNumericNodeID(0, 5) // UInt16
	case "UDINT":
		return ua.NewNumericNodeID(0, 7) // UInt32
	case "ULINT":
		return ua.NewNumericNodeID(0, 9) // UInt64
	case "REAL":
		return ua.NewNumericNodeID(0, 10) // Float
	case "LREAL":
		return ua.NewNumericNodeID(0, 11) // Double
	default:
		// 構造体型: カスタム DataType ノード（ns=X;s=_dt_<型名>）
		if isStructDataType(dataType) {
			return ua.NewStringNodeID(ns.nsID, "_dt_"+dataType)
		}
		return ua.NewNumericNodeID(0, 12) // String（STRING[n] 等）
	}
}

// errDV はエラーステータスを持つ DataValue を返す
func errDV(status ua.StatusCode) *ua.DataValue {
	return &ua.DataValue{
		EncodingMask:    ua.DataValueServerTimestamp | ua.DataValueStatusCode,
		ServerTimestamp: time.Now(),
		Status:          status,
	}
}
