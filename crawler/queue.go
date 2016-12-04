package crawler

type ConcurrentCount struct {
	count int
	done  chan bool
}

func (c *ConcurrentCount) Add() {
	c.done <- true
}

func (c *ConcurrentCount) Done() {
	c.done <- false
}

func (c *ConcurrentCount) Wait() {
	for {
		if ok := <-c.done; ok {
			c.count++
		} else {
			c.count--
			if c.count <= 0 {
				break
			}
		}
	}
}
