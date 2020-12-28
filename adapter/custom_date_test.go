package adapter_test

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/integration-system/isp-lib/v2/scripts"
	"isp-script-service/adapter"
	"isp-script-service/conf"
	"isp-script-service/controller"
	"isp-script-service/domain"
	"isp-script-service/service"

	"github.com/integration-system/isp-lib/v2/config"
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
)

const (
	remoteConf = `{
		"scriptExecutionTimeoutMs": 200
	}`

	id      = "123"
	version = 10

	objectFile = "obj.json"
	scriptFile = "custom_date.js"
)

var (
	data    map[string]interface{}
	request = domain.ExecuteRequest{}
)

func init() {
	config.InitRemoteConfig(&conf.RemoteConfig{}, []byte(remoteConf))

	obj, err := readFile(objectFile)
	if err != nil {
		return
	}
	if err = json.Unmarshal(obj, &data); err != nil {
		return
	}

	obj, err = readFile(scriptFile)
	if err != nil {
		return
	}
	script := string(obj)
	arg := map[string]interface{}{
		"id":      id,
		"version": version,
		"data":    data,
	}
	request = domain.ExecuteRequest{
		Script: script,
		Arg:    arg,
	}
}

func TestCustomDate(t *testing.T) {
	a := assert.New(t)

	execute := controller.Script.Execute(request)
	if !a.Zero(execute.Error) {
		return
	}
	executeResult, _ := json.Marshal(execute.Result)

	customDate := adapter.GetCustomData(data, id, version)
	customDateResult, _ := json.Marshal(customDate)

	if !a.Equal(string(customDateResult), string(executeResult)) {
		log.Println("not equal")
	}
}

func BenchmarkGoCustomData(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = adapter.GetCustomData(data, id, version)
	}
}

func BenchmarkJsCustomData(b *testing.B) {
	scr, err := service.Script.Create(request.Script)
	if err != nil {
		panic(err)
	}
	scriptEngine := scripts.NewEngine()
	for i := 0; i < b.N; i++ {
		_, _ = scriptEngine.Execute(scr, request.Arg)
	}
}

func BenchmarkInlineJson(b *testing.B) {
	c, err := service.Script.Create("var b = JSON.parse(arg); return JSON.stringify(b)")
	if err != nil {
		panic(err)
	}
	data, err := readFile(objectFile)
	if err != nil {
		panic(err)
	}
	scriptEngine := scripts.NewEngine()
	for i := 0; i < b.N; i++ {
		_, err := scriptEngine.Execute(c, string(data))
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkJsoniterJson(b *testing.B) {
	c, err := service.Script.Create("var b = arg; return b")
	if err != nil {
		panic(err)
	}
	data, err := readFile(objectFile)
	if err != nil {
		panic(err)
	}
	json := jsoniter.ConfigFastest
	scriptEngine := scripts.NewEngine()
	for i := 0; i < b.N; i++ {
		m := make(map[string]interface{})
		if err := json.Unmarshal(data, &m); err != nil {
			panic(err)
		}
		res, err := scriptEngine.Execute(c, m)
		if err != nil {
			panic(err)
		}
		if _, err = json.Marshal(res); err != nil {
			panic(err)
		}
	}
}

func readFile(name string) ([]byte, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	obj, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return obj, nil
}
