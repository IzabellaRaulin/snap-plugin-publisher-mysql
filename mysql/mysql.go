/*
http://www.apache.org/licenses/LICENSE-2.0.txt


Copyright 2015 Intel Corporation

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mysql

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"

	"github.com/intelsdi-x/pulse/control/plugin"
	"github.com/intelsdi-x/pulse/control/plugin/cpolicy"
	"github.com/intelsdi-x/pulse/core/ctypes"

	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

const (
	name       = "mysql"
	version    = 4
	pluginType = plugin.PublisherPluginType
)

type mysqlPublisher struct {
}

func NewMySQLPublisher() *mysqlPublisher {
	return &mysqlPublisher{}
}

// Publish sends data to a MySQL server
func (s *mysqlPublisher) Publish(contentType string, content []byte, config map[string]ctypes.ConfigValue) error {
	logger := log.New()
	logger.Println("Publishing started")
	var metrics []plugin.PluginMetricType

	switch contentType {
	case plugin.PulseGOBContentType:
		dec := gob.NewDecoder(bytes.NewBuffer(content))
		if err := dec.Decode(&metrics); err != nil {
			logger.Printf("Error decoding: error=%v content=%v", err, content)
			return err
		}
	default:
		logger.Printf("Error unknown content type '%v'", contentType)
		return errors.New(fmt.Sprintf("Unknown content type '%s'", contentType))
	}

	logger.Printf("publishing %v to %v", metrics, config)

	// Open connection and ping to make sure it works
	username := config["username"].(ctypes.ConfigValueStr).Value
	password := config["password"].(ctypes.ConfigValueStr).Value
	database := config["database"].(ctypes.ConfigValueStr).Value
	tableName := config["tablename"].(ctypes.ConfigValueStr).Value
	tableColumns := "(timestamp VARCHAR(200), source_column VARCHAR(200), key_column VARCHAR(200), value_column VARCHAR(200))"
	db, err := sql.Open("mysql", username+":"+password+"@/"+database)
	defer db.Close()
	if err != nil {
		logger.Printf("Error: %v", err)
		return err
	}
	err = db.Ping()
	if err != nil {
		logger.Printf("Error: %v", err)
		return err
	}

	// Create the table if it's not already there
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS" + " " + tableName + " " + tableColumns)
	if err != nil {
		logger.Printf("Error: %v", err)
		return err
	}
	// Put the values into the database with the current time
	tableValues := "VALUES( ?, ?, ?, ? )"
	insert, err := db.Prepare("INSERT INTO" + " " + tableName + " " + tableValues)
	if err != nil {
		logger.Printf("Error: %v", err)
		return err
	}
	var key, value string
	for _, m := range metrics {
		key = sliceToString(m.Namespace())
		value, err = interfaceToString(m.Data())
		if err != nil {
			logger.Printf("Error: %v", err)
			return err
		}
		_, err := insert.Exec(m.Timestamp(), m.Source(), key, value)
		if err != nil {
			panic(err)
		}

	}

	return nil
}

func Meta() *plugin.PluginMeta {
	return plugin.NewPluginMeta(name, version, pluginType, []string{plugin.PulseGOBContentType}, []string{plugin.PulseGOBContentType})
}

func (f *mysqlPublisher) GetConfigPolicy() (*cpolicy.ConfigPolicy, error) {
	cp := cpolicy.New()
	config := cpolicy.NewPolicyNode()

	username, err := cpolicy.NewStringRule("username", true, "root")
	handleErr(err)
	username.Description = "Username to login to the MySQL server"

	password, err := cpolicy.NewStringRule("password", true, "root")
	handleErr(err)
	password.Description = "Password to login to the MySQL server"

	database, err := cpolicy.NewStringRule("database", true, "PULSE_TEST")
	handleErr(err)
	database.Description = "The MySQL database that data will be pushed to"

	tableName, err := cpolicy.NewStringRule("tablename", true, "info")
	handleErr(err)
	tableName.Description = "The MySQL table within the database where information will be stored"

	config.Add(username, password, database, tableName)

	cp.Add([]string{""}, config)
	return cp, nil
}

func handleErr(e error) {
	if e != nil {
		panic(e)
	}
}

func sliceToString(slice []string) string {
	return strings.Join(slice, ", ")
}

// Supported types: []string, []int, int, string
func interfaceToString(face interface{}) (string, error) {
	var (
		ret string
		err error
	)
	switch val := face.(type) {
	case []string:
		ret = sliceToString(val)
	case []int:
		length := len(val)
		if length == 0 {
			return ret, err
		}
		ret = strconv.Itoa(val[0])
		if length == 1 {
			return ret, err
		}
		for i := 1; i < length; i++ {
			ret += ", "
			ret += strconv.Itoa(val[i])
		}
	case int:
		ret = strconv.Itoa(val)
	case string:
		ret = val
	default:
		err = errors.New("unsupported type")
	}
	return ret, err
}
