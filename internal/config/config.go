package config

import (
	"flag"
	"fmt"
	"github.com/caarlos0/env/v6"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"os"
	"time"
)

type Config struct {
	RunAddr                                       string        `env:"SERVER_ADDRESS" envDefault:":8081" validate:"hostname_port"`
	LogLevel                                      string        `env:"LOG_LEVEL" envDefault:"debug" validate:"loglevel"`
	DatabaseDSN                                   string        `env:"DATABASE_URI"`
	MigrationsDir                                 string        `env:"MIGRATIONS_DIR" envDefault:"migrations"`
	AuthCookieSigningSecretKey                    string        `env:"AUTH_COOKIE_SIGNING_SECRET_KEY" envDefault:"LduYtmp2gWSRuyQyRHqbog=="`
	AuthCookieName                                string        `env:"AUTH_COOKIE_NAME" envDefault:"auth"`
	DelayBetweenQueueFetchesForBalancesCalculator time.Duration `env:"DELAY_BETWEEN_QUEUE_FETCHES_FOR_BALANCES_CALCULATOR" envDefault:"5s"`
	ErrorChannelCapacity                          int           `env:"ERROR_CHANNEL_CAPACITY" envDefault:"1024"`
	DelayBetweenQueueFetchesForAccrualsFetcher    time.Duration `env:"DELAY_BETWEEN_QUEUE_FETCHES_FOR_ACCRUALS_FETCHER" envDefault:"5s"`
	OrdersBatchSizeForBalancesCalculator          int           `env:"ORDERS_BATCH_SIZE_FOR_BALANCES_CALCULATOR" envDefault:"500"`
	OrdersBatchSizeForAccrualsFetcher             int           `env:"ORDERS_BATCH_SIZE_FOR_ACCRUALS_FETCHER" envDefault:"500"`
	SchemaForAccrualsFetcher                      string        `env:"SCHEMA_FOR_ACCRUALS_FETCHER" envDefault:"http"`
	HostForAccrualsFetcher                        string        `env:"HOST_FOR_ACCRUALS_FETCHER" envDefault:"localhost"`
	PortForAccrualsFetcher                        string        `env:"PORT_FOR_ACCRUALS_FETCHER" envDefault:"8080"`
	HttpClientTimeoutForAccrualsFetcher           time.Duration `env:"HTTP_CLIENT_TIMEOUT_FOR_ACCRUALS_FETCHER" envDefault:"10s"`
}

func validateFilePath(fieldLevel validator.FieldLevel) bool {
	path := fieldLevel.Field().String()
	_, err := os.Stat(path)

	return err == nil || os.IsNotExist(err)
}

func validateLogLevel(fieldLevel validator.FieldLevel) bool {
	value := fieldLevel.Field().String()

	allowedLogLevels := map[string]bool{
		"debug":   true,
		"info":    true,
		"warning": true,
		"error":   true,
		"fatal":   true,
	}

	return allowedLogLevels[value]
}

func (conf *Config) Validate() error {
	validate := validator.New()

	err := validate.RegisterValidation("loglevel", validateLogLevel)
	if err != nil {
		return err
	}

	err = validate.RegisterValidation("filepath", validateFilePath)
	if err != nil {
		return err
	}

	return validate.Struct(conf)
}

type InitOption func(*initOptions)

type initOptions struct {
	disableFlagsParsing bool
}

func WithDisableFlagsParsing(disableFlagsParsing bool) InitOption {
	return func(options *initOptions) {
		options.disableFlagsParsing = disableFlagsParsing
	}
}

func New(optionsProto ...InitOption) (*Config, error) {
	options := &initOptions{
		disableFlagsParsing: false,
	}
	for _, protoOption := range optionsProto {
		protoOption(options)
	}

	err := godotenv.Load()
	if err != nil {
		return nil, fmt.Errorf("unable to load .env file:  %w", err)
	}

	values := Config{}

	err = env.Parse(&values)
	if err != nil {
		return nil, err
	}

	if !options.disableFlagsParsing {
		flag.StringVar(&values.RunAddr, "a", values.RunAddr, "address and port to run server")
		flag.StringVar(&values.DatabaseDSN, "d", values.DatabaseDSN, "A string with the database connection details")
		flag.Parse()
	}

	return &values, values.Validate()
}
