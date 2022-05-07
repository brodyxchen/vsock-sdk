package vsock

import (
	"fmt"
	"github.com/brodyxchen/vsock/client"
	"github.com/brodyxchen/vsock/models"
	"sync"
	"testing"
	"time"
)

type handleFunc func([]byte) ([]byte, error)

func TestKClientMRoutineNReq(t *testing.T) {
	LaunchCustomExampleServer(7070, time.Second, time.Second, time.Second*10, true, func(bytes []byte) ([]byte, error) {
		msg := string(bytes)
		//delay := time.Millisecond*80 + time.Duration(rand.Int63() % int64(time.Millisecond*40))
		//time.Sleep(delay)
		time.Sleep(time.Millisecond * 10)
		return []byte("rsp:" + msg), nil
	})

	// K-clients，每个client开启M-routines, 每个routine发送N-req, 总共 K*M*N个req
	K := 10
	M := 100
	N := 1000

	type Node struct {
		Msg   string
		Index int64
		Conns map[int64]int
		Count int
	}

	resultMap := make(map[string]*Node, 0)
	resultChan := make(chan *Node, K*N)
	go func() {
		for result := range resultChan {
			node, ok := resultMap[result.Msg]
			if ok {
				node.Count++
				node.Conns[result.Index]++
				resultMap[result.Msg] = node
			} else {
				result.Count++
				result.Conns = make(map[int64]int)
				result.Conns[result.Index]++
				resultMap[result.Msg] = result
			}
		}
	}()

	wg := sync.WaitGroup{}
	wg.Add(K * M)
	for k := 0; k < K; k++ {
		go func(kk int) {
			cfg := &client.Config{
				Timeout:         time.Millisecond * 100,
				PoolIdleTimeout: 0,
				PoolMaxCapacity: 0,
				WriteBufferSize: 0,
				ReadBufferSize:  0,
			}
			cli := NewClient(cfg)
			addr := &models.HttpAddr{
				IP:   "127.0.0.1",
				Port: 7070,
			}

			for m := 0; m < M; m++ {
				go func(kkk, mmm int) {
					for j := 0; j < N; j++ {
						req := "test client"
						deadline := time.Time{}
						if cli.Timeout > 0 {
							deadline = time.Now().Add(cli.Timeout)
						}
						connIndex, rspBytes, err := cli.SendTest(addr, "test", []byte(req), deadline)

						if err != nil {
							resultChan <- &Node{
								Msg:   err.Error(),
								Index: connIndex,
							}
							//if err != io.ErrUnexpectedEOF {
							//	t.Fatal(fmt.Sprintf("client[%v].Request[%v]: %v", ii, j, err.Error()))
							//} else {
							//	t.Fatal(fmt.Sprintf("client[%v].Request[%v]: %v", ii, j, err.Error()))	// dial tcp 127.0.0.1:7070: socket: too many open files
							//}
						} else {
							resultChan <- &Node{
								Msg:   string(rspBytes),
								Index: connIndex,
							}
							//if string(rspBytes) != "rsp:" + req {
							//	t.Fatal(fmt.Sprintf("client[%v].Request[%v]: rsp mismatch: %v", ii, j, string(rspBytes)))
							//}
						}
					}
					wg.Done()
					//fmt.Printf("client[%v].rountine[%v] done\n", kkk, mmm)
				}(kk, m)
			}
		}(k)
	}
	wg.Wait()

	for k, v := range resultMap {
		fmt.Printf("[%v]%v conns:{%+v}\n", k, v.Count, v.Conns)
	}
	select {}
}
