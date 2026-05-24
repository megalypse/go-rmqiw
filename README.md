# RMQIW

RMQIW is an interactive terminal tool for running RabbitMQ-driven workflows and checking their effects in Postgres.

Each workflow is a JSON file made of ordered steps. A step publishes one message to RabbitMQ, then repeatedly runs a Postgres query until the query returns `true`. This makes the tool useful for validating event-driven flows end to end.

The repository includes a complete local mock stack:

- RabbitMQ with the management UI
- Postgres
- A mock backend that consumes RabbitMQ messages and inserts records into Postgres
- A sample multi-step journey

## Requirements

- Go, matching the version in `go.mod`
- Docker and Docker Compose
- `make`

## Quick Start

Start the local RabbitMQ, Postgres, and mock backend:

```sh
docker compose up -d --build
```

Create a local RMQIW config directory from the bundled mocks:

```sh
make init-mocks
```

Install the CLI:

```sh
make install-cli
```

Reload your shell so the generated `PATH` and `RMQIW_PATH` exports are available:

```sh
source ~/.zshrc
```

Run the tool:

```sh
rmqiw
```

If you do not want `make` to update your shell profile, use:

```sh
make init-mocks UPDATE_SHELL=0
make install-cli UPDATE_SHELL=0
export RMQIW_PATH="$HOME/rmqiw"
export PATH="$HOME/.local/bin:$PATH"
rmqiw
```

## Running Without Installing

You can run directly from the repository:

```sh
docker compose up -d --build
RMQIW_PATH="$PWD/rmqiuwpath" go run .
```

The repository sample directory is currently named `rmqiuwpath`.

## Local Services

The Docker Compose stack exposes:

| Service | Address | Credentials |
| --- | --- | --- |
| RabbitMQ | `localhost:5672` | `rmqiw` / `rmqiw` |
| RabbitMQ Management | `http://localhost:15672` | `rmqiw` / `rmqiw` |
| Postgres | `localhost:5432` | `rmqiw` / `rmqiw` |

The mock backend creates:

- exchange: `rmqiw.mock`
- queues:
  - `rmqiw.mock.users`
  - `rmqiw.mock.orders`
  - `rmqiw.mock.payments`
- table: `mock_events`

Check consumed events:

```sh
docker compose exec psql psql -U rmqiw -d rmqiw -c \
  "SELECT flow_step, routing_key, body, received_at FROM mock_events ORDER BY id DESC LIMIT 10;"
```

Stop the stack:

```sh
docker compose down
```

Remove persisted RabbitMQ and Postgres volumes:

```sh
docker compose down -v
```

## CLI Usage

When you start `rmqiw`, it opens an interactive TUI.

1. Select a journey.
2. Select a pace.
3. Watch each step publish a message and wait for the database check.

Available paces:

- `The clock is ticking`: runs all steps continuously.
- `Step by step`: pauses before each step and continues only after pressing `enter`.

Keys:

| Key | Action |
| --- | --- |
| `up` / `down` | Move through menu options |
| `enter` | Select an option or run the next step |
| `esc` | Quit |

## Configuration Directory

The app reads its configuration from the directory pointed to by `RMQIW_PATH`.

That directory must contain:

```text
RMQIW_PATH/
  config.json
  flows/
    any-flow-name.json
```

Create the sample directory:

```sh
make init-mocks
```

By default this creates:

```text
$HOME/rmqiw/
  config.json
  flows/
    flow1.json
```

Use a different directory:

```sh
make init-mocks RMQIW_PATH="$HOME/dev/rmqiw-config"
export RMQIW_PATH="$HOME/dev/rmqiw-config"
```

Use a different source template:

```sh
make init-mocks MOCK_SOURCE=./rmqiuwpath RMQIW_PATH="$HOME/rmqiw"
```

## `config.json`

`config.json` defines the RabbitMQ and Postgres connections used by the CLI.

Example:

```json
{
  "postgres": {
    "host": "localhost",
    "port": 5432,
    "user": "rmqiw",
    "password": "rmqiw",
    "database": "rmqiw",
    "ssl_mode": "disable"
  },
  "rabbitmq": {
    "host": "localhost",
    "port": 5672,
    "user": "rmqiw",
    "password": "rmqiw",
    "vhost": "/"
  }
}
```

Environment variables can override connection fields:

| Variable | Field |
| --- | --- |
| `RMQIW_POSTGRES_HOST` | `postgres.host` |
| `RMQIW_POSTGRES_PORT` | `postgres.port` |
| `RMQIW_POSTGRES_USER` | `postgres.user` |
| `RMQIW_POSTGRES_PASSWORD` | `postgres.password` |
| `RMQIW_POSTGRES_DATABASE` | `postgres.database` |
| `RMQIW_POSTGRES_SSL_MODE` | `postgres.ssl_mode` |
| `RMQIW_RABBITMQ_HOST` | `rabbitmq.host` |
| `RMQIW_RABBITMQ_PORT` | `rabbitmq.port` |
| `RMQIW_RABBITMQ_USER` | `rabbitmq.user` |
| `RMQIW_RABBITMQ_PASSWORD` | `rabbitmq.password` |
| `RMQIW_RABBITMQ_VHOST` | `rabbitmq.vhost` |

The Docker mock backend uses these overrides to connect to `psql` and `rmq` inside the Compose network while the host CLI keeps using `localhost`.

## Flow Files

Every `*.json` file inside `RMQIW_PATH/flows` is loaded as a journey.

Minimal shape:

```json
{
  "name": "User checkout journey",
  "description": "Publishes user, order, and payment events and waits for each backend record.",
  "steps": [
    {
      "name": "Publish user.created",
      "description": "Sends a user.created message.",
      "poll_interval": 100,
      "poll_query": "SELECT EXISTS (SELECT 1 FROM mock_events WHERE flow_step = 'checkout-create-user')",
      "message": {
        "exchange": "rmqiw.mock",
        "routing_key": "users.created",
        "headers": {
          "flow_step": "checkout-create-user"
        },
        "body": {
          "id": "user-1",
          "name": "Alice"
        }
      }
    }
  ]
}
```

### Step Fields

| Field | Meaning |
| --- | --- |
| `name` | Label shown in the TUI |
| `description` | Human-readable context for the step |
| `poll_interval` | Interval between database checks, in milliseconds |
| `poll_query` | SQL query that must return one boolean column |
| `message.exchange` | RabbitMQ exchange |
| `message.routing_key` | RabbitMQ routing key |
| `message.headers` | RabbitMQ message headers |
| `message.body` | Message payload |

`poll_query` must return a boolean. The usual pattern is:

```sql
SELECT EXISTS (
  SELECT 1
  FROM some_table
  WHERE some_condition = true
)
```

`message.body` is a JSON raw message. It can be any JSON value:

```json
"plain text"
```

```json
{"id": "user-1"}
```

```json
[1, 2, 3]
```

```json
true
```

```json
null
```

The bytes published to RabbitMQ are the raw JSON bytes from the flow file.

## Sample Journey

The bundled sample journey has three steps:

1. Publish `users.created`
2. Publish `orders.created`
3. Publish `payments.captured`

The mock backend consumes those messages and inserts records into `mock_events`. Each step waits for its corresponding record before continuing.

Run it continuously:

```sh
rmqiw
# Select "User checkout journey"
# Select "The clock is ticking"
```

Run it manually:

```sh
rmqiw
# Select "User checkout journey"
# Select "Step by step"
# Press enter before each step
```

Verify results:

```sh
docker compose exec psql psql -U rmqiw -d rmqiw -c \
  "SELECT flow_step, routing_key, body FROM mock_events WHERE flow_step LIKE 'checkout-%' ORDER BY id DESC LIMIT 3;"
```

## Make Targets

Show available targets:

```sh
make help
```

Install the CLI:

```sh
make install-cli
```

Defaults:

- binary name: `rmqiw`
- install directory: `$HOME/.local/bin`
- shell profile: `$HOME/.zshrc`

Customize:

```sh
make install-cli APP_NAME=rmqiw INSTALL_DIR="$HOME/bin"
```

Generate mock config files:

```sh
make init-mocks
```

Customize:

```sh
make init-mocks RMQIW_PATH="$HOME/dev/rmqiw"
```

Disable shell profile updates:

```sh
make install-cli UPDATE_SHELL=0
make init-mocks UPDATE_SHELL=0
```

## Developing

Run tests:

```sh
go test ./...
```

Run the CLI from source:

```sh
RMQIW_PATH="$PWD/rmqiuwpath" go run .
```

Rebuild the mock backend image:

```sh
docker compose build mock-backend
```

View mock backend logs:

```sh
docker compose logs -f mock-backend
```

Inspect RabbitMQ queues:

```sh
docker compose exec rmq rabbitmqctl list_queues name messages consumers
```

Inspect RabbitMQ bindings:

```sh
docker compose exec rmq rabbitmqctl list_bindings source_name destination_name routing_key
```

## Troubleshooting

### `RMQIW_PATH` is not set

The CLI needs `RMQIW_PATH` to point to a directory with `config.json` and `flows/`.

Fix:

```sh
make init-mocks
source ~/.zshrc
```

Or set it manually:

```sh
export RMQIW_PATH="$PWD/rmqiuwpath"
```

### Cannot connect to RabbitMQ

Make sure Docker Compose is running:

```sh
docker compose ps
```

RabbitMQ should be healthy and expose `5672`.

Check credentials in `config.json`:

```json
{
  "rabbitmq": {
    "host": "localhost",
    "port": 5672,
    "user": "rmqiw",
    "password": "rmqiw",
    "vhost": "/"
  }
}
```

### Cannot connect to Postgres

Check the service:

```sh
docker compose ps psql
```

Check credentials:

```sh
docker compose exec psql psql -U rmqiw -d rmqiw -c "SELECT 1;"
```

### A step times out

A step times out when its `poll_query` does not return `true` within the step timeout.

Check:

- the message was routed to the expected queue
- a consumer is attached to the queue
- the backend inserted the expected database row
- the `poll_query` matches the inserted data

Useful commands:

```sh
docker compose exec rmq rabbitmqctl list_queues name messages consumers
docker compose logs -f mock-backend
docker compose exec psql psql -U rmqiw -d rmqiw -c "SELECT * FROM mock_events ORDER BY id DESC LIMIT 10;"
```

### Shell profile was not updated

Use the exports printed by `make`:

```sh
export PATH="$HOME/.local/bin:$PATH"
export RMQIW_PATH="$HOME/rmqiw"
```

Or rerun with an explicit profile:

```sh
make install-cli SHELL_PROFILE="$HOME/.bashrc"
make init-mocks SHELL_PROFILE="$HOME/.bashrc"
```
