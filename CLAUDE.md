# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Rules

- Always respond in Japanese (日本語で応答すること)

## Project Overview

このプロジェクトは PLC シミュレーター です。マルチプロトコル対応で、Modbus（TCP/RTU/RTU ASCII）およびOMRON FINSプロトコルをサポートしています。

### 主な機能

- **マルチプロトコル対応**: Modbus（TCP/RTU/ASCII）、OMRON FINS/UDP
- **変数管理**: IEC 61131-3準拠のデータ型（スカラー、配列、構造体、STRING[n]）をサポート
- **スクリプト機能**: JavaScriptで周期処理を記述。const/let対応（IIFE wrapping）、実行エラーのGUI表示
- **レジスタ操作**: GUIからメモリエリアの値を直接操作可能
- **モニタリング**: 任意のレジスタをリアルタイム監視・書き込み（ドラッグ&ドロップ並べ替え対応）
- **プロトコルマッピング**: 変数を複数プロトコルのメモリアドレスにマッピング可能

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
- **ScriptEngine** (`internal/infrastructure/scripting/engine.go`): gojaベースのJavaScript実行エンジン
  - `plc`オブジェクトでDataStoreおよびVariableStoreにアクセス可能
  - スクリプトコードをIIFE `(function(){...})();` でラップして、const/let再宣言エラーを回避
  - 実行時エラーを保存して`GetLastError()`で取得可能
  - 周期実行中のpanicをキャッチしてエラーとして記録
  - TIME/DATE型シンタックスシュガー: `plc.readTimeMs(name)`, `plc.writeTimeMs(name, ms)` など、変数の読み取り〜数値変換〜書き込みをワンステップで実行（内部でparse/formatを自動適用）

### フロントエンド構成（スキーマ駆動UI）

```
frontend/src/
├── components/
│   ├── ServerPanel.tsx     # スキーマ駆動のサーバー設定UI
│   ├── RegisterPanel.tsx   # 汎用メモリ操作UI（サブタブで一覧/モニタリング切替）
│   ├── MonitoringView.tsx  # カスタムレジスタモニタリング
│   ├── VariableView.tsx    # 変数管理UI（IEC 61131-3データ型対応）
│   └── ScriptPanel.tsx     # スクリプト管理（エラー表示機能付き）
└── App.tsx                 # タブベースのメインUI
```

#### ServerPanel.tsx
`GetProtocolSchema()`から取得したスキーマに基づき、`DynamicField`コンポーネントで動的にフォームを生成します。

#### RegisterPanel.tsx
「一覧表示」と「モニタリング」のサブタブを持ちます。
- **一覧表示**: メモリエリアごとのレジスタ値を表示・編集
- **モニタリング**: 任意のレジスタを登録してリアルタイム監視・書き込み可能
  - ドラッグ＆ドロップで並び替え可能（@dnd-kit使用）
  - プロトコル変更時は確認ダイアログ後にリストクリア

#### VariableView.tsx
IEC 61131-3準拠の変数管理機能。
- **データ型サポート**:
  - スカラー型: BOOL, SINT, INT, DINT, USINT, UINT, UDINT, REAL, LREAL, STRING[n]
  - 配列型: ARRAY[型;サイズ]（例: ARRAY[INT;10]）
  - 構造体型: カスタム構造体定義（ネスト可能）
- **変数表示**: 構造体フィールドと配列要素をフラット化して行単位で表示
  - 展開/折りたたみ機能（構造体・配列のヘッダー行）
  - アドレスオフセット計算（フィールド・要素ごと）
- **値編集ダイアログ**:
  - 再帰的エディタで複雑なデータ構造に対応
  - 構造体配列要素は折りたたみ可能（`StructArrayElementEditor`）
  - 数値入力は10進、16進（0x）、2進（0b）対応
- **プロトコルマッピング**: 変数を複数プロトコルのメモリアドレスにマッピング可能
  - 2行表示レイアウト（ヘッダ行 + コントロール行）
- **構造体型管理**: 構造体型の登録・編集・削除機能
  - 編集時は既存の定義を読み込んでフォームに展開
  - データ型の自動解析（ARRAY、STRING、構造体、スカラー）
  - スクロール対応（多数のフィールドを持つ構造体に対応）

#### ScriptPanel.tsx
JavaScript（goja）でPLC動作を記述。
- **エラー表示**: 実行時エラーをGUIに表示（タイムスタンプ付き、クリアボタン）
- **const/let対応**: スクリプトコードをIIFEでラップして再宣言エラーを回避
- **plcオブジェクト**:
  - メモリアクセス: `plc.readBit()`, `plc.writeBit()`, `plc.readWord()`, `plc.writeWord()`
  - 変数アクセス: `plc.readVariable()`, `plc.writeVariable()`, `plc.readArrayElement()`, `plc.writeArrayElement()`, `plc.readStructField()`, `plc.writeStructField()`
  - TIME/DATE シンタックスシュガー: `plc.readTimeMs()`, `plc.writeTimeMs()`, `plc.readDateSec()`, `plc.writeDateSec()`, `plc.readTimeOfDayMs()`, `plc.writeTimeOfDayMs()`, `plc.readDateAndTimeSec()`, `plc.writeDateAndTimeSec()`（変数の読み取り・パース・フォーマット・書き込みをワンステップで実行）
  - TIME/DATE 文字列変換: `plc.parseTime()`, `plc.formatTime()`, `plc.parseDate()`, `plc.formatDate()`, `plc.parseTimeOfDay()`, `plc.formatTimeOfDay()`, `plc.parseDateAndTime()`, `plc.formatDateAndTime()`

### Wailsバインディング

`app.go`でフロントエンドに公開するメソッドを定義。`wails generate module`でTypeScript型定義を自動生成。

主要API:
- **プロトコル設定**:
  - `GetProtocolSchema(protocolType)`: プロトコルのスキーマ（バリアント、フィールド定義）を取得
  - `GetCurrentConfig()`: 現在の設定を取得
  - `UpdateConfig(dto)`: 設定を更新
- **メモリ操作**:
  - `ReadBits()`, `WriteBit()`, `ReadWords()`, `WriteWord()`: 汎用メモリ操作
- **モニタリング**:
  - `GetMonitoringItems()`, `AddMonitoringItem()`, `UpdateMonitoringItem()`, `DeleteMonitoringItem()`, `ReorderMonitoringItem()`, `ClearMonitoringItems()`
- **変数管理**:
  - `GetVariables()`, `CreateVariable()`, `UpdateVariableValue()`, `DeleteVariable()`: 変数CRUD操作
  - `GetDataTypes()`: サポートされているデータ型一覧を取得
  - `GetStructTypes()`, `RegisterStructType()`, `DeleteStructType()`: 構造体型管理
  - `UpdateVariableMappings()`: 変数のプロトコルマッピング設定
- **スクリプト**:
  - `GetScripts()`, `GetScript()`, `CreateScript()`, `UpdateScript()`, `DeleteScript()`: スクリプトCRUD操作
  - `StartScript()`, `StopScript()`: スクリプト実行制御
  - `ClearScriptError()`: スクリプトエラーをクリア

### 変数管理とデータ型システム

#### IEC 61131-3準拠のデータ型

変数は以下のデータ型をサポート：

1. **スカラー型**:
   - 整数: SINT(8bit), INT(16bit), DINT(32bit), USINT, UINT, UDINT
   - 浮動小数点: REAL(32bit), LREAL(64bit)
   - 論理: BOOL
   - 文字列: STRING[n] (固定長、nはバイト数)
   - 時間・日付: TIME, DATE, TIME_OF_DAY, DATE_AND_TIME
     - TIME: 時間間隔（"T#1h30m45s" など、内部はミリ秒int32で2ワード保存）
     - DATE: 日付（"D#2024-01-01" など、内部はその日の0時0分0秒のUnix秒uint64で4ワード保存）
     - TIME_OF_DAY: 1日の中の時刻（"TOD#12:30:15" など、内部はミリ秒uint32で2ワード保存）
     - DATE_AND_TIME: 日付と時刻（"DT#2024-01-01-12:30:15" など、内部はUnix秒uint64で4ワード保存）

2. **配列型**: `ARRAY[要素型;要素数]`
   - 例: `ARRAY[INT;10]`, `ARRAY[MyStruct;5]`
   - 多次元配列は `ARRAY[ARRAY[INT;5];3]` のように表現

3. **構造体型**: カスタム定義可能
   - フィールドはスカラー、配列、構造体のいずれか
   - 再帰的なネストに対応
   - ワードオフセットは自動計算

#### 変数の内部構造

- **フラット化表示**: 構造体フィールドと配列要素を行単位で展開
- **アドレスオフセット**: 各フィールド・要素のワードオフセットを計算して表示
- **展開/折りたたみ**: 構造体・配列のヘッダー行で子要素の表示を制御
- **再帰的編集**: ネストされたデータ構造を再帰的に編集可能

#### プロトコルマッピング

変数を複数プロトコルのメモリアドレスにマッピング可能：
- 各マッピングは `protocolType`, `memoryArea`, `address`, `endianness` を指定
- フィールド・要素のアドレスは自動計算（ベースアドレス + オフセット）
- エンディアンは変数ごとに設定可能（big/little）

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

## 重要な実装ポイント

### Reactコンポーネントの状態管理

- **コンポーネントの定義位置**: 関数コンポーネント内で別のコンポーネントを定義すると、再レンダリング時に新しいコンポーネントとして扱われ、`useState`などの状態がリセットされる
  - 例: `StructArrayElementEditor`は`VariableView`の外で定義
  - 内部で使用する関数は props で渡す
- **キーの設定**: リスト描画時は安定した`key`を設定（インデックスだけでなく、パスベースのキーを使用）

### スクリプトエンジン

- **const/let対応**: goja VMで同じプログラムを周期的に実行すると再宣言エラーが発生するため、IIFEでラップ
- **エラーハンドリング**: runtime panic をキャッチして`lastError`フィールドに保存し、GUI表示可能に

### ダイアログスタイルの統一

すべてのダイアログ（変数編集、マッピング設定、構造体型管理、モニタリング追加、レジスタ書き込み）は統一されたスタイルパターンを使用：
- **`.dialog-row`**: ラベルとコントロールを横並びに配置
- **インラインスタイル削除**: すべての `<select>` と `<input>` からインラインスタイルを削除
- **スクロール対応**: ダイアログ全体に `maxHeight: '80vh'` を設定し、コンテンツ領域で `overflowY: 'auto'`
- **flexboxレイアウト**: ダイアログ全体を `display: flex, flexDirection: 'column'` でレイアウト

### 値編集ダイアログ

- **スクロール**: ダイアログ全体で1つのスクロール領域を使用（配列要素部分に独自スクロールを設定しない）
- **再帰的エディタ**: `renderValueEditor`は`depth`パラメータで再帰の深さを追跡し、インデントを調整

### 構造体型管理ダイアログ

- **編集機能**: 既存の構造体型を編集可能（編集ボタンで定義を読み込み、更新で保存）
- **データ型解析**: フィールドのデータ型文字列を解析してフォーム形式に変換
  - `ARRAY[型;サイズ]` → 配列カテゴリ + 要素型 + サイズ
  - `STRING[n]` → スカラーカテゴリ + STRING型 + バイト長
  - 構造体型名 → 構造体カテゴリ + 型名
- **型名の不変性**: 編集モード時は型名の入力欄を無効化（型名は変更不可）

## License

MIT License (Copyright 2026 bamchoh)
