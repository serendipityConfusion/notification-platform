package config

type RedisConfig struct {
	Addr     string `json:"addr" yaml:"addr"`
	Password string `json:"password" yaml:"password"`
	UserName string `json:"username" yaml:"username"`
}
