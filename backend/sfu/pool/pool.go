package pool

import (
	"sync"
)

// Free stores the index of the slots.
type Pool struct {
	Slots []*MediaSlot
	Free  []int

	Mu sync.Mutex
}

func (p *Pool) Acquire() {

}

func (p *Pool) Release() {

}

func (p *Pool) Grow() {

}
