package utils

import "os"

var (
	PG_DSN        = os.Getenv("PG_DSN")
	PG_POOL_CONNS = GetEnvOrDefaultInt("PG_POOL_CONNS", 2)

	REDIS_ADDR       = os.Getenv("REDIS_ADDR")
	REDIS_PASSWORD   = os.Getenv("REDIS_PASSWORD")
	REDIS_POOL_CONNS = GetEnvOrDefaultInt("REDIS_POOL_CONNS", 2)

	// V_NAMESPACE Virtual namespace for redis hash map name
	V_NAMESPACE = os.Getenv("V_NAMESPACE")

	// this pod can be reached at url: {POD_NAME}{POD_BASE_DOMAIN}
	POD_NAME = os.Getenv("POD_NAME")
	// this pod can be reached at url: {POD_NAME}{POD_BASE_DOMAIN}
	POD_BASE_DOMAIN = os.Getenv("POD_BASE_DOMAIN")
	// overrides {POD_NAME}{POD_BASE_DOMAIN} to advertise {POD_URL}
	POD_URL = os.Getenv("POD_URL")

	HTTP_PORT = GetEnvOrDefault("HTTP_PORT", "8080")

	// Whether to use https for inter-pod communication, defaults to false
	POD_HTTPS = os.Getenv("POD_HTTPS") == "1"

	// Whether to emit traces in the HTTP logs
	TRACES = os.Getenv("TRACES") == "1"
)
