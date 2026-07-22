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

var (
	publisherMu      sync.Mutex
	publisherRMQ     interfaces.Publisher
	publisherProfile *cfg.Config
)

func GetRmq() (interfaces.Publisher, error) {
	config, err := cfg.GetCfg()
	if err != nil {
		return nil, err
	}
	return GetRmqForConfig(config)
}

func GetRmqForConfig(config *cfg.Config) (interfaces.Publisher, error) {
	publisherMu.Lock()
	defer publisherMu.Unlock()

	if publisherRMQ != nil && publisherProfile == config {
		return publisherRMQ, nil
	}

	next, err := newPublisherRMQ(config)
	if err != nil {
		return nil, err
	}

	if current, ok := publisherRMQ.(*PublisherRMQ); ok {
		_ = current.Close()
	}
	publisherRMQ = next
	publisherProfile = config

	return publisherRMQ, nil
}

func CheckConnection(config *cfg.Config) error {
	conn, err := amqp.Dial(rabbitMQURL(config.RabbitMQ))
	if err != nil {
		return err
	}

	return conn.Close()
}

type PublisherRMQ struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

func newPublisherRMQ(config *cfg.Config) (interfaces.Publisher, error) {
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
		if config.TLS {
			port = 5671
		} else {
			port = 5672
		}
	}

	vhost := config.VHost
	if vhost == "" {
		vhost = "/"
	}

	scheme := "amqp"
	if config.TLS {
		scheme = "amqps"
	}

	uri := url.URL{
		Scheme: scheme,
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
