#!/usr/local/bin/fish

function cd
    if count $argv > /dev/null
        switch $argv
            case -m
                set path (cd-utils bookmark choice)
                if [ $path = "No Bookmark" ]
                    echo $path
                    return
                end
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

function cb
    if count $argv > /dev/null
        switch $argv[1]
            case -l
                cd-utils bookmark list
            case -a
                cd-utils bookmark add $argv[2]
            case -d
                cd-utils bookmark delete
        end
    else
        cd -m
    end
end
