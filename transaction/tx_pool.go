package transaction

import (
	"sync"
)

// Thread-safe transaction pool.
// 线程安全事务池
type transactionPool struct {
	transactions map[TxKey]Tx

	mu sync.RWMutex
}

func createTransactionPool() *transactionPool {
	return &transactionPool{
		transactions: make(map[TxKey]Tx),
	}
}

func (store *transactionPool) put(key TxKey, tx Tx) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.transactions[key] = tx
}

func (store *transactionPool) get(key TxKey) (Tx, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	tx, ok := store.transactions[key]
	return tx, ok
}

func (store *transactionPool) drop(key TxKey) bool {
	if _, ok := store.get(key); !ok {
		return false
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	delete(store.transactions, key)
	return true
}

func (store *transactionPool) all() []Tx {
	all := make([]Tx, 0)
	store.mu.RLock()
	defer store.mu.RUnlock()
	for _, tx := range store.transactions {
		all = append(all, tx)
	}

	return all
}
