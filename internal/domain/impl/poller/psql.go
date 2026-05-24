package poller

import (
	"context"
	"net"
	"net/url"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/megalypse/go/rmqiw/internal/cfg"
	"github.com/megalypse/go/rmqiw/internal/domain/interfaces"
)

var pollerPsql interfaces.Poller

func GetPsql(ctx context.Context) (interfaces.Poller, error) {
	var err error
	if pollerPsql == nil {
		pollerPsql, err = newPollerPsql(ctx)
		if err != nil {
			return nil, err
		}
	}

	return pollerPsql, err
}

type psql struct {
	conn *pgx.Conn
}

func newPollerPsql(ctx context.Context) (interfaces.Poller, error) {
	config, err := cfg.GetCfg()
	if err != nil {
		return nil, err
	}

	conn, err := pgx.Connect(ctx, postgresURL(config.Postgres))
	if err != nil {
		return nil, err
	}

	return &psql{
		conn: conn,
	}, nil
}

func (p *psql) Poll(ctx context.Context, query string) (bool, error) {
	var ok bool
	err := p.conn.QueryRow(ctx, query).Scan(&ok)
	if err != nil {
		return false, err
	}

	return ok, nil
}

func (p *psql) Close(ctx context.Context) error {
	return p.conn.Close(ctx)
}

func postgresURL(config cfg.PostgresConfig) string {
	port := config.Port
	if port == 0 {
		port = 5432
	}

	sslMode := config.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	uri := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(config.User, config.Password),
		Host:   net.JoinHostPort(config.Host, strconv.Itoa(port)),
	}

	if config.Database != "" {
		uri.Path = config.Database
	}

	query := uri.Query()
	query.Set("sslmode", sslMode)
	query.Set("default_transaction_read_only", "on")
	uri.RawQuery = query.Encode()

	return uri.String()
}
