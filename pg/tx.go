package pg

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/danthegoodman1/PSQLGateway/utils"
	"github.com/jackc/pgx/v4/pgxpool"
	"time"
)

type (
	TxMeta struct {
		ExpiryTime time.Time
		NodeID     string
		DSNHash    string
		ID         string
		PoolConn   *pgxpool.Conn
	}
)

func NewTxMeta(nodeID, dsnHash string) *TxMeta {
	return &TxMeta{
		ExpiryTime: time.Now().Add(time.Second * 30),
		NodeID:     nodeID,
		DSNHash:    dsnHash,
		ID:         utils.GenRandomID("tx_"),
	}
}

func (tx *TxMeta) MakeKey() (string, error) {
	txJSON, err := json.Marshal(tx)
	if err != nil {
		return "", fmt.Errorf("error in json.Marshal: %w", err)
	}

	return base64.StdEncoding.EncodeToString(txJSON), nil
}

func DecodeTxKey(key string) (*TxMeta, error) {
	txJSON, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("error in base64.StdEncoding.DecodeString: %w", err)
	}
	var txMeta TxMeta
	err = json.Unmarshal(txJSON, &txMeta)
	if err != nil {
		return nil, fmt.Errorf("error in json.Unmarshal: %w", err)
	}
	return &txMeta, nil
}
