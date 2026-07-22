package cfg

type Config struct {
	Name      string         `json:"name"`
	Postgres  PostgresConfig `json:"postgres"`
	RabbitMQ  RabbitMQConfig `json:"rabbitmq"`
	Vars      map[string]any `json:"vars"`
	Variables map[string]any `json:"variables"`
}

type PostgresConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
	SSLMode  string `json:"ssl_mode"`
}

type RabbitMQConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	VHost    string `json:"vhost"`
	TLS      bool   `json:"tls"`
}
