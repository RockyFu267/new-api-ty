package common

import (
	"io"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// NewOutboundJSONBody wraps the already-marshaled upstream request body into a
// BodyStorage. Playground requests always stay in memory. Otherwise, when disk
// cache is enabled and the payload exceeds the configured
// threshold, the data is written to a temp file and the original []byte can be
// GC'd, significantly reducing the heap residency while waiting for the
// upstream provider to respond (the dominant cost for large base64 payloads).
//
// In memory mode the underlying memoryStorage reuses the same backing array,
// so this is equivalent to bytes.NewReader(data) in terms of memory usage.
//
// The caller MUST invoke closer.Close() once the upstream call has finished
// (typically via defer) to release the disk file / memory accounting.
//
// The returned body exposes its size and replay capability without exposing
// io.Closer. Request construction uses that metadata to populate ContentLength
// and GetBody, while the caller retains ownership of the underlying storage
// through the separately returned closer.
func NewOutboundJSONBody(data []byte, contexts ...*gin.Context) (body common.ReplayableBody, closer io.Closer, err error) {
	var storage common.BodyStorage
	if len(contexts) > 0 {
		storage, err = common.CreateRequestBodyStorage(contexts[0], data)
	} else {
		storage, err = common.CreateBodyStorage(data)
	}
	if err != nil {
		return nil, nil, err
	}
	return common.NewReplayableBodyReader(storage), storage, nil
}
