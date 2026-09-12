package engine

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

type RequestType string

const (
	RequestUpload   RequestType = "upload"   // someone wants to SEND a file
	RequestDownload RequestType = "download" // someone wants to PULL the shared file
)

type RequestStatus string

const (
	StatusPending  RequestStatus = "pending"
	StatusAccepted RequestStatus = "accepted"
	StatusRejected RequestStatus = "rejected"
)

type TransferRequest struct {
	ID         string
	Type       RequestType
	FromDevice string // whatever name the requester sent, "Unknown device" if blank
	FileName   string // for uploads: name of the incoming file. for downloads: the shared file's name
	FileSize   int64
	Status     RequestStatus
	Token      string // set once Accepted; the actual /upload or /download call must present this
	CreatedAt  time.Time
}

// ConsentStore tracks pending transfer requests in memory. Not persisted
// across restarts — that's fine, a restarted server has no in-flight
// transfers to remember anyway.
type ConsentStore struct {
	mu       sync.Mutex
	requests map[string]*TransferRequest
}

func NewConsentStore() *ConsentStore {
	return &ConsentStore{requests: make(map[string]*TransferRequest)}
}

// Create registers a new pending request and returns it.
func (cs *ConsentStore) Create(reqType RequestType, fromDevice, fileName string, fileSize int64) (*TransferRequest, error) {
	id, err := randomID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate request id: %w", err)
	}

	req := &TransferRequest{
		ID:         id,
		Type:       reqType,
		FromDevice: fromDevice,
		FileName:   fileName,
		FileSize:   fileSize,
		Status:     StatusPending,
		CreatedAt:  time.Now(),
	}

	cs.mu.Lock()
	cs.requests[id] = req
	cs.mu.Unlock()

	return req, nil
}

// Get returns the request by ID, or nil if it doesn't exist.
func (cs *ConsentStore) Get(id string) *TransferRequest {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.requests[id]
}

const pendingTimeout = 30 * time.Second

// Pending returns all requests still awaiting a response, oldest first.
// This is what the laptop's UI polls to know what to show a popup for.
func (cs *ConsentStore) Pending() []*TransferRequest {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	var pending []*TransferRequest
	for _, req := range cs.requests {
		if req.Status != StatusPending {
			continue
		}
		if time.Since(req.CreatedAt) > pendingTimeout {
			req.Status = StatusRejected // auto-expire
			continue
		}
		pending = append(pending, req)
	}
	return pending
}

// Respond accepts or rejects a pending request. On accept, generates and
// stores a one-time token that the follow-up /upload or /download call
// must present.
func (cs *ConsentStore) Respond(id string, accept bool) (*TransferRequest, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	req, ok := cs.requests[id]
	if !ok {
		return nil, fmt.Errorf("request %s not found", id)
	}
	if req.Status != StatusPending {
		return nil, fmt.Errorf("request %s already %s", id, req.Status)
	}

	if accept {
		token, err := randomID()
		if err != nil {
			return nil, fmt.Errorf("failed to generate token: %w", err)
		}
		req.Token = token
		req.Status = StatusAccepted
	} else {
		req.Status = StatusRejected
	}

	return req, nil
}

// ValidateToken checks whether a token corresponds to an accepted request
// of the expected type, returning the request if so. Used by the actual
// /upload and /download handlers to enforce that consent happened first.
func (cs *ConsentStore) ValidateToken(token string, expectedType RequestType) *TransferRequest {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for _, req := range cs.requests {
		if req.Token == token && req.Status == StatusAccepted && req.Type == expectedType {
			return req
		}
	}
	return nil
}

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
