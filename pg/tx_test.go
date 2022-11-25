package pg

import (
	"context"
	"github.com/danthegoodman1/SQLGateway/red"
	"testing"
	"time"
)

func TestTransactionStorage(t *testing.T) {
	txMeta := &red.TransactionMeta{
		TxID:   "test",
		PodID:  "testpod",
		Expiry: time.Now().Add(time.Second * 10),
		PodURL: "localhost:8080",
	}

	err := red.SetTransaction(context.Background(), txMeta)
	if err != nil {
		t.Fatal(err)
	}

	txBack, err := red.GetTransaction(context.Background(), txMeta.TxID)
	if err != nil {
		t.Fatal(err)
	}

	if txBack.TxID != txMeta.TxID {
		t.Fatalf("transactions not the same: %+v %+v", txBack, txMeta)
	}
}
