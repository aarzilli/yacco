module github.com/aarzilli/yacco

require (
	github.com/BurntSushi/xgb v0.0.0-20160522181843-27f122750802
	github.com/aarzilli/go2def v0.0.0-20220703140930-d8673a371c40
	github.com/godbus/dbus/v5 v5.1.0
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0
	github.com/kr/pty v0.0.0-20160716204620-ce7fa45920dc
	github.com/lionkov/go9p v0.0.0-20180620135904-0a603dd6808a
	github.com/sourcegraph/jsonrpc2 v0.0.0-20190106185902-35a74f039c6a
	github.com/wendal/readline-go v0.0.0-20130305043046-3ff003ef4c80
	golang.org/x/exp v0.0.0-20180710024300-14dda7b62fcd
	golang.org/x/image v0.0.0-20180708004352-c73c2afc3b81
	golang.org/x/mobile v0.0.0-20180719123216-371a4e8cb797
	golang.org/x/tools v0.1.12
)

require (
	github.com/gorilla/websocket v1.5.3 // indirect
	golang.org/x/mod v0.6.0-dev.0.20220419223038-86c51ed26bb4 // indirect
	golang.org/x/sys v0.0.0-20220722155257-8c9f86f7a55f // indirect
)

replace golang.org/x/exp => github.com/aarzilli/exp v0.0.0-20250508142343-e302c350bfce

replace github.com/BurntSushi/xgb => github.com/aarzilli/xgb v0.0.0-20170123161437-2ca2a6c0622c

replace github.com/golang/freetype => github.com/aarzilli/freetype v0.0.0-20180724121948-6c8832ae5783

go 1.18

