# SQLGateway
 

Access your SQL database over HTTP like it’s a SQL database but with superpowers. An edge function's best friend

**Superpowers include:**

- HTTP access for SQL databases enable WASM-based runtimes to use TCP-connected DBs
- Connection pooling protects from reconnects and idle connections, while also providing protection with a predictable max-load on the DB
- Automatic query and transaction tracing finds slow spots and anomalies
- Caching capabilities for frequent queries

_Currently only the PSQL protocol is supported. Other protocol support (like MySQL) are on the roadmap._

## Why This Exists

I wanted to use Cloudflare Workers, but also the Postgres ecosystem.

The idea was to keep the HTTP layer out of the way and make it feel like you are talking to a normal SQL database.

### Querying and Transactions

Send single queries, multiple queries will run atomically in a transaction, and starting a transaction will give you a consistent pool connection that allows you to go back and forth between the DB and your code just like normal. The nodes in the cluster will automatically route transaction queries to the correct node.

### Automatic query and transaction tracing

Metric logs emitted on the performance of individual queries, as well as entire transactions.

Coming soon (maybe?): Alerting and dashboards (for now just use some logging provider)

### Caching (Coming Soon)
Specify SELECTs that don’t need to be consistent you can have them cache and TTL with stale-while-revalidate support.

### Connection Pooling

Prevent constant session creation from creating unnecessary load on the DB, and burst execution environments from holding idle connections that won't be used again. 

Use HTTP Keep-Alive to keep connections warm for Lambda-like environments, but don’t risk overloading the DB with new connections or leaving tons of resource-intensive DB sessions idle.

### Database Throttling Under Load
With a finite number of pool connections, you prevent uncapped load from hitting your database directly. Take down SQLGateway before taking down your DB.

## /query endpoint

If given a single item, it will be run directly on the connection

If given multiple items, they will be run within the same transaction. You will receive the results of all that succeed,
however if a single query fails then the entire transaction will fail, and all queries will remain un-applied regardless
of whether there were rows returned.

## Running distributed tests

Run 2 instances connected to a local CRDB/Postgres and Redis like the following (DSN and Redis env vars omitted):

```POD_URL="localhost:8080" HTTP_PORT="8080" task```

```POD_URL="localhost:8081" HTTP_PORT="8081" task```