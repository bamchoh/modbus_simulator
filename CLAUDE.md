# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Rules

- Always respond in Japanese (日本語で応答すること)

## Project Overview

このプロジェクトは PLC シミュレーター です。Modbus（TCP/RTU/ASCII）をサポートしています。

### 主な機能

- **マルチプロトコル対応**: Modbus TCP / Modbus RTU / Modbus ASCII を個別のサーバーとして起動可能
- **複数サーバー同時実行**: 異なるプロトコル（例: Modbus TCP + Modbus RTU）を同時に起動可能。各サーバーは独立したメモリ空間を持つ
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
    │   ├── factory.go      # ModbusServerFactory（TCP/RTU/ASCIIの3ファクトリー）
    │   ├── server.go       # ModbusServer
    │   └── datastore.go    # ModbusDataStore
    └── scripting/    # JSエンジン（goja使用）
```

### 主要コンポーネント

- **PLCService** (`internal/application/plc_service.go`): メインサービス。プロトコル非依存で、複数のサーバーインスタンスを同時管理
  - `servers map[protocol.ProtocolType]*serverInstance` で各プロトコルのサーバーを保持
  - 各プロトコルタイプは最大1インスタンス（プロトコルタイプをサーバー識別子として利用）
  - Modbus の各バリアントは独立した ProtocolType: `"modbus-tcp"`, `"modbus-rtu"`, `"modbus-ascii"`
  - `variableStore` と `scriptEngine` は全サーバーで共有
- **ServerFactory** (`internal/domain/protocol/server.go`): プロトコルサーバーを作成するファクトリーインターフェース
  - `GetConfigFields()`: スキーマ駆動UIのためのフィールド定義を返す
  - `GetProtocolCapabilities()`: UnitIDサポート等の機能情報を返す
  - `ConfigToMap()` / `MapToConfig()`: 設定の変換
- **ModbusServerFactory** (`internal/infrastructure/modbus/factory.go`): `fixedVariant` フィールドで TCP/RTU/ASCII を固定した3種のファクトリー
  - `NewModbusTCPServerFactory()`, `NewModbusRTUServerFactory()`, `NewModbusASCIIServerFactory()` で生成
  - それぞれ `ProtocolType()` が `"modbus-tcp"` / `"modbus-rtu"` / `"modbus-ascii"` を返す
  - `init()` で3つ全てを `protocol.Register()` に登録
- **DataStore** (`internal/domain/protocol/server.go`): プロトコル共通のメモリ操作インターフェース
  - `ReadBits()`, `WriteBit()`, `ReadWords()`, `WriteWord()`: 汎用メモリ操作
  - `Snapshot()`, `Restore()`: Export/Import用
  - `GetAreas()`: `MemoryArea` スライスを返す。`MemoryArea.OneOrigin` が true のエリアはUIで1オリジンアドレスを表示する（内部は常に0ベース）。Modbusの全4エリアは `OneOrigin: true`
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
サーバーインスタンスの一覧を表示・管理します。
- **サーバー一覧**: `GetServerInstances()` で1秒ポーリングして全サーバーのステータスを更新
- **サーバー追加**: 「サーバーを追加」ダイアログで未追加のプロトコルから選択して追加（`AddServer(protocolType, variant)`）
- **個別操作**: 各サーバーの開始/停止/設定変更/削除が独立して可能
- **設定パネル**: `GetProtocolSchema(protocolType)` から取得したスキーマに基づき `DynamicField` で動的フォームを生成
- `ServerInstanceRow` と `DynamicField` は `ServerPanel` 関数の外部に定義（state リセット防止）

#### RegisterPanel.tsx
「一覧表示」と「モニタリング」のサブタブを持ちます。
- **プロトコル選択**: 複数サーバー起動時は上部にプロトコル選択セレクトを表示
- **一覧表示**: 選択中プロトコルのメモリエリアごとのレジスタ値を表示・編集
- **モニタリング**: 任意のレジスタを登録してリアルタイム監視・書き込み可能
  - ドラッグ＆ドロップで並び替え可能（@dnd-kit使用）
- **1オリジンアドレス表示**: `area.oneOrigin` が true のエリアはアドレス表示を+1（内部値は0ベースを維持）

#### MonitoringView.tsx
プロトコルを横断したレジスタモニタリング。
- **Props**: `serverInstances: ServerInstanceDTO[]`（RegisterPanel から渡す）
- **プロトコル別メモリエリア**: `memoryAreasByProtocol: Record<string, MemoryAreaDTO[]>` でプロトコルごとにエリア一覧をキャッシュ
- **項目追加ダイアログ**: 複数サーバー時はプロトコル選択セレクトを表示
- **サーバー変化の検出**: `protocolTypesKey = serverInstances.map(i => i.protocolType).join(',')` でサーバー構成変化を効率的に検出してエリアを再取得
- **API 呼び出し**: `ReadWords(item.protocolType, item.memoryArea, ...)` など全 API に `protocolType` を渡す

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
- **プロトコルマッピング**: 変数を複数の起動中サーバーのメモリアドレスにマッピング可能
  - マッピングダイアログのプロトコル選択は `GetServerInstances()` で取得した起動中サーバーから生成
  - メモリエリア選択は `memoryAreasByProtocol[m.protocolType]` で取得
  - 2行表示レイアウト（ヘッダ行 + コントロール行）
  - マッピング競合警告: 他の変数と同じレジスタを使用している場合、ダイアログ内と変数一覧の両方に警告を表示
- **構造体型管理**: 構造体型の登録・編集・削除機能
  - 編集時は既存の定義を読み込んでフォームに展開
  - データ型の自動解析（ARRAY、STRING、構造体、スカラー）
  - スクロール対応（多数のフィールドを持つ構造体に対応）

#### ScriptPanel.tsx
JavaScript（goja）でPLC動作を記述。
- **エラー表示**: 実行時エラーをGUIに表示（タイムスタンプ付き、クリアボタン）
- **コンソールログ表示**: スクリプト一覧の下に「コンソール」セクションを常時表示
  - `console.log()` の出力をタイムスタンプ・スクリプト名付きで一覧表示
  - 新しいログ追加時に自動スクロール、クリアボタン付き
  - テスト実行（「テスト実行」ボタン）時の出力もスクリプト名「テスト実行」で表示
- **const/let対応**: スクリプトコードをIIFEでラップして再宣言エラーを回避
- **plcオブジェクト**:
  - メモリアクセス: `plc.readBit()`, `plc.writeBit()`, `plc.readWord()`, `plc.writeWord()`
  - 変数アクセス: `plc.readVariable()`, `plc.writeVariable()`, `plc.readArrayElement()`, `plc.writeArrayElement()`, `plc.readStructField()`, `plc.writeStructField()`
  - TIME/DATE シンタックスシュガー: `plc.readTimeMs()`, `plc.writeTimeMs()`, `plc.readDateSec()`, `plc.writeDateSec()`, `plc.readTimeOfDayMs()`, `plc.writeTimeOfDayMs()`, `plc.readDateAndTimeSec()`, `plc.writeDateAndTimeSec()`（変数の読み取り・パース・フォーマット・書き込みをワンステップで実行）
  - TIME/DATE 文字列変換: `plc.parseTime()`, `plc.formatTime()`, `plc.parseDate()`, `plc.formatDate()`, `plc.parseTimeOfDay()`, `plc.formatTimeOfDay()`, `plc.parseDateAndTime()`, `plc.formatDateAndTime()`

### Wailsバインディング

`app.go`でフロントエンドに公開するメソッドを定義。`wails generate module`でTypeScript型定義を自動生成。

主要API:
- **サーバー管理**:
  - `GetServerInstances()`: 全サーバーインスタンスの一覧（protocolType, displayName, variant, status）を取得
  - `AddServer(protocolType, variantID)`: 新しいサーバーインスタンスを追加
  - `RemoveServer(protocolType)`: サーバーインスタンスを削除
  - `StartServer(protocolType)`, `StopServer(protocolType)`: サーバーの起動/停止
  - `GetServerStatus(protocolType)`: サーバーのステータス取得
  - `GetProtocolSchema(protocolType)`: プロトコルのスキーマ（バリアント、フィールド定義）を取得
  - `GetServerConfig(protocolType)`: 特定サーバーの設定を取得
  - `UpdateServerConfig(dto)`: サーバー設定を更新（`ServerConfigDTO.protocolType` で対象を指定）
  - `GetAvailableProtocols()`: 追加可能なプロトコル一覧を取得
- **UnitID 設定**:
  - `GetUnitIDSettings(protocolType)`, `SetUnitIDEnabled(protocolType, unitId, enabled)`: UnitID 応答設定
  - `GetDisabledUnitIDs(protocolType)`, `SetDisabledUnitIDs(protocolType, ids)`: 無効 UnitID の一括管理
- **メモリ操作**（全て `protocolType string` を第1引数に取る）:
  - `GetMemoryAreas(protocolType)`: メモリエリア一覧を取得
  - `ReadBits(protocolType, area, address, count)`, `WriteBit(protocolType, area, address, value)`
  - `ReadWords(protocolType, area, address, count)`, `WriteWord(protocolType, area, address, value)`
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
  - `GetConsoleLogs()`, `ClearConsoleLogs()`: コンソールログの取得・クリア

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
- **競合警告**: 複数の変数が同じレジスタに割り当てられた場合、ワーニングを表示（保存は可能）
  - ダイアログ内: 各マッピング行の下に `.mapping-conflict-warning` で警告メッセージ
  - 変数一覧: マッピング列の先頭に `.mapping-conflict-icon` で ⚠ アイコン（ホバーでツールチップ）
  - 同一レジスタに複数変数をマッピングした場合の動作は未定義（last-writer-wins、読み取りは非決定的）

### 主要 DTO

`internal/application/dto.go` に定義:
- **`ServerInstanceDTO`**: サーバーインスタンス一覧表示用（`protocolType`, `displayName`, `variant`, `status`）
- **`ServerConfigDTO`**: サーバー設定の取得/更新用（`protocolType`, `variant`, `settings`）
- **`ServerSnapshotDTO`**: Export/Import 用の単一サーバースナップショット
- **`MonitoringItemDTO`**: `protocolType` フィールドを含む（どのサーバーのアドレスかを示す）
- **`ProjectDataDTO`**: Version 3 形式で `Servers []ServerSnapshotDTO` にマルチサーバー構成を保存。Version 1/2 との後方互換性あり

### 設定ファイル

アプリケーションの設定は以下の場所に保存されます：
- **モニタリング設定**: `%APPDATA%\PLCSimulator\monitoring_config.json`
  - 登録したモニタリング項目（プロトコルタイプ、メモリエリア、アドレス、ビット幅、エンディアン、表示形式）

## 新プロトコル追加手順

1. `internal/infrastructure/newprotocol/` ディレクトリを作成
2. 以下のファイルを実装:
   - `factory.go`: `ServerFactory`インターフェースを実装
   - `server.go`: `ProtocolServer`インターフェースを実装
   - `datastore.go`: `DataStore`インターフェースを実装
3. `factory.go`の`init()`で`protocol.Register()`を呼び出してレジストリに登録
4. **フロントエンド変更不要** - スキーマから自動生成

## 重要な実装ポイント

### 1オリジンアドレス表示

- **設計方針**: プロトコル固有の知識（どのエリアが1オリジンか）はバックエンドが保持し、フロントエンドはビュー層に持たせない
- **`MemoryArea.OneOrigin`**: `GetAreas()` が返す `MemoryArea` 構造体のフィールド。true の場合、UIでのアドレス表示を1オリジンにする
- **内部値は常に0ベース**: API (ReadWords/WriteWord等) には0ベースのアドレスを渡す。表示・入力変換のみUIで行う
- **フロントエンドの変換**: `area.oneOrigin` を参照して表示時に+1、入力時に-1。ハードコードの判定リストは持たない

### Reactコンポーネントの状態管理

- **コンポーネントの定義位置**: 関数コンポーネント内で別のコンポーネントを定義すると、再レンダリング時に新しいコンポーネントとして扱われ、`useState`などの状態がリセットされる
  - 例: `StructArrayElementEditor`は`VariableView`の外で定義
  - 内部で使用する関数は props で渡す
- **キーの設定**: リスト描画時は安定した`key`を設定（インデックスだけでなく、パスベースのキーを使用）

### スクリプトエンジン

- **const/let対応**: goja VMで同じプログラムを周期的に実行すると再宣言エラーが発生するため、IIFEでラップ
- **エラーハンドリング**: runtime panic をキャッチして`lastError`フィールドに保存し、GUI表示可能に
- **コンソールログ**: `console.log()` はバッファ（最大500件）に蓄積。`ConsoleLogEntry` に scriptID・scriptName・message・At を保存。フロントエンドは1秒ポーリングで `GetConsoleLogs()` を取得して表示。ミューテックスで保護
  - `createVM(scriptID, scriptName string)` にスクリプト識別子を渡すことで、どのスクリプトの出力かを記録
  - テスト実行（`RunOnce`）は scriptID="" / scriptName="テスト実行" でバッファに追加

### ダイアログスタイルの統一

すべてのダイアログ（変数編集、マッピング設定、構造体型管理、モニタリング追加、レジスタ書き込み）は統一されたスタイルパターンを使用：
- **`.dialog-row`**: ラベルとコントロールを横並びに配置
- **インラインスタイル削除**: すべての `<select>` と `<input>` からインラインスタイルを削除
- **スクロール対応**: ダイアログ全体に `maxHeight: '80vh'` を設定し、コンテンツ領域で `overflowY: 'auto'`
- **flexboxレイアウト**: ダイアログ全体を `display: flex, flexDirection: 'column'` でレイアウト

### 値編集ダイアログ

- **スクロール**: ダイアログ全体で1つのスクロール領域を使用（配列要素部分に独自スクロールを設定しない）
- **再帰的エディタ**: `renderValueEditor`は`depth`パラメータで再帰の深さを追跡し、インデントを調整

### マッピング競合検出

- **`findMappingConflicts(mapping)`** (`VariableView.tsx`): マッピングダイアログで各行の競合チェック。`getWordCount` でワード数を計算し、アドレス範囲の重複をチェック
- **`getVariableMappingConflicts(variable)`** (`VariableView.tsx`): 変数一覧テーブルで各変数の競合チェック。保存済みマッピングをすべてスキャン
- **未定義動作の注意**: 同一レジスタへの複数変数マッピングはサポート対象外。書き込みは last-writer-wins、DataStore → Variable の同期は Go マップの反復順に依存して非決定的

### 構造体型管理ダイアログ

- **編集機能**: 既存の構造体型を編集可能（編集ボタンで定義を読み込み、更新で保存）
- **データ型解析**: フィールドのデータ型文字列を解析してフォーム形式に変換
  - `ARRAY[型;サイズ]` → 配列カテゴリ + 要素型 + サイズ
  - `STRING[n]` → スカラーカテゴリ + STRING型 + バイト長
  - 構造体型名 → 構造体カテゴリ + 型名
- **型名の不変性**: 編集モード時は型名の入力欄を無効化（型名は変更不可）

## License

MIT License (Copyright 2026 bamchoh)
