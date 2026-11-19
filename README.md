# aurch

AURパッケージの簡易セキュリティチェッカー（Go / Bubble Tea製TUI）。

## 使い方

```sh
aurch <package-name>
```

## 動作

1. 引数でパッケージ名を受け取り、`/tmp` に一時作業ディレクトリを作成（mktemp -d 相当）
2. `paru --getpkgbuild` または `yay -G`（内部で git clone）でAURパッケージを取得
3. 取得ファイルをすべて読み取り専用化（`chmod -R a-w` 相当）
4. PKGBUILD / *.install / .SRCINFO を解析（コメント行は除外）：
   - URL（http://, https://, git://, ftp://）
   - 禁止コマンド（rm -rf /, mkfs, dd if=, chmod 777, curl | sh, wget | bash など）
   - パッケージマネージャ呼び出し（go get, pip install, npm install, cargo build,
     yay -S, paru -S, pacman -S など）
5. 結果をTUIで表示：
   - ディレクトリツリー
   - 検知行の一覧（一律表示・色分けなし、j/k でスクロール）
   - 大量ヒット時は警告表示（例: `⚠️ 32件検出`）
6. TUI上でのキー操作：
   - `i` : 作業ディレクトリを削除し、paru/yay を自動検出（paru優先・exec.LookPath）して
     インストール実行：
     ```sh
     paru --review --needed --cleanafter --removemake <pkg>
     # または
     yay  --review --needed --cleanafter --removemake <pkg>
     ```
   - `q` / `esc` / `ctrl+c` : 終了

## エラーハンドリング

- AUR helper（paru/yay）が見つからない場合はエラーメッセージを表示
- インストール失敗時はエラーコード（exit code 1、ヘルパーの終了コードをメッセージに表示）を返す
- プログラム終了時は defer で作業ディレクトリを確実に削除

## 注意

aurch は**簡易チェッカー**です。パターンマッチによる目安の提示のみを行い、
最終的なPKGBUILDの内容確認は yay/paru の `--review` 機能に委ねます。
インストール時に必ずレビュー表示でPKGBUILDをご確認ください。

## ビルド・テスト

```sh
go build -o aurch .
go test ./...
```
