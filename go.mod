module github.com/aarzilli/yacco

require (
	github.com/BurntSushi/xgb v0.0.0-20160522181843-27f122750802
	github.com/aarzilli/go2def v0.0.0-20200405090725-d6ba1b677cfd
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0
	github.com/kr/pty v0.0.0-20160716204620-ce7fa45920dc
	github.com/lionkov/go9p v0.0.0-20180620135904-0a603dd6808a
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646 // indirect
	github.com/sourcegraph/jsonrpc2 v0.0.0-20190106185902-35a74f039c6a
	github.com/wendal/readline-go v0.0.0-20130305043046-3ff003ef4c80
	golang.org/x/exp v0.0.0-20180710024300-14dda7b62fcd
	golang.org/x/image v0.0.0-20180708004352-c73c2afc3b81
	golang.org/x/mobile v0.0.0-20180719123216-371a4e8cb797
	golang.org/x/tools v0.1.8-0.20211028023602-8de2a7fd1736
)

replace golang.org/x/exp => github.com/aarzilli/exp v0.0.0-20180724135916-edb5f9d2bb76

replace github.com/BurntSushi/xgb => github.com/aarzilli/xgb v0.0.0-20170123105216-2ca2a6c0622c

replace github.com/golang/freetype => github.com/aarzilli/freetype v0.0.0-20160928082430-6c8832ae5783

go 1.13
