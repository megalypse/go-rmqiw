package publisher

import (
	"context"
	"net"
	"net/url"
	"strconv"
	"sync"

	"github.com/megalypse/go/rmqiw/internal/cfg"
	"github.com/megalypse/go/rmqiw/internal/domain/interfaces"
	amqp "github.com/rabbitmq/amqp091-go"
)

var GetRmq = sync.OnceValues(newPublisherRMQ)

type PublisherRMQ struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

func newPublisherRMQ() (interfaces.Publisher, error) {
	config, err := cfg.GetCfg()
	if err != nil {
		return nil, err
	}

	conn, err := amqp.Dial(rabbitMQURL(config.RabbitMQ))
	if err != nil {
		return nil, err
	}

	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	return &PublisherRMQ{
		conn:    conn,
		channel: channel,
	}, nil
}

func (p *PublisherRMQ) Publish(
	ctx context.Context,
	exchange string,
	routingKey string,
	headers map[string]string,
	body []byte,
) error {
	return p.channel.PublishWithContext(
		ctx,
		exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			Headers: amqpHeaders(headers),
			Body:    body,
		},
	)
}

func (p *PublisherRMQ) Close() error {
	if err := p.channel.Close(); err != nil {
		_ = p.conn.Close()
		return err
	}

	return p.conn.Close()
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

func amqpHeaders(headers map[string]string) amqp.Table {
	if len(headers) == 0 {
		return nil
	}

	table := make(amqp.Table, len(headers))
	for key, value := range headers {
		table[key] = value
	}

	return table
}
