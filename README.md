# SQLGateway <!-- omit in toc -->


Access your SQL database over HTTP like it’s a SQL database but with superpowers. An edge function's best friend.

**Superpowers include:**

- HTTP access for SQL databases enable WASM-based runtimes to use TCP-connected DBs
- Connection pooling protects from reconnects, wasted idle connections, and bursts of load
- Automatic query and transaction tracing
- Caching capabilities

_Currently only the PSQL protocol is supported. Additional protocol support (like MySQL) is on the roadmap._

- [Quick Start](#quick-start)
- [Why This Exists](#why-this-exists)
  - [Querying and Transactions](#querying-and-transactions)
  - [Automatic query and transaction tracing](#automatic-query-and-transaction-tracing)
  - [Caching (Coming Soon)](#caching-coming-soon)
  - [Connection Pooling](#connection-pooling)
  - [Database Throttling Under Load](#database-throttling-under-load)
- [API](#api)
  - [GET /hc](#get-hc)
  - [POST /psql/query](#post-psqlquery)
  - [/psql/begin](#psqlbegin)
  - [/psql/commit](#psqlcommit)
  - [/psql/rollback](#psqlrollback)
  - [Error handling](#error-handling)
- [Configuration](#configuration)
- [Auth](#auth)
- [Clustered vs. Single Node](#clustered-vs-single-node)
- [Transactions](#transactions)
- [Running distributed tests](#running-distributed-tests)

## Quick Start


Pull this repo:

```
git clone https://github.com/danthegoodman1/SQLGateway && cd SQLGateway
```

Run the `docker-compose.yml` file:

```
docker compose up
```

Then in another terminal, run:

```
curl --location --request POST 'http://localhost:8080/psql/query' \
--header 'Content-Type: application/json' \
--data-raw '{
    "Queries": [
        {
            "Statement": "SELECT 1 as num_one, NOW() as current_time"
        }
    ]
}'
```

You should get the following back (with a different time):

```
{"Queries":[{"Columns":[["num_one","current_time"]],"Rows":[[1,"2022-11-27T19:20:13.030114Z"]],"TimeNS":958400}]}
```

## Why This Exists

I wanted to use Cloudflare Workers, but also the Postgres ecosystem (specifically CockroachDB Serverless).

The idea was to keep the HTTP layer out of the way and make it feel like you are talking to a normal SQL database.

Now we can connect the two worlds of WASM-runtimes and SQL databases without vendor lock-in!

Some WASM runtimes that can now use SQL databases:

- Cloudflare Workers
- Vercel Edge Functions
- Fastly Compute@Edge
- Netlify Functions _note: [this](https://wasmedge.org/book/en/write_wasm/js/networking.html#tcp-client) seems to indicate that TCP connections may be supported, since they (at least used to) use WasmEdge. I have not bothered testing however :P_

Some Databases that WASM runtimes can now use:

- AWS RDS & Aurora
- GCP Cloud SQL
- CockroachDB Dedicated & Serverless
- DigitalOcean managed databases
- UpCloud Managed Databases

### Querying and Transactions

Send single queries, or send an array of queries to run atomically in a transaction.

Start a transaction and go back and forth between the DB and your code just like normal. The nodes in the cluster will automatically route transaction queries to the correct node (coordinated through Redis). Abandoned transactions will be garbage collected.

### Automatic query and transaction tracing

Metric logs emitted on the performance of individual queries, as well as entire transactions. Build dashboards and create alerts to find slowdowns and hot-spots in your code.

Coming soon (maybe?): Alerting and dashboards (for now just use some logging provider)

### Caching (Coming Soon)

Specify SELECTs that don’t need to be consistent you can have them cache and TTL with stale-while-revalidate support.

### Connection Pooling

Prevent constant session creation from creating unnecessary load on the DB, and burst execution environments from holding idle connections that won't be used again. 

Use HTTP Keep-Alive to keep connections warm for Lambda-like environments, but don’t risk overloading the DB with new connections or leaving tons of resource-intensive DB sessions idle.

### Database Throttling Under Load

With a finite number of pool connections, you prevent uncapped load from hitting your database directly.

## API

### GET /hc

Health check endpoint, only guarantees that the HTTP server is running.

### POST /psql/query

Request Body:

_`*` indicates optional_
```
{
  Queries: []{
      Statement:   string
      Params:      []any
      Exec:        *bool // if provided, then no `Rows` or `Columns` will be returned for this query.
      TxKey:       *string
    }
    
  TxID:    *string
}
```

Examples:
```json
{
  "Queries": [
    {
      "Statement": "SELECT $1::INT8 as a_number",
      "Params": [
        42
      ]
    }
  ]
}
```
```json
{
  "Queries": [
    {
      "Statement": "CREATE TABLE test_table IF NOT EXISTS ( id TEXT NOT NULL, val TEXT NOT NULL, PRIMARY KEY(id) )",
      "Exec": true
    }
  ]
}
```

**Note:** Casting is probably required for parameters as due to the primitive type selection the SQL cannot always interpret which SQL type a JSON property should use.

If given a single query, it will be run directly on the connection.

If given multiple items, they will be run within the same transaction. You will receive the results of all that succeed,
however if a single query fails then the entire transaction will fail, and all queries will remain un-applied regardless
of whether there were rows returned. Rows will be returned for the successful queries of a failing transaction.

If a `TxID` is provided, then it will be run within a transaction, proxying if required.

DO NOT CALL `COMMIT` OR `ROLLBACK` through here, that should be handled via the respective endpoints, or functions within the client libraries.

Response Body:

```
{
    Queries []{
        Columns:  [][]any
        Rows:     [][]any
        Error:    *string
        TimeNS:   *int64 
    }
    
    // Whether this was proxied to a remote node
    Remote: bool
}
```

Any query errors that occur will be included in the response body, rather than failing the request.

### /psql/begin

Starts a new transaction.

Request Body:

```
{
    TxTimeoutSec: *int64 // sets the garbage collection timeout, default `30`
}
```

Returns the transaction ID that must be carried through subsequent requests.

Response Body:

```
{
    TxID: string
}
```

### /psql/commit

Commits an existing transaction. Returns status `200` and no content if successful.

Request Body:

```
{
    TxID: string
}
```

### /psql/rollback

Rolls back an existing transaction. Returns status `200` and no content if successful. 

Request Body:

```
{
    TxID: string
}
```

### Error handling

All processing errors (not query errors) will return a 4XX/5XX error code, and as a `text/plain` response body.

## Configuration

Configuration is done through environment variables

| Env Var            | Description                                                                                                                | Required?                  | Default |
|--------------------|----------------------------------------------------------------------------------------------------------------------------|----------------------------|---------|
| `PG_DSN`           | PSQL wire protocol DSN. Used to connect to DB                                                                              | Yes                        |         |
| `PG_POOL_CONNS`    | Number of pool connections to acquire                                                                                      | No                         | `2`     |
| `REDIS_ADDR`       | Redis Address. Currently used in non-cluster mode (standard client).<br/>If omitted then clustering features are disabled. | No                         |         |
| `REDIS_PASSWORD`   | Redis connection password                                                                                                  | No                         |         |
| `REDIS_POOL_CONNS` | Number of pool connections to Redis.                                                                                       | No                         | `2`     |
| `V_NAMESPACE`      | Virtual namespace for Redis. Sets the key prefix for Service discovery.                                                    | Yes (WIP, so No currently) |         |
| `POD_URL`          | Direct URL that this pod/node can be reached at.<br/>Replaces `POD_NAME` and `POD_BASE_DOMAIN` if exists.                  | Yes (conditional)          |         |
| `POD_NAME`         | Name of the node/pod (k8s semantics).<br/>Pod can be reached at {POD_NAME}{POD_BASE_DOMAIN}                                | Yes (conditional)          |         |
| `POD_BASE_DOMAIN`  | Base domain of the node/pod (k8s semantics).<br/>Pod can be reached at {POD_NAME}{POD_BASE_DOMAIN}                         | Yes (conditional)          |         |
| `HTTP_PORT`        | HTTP port to run the HTTP(2) server on                                                                                     | No                         | `8080`  |
| `POD_HTTPS`        | Indicates whether the pods should use HTTPS to contact each other.<br/>Set to `1` if they should use HTTPS.                | No                         |         |
| `TRACES`           | Indicates whether query trace information should be included in log contexts.<br/>Set to `1` if they should be.            | No                         |         |
| `DEBUG`            | Indicates whether the debug log level should be enabled.<br/>Set to `1` to enable.                                         | No                         |         |
| `PRETTY`           | Indicates whether pretty logs should be printed.<br/>Set to `1` to enable.                                                 |                            |         |
| `AUTH_USER`        | Sets the Basic Auth username required to connect. Requires that `AUTH_PASS` be set as well                                 | Yes (conditional)          |         |
| `AUTH_PASS`        | Sets the Basic Auth password required to connect. Requires that `AUTH_USER` be set as well                                 | Yes (conditional)          |         |

## Auth

SQLGateway optionally supports Basic Auth in the style of `http(s)://USER:PASS@your.domain.tld`, just like a DB DSN.

To configure a username and password, set the env vars `AUTH_USER` and `AUTH_PASS`, then access with `http://{AUTH_USER}:{AUTH_PASS}@yourdomain`

## Performance and Overhead

With some light testing on my laptop (2019 16" MBP, 8 core 64GB ram) selecting 10,001 rows directly from `pgx` without processing takes ~8-9ms, while requesting that same query through SQLGateway takes ~12-13ms.

SQLGateway currently uses the native JSON package, which is known to be very slow. The binding overhead can be optimized by >10x with a more efficient JSON package for binding.

HTTP overhead is ~80us per request, a bit more if you add in Basic Auth.

## Clustered vs. Single Node

SQLGateway can either be run in a cluster, or as a single node.

If running as a single node, ensure to omit the `REDIS_ADDR` env var.

When running in clustered mode (`REDIS_ADDR` env var present), it will require that a connection to Redis can be established.

When transactions are not found locally, a lookup to Redis will be attempted. If the transaction is found on a remote pod,
the request will be proxied to the remote pod.

Redis Cluster mode support and etcd support are on the roadmap.

## Transactions

Transactions (and query requests) have a default timeout of 30 seconds.

When any query in a transaction fails, the transaction is automatically rolled back and the pool connection released, meaning that the client that errors is not responsible for doing so.

If a transaction times out then it will also automatically roll back and release the pool connection.

If a pod crashes while it has a transaction, then the transaction will be immediately released, but may remain present within Redis.
A special error is returned for this indicating this may be the case.

If Redis crashes while a transaction is still held on a pod, then other pods will not be able to route transaction queries to this pod.
The timeout will garbage collect these transactions, but the connection will remain held until it times out. 

## Running distributed tests

Run 2 instances connected to a local CRDB/Postgres and Redis like the following (DSN and Redis env vars omitted):

```POD_URL="localhost:8080" HTTP_PORT="8080" task```

```POD_URL="localhost:8081" HTTP_PORT="8081" task```
