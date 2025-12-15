package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash/adler32"
	"io"
	"os"
)

// DeltaSync provides delta synchronization functionality using rolling checksums
type DeltaSync struct {
	blockSize int64
}

// NewDeltaSync creates a new delta sync handler
func NewDeltaSync(blockSize int64) *DeltaSync {
	if blockSize <= 0 {
		blockSize = DefaultBlockSize
	}
	if blockSize < MinBlockSize {
		blockSize = MinBlockSize
	}
	if blockSize > MaxBlockSize {
		blockSize = MaxBlockSize
	}
	return &DeltaSync{blockSize: blockSize}
}

// ComputeBlockHashes computes block hashes for a file
func (ds *DeltaSync) ComputeBlockHashes(filePath string) (*BlockHashResponse, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()
	
	// Get file size
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	fileSize := info.Size()
	
	// Compute full file hash
	fullHash := sha256.New()
	if _, err := io.Copy(fullHash, f); err != nil {
		return nil, fmt.Errorf("failed to compute file hash: %w", err)
	}
	fileHash := hex.EncodeToString(fullHash.Sum(nil))
	
	// Seek back to start
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek: %w", err)
	}
	
	// Compute block hashes
	var blocks []BlockHash
	buffer := make([]byte, ds.blockSize)
	offset := int64(0)
	
	for {
		n, err := f.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read block: %w", err)
		}
		
		block := buffer[:n]
		
		// Compute strong hash (SHA-256)
		strongHash := sha256.Sum256(block)
		
		// Compute weak hash (Adler-32 for rolling checksum)
		weakHash := adler32.Checksum(block)
		
		blocks = append(blocks, BlockHash{
			Offset: offset,
			Size:   int64(n),
			Hash:   hex.EncodeToString(strongHash[:]),
			Weak:   weakHash,
		})
		
		offset += int64(n)
	}
	
	return &BlockHashResponse{
		Path:      filePath,
		Size:      fileSize,
		Hash:      fileHash,
		BlockSize: ds.blockSize,
		Blocks:    blocks,
	}, nil
}

// ComputeBlockHashesFromPath is a convenience wrapper
func (ds *DeltaSync) ComputeBlockHashesFromPath(sharePath, filePath string) (*BlockHashResponse, error) {
	fullPath := sharePath + "/" + filePath
	response, err := ds.ComputeBlockHashes(fullPath)
	if err != nil {
		return nil, err
	}
	response.Path = filePath // Return relative path
	return response, nil
}

// DiffBlocks computes which blocks need to be transferred
func (ds *DeltaSync) DiffBlocks(localBlocks, remoteBlocks []BlockHash) []BlockHash {
	// Create a map of remote blocks by hash
	remoteMap := make(map[string]BlockHash, len(remoteBlocks))
	for _, b := range remoteBlocks {
		remoteMap[b.Hash] = b
	}
	
	// Find blocks that differ
	var diffBlocks []BlockHash
	for _, local := range localBlocks {
		if _, exists := remoteMap[local.Hash]; !exists {
			diffBlocks = append(diffBlocks, local)
		}
	}
	
	return diffBlocks
}

// ComputeRollingChecksum computes the Adler-32 rolling checksum
func ComputeRollingChecksum(data []byte) uint32 {
	return adler32.Checksum(data)
}

// RollingChecksumUpdate updates a rolling checksum when sliding the window
type RollingChecksumUpdate struct {
	a, b     uint32
	blockLen uint32
}

// NewRollingChecksumUpdate creates a new rolling checksum updater
func NewRollingChecksumUpdate(blockLen int) *RollingChecksumUpdate {
	return &RollingChecksumUpdate{
		a:        1,
		b:        0,
		blockLen: uint32(blockLen),
	}
}

// Init initializes the rolling checksum with a block of data
func (r *RollingChecksumUpdate) Init(data []byte) uint32 {
	r.a = 1
	r.b = 0
	for _, d := range data {
		r.a = (r.a + uint32(d)) % 65521
		r.b = (r.b + r.a) % 65521
	}
	return (r.b << 16) | r.a
}

// Update slides the window by removing oldByte and adding newByte
func (r *RollingChecksumUpdate) Update(oldByte, newByte byte) uint32 {
	r.a = (r.a - uint32(oldByte) + uint32(newByte)) % 65521
	r.b = (r.b - r.blockLen*uint32(oldByte) + r.a - 1) % 65521
	return (r.b << 16) | r.a
}

// Checksum returns the current checksum value
func (r *RollingChecksumUpdate) Checksum() uint32 {
	return (r.b << 16) | r.a
}

// FindMatchingBlocks finds blocks in a file that match remote blocks
func (ds *DeltaSync) FindMatchingBlocks(filePath string, remoteBlocks []BlockHash) ([]int, error) {
	// Build lookup maps
	weakMap := make(map[uint32][]int) // weak hash -> indices in remoteBlocks
	for i, b := range remoteBlocks {
		weakMap[b.Weak] = append(weakMap[b.Weak], i)
	}
	
	strongMap := make(map[string]int) // strong hash -> index
	for i, b := range remoteBlocks {
		strongMap[b.Hash] = i
	}
	
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	
	var matches []int // Indices of matching remote blocks
	matchedBlocks := make(map[int]bool)
	
	buffer := make([]byte, ds.blockSize)
	window := make([]byte, ds.blockSize)
	windowPos := 0
	windowFull := false
	
	roller := NewRollingChecksumUpdate(int(ds.blockSize))
	
	// Read file and find matches
	for {
		n, err := f.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		
		for i := 0; i < n; i++ {
			// Add byte to window
			if windowFull {
				// Slide window
				copy(window, window[1:])
				window[len(window)-1] = buffer[i]
				windowPos++
			} else {
				window[windowPos] = buffer[i]
				windowPos++
				if windowPos == int(ds.blockSize) {
					windowFull = true
					windowPos = 0
				} else {
					continue
				}
			}
			
			// Compute weak hash
			var weak uint32
			if windowPos == 0 && !windowFull {
				weak = roller.Init(window)
			} else {
				// This is simplified - in production, use proper rolling update
				weak = ComputeRollingChecksum(window)
			}
			
			// Check for potential matches
			if candidates, ok := weakMap[weak]; ok {
				// Compute strong hash
				strong := sha256.Sum256(window)
				strongStr := hex.EncodeToString(strong[:])
				
				if idx, ok := strongMap[strongStr]; ok && !matchedBlocks[idx] {
					matches = append(matches, idx)
					matchedBlocks[idx] = true
					// Skip to next block boundary
					if ok && len(candidates) > 0 {
						for _, c := range candidates {
							if !matchedBlocks[c] && remoteBlocks[c].Hash == strongStr {
								matchedBlocks[c] = true
							}
						}
					}
				}
			}
		}
	}
	
	return matches, nil
}

// BlockTransferPlan describes which blocks need to be transferred
type BlockTransferPlan struct {
	FilePath        string       `json:"file_path"`
	FileSize        int64        `json:"file_size"`
	BlockSize       int64        `json:"block_size"`
	TotalBlocks     int          `json:"total_blocks"`
	BlocksToSend    []BlockHash  `json:"blocks_to_send"`
	BlocksToReceive []int        `json:"blocks_to_receive"` // Indices in the target's block list
	BytesToTransfer int64        `json:"bytes_to_transfer"`
	Savings         float64      `json:"savings_percent"`
}

// CreateTransferPlan creates a plan for transferring a file using delta sync
func (ds *DeltaSync) CreateTransferPlan(localHashes, remoteHashes *BlockHashResponse) *BlockTransferPlan {
	// If files are identical, no transfer needed
	if localHashes.Hash == remoteHashes.Hash {
		return &BlockTransferPlan{
			FilePath:        localHashes.Path,
			FileSize:        localHashes.Size,
			BlockSize:       ds.blockSize,
			TotalBlocks:     len(localHashes.Blocks),
			BlocksToSend:    nil,
			BlocksToReceive: nil,
			BytesToTransfer: 0,
			Savings:         100.0,
		}
	}
	
	// Build remote block map
	remoteBlockMap := make(map[string]int) // hash -> index
	for i, b := range remoteHashes.Blocks {
		remoteBlockMap[b.Hash] = i
	}
	
	// Find blocks that need to be sent
	var blocksToSend []BlockHash
	var blocksToReceive []int
	var bytesToTransfer int64
	
	for _, local := range localHashes.Blocks {
		if idx, exists := remoteBlockMap[local.Hash]; exists {
			// Block already exists on remote
			blocksToReceive = append(blocksToReceive, idx)
		} else {
			// Block needs to be sent
			blocksToSend = append(blocksToSend, local)
			bytesToTransfer += local.Size
		}
	}
	
	savings := 0.0
	if localHashes.Size > 0 {
		savings = float64(localHashes.Size-bytesToTransfer) / float64(localHashes.Size) * 100
	}
	
	return &BlockTransferPlan{
		FilePath:        localHashes.Path,
		FileSize:        localHashes.Size,
		BlockSize:       ds.blockSize,
		TotalBlocks:     len(localHashes.Blocks),
		BlocksToSend:    blocksToSend,
		BlocksToReceive: blocksToReceive,
		BytesToTransfer: bytesToTransfer,
		Savings:         savings,
	}
}

// ReadBlock reads a specific block from a file
func (ds *DeltaSync) ReadBlock(filePath string, offset, size int64) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	
	data := make([]byte, size)
	n, err := io.ReadFull(f, data)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return data[:n], nil
	}
	if err != nil {
		return nil, err
	}
	
	return data, nil
}

// WriteBlock writes a block to a specific position in a file
func (ds *DeltaSync) WriteBlock(filePath string, offset int64, data []byte) error {
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	
	_, err = f.Write(data)
	return err
}

