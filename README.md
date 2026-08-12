# cd-utils

fish の `cd` を拡張するための小さな CLI ツールです。

- `cd` した先のディレクトリを **履歴** として SQLite に蓄積する
- よく使うディレクトリを **ブックマーク** として登録する
- 履歴・ブックマークを **fzf** で絞り込んで、そのままそこへ移動する

`cd-utils` バイナリ本体はデータの記録と検索だけを行い、実際のディレクトリ移動は
同梱の `cd-utils.fish` が担当します（`cd` / `cb` 関数を定義します）。

## 必要なもの

| ツール | 用途 |
| --- | --- |
| [fish](https://fishshell.com/) | `cd` / `cb` 関数の実行シェル |
| [fzf](https://github.com/junegunn/fzf) | 履歴・ブックマークの絞り込み UI |
| [mise](https://mise.jdx.dev/) | ビルド時の Go ツールチェーン管理・タスクランナー |

Go 自体は mise が入れるので、別途インストールしておく必要はありません。

任意（無くても他の機能は動きます）:

| ツール | 用途 |
| --- | --- |
| [git-projects](https://github.com/hosokawa-kenshi/git-projects) | `cd -` / `cd -g` / `cd -G` で git リポジトリを選ぶのに使う |

`git-projects` が未インストールの場合、`cd -` / `-g` / `-G` はその旨を stderr に
出して終了ステータス 1 で終わります（他のサブコマンドには影響しません）。

## インストール

```console
$ git clone git@github.com:td72/cd-utils.git
$ cd cd-utils
$ mise trust          # このリポジトリの mise.toml を信頼する
$ mise install        # mise.toml / mise.lock に従って Go を入れる
$ mise run build      # go build ./... （カレントに cd-utils バイナリができる）
```

`PATH` の通った場所に置きたい場合は `go install` を使います。

```console
$ mise exec -- go install .
```

`$GOBIN`（未設定なら `$GOPATH/bin`、既定では `~/go/bin`）に `cd-utils` が入るので、
そこに `PATH` を通しておいてください。`cd-utils.fish` は `cd-utils` が `PATH` 上に
あることを前提にしています。

## fish へのセットアップ

`cd-utils.fish` を `conf.d` に置くと、fish 起動時に自動で読み込まれます。

```console
$ ln -s (pwd)/cd-utils.fish ~/.config/fish/conf.d/cd-utils.fish
```

`config.fish` から明示的に読み込んでも構いません。

```fish
source /path/to/cd-utils/cd-utils.fish
```

読み込んだら、新しい fish を起動するか `exec fish` で反映します。

> **注意: 既存の定義を先に削除すること**
>
> `cd-utils.fish` は `cd` / `cb` / `fish_postexec` フックをまとめて定義します。
> 同名の定義が `~/.config/fish` に残っていると二重定義になり、どちらが効くか
> 分からなくなります。次のファイルがあれば先に削除してください。
>
> - `~/.config/fish/functions/cd.fish`
> - `~/.config/fish/functions/cb.fish`
> - `~/.config/fish/conf.d/cd_utils_postexec.fish`
>
> これらは `cd-utils.fish` に取り込み済みです。

## 使い方

### `cd`

| コマンド | 動作 |
| --- | --- |
| `cd` | ホームディレクトリへ移動（従来どおり） |
| `cd <path>` | `<path>` へ移動（従来どおり。成功時のみ履歴に記録） |
| `cd -m` | ブックマークから fzf で選んで移動 |
| `cd -b` | ひとつ前にいたディレクトリへ移動 |
| `cd -h` | 履歴から fzf で選んで移動 |
| `cd -l` | 履歴を一覧表示する（移動しない）。`cd -l --abs` のように追加引数を渡せる |
| `cd -` / `cd -g` | `git projects choice all` で git リポジトリを選んで移動 |
| `cd -G` | `git projects choice` で git リポジトリを選んで移動（絞り込み済み） |
| `cd -t` | いまいる git リポジトリのトップレベルへ移動 |

移動に成功すると最後に `ls` を実行します。移動に失敗した場合は履歴に記録されません。
fzf をキャンセルした場合は移動もエラーもせず、そのままのディレクトリに留まります。

`cd` は移動の前後で現在地を履歴に記録します。加えて `fish_postexec` フックが
コマンド実行のたびに現在地を記録するため、「いま居るディレクトリが常に履歴の先頭」
という状態が保たれます。`cd -b` が「ひとつ前」を返せるのはこの前提のおかげです。

### `cb`（ブックマーク操作）

| コマンド | 動作 |
| --- | --- |
| `cb` | `cd -m` と同じ（ブックマークから選んで移動） |
| `cb -l` | ブックマーク一覧を表示 |
| `cb -a [path]` | `path`（省略時はカレントディレクトリ）をブックマークに追加 |
| `cb -d` | ブックマークを選んで削除 |

上記以外のオプションを渡した場合は usage を stderr に出して終了ステータス 1 で
終わります。

### `cd-utils` サブコマンド

`cd-utils` は **stdout にはデータだけを出力** します。メッセージやエラーは
すべて stderr に出し、失敗時は非 0 で終了します。`cd-utils.fish` はこの契約に
依存して、コマンド置換 `(cd-utils ...)` で受け取った出力をそのままパスとして
扱い、成否は終了ステータスで判定しています。**stdout に余計な文字列を足すと
fish 側の `cd` が壊れます。**

| コマンド | stdout | stderr / 終了コード |
| --- | --- | --- |
| `cd-utils history list [--abs\|-a]` | 履歴を 1 行 1 パス。0 件なら無出力 | exit 0 |
| `cd-utils history update` | 無出力 | 存在しなくなったパスを `remove: <path>` として stderr に出す。exit 0 |
| `cd-utils history add <path>` | 無出力 | 引数なし / ディレクトリでない → stderr + exit 1 |
| `cd-utils history choice` | 選択した絶対パスを 1 行。キャンセル時は無出力 | 履歴 0 件 → stderr + exit 1。キャンセルは exit 0 |
| `cd-utils history latest` | ひとつ前のディレクトリの絶対パスを 1 行 | 履歴が 2 件未満 → 無出力 + exit 1 |
| `cd-utils bookmark list [--abs\|-a]` | ブックマークを 1 行 1 パス | 0 件 → stderr に `No bookmark` + exit 0 |
| `cd-utils bookmark add <path>` | 無出力 | 登録済み → stderr + exit 0。引数なし / 非ディレクトリ → stderr + exit 1 |
| `cd-utils bookmark choice` | 選択した絶対パスを 1 行 | 0 件 → stderr + exit 1。キャンセルは無出力 + exit 0 |
| `cd-utils bookmark delete` | 無出力 | 確認プロンプトは stderr。`y` / `Y` / `yes` のときだけ削除し、それ以外は中止 |
| `cd-utils window_path update -i <id> -p <path>` | 無出力 | 失敗時 stderr + exit 1 |
| `cd-utils window_path latest -i <id>` | 絶対パスを 1 行 | 該当なし → 無出力 + exit 1 |

フラグの別名は `--abs, -a` / `--window_id, -i` / `--path, -p` です。

`list` 系は既定でホームディレクトリ配下を `~/...` の形（チルダ表記）で表示します。
`--abs` / `-a` を付けると絶対パスで出力します。`choice` / `latest` は fish から
`cd` に渡す前提なので常に絶対パスです。

存在しなくなったディレクトリが履歴に溜まってきたら、`cd-utils history update`
で掃除できます。

## 設定

履歴・ブックマークを保存する SQLite ファイルの場所は、次の順で解決されます。

1. 環境変数 `$CD_HISTORY_CACHE_PATH`（設定されていればそのパスをそのまま使う）
2. `$XDG_CACHE_HOME/cd-history.db`
3. `$HOME/.cache/cd-history.db`

例:

```fish
set -gx CD_HISTORY_CACHE_PATH ~/.local/share/cd-utils/cd-history.db
```

## 開発

タスクは mise で定義されています。

| タスク | 内容 |
| --- | --- |
| `mise run build` | `go build ./...` |
| `mise run test` | `go test ./...` |
| `mise run vet` | `go vet ./...` |
| `mise run fmt` | `gofmt -w .` |
| `mise run fmt:check` | 未フォーマットのファイルがあれば失敗 |
| `mise run tidy` | `go mod tidy -diff`（go.mod / go.sum が untidy なら失敗） |
| `mise run ci` | 上記のうち `fmt:check` / `vet` / `tidy` / `build` / `test` をまとめて実行 |

SQLite ドライバは純 Go 実装を使うため cgo は不要で、`mise.toml` の `[env]` で
`CGO_ENABLED = "0"` を設定しています。

## 既知の制限

- `cd` を上書きするため、fish 標準の `cd -` / `prevd` / `nextd` / `cdh`
  （`dirprev` / `dirnext` によるディレクトリ履歴）は使えません。`-` は
  `git projects choice all` に割り当てています。ひとつ前のディレクトリへは
  `cd -b` を使ってください。
- `window_path` サブコマンド（`update` / `latest`）は実装されていますが、
  `cd-utils.fish` からは配線されていません。ターミナルのウィンドウ / ペインごとに
  cwd を記憶して復元する用途を想定した機能ですが、現状は未使用です。
- `cd-utils.fish` は fish 専用です。bash / zsh 向けの統合はありません。
- 履歴・ブックマークは 1 つの SQLite ファイルを共有します。複数のウィンドウや
  ペインで並行して `cd` すると、`cd -b` が別ペインで移動した先を返すことがあります。
