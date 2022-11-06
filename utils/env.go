package utils

import "os"

var (
	PG_DSN     = os.Getenv("PG_DSN")
	POOL_CONNS = GetEnvOrDefaultInt("POOL_CONNS", 10)
)
