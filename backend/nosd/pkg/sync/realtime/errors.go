package realtime

import "errors"

var (
	// ErrClientNotFound indicates the client was not found
	ErrClientNotFound = errors.New("client not found")

	// ErrFileLocked indicates the file is already locked by another user
	ErrFileLocked = errors.New("file is locked by another user")

	// ErrNotLockOwner indicates the user is not the lock owner
	ErrNotLockOwner = errors.New("not the lock owner")

	// ErrSendBufferFull indicates the client's send buffer is full
	ErrSendBufferFull = errors.New("send buffer full")

	// ErrInvalidMessage indicates an invalid message format
	ErrInvalidMessage = errors.New("invalid message format")

	// ErrUnauthorized indicates the client is not authorized
	ErrUnauthorized = errors.New("unauthorized")

	// ErrChannelNotFound indicates the channel was not found
	ErrChannelNotFound = errors.New("channel not found")

	// ErrAlreadySubscribed indicates the client is already subscribed
	ErrAlreadySubscribed = errors.New("already subscribed to channel")

	// ErrNotSubscribed indicates the client is not subscribed
	ErrNotSubscribed = errors.New("not subscribed to channel")

	// ErrSessionExpired indicates the session has expired
	ErrSessionExpired = errors.New("session expired")

	// ErrRateLimited indicates the client is rate limited
	ErrRateLimited = errors.New("rate limited")
)

