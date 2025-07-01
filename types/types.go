package types

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/sqids/sqids-go"
)

var (
	// APIVersion is the version of the API.
	APIVersion = "v1"
)

// ContextKey is a type used for keys in the context.
type ContextKey string

// Payload represents the structure of the JSON payload expected in requests.
// It contains the short URL and the long URL.
type Payload struct {
	ShortURL string `json:"ShortURL"`
	LongURL  string `json:"LongURL"`
}

// SqidsGen is a generator for unique IDs using the sqids package.
type SqidsGen struct {
	Sqid *sqids.Sqids
}

// NewSqidsGen creates a new instance of SqidsGen.
func NewSqidsGen() *SqidsGen {
	squid, _ := sqids.New()
	sqidsGen := &SqidsGen{
		Sqid: squid,
	}
	return sqidsGen
}

// Generate creates a new unique ID using the sqids package.
// It encodes an array of uint64 values into a string ID.
func (s *SqidsGen) Generate(arr []uint64) string {
	id, _ := s.Sqid.Encode(arr)
	return id
}

// DecodePayload decodes the JSON payload from the request body.
func DecodePayload(r *http.Request) (*Payload, error) {
	var payload Payload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return nil, NewBadRequestError([]Details{
			{Field: "body", Issue: "Invalid JSON format"},
		})
	}
	return &payload, nil
}

// GlobalCounter is a thread-safe counter that can be used to generate unique IDs.
// It uses a mutex to ensure that increments and reads are safe in a concurrent environment.
type GlobalCounter struct {
	mu    sync.Mutex
	count uint64
}

// Increment increases the counter by 1 in a thread-safe manner.
func (c *GlobalCounter) Increment() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count++
}

// Count returns the current value of the counter without incrementing it.
func (c *GlobalCounter) Count() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}

// GetAndIncrement returns the current value of the counter and then increments it.
func (c *GlobalCounter) GetAndIncrement() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count++
	return c.count
}

// NewGlobalCounter creates a new instance of GlobalCounter.
func NewGlobalCounter() *GlobalCounter {
	return &GlobalCounter{
		count: 0,
	}
}