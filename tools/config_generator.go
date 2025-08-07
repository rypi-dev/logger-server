package main

import (
	"flag"
	"fmt"
	"os"
	"text/template"
)

const fluentTemplate = `
[SERVICE]
    Flush        {{.Flush}}
    Log_Level    {{.LogLevel}}
    Parsers_File parsers.conf

[INPUT]
    Name   http
    Listen 0.0.0.0
    Port   {{.Port}}
    Format json
    Tag    incoming.log

[FILTER]
    Name       lua
    Match      incoming.*
    script     lua/inject_header.lua
    call       add_log_level_header

[FILTER]
    Name       modify
    Match      incoming.*
    Add        stack multi-lang

[OUTPUT]
    Name   http
    Match  incoming.*
    Host   {{.Host}}
    Port   {{.HostPort}}
    URI    /log
    Format json
    Header Content-Type application/json
    Header X-Log-Level ${X-Log-Level}
`

type Config struct {
	Flush    int
	LogLevel string
	Port     int
	Host     string
	HostPort int
}

func main() {
	flush := flag.Int("flush", 1, "Flush interval")
	logLevel := flag.String("loglevel", "info", "Log level")
	port := flag.Int("port", 8888, "HTTP input port")
	host := flag.String("host", "logger-server", "Output host")
	hostPort := flag.Int("hostport", 8080, "Output host port")
	outFile := flag.String("out", "fluent-bit.conf", "Output filename")
	flag.Parse()

	cfg := Config{
		Flush:    *flush,
		LogLevel: *logLevel,
		Port:     *port,
		Host:     *host,
		HostPort: *hostPort,
	}

	f, err := os.Create(*outFile)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	tmpl, err := template.New("fluent").Parse(fluentTemplate)
	if err != nil {
		panic(err)
	}

	err = tmpl.Execute(f, cfg)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Config generated to %s\n", *outFile)
}