package binaryheap

type Heap struct {
	data       []interface{}
	comparator func(a, b interface{}) int
}

func NewWith(comparator func(a, b interface{}) int) *Heap {
	return &Heap{
		data:       make([]interface{}, 0),
		comparator: comparator,
	}
}

func (h *Heap) Push(value interface{}) {
	h.data = append(h.data, value)
	h.up(len(h.data) - 1)
}

func (h *Heap) Pop() (interface{}, bool) {
	if len(h.data) == 0 {
		return nil, false
	}

	top := h.data[0]
	last := len(h.data) - 1
	h.swap(0, last)
	h.data = h.data[:last]
	if len(h.data) > 0 {
		h.down(0)
	}

	return top, true
}

func (h *Heap) Empty() bool {
	return len(h.data) == 0
}

func (h *Heap) up(index int) {
	for index > 0 {
		parent := (index - 1) / 2
		if h.comparator(h.data[index], h.data[parent]) >= 0 {
			break
		}
		h.swap(index, parent)
		index = parent
	}
}

func (h *Heap) down(index int) {
	for {
		left := 2*index + 1
		right := left + 1
		smallest := index

		if left < len(h.data) && h.comparator(h.data[left], h.data[smallest]) < 0 {
			smallest = left
		}
		if right < len(h.data) && h.comparator(h.data[right], h.data[smallest]) < 0 {
			smallest = right
		}
		if smallest == index {
			return
		}

		h.swap(index, smallest)
		index = smallest
	}
}

func (h *Heap) swap(i, j int) {
	h.data[i], h.data[j] = h.data[j], h.data[i]
}
