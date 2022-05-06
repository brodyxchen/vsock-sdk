package client

import (
	"sync"
	"time"
)

type ConnPool struct {
	pool  map[connectKey][]*PersistConn
	mutex sync.RWMutex

	idleTimeout time.Duration

	maxCountPerKey int
}

func (cp *ConnPool) Get(key connectKey) *PersistConn {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	var idleBegin time.Time
	if cp.idleTimeout > 0 {
		idleBegin = time.Now().Add(-cp.idleTimeout)
	}

	list, ok := cp.pool[key]
	if !ok {
		return nil
	}

	// 倒序查找
	for len(list) > 0 {
		pConn := list[len(list)-1]

		if pConn.closed {
			list = list[:len(list)-1]
			pConn.close()
			continue
		}

		tooOld := !idleBegin.IsZero() && pConn.idleAt.Round(0).Before(idleBegin)
		if tooOld {
			list = list[:len(list)-1]
			pConn.close()
			continue
		}

		// 取出
		list = list[:len(list)-1] // 从缓存删除
		if len(list) > 0 {
			cp.pool[key] = list
		} else {
			delete(cp.pool, key)
		}

		// 清理数据
		if pConn.idleTimer != nil {
			pConn.idleTimer.Stop()
			pConn.idleTimer = nil
		}
		pConn.idleAt = time.Time{}
		return pConn
	}

	return nil
}

func (cp *ConnPool) Put(conn *PersistConn) {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	idleTimeout := cp.idleTimeout

	conn.reused = true
	conn.idleAt = time.Now()
	if conn.idleTimer != nil {
		conn.idleTimer.Reset(idleTimeout)
	} else {
		conn.idleTimer = time.AfterFunc(idleTimeout, conn.closeAndRemove) // 空闲状态才关闭
	}

	key := conn.key
	list, ok := cp.pool[key]
	if !ok {
		list = make([]*PersistConn, 0)
	}

	if cp.maxCountPerKey > 0 && len(list) >= cp.maxCountPerKey {
		cutPoint := len(list) - cp.maxCountPerKey + 1
		removed := list[:cutPoint]
		for _, v := range removed {
			v.close()
		}
		list = list[cutPoint:]
	}

	list = append(list, conn)
	cp.pool[key] = list
}

func (cp *ConnPool) Remove(pConn *PersistConn) {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	list, ok := cp.pool[pConn.key]
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
		cp.pool[conn.key] = list[:len(list)-1]
	}
}

func (cp *ConnPool) Exist(pConn *PersistConn) bool {
	cp.mutex.RLock()
	defer cp.mutex.RUnlock()

	list, ok := cp.pool[pConn.key]
	if !ok {
		return false
	}

	if len(list) <= 0 {
		return false
	}

	for _, conn := range list {
		if conn != pConn {
			continue
		}
		return true
	}

	return false
}
