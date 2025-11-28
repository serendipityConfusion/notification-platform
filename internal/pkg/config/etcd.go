package config

import "time"

type EtcdConfig struct {
	Endpoints   []string      `json:"endpoints" yaml:"endpoints"`
	DialTimeout time.Duration `json:"dial-timeout" yaml:"dial-timeout"`
	Username    string        `json:"username" yaml:"username"`
	Password    string        `json:"password" yaml:"password"`
}
