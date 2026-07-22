# RMQIW
RMQ in Wonderland

`RMQIW_PATH` é a principal variável de ambiente da ferramenta. Ela deve apontar para um diretório contendo `config.json` e `flows/`.

```text
$RMQIW_PATH/
  config.json
  flows/
    checkout.json
```

## Instalar

```sh
make install-cli
source ~/.zshrc
```

Por padrão, a CLI é instalada em `$HOME/.local/bin/rmqiw` e esse diretório é adicionado ao `PATH`.

Sem alterar o shell profile:

```sh
make install-cli UPDATE_SHELL=0
export PATH="$HOME/.local/bin:$PATH"
```

Customizando:

```sh
make install-cli APP_NAME=rmqiw INSTALL_DIR="$HOME/bin"
```

## Configurar

Crie uma estrutura inicial:

```sh
make init-mocks
source ~/.zshrc
```

Por padrão, isso cria `$HOME/rmqiw` e adiciona:

```sh
export RMQIW_PATH="$HOME/rmqiw"
```

Para outro diretório:

```sh
make init-mocks RMQIW_PATH="$HOME/dev/rmqiw"
export RMQIW_PATH="$HOME/dev/rmqiw"
```

## `config.json`

```json
{
  "vars": {
    "tenantId": "tenant-1",
    "merchantId": {
      "sql": "SELECT id FROM merchants LIMIT 1"
    }
  },
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

Também é possível sobrescrever conexão por env vars:

```sh
RMQIW_POSTGRES_HOST=localhost
RMQIW_POSTGRES_PORT=5432
RMQIW_POSTGRES_USER=rmqiw
RMQIW_POSTGRES_PASSWORD=rmqiw
RMQIW_POSTGRES_DATABASE=rmqiw
RMQIW_POSTGRES_SSL_MODE=disable

RMQIW_RABBITMQ_HOST=localhost
RMQIW_RABBITMQ_PORT=5672
RMQIW_RABBITMQ_USER=rmqiw
RMQIW_RABBITMQ_PASSWORD=rmqiw
RMQIW_RABBITMQ_VHOST=/
```

## Adicionar Flows

Cada `*.json` em `$RMQIW_PATH/flows` vira uma jornada na TUI.

Cada step:

1. publica uma mensagem no RabbitMQ;
2. executa `poll_query` no Postgres até retornar `true`;
3. avança para o próximo step.

Exemplo:

```json
{
  "name": "User checkout journey",
  "description": "Publishes user, order, and payment events.",
  "steps": [
    {
      "name": "Publish user.created",
      "description": "Sends a user.created message.",
      "poll_interval": 100,
      "timeout": 10,
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

Campos do step:

- `poll_interval`: intervalo entre polls, em milissegundos.
- `timeout`: timeout do step, em segundos. Se omitido ou `0`, usa `10`.
- `poll_query`: query que deve retornar uma coluna booleana.
- `message.exchange`: exchange RabbitMQ.
- `message.routing_key`: routing key.
- `message.headers`: headers da mensagem.
- `message.body`: payload publicado.

`poll_query` normalmente usa `SELECT EXISTS`:

```sql
SELECT EXISTS (
  SELECT 1
  FROM alguma_tabela
  WHERE alguma_condicao = true
)
```

`message.body` aceita qualquer JSON válido: string, objeto, array, número, booleano ou `null`.

### Variáveis geradas

Flows podem declarar variáveis geradas na raiz do arquivo:

```json
{
  "name": "User checkout journey",
  "vars": {
    "requestId": "uuid()",
    "tenantId": "tenant-1",
    "retryCount": 3,
    "merchantId": {
      "sql": "SELECT id FROM merchants LIMIT 1"
    }
  },
  "steps": [
    {
      "poll_query": "SELECT EXISTS (SELECT 1 FROM events WHERE request_id = '{{requestId}}' AND tenant_id = '{{tenantId}}')",
      "message": {
        "headers": {
          "x-request-id": "{{requestId}}",
          "x-tenant-id": "{{tenantId}}"
        },
        "body": {
          "id": "{{requestId}}",
          "tenant_id": "{{tenantId}}",
          "retry_count": "{{retryCount}}",
          "merchant_id": "{{merchantId}}"
        }
      }
    }
  ]
}
```

O sufixo `()` indica uma chamada de função geradora. Valores sem `()` são literais hardcoded e podem ser strings, números, booleanos, objetos, arrays ou `null`.
As mesmas variáveis também podem ser definidas globalmente no `config.json` usando `vars`; variáveis do flow sobrescrevem variáveis globais com o mesmo nome.
Uma variável também pode vir de SQL usando `{"sql": "SELECT ..."}`. Essa query deve retornar exatamente uma linha e uma coluna; o valor dessa coluna vira o valor da variável.
O valor pode ser usado com `{{nomeDaVar}}` em `message.body`, `message.headers` e `poll_query`. Quando um campo do body é exatamente `{{nomeDaVar}}`, o tipo original é preservado. A função disponível inicialmente é `uuid()`.
Também é aceito declarar a variável diretamente na raiz, por exemplo `"requestId": "uuid()"`.

## Usar

```sh
rmqiw
```

Na TUI:

- `up` / `down`: navegar;
- `enter`: selecionar ou continuar;
- `esc`: sair.

Ritmos:

- `The clock is ticking`: executa todos os steps em sequência.
- `Step by step`: pausa antes de cada step e continua com `enter`.

## Desenvolvimento

```sh
RMQIW_PATH="$PWD/rmqiuwpath" go run .
go test ./...
```
