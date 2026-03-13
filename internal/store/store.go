package store

import (
	"context"

	"github.com/gustycube/spyder/internal/emit"
)

// Store persists batch data to a database.
type Store interface {
	// WriteBatch persists all nodes and edges from a batch.
	WriteBatch(ctx context.Context, batch emit.Batch) error

	// Close cleanly shuts down the store connection.
	Close(ctx context.Context) error
}
