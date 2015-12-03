package utils

import (
	"container/list"
	"reflect"
)

// TODO add mutex locker
type LBRing struct {
	List *list.List
	_cur *list.Element
	Len  int
}

func (l *LBRing) Next() interface{} {
	if l._cur == nil && l.List.Len() == 0 {
		return nil
	} else {
		if l._cur != nil {
			ret := l._cur
			l._cur = l._cur.Next()
			if l._cur == nil {
				l._cur = l.List.Front()
			}
			return ret.Value
		} else {
			l._cur = l.List.Front().Next()
			return l.List.Front().Value
		}
	}
	return nil
}

func (l *LBRing) Add(a interface{}) {
	if l.List == nil {
		l.List = list.New()
	}
	l.List.PushBack(a)
	l.Len++
}

func (l *LBRing) Remove(a interface{}) {
	for h := l.List.Front(); h != nil; h = h.Next() {
		if reflect.DeepEqual(h.Value, a) {
			if l._cur == h {
				l.Next()
				if l._cur == h {
					l._cur = nil
				}
			}
			l.List.Remove(h)
			l.Len--
		}
	}
}
