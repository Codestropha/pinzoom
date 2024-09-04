package config

type Config struct {
	Http HttpConfig `yaml:"http"`
}

type HttpConfig struct {
	Listen string `yaml:"listen"`
}

func NewConfig() *Config {
	return &Config{
		Http: HttpConfig{Listen: ":3002"},
	}
}
