package sensucheckmysql

import (
	"fmt"

	"gopkg.in/ini.v1"
)

func ParseIni(iniFile, Section string) (dbUser string, dbPass string, dbSocket string, dbHost string, err error) {

	ini, err := ini.Load(iniFile)
	if err != nil {
		return "", "", "", "", fmt.Errorf("error parsing ini file: %v", err)
	}
	dbUser = ini.Section(Section).Key("user").String()
	dbPass = ini.Section(Section).Key("password").String()
	dbSocket = ini.Section(Section).Key("socket").String()
	dbHost = ini.Section(Section).Key("host").String()

	return dbUser, dbPass, dbSocket, dbHost, nil
}
