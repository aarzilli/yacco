package main

import (
	"time"
	"yacco/buf"
)

type refreshRequest struct {
	kb        *buf.Buffer // buffer to refresh
	eds       []*Editor   // editors to scroll
	scrollAll bool
}

var refreshRequests = []refreshRequest{}

func appendRefreshMsg(rr *refreshRequest, ed *Editor, scroll bool) {
	if ed == nil {
		rr.scrollAll = rr.scrollAll || scroll
	} else {
		for i := range rr.eds {
			if rr.eds[i] == ed {
				return
			}
		}
		rr.eds = append(rr.eds, ed)
	}
}

func RefreshMsg(b *buf.Buffer, ed *Editor, scroll bool) func() {
	return func() {
		startTimer := len(refreshRequests) == 0
		found := false
		for i := range refreshRequests {
			if refreshRequests[i].kb == b {
				found = true
				appendRefreshMsg(&refreshRequests[i], ed, scroll)
				break
			}
		}
		if !found {
			refreshRequests = append(refreshRequests, refreshRequest{kb: b, eds: []*Editor{}, scrollAll: scroll})
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
					e.BufferRefreshEx(false, refreshRequests[i].scrollAll)
				}
			}
		}

		if !refreshRequests[i].scrollAll {
			for _, ed := range refreshRequests[i].eds {
				ed.BufferRefresh()
			}
		}
	}
	refreshRequests = refreshRequests[0:0]
}
