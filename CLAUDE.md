# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Rules

- Always respond in Japanese (日本語で応答すること)

## Project Overview

このプロジェクトは PLC シミュレーター です。通信部分は Modbus TCP サーバー、Modbus RTU サーバーを切り替えることができます。スクリプトを記述することで周期処理を記述することができます。スクリプトは Javascript で記述します。GUIも持っており、各種レジスタの情報が一覧表示できます。もちろん GUI からレジスタの値を操作することも可能です。

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

### ディレクトリ構造（DDD）

```
internal/
├── domain/           # ドメイン層
│   ├── register/     # レジスタエンティティ（Coil, DiscreteInput, Holding, Input）
│   ├── script/       # スクリプトエンティティ
│   └── server/       # サーバー設定
├── application/      # アプリケーション層（PLCService）
└── infrastructure/   # インフラ層
    ├── modbus/       # Modbusサーバー実装（simonvetter/modbus使用）
    └── scripting/    # JSエンジン（goja使用）
```

### 主要コンポーネント

- **PLCService** (`internal/application/plc_service.go`): メインのサービス。サーバー管理、レジスタ操作、スクリプト管理を統括
- **RegisterStore** (`internal/domain/register/register.go`): スレッドセーフなレジスタストア（各65536個）
- **ScriptEngine** (`internal/infrastructure/scripting/engine.go`): gojaベースのJavaScript実行エンジン。`plc`オブジェクトでレジスタにアクセス可能
- **ModbusServer** (`internal/infrastructure/modbus/server.go`): TCP/RTU/RTU ASCIIに対応したModbusサーバー

### フロントエンド構成

```
frontend/src/
├── components/
│   ├── ServerPanel.tsx   # サーバー設定・制御
│   ├── RegisterPanel.tsx # レジスタ表示・編集
│   └── ScriptPanel.tsx   # スクリプト管理
└── App.tsx               # タブベースのメインUI
```

### Wailsバインディング

`app.go`でフロントエンドに公開するメソッドを定義。`wails generate module`でTypeScript型定義を自動生成。

## License

MIT License (Copyright 2026 bamchoh)
