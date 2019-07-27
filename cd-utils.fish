#!/usr/local/bin/fish

function cd
    if count $argv > /dev/null
        switch $argv
            case -b
                set path (cd-utils history latest)
            case -h
                set path (cd-utils history choice)
            case '*'
                set path $argv
        end
    else
        set path $HOME
    end
    if [ ! -n "$path" ]
        return
    end
    cd-utils history add $path
    builtin cd $path
end
