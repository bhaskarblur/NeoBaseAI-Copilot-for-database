package utils

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
)

// ============================================================================
// sync.Pool for Memory Optimization - Reduces GC Pressure
// ============================================================================

// JSONBufferPool provides reusable bytes.Buffer for JSON operations
// Reduces allocations in hot paths (processLLMResponse, sendStreamEvent, etc)
var JSONBufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// GetJSONBuffer retrieves a buffer from the pool
func GetJSONBuffer() *bytes.Buffer {
	return JSONBufferPool.Get().(*bytes.Buffer)
}

// PutJSONBuffer returns a buffer to the pool after resetting it
func PutJSONBuffer(buf *bytes.Buffer) {
	buf.Reset()
	JSONBufferPool.Put(buf)
}

// StringBuilderPool provides reusable strings.Builder for string concatenation
var StringBuilderPool = sync.Pool{
	New: func() interface{} {
		return new(strings.Builder)
	},
}

// GetStringBuilder retrieves a builder from the pool
func GetStringBuilder() *strings.Builder {
	return StringBuilderPool.Get().(*strings.Builder)
}

// PutStringBuilder returns a builder to the pool after resetting it
func PutStringBuilder(sb *strings.Builder) {
	sb.Reset()
	StringBuilderPool.Put(sb)
}

// ============================================================================
// Pre-allocated Pointer Values - Avoids Heap Allocation
// ============================================================================

// For frequently used pointer values, pre-allocate them once
// instead of creating new pointers on every use
var (
	// Pre-allocated boolean pointers
	truePtr  = true
	falsePtr = false

	// Pointers to pre-allocated values
	TrueBoolPtr  = &truePtr
	FalseBoolPtr = &falsePtr
)

// StringPtr returns a pointer to a string value
// Only used when you truly need a pointer to an optional field
func StringPtr(s string) *string {
	return &s
}

// IntPtr returns a pointer to an int value
func IntPtr(i int) *int {
	return &i
}

// Float64Ptr returns a pointer to a float64 value
func Float64Ptr(f float64) *float64 {
	return &f
}

// Int32Ptr returns a pointer to an int32 value
func Int32Ptr(i int32) *int32 {
	return &i
}

// BoolPtr returns a pointer to a bool value
func BoolPtr(b bool) *bool {
	return &b
}

// TruePtr returns a pointer to the pre-allocated true value
// Use this instead of BoolPtr(true) in hot paths
func TruePtr() *bool {
	return TrueBoolPtr
}

// FalsePtr returns a pointer to the pre-allocated false value
// Use this instead of BoolPtr(false) in hot paths
func FalsePtr() *bool {
	return FalseBoolPtr
}

// ============================================================================
// Optimized JSON Marshaling - Uses Pooled Buffers
// ============================================================================

// MarshalJSON marshals obj to JSON using a pooled buffer
// Better than json.Marshal for high-frequency calls
// Returns []byte that should not be retained after use
func MarshalJSON(obj interface{}) ([]byte, error) {
	buf := GetJSONBuffer()
	defer PutJSONBuffer(buf)

	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(obj); err != nil {
		return nil, err
	}

	// Copy buffer content before returning
	// (buffer will be reset when returned to pool)
	b := buf.Bytes()
	result := make([]byte, len(b))
	copy(result, b)
	return result, nil
}

// ============================================================================
// String Map Pooling - Reduces Map Allocations
// ============================================================================

// StringMapPool provides reusable maps for temporary use
var StringMapPool = sync.Pool{
	New: func() interface{} {
		return make(map[string]interface{})
	},
}

// GetStringMap retrieves a map from the pool
func GetStringMap() map[string]interface{} {
	return StringMapPool.Get().(map[string]interface{})
}

// PutStringMap returns a map to the pool after clearing it
func PutStringMap(m map[string]interface{}) {
	// Clear all entries
	for k := range m {
		delete(m, k)
	}
	StringMapPool.Put(m)
}

// ============================================================================
// Byte Slice Pooling - Reduces Buffer Allocations
// ============================================================================

// ByteSlicePool provides reusable byte slices (4KB)
var ByteSlicePool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 4096)
	},
}

// GetByteSlice retrieves a byte slice from the pool
func GetByteSlice() []byte {
	return ByteSlicePool.Get().([]byte)
}

// PutByteSlice returns a byte slice to the pool
func PutByteSlice(b []byte) {
	ByteSlicePool.Put(b[:4096])
}
