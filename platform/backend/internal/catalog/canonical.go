package catalog

import (
	"regexp"
	"strings"
)

var versionSuffix = regexp.MustCompile(`\d[\d.]*$`)

var canonicalAliases = map[string]string{
	"apache":          "apache",
	"apache2":         "apache",
	"httpd":           "apache",
	"elastic":         "elasticsearch",
	"mariadb":         "mysql",
	"mongod":          "mongodb",
	"mysqld":          "mysql",
	"openssh":         "openssh",
	"php":             "phpfpm",
	"php-fpm":         "phpfpm",
	"phpfpm":          "phpfpm",
	"postgresql":      "postgresql",
	"postgres":        "postgresql",
	"rabbitmq":        "rabbitmq",
	"rabbitmq-server": "rabbitmq",
	"redis-server":    "redis",
	"sshd":            "openssh",
	"nginx":      		"nginx",
	"openresty":  		"nginx",
}

func Canonicalize(rawName string) (string, bool) {
	name := strings.ToLower(strings.TrimSpace(rawName))
	if name == "" {
		return "", false
	}

	if _, ok := Catalog[name]; ok {
		return name, true
	}
	if canonical, ok := canonicalAliases[name]; ok {
		if _, ok := Catalog[canonical]; ok {
			return canonical, true
		}
	}

	stripped := versionSuffix.ReplaceAllString(name, "")
	stripped = strings.TrimRight(stripped, "-_. ")
	if stripped != "" && stripped != name {
		if _, ok := Catalog[stripped]; ok {
			return stripped, true
		}
		if canonical, ok := canonicalAliases[stripped]; ok {
			if _, ok := Catalog[canonical]; ok {
				return canonical, true
			}
		}
	}

	return "", false
}

func ResolveService(rawName string) (ServiceDef, bool) {
	canonical, ok := Canonicalize(rawName)
	if !ok {
		return ServiceDef{}, false
	}
	return Get(canonical)
}

func IsScannable(rawName string) bool {
	def, ok := ResolveService(rawName)
	return ok && def.NVD != nil
}
