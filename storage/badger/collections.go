package badger

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type Collections struct {
	db *badger.DB
}

func NewCollections(db *badger.DB) *Collections {
	c := Collections{
		db: db,
	}
	return &c
}

func (c *Collections) StoreLight(collection *flow.LightCollection) error {
	return c.db.Update(func(tx *badger.Txn) error {
		err := operation.InsertCollection(collection)(tx)
		if err != nil {
			return fmt.Errorf("could not insert collection: %w", err)
		}

		return nil
	})
}

func (c *Collections) Store(collection *flow.Collection) error {
	return c.db.Update(func(btx *badger.Txn) error {
		light := collection.Light()
		err := operation.InsertCollection(&light)(btx)
		if err != nil {
			return fmt.Errorf("could not insert collection: %w", err)
		}

		for _, tx := range collection.Transactions {
			err = operation.InsertTransaction(tx)(btx)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (c *Collections) ByID(colID flow.Identifier) (*flow.Collection, error) {
	var (
		light      flow.LightCollection
		collection flow.Collection
	)

	err := c.db.View(func(btx *badger.Txn) error {
		err := operation.RetrieveCollection(colID, &light)(btx)
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return storage.ErrNotFound
			}
			return fmt.Errorf("could not retrieve collection: %w", err)
		}

		for _, txID := range light.Transactions {
			var tx flow.TransactionBody
			err = operation.RetrieveTransaction(txID, &tx)(btx)
			if err != nil {
				if errors.Is(err, badger.ErrKeyNotFound) {
					return storage.ErrNotFound
				}
				return fmt.Errorf("could not retrieve transaction: %w", err)
			}

			collection.Transactions = append(collection.Transactions, &tx)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &collection, nil
}

func (c *Collections) LightByID(colID flow.Identifier) (*flow.LightCollection, error) {
	var collection flow.LightCollection

	err := c.db.View(func(tx *badger.Txn) error {
		err := operation.RetrieveCollection(colID, &collection)(tx)
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return storage.ErrNotFound
			}
			return fmt.Errorf("could nto retrieve transaction: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &collection, nil
}

func (c *Collections) Remove(colID flow.Identifier) error {
	return c.db.Update(func(btx *badger.Txn) error {
		err := operation.RemoveCollection(colID)(btx)
		if err != nil {
			return fmt.Errorf("could not remove collection: %w", err)
		}
		return nil
	})
}
