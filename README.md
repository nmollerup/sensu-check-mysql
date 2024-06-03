[![Sensu Bonsai Asset](https://img.shields.io/badge/Bonsai-Download%20Me-brightgreen.svg?colorB=89C967&logo=sensu)](https://bonsai.sensu.io/assets/nmollerup/sensu-check-mysql)
[![goreleaser](https://github.com/nmollerup/sensu-check-mysql/actions/workflows/release.yml/badge.svg)](https://github.com/nmollerup/sensu-check-mysql/actions/workflows/release.yml) [![Go Test](https://github.com/nmollerup/sensu-check-mysql/actions/workflows/test.yml/badge.svg)](https://github.com/nmollerup/sensu-check-mysql/actions/workflows/test.yml) 
# sensu-check-mysql

## Table of Contents

## Usage

### Help Text Output

```
Mysql alive check

Usage:
  check-mysql-alive [flags]
  check-mysql-alive [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  version     Print the version number of this plugin

Flags:
  -d, --database string      Database schema to connect to (default "test")
  -h, --help                 help for check-mysql-alive
      --hostname string      Hostname to login to (default "localhost")
  -i, --ini string           Location of my.cnf ini file for access to MySQL
      --ini-section string   Section to use from my.cnf ini file (default "client")
  -p, --password string      Password for user
      --port uint            Port to connect to (default 3306)
  -s, --socket string        Socket to use
  -u, --user string          MySQL user to connect

Use "check-mysql-alive [command] --help" for more information about a command.
```

### Configuration

The check connects to the supplied MySQL database and returns OK if it works. Ini file overrides commandline arguments for user/pass.
