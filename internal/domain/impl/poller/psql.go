package poller

import (
	"context"
	"net"
	"net/url"
	"strconv"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/megalypse/go/rmqiw/internal/cfg"
	"github.com/megalypse/go/rmqiw/internal/domain/interfaces"
)

var (
	pollerMu      sync.Mutex
	pollerPsql    interfaces.Poller
	pollerProfile *cfg.Config
)

func GetPsql(ctx context.Context) (interfaces.Poller, error) {
	config, err := cfg.GetCfg()
	if err != nil {
		return nil, err
	}
	return GetPsqlForConfig(ctx, config)
}

func GetPsqlForConfig(ctx context.Context, config *cfg.Config) (interfaces.Poller, error) {
	pollerMu.Lock()
	defer pollerMu.Unlock()

	if pollerPsql != nil && pollerProfile == config {
		return pollerPsql, nil
	}

	next, err := newPollerPsql(ctx, config)
	if err != nil {
		return nil, err
	}

	if current, ok := pollerPsql.(*psql); ok {
		_ = current.Close(ctx)
	}
	pollerPsql = next
	pollerProfile = config

	return pollerPsql, nil
}

func CheckConnection(ctx context.Context, config *cfg.Config) error {
	conn, err := pgx.Connect(ctx, postgresURL(config.Postgres))
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	return conn.Ping(ctx)
}

type psql struct {
	conn *pgx.Conn
}

func newPollerPsql(ctx context.Context, config *cfg.Config) (interfaces.Poller, error) {
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
