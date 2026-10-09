// Modified from sing, Copyright (C) 2022 by nekohasekai <contact-sagernet@sekai.icu>, GPL-3.0-or-later.
// https://github.com/SagerNet/sing/blob/6f21f2425a959912c37d2ef43d61e2a663315dea/common/varbin

package singvarbin

import (
	"encoding/binary"
	"fmt"
	"io"
	"unsafe"
)

type Reader interface {
	io.Reader
	io.ByteReader
}

type dummyReader struct {
	io.Reader
}

func (r *dummyReader) ReadByte() (byte, error) {
	var bb [1]byte
	_, err := io.ReadFull(r.Reader, bb[:1])
	if err != nil {
		return 0, err
	}
	return bb[0], nil
}

type Writer interface {
	io.Writer
	io.ByteWriter
}

type dummyWriter struct {
	io.Writer
}

func (w *dummyWriter) WriteByte(b byte) error {
	_, err := w.Write([]byte{b})
	return err
}

func NewReader(r io.Reader) Reader {
	if rr, ok := r.(Reader); ok {
		return rr
	}
	return &dummyReader{r}
}

func NewWriter(w io.Writer) Writer {
	if ww, ok := w.(Writer); ok {
		return ww
	}
	return &dummyWriter{w}
}

func WriteUvarint(w io.Writer, value uint64) error {
	var data [binary.MaxVarintLen64]byte
	length := binary.PutUvarint(data[:], value)
	// io.Writer returns a non-nil error if it writes less than len(p).
	_, err := w.Write(data[:length])
	return err
}

func ReadSlice[T ~uint8 | ~uint16 | ~uint64](r Reader) ([]T, error) {
	length, err := binary.ReadUvarint(r)
	if err != nil {
		return nil, err
	}
	if length > uint64(int(^uint(0)>>1))/uint64(binary.Size(*new(T))) {
		return nil, fmt.Errorf("invalid slice length: %d", length)
	}
	result := make([]T, 0, min(length, 1024))
	// Grow as data arrives, so a corrupt length cannot cause a large allocation.
	for length > 0 {
		size := int(min(length, 1024))
		start := len(result)
		result = append(result, make([]T, size)...)
		err = binary.Read(r, binary.BigEndian, result[start:])
		if err != nil {
			return nil, err
		}
		length -= uint64(size)
	}
	return result, nil
}

func WriteSlice[T ~uint8 | ~uint16 | ~uint64](w Writer, value []T) error {
	err := WriteUvarint(w, uint64(len(value)))
	if err != nil {
		return err
	}
	return binary.Write(w, binary.BigEndian, value)
}

// WriteString writes value without copying it into a []byte,
// io.Writer must not modify the slice passed to Write.
func WriteString(w Writer, value string) error {
	err := WriteUvarint(w, uint64(len(value)))
	if err != nil {
		return err
	}
	_, err = w.Write(unsafe.Slice(unsafe.StringData(value), len(value)))
	return err
}
