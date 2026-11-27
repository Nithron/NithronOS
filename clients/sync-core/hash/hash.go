// Package hash provides file hashing utilities for sync.
package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"hash/adler32"
	"io"
	"os"
)

// DefaultBlockSize is the default block size for delta sync (4MB).
const DefaultBlockSize = 4 * 1024 * 1024

// FileHash computes the SHA-256 hash of a file.
func FileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	return ReaderHash(f)
}

// ReaderHash computes the SHA-256 hash of a reader.
func ReaderHash(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// BytesHash computes the SHA-256 hash of bytes.
func BytesHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// BlockHashes computes block hashes for a file (for delta sync).
type BlockHashes struct {
	BlockSize int
	FileSize  int64
	Blocks    []BlockHash
}

// BlockHash contains the hashes for a single block.
type BlockHash struct {
	Index      int
	Offset     int64
	Size       int
	StrongHash string // SHA-256
	WeakHash   uint32 // Adler-32
}

// ComputeBlockHashes computes block hashes for a file.
func ComputeBlockHashes(path string, blockSize int) (*BlockHashes, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	return ComputeBlockHashesFromReader(f, stat.Size(), blockSize)
}

// ComputeBlockHashesFromReader computes block hashes from a reader.
func ComputeBlockHashesFromReader(r io.Reader, fileSize int64, blockSize int) (*BlockHashes, error) {
	if blockSize <= 0 {
		blockSize = DefaultBlockSize
	}

	result := &BlockHashes{
		BlockSize: blockSize,
		FileSize:  fileSize,
	}

	buf := make([]byte, blockSize)
	offset := int64(0)
	index := 0

	for {
		n, err := io.ReadFull(r, buf)
		if n > 0 {
			block := buf[:n]

			bh := BlockHash{
				Index:      index,
				Offset:     offset,
				Size:       n,
				StrongHash: BytesHash(block),
				WeakHash:   adler32.Checksum(block),
			}
			result.Blocks = append(result.Blocks, bh)

			offset += int64(n)
			index++
		}

		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

// RollingHash implements the rolling checksum for delta sync.
type RollingHash struct {
	window    []byte
	windowLen int
	pos       int
	full      bool
	a, b      uint32
}

// NewRollingHash creates a new rolling hash with the given window size.
func NewRollingHash(windowSize int) *RollingHash {
	return &RollingHash{
		window:    make([]byte, windowSize),
		windowLen: windowSize,
	}
}

// Write adds a byte to the rolling hash.
func (r *RollingHash) Write(c byte) uint32 {
	if r.full {
		old := uint32(r.window[r.pos])
		r.a = (r.a - old + uint32(c)) % 65521
		r.b = (r.b - uint32(r.windowLen)*old + r.a) % 65521
	} else {
		r.a = (r.a + uint32(c)) % 65521
		r.b = (r.b + r.a) % 65521
	}

	r.window[r.pos] = c
	r.pos = (r.pos + 1) % r.windowLen

	if r.pos == 0 {
		r.full = true
	}

	return r.Sum32()
}

// Sum32 returns the current checksum.
func (r *RollingHash) Sum32() uint32 {
	return (r.b << 16) | r.a
}

// Reset resets the rolling hash.
func (r *RollingHash) Reset() {
	r.a = 0
	r.b = 0
	r.pos = 0
	r.full = false
}

// DeltaPlan represents a plan for delta synchronization.
type DeltaPlan struct {
	Operations []DeltaOp
	SavedBytes int64
	TotalBytes int64
}

// DeltaOp represents a single delta operation.
type DeltaOp struct {
	Type       string // "copy" or "data"
	Offset     int64  // Source offset for copy, or data offset
	Size       int    // Size of the operation
	BlockIndex int    // For copy: index of matching block
	Data       []byte // For data: the actual data to transfer
}

// ComputeDeltaPlan computes the delta between local and remote block hashes.
func ComputeDeltaPlan(localPath string, remoteBlocks *BlockHashes, blockSize int) (*DeltaPlan, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	return ComputeDeltaPlanFromReader(f, stat.Size(), remoteBlocks, blockSize)
}

// ComputeDeltaPlanFromReader computes delta from a reader.
func ComputeDeltaPlanFromReader(r io.Reader, localSize int64, remoteBlocks *BlockHashes, blockSize int) (*DeltaPlan, error) {
	if blockSize <= 0 {
		blockSize = DefaultBlockSize
	}

	// Build a map of weak hash -> block indices
	weakMap := make(map[uint32][]int)
	for i, b := range remoteBlocks.Blocks {
		weakMap[b.WeakHash] = append(weakMap[b.WeakHash], i)
	}

	plan := &DeltaPlan{
		TotalBytes: localSize,
	}

	// Read the entire file (for simplicity - production would stream)
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return plan, nil
	}

	// Find matching blocks. The weak hash rolls one byte at a time (Adler-32,
	// the same value adler32.Checksum gives the server); the strong hash is
	// computed only when the weak one matches. (It was recomputed from
	// scratch at every offset, and each remote block could be reused only
	// once, so repeated content was sent again.)
	var pendingData []byte
	pos := 0
	var win adlerWindow
	haveWin := false
	if len(data) >= blockSize {
		win = newAdlerWindow(data[:blockSize])
		haveWin = true
	}

	// The remote file's last block is usually shorter than blockSize and
	// can only match at one place: where it would end the new file.
	var tail *BlockHash
	if n := len(remoteBlocks.Blocks); n > 0 && remoteBlocks.Blocks[n-1].Size < blockSize {
		tail = &remoteBlocks.Blocks[n-1]
	}

	for pos < len(data) {
		if tail != nil && len(data)-pos == tail.Size && BytesHash(data[pos:]) == tail.StrongHash {
			if len(pendingData) > 0 {
				plan.Operations = append(plan.Operations, DeltaOp{Type: "data", Size: len(pendingData), Data: pendingData})
				pendingData = nil
			}
			plan.Operations = append(plan.Operations, DeltaOp{Type: "copy", Offset: tail.Offset, Size: tail.Size, BlockIndex: tail.Index})
			plan.SavedBytes += int64(tail.Size)
			break
		}
		if haveWin {
			if indices, ok := weakMap[win.sum()]; ok {
				blockData := data[pos : pos+blockSize]
				strongHash := BytesHash(blockData)
				matched := -1
				for _, idx := range indices {
					if remoteBlocks.Blocks[idx].StrongHash == strongHash {
						matched = idx
						break
					}
				}
				if matched >= 0 {
					if len(pendingData) > 0 {
						plan.Operations = append(plan.Operations, DeltaOp{Type: "data", Size: len(pendingData), Data: pendingData})
						pendingData = nil
					}
					plan.Operations = append(plan.Operations, DeltaOp{
						Type:       "copy",
						Offset:     remoteBlocks.Blocks[matched].Offset,
						Size:       blockSize,
						BlockIndex: matched,
					})
					plan.SavedBytes += int64(blockSize)
					pos += blockSize
					haveWin = pos+blockSize <= len(data)
					if haveWin {
						win = newAdlerWindow(data[pos : pos+blockSize])
					}
					continue
				}
			}
		}

		// No match here: this byte is new data; the window moves one byte.
		pendingData = append(pendingData, data[pos])
		if haveWin && pos+blockSize < len(data) {
			win.roll(data[pos], data[pos+blockSize])
		} else {
			haveWin = false
		}
		pos++
	}

	// Emit any remaining pending data
	if len(pendingData) > 0 {
		plan.Operations = append(plan.Operations, DeltaOp{
			Type: "data",
			Size: len(pendingData),
			Data: pendingData,
		})
	}

	return plan, nil
}

// QuickHash computes a quick hash for comparison (first + last 64KB + size).
func QuickHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return "", err
	}

	size := stat.Size()
	if size == 0 {
		return fmt.Sprintf("empty:%d", stat.ModTime().UnixNano()), nil
	}

	h := sha256.New()

	// Read first 64KB
	// A failed read is an error, not a hash of whatever was read (that
	// could make a changed file look unchanged).
	buf := make([]byte, 64*1024)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "", err
	}
	h.Write(buf[:n])

	// Read last 64KB if file is large enough
	if size > 128*1024 {
		n, err = f.ReadAt(buf, size-64*1024)
		if err != nil && err != io.EOF {
			return "", err
		}
		h.Write(buf[:n])
	}

	// Include size in hash
	fmt.Fprintf(h, ":%d", size)

	return hex.EncodeToString(h.Sum(nil)), nil
}

// IncrementalHasher provides incremental hashing.
type IncrementalHasher struct {
	hash hash.Hash
}

// NewIncrementalHasher creates a new incremental hasher.
func NewIncrementalHasher() *IncrementalHasher {
	return &IncrementalHasher{
		hash: sha256.New(),
	}
}

// Write adds data to the hash.
func (h *IncrementalHasher) Write(data []byte) (int, error) {
	return h.hash.Write(data)
}

// Sum returns the current hash value.
func (h *IncrementalHasher) Sum() string {
	return hex.EncodeToString(h.hash.Sum(nil))
}

// Reset resets the hasher.
func (h *IncrementalHasher) Reset() {
	h.hash.Reset()
}

// adlerWindow is Adler-32 over a fixed-size window that can move one byte
// at a time; sum() equals adler32.Checksum of the window's bytes.
type adlerWindow struct {
	a, b uint32
	n    int64
}

const adlerMod = 65521

func newAdlerWindow(block []byte) adlerWindow {
	s := adler32.Checksum(block)
	return adlerWindow{a: s & 0xffff, b: s >> 16, n: int64(len(block))}
}

// roll drops out (the window's first byte) and appends in.
func (w *adlerWindow) roll(out, in byte) {
	a := (int64(w.a) - int64(out) + int64(in)) % adlerMod
	if a < 0 {
		a += adlerMod
	}
	b := (int64(w.b) - w.n*int64(out) + a - 1) % adlerMod
	if b < 0 {
		b += adlerMod
	}
	w.a, w.b = uint32(a), uint32(b)
}

func (w adlerWindow) sum() uint32 { return w.b<<16 | w.a }
