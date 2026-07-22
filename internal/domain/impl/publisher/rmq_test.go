package publisher

import (
	"testing"

	"github.com/megalypse/go/rmqiw/internal/cfg"
)

func TestRabbitMQURLUsesTLS(t *testing.T) {
	got := rabbitMQURL(cfg.RabbitMQConfig{
		Host:     "rabbitmq.internal",
		User:     "user",
		Password: "password",
		VHost:    "/",
		TLS:      true,
	})

	want := "amqps://user:password@rabbitmq.internal:5671/"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestRabbitMQURLPreservesExplicitTLSPort(t *testing.T) {
	got := rabbitMQURL(cfg.RabbitMQConfig{
		Host:     "rabbitmq.internal",
		Port:     5672,
		User:     "user",
		Password: "password",
		VHost:    "/",
		TLS:      true,
	})

	want := "amqps://user:password@rabbitmq.internal:5672/"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
