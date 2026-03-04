package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "modbus_simulator/pb/pluginpb"
)

// ManifestCapabilities は plugin.json のプロトコル機能情報を表す
type ManifestCapabilities struct {
	SupportsUnitID         bool `json:"supports_unit_id"`
	UnitIDMin              int  `json:"unit_id_min"`
	UnitIDMax              int  `json:"unit_id_max"`
	SupportsNodePublishing bool `json:"supports_node_publishing"`
}

// ManifestVariant は plugin.json のバリアント情報を表す
type ManifestVariant struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// PluginManifest は plugin.json のスキーマを表す
type PluginManifest struct {
	Name         string               `json:"name"`
	Entrypoint   string               `json:"entrypoint"`
	Version      string               `json:"version"`
	Author       string               `json:"author,omitempty"`
	Description  string               `json:"description,omitempty"`
	ProtocolType string               `json:"protocol_type"`
	DisplayName  string               `json:"display_name"`
	Variants     []ManifestVariant    `json:"variants"`
	Capabilities ManifestCapabilities `json:"capabilities"`
}

// PluginManifestEntry はマニフェストとそのディレクトリを保持する
type PluginManifestEntry struct {
	Manifest *PluginManifest
	Dir      string // plugin.json があるディレクトリ（entrypoint の解決に使用）
}

// PluginProcess は起動中のプラグインプロセスを表す
type PluginProcess struct {
	path         string
	cmd          *exec.Cmd
	conn         *grpc.ClientConn
	PluginClient pb.PluginServiceClient
	DSClient     pb.DataStoreServiceClient
	Metadata     *pb.PluginMetadata
	Port         int

	mu      sync.RWMutex
	crashed bool
	exitErr error
}

// IsCrashed はプラグインプロセスがクラッシュしているかどうかを返す
func (p *PluginProcess) IsCrashed() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.crashed
}

// ExitError はプラグインプロセスの終了エラーを返す
func (p *PluginProcess) ExitError() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.exitErr
}

// Conn は gRPC クライアント接続を返す
func (p *PluginProcess) Conn() *grpc.ClientConn {
	return p.conn
}

// PluginProcessManager はプラグインプロセスのライフサイクルを管理する
type PluginProcessManager struct {
	mu        sync.RWMutex
	plugins   []*PluginProcess
	hostAddr  string  // HostGrpcServer のアドレス（プラグイン起動時に渡す）
	jobHandle uintptr // Windows Job Object ハンドル（ホスト終了時に子プロセスを自動終了）
}

// NewPluginProcessManager は PluginProcessManager を作成する
func NewPluginProcessManager(hostGrpcAddr string) *PluginProcessManager {
	return &PluginProcessManager{
		hostAddr:  hostGrpcAddr,
		jobHandle: initProcessJobObject(),
	}
}

// DiscoverManifests はプラグインディレクトリを走査してマニフェストのみを読み込む。
// プロセスは起動しない。protocol_type が空のエントリはスキップする（旧 plugin.json 対策）。
func (m *PluginProcessManager) DiscoverManifests(pluginsDir string) ([]*PluginManifestEntry, error) {
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil, fmt.Errorf("プラグインディレクトリの読み取り失敗: %w", err)
	}

	var result []*PluginManifestEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pluginDir := filepath.Join(pluginsDir, entry.Name())
		manifest, err := loadManifest(pluginDir)
		if err != nil {
			continue
		}
		if manifest.ProtocolType == "" {
			fmt.Fprintf(os.Stderr, "[WARN] plugin.json に protocol_type がありません（スキップ）: %s\n", pluginDir)
			continue
		}
		result = append(result, &PluginManifestEntry{Manifest: manifest, Dir: pluginDir})
	}
	return result, nil
}

// Discover はプラグインディレクトリ以下のサブフォルダを走査し、
// plugin.json マニフェストを持つプラグインを起動してファクトリー情報を取得する。
// 返された []*PluginProcess はそれぞれ起動済みのプラグインプロセスを表す。
func (m *PluginProcessManager) Discover(pluginsDir string) ([]*PluginProcess, error) {
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil, fmt.Errorf("プラグインディレクトリの読み取り失敗: %w", err)
	}

	var result []*PluginProcess
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pluginDir := filepath.Join(pluginsDir, entry.Name())
		manifest, err := loadManifest(pluginDir)
		if err != nil {
			// plugin.json がないフォルダはスキップ
			continue
		}
		entrypoint := filepath.Join(pluginDir, manifest.Entrypoint)
		proc, err := m.Launch(entrypoint)
		if err != nil {
			// 起動失敗は警告ログのみ（他のプラグインは継続）
			fmt.Fprintf(os.Stderr, "[WARN] プラグイン起動失敗 %s (%s): %v\n", manifest.Name, entrypoint, err)
			continue
		}
		result = append(result, proc)

		m.mu.Lock()
		m.plugins = append(m.plugins, proc)
		m.mu.Unlock()
	}
	return result, nil
}

// loadManifest はプラグインディレクトリの plugin.json を読み込む
func loadManifest(pluginDir string) (*PluginManifest, error) {
	manifestPath := filepath.Join(pluginDir, "plugin.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}
	var manifest PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("plugin.json のパース失敗 (%s): %w", manifestPath, err)
	}
	if manifest.Entrypoint == "" {
		return nil, fmt.Errorf("plugin.json に entrypoint が指定されていない (%s)", manifestPath)
	}
	return &manifest, nil
}

// Launch はプラグイン実行ファイルを起動し、gRPC 接続を確立して PluginProcess を返す
func (m *PluginProcessManager) Launch(pluginPath string) (*PluginProcess, error) {
	proc := &PluginProcess{path: pluginPath}

	cmd := exec.Command(pluginPath, "--host-grpc-addr="+m.hostAddr)
	setSysProcAttr(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout パイプ作成失敗: %w", err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("プラグイン起動失敗: %w", err)
	}
	proc.cmd = cmd

	// ホストプロセス終了時に子プロセスも自動終了するよう Job Object に割り当てる
	addProcessToJobObject(m.jobHandle, cmd.Process.Pid)

	// stdout から "GRPC_PORT=N" を読み取る（タイムアウト付き）
	port, err := readGrpcPort(stdout, 10*time.Second)
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("gRPC ポート取得失敗: %w", err)
	}
	proc.Port = port

	// gRPC 接続を確立
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("gRPC 接続失敗 %s: %w", addr, err)
	}
	proc.conn = conn
	proc.PluginClient = pb.NewPluginServiceClient(conn)
	proc.DSClient = pb.NewDataStoreServiceClient(conn)

	// メタデータを取得
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	meta, err := proc.PluginClient.GetMetadata(ctx, &pb.Empty{})
	if err != nil {
		_ = conn.Close()
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("メタデータ取得失敗: %w", err)
	}
	proc.Metadata = meta

	// クラッシュ監視 goroutine
	go func() {
		exitErr := cmd.Wait()
		proc.mu.Lock()
		proc.crashed = true
		proc.exitErr = exitErr
		proc.mu.Unlock()
		fmt.Fprintf(os.Stderr, "[ERROR] プラグインプロセスが終了: %s (error=%v)\n", pluginPath, exitErr)
	}()

	return proc, nil
}

// Shutdown は全プラグインプロセスを停止する
func (m *PluginProcessManager) Shutdown() {
	m.mu.RLock()
	plugins := make([]*PluginProcess, len(m.plugins))
	copy(plugins, m.plugins)
	m.mu.RUnlock()

	for _, p := range plugins {
		shutdownPlugin(p)
	}
}

// RemovePlugin は管理リストからプラグインを削除する
func (m *PluginProcessManager) RemovePlugin(p *PluginProcess) {
	shutdownPlugin(p)
	m.mu.Lock()
	for i, proc := range m.plugins {
		if proc == p {
			m.plugins = append(m.plugins[:i], m.plugins[i+1:]...)
			break
		}
	}
	m.mu.Unlock()
}

func shutdownPlugin(p *PluginProcess) {
	if p.conn != nil {
		_ = p.conn.Close()
	}
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
}

// readGrpcPort は stdout から "GRPC_PORT=N" 行を読み取ってポート番号を返す
func readGrpcPort(stdout io.Reader, timeout time.Duration) (int, error) {
	portCh := make(chan int, 1)
	errCh := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "GRPC_PORT=") {
				portStr := strings.TrimPrefix(line, "GRPC_PORT=")
				port, err := strconv.Atoi(strings.TrimSpace(portStr))
				if err != nil {
					errCh <- fmt.Errorf("ポート番号のパース失敗: %w", err)
					return
				}
				portCh <- port
				// 残りの stdout は非同期で読み続ける（プラグインのログ等）
				go func() {
					for scanner.Scan() {
						// プラグインの stdout はここで消費（ブロック防止）
					}
				}()
				return
			}
		}
		if err := scanner.Err(); err != nil {
			errCh <- fmt.Errorf("stdout 読み取りエラー: %w", err)
		} else {
			errCh <- fmt.Errorf("stdout が閉じられたが GRPC_PORT が見つからない")
		}
	}()

	select {
	case port := <-portCh:
		return port, nil
	case err := <-errCh:
		return 0, err
	case <-time.After(timeout):
		return 0, fmt.Errorf("gRPC ポート待機タイムアウト (%v)", timeout)
	}
}
