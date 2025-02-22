package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger/operation"
)

// Guarantees implements persistent storage for collection guarantees.
type Guarantees struct {
	db    *badger.DB
	cache *Cache
}

func NewGuarantees(collector module.CacheMetrics, db *badger.DB) *Guarantees {

	store := func(key interface{}, val interface{}) func(*badger.Txn) error {
		collID := key.(flow.Identifier)
		guarantee := val.(*flow.CollectionGuarantee)
		return operation.SkipDuplicates(operation.InsertGuarantee(collID, guarantee))
	}

	retrieve := func(key interface{}) func(*badger.Txn) (interface{}, error) {
		collID := key.(flow.Identifier)
		var guarantee flow.CollectionGuarantee
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.RetrieveGuarantee(collID, &guarantee)(tx)
			return &guarantee, err
		}
	}

	g := &Guarantees{
		db: db,
		cache: newCache(collector, metrics.ResourceGuarantee,
			withLimit(flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve)),
	}

	return g
}

func (g *Guarantees) storeTx(guarantee *flow.CollectionGuarantee) func(*badger.Txn) error {
	return g.cache.Put(guarantee.ID(), guarantee)
}

func (g *Guarantees) retrieveTx(collID flow.Identifier) func(*badger.Txn) (*flow.CollectionGuarantee, error) {
	return func(tx *badger.Txn) (*flow.CollectionGuarantee, error) {
		val, err := g.cache.Get(collID)(tx)
		if err != nil {
			return nil, err
		}
		return val.(*flow.CollectionGuarantee), nil
	}
}

func (g *Guarantees) Store(guarantee *flow.CollectionGuarantee) error {
	return operation.RetryOnConflict(g.db.Update, g.storeTx(guarantee))
}

func (g *Guarantees) ByCollectionID(collID flow.Identifier) (*flow.CollectionGuarantee, error) {
	tx := g.db.NewTransaction(false)
	defer tx.Discard()
	return g.retrieveTx(collID)(tx)
}
