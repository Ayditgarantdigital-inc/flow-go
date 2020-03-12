package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertGuarantee(guarantee *flow.CollectionGuarantee) func(*badger.Txn) error {
	return insert(makePrefix(codeGuarantee, guarantee.CollectionID), guarantee)
}

func CheckGuarantee(collID flow.Identifier, exists *bool) func(*badger.Txn) error {
	return check(makePrefix(codeGuarantee, collID), exists)
}

func RetrieveGuarantee(collID flow.Identifier, guarantee *flow.CollectionGuarantee) func(*badger.Txn) error {
	return retrieve(makePrefix(codeGuarantee, collID), guarantee)
}

func IndexGuaranteePayload(height uint64, blockID flow.Identifier, parentID flow.Identifier, guaranteeIDs flow.IdentifierList) func(*badger.Txn) error {
	return insert(toPayloadIndex(codeIndexGuarantee, height, blockID, parentID), guaranteeIDs)
}

func LookupGuaranteePayload(height uint64, blockID flow.Identifier, parentID flow.Identifier, collIDs *flow.IdentifierList) func(*badger.Txn) error {
	return retrieve(toPayloadIndex(codeIndexGuarantee, height, blockID, parentID), collIDs)
}

// VerifyGuaranteePayload verifies that the candidate collection IDs
// don't exist in any ancestor block.
func VerifyGuaranteePayload(height uint64, blockID flow.Identifier, collIDs flow.IdentifierList) func(*badger.Txn) error {
	return iterate(makePrefix(codeIndexGuarantee, height), makePrefix(codeIndexGuarantee, uint64(0)), validatepayload(blockID, collIDs))
}

// CheckGuaranteePayload populates `invalidIDs` with any IDs in the candidate
// set that already exist in an ancestor block.
func CheckGuaranteePayload(height uint64, blockID flow.Identifier, candidateIDs flow.IdentifierList, invalidIDs *map[flow.Identifier]struct{}) func(*badger.Txn) error {
	return iterate(makePrefix(codeIndexGuarantee, height), makePrefix(codeIndexGuarantee, uint64(0)), searchduplicates(blockID, candidateIDs, invalidIDs))
}
