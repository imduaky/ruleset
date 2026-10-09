// Modified from sing, Copyright (C) 2022 by nekohasekai <contact-sagernet@sekai.icu>, GPL-3.0-or-later,
// which is modified from https://github.com/openacid/succinct (MIT).
// https://github.com/SagerNet/sing/blob/6f21f2425a959912c37d2ef43d61e2a663315dea/common/domain/set.go

package singdomain

import (
	"fmt"
	"math/bits"

	"github.com/imduaky/ruleset/pkg/internal/singvarbin"
)

// SRS uses sing's LOUDS succinct trie format, derived from openacid/succinct.
// Only construction, serialization and key recovery are needed here.
type succinctSet struct {
	leaves, labelBitmap []uint64
	labels              []byte
	nodeStarts          []int
}

func newSuccinctSet(keys []string) *succinctSet {
	ss := new(succinctSet)
	type entry struct{ start, end, column int }
	queue := []entry{{0, len(keys), 0}}
	bitIndex := 0
	for i := 0; i < len(queue); i++ {
		item := queue[i]
		if item.start < item.end && item.column == len(keys[item.start]) {
			item.start++
			setBit(&ss.leaves, i, 1)
		}
		for j := item.start; j < item.end; {
			start := j
			for j < item.end && keys[j][item.column] == keys[start][item.column] {
				j++
			}
			queue = append(queue, entry{start, j, item.column + 1})
			ss.labels = append(ss.labels, keys[start][item.column])
			setBit(&ss.labelBitmap, bitIndex, 0)
			bitIndex++
		}
		setBit(&ss.labelBitmap, bitIndex, 1)
		bitIndex++
	}
	return ss
}

func readSuccinctSet(r singvarbin.Reader) (*succinctSet, error) {
	version, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	if version != 0 {
		return nil, fmt.Errorf("domain: invalid trie version: %d", version)
	}
	leaves, err := singvarbin.ReadSlice[uint64](r)
	if err != nil {
		return nil, err
	}
	bitmap, err := singvarbin.ReadSlice[uint64](r)
	if err != nil {
		return nil, err
	}
	labels, err := singvarbin.ReadSlice[byte](r)
	if err != nil {
		return nil, err
	}
	ones, lastOne := 0, -1
	for i, word := range bitmap {
		ones += bits.OnesCount64(word)
		if word != 0 {
			lastOne = i*64 + 63 - bits.LeadingZeros64(word)
		}
	}
	zeros := lastOne + 1 - ones
	if ones != zeros+1 || len(labels) != zeros {
		return nil, fmt.Errorf("domain: malformed succinct set")
	}
	ss := &succinctSet{leaves: leaves, labelBitmap: bitmap, labels: labels}
	ss.nodeStarts = make([]int, 0, ones)
	ss.nodeStarts = append(ss.nodeStarts, 0)
	pending := 1
	for i := 0; i <= lastOne; i++ {
		if getBit(bitmap, i) {
			pending--
			if i < lastOne {
				ss.nodeStarts = append(ss.nodeStarts, i+1)
			}
		} else {
			pending++
		}
		if pending == 0 && i != lastOne {
			return nil, fmt.Errorf("domain: disconnected succinct set")
		}
	}
	return ss, nil
}

func (ss *succinctSet) Write(w singvarbin.Writer) error {
	err := w.WriteByte(0)
	if err != nil {
		return err
	}
	err = singvarbin.WriteSlice(w, ss.leaves)
	if err != nil {
		return err
	}
	err = singvarbin.WriteSlice(w, ss.labelBitmap)
	if err != nil {
		return err
	}
	return singvarbin.WriteSlice(w, ss.labels)
}

func (ss *succinctSet) keys() []string {
	var result []string
	var key []byte
	type frame struct{ node, bitIndex int }
	stack := []frame{{0, 0}}
	if getBit(ss.leaves, 0) {
		result = append(result, "")
	}
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if getBit(ss.labelBitmap, top.bitIndex) {
			stack = stack[:len(stack)-1]
			if len(stack) > 0 {
				key = key[:len(key)-1]
				stack[len(stack)-1].bitIndex++
			}
			continue
		}
		labelIndex := top.bitIndex - top.node
		key = append(key, ss.labels[labelIndex])
		node := labelIndex + 1
		if getBit(ss.leaves, node) {
			result = append(result, string(key))
		}
		stack = append(stack, frame{node, ss.nodeStarts[node]})
	}
	return result
}

func setBit(bitmap *[]uint64, index int, value uint64) {
	for index/64 >= len(*bitmap) {
		*bitmap = append(*bitmap, 0)
	}
	(*bitmap)[index/64] |= value << uint(index%64)
}

func getBit(bitmap []uint64, index int) bool {
	return index/64 < len(bitmap) && bitmap[index/64]&(1<<uint(index%64)) != 0
}
