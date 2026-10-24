// Package wire holds pure byte-level decoders for the decompressed .rec buffer.
// It carries no game semantics and must not import the dissect package.
package wire

import (
	"encoding/binary"
	"encoding/hex"
)

// Handle is an object or entity handle in file byte order.
type Handle [8]byte

func (h Handle) String() string { return hex.EncodeToString(h[:]) }

// IsNull reports whether every byte of the handle is zero.
func (h Handle) IsNull() bool { return h == Handle{} }

// FieldID is a property field id in file byte order.
type FieldID [4]byte

func (f FieldID) String() string { return hex.EncodeToString(f[:]) }

// ClassID is an object class id in file byte order.
type ClassID [4]byte

func (c ClassID) String() string { return hex.EncodeToString(c[:]) }

// handleAt copies b[p:p+8]; the caller guarantees bounds.
func handleAt(b []byte, p int) Handle {
	var h Handle
	copy(h[:], b[p:p+8])
	return h
}

// HandleOf converts a little-endian uint64 handle (as dissect.readHandle returns) into file byte order.
func HandleOf(v uint64) Handle {
	var h Handle
	binary.LittleEndian.PutUint64(h[:], v)
	return h
}
