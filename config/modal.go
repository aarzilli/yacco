package config

import (
	"fmt"
	"os"
	"strings"
)

var ModalEnabled = false
var Modal = &ModalMapOrAction{Map: map[string]*ModalMapOrAction{}}

type ModalMapOrAction struct {
	Action string
	Map    map[string]*ModalMapOrAction
}

func postprocessModal(keys map[string]string) {
	ModalEnabled = true
	for keyseq, action := range keys {
		mmoa := modalCreate(Modal, strings.Split(keyseq, " "))
		if mmoa == nil || mmoa.Map != nil {
			fmt.Fprintf(os.Stderr, "Action for %s conflicts with other actions", keyseq)
			return
		}
		if mmoa.Action != "" {
			fmt.Fprintf(os.Stderr, "Action for %s defined to both %q and %q", keyseq, mmoa.Action, action)
			return
		}
		mmoa.Action = action
	}
}

func modalCreate(m *ModalMapOrAction, v []string) *ModalMapOrAction {
	if len(v) == 0 {
		return m
	}
	if m.Action != "" {
		return nil
	}
	if m.Map == nil {
		m.Map = make(map[string]*ModalMapOrAction)
	}
	if m.Map[v[0]] == nil {
		m.Map[v[0]] = new(ModalMapOrAction)
	}
	return modalCreate(m.Map[v[0]], v[1:])
}
