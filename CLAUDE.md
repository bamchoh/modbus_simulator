# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Rules

- Always respond in Japanese (日本語で応答すること)

## Project Overview

このプロジェクトは PLC シミュレーター です。マルチプロトコル対応で、Modbus（TCP/RTU/RTU ASCII）およびOMRON FINSプロトコルをサポートしています。スクリプトを記述することで周期処理を記述することができます。スクリプトは Javascript で記述します。GUIも持っており、各種レジスタの情報が一覧表示できます。もちろん GUI からレジスタの値を操作することも可能です。モニタリング機能により、任意のレジスタを登録してリアルタイムで監視・操作できます。

## Build and Development Commands

```bash
# 開発モード（ホットリロード有効）
wails dev

# プロダクションビルド
wails build

# Goバインディングの再生成
wails generate module

# フロントエンドのみビルド
cd frontend && npm run build

# Goコードのビルド確認
go build ./...
```

## Architecture

Wails v2をベースに、フロントエンドはVite + React + TypeScript、バックエンドはGoで構成。

### ディレクトリ構造（DDD + マルチプロトコル）

```
internal/
├── domain/           # ドメイン層
│   ├── protocol/     # プロトコル共通インターフェース（ServerFactory, DataStore, ConfigField等）
│   ├── datastore/    # DataStore共通定義
│   ├── script/       # スクリプトエンティティ
│   └── register/     # レジスタエンティティ（レガシー）
├── application/      # アプリケーション層
│   ├── plc_service.go  # メインサービス（プロトコル非依存、モニタリング管理含む）
│   └── dto.go          # DTO定義（ProtocolSchemaDTO, MonitoringItemDTO等）
└── infrastructure/   # インフラ層（プロトコル実装）
    ├── modbus/       # Modbusサーバー実装
    │   ├── factory.go      # ModbusServerFactory
    │   ├── server.go       # ModbusServer
    │   └── datastore.go    # ModbusDataStore
    ├── fins/         # FINSサーバー実装
    │   ├── factory.go      # FINSServerFactory
    │   ├── server.go       # FINSServer
    │   └── datastore.go    # FINSDataStore
    └── scripting/    # JSエンジン（goja使用）
```

### 主要コンポーネント

- **PLCService** (`internal/application/plc_service.go`): メインサービス。プロトコル非依存で、アクティブな1つのプロトコルのみを管理
- **ServerFactory** (`internal/domain/protocol/server.go`): プロトコルサーバーを作成するファクトリーインターフェース
  - `GetConfigFields()`: スキーマ駆動UIのためのフィールド定義を返す
  - `GetProtocolCapabilities()`: UnitIDサポート等の機能情報を返す
  - `ConfigToMap()` / `MapToConfig()`: 設定の変換
- **DataStore** (`internal/domain/protocol/server.go`): プロトコル共通のメモリ操作インターフェース
  - `ReadBits()`, `WriteBit()`, `ReadWords()`, `WriteWord()`: 汎用メモリ操作
  - `Snapshot()`, `Restore()`: Export/Import用
- **ScriptEngine** (`internal/infrastructure/scripting/engine.go`): gojaベースのJavaScript実行エンジン。`plc`オブジェクトでDataStoreにアクセス可能

### フロントエンド構成（スキーマ駆動UI）

```
frontend/src/
├── components/
│   ├── ServerPanel.tsx     # スキーマ駆動のサーバー設定UI
│   ├── RegisterPanel.tsx   # 汎用メモリ操作UI（サブタブで一覧/モニタリング切替）
│   ├── MonitoringView.tsx  # カスタムレジスタモニタリング
│   └── ScriptPanel.tsx     # スクリプト管理
└── App.tsx                 # タブベースのメインUI
```

ServerPanel.tsxは`GetProtocolSchema()`から取得したスキーマに基づき、`DynamicField`コンポーネントで動的にフォームを生成します。

RegisterPanel.tsxは「一覧表示」と「モニタリング」のサブタブを持ち、モニタリングでは任意のレジスタを登録してリアルタイム監視・書き込みが可能です。モニタリング項目はドラッグ＆ドロップで並び替え可能（@dnd-kit使用）。プロトコル変更時はメモリエリアが異なるため、確認ダイアログ後にモニタリングリストがクリアされます。

### Wailsバインディング

`app.go`でフロントエンドに公開するメソッドを定義。`wails generate module`でTypeScript型定義を自動生成。

主要API:
- `GetProtocolSchema(protocolType)`: プロトコルのスキーマ（バリアント、フィールド定義）を取得
- `GetCurrentConfig()`: 現在の設定を取得
- `UpdateConfig(dto)`: 設定を更新
- `ReadBits()`, `WriteBit()`, `ReadWords()`, `WriteWord()`: 汎用メモリ操作
- `GetMonitoringItems()`, `AddMonitoringItem()`, `UpdateMonitoringItem()`, `DeleteMonitoringItem()`, `ReorderMonitoringItem()`, `ClearMonitoringItems()`: モニタリング項目管理

### 設定ファイル

アプリケーションの設定は以下の場所に保存されます：
- **モニタリング設定**: `%APPDATA%\PLCSimulator\monitoring_config.json`
  - 登録したモニタリング項目（メモリエリア、アドレス、ビット幅、エンディアン、表示形式）

## 新プロトコル追加手順

1. `internal/infrastructure/newprotocol/` ディレクトリを作成
2. 以下のファイルを実装:
   - `factory.go`: `ServerFactory`インターフェースを実装
   - `server.go`: `ProtocolServer`インターフェースを実装
   - `datastore.go`: `DataStore`インターフェースを実装
3. `factory.go`の`init()`で`protocol.Register()`を呼び出してレジストリに登録
4. **フロントエンド変更不要** - スキーマから自動生成

## License

MIT License (Copyright 2026 bamchoh)
