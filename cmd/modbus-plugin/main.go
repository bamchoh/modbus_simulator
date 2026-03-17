package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	"modbus_simulator/cmd/modbus-plugin/server"
)

func main() {
	protocolType := flag.String("protocol-type", "modbus-tcp", "プロトコルタイプ (modbus-tcp, modbus-rtu, modbus-ascii)")
	_ = flag.String("host-grpc-addr", "", "ホスト側 gRPC サーバーアドレス（Modbus プラグインでは未使用）")
	flag.Parse()

	fmt.Fprintln(os.Stderr, "Modbus Plugin starting... protocol-type="+*protocolType)
	// ランダムな空きポートで gRPC サーバーを起動
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] gRPC リスナー起動失敗: %v\n", err)
		os.Exit(1)
	}
	port := lis.Addr().(*net.TCPAddr).Port

	// gRPC サーバーを作成してサービスを登録
	grpcServer := grpc.NewServer()
	pluginSrv := server.NewPluginServer(*protocolType)
	pluginSrv.Register(grpcServer)

	// サーバーをバックグラウンドで起動
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] gRPC サーバーエラー: %v\n", err)
		}
	}()

	// ホストが読み取るポート番号を stdout に出力
	fmt.Printf("GRPC_PORT=%d\n", port)

	// SIGTERM/SIGINT でグレースフルシャットダウン
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	grpcServer.GracefulStop()
}
