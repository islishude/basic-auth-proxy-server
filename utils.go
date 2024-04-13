package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

type Backend struct {
	Target    string `json:"target" yaml:"target"`
	Readiness string `json:"readiness" yaml:"readiness"`
}

type Config struct {
	Port    uint16            `json:"port" yaml:"port"`
	Backend *Backend          `json:"backend" yaml:"backend"`
	Users   map[string]string `json:"users" yaml:"users"`
}

func ParseConfig(p string) (*Config, error) {
	file, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}

	conf := new(Config)
	switch ext := path.Ext(p); ext {
	case ".json":
		err = json.Unmarshal(file, &conf)
	case ".yaml", ".yml":
		err = yaml.Unmarshal(file, &conf)
	default:
		err = fmt.Errorf("not supported file extension %s", ext)
	}
	if err != nil {
		return nil, err
	}
	if conf.Port == 0 {
		conf.Port = 8080
	}
	return conf, nil
}

func VerifyUserPass(db map[string]string, user, pass string) bool {
	hashedPassword, validUser := db[user]
	if !validUser {
		// The user is not found. Use a fixed password hash to
		// prevent user enumeration by timing requests.
		// This is a bcrypt-hashed version of "fakepassword".
		hashedPassword = "$2y$10$QOauhQNbBCuQDKes6eFzPeMqBSjb7Mr5DUmpZ/VcEd00UAV/LDeSi"
	}

	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(pass)) == nil && validUser
}
