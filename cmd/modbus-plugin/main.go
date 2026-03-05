package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	"modbus_simulator/cmd/modbus-plugin/server"
)

func main() {
	fmt.Fprintln(os.Stderr, "Modbus Plugin starting...")
	// ランダムな空きポートで gRPC サーバーを起動
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] gRPC リスナー起動失敗: %v\n", err)
		os.Exit(1)
	}
	port := lis.Addr().(*net.TCPAddr).Port

	// gRPC サーバーを作成してサービスを登録
	grpcServer := grpc.NewServer()
	pluginSrv := server.NewPluginServer()
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
