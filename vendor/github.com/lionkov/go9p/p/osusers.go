// Copyright 2009 The Go9p Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package p

import (
	"os/user"
	"strconv"
	"sync"
)

type osUser struct {
	*user.User
	uid int
	gid int
}

type osUsers struct {
	groups map[int]*osGroup
	sync.Mutex
}

// Simple Users implementation that defers to os/user and fakes
// looking up groups by gid only.
var OsUsers *osUsers

func init() {
	OsUsers = new(osUsers)
	OsUsers.groups = make(map[int]*osGroup)
}

func (u *osUser) Name() string { return u.Username }

func (u *osUser) Id() int { return u.uid }

func (u *osUser) Groups() []Group { return []Group{OsUsers.Gid2Group(u.gid)} }

func (u *osUser) IsMember(g Group) bool { return u.gid == g.Id() }

type osGroup struct {
	gid int
}

func (g *osGroup) Name() string { return "" }

func (g *osGroup) Id() int { return g.gid }

func (g *osGroup) Members() []User { return nil }

func newUser(u *user.User) *osUser {
	uid, uerr := strconv.Atoi(u.Uid)
	gid, gerr := strconv.Atoi(u.Gid)
	if uerr != nil || gerr != nil {
		/* non-numeric uid/gid => unsupported system */
		return nil
	}
	return &osUser{u, uid, gid}
}

func (up *osUsers) Uid2User(uid int) User {
	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return nil
	}
	return newUser(u)
}

func (up *osUsers) Uname2User(uname string) User {
	u, err := user.Lookup(uname)
	if err != nil {
		return nil
	}
	return newUser(u)
}

func (up *osUsers) Gid2Group(gid int) Group {
	up.Lock()
	group, present := up.groups[gid]
	if present {
		up.Unlock()
		return group
	}

	group = new(osGroup)
	group.gid = gid
	up.groups[gid] = group
	up.Unlock()
	return group
}

func (up *osUsers) Gname2Group(gname string) Group {
	return nil
}
