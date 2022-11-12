package utils

import "os"

var (
	PG_DSN     = os.Getenv("PG_DSN")
	POOL_CONNS = GetEnvOrDefaultInt("POOL_CONNS", 10)

	CACHE_DEFAULT = os.Getenv("CACHE_DEFAULT") == "1"
	K8S_SD        = os.Getenv("K8S_SD") == "1"
)
