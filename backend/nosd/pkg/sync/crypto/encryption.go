// Package crypto provides end-to-end encryption for NithronSync.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
)

const (
	// KeySize is the size of encryption keys in bytes (256 bits)
	KeySize = 32

	// NonceSize is the size of nonces for AES-GCM
	NonceSize = 12

	// SaltSize is the size of salts for key derivation
	SaltSize = 32

	// ChunkSize is the size of chunks for streaming encryption (1MB)
	ChunkSize = 1024 * 1024

	// KeyDerivationMemory is the memory parameter for Argon2id
	KeyDerivationMemory = 64 * 1024

	// KeyDerivationTime is the time parameter for Argon2id
	KeyDerivationTime = 3

	// KeyDerivationParallelism is the parallelism parameter for Argon2id
	KeyDerivationParallelism = 4
)

// EncryptionAlgorithm represents the encryption algorithm used
type EncryptionAlgorithm string

const (
	AlgorithmAESGCM         EncryptionAlgorithm = "aes-256-gcm"
	AlgorithmChaCha20Poly   EncryptionAlgorithm = "chacha20-poly1305"
	AlgorithmXChaCha20Poly  EncryptionAlgorithm = "xchacha20-poly1305"
)

// KeyType represents the type of encryption key
type KeyType string

const (
	KeyTypeMaster     KeyType = "master"
	KeyTypeFile       KeyType = "file"
	KeyTypeShare      KeyType = "share"
	KeyTypeDevice     KeyType = "device"
	KeyTypeRecovery   KeyType = "recovery"
)

// EncryptedKey represents an encrypted key
type EncryptedKey struct {
	ID           string              `json:"id"`
	Type         KeyType             `json:"type"`
	Algorithm    EncryptionAlgorithm `json:"algorithm"`
	EncryptedData []byte             `json:"encrypted_data"`
	Nonce        []byte              `json:"nonce"`
	Salt         []byte              `json:"salt,omitempty"`
	CreatedAt    time.Time           `json:"created_at"`
	ExpiresAt    *time.Time          `json:"expires_at,omitempty"`
	Metadata     map[string]string   `json:"metadata,omitempty"`
}

// KeyPair represents an ECDH key pair
type KeyPair struct {
	PrivateKey *ecdh.PrivateKey
	PublicKey  *ecdh.PublicKey
}

// FileEncryptionHeader contains metadata for encrypted files
type FileEncryptionHeader struct {
	Version       int                 `json:"version"`
	Algorithm     EncryptionAlgorithm `json:"algorithm"`
	KeyID         string              `json:"key_id"`
	EncryptedKey  []byte              `json:"encrypted_key"`
	KeyNonce      []byte              `json:"key_nonce"`
	ChunkSize     int                 `json:"chunk_size"`
	OriginalSize  int64               `json:"original_size"`
	OriginalHash  string              `json:"original_hash"`
	CreatedAt     time.Time           `json:"created_at"`
	Metadata      map[string]string   `json:"metadata,omitempty"`
}

// KeyManager manages encryption keys
type KeyManager struct {
	dataDir     string
	masterKey   []byte
	deviceKeys  map[string]*KeyPair
	shareKeys   map[string][]byte
	mu          sync.RWMutex
}

// NewKeyManager creates a new key manager
func NewKeyManager(dataDir string) (*KeyManager, error) {
	km := &KeyManager{
		dataDir:    dataDir,
		deviceKeys: make(map[string]*KeyPair),
		shareKeys:  make(map[string][]byte),
	}

	// Create keys directory
	keysDir := filepath.Join(dataDir, "keys")
	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create keys directory: %w", err)
	}

	return km, nil
}

// InitializeMasterKey initializes or loads the master key
func (km *KeyManager) InitializeMasterKey(password string) error {
	km.mu.Lock()
	defer km.mu.Unlock()

	masterKeyPath := filepath.Join(km.dataDir, "keys", "master.key")

	// Check if master key exists
	if _, err := os.Stat(masterKeyPath); os.IsNotExist(err) {
		// Generate new master key
		salt := make([]byte, SaltSize)
		if _, err := rand.Read(salt); err != nil {
			return fmt.Errorf("failed to generate salt: %w", err)
		}

		// Derive key from password
		km.masterKey = argon2.IDKey(
			[]byte(password),
			salt,
			KeyDerivationTime,
			KeyDerivationMemory,
			KeyDerivationParallelism,
			KeySize,
		)

		// Generate a random master encryption key
		encryptionKey := make([]byte, KeySize)
		if _, err := rand.Read(encryptionKey); err != nil {
			return fmt.Errorf("failed to generate master key: %w", err)
		}

		// Encrypt the master encryption key with the derived key
		encrypted, nonce, err := encryptAESGCM(encryptionKey, km.masterKey)
		if err != nil {
			return fmt.Errorf("failed to encrypt master key: %w", err)
		}

		// Save encrypted master key
		masterKeyData := &EncryptedKey{
			ID:            "master",
			Type:          KeyTypeMaster,
			Algorithm:     AlgorithmAESGCM,
			EncryptedData: encrypted,
			Nonce:         nonce,
			Salt:          salt,
			CreatedAt:     time.Now(),
		}

		data, err := json.Marshal(masterKeyData)
		if err != nil {
			return fmt.Errorf("failed to marshal master key: %w", err)
		}

		if err := os.WriteFile(masterKeyPath, data, 0600); err != nil {
			return fmt.Errorf("failed to write master key: %w", err)
		}

		km.masterKey = encryptionKey
	} else {
		// Load existing master key
		data, err := os.ReadFile(masterKeyPath)
		if err != nil {
			return fmt.Errorf("failed to read master key: %w", err)
		}

		var masterKeyData EncryptedKey
		if err := json.Unmarshal(data, &masterKeyData); err != nil {
			return fmt.Errorf("failed to parse master key: %w", err)
		}

		// Derive key from password
		derivedKey := argon2.IDKey(
			[]byte(password),
			masterKeyData.Salt,
			KeyDerivationTime,
			KeyDerivationMemory,
			KeyDerivationParallelism,
			KeySize,
		)

		// Decrypt master key
		km.masterKey, err = decryptAESGCM(masterKeyData.EncryptedData, masterKeyData.Nonce, derivedKey)
		if err != nil {
			return fmt.Errorf("failed to decrypt master key (wrong password?): %w", err)
		}
	}

	return nil
}

// GenerateShareKey generates a new encryption key for a share
func (km *KeyManager) GenerateShareKey(shareID string) ([]byte, error) {
	km.mu.Lock()
	defer km.mu.Unlock()

	if km.masterKey == nil {
		return nil, errors.New("master key not initialized")
	}

	// Generate random share key
	shareKey := make([]byte, KeySize)
	if _, err := rand.Read(shareKey); err != nil {
		return nil, fmt.Errorf("failed to generate share key: %w", err)
	}

	// Encrypt share key with master key
	encrypted, nonce, err := encryptAESGCM(shareKey, km.masterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt share key: %w", err)
	}

	// Save encrypted share key
	shareKeyData := &EncryptedKey{
		ID:            shareID,
		Type:          KeyTypeShare,
		Algorithm:     AlgorithmAESGCM,
		EncryptedData: encrypted,
		Nonce:         nonce,
		CreatedAt:     time.Now(),
		Metadata: map[string]string{
			"share_id": shareID,
		},
	}

	shareKeyPath := filepath.Join(km.dataDir, "keys", fmt.Sprintf("share_%s.key", shareID))
	data, err := json.Marshal(shareKeyData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal share key: %w", err)
	}

	if err := os.WriteFile(shareKeyPath, data, 0600); err != nil {
		return nil, fmt.Errorf("failed to write share key: %w", err)
	}

	km.shareKeys[shareID] = shareKey
	return shareKey, nil
}

// GetShareKey retrieves the encryption key for a share
func (km *KeyManager) GetShareKey(shareID string) ([]byte, error) {
	km.mu.Lock()
	defer km.mu.Unlock()

	// Check cache
	if key, ok := km.shareKeys[shareID]; ok {
		return key, nil
	}

	if km.masterKey == nil {
		return nil, errors.New("master key not initialized")
	}

	// Load from disk
	shareKeyPath := filepath.Join(km.dataDir, "keys", fmt.Sprintf("share_%s.key", shareID))
	data, err := os.ReadFile(shareKeyPath)
	if err != nil {
		return nil, fmt.Errorf("share key not found: %w", err)
	}

	var shareKeyData EncryptedKey
	if err := json.Unmarshal(data, &shareKeyData); err != nil {
		return nil, fmt.Errorf("failed to parse share key: %w", err)
	}

	// Decrypt share key
	shareKey, err := decryptAESGCM(shareKeyData.EncryptedData, shareKeyData.Nonce, km.masterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt share key: %w", err)
	}

	km.shareKeys[shareID] = shareKey
	return shareKey, nil
}

// GenerateDeviceKeyPair generates a new ECDH key pair for a device
func (km *KeyManager) GenerateDeviceKeyPair(deviceID string) (*KeyPair, error) {
	km.mu.Lock()
	defer km.mu.Unlock()

	// Generate ECDH key pair using X25519
	curve := ecdh.X25519()
	privateKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	keyPair := &KeyPair{
		PrivateKey: privateKey,
		PublicKey:  privateKey.PublicKey(),
	}

	// Store encrypted private key
	if km.masterKey != nil {
		encrypted, nonce, err := encryptAESGCM(privateKey.Bytes(), km.masterKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt device key: %w", err)
		}

		deviceKeyData := &EncryptedKey{
			ID:            deviceID,
			Type:          KeyTypeDevice,
			Algorithm:     AlgorithmAESGCM,
			EncryptedData: encrypted,
			Nonce:         nonce,
			CreatedAt:     time.Now(),
			Metadata: map[string]string{
				"device_id":  deviceID,
				"public_key": base64.StdEncoding.EncodeToString(privateKey.PublicKey().Bytes()),
			},
		}

		deviceKeyPath := filepath.Join(km.dataDir, "keys", fmt.Sprintf("device_%s.key", deviceID))
		data, err := json.Marshal(deviceKeyData)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal device key: %w", err)
		}

		if err := os.WriteFile(deviceKeyPath, data, 0600); err != nil {
			return nil, fmt.Errorf("failed to write device key: %w", err)
		}
	}

	km.deviceKeys[deviceID] = keyPair
	return keyPair, nil
}

// GetPublicKeyForSharing returns a public key that can be shared with others
func (km *KeyManager) GetPublicKeyForSharing(deviceID string) (string, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	if keyPair, ok := km.deviceKeys[deviceID]; ok {
		return base64.StdEncoding.EncodeToString(keyPair.PublicKey.Bytes()), nil
	}

	return "", errors.New("device key not found")
}

// GenerateRecoveryKey generates a recovery key for account recovery
func (km *KeyManager) GenerateRecoveryKey() (string, error) {
	km.mu.Lock()
	defer km.mu.Unlock()

	if km.masterKey == nil {
		return "", errors.New("master key not initialized")
	}

	// Generate recovery key (random bytes encoded as words or hex)
	recoveryBytes := make([]byte, 32)
	if _, err := rand.Read(recoveryBytes); err != nil {
		return "", fmt.Errorf("failed to generate recovery key: %w", err)
	}

	// Encrypt master key with recovery key
	recoveryDerivedKey := argon2.IDKey(
		recoveryBytes,
		[]byte("nithron-sync-recovery"),
		KeyDerivationTime,
		KeyDerivationMemory,
		KeyDerivationParallelism,
		KeySize,
	)

	encrypted, nonce, err := encryptAESGCM(km.masterKey, recoveryDerivedKey)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt with recovery key: %w", err)
	}

	// Save encrypted recovery data
	recoveryData := &EncryptedKey{
		ID:            "recovery",
		Type:          KeyTypeRecovery,
		Algorithm:     AlgorithmAESGCM,
		EncryptedData: encrypted,
		Nonce:         nonce,
		CreatedAt:     time.Now(),
	}

	recoveryPath := filepath.Join(km.dataDir, "keys", "recovery.key")
	data, err := json.Marshal(recoveryData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal recovery data: %w", err)
	}

	if err := os.WriteFile(recoveryPath, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write recovery data: %w", err)
	}

	// Return recovery key as hex string
	return hex.EncodeToString(recoveryBytes), nil
}

// RecoverWithKey recovers the master key using a recovery key
func (km *KeyManager) RecoverWithKey(recoveryKeyHex string) error {
	km.mu.Lock()
	defer km.mu.Unlock()

	recoveryBytes, err := hex.DecodeString(recoveryKeyHex)
	if err != nil {
		return fmt.Errorf("invalid recovery key format: %w", err)
	}

	// Load recovery data
	recoveryPath := filepath.Join(km.dataDir, "keys", "recovery.key")
	data, err := os.ReadFile(recoveryPath)
	if err != nil {
		return fmt.Errorf("recovery data not found: %w", err)
	}

	var recoveryData EncryptedKey
	if err := json.Unmarshal(data, &recoveryData); err != nil {
		return fmt.Errorf("failed to parse recovery data: %w", err)
	}

	// Derive key from recovery bytes
	recoveryDerivedKey := argon2.IDKey(
		recoveryBytes,
		[]byte("nithron-sync-recovery"),
		KeyDerivationTime,
		KeyDerivationMemory,
		KeyDerivationParallelism,
		KeySize,
	)

	// Decrypt master key
	km.masterKey, err = decryptAESGCM(recoveryData.EncryptedData, recoveryData.Nonce, recoveryDerivedKey)
	if err != nil {
		return fmt.Errorf("failed to decrypt master key: %w", err)
	}

	return nil
}

// ChangePassword changes the master key password
func (km *KeyManager) ChangePassword(oldPassword, newPassword string) error {
	km.mu.Lock()
	defer km.mu.Unlock()

	masterKeyPath := filepath.Join(km.dataDir, "keys", "master.key")

	// Load and verify old password
	data, err := os.ReadFile(masterKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read master key: %w", err)
	}

	var masterKeyData EncryptedKey
	if err := json.Unmarshal(data, &masterKeyData); err != nil {
		return fmt.Errorf("failed to parse master key: %w", err)
	}

	// Verify old password
	oldDerivedKey := argon2.IDKey(
		[]byte(oldPassword),
		masterKeyData.Salt,
		KeyDerivationTime,
		KeyDerivationMemory,
		KeyDerivationParallelism,
		KeySize,
	)

	_, err = decryptAESGCM(masterKeyData.EncryptedData, masterKeyData.Nonce, oldDerivedKey)
	if err != nil {
		return errors.New("incorrect old password")
	}

	// Generate new salt and derive new key
	newSalt := make([]byte, SaltSize)
	if _, err := rand.Read(newSalt); err != nil {
		return fmt.Errorf("failed to generate new salt: %w", err)
	}

	newDerivedKey := argon2.IDKey(
		[]byte(newPassword),
		newSalt,
		KeyDerivationTime,
		KeyDerivationMemory,
		KeyDerivationParallelism,
		KeySize,
	)

	// Re-encrypt master key with new password
	encrypted, nonce, err := encryptAESGCM(km.masterKey, newDerivedKey)
	if err != nil {
		return fmt.Errorf("failed to re-encrypt master key: %w", err)
	}

	// Update and save
	masterKeyData.EncryptedData = encrypted
	masterKeyData.Nonce = nonce
	masterKeyData.Salt = newSalt

	newData, err := json.Marshal(masterKeyData)
	if err != nil {
		return fmt.Errorf("failed to marshal master key: %w", err)
	}

	if err := os.WriteFile(masterKeyPath, newData, 0600); err != nil {
		return fmt.Errorf("failed to write master key: %w", err)
	}

	return nil
}

// FileEncryptor handles file encryption and decryption
type FileEncryptor struct {
	keyManager *KeyManager
	algorithm  EncryptionAlgorithm
}

// NewFileEncryptor creates a new file encryptor
func NewFileEncryptor(km *KeyManager, algorithm EncryptionAlgorithm) *FileEncryptor {
	if algorithm == "" {
		algorithm = AlgorithmXChaCha20Poly
	}
	return &FileEncryptor{
		keyManager: km,
		algorithm:  algorithm,
	}
}

// EncryptFile encrypts a file and returns the encrypted data
func (fe *FileEncryptor) EncryptFile(shareID string, plaintext []byte, metadata map[string]string) ([]byte, error) {
	// Get share key
	shareKey, err := fe.keyManager.GetShareKey(shareID)
	if err != nil {
		return nil, fmt.Errorf("failed to get share key: %w", err)
	}

	// Generate file-specific key
	fileKey := make([]byte, KeySize)
	if _, err := rand.Read(fileKey); err != nil {
		return nil, fmt.Errorf("failed to generate file key: %w", err)
	}

	// Encrypt file key with share key
	encryptedFileKey, fileKeyNonce, err := encryptAESGCM(fileKey, shareKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt file key: %w", err)
	}

	// Calculate original hash
	hash := sha256.Sum256(plaintext)

	// Create header
	header := FileEncryptionHeader{
		Version:       1,
		Algorithm:     fe.algorithm,
		KeyID:         shareID,
		EncryptedKey:  encryptedFileKey,
		KeyNonce:      fileKeyNonce,
		ChunkSize:     ChunkSize,
		OriginalSize:  int64(len(plaintext)),
		OriginalHash:  hex.EncodeToString(hash[:]),
		CreatedAt:     time.Now(),
		Metadata:      metadata,
	}

	headerBytes, err := json.Marshal(header)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal header: %w", err)
	}

	// Encrypt file content based on algorithm
	var ciphertext []byte
	switch fe.algorithm {
	case AlgorithmXChaCha20Poly:
		ciphertext, err = fe.encryptXChaCha20Poly1305(plaintext, fileKey)
	case AlgorithmChaCha20Poly:
		ciphertext, err = fe.encryptChaCha20Poly1305(plaintext, fileKey)
	case AlgorithmAESGCM:
		ciphertext, _, err = encryptAESGCM(plaintext, fileKey)
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", fe.algorithm)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to encrypt file content: %w", err)
	}

	// Combine header length + header + ciphertext
	headerLen := len(headerBytes)
	result := make([]byte, 4+headerLen+len(ciphertext))
	result[0] = byte(headerLen >> 24)
	result[1] = byte(headerLen >> 16)
	result[2] = byte(headerLen >> 8)
	result[3] = byte(headerLen)
	copy(result[4:4+headerLen], headerBytes)
	copy(result[4+headerLen:], ciphertext)

	return result, nil
}

// DecryptFile decrypts an encrypted file
func (fe *FileEncryptor) DecryptFile(shareID string, ciphertext []byte) ([]byte, *FileEncryptionHeader, error) {
	if len(ciphertext) < 4 {
		return nil, nil, errors.New("invalid encrypted file: too short")
	}

	// Read header length
	headerLen := int(ciphertext[0])<<24 | int(ciphertext[1])<<16 | int(ciphertext[2])<<8 | int(ciphertext[3])

	if len(ciphertext) < 4+headerLen {
		return nil, nil, errors.New("invalid encrypted file: header truncated")
	}

	// Parse header
	var header FileEncryptionHeader
	if err := json.Unmarshal(ciphertext[4:4+headerLen], &header); err != nil {
		return nil, nil, fmt.Errorf("failed to parse header: %w", err)
	}

	// Get share key
	shareKey, err := fe.keyManager.GetShareKey(shareID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get share key: %w", err)
	}

	// Decrypt file key
	fileKey, err := decryptAESGCM(header.EncryptedKey, header.KeyNonce, shareKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decrypt file key: %w", err)
	}

	// Decrypt file content
	encryptedContent := ciphertext[4+headerLen:]
	var plaintext []byte

	switch header.Algorithm {
	case AlgorithmXChaCha20Poly:
		plaintext, err = fe.decryptXChaCha20Poly1305(encryptedContent, fileKey)
	case AlgorithmChaCha20Poly:
		plaintext, err = fe.decryptChaCha20Poly1305(encryptedContent, fileKey)
	case AlgorithmAESGCM:
		// Extract nonce from ciphertext
		if len(encryptedContent) < NonceSize {
			return nil, nil, errors.New("invalid encrypted content")
		}
		nonce := encryptedContent[:NonceSize]
		plaintext, err = decryptAESGCM(encryptedContent[NonceSize:], nonce, fileKey)
	default:
		return nil, nil, fmt.Errorf("unsupported algorithm: %s", header.Algorithm)
	}

	if err != nil {
		return nil, nil, fmt.Errorf("failed to decrypt file content: %w", err)
	}

	// Verify hash
	hash := sha256.Sum256(plaintext)
	if hex.EncodeToString(hash[:]) != header.OriginalHash {
		return nil, nil, errors.New("file integrity check failed")
	}

	return plaintext, &header, nil
}

// EncryptStream encrypts data from a reader and writes to a writer
func (fe *FileEncryptor) EncryptStream(shareID string, reader io.Reader, writer io.Writer, metadata map[string]string) error {
	// Get share key
	shareKey, err := fe.keyManager.GetShareKey(shareID)
	if err != nil {
		return fmt.Errorf("failed to get share key: %w", err)
	}

	// Generate file-specific key
	fileKey := make([]byte, KeySize)
	if _, err := rand.Read(fileKey); err != nil {
		return fmt.Errorf("failed to generate file key: %w", err)
	}

	// Encrypt file key with share key
	encryptedFileKey, fileKeyNonce, err := encryptAESGCM(fileKey, shareKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt file key: %w", err)
	}

	// Create AEAD cipher
	aead, err := chacha20poly1305.NewX(fileKey)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	// Write placeholder header (will update at end)
	header := FileEncryptionHeader{
		Version:      1,
		Algorithm:    AlgorithmXChaCha20Poly,
		KeyID:        shareID,
		EncryptedKey: encryptedFileKey,
		KeyNonce:     fileKeyNonce,
		ChunkSize:    ChunkSize,
		CreatedAt:    time.Now(),
		Metadata:     metadata,
	}

	headerBytes, _ := json.Marshal(header)
	headerLen := len(headerBytes)

	// Write header length placeholder
	headerLenBytes := make([]byte, 4)
	headerLenBytes[0] = byte(headerLen >> 24)
	headerLenBytes[1] = byte(headerLen >> 16)
	headerLenBytes[2] = byte(headerLen >> 8)
	headerLenBytes[3] = byte(headerLen)

	if _, err := writer.Write(headerLenBytes); err != nil {
		return fmt.Errorf("failed to write header length: %w", err)
	}
	if _, err := writer.Write(headerBytes); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Encrypt chunks
	hasher := sha256.New()
	totalSize := int64(0)
	chunkNum := uint64(0)
	buf := make([]byte, ChunkSize)

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			hasher.Write(buf[:n])
			totalSize += int64(n)

			// Generate nonce for this chunk
			nonce := make([]byte, chacha20poly1305.NonceSizeX)
			nonce[0] = byte(chunkNum >> 56)
			nonce[1] = byte(chunkNum >> 48)
			nonce[2] = byte(chunkNum >> 40)
			nonce[3] = byte(chunkNum >> 32)
			nonce[4] = byte(chunkNum >> 24)
			nonce[5] = byte(chunkNum >> 16)
			nonce[6] = byte(chunkNum >> 8)
			nonce[7] = byte(chunkNum)

			// Encrypt chunk
			ciphertext := aead.Seal(nil, nonce, buf[:n], nil)

			// Write chunk length + nonce + ciphertext
			chunkLen := len(nonce) + len(ciphertext)
			chunkLenBytes := make([]byte, 4)
			chunkLenBytes[0] = byte(chunkLen >> 24)
			chunkLenBytes[1] = byte(chunkLen >> 16)
			chunkLenBytes[2] = byte(chunkLen >> 8)
			chunkLenBytes[3] = byte(chunkLen)

			if _, err := writer.Write(chunkLenBytes); err != nil {
				return fmt.Errorf("failed to write chunk length: %w", err)
			}
			if _, err := writer.Write(nonce); err != nil {
				return fmt.Errorf("failed to write nonce: %w", err)
			}
			if _, err := writer.Write(ciphertext); err != nil {
				return fmt.Errorf("failed to write ciphertext: %w", err)
			}

			chunkNum++
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
	}

	// Write end marker
	endMarker := make([]byte, 4)
	if _, err := writer.Write(endMarker); err != nil {
		return fmt.Errorf("failed to write end marker: %w", err)
	}

	return nil
}

func (fe *FileEncryptor) encryptXChaCha20Poly1305(plaintext, key []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	ciphertext := aead.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

func (fe *FileEncryptor) decryptXChaCha20Poly1305(ciphertext, key []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < chacha20poly1305.NonceSizeX {
		return nil, errors.New("ciphertext too short")
	}

	nonce := ciphertext[:chacha20poly1305.NonceSizeX]
	encryptedData := ciphertext[chacha20poly1305.NonceSizeX:]

	return aead.Open(nil, nonce, encryptedData, nil)
}

func (fe *FileEncryptor) encryptChaCha20Poly1305(plaintext, key []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, chacha20poly1305.NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	ciphertext := aead.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

func (fe *FileEncryptor) decryptChaCha20Poly1305(ciphertext, key []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < chacha20poly1305.NonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce := ciphertext[:chacha20poly1305.NonceSize]
	encryptedData := ciphertext[chacha20poly1305.NonceSize:]

	return aead.Open(nil, nonce, encryptedData, nil)
}

// Helper functions for AES-GCM encryption
func encryptAESGCM(plaintext, key []byte) (ciphertext, nonce []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	nonce = make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}

	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

func decryptAESGCM(ciphertext, nonce, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return gcm.Open(nil, nonce, ciphertext, nil)
}

// DeriveKey derives a key using HKDF
func DeriveKey(secret, salt, info []byte, keyLen int) ([]byte, error) {
	reader := hkdf.New(sha256.New, secret, salt, info)
	key := make([]byte, keyLen)
	if _, err := io.ReadFull(reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

// SharedSecretFromECDH computes a shared secret from ECDH key exchange
func SharedSecretFromECDH(privateKey *ecdh.PrivateKey, peerPublicKeyBytes []byte) ([]byte, error) {
	curve := ecdh.X25519()
	peerPublicKey, err := curve.NewPublicKey(peerPublicKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("invalid peer public key: %w", err)
	}

	return privateKey.ECDH(peerPublicKey)
}

