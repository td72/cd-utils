# cd-utils: fish integration
#
# fish の `cd` を上書きし、移動履歴とブックマークを cd-utils (SQLite) に記録する。
#
# 前提となる cd-utils の CLI 契約:
#   - stdout に出るのは「データ」だけ (パスなど)
#   - メッセージ・エラーはすべて stderr に出る
#   - 失敗時は非 0 で終了する
# ここではコマンド置換 `(cd-utils ...)` の結果をそのままパスとして扱うので、
# cd-utils が stdout に余計な文字列を出さないことに依存している。
#
# 注意: `cd` を上書きするため、fish 標準の `cd -` / `prevd` / `nextd` / `cdh`
# (dirprev/dirnext によるディレクトリ履歴) は使えなくなる。直前のディレクトリへは
# `cd -b` を使うこと。`-` は `git projects choice all` に割り当てている。

function __cd_utils_record --description 'Record the current directory into cd history'
    # $PWD は必ず絶対パスなので、相対パス引数で移動した場合も正しく記録される。
    command cd-utils history add $PWD
end

function __cd_utils_postexec --on-event fish_postexec --description 'Keep the current directory at the head of cd history'
    # コマンドを実行するたびに現在地を touch しておく。
    # 「いま居るディレクトリが常に履歴の先頭」という前提が、
    # `cd -b` (2 番目 = 直前のディレクトリ) と `cd -h` の並び順を支えている。
    __cd_utils_record
end

function __cd_utils_git_projects --description 'Run git-projects, or explain that it is missing'
    if not type -q git-projects
        echo "cd: git-projects が見つかりません (cd - / -g / -G で必要です)" >&2
        echo "cd: インストールするか、代わりに cd -m / -h を使ってください" >&2
        return 1
    end
    command git projects $argv
end

function __cd_utils_cb_usage --description 'Print cb usage'
    echo "usage: cb [-l [--abs|-a]] | cb -a [path] | cb -d" >&2
    echo "  (no option)  ブックマークから選んで移動する (cd -m と同じ)" >&2
    echo "  -l           ブックマーク一覧を表示する" >&2
    echo "  -a [path]    path (省略時は \$PWD) をブックマークに追加する" >&2
    echo "  -d           ブックマークを選んで削除する" >&2
end

function cd --description 'cd with history / bookmark support (cd-utils)'
    set -l target

    if test (count $argv) -eq 0
        set target $HOME
    else
        switch $argv[1]
            case -m
                # ブックマークから fzf で選ぶ
                set target (command cd-utils bookmark choice)
                or return 1
            case -b
                # ひとつ前にいたディレクトリ
                set target (command cd-utils history latest)
                or return 1
            case -h
                # 履歴から fzf で選ぶ
                set target (command cd-utils history choice)
                or return 1
            case -l
                # 履歴を表示するだけ (移動しない)。--abs / -a などはそのまま渡す。
                command cd-utils history list $argv[2..-1]
                return $status
            case '-' '-g'
                # git 管理下のプロジェクトから選ぶ (全件)
                set target (__cd_utils_git_projects choice all)
                or return 1
            case '-G'
                # git 管理下のプロジェクトから選ぶ (絞り込み済み)
                set target (__cd_utils_git_projects choice)
                or return 1
            case -t
                # いまいる git リポジトリのトップレベル
                set target (command git rev-parse --show-toplevel)
                or return 1
            case '*'
                set target $argv
        end
    end

    # fzf をキャンセルした場合などは空になる。その場合は移動しない。
    if test -z "$target"
        return 0
    end

    # 出発地を履歴に積んでおく。これで移動後に `cd -b` が
    # 「直前にいたディレクトリ」を返せるようになる。
    __cd_utils_record

    # 移動に成功したときだけ移動先を履歴に積む。
    builtin cd $target
    or return 1
    __cd_utils_record
    ls
end

function cb --description 'cd-utils bookmark helper'
    if test (count $argv) -eq 0
        cd -m
        return $status
    end

    switch $argv[1]
        case -l
            command cd-utils bookmark list $argv[2..-1]
        case -a
            if set -q argv[2]
                command cd-utils bookmark add $argv[2]
            else
                command cd-utils bookmark add $PWD
            end
        case -d
            command cd-utils bookmark delete
        case '*'
            echo "cb: unknown option: $argv[1]" >&2
            __cd_utils_cb_usage
            return 1
    end
end
