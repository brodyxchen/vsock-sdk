package vsock

import (
	"sync"
	"time"
)

type ConnPool struct {
	pool  map[connectKey][]*PersistConn
	mutex sync.RWMutex

	maxCount int
}

func (cp *ConnPool) Get(key connectKey, idleBegin time.Time) *PersistConn {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	list, ok := cp.pool[key]
	if !ok {
		return nil
	}

	// 倒序查找
	for len(list) > 0 {
		pConn := list[len(list)-1]

		if pConn.closed {
			list = list[:len(list)-1]
			continue
		}

		tooOld := !idleBegin.IsZero() && pConn.idleAt.Round(0).Before(idleBegin)
		if tooOld {
			list = list[:len(list)-1]
			pConn.closeIfIdle()
			continue
		}

		list = list[:len(list)-1] // 从缓存删除
		if len(list) > 0 {
			cp.pool[key] = list
		} else {
			delete(cp.pool, key)
		}

		pConn.state = ConnStateBusy
		return pConn
	}

	return nil
}

func (cp *ConnPool) Put(conn *PersistConn, idleTimeout time.Duration) {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	conn.reused = true
	conn.state = ConnStateIdle

	conn.idleAt = time.Now()
	if conn.idleTimer != nil {
		conn.idleTimer.Reset(idleTimeout)
	} else {
		conn.idleTimer = time.AfterFunc(idleTimeout, conn.closeIfIdle) // 空闲状态才关闭
	}

	key := conn.connectKey
	list, ok := cp.pool[key]
	if !ok {
		list = make([]*PersistConn, 0)
	}

	if cp.maxCount > 0 && len(list) >= cp.maxCount {
		cut := len(list) - cp.maxCount + 1
		list = list[cut:]
	}

	list = append(list, conn)
	cp.pool[key] = list
}

func (cp *ConnPool) Remove(pConn *PersistConn) {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	list, ok := cp.pool[pConn.connectKey]
	if !ok {
		return
	}

	if len(list) <= 0 {
		return
	}

	for k, conn := range list {
		if conn != pConn {
			continue
		}

		copy(list[k:], list[k+1:])
		cp.pool[conn.connectKey] = list[:len(list)-1]
	}
}

//func (cp *ConnPool) Exist(pConn *PersistConn) bool {
//	list, ok := cp.pool[pConn.connectKey]
//	if !ok {
//		return false
//	}
//
//	if len(list) <= 0 {
//		return false
//	}
//
//	for _, conn := range list {
//		if conn != pConn {
//			continue
//		}
//
//		return true
//	}
//
//	return false
//}
