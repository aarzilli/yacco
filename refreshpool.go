package main

import (
	"time"
	"yacco/buf"
)

type refreshRequest struct {
	kb        *buf.Buffer         // buffer to refresh
	brs       []BufferRefreshable // editors to scroll
	scrollAll bool
}

var refreshRequests = []refreshRequest{}

func appendRefreshMsg(rr *refreshRequest, br BufferRefreshable, scroll bool) {
	if br == nil {
		rr.scrollAll = rr.scrollAll || scroll
	} else {
		for i := range rr.brs {
			if rr.brs[i] == br {
				return
			}
		}
		rr.brs = append(rr.brs, br)
	}
}

func RefreshMsg(b *buf.Buffer, br BufferRefreshable, scroll bool) func() {
	return func() {
		startTimer := len(refreshRequests) == 0
		found := false
		for i := range refreshRequests {
			if refreshRequests[i].kb == b {
				found = true
				appendRefreshMsg(&refreshRequests[i], br, scroll)
				break
			}
		}
		if !found {
			refreshRequests = append(refreshRequests, refreshRequest{kb: b, brs: []BufferRefreshable{}, scrollAll: false})
		}
		if startTimer {
			go func() {
				time.Sleep(50 * time.Millisecond)
				sideChan <- RefreshReal
			}()
		}
	}
}

func RefreshReal() {
	for i := range refreshRequests {
		for _, col := range Wnd.cols.cols {
			for _, e := range col.editors {
				if e.bodybuf == refreshRequests[i].kb {
					e.BufferRefreshEx(false, false, refreshRequests[i].scrollAll)
				}
			}
		}

		if !refreshRequests[i].scrollAll {
			for _, br := range refreshRequests[i].brs {
				br.BufferRefresh(false)
			}
		}
	}
	refreshRequests = refreshRequests[0:0]
}
