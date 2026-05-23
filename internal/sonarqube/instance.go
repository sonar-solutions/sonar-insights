package sonarqube

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
)

type Product int

const (
	Server Product = iota
	Cloud
)

type SonarInstance struct {
	Product Product
	Version string
	BaseURL string
	Token   string
	Client  *http.Client
}

func (s SonarInstance) AuthorizationHeader() string {
	if s.useBearer() {
		return "Bearer " + s.Token
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(s.Token + ":"))
	return "Basic " + encoded
}

func (s SonarInstance) useBearer() bool {
	if s.Product == Cloud {
		return true
	}
	parts := strings.SplitN(s.Version, ".", 3)
	if len(parts) < 2 {
		return true
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return true
	}
	return major > 10 || (major == 10 && minor >= 2)
}
