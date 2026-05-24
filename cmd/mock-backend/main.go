package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/megalypse/go/rmqiw/internal/cfg"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	mockExchange = "rmqiw.mock"
)

type queueBinding struct {
	queue      string
	routingKey string
}

var queueBindings = []queueBinding{
	{queue: "rmqiw.mock.users", routingKey: "users.*"},
	{queue: "rmqiw.mock.orders", routingKey: "orders.*"},
	{queue: "rmqiw.mock.payments", routingKey: "payments.*"},
}

func main() {
	ctx := context.Background()

	config, err := cfg.GetCfg()
	if err != nil {
		log.Fatal(err)
	}

	pg, err := connectPostgres(ctx, config.Postgres)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = pg.Close(ctx)
	}()

	if err := ensureSchema(ctx, pg); err != nil {
		log.Fatal(err)
	}

	rmq, err := connectRabbitMQ(config.RabbitMQ)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = rmq.Close()
	}()

	channel, err := rmq.Channel()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = channel.Close()
	}()

	if err := ensureTopology(channel); err != nil {
		log.Fatal(err)
	}

	log.Println("mock backend ready")
	consume(ctx, channel, pg)
}

func connectPostgres(ctx context.Context, config cfg.PostgresConfig) (*pgx.Conn, error) {
	var conn *pgx.Conn
	var err error
	for attempt := 0; attempt < 30; attempt++ {
		conn, err = pgx.Connect(ctx, postgresURL(config))
		if err == nil {
			return conn, nil
		}

		time.Sleep(time.Second)
	}

	return nil, err
}

func connectRabbitMQ(config cfg.RabbitMQConfig) (*amqp.Connection, error) {
	var conn *amqp.Connection
	var err error
	for attempt := 0; attempt < 30; attempt++ {
		conn, err = amqp.Dial(rabbitMQURL(config))
		if err == nil {
			return conn, nil
		}

		time.Sleep(time.Second)
	}

	return nil, err
}

func ensureSchema(ctx context.Context, conn *pgx.Conn) error {
	_, err := conn.Exec(ctx, `
CREATE TABLE IF NOT EXISTS mock_events (
	id BIGSERIAL PRIMARY KEY,
	routing_key TEXT NOT NULL,
	flow_step TEXT NOT NULL,
	headers JSONB NOT NULL DEFAULT '{}'::jsonb,
	body JSONB NOT NULL DEFAULT '{}'::jsonb,
	received_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`)
	return err
}

func ensureTopology(channel *amqp.Channel) error {
	if err := channel.ExchangeDeclare(mockExchange, "topic", true, false, false, false, nil); err != nil {
		return err
	}

	for _, binding := range queueBindings {
		_, err := channel.QueueDeclare(binding.queue, true, false, false, false, nil)
		if err != nil {
			return err
		}

		err = channel.QueueBind(binding.queue, binding.routingKey, mockExchange, false, nil)
		if err != nil {
			return err
		}
	}

	return nil
}

func consume(ctx context.Context, channel *amqp.Channel, pg *pgx.Conn) {
	deliveries := make(chan amqp.Delivery)

	for _, binding := range queueBindings {
		queueDeliveries, err := channel.Consume(binding.queue, "", false, false, false, false, nil)
		if err != nil {
			log.Fatal(err)
		}

		go forwardDeliveries(queueDeliveries, deliveries)
	}

	for delivery := range deliveries {
		if err := insertEvent(ctx, pg, delivery); err != nil {
			log.Printf("insert failed: %v", err)
			_ = delivery.Nack(false, true)
			continue
		}

		_ = delivery.Ack(false)
	}
}

func forwardDeliveries(input <-chan amqp.Delivery, output chan<- amqp.Delivery) {
	for delivery := range input {
		output <- delivery
	}
}

func insertEvent(ctx context.Context, conn *pgx.Conn, delivery amqp.Delivery) error {
	headers, err := json.Marshal(delivery.Headers)
	if err != nil {
		return err
	}

	body := delivery.Body
	if !json.Valid(body) {
		body = []byte(fmt.Sprintf(`{"raw":%q}`, string(delivery.Body)))
	}

	flowStep := fmt.Sprint(delivery.Headers["flow_step"])
	if flowStep == "" || flowStep == "<nil>" {
		flowStep = delivery.RoutingKey
	}

	_, err = conn.Exec(
		ctx,
		`INSERT INTO mock_events (routing_key, flow_step, headers, body) VALUES ($1, $2, $3, $4)`,
		delivery.RoutingKey,
		flowStep,
		headers,
		body,
	)
	return err
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
	uri.RawQuery = query.Encode()

	return uri.String()
}

func rabbitMQURL(config cfg.RabbitMQConfig) string {
	port := config.Port
	if port == 0 {
		port = 5672
	}

	vhost := config.VHost
	if vhost == "" {
		vhost = "/"
	}

	uri := url.URL{
		Scheme: "amqp",
		User:   url.UserPassword(config.User, config.Password),
		Host:   net.JoinHostPort(config.Host, strconv.Itoa(port)),
		Path:   vhost,
	}

	return uri.String()
}
