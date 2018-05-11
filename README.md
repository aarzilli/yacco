## What is this?!

This is a reimplementation of [acme](https://www.youtube.com/watch?v=dP1xVpMPn8M) with features deemed unclean by the wider plan 9 community. It is written in [Go](https://golang.org/).

## Differences from acme

There's a number of extra features and changes from acme, since I've never bothered cataloguing them I can't list them now. This is a list of some of the more important ones

* Plumbing is builtin and can be configured from yacco's config file. Right clicking outside of a selection will "plumb" the entire line instead of the single word under the seletion. The first rule to match the text around the position of the click will be executed. This change was done to make dealing with python easier.

* Default keybindings are defined in config/keys.go, additional keybindings can be defined using the "Keybindings" section of the configuration file. Up/down change line, left/right move by one character, ctrl-left/ctrl-right move by one word. Ctrl-backspace deletes one word.

* Cutting is called Cut instead of Snarf. Pasting will attempt to adjust the indenation of the text being pasted.

* Ctrl+left click is equivalent to middle clicking. Ctrl+middle is equivalent to the weird middle+left click chord in acme.

* The version of win shipped with yacco will strip ANSI escape codes from the output before sending it to yacco. It even supports a few of them.

* There's a number of differences in the Edit languages due to either underspecification in the man page, mistakes or deliberate changes and additions. The 's' command will always replace all occourences in the selection, the 'g' comamnd will only evaluate its argument when the regexp matches the entire selection. The 'X' and 'Y' commands have barely been tested.

* Minimal syntax highlighting is implemented. The only supported languages are Go, C, C++, Java, Javascript and Python. The rules are in config/config.go, the LanguageRules variable. All that's implemented is highlighting strings and comments in different colors, there's no provisions for coloring numbers or keywords differently.

* Color themes are defined in config/color_schemes.go. The theme can be changed by using the Theme build in command or by changing adding a -t option to the startup script.

* Ctrl-f/Ctrl-g implement the search-as-you-type-interactive-search that every other editor has.

* LookFile or Ctrl-q implements the fuzzy-file-search feature that every other editor has. Type return to open the first search result, or right click on any of the results.

* Dump will save to `~/.config/yacco/` by default, the dump file will be updated every time you save any open file.

## Acme compatibility

* Mouse chording was implemented but I found it inferior to ctrl+c/v/x and never used it. It probably has bugs.

* The 9p protocol is the (more or less) same as acme but yacco serves it at $NAMESPACE/yacco.$YACCOPID if NAMESPACE is defined, or at /tmp/ns.$USER.$DISPLAY/yacco.$YACCOPID if NAMESPACE is not defined. If you want yacco to pretend to be acme you can do that with the -acme command line option. I've tested this with win and Mail from plan9port and it seems to work, but testing was far from thorough.

## Installing

Run `./install.sh installdir`. This will create two files and one directory:

* `$installdir/yaccodir`: a directory containing all of yacco's executables
* `$installdir/yacco`: yacco's startup script (change this if you want a different default theme)
* `~/.config/yacco/rc`: yacco's configuration file.

## Compiling on macOS/Windows

* The verison of shiny in `vendor/golang.org/x/exp/shiny/` is modified and does not compile on windows/macOS anymore. There should only be two added methods SetTitle and WarpMouse, neither is indespensible and they can both be stubbed out with little loss of functionality.

* Assumptions of unix-like behavior (and even linux-like in particular) were made throughout the program, using it on non-linux systems (especially windows) may cause difficulties.
