package load_test

type TestEndpointConfig struct {
	Proto      string // http or https
	Host       string // localhost or google.com
	Port       string // 1234
	PathPrefix string // api/foo/bar   (no prefix or trailing slashes)
}
