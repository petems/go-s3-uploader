package main

import "sync"

type syncedList struct {
	list []string
	sync.Mutex
}

func (sl *syncedList) add(item string) {
	sl.Lock()
	sl.list = append(sl.list, item)
	sl.Unlock()
}
