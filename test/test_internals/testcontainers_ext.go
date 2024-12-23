package test_internals

import "github.com/testcontainers/testcontainers-go"

type EnvCustomizer struct {
	testcontainers.ContainerCustomizer
	varName  string
	varValue string
}

func WithEnvironment(name string, value string) *EnvCustomizer {
	return &EnvCustomizer{
		varName:  name,
		varValue: value,
	}
}

func (c *EnvCustomizer) Customize(req *testcontainers.GenericContainerRequest) error {
	if req.Env == nil {
		req.Env = make(map[string]string)
	}
	req.Env[c.varName] = c.varValue
	return nil
}
